package loudness

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"
)

const (
	// Null-test thresholds: the energy below which what is left over after
	// undoing the gain is the cost of rewriting the file rather than a change
	// to the audio.
	//
	// Calibrated by measurement, not judgement. Rewriting a lossy file adds a
	// generation of codec noise even at identical settings; across real music
	// that leftover measures -39 to -47 dB, and pure synthetic tones - which an
	// encoder reproduces almost exactly - reach -90. Running the same track
	// through an actual limiter or compressor instead leaves -10 to -15. The
	// two outcomes are 25 dB apart at their closest, so -30 separates them with
	// room on both sides.
	//
	// A lossless round trip has no codec noise to account for and should null
	// almost perfectly, so anything above -60 there means the audio itself was
	// altered.
	nullResidualSafeLossyDB    = -30.0
	nullResidualSafeLosslessDB = -60.0

	// pureGainToleranceDB: under a constant gain the true peak moves by exactly
	// the applied gain and the loudness range does not move at all. Allow this
	// much slack for encoder/resampler noise before calling it non-linear.
	pureGainToleranceDB = 0.5
)

// Measurement is everything measurable about one audio file at a point in time.
type Measurement struct {
	Probe    *ffmpeg.FileProbe
	LUFS     float64
	TruePeak float64
	LRA      float64
}

// Measure probes the container properties and measures the loudness of path.
func Measure(ctx context.Context, normalizer ffmpeg.LoudnessNormalizer, path string, target ffmpeg.LoudnessTarget) (*Measurement, error) {
	probe, err := ffmpeg.ProbeFile(ctx, path)
	if err != nil {
		return nil, err
	}
	// The probe tells us how long the track is, so the analysis - which decodes
	// all of it - can be bounded by its actual length rather than a blanket
	// limit that would either be too tight for a long mix or useless for a
	// short one.
	ctx, cancel := context.WithTimeout(ctx, ffmpeg.DecodeTimeout(probe.Duration))
	defer cancel()

	analysis, err := normalizer.AnalyzeLoudness(ctx, path, target)
	if err != nil {
		return nil, err
	}
	return &Measurement{
		Probe:    probe,
		LUFS:     analysis.InputIntegrated,
		TruePeak: analysis.InputTruePeak,
		LRA:      analysis.InputLRA,
	}, nil
}

// Audit builds the complete before/after record for one track.
//
// When a pre-normalization backup exists, the backup is the "before" and the
// file on disk is the "after", and the two are compared directly - including a
// null test that proves whether anything beyond the level changed. When no
// backup exists the file has not been processed, so its current state is
// recorded as the "before" and there is nothing to compare against yet.
func Audit(ctx context.Context, normalizer ffmpeg.LoudnessNormalizer, mediaFileID, libraryPath, trackPath string,
	target ffmpeg.LoudnessTarget, tolerance float64, backupFolder string) *model.LoudnessAudit {
	return auditWith(ctx, normalizer, mediaFileID, libraryPath, trackPath, nil, target, tolerance, backupFolder)
}

