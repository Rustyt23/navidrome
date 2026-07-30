package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/gcsync"
	"github.com/navidrome/navidrome/core/loudness"
	"github.com/navidrome/navidrome/core/silencetrim"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// loudnessRunOptions builds the engine options from the current configuration.
func loudnessRunOptions(mf *model.MediaFile) loudness.OptimizeOptions {
	options := conf.Server.Scanner.LoudnessNormalization
	return loudness.OptimizeOptions{
		Target: ffmpeg.LoudnessTarget{
			IntegratedLUFS: options.TargetLUFS,
			TruePeak:       options.TruePeak,
			LRA:            options.LRA,
		},
		Tolerance:    effectiveManualLoudnessTolerance(options.Tolerance),
		Backup:       options.Backup,
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
	opts := loudnessRunOptions(mf)
	if silenceTrimBlocksLoudness(mf, trackPath, opts.BackupFolder) {
		return loudness.OptimizeResult{}, fmt.Errorf(
			"restore the active start/end silence trim before LUFS processing",
		)
	}

	decision := loudness.DecisionPending
	if audit, err := ds.LoudnessAudit(ctx).Get(mf.ID); err == nil && audit != nil {
		decision = audit.Decision
	}

	res, err := loudness.Optimize(ctx, normalizer, trackPath, decision, opts)
	if err != nil {
		return res, err
	}

	// Refresh the audit record so the pages reflect what just happened. The run
	// already measured both sides of the change, so the record is built from
	// those rather than decoding the same files again.
	audit := loudness.AuditFromOptimize(ctx, normalizer, mf.ID, mf.LibraryPath, trackPath, res,
		opts.Target, opts.Tolerance, opts.BackupFolder)
	if err := ds.LoudnessAudit(ctx).Put(audit); err != nil {
		log.Warn(ctx, "Could not save loudness audit record", "id", mf.ID, err)
	}

	if res.Changed {
		updateSongLoudnessTag(ctx, ds.MediaFile(ctx), mf.ID, res.NewLUFS)

		uploadPath := trackPath
		if dest, err := copyTrackToSyncMP3Folder(mf.LibraryPath, trackPath); err != nil {
			log.Warn(ctx, "Could not copy LUFS-updated track to sync folder", "path", trackPath, err)
		} else if dest != "" {
			uploadPath = dest
		}
		if gcsync.IsEligibleLUFS(res.OldLUFS, res.NewLUFS, opts.Target.IntegratedLUFS) {
			gcsync.GetInstance().EnqueueMP3(uploadPath,
				fmt.Sprintf("LUFS optimised: %.2f -> %.2f (target %.2f)",
					res.OldLUFS, res.NewLUFS, opts.Target.IntegratedLUFS))
		}
	}
	return res, nil
}

// The durable generation record closes the short interval where the verified
// trimmed file is already installed but its database audit has not yet been
// committed. LUFS must respect either proof, otherwise it could rewrite the
// result and make its exact pre-trim restore ambiguous.
func silenceTrimBlocksLoudness(
	mf *model.MediaFile,
	trackPath string,
	backupFolder string,
) bool {
	if silencetrim.ResultFileUnchanged(mf.SilenceTrimAudit, trackPath) {
		return true
	}
	_, state, err := silencetrim.ReconcileGeneration(
		trackPath,
		mf.LibraryPath,
		backupFolder,
		mf.ID,
	)
	if state == silencetrim.GenerationStateResult {
		return true
	}
	if err != nil {
		// Fail closed during the journal-before-database window. An explicit
		// inactive audit (verified backup, no result generation) proves a prior
		// trim was restored and allows a later LUFS rewrite.
		return !silencetrim.CanIgnoreGenerationError(
			err,
			mf.SilenceTrimAudit,
			trackPath,
		)
	}
	return false
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
