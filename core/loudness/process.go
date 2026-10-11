package loudness

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/gcpsync"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// RunOptions builds the engine options from the current configuration.
// Whether originals are kept is read from the stored setting rather than the
// config file, since it is switchable from the UI.
func RunOptions(ctx context.Context, ds model.DataStore, mf *model.MediaFile) OptimizeOptions {
	options := conf.Server.Scanner.LoudnessNormalization
	return OptimizeOptions{
		Target: ffmpeg.LoudnessTarget{
			IntegratedLUFS: options.TargetLUFS,
			TruePeak:       options.TruePeak,
			LRA:            options.LRA,
		},
		Tolerance:    effectiveTolerance(options.Tolerance),
		Backup:       BackupEnabled(ctx, ds),
		LibraryPath:  mf.LibraryPath,
		BackupFolder: options.BackupFolder,
		MediaFileID:  mf.ID,
	}
}

// OptimizeTrack runs the engine on a single persisted track, updates its audit
// and library metadata, and stages changed MP3 audio for Sync. Callers must
// hold a file-work claim to exclude other analysis, restore, and optimisation jobs.
// The decision only matters for phase 2 tracks; phase 1 tracks are a plain
// constant gain and phase 0 tracks are left alone.
func OptimizeTrack(ctx context.Context, ds model.DataStore, normalizer ffmpeg.LoudnessNormalizer,
	mf *model.MediaFile) (OptimizeResult, error) {
	trackPath := filepath.FromSlash(mf.Path)
	if !filepath.IsAbs(trackPath) {
		trackPath = filepath.Join(mf.LibraryPath, trackPath)
	}
	trackPath = filepath.Clean(trackPath)
	opts := RunOptions(ctx, ds, mf)

	decision := DecisionPending
	if audit, err := ds.LoudnessAudit(ctx).Get(mf.ID); err == nil && audit != nil {
		decision = audit.Decision
	}

	res, err := Optimize(ctx, normalizer, trackPath, decision, opts)
	if err != nil {
		// Record the failure before giving up. Returning bare left the song with
		// no audit row at all: not analysed, not optimised, no verdict, and
		// picked up again by every later run to fail the same way. The only
		// evidence was a counter in the progress bar, which is gone the moment
		// the page is reloaded.
		//
		// Not written when the run is being cancelled: a track abandoned
		// mid-stop has not failed, and marking it failed would bury a good
		// reading under an error the next run would have to clear.
		if ctx.Err() == nil {
			recordCtx := context.WithoutCancel(ctx)
			failed := FailedAudit(mf.ID, trackPath, err)
			if putErr := ds.LoudnessAudit(recordCtx).Put(failed); putErr != nil {
				log.Warn(recordCtx, "Could not save loudness failure record", "id", mf.ID, putErr)
			}
		}
		return res, err
	}

	// Past this point the file on disk may already have been replaced, so the
	// record of it has to be written even if the run is being cancelled. A
	// cancelled context would fail the write and leave a rewritten file that
	// every page still describes as untouched - the one inconsistency this audit
	// exists to prevent. Measuring stays cancellable; only the recording does not.
	recordCtx := context.WithoutCancel(ctx)

	// Refresh the audit record so the pages reflect what just happened. The run
	// already measured both sides of the change, so the record is built from
	// those rather than decoding the same files again.
	audit := AuditFromOptimize(recordCtx, normalizer, mf.ID, mf.LibraryPath, trackPath, res,
		opts.Target, opts.Tolerance, opts.BackupFolder)
	if err := ds.LoudnessAudit(recordCtx).Put(audit); err != nil {
		log.Warn(recordCtx, "Could not save loudness audit record", "id", mf.ID, err)
	}

	// Loudness normalization rewrites the song in place, keeps its original in
	// the backup folder, records the audit - and then hands the result to
	// GCPSync so the bucket copy stops being the un-normalized one.
	//
	// The staged copy preserves the library layout and waits for the Sync
	// button unless automatic uploads have explicitly been enabled.
	if res.Changed {
		repo := ds.MediaFile(recordCtx)
		if err := repo.UpdateLoudnessTags(mf.ID, res.NewLUFS); err != nil {
			log.Warn(recordCtx, "Could not update song LUFS tag", "id", mf.ID, "lufs", res.NewLUFS, err)
		}

		// The row still describes the file as it was before the rewrite. The
		// scanner would normally correct that on its next pass, but it decides
		// a file is worth re-reading by comparing timestamps, and nothing here
		// moves the file's - so left alone the row stays wrong indefinitely.
		if res.AfterSet != nil && res.AfterSet.Probe != nil {
			p := res.AfterSet.Probe
			if err := repo.UpdateAudioProperties(mf.ID, model.AudioFileProperties{
				BitRate:    p.BitRate,
				SampleRate: p.SampleRate,
				BitDepth:   p.BitDepth,
				Channels:   p.Channels,
				Duration:   float32(p.Duration),
				Size:       p.Size,
			}); err != nil {
				log.Warn(recordCtx, "Could not update song properties after optimise", "id", mf.ID, err)
			}
		}

		// The one correction on this page that a listener can hear. The old
		// ReplayGain values describe a level this song no longer has, and they
		// are handed to every client and to the built-in player, which then
		// quietens an already-normalized track by the old amount - undoing the
		// work on every playback. Apply strips them from the file; this strips
		// them from the row.
		if err := repo.ClearReplayGain(mf.ID); err != nil {
			log.Warn(recordCtx, "Could not clear stale ReplayGain after optimise", "id", mf.ID, err)
		}

		// Staged rather than uploaded from where it lies: SyncFolder/mp3 is the
		// marker that says "processed", and it survives a restart that would
		// lose the in-memory upload queue.
		if strings.EqualFold(filepath.Ext(trackPath), ".mp3") &&
			res.AfterSet != nil && res.AfterSet.Probe != nil && strings.EqualFold(res.AfterSet.Probe.Codec, "mp3") {
			if err := gcpsync.StageSong(recordCtx, ds, mf, "LUFS normalized"); err != nil {
				return res, fmt.Errorf("audio normalized, but sync preparation failed: %w", err)
			}
		}
	}
	return res, nil
}

func effectiveTolerance(tolerance float64) float64 {
	if tolerance <= 0 {
		return conf.DefaultLoudnessNormalizationTolerance
	}
	return tolerance
}
