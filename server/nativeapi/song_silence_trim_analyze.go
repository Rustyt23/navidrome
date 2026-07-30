package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/silencetrim"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

type silenceAnalyzeJob struct {
	stopping  atomic.Bool
	startedAt atomic.Int64
	total     atomic.Int64
	processed atomic.Int64
	safe      atomic.Int64
	review    atomic.Int64
	blocked   atomic.Int64
	failed    atomic.Int64
	stopMu    sync.Mutex
	stop      chan struct{}
}

var silenceAnalyze silenceAnalyzeJob

type silenceAnalyzeStatus struct {
	Running   bool   `json:"running"`
	Stopping  bool   `json:"stopping"`
	StartedAt string `json:"startedAt,omitempty"`
	Total     int64  `json:"total"`
	Processed int64  `json:"processed"`
	Safe      int64  `json:"safe"`
	Review    int64  `json:"review"`
	Blocked   int64  `json:"blocked"`
	Failed    int64  `json:"failed"`
	Message   string `json:"message,omitempty"`
}

func currentSilenceAnalyzeStatus(message string) silenceAnalyzeStatus {
	status := silenceAnalyzeStatus{
		Running:   silenceAnalyzeRunning.Load(),
		Stopping:  silenceAnalyze.stopping.Load(),
		Total:     silenceAnalyze.total.Load(),
		Processed: silenceAnalyze.processed.Load(),
		Safe:      silenceAnalyze.safe.Load(),
		Review:    silenceAnalyze.review.Load(),
		Blocked:   silenceAnalyze.blocked.Load(),
		Failed:    silenceAnalyze.failed.Load(),
		Message:   message,
	}
	if started := silenceAnalyze.startedAt.Load(); started > 0 {
		status.StartedAt = time.Unix(started, 0).Format(time.RFC3339)
	}
	return status
}

func (n *Router) startSilenceTrimAnalyze() http.HandlerFunc {
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
		if loudnessAnalyze.running.Load() {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(currentSilenceAnalyzeStatus(
				"A LUFS analysis is in progress",
			))
			return
		}
		release, busy := claimLoudnessFileWork(&silenceAnalyzeRunning)
		if busy != "" {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(currentSilenceAnalyzeStatus(
				busy + " is in progress",
			))
			return
		}
		silenceAnalyze.stopping.Store(false)
		silenceAnalyze.startedAt.Store(time.Now().Unix())
		silenceAnalyze.total.Store(0)
		silenceAnalyze.processed.Store(0)
		silenceAnalyze.safe.Store(0)
		silenceAnalyze.review.Store(0)
		silenceAnalyze.blocked.Store(0)
		silenceAnalyze.failed.Store(0)
		silenceAnalyze.stopMu.Lock()
		silenceAnalyze.stop = make(chan struct{})
		silenceAnalyze.stopMu.Unlock()

		go n.runSilenceTrimAnalyze(context.WithoutCancel(r.Context()), payload, release)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(currentSilenceAnalyzeStatus(
			"Dry-run analysis started; no audio files will be modified",
		))
	}
}

