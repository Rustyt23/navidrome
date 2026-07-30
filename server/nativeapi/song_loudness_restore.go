package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/loudness"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/slice"
)

type songLoudnessRestoreResult struct {
	ID     string   `json:"id"`
	Status string   `json:"status"` // restored | skipped | failed
	LUFS   *float64 `json:"lufs,omitempty"`
	Error  string   `json:"error,omitempty"`
}

type songLoudnessRestoreResponse struct {
	IDs      []string                    `json:"ids"`
	Restored []string                    `json:"restored"`
	Skipped  []string                    `json:"skipped"`
	Failed   []string                    `json:"failed"`
	Results  []songLoudnessRestoreResult `json:"results"`
}

// restoreSongLoudness puts the stored originals back for the selected songs.
//
// This is the promise the backups exist to keep: whatever normalization did to
// a song can be undone on request, exactly, without re-encoding it back - the
// file the client started with is copied into place unchanged.
func (n *Router) restoreSongLoudness() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var payload songLoudnessPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ids := slice.Unique(payload.IDs)
		if len(ids) == 0 {
			http.Error(w, "ids are required", http.StatusBadRequest)
			return
		}
		// Restoring a track something else is in the middle of rewriting would
		// have the two racing for the same file, and whichever finished last
		// would win - which for a restore means it silently does not stick.
		release, busy := claimLoudnessFileWork(&restoreLoudnessRunning)
		if busy != "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(songLoudnessRestoreResponse{
				IDs: ids, Failed: ids,
				Results: []songLoudnessRestoreResult{{
					Status: "failed",
					Error:  busy + " is in progress - wait for it to finish before restoring",
				}},
			})
			return
		}
		defer release()

		response := restoreSelectedSongs(ctx, n.ds, ids)
		status := http.StatusOK
		if len(response.Restored) == 0 && len(response.Failed) > 0 {
			status = http.StatusInternalServerError
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Error(ctx, "Error sending song loudness restore response", err)
		}
	}
}

func restoreSelectedSongs(ctx context.Context, ds model.DataStore, ids []string) songLoudnessRestoreResponse {
	options := conf.Server.Scanner.LoudnessNormalization
	target := ffmpeg.LoudnessTarget{
		IntegratedLUFS: options.TargetLUFS,
		TruePeak:       options.TruePeak,
		LRA:            options.LRA,
	}
	tolerance := effectiveManualLoudnessTolerance(options.Tolerance)
	normalizer := ffmpeg.NewLoudnessNormalizer()
	repo := ds.MediaFile(ctx)
	audits := ds.LoudnessAudit(ctx)

	response := songLoudnessRestoreResponse{IDs: ids}
	for _, id := range ids {
		result := songLoudnessRestoreResult{ID: id}
		record := func(bucket *[]string, status, msg string) {
			result.Status = status
			result.Error = msg
			*bucket = append(*bucket, id)
			response.Results = append(response.Results, result)
		}

		mf, err := repo.Get(id)
		if err != nil {
			record(&response.Failed, "failed", err.Error())
			continue
		}
		if mf.Path == "" || mf.LibraryPath == "" {
			record(&response.Failed, "failed", "song file is missing")
			continue
		}
		trackPath := absoluteSelectedMediaPath(mf.LibraryPath, mf.Path)
		if silenceTrimBlocksLoudness(mf, trackPath, options.BackupFolder) {
			record(
				&response.Failed,
				"failed",
				"restore the active start/end silence trim before restoring LUFS",
			)
			continue
		}

		// Reported as skipped rather than failed: a song that was never
		// rewritten has nothing to restore, and selecting a whole page should
		// not look like an error.
		if ffmpeg.FindLoudnessBackup(options.BackupFolder, mf.LibraryPath, id, trackPath) == "" {
			record(&response.Skipped, "skipped", "no stored original - this song was never rewritten")
			continue
		}

		previous, err := audits.Get(id)
		if err != nil {
			log.Warn(ctx, "Could not read loudness audit before restoring", "id", id, err)
			previous = nil
		}

		res, err := loudness.Restore(ctx, normalizer, id, mf.LibraryPath, trackPath,
			previous, target, tolerance, options.BackupFolder)
		if err != nil {
			log.Warn(ctx, "Could not restore original song", "id", id, "path", trackPath, err)
			record(&response.Failed, "failed", err.Error())
			continue
		}

		if err := audits.Put(res.Audit); err != nil {
			log.Warn(ctx, "Restored the song but could not update its audit record", "id", id, err)
		}
		// Put deliberately preserves the client's decision, but a decision that
		// has just been undone must not be reapplied by the next phase 2 run.
		if err := audits.SetDecision(id, loudness.DecisionPending); err != nil {
			log.Warn(ctx, "Restored the song but could not clear its decision", "id", id, err)
		}
		updateSongLoudnessTag(ctx, repo, id, res.LUFS)
		if _, err := copyTrackToSyncMP3Folder(mf.LibraryPath, trackPath); err != nil {
			log.Warn(ctx, "Restored the song but could not update the sync folder copy", "path", trackPath, err)
		}

		log.Info(ctx, "Restored original song", "id", id, "path", trackPath, "lufs", res.LUFS)
		result.LUFS = &res.LUFS
		record(&response.Restored, "restored", "")
	}
	return response
}