// auditWith is Audit with the option of reusing a measurement of trackPath the
// caller already holds. Measuring decodes the entire file, so a caller that has
// just measured it should not pay for it a second time.
func auditWith(ctx context.Context, normalizer ffmpeg.LoudnessNormalizer, mediaFileID, libraryPath, trackPath string,
	current *Measurement, target ffmpeg.LoudnessTarget, tolerance float64, backupFolder string) *model.LoudnessAudit {
	audit := &model.LoudnessAudit{MediaFileID: mediaFileID, AnalyzedAt: time.Now()}

	backup := ffmpeg.FindLoudnessBackup(backupFolder, libraryPath, mediaFileID, trackPath)
	audit.HasBackup = backup != ""

	if current == nil {
		measured, err := Measure(ctx, normalizer, trackPath, target)
		if err != nil {
			audit.Status = model.LoudnessStatusFailed
			audit.Verdict = model.LoudnessVerdictFailed
			audit.Error = err.Error()
			return audit
		}
		current = measured
	}

	// The phase describes what still needs doing, judged from the file as it
	// stands now: reachable by a constant gain, or in need of a decision.
	plan := PlanFor(current.LUFS, current.TruePeak, target.IntegratedLUFS, target.TruePeak, tolerance, current.Probe.BitRate)
	audit.Phase = plan.Phase

	if !audit.HasBackup {
		// Never processed: the file as it stands is the "before" snapshot.
		recordBefore(audit, current)
		audit.Status = model.LoudnessStatusAnalyzed
		if plan.Phase == PhaseDone {
			audit.Verdict = model.LoudnessVerdictUntouched
			audit.Action = model.LoudnessActionSkipped
		}
		return audit
	}

	original, err := Measure(ctx, normalizer, backup, target)
	if err != nil {
		// The processed file measured fine, so keep it as the before snapshot
		// rather than losing the measurement entirely.
		recordBefore(audit, current)
		audit.Status = model.LoudnessStatusFailed
		audit.Verdict = model.LoudnessVerdictFailed
		audit.Error = fmt.Sprintf("reading backup: %v", err)
		return audit
	}

	recordBefore(audit, original)
	recordAfter(audit, current)
	audit.Status = model.LoudnessStatusProcessed

	gain := current.LUFS - original.LUFS
	audit.GainApplied = &gain

	if residual, err := ffmpeg.NullResidual(ctx, backup, trackPath, gain); err == nil {
		audit.NullResidual = &residual
	} else {
		audit.Error = fmt.Sprintf("null test: %v", err)
	}

	audit.Action = inferAction(audit, original, current, gain)
	audit.Verdict = verdict(audit, original, current)
	return audit
}

// AuditFromOptimize builds the audit record out of the measurements a run has
// already taken, rather than measuring the same two files over again.
//
// A run measures the track before it encodes, and measures the result to verify
// it. That is exactly the before/after pair an audit records, so calling Audit
// afterwards decodes both files a second time - the most expensive thing in the
// pipeline - purely to learn what the run already knows. Only the null test,
// which compares the stored original against the finished file, still has work
// to do.
//
// The shortcut holds only when this run is what stored the original. An
// existing backup is never overwritten, so for a track being processed a second
// time the stored original is older than anything this run measured and the
// "before" snapshot has to be read from the backup itself.
func AuditFromOptimize(ctx context.Context, normalizer ffmpeg.LoudnessNormalizer,
	mediaFileID, libraryPath, trackPath string, res OptimizeResult,
	target ffmpeg.LoudnessTarget, tolerance float64, backupFolder string) *model.LoudnessAudit {
	// Whichever side the run measured last is the file as it now stands on disk.
	current := res.BeforeSet
	if res.Changed && res.AfterSet != nil {
		current = res.AfterSet
	}

	if !res.Changed || !res.BackupCreated || res.BeforeSet == nil || res.AfterSet == nil {
		// Either nothing was rewritten, or the stored original predates this
		// run. Reuse whatever was measured of the file on disk and let the
		// normal path read the backup if there is one.
		audit := auditWith(ctx, normalizer, mediaFileID, libraryPath, trackPath, current,
			target, tolerance, backupFolder)
		// Why a produced file was thrown away is known only to the run that
		// threw it away, and cannot be reconstructed afterwards. Without it a
		// track that refuses to process looks exactly like one nothing was
		// ever attempted on.
		if res.Rejected != "" {
			audit.Error = res.Rejected
			// Marked so the next run does not rebuild the same rejected file.
			// Nothing about the track or the settings has changed, so the
			// outcome would not either; a fresh analysis clears this and the
			// track is tried again.
			audit.Action = model.LoudnessActionRefused
		}
		return audit
	}

	before, after := res.BeforeSet, res.AfterSet
	audit := &model.LoudnessAudit{MediaFileID: mediaFileID, AnalyzedAt: time.Now()}

	backup := ffmpeg.FindLoudnessBackup(backupFolder, libraryPath, mediaFileID, trackPath)
	audit.HasBackup = backup != ""

	recordBefore(audit, before)
	recordAfter(audit, after)
	audit.Status = model.LoudnessStatusProcessed
	audit.Phase = PlanFor(after.LUFS, after.TruePeak, target.IntegratedLUFS, target.TruePeak, tolerance, after.Probe.BitRate).Phase

	gain := after.LUFS - before.LUFS
	audit.GainApplied = &gain

	if backup != "" {
		if residual, err := ffmpeg.NullResidual(ctx, backup, trackPath, gain); err == nil {
			audit.NullResidual = &residual
		} else {
			audit.Error = fmt.Sprintf("null test: %v", err)
		}
	}

	audit.Action = inferAction(audit, before, after, gain)
	audit.Verdict = verdict(audit, before, after)
	return audit
}

