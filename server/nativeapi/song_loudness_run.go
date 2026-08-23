package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/loudness"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// loudnessRunOptions builds the engine options from the current configuration.
// Whether originals are kept is read from the stored setting rather than the
// config file, since it is switchable from the UI.
func loudnessRunOptions(ctx context.Context, ds model.DataStore, mf *model.MediaFile) loudness.OptimizeOptions {
	options := conf.Server.Scanner.LoudnessNormalization
	return loudness.OptimizeOptions{
		Target: ffmpeg.LoudnessTarget{
			IntegratedLUFS: options.TargetLUFS,
			TruePeak:       options.TruePeak,
			LRA:            options.LRA,
		},
		Tolerance:    effectiveManualLoudnessTolerance(options.Tolerance),
		Backup:       loudness.BackupEnabled(ctx, ds),
		LibraryPath:  mf.LibraryPath,
		BackupFolder: options.BackupFolder,
		MediaFileID:  mf.ID,
	}
}

// optimizeOneTrack runs the engine on a single track and records the audit.
// The decision only matters for phase 2 tracks; phase 1 tracks are a plain
// constant gain and phase 0 tracks are left alone.
func optimizeOneTrack(ctx context.Context, ds model.DataStore, normalizer ffmpeg.LoudnessNormalizer,
	mf *model.MediaFile) (loudness.OptimizeResult, error) {
	trackPath := absoluteSelectedMediaPath(mf.LibraryPath, mf.Path)
	opts := loudnessRunOptions(ctx, ds, mf)

	decision := loudness.DecisionPending
	if audit, err := ds.LoudnessAudit(ctx).Get(mf.ID); err == nil && audit != nil {
		decision = audit.Decision
	}

	res, err := loudness.Optimize(ctx, normalizer, trackPath, decision, opts)
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
			failed := loudness.FailedAudit(mf.ID, trackPath, err)
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
	audit := loudness.AuditFromOptimize(recordCtx, normalizer, mf.ID, mf.LibraryPath, trackPath, res,
		opts.Target, opts.Tolerance, opts.BackupFolder)
	if err := ds.LoudnessAudit(recordCtx).Put(audit); err != nil {
		log.Warn(recordCtx, "Could not save loudness audit record", "id", mf.ID, err)
	}

	// Nothing outside the library is touched. Loudness normalization rewrites the
	// song in place, keeps its original in the backup folder and records the
	// audit - and stops there.
	//
	// It used to also copy the result into SyncFolder/mp3 and hand that copy to
	// gcsync, which uploaded it and then moved it into the music folder under its
	// basename alone - leaving a flattened duplicate of every normalized song at
	// the library root, which the next scan indexed as a new track. The sync
	// folder is an inbox for files arriving from outside; these files are already
	// in the library, so there was nothing there to ingest.
	if res.Changed {
		repo := ds.MediaFile(recordCtx)
		updateSongLoudnessTag(recordCtx, repo, mf.ID, res.NewLUFS)

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
	}
	return res, nil
}

type loudnessDecisionPayload struct {
	IDs      []string `json:"ids"`
	Decision string   `json:"decision"`
}

func validLoudnessDecision(d string) bool {
	switch d {
	case loudness.DecisionPending, loudness.DecisionLimit, loudness.DecisionCeiling, loudness.DecisionSkip:
		return true
	}
	return false
}

// setLoudnessDecision records the client's choice for phase 2 tracks. It only
// stores the intent; nothing is applied until a phase 2 run is started.
func (n *Router) setLoudnessDecision() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var payload loudnessDecisionPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload.Decision = strings.TrimSpace(payload.Decision)
		if !validLoudnessDecision(payload.Decision) {
			http.Error(w, "invalid decision", http.StatusBadRequest)
			return
		}
		if len(payload.IDs) == 0 {
			http.Error(w, "ids are required", http.StatusBadRequest)
			return
		}

		repo := n.ds.LoudnessAudit(ctx)
		updated := 0
		for _, id := range payload.IDs {
			if err := repo.SetDecision(id, payload.Decision); err != nil {
				log.Warn(ctx, "Could not save loudness decision", "id", id, err)
				continue
			}
			updated++
		}
		log.Info(ctx, "Loudness decisions saved", "decision", payload.Decision, "tracks", updated)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"updated": updated, "decision": payload.Decision})
	}
}
