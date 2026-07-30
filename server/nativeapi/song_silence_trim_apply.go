package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/silencetrim"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/slice"
)

type silenceApplyJob struct {
	stopping  atomic.Bool
	startedAt atomic.Int64
	total     atomic.Int64
	processed atomic.Int64
	trimmed   atomic.Int64
	skipped   atomic.Int64
	failed    atomic.Int64
	stopMu    sync.Mutex
	stop      chan struct{}
}

var silenceApply silenceApplyJob

type silenceApplyStatus struct {
	Running   bool   `json:"running"`
	Stopping  bool   `json:"stopping"`
	StartedAt string `json:"startedAt,omitempty"`
	Total     int64  `json:"total"`
	Processed int64  `json:"processed"`
	Trimmed   int64  `json:"trimmed"`
	Skipped   int64  `json:"skipped"`
	Failed    int64  `json:"failed"`
	Message   string `json:"message,omitempty"`
}

func currentSilenceApplyStatus(message string) silenceApplyStatus {
	status := silenceApplyStatus{
		Running:   silenceApplyRunning.Load(),
		Stopping:  silenceApply.stopping.Load(),
		Total:     silenceApply.total.Load(),
		Processed: silenceApply.processed.Load(),
		Trimmed:   silenceApply.trimmed.Load(),
		Skipped:   silenceApply.skipped.Load(),
		Failed:    silenceApply.failed.Load(),
		Message:   message,
	}
	if started := silenceApply.startedAt.Load(); started > 0 {
		status.StartedAt = time.Unix(started, 0).Format(time.RFC3339)
	}
	return status
}

func (n *Router) startSilenceTrimApply() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload silenceTrimPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload.IDs = uniqueStrings(payload.IDs)
		if !payload.All && len(payload.IDs) == 0 {
			http.Error(w, "ids are required (or set all=true)", http.StatusBadRequest)
			return
		}
		if payload.All && len(payload.LibraryIDs) == 0 {
			http.Error(w, "libraryIds are required when all=true", http.StatusBadRequest)
			return
		}
		release, busy := claimLoudnessFileWork(&silenceApplyRunning)
		if busy != "" {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(currentSilenceApplyStatus(
				busy + " is in progress",
			))
			return
		}
		silenceApply.stopping.Store(false)
		silenceApply.startedAt.Store(time.Now().Unix())
		silenceApply.total.Store(0)
		silenceApply.processed.Store(0)
		silenceApply.trimmed.Store(0)
		silenceApply.skipped.Store(0)
		silenceApply.failed.Store(0)
		silenceApply.stopMu.Lock()
		silenceApply.stop = make(chan struct{})
		silenceApply.stopMu.Unlock()

		go n.runSilenceTrimApply(context.WithoutCancel(r.Context()), payload, release)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(currentSilenceApplyStatus(
			"Applying safe and approved edge trims",
		))
	}
}