// MeasureOriginal records only what the song is right now: one loudness pass
// plus a container probe.
//
// It deliberately skips the two expensive parts of a full audit - re-measuring
// the stored original and running the null test - because neither says
// anything about a file that has not been rewritten. For a processed track it
// therefore reports the file as it currently stands, not its pre-processing
// original, so it is only used where no audit exists yet.
func MeasureOriginal(ctx context.Context, normalizer ffmpeg.LoudnessNormalizer, mediaFileID, trackPath string,
	target ffmpeg.LoudnessTarget, tolerance float64) *model.LoudnessAudit {
	audit := &model.LoudnessAudit{MediaFileID: mediaFileID, AnalyzedAt: time.Now()}

	current, err := Measure(ctx, normalizer, trackPath, target)
	if err != nil {
		audit.Status = model.LoudnessStatusFailed
		audit.Verdict = model.LoudnessVerdictFailed
		audit.Error = err.Error()
		return audit
	}

	recordBefore(audit, current)
	audit.Status = model.LoudnessStatusAnalyzed

	plan := PlanFor(current.LUFS, current.TruePeak, target.IntegratedLUFS, target.TruePeak, tolerance, current.Probe.BitRate)
	audit.Phase = plan.Phase
	if plan.Phase == PhaseDone {
		audit.Verdict = model.LoudnessVerdictUntouched
		audit.Action = model.LoudnessActionSkipped
	}
	return audit
}

func recordBefore(audit *model.LoudnessAudit, m *Measurement) {
	lufs, tp, lra := m.LUFS, m.TruePeak, m.LRA
	audit.LufsBefore = &lufs
	audit.TpBefore = &tp
	audit.LraBefore = &lra
	audit.CodecBefore = m.Probe.Codec
	audit.BitrateBefore = m.Probe.BitRate
	audit.SampleRateBefore = m.Probe.SampleRate
	audit.BitDepthBefore = m.Probe.BitDepth
	audit.ChannelsBefore = m.Probe.Channels
	audit.DurationBefore = m.Probe.Duration
	audit.SizeBefore = m.Probe.Size
	audit.ArtBefore = m.Probe.HasArt
}

func recordAfter(audit *model.LoudnessAudit, m *Measurement) {
	lufs, tp, lra := m.LUFS, m.TruePeak, m.LRA
	audit.LufsAfter = &lufs
	audit.TpAfter = &tp
	audit.LraAfter = &lra
	audit.ArtAfter = m.Probe.HasArt
	// Same probe as the before snapshot, so the two are directly comparable.
	audit.CodecAfter = m.Probe.Codec
	audit.BitrateAfter = m.Probe.BitRate
	audit.SampleRateAfter = m.Probe.SampleRate
	audit.BitDepthAfter = m.Probe.BitDepth
	audit.ChannelsAfter = m.Probe.Channels
	audit.DurationAfter = m.Probe.Duration
	audit.SizeAfter = m.Probe.Size
}

