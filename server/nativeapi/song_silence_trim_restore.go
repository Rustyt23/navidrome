package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/silencetrim"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/utils/filelock"
)

type silenceRestoreResponse struct {
	IDs      []string `json:"ids"`
	Restored []string `json:"restored"`
	Skipped  []string `json:"skipped"`
	Failed   []string `json:"failed"`
}

func (n *Router) restoreSongSilenceTrim() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload silenceTrimPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload.IDs = uniqueStrings(payload.IDs)
		if len(payload.IDs) == 0 {
			http.Error(w, "ids are required", http.StatusBadRequest)
			return
		}
		release, busy := claimLoudnessFileWork(&silenceRestoreRunning)
		if busy != "" {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"message": busy + " is in progress",
			})
			return
		}
		defer release()

		ctx := r.Context()
		options := conf.Server.Scanner.LoudnessNormalization
		response := silenceRestoreResponse{IDs: payload.IDs}

		for _, id := range payload.IDs {
			status, err := n.restoreOneSilenceTrim(
				ctx,
				id,
				options.BackupFolder,
			)
			switch {
			case err != nil:
				log.Warn(ctx, "Could not restore pre-trim song", "id", id, err)
				response.Failed = append(response.Failed, id)
			case status == "restored":
				response.Restored = append(response.Restored, id)
			default:
				response.Skipped = append(response.Skipped, id)
			}
		}

		status := http.StatusOK
		if len(response.Restored) == 0 && len(response.Failed) > 0 {
			status = http.StatusInternalServerError
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(response)
	}
}

func (n *Router) restoreOneSilenceTrim(
	ctx context.Context,
	id string,
	backupFolder string,
) (string, error) {
	files := n.ds.MediaFile(ctx)
	audits := n.ds.SilenceTrimAudit(ctx)
	mf, err := files.Get(id)
	if err != nil {
		return "", err
	}
	path := absoluteSelectedMediaPath(mf.LibraryPath, mf.Path)
	unlock := filelock.Lock(path)
	defer unlock()

	previous := mf.SilenceTrimAudit
	if previous == nil {
		previous, _ = audits.Get(id)
	}
	generation, generationState, generationErr := silencetrim.ReconcileGeneration(
		path,
		mf.LibraryPath,
		backupFolder,
		mf.ID,
	)
	if generationErr != nil &&
		!silencetrim.CanIgnoreGenerationError(generationErr, previous, path) {
		return "", generationErr
	}
	if generationState == silencetrim.GenerationStateResult {
		previous = generation
	}
	if generationState == silencetrim.GenerationStateSource {
		restored := silencetrim.AnalyzeRecoveredSource(ctx, mf, path, generation)
		restored.Reason = "Exact pre-trim file was already restored; analysis reconciled from the durable generation record"
		if err := files.UpdateFileSizeAndDuration(
			id,
			restored.SizeBefore,
			restored.DurationBefore,
		); err != nil {
			return "", err
		}
		if err := audits.Put(restored); err != nil {
			return "", err
		}
		if _, err := copyTrackToSyncMP3Folder(mf.LibraryPath, path); err != nil {
			log.Warn(ctx, "Reconciled silence restore but could not update sync copy", "path", path, err)
		}
		return "restored", nil
	}
	if previous == nil || previous.ResultSHA256 == "" {
		return "skipped", nil
	}

	stat, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".silence-restore-rollback-*")
	if err != nil {
		return "", err
	}
	rollbackPath := temp.Name()
	if err := temp.Close(); err != nil {
		_ = os.Remove(rollbackPath)
		return "", err
	}
	_ = os.Remove(rollbackPath)
	defer os.Remove(rollbackPath)
	if err := ffmpeg.CopyFileAtomic(path, rollbackPath, stat.Mode()); err != nil {
		return "", err
	}
	rollbackHash, err := ffmpeg.FileSHA256(rollbackPath)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(rollbackHash, previous.ResultSHA256) {
		return "", fmt.Errorf("restore refused because the rollback snapshot is not the verified trim result")
	}

	audit, restoreErr := silencetrim.RestoreLocked(
		ctx,
		mf,
		path,
		previous,
		backupFolder,
	)
	if restoreErr != nil {
		rolledBack, rollbackErr := restoreSilenceSnapshotIfCurrent(
			path,
			rollbackPath,
			stat.Mode(),
			previous.BackupSHA256,
			rollbackHash,
		)
		if rollbackErr != nil {
			log.Error(
				ctx,
				"Could not conditionally roll back failed silence restore",
				"id", id,
				"path", path,
				"restoreError", restoreErr,
				"rollbackError", rollbackErr,
			)
		}
		if !rolledBack {
			log.Warn(ctx, "Failed silence restore left unknown/newer bytes untouched", "id", id, "path", path)
		}
		return "", restoreErr
	}

	persistErr := audits.Put(audit)
	if persistErr == nil {
		persistErr = files.UpdateFileSizeAndDuration(id, audit.SizeBefore, audit.DurationBefore)
	}
	if persistErr != nil {
		rolledBack, rollbackErr := restoreSilenceSnapshotIfCurrent(
			path,
			rollbackPath,
			stat.Mode(),
			audit.SourceSHA256,
			rollbackHash,
		)
		if rollbackErr != nil {
			log.Error(
				ctx,
				"Could not conditionally roll back silence restore after database failure",
				"id", id,
				"path", path,
				"persistError", persistErr,
				"rollbackError", rollbackErr,
			)
		}
		if rolledBack {
			if err := audits.Put(previous); err != nil {
				log.Error(ctx, "Could not restore prior silence audit after rollback", "id", id, err)
			}
			if err := files.UpdateFileSizeAndDuration(
				id,
				previous.SizeAfter,
				previous.DurationAfter,
			); err != nil {
				log.Error(ctx, "Could not restore prior media metadata after rollback", "id", id, err)
			}
		}
		return "", persistErr
	}

	if _, err := copyTrackToSyncMP3Folder(mf.LibraryPath, path); err != nil {
		log.Warn(ctx, "Restored song but could not update sync copy", "path", path, err)
	}
	return "restored", nil
}

func restoreSilenceSnapshotIfCurrent(
	trackPath string,
	rollbackPath string,
	mode os.FileMode,
	expectedCurrentSHA256 string,
	rollbackSHA256 string,
) (bool, error) {
	currentHash, err := ffmpeg.FileSHA256(trackPath)
	if err != nil {
		return false, err
	}
	if strings.EqualFold(currentHash, rollbackSHA256) {
		return true, nil
	}
	if expectedCurrentSHA256 == "" ||
		!strings.EqualFold(currentHash, expectedCurrentSHA256) {
		return false, fmt.Errorf("rollback refused because the current song has an unrecognized generation")
	}
	if err := ffmpeg.CopyFileAtomic(rollbackPath, trackPath, mode); err != nil {
		return false, err
	}
	restoredHash, err := ffmpeg.FileSHA256(trackPath)
	if err != nil {
		return false, err
	}
	if !strings.EqualFold(restoredHash, rollbackSHA256) {
		return false, fmt.Errorf("rollback snapshot failed its SHA-256 check")
	}
	return true, nil
}