func (n *Router) runSilenceTrimAnalyze(
	ctx context.Context,
	payload silenceTrimPayload,
	release func(),
) {
	defer func() {
		silenceAnalyze.stopMu.Lock()
		silenceAnalyze.stop = nil
		silenceAnalyze.stopMu.Unlock()
		silenceAnalyze.stopping.Store(false)
		release()
	}()

	tracks, err := collectSilenceTrimTargets(ctx, n.ds, payload)
	if err != nil {
		log.Error(ctx, "Silence-trim analysis could not collect songs", err)
		return
	}
	silenceAnalyze.total.Store(int64(len(tracks)))
	audits := n.ds.SilenceTrimAudit(ctx)
	files := n.ds.MediaFile(ctx)

	workers := conf.Server.Scanner.LoudnessNormalization.Parallelism
	if workers <= 0 {
		workers = max(1, runtime.NumCPU()/2)
	}
	if workers > len(tracks) {
		workers = max(1, len(tracks))
	}

	work := make(chan model.MediaFile)
	var group sync.WaitGroup
	group.Add(workers)
	for range workers {
		go func() {
			defer group.Done()
			for mf := range work {
				path := absoluteSelectedMediaPath(mf.LibraryPath, mf.Path)
				previous := mf.SilenceTrimAudit
				generation, generationState, generationErr := silencetrim.ReconcileGeneration(
					path,
					mf.LibraryPath,
					conf.Server.Scanner.LoudnessNormalization.BackupFolder,
					mf.ID,
				)
				if generationState == silencetrim.GenerationStateResult {
					if err := files.UpdateFileSizeAndDuration(
						mf.ID,
						generation.SizeAfter,
						generation.DurationAfter,
					); err != nil {
						log.Warn(ctx, "Could not reconcile recovered silence-trim media metadata", "id", mf.ID, err)
						silenceAnalyze.failed.Add(1)
						silenceAnalyze.processed.Add(1)
						continue
					}
					if err := audits.Put(generation); err != nil {
						log.Warn(ctx, "Could not save recovered silence-trim generation", "id", mf.ID, err)
						silenceAnalyze.failed.Add(1)
					} else if _, err := copyTrackToSyncMP3Folder(mf.LibraryPath, path); err != nil {
						log.Warn(ctx, "Recovered silence trim but could not update sync copy", "path", path, err)
					}
					silenceAnalyze.processed.Add(1)
					continue
				}
				sourceGeneration := generationState == silencetrim.GenerationStateSource
				if generationState == silencetrim.GenerationStateSource {
					source := *generation
					source.Status = model.SilenceTrimStatusAnalyzed
					source.Integrity = model.SilenceTrimIntegrityPending
					source.ResultSHA256 = ""
					source.ResultModifiedAt = nil
					source.AppliedAt = nil
					source.Decision = model.SilenceTrimDecisionPending
					if previous != nil &&
						previous.Status == model.SilenceTrimStatusAnalyzed &&
						silencetrim.SameProposal(previous, generation) {
						source.Decision = previous.Decision
					}
					previous = &source
				}
				if generationErr != nil &&
					!silencetrim.CanIgnoreGenerationError(generationErr, previous, path) {
					stale := model.SilenceTrimAudit{
						MediaFileID: mf.ID,
						AnalyzedAt:  time.Now(),
					}
					if previous != nil {
						stale = *previous
					}
					stale.Status = model.SilenceTrimStatusFailed
					stale.Integrity = model.SilenceTrimIntegrityFailed
					stale.Error = generationErr.Error()
					stale.Reason = "Protected restore history was kept; current bytes match neither side of the durable trim generation"
					if err := audits.Put(&stale); err != nil {
						log.Warn(ctx, "Could not save divergent silence-trim generation", "id", mf.ID, err)
					}
					silenceAnalyze.failed.Add(1)
					silenceAnalyze.processed.Add(1)
					continue
				}
				if previous != nil &&
					previous.HasBackup &&
					previous.ResultSHA256 != "" &&
					silencetrim.ResultFileUnchanged(previous, path) {
					if previous.Status != model.SilenceTrimStatusProcessed ||
						previous.Integrity != model.SilenceTrimIntegrityVerified {
						current := *previous
						current.Status = model.SilenceTrimStatusProcessed
						current.Integrity = model.SilenceTrimIntegrityVerified
						current.Error = ""
						if err := audits.Put(&current); err != nil {
							log.Warn(ctx, "Could not refresh verified silence-trim state", "id", mf.ID, err)
							silenceAnalyze.failed.Add(1)
						}
					}
					silenceAnalyze.processed.Add(1)
					continue
				}
				if previous != nil &&
					previous.HasBackup &&
					previous.ResultSHA256 != "" {
					stale := *previous
					stale.Status = model.SilenceTrimStatusFailed
					stale.Integrity = model.SilenceTrimIntegrityFailed
					stale.Error = "the current file no longer matches the verified silence-trim result"
					stale.Reason = "Protected restore history was kept; resolve later audio processing before trimming again"
					if err := audits.Put(&stale); err != nil {
						log.Warn(ctx, "Could not save stale silence-trim state", "id", mf.ID, err)
					}
					silenceAnalyze.failed.Add(1)
					silenceAnalyze.processed.Add(1)
					continue
				}
				var audit *model.SilenceTrimAudit
				if generationState == silencetrim.GenerationStateSource {
					audit = silencetrim.AnalyzeRecoveredSource(ctx, &mf, path, generation)
				} else {
					audit = silencetrim.Analyze(ctx, &mf, path)
				}
				// A replaced/edited file gets a new proposal, but the immutable
				// pre-trim backup remains real and restorable.
				if previous != nil && previous.HasBackup {
					audit.HasBackup = true
					audit.BackupSHA256 = previous.BackupSHA256
				}
				if previous != nil && silencetrim.SameProposal(previous, audit) {
					audit.Decision = previous.Decision
				}
				if err := audits.Put(audit); err != nil {
					log.Warn(ctx, "Could not save silence-trim analysis", "id", mf.ID, err)
					silenceAnalyze.failed.Add(1)
					silenceAnalyze.processed.Add(1)
					continue
				}
				if sourceGeneration {
					if err := files.UpdateFileSizeAndDuration(
						mf.ID,
						audit.SizeBefore,
						audit.DurationBefore,
					); err != nil {
						log.Warn(ctx, "Could not reconcile restored silence-trim media metadata", "id", mf.ID, err)
						silenceAnalyze.failed.Add(1)
						silenceAnalyze.processed.Add(1)
						continue
					}
					if _, err := copyTrackToSyncMP3Folder(mf.LibraryPath, path); err != nil {
						log.Warn(ctx, "Reconciled silence restore but could not update sync copy", "path", path, err)
					}
				}
				switch {
				case audit.Status == model.SilenceTrimStatusFailed:
					silenceAnalyze.failed.Add(1)
				case audit.Classification == model.SilenceTrimClassSafe:
					silenceAnalyze.safe.Add(1)
				case audit.Classification == model.SilenceTrimClassReview:
					silenceAnalyze.review.Add(1)
				case audit.Classification == model.SilenceTrimClassBlocked:
					silenceAnalyze.blocked.Add(1)
				}
				silenceAnalyze.processed.Add(1)
			}
		}()
	}

	silenceAnalyze.stopMu.Lock()
	stop := silenceAnalyze.stop
	silenceAnalyze.stopMu.Unlock()
dispatch:
	for _, mf := range tracks {
		select {
		case <-stop:
			break dispatch
		case work <- mf:
		}
	}
	close(work)
	group.Wait()
}

func (n *Router) silenceTrimAnalyzeStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(currentSilenceAnalyzeStatus(""))
	}
}

func (n *Router) stopSilenceTrimAnalyzeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !silenceAnalyzeRunning.Load() {
			_ = json.NewEncoder(w).Encode(currentSilenceAnalyzeStatus(
				"No silence analysis is running",
			))
			return
		}
		silenceAnalyze.stopMu.Lock()
		if !silenceAnalyze.stopping.Load() && silenceAnalyze.stop != nil {
			silenceAnalyze.stopping.Store(true)
			close(silenceAnalyze.stop)
		}
		silenceAnalyze.stopMu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(currentSilenceAnalyzeStatus(
			"Stopping after active songs finish",
		))
	}
}