// inferAction works out whether the change was a constant gain or something
// that reshaped the audio.
//
// The null test decides wherever it is available. It measures how much of the
// file changed beyond the level, and the two outcomes are 25 dB apart at their
// closest - far more separation than any other signal offers.
//
// Without one, the peak and the loudness range answer instead, and the peak is
// read in one direction only. A peak LOWER than the gain predicts means
// something pushed it down, which is what limiting does. A peak HIGHER than
// predicted cannot be limiting - a limiter never raises anything - it is the
// encoder rebuilding the waveform imperfectly, which low-bitrate sources do by
// up to 0.8 dB. Treating the two alike reported untouched tracks as reshaped.
func inferAction(audit *model.LoudnessAudit, before, after *Measurement, gain float64) string {
	if audit.NullResidual != nil {
		if *audit.NullResidual > NullResidualThreshold(after.Probe.Codec) {
			return model.LoudnessActionLimited
		}
		return model.LoudnessActionGain
	}

	if math.Abs(after.LRA-before.LRA) > pureGainToleranceDB {
		return model.LoudnessActionLimited
	}
	if shavedBy := (before.TruePeak + gain) - after.TruePeak; shavedBy > pureGainToleranceDB {
		return model.LoudnessActionLimited
	}
	return model.LoudnessActionGain
}

// verdict grades the change. Format degradation outranks everything else: once
// the codec, bitrate, sample rate, bit depth, channel count, duration or cover
// art changed, the file is no longer the client's original regardless of how
// well the loudness landed.
//
// Below that, "re-encoded" means what it says: the file came back in a worse
// format than it went in. It is not a place to put a change that could not be
// explained, which is what it became while the null test reported here directly
// - a track whose audio was untouched but whose rewrite cost more than expected
// was labelled as having lost quality, which is a different and worse claim.
func verdict(audit *model.LoudnessAudit, before, after *Measurement) string {
	if len(IntegrityIssues(before, after)) > 0 {
		return model.LoudnessVerdictReencoded
	}
	// Whether the audio itself was reshaped is settled by inferAction, which
	// reads the null test where there is one and the peak and range where there
	// is not.
	if audit.Action == model.LoudnessActionLimited {
		return model.LoudnessVerdictDynamicsChanged
	}
	// A backstop for a track with no null test whose range moved anyway.
	if math.Abs(after.LRA-before.LRA) > pureGainToleranceDB {
		return model.LoudnessVerdictDynamicsChanged
	}
	return model.LoudnessVerdictSafe
}

// IsLossy reports whether a codec discards information on encode, and so adds
// a generation of noise every time the file is rewritten.
func IsLossy(codec string) bool {
	switch strings.ToLower(codec) {
	case "flac", "alac", "wavpack", "pcm_s16le", "pcm_s24le", "pcm_s32le", "pcm_f32le":
		return false
	default:
		return true
	}
}

// NullResidualThreshold returns the level below which the leftover from a null
// test is codec noise rather than a change to the audio.
func NullResidualThreshold(codec string) float64 {
	if IsLossy(codec) {
		return nullResidualSafeLossyDB
	}
	return nullResidualSafeLosslessDB
}

// IntegrityIssues lists everything that changed which should not have.
func IntegrityIssues(before, after *Measurement) []string {
	var issues []string
	b, a := before.Probe, after.Probe
	if b.Codec != a.Codec {
		issues = append(issues, fmt.Sprintf("codec %s->%s", b.Codec, a.Codec))
	}
	if a.BitRate < b.BitRate {
		issues = append(issues, fmt.Sprintf("bitrate %dk->%dk", b.BitRate, a.BitRate))
	}
	if b.SampleRate != a.SampleRate {
		issues = append(issues, fmt.Sprintf("rate %d->%d", b.SampleRate, a.SampleRate))
	}
	if b.BitDepth > 0 && a.BitDepth > 0 && b.BitDepth != a.BitDepth {
		issues = append(issues, fmt.Sprintf("depth %d->%d", b.BitDepth, a.BitDepth))
	}
	if b.Channels != a.Channels {
		issues = append(issues, fmt.Sprintf("channels %d->%d", b.Channels, a.Channels))
	}
	if math.Abs(b.Duration-a.Duration) > 0.05 {
		issues = append(issues, "duration changed")
	}
	if b.HasArt && !a.HasArt {
		issues = append(issues, "cover art lost")
	}
	return issues
}
