package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/go-chi/chi/v5"
	coresilence "github.com/navidrome/navidrome/core/silence"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

const (
	silenceAnalysisWorkers   = 2
	silenceAnalysisBatchSize = 500
)

type silenceAnalyzeJob struct {
	running   atomic.Bool
	total     atomic.Int64
	processed atomic.Int64
	failed    atomic.Int64
	inFlight  atomic.Int64
	startedAt atomic.Int64
	messageMu sync.RWMutex
	message   string
}

type silenceAnalyzePayload struct {
	IDs []string `json:"ids"`
	All bool     `json:"all"`
}

type silenceAnalyzeStatus struct {
	Running   bool   `json:"running"`
	StartedAt string `json:"startedAt,omitempty"`
	Total     int64  `json:"total"`
	Processed int64  `json:"processed"`
	Failed    int64  `json:"failed"`
	InFlight  int64  `json:"inFlight"`
	Message   string `json:"message,omitempty"`
}

func newSilenceAnalyzeJob() *silenceAnalyzeJob {
	return &silenceAnalyzeJob{}
}

func (j *silenceAnalyzeJob) setMessage(message string) {
	j.messageMu.Lock()
	j.message = message
	j.messageMu.Unlock()
}

func (j *silenceAnalyzeJob) status() silenceAnalyzeStatus {
	j.messageMu.RLock()
	message := j.message
	j.messageMu.RUnlock()
	status := silenceAnalyzeStatus{
		Running:   j.running.Load(),
		Total:     j.total.Load(),
		Processed: j.processed.Load(),
		Failed:    j.failed.Load(),
		InFlight:  j.inFlight.Load(),
		Message:   message,
	}
	if startedAt := j.startedAt.Load(); startedAt > 0 {
		status.StartedAt = time.Unix(startedAt, 0).Format(time.RFC3339)
	}
	return status
}

func (j *silenceAnalyzeJob) begin() bool {
	if !j.running.CompareAndSwap(false, true) {
		return false
	}
	j.total.Store(0)
	j.processed.Store(0)
	j.failed.Store(0)
	j.inFlight.Store(0)
	j.startedAt.Store(time.Now().Unix())
	j.setMessage("Silence analysis started - audio files are read only")
	return true
}

func (n *Router) addSongSilenceRoute(r chi.Router) {
	r.Route("/song/silence", func(r chi.Router) {
		r.Get("/analyze", n.silenceAnalyzeStatusHandler())
		r.Post("/analyze", n.startSilenceAnalyzeHandler())
		r.Get("/integrity", n.silenceIntegrityHandler())
		r.Get("/trim", n.silenceMutationStatusHandler())
		r.Post("/trim", n.startSilenceMutationHandler(silenceMutationTrim))
		r.Get("/restore", n.silenceMutationStatusHandler())
		r.Post("/restore", n.startSilenceMutationHandler(silenceMutationRestore))
	})
}

func (n *Router) silenceAnalyzeStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(n.silenceJob.status())
	}
}

func (n *Router) startSilenceAnalyzeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var payload silenceAnalyzePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if (payload.All && len(payload.IDs) > 0) || (!payload.All && len(payload.IDs) == 0) {
			http.Error(w, "provide either ids or all=true", http.StatusBadRequest)
			return
		}
		n.silenceOperationMu.Lock()
		mutationRunning := n.silenceMutation.running.Load()
		started := !mutationRunning && n.silenceJob.begin()
		n.silenceOperationMu.Unlock()
		if !started {
			w.WriteHeader(http.StatusConflict)
			status := n.silenceJob.status()
			status.Message = "Another silence analysis, trim, or restore job is already running"
			_ = json.NewEncoder(w).Encode(status)
			return
		}

		ctx := context.WithoutCancel(r.Context())
		go n.runSilenceAnalyze(ctx, payload)
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(n.silenceJob.status())
	}
}

func (n *Router) runSilenceAnalyze(ctx context.Context, payload silenceAnalyzePayload) {
	defer n.silenceJob.running.Store(false)
	work := make(chan model.MediaFile)
	var wait sync.WaitGroup
	wait.Add(silenceAnalysisWorkers)
	for range silenceAnalysisWorkers {
		go func() {
			defer wait.Done()
			analyzer := coresilence.NewAnalyzer()
			for mediaFile := range work {
				n.analyzeSongSilence(ctx, analyzer, mediaFile)
			}
		}()
	}
	dispatchErr := n.dispatchSilenceTargets(ctx, payload, work)
	close(work)
	wait.Wait()

	if dispatchErr != nil {
		n.silenceJob.failed.Add(1)
		n.silenceJob.setMessage("Silence analysis stopped because the song library could not be read")
		log.Error(ctx, "Silence analysis could not read media files", dispatchErr)
		return
	}
	if n.silenceJob.total.Load() == 0 {
		n.silenceJob.setMessage("No available songs were found to analyze")
		return
	}
	if n.silenceJob.failed.Load() > 0 {
		n.silenceJob.setMessage("Silence analysis finished with some failures")
	} else {
		n.silenceJob.setMessage("Silence analysis completed")
	}
}

