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
	// Null-test thresholds: the level below which what is left over after
	// undoing the gain is just codec noise rather than a change to the audio.
	//
	// The floor depends on the format. Re-encoding a lossy file always adds a
	// generation of codec noise even when the settings are identical - around
	// -30 dB for real music at 320 kbps - while a lossless round trip leaves
	// almost nothing. Reshaped audio sits far above either: dynamic-range
	// processing measures around -3 dB.
	nullResidualSafeLossyDB    = -25.0
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
	audit := &model.LoudnessAudit{MediaFileID: mediaFileID, AnalyzedAt: time.Now()}

	backup := ffmpeg.FindLoudnessBackup(backupFolder, libraryPath, trackPath)
	audit.HasBackup = backup != ""

	current, err := Measure(ctx, normalizer, trackPath, target)
	if err != nil {
		audit.Status = model.LoudnessStatusFailed
		audit.Verdict = model.LoudnessVerdictFailed
		audit.Error = err.Error()
		return audit
	}

	// The phase describes what still needs doing, judged from the file as it
	// stands now: reachable by a constant gain, or in need of a decision.
	plan := PlanFor(current.LUFS, current.TruePeak, target.IntegratedLUFS, target.TruePeak, tolerance)
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

	audit.Action = inferAction(original, current, gain)
	audit.Verdict = verdict(audit, original, current)
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

	plan := PlanFor(current.LUFS, current.TruePeak, target.IntegratedLUFS, target.TruePeak, tolerance)
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
}

// inferAction works out whether the change was a constant gain or something
// that reshaped the audio. Under a constant gain the true peak moves by exactly
// the gain and the loudness range is unchanged; both are mathematical
// properties, so a meaningful deviation in either means dynamics were touched.
func inferAction(before, after *Measurement, gain float64) string {
	expectedTP := before.TruePeak + gain
	tpDrift := math.Abs(after.TruePeak - expectedTP)
	lraDrift := math.Abs(after.LRA - before.LRA)
	if tpDrift <= pureGainToleranceDB && lraDrift <= pureGainToleranceDB {
		return model.LoudnessActionGain
	}
	return model.LoudnessActionLimited
}

// verdict grades the change. Format degradation outranks everything else: once
// the codec, bitrate, sample rate, bit depth, channel count, duration or cover
// art changed, the file is no longer the client's original regardless of how
// well the loudness landed.
func verdict(audit *model.LoudnessAudit, before, after *Measurement) string {
	if len(IntegrityIssues(before, after)) > 0 {
		return model.LoudnessVerdictReencoded
	}
	if audit.NullResidual != nil && *audit.NullResidual > NullResidualThreshold(after.Probe.Codec) {
		return model.LoudnessVerdictDynamicsChanged
	}
	if audit.Action == model.LoudnessActionLimited {
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
