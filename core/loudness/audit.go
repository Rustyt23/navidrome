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
	// Null-test floors: the energy below which what is left over after undoing
	// the gain is the cost of rewriting the file rather than a change to the
	// audio.
	//
	// Calibrated by measurement, not judgement. Rewriting a lossy file adds a
	// generation of codec noise even at identical settings; across real music
	// that leftover measures -39 to -47 dB, and pure synthetic tones - which an
	// encoder reproduces almost exactly - reach -90. Running the same track
	// through an actual limiter or compressor instead leaves -10 to -15. The two
	// outcomes are 25 dB apart at their closest, so the middle figure separates
	// them with room on both sides.
	//
	// They differ by bitrate because the noise floor does - see
	// NullResidualThreshold. A lossless round trip has no codec noise to account
	// for and should null almost perfectly, so anything above -60 there means
	// the audio itself was altered.
	nullResidualNoisyLossyDB   = -25.0 // 160k and below
	nullResidualSafeLossyDB    = -30.0 // 192k to 256k - the measured middle
	nullResidualCleanLossyDB   = -35.0 // above 256k
	nullResidualSafeLosslessDB = -60.0

	// pureGainToleranceDB: under a constant gain the true peak moves by exactly
	// the applied gain and the loudness range does not move at all. Allow this
	// much slack for encoder/resampler noise before calling it non-linear.
	pureGainToleranceDB = 0.5

	// corroboratedShaveDB is how far the peak must fall below what the gain
	// predicts to count as limiting when the null test already says a great deal
	// of the file changed.
	//
	// Lower than pureGainToleranceDB on purpose: evidence needs to be stronger
	// when it stands alone than when something independent agrees with it. This
	// is the level at which a peak stops tracking its gain for any reason other
	// than measurement noise, so anything above it is a real movement - too
	// small to convict on by itself, enough to convict on with a witness.
	corroboratedShaveDB = 0.15
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
		switch plan.Phase {
		case PhaseDone:
			audit.Verdict = model.LoudnessVerdictUntouched
			audit.Action = model.LoudnessActionSkipped
		case PhaseCloseEnough:
			// The file was never opened, so nothing can be said about whether it
			// was harmed - no verdict. What happened to it is that it was left
			// alone on purpose, which the phase records and the outcome column
			// reads. "No change needed" would be the wrong claim here: a change
			// was needed, and was judged not worth what it would cost.
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
//
// When backups are turned off there is no original to read and never will be,
// so the run's own measurements are the only record this track will ever have.
// They are used directly. Falling through to the no-backup path instead - which
// takes the file on disk as its own "before" - would record a track that had
// just been rewritten as never touched, with the rewritten loudness stored as
// its original: a claim that is wrong, unfalsifiable once the run is over, and
// applied to every song in the library.
func AuditFromOptimize(ctx context.Context, normalizer ffmpeg.LoudnessNormalizer,
	mediaFileID, libraryPath, trackPath string, res OptimizeResult,
	target ffmpeg.LoudnessTarget, tolerance float64, backupFolder string) *model.LoudnessAudit {
	// Whichever side the run measured last is the file as it now stands on disk.
	current := res.BeforeSet
	if res.Changed && res.AfterSet != nil {
		current = res.AfterSet
	}

	// A run that stored the original, or one that was never going to.
	measuredThisRun := res.BackupCreated || res.BackupSkipped

	if !res.Changed || !measuredThisRun || res.BeforeSet == nil || res.AfterSet == nil {
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

// MeasureOriginal records what a song's original was: one loudness pass plus a
// container probe.
//
// It skips the two expensive parts of a full audit - re-measuring the stored
// original and running the null test - because neither says anything about a
// file that has not been rewritten.
//
// A file that HAS been rewritten is a different matter, and is why the stored
// original is looked for first. Measuring the file on disk there records the
// normalized loudness as the song's original: wrong on the page, wrong in the
// tag, and wrong in a way that survives. Worse, a restore trusts that snapshot
// to describe the file it puts back, so the track would be reported at the
// loudness it had while normalized after being returned to one 20 dB away.
// Those songs get the full audit instead, which reads the original from the
// backup where it actually is.
func MeasureOriginal(ctx context.Context, normalizer ffmpeg.LoudnessNormalizer,
	mediaFileID, libraryPath, trackPath string, target ffmpeg.LoudnessTarget, tolerance float64,
	backupFolder string) *model.LoudnessAudit {
	if ffmpeg.FindLoudnessBackup(backupFolder, libraryPath, mediaFileID, trackPath) != "" {
		return Audit(ctx, normalizer, mediaFileID, libraryPath, trackPath, target, tolerance, backupFolder)
	}

	current, err := Measure(ctx, normalizer, trackPath, target)
	if err != nil {
		return FailedAudit(mediaFileID, err)
	}
	return analyzedAudit(mediaFileID, current, target, tolerance)
}

// FailedAudit is the record for a song the engine could not get through at all.
//
// It exists so that a hard failure leaves a mark. A run that gave up without
// writing anything left the song looking untouched: not analysed, not
// optimised, no error, no count - and picked up again by the next run, to fail
// the same way for ever. Ten songs in a production library sat in exactly that
// state, invisible, because ffmpeg could not copy their cover art.
//
// The error text is kept verbatim. It is what someone debugging needs, and the
// UI translates it into something a client can read rather than replacing it.
func FailedAudit(mediaFileID string, err error) *model.LoudnessAudit {
	audit := &model.LoudnessAudit{
		MediaFileID: mediaFileID,
		AnalyzedAt:  time.Now(),
		Status:      model.LoudnessStatusFailed,
		Verdict:     model.LoudnessVerdictFailed,
	}
	if err != nil {
		audit.Error = err.Error()
	}
	return audit
}

// analyzedAudit is the record for a file that was measured and not rewritten:
// its current state is its own "before" snapshot, and there is no "after"
// because nothing was applied.
func analyzedAudit(mediaFileID string, m *Measurement, target ffmpeg.LoudnessTarget,
	tolerance float64) *model.LoudnessAudit {
	audit := &model.LoudnessAudit{MediaFileID: mediaFileID, AnalyzedAt: time.Now()}
	recordBefore(audit, m)
	audit.Status = model.LoudnessStatusAnalyzed

	plan := PlanFor(m.LUFS, m.TruePeak, target.IntegratedLUFS, target.TruePeak, tolerance, m.Probe.BitRate)
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
// Only direct evidence answers this. The loudness range closing means the gap
// between the loud and quiet passages narrowed, which a level change cannot do.
// A peak LOWER than the gain predicts means something pushed it down, which is
// what limiting does. Between them they say what happened to the music.
//
// The peak is read in one direction only. A peak HIGHER than predicted cannot
// be limiting - a limiter never raises anything - it is the encoder rebuilding
// the waveform imperfectly, which low-bitrate sources do by up to 0.8 dB.
// Treating the two alike reported untouched tracks as reshaped.
//
// The null test corroborates rather than decides. On its own it cannot say the
// dynamics were touched: a rewrite that cost more than its format should and a
// rewrite that squashed the peaks both leave a large leftover, and nothing in
// that one number separates them. Deciding on it alone put "peaks trimmed"
// against tracks whose peaks nothing had touched.
//
// It is far from useless though, and dropping it entirely loses real detections.
// Window to the Soul shaved 0.44 dB off its peak and closed its range by 0.1 -
// both under the thresholds above, both dismissed - while its null test sat at
// -10.9, nineteen decibels above the floor. Neither direct signal was large
// enough to trust on its own; together with the null test agreeing, they are.
//
// So a movement too small to stand alone is enough when the null test says a
// great deal of the file changed. What is not enough is the null test with no
// movement at all: a peak that tracked the gain exactly was not limited, however
// expensive the rewrite turned out to be.
func inferAction(audit *model.LoudnessAudit, before, after *Measurement, gain float64) string {
	lraMoved := math.Abs(after.LRA - before.LRA)
	shavedBy := (before.TruePeak + gain) - after.TruePeak

	if lraMoved > pureGainToleranceDB || shavedBy > pureGainToleranceDB {
		return model.LoudnessActionLimited
	}
	if rewriteWasCostly(audit, after) && shavedBy > corroboratedShaveDB {
		return model.LoudnessActionLimited
	}
	return model.LoudnessActionGain
}

// rewriteWasCostly reports whether the null test found more difference from the
// original than re-encoding this format accounts for.
func rewriteWasCostly(audit *model.LoudnessAudit, after *Measurement) bool {
	return audit.NullResidual != nil &&
		*audit.NullResidual > NullResidualThreshold(after.Probe.Codec, after.Probe.BitRate)
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
	// Whether the audio itself was reshaped is settled by inferAction, from the
	// loudness range and the true peak - the two signals that say what happened
	// to the music rather than how much of the file moved.
	if audit.Action == model.LoudnessActionLimited {
		return model.LoudnessVerdictDynamicsChanged
	}
	if math.Abs(after.LRA-before.LRA) > pureGainToleranceDB {
		return model.LoudnessVerdictDynamicsChanged
	}
	// Nothing reshaped the audio and the format survived, yet more of the file
	// differs from the original than re-encoding should account for. That is
	// worth saying, and worth saying as itself: the file cost more to rewrite
	// than it should have, which is not the same claim as the peaks having been
	// trimmed.
	if rewriteWasCostly(audit, after) {
		return model.LoudnessVerdictRewriteCostly
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
//
// It scales with the bitrate because the noise floor does. Every other
// codec-sensitive figure in this system already does the same - what a rewrite
// costs in loudness, how far the peak springs back - because a 128k file and a
// 320k file are not the same problem. One threshold for both had to sit low
// enough not to fail the noisy end, which left the clean end barely checked, or
// high enough to check the clean end, which failed the noisy one. The noisy end
// is the client's worst material, so that is where the false alarms landed.
//
//	PROVISIONAL. The lossless figure and the mid-range are measured; the values
//	at the two lossy extremes are interpolated from them and from the gain-only
//	rewrites recorded in verdict_test.go (-39.0 and -41.6). Replace them with
//	real numbers using cmd/measure-null-floor, which rewrites a sample of the
//	library at each bitrate with no gain at all and reports the floor it finds.
func NullResidualThreshold(codec string, bitRate int) float64 {
	if !IsLossy(codec) {
		return nullResidualSafeLosslessDB
	}
	switch {
	case bitRate <= 0:
		// Unknown bitrate: assume the noisiest case rather than fail a file for
		// noise its format was always going to produce.
		return nullResidualNoisyLossyDB
	case bitRate <= 160:
		return nullResidualNoisyLossyDB
	case bitRate <= 256:
		return nullResidualSafeLossyDB
	default:
		return nullResidualCleanLossyDB
	}
}

// bitrateLost reports whether the rewrite cost the file data per second.
//
// Only a lossy file can lose any. For a lossless one the bitrate is how well
// the audio happened to compress, not how much of it survived: turning a track
// down leaves smaller numbers to encode, so it compresses better and the figure
// drops with nothing lost at all. Measured on a real FLAC, a 4 dB reduction took
// it from 620 to 592 kbps - which, read as damage, rejected the output and
// refused the track for ever. Since reaching -12.6 means turning most masters
// down, that was every lossless file in the library.
//
// A lossy file that comes back at the format's ceiling is also let through. The
// encoder is asked for at least what the source had, so it can only land lower
// when the source was probed above what the format can hold - which happens when
// the stream reports no bitrate and the container's figure, cover art and tags
// included, stands in for it.
func bitrateLost(before, after *ffmpeg.FileProbe) bool {
	if after.BitRate >= before.BitRate {
		return false
	}
	if !IsLossy(before.Codec) {
		return false
	}
	if max := ffmpeg.MaxBitrateFor(after.Codec); max > 0 && after.BitRate >= max {
		return false
	}
	return true
}

// IntegrityIssues lists everything that changed which should not have.
func IntegrityIssues(before, after *Measurement) []string {
	var issues []string
	b, a := before.Probe, after.Probe
	if b.Codec != a.Codec {
		issues = append(issues, fmt.Sprintf("codec %s->%s", b.Codec, a.Codec))
	}
	if bitrateLost(b, a) {
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
