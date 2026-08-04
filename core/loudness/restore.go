package loudness

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/filelock"
)

// RestoreResult reports what happened to one restored track.
type RestoreResult struct {
	// LUFS is the loudness the track is back to, for the stored tag.
	LUFS float64
	// Audit is the record rewritten to describe the file as it now stands.
	Audit *model.LoudnessAudit
}

// Restore puts the client's original back and rewrites the audit record to
// match.
//
// Normally nothing needs measuring afterwards. The audit's "before" snapshot is
// the measurement of the very file just copied back, so it is still exactly
// right - and it was taken before the track was ever rewritten, which is the
// one measurement that cannot be reproduced if the backup is later lost.
//
// It is only reusable when it really came from the stored original. A record
// with no snapshot, or one whose snapshot was taken from the file while it was
// normalized, is rebuilt by measuring the restored file instead.
func Restore(ctx context.Context, normalizer ffmpeg.LoudnessNormalizer, mediaFileID, libraryPath, trackPath string,
	previous *model.LoudnessAudit, target ffmpeg.LoudnessTarget, tolerance float64,
	backupFolder string) (RestoreResult, error) {
	unlock := filelock.Lock(trackPath)
	defer unlock()

	// Nothing is overwritten on the strength of an unchecked file. A restore
	// destroys the working song first and would only discover a corrupt or
	// mismatched backup afterwards, by which point there is nothing to go back
	// to.
	backup := ffmpeg.FindLoudnessBackup(backupFolder, libraryPath, mediaFileID, trackPath)
	if backup == "" {
		return RestoreResult{}, fmt.Errorf("no stored original for %s", trackPath)
	}
	if err := verifyStoredOriginal(ctx, backup, previous); err != nil {
		return RestoreResult{}, err
	}

	if err := ffmpeg.RestoreOriginal(trackPath, libraryPath, backupFolder, mediaFileID); err != nil {
		return RestoreResult{}, err
	}

	// The stored snapshot may be carried over only when it is a measurement of
	// the stored original - which is what HasBackup records. A snapshot taken
	// from the file while it was normalized describes the wrong audio entirely,
	// and carrying it forward would report the track at the loudness it had
	// before the restore rather than the one it has now.
	if previous != nil && previous.LufsBefore != nil && previous.HasBackup {
		audit := revertedAudit(previous, target, tolerance)
		return RestoreResult{LUFS: *audit.LufsBefore, Audit: audit}, nil
	}

	// Nothing trustworthy to rebuild the record from, so measure what is now on
	// disk. One pass, not a full audit: the file was just copied from the stored
	// original, so comparing the two would only measure them against themselves.
	measured, err := Measure(ctx, normalizer, trackPath, target)
	if err != nil {
		return RestoreResult{}, fmt.Errorf("restored, but could not measure the result: %w", err)
	}
	audit := analyzedAudit(mediaFileID, measured, target, tolerance)
	// The original is still stored, so the track can be restored again.
	audit.HasBackup = true
	return RestoreResult{LUFS: *audit.LufsBefore, Audit: audit}, nil
}

// verifyStoredOriginal refuses a backup that cannot be read as audio, or that
// is not the recording this song's record describes.
//
// The second check is what catches a backup left behind by a different song.
// Backups stored by an earlier version were keyed by library path, so a song
// arriving where a deleted one used to live inherits its backup - and restoring
// would replace the client's song with someone else's recording. The stored
// snapshot says exactly what the original was; anything that disagrees is not
// it.
func verifyStoredOriginal(ctx context.Context, backupPath string, previous *model.LoudnessAudit) error {
	probe, err := ffmpeg.ProbeFile(ctx, backupPath)
	if err != nil {
		return fmt.Errorf("the stored original could not be read as audio, so nothing was changed: %w", err)
	}
	if previous == nil {
		// No record to check against. The file is readable audio, which is as
		// much as can be established.
		return nil
	}
	mismatch := func(what string, want, got any) error {
		return fmt.Errorf("the stored original does not match this song (%s %v, expected %v), so nothing was changed",
			what, got, want)
	}
	if previous.SizeBefore > 0 && probe.Size > 0 && previous.SizeBefore != probe.Size {
		return mismatch("size", previous.SizeBefore, probe.Size)
	}
	if previous.DurationBefore > 0 && probe.Duration > 0 &&
		math.Abs(previous.DurationBefore-probe.Duration) > durationMatchToleranceSec {
		return mismatch("duration", previous.DurationBefore, probe.Duration)
	}
	if previous.CodecBefore != "" && probe.Codec != "" && previous.CodecBefore != probe.Codec {
		return mismatch("format", previous.CodecBefore, probe.Codec)
	}
	return nil
}

// durationMatchToleranceSec: the same file probed twice reports the same length,
// so only rounding belongs in here.
const durationMatchToleranceSec = 0.05

// revertedAudit rebuilds the record for a track put back to its original.
//
// The before snapshot is carried over untouched - it describes the file that is
// now on disk. Everything that described the rewrite is dropped: the after
// snapshot, the gain, the null test and the verdict all referred to audio that
// no longer exists, and leaving any of it in place would misreport the track as
// still processed.
func revertedAudit(previous *model.LoudnessAudit, target ffmpeg.LoudnessTarget, tolerance float64) *model.LoudnessAudit {
	audit := &model.LoudnessAudit{
		MediaFileID:      previous.MediaFileID,
		Status:           model.LoudnessStatusAnalyzed,
		LufsBefore:       previous.LufsBefore,
		TpBefore:         previous.TpBefore,
		LraBefore:        previous.LraBefore,
		CodecBefore:      previous.CodecBefore,
		BitrateBefore:    previous.BitrateBefore,
		SampleRateBefore: previous.SampleRateBefore,
		BitDepthBefore:   previous.BitDepthBefore,
		ChannelsBefore:   previous.ChannelsBefore,
		DurationBefore:   previous.DurationBefore,
		SizeBefore:       previous.SizeBefore,
		ArtBefore:        previous.ArtBefore,
		// The original is kept, so the track can be restored again.
		HasBackup:  true,
		AnalyzedAt: time.Now(),
	}

	// The track is back where it started, so what remains to be done to it is
	// whatever was needed before anything was applied.
	peak := 0.0
	if audit.TpBefore != nil {
		peak = *audit.TpBefore
	}
	plan := PlanFor(*audit.LufsBefore, peak, target.IntegratedLUFS, target.TruePeak, tolerance, previous.BitrateBefore)
	audit.Phase = plan.Phase
	if plan.Phase == PhaseDone {
		audit.Verdict = model.LoudnessVerdictUntouched
		audit.Action = model.LoudnessActionSkipped
	}
	return audit
}