func (n *Router) analyzeSongSilence(ctx context.Context, analyzer coresilence.Analyzer, mediaFile model.MediaFile) {
	n.silenceJob.inFlight.Add(1)
	mediaPath := silenceMediaPath(mediaFile.LibraryPath, mediaFile.Path)
	result, analysisErr := analyzer.Analyze(ctx, mediaPath, float64(mediaFile.Duration))
	n.silenceJob.inFlight.Add(-1)
	sourceSize, sourceUpdatedAt := mediaFile.Size, mediaFile.UpdatedAt
	if currentInfo, statErr := os.Stat(mediaPath); statErr == nil {
		sourceSize = currentInfo.Size()
		sourceUpdatedAt = currentInfo.ModTime()
	}

	analysis := &model.SilenceAnalysis{
		MediaFileID:     mediaFile.ID,
		ThresholdDB:     coresilence.DefaultThresholdDB,
		MinimumSilence:  coresilence.DefaultMinimumSilence,
		SourceSize:      sourceSize,
		SourceUpdatedAt: sourceUpdatedAt,
		AnalyzedAt:      time.Now(),
	}
	failed := analysisErr != nil
	if failed {
		analysis.Status = model.SilenceStatusFailed
		analysis.Error = analysisErr.Error()
		log.Warn(ctx, "Silence analysis failed", "id", mediaFile.ID, "path", mediaFile.Path, "err", analysisErr)
	} else {
		analysis.Status = model.SilenceStatusAnalyzed
		analysis.LeadingSilence = float64Pointer(result.Leading)
		analysis.TrailingSilence = float64Pointer(result.Trailing)
	}
	if err := n.ds.SilenceAnalysis(ctx).Put(analysis); err != nil {
		failed = true
		log.Warn(ctx, "Could not save silence analysis", "id", mediaFile.ID, "err", err)
	}
	if failed {
		n.silenceJob.failed.Add(1)
	}
	n.silenceJob.processed.Add(1)
}

func (n *Router) dispatchSilenceTargets(ctx context.Context, payload silenceAnalyzePayload, work chan<- model.MediaFile) error {
	repository := n.ds.MediaFile(ctx)
	usable := func(mediaFile *model.MediaFile) bool {
		return !mediaFile.Missing && strings.TrimSpace(mediaFile.Path) != "" && strings.TrimSpace(mediaFile.LibraryPath) != ""
	}
	dispatch := func(mediaFile model.MediaFile) {
		if !usable(&mediaFile) {
			return
		}
		n.silenceJob.total.Add(1)
		work <- mediaFile
	}
	if payload.All {
		// Page through the library so a 90k-song run never holds 90k complete
		// MediaFile records (including tags and participants) in memory at once.
		for offset := 0; ; offset += silenceAnalysisBatchSize {
			batch, err := repository.GetAll(model.QueryOptions{
				Sort:   "media_file.id",
				Order:  "ASC",
				Max:    silenceAnalysisBatchSize,
				Offset: offset,
				Filters: squirrel.Eq{
					"media_file.missing": false,
				},
			})
			if err != nil {
				return err
			}
			for _, mediaFile := range batch {
				dispatch(mediaFile)
			}
			if len(batch) < silenceAnalysisBatchSize {
				break
			}
		}
		return nil
	}

	seen := make(map[string]struct{}, len(payload.IDs))
	for _, id := range payload.IDs {
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		mediaFile, err := repository.Get(id)
		if err != nil {
			log.Warn(ctx, "Could not load song for silence analysis", "id", id, "err", err)
			continue
		}
		dispatch(*mediaFile)
	}
	return nil
}

func silenceMediaPath(libraryPath, mediaPath string) string {
	path := filepath.FromSlash(strings.TrimSpace(mediaPath))
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(filepath.FromSlash(libraryPath), path))
}

func float64Pointer(value float64) *float64 {
	return &value
}