func (n *Router) runSilenceTrimApply(
	ctx context.Context,
	payload silenceTrimPayload,
	release func(),
) {
	defer func() {
		silenceApply.stopMu.Lock()
		silenceApply.stop = nil
		silenceApply.stopMu.Unlock()
		silenceApply.stopping.Store(false)
		release()
	}()

	tracks, err := collectSilenceApplyTargets(ctx, n.ds, payload)
	if err != nil {
		log.Error(ctx, "Silence trim could not collect songs", err)
		return
	}
	silenceApply.total.Store(int64(len(tracks)))
	options := conf.Server.Scanner.LoudnessNormalization
	repo := n.ds.MediaFile(ctx)
	audits := n.ds.SilenceTrimAudit(ctx)

	silenceApply.stopMu.Lock()
	stop := silenceApply.stop
	silenceApply.stopMu.Unlock()
	for _, mf := range tracks {
		select {
		case <-stop:
			return
		default:
		}

		stored := mf.SilenceTrimAudit
		path := absoluteSelectedMediaPath(mf.LibraryPath, mf.Path)
		result, applyErr := silencetrim.Apply(
			ctx,
			&mf,
			path,
			stored,
			options.BackupFolder,
		)
		persistErr := error(nil)
		if result.Audit != nil {
			persistErr = audits.Put(result.Audit)
			if persistErr != nil {
				log.Warn(ctx, "Could not save silence-trim result", "id", mf.ID, persistErr)
				if result.Changed {
					if _, rollbackErr := silencetrim.Restore(
						ctx,
						&mf,
						path,
						result.Audit,
						options.BackupFolder,
					); rollbackErr != nil {
						log.Error(
							ctx,
							"Could not roll back silence trim after audit write failed",
							"id", mf.ID,
							"path", path,
							"persistError", persistErr,
							"rollbackError", rollbackErr,
						)
					}
				}
			}
		}
		if applyErr == nil &&
			persistErr == nil &&
			(result.Changed || result.Reconciled) {
			if err := repo.UpdateFileSizeAndDuration(
				mf.ID,
				result.Audit.SizeAfter,
				result.Audit.DurationAfter,
			); err != nil {
				if result.Changed {
					restored, rollbackErr := silencetrim.Restore(
						ctx,
						&mf,
						path,
						result.Audit,
						options.BackupFolder,
					)
					if rollbackErr == nil && restored != nil {
						if auditErr := audits.Put(restored); auditErr != nil {
							log.Error(ctx, "Could not save rolled-back silence audit", "id", mf.ID, auditErr)
						}
					}
					if rollbackErr != nil {
						log.Error(
							ctx,
							"Could not roll back silence trim after media metadata write failed",
							"id", mf.ID,
							"path", path,
							"metadataError", err,
							"rollbackError", rollbackErr,
						)
					}
					result.Changed = false
				}
				applyErr = fmt.Errorf("updating song duration after trim: %w", err)
			}
		}

		switch {
		case applyErr != nil ||
			persistErr != nil ||
			(result.Audit != nil && result.Audit.Status == model.SilenceTrimStatusFailed):
			silenceApply.failed.Add(1)
			if applyErr != nil {
				log.Warn(ctx, "Could not trim song edges", "id", mf.ID, "path", path, applyErr)
			}
		case result.Changed || result.Reconciled:
			silenceApply.trimmed.Add(1)
			if _, err := copyTrackToSyncMP3Folder(mf.LibraryPath, path); err != nil {
				log.Warn(ctx, "Trimmed song but could not update sync copy", "path", path, err)
			}
		default:
			silenceApply.skipped.Add(1)
		}
		silenceApply.processed.Add(1)
	}
}

// An "apply all" run is deliberately not a second analysis of every song. It
// opens only dry-run proposals that are safe, or review proposals the client
// approved, and never reopens a result already applied.
func collectSilenceApplyTargets(
	ctx context.Context,
	ds model.DataStore,
	payload silenceTrimPayload,
) ([]model.MediaFile, error) {
	if !payload.All {
		return collectSilenceTrimTargets(ctx, ds, payload)
	}
	filter := squirrel.And{
		squirrel.Eq{"media_file.missing": false},
		squirrel.Eq{"media_file.library_id": slice.Unique(payload.LibraryIDs)},
		squirrel.Eq{"media_file_silence_trim.status": model.SilenceTrimStatusAnalyzed},
		squirrel.Or{
			squirrel.Eq{
				"media_file_silence_trim.classification": model.SilenceTrimClassSafe,
			},
			squirrel.And{
				squirrel.Eq{
					"media_file_silence_trim.classification": model.SilenceTrimClassReview,
				},
				squirrel.Eq{
					"media_file_silence_trim.decision": model.SilenceTrimDecisionApprove,
				},
			},
		},
	}
	cursor, err := ds.MediaFile(ctx).GetCursor(model.QueryOptions{Filters: filter})
	if err != nil {
		return nil, err
	}
	var tracks []model.MediaFile
	for mf, err := range cursor {
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, mf)
	}
	return tracks, nil
}

func (n *Router) silenceTrimApplyStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(currentSilenceApplyStatus(""))
	}
}

func (n *Router) stopSilenceTrimApplyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !silenceApplyRunning.Load() {
			_ = json.NewEncoder(w).Encode(currentSilenceApplyStatus(
				"No silence trim is running",
			))
			return
		}
		silenceApply.stopMu.Lock()
		if !silenceApply.stopping.Load() && silenceApply.stop != nil {
			silenceApply.stopping.Store(true)
			close(silenceApply.stop)
		}
		silenceApply.stopMu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(currentSilenceApplyStatus(
			"Stopping after the active song finishes",
		))
	}
}
