package nativeapi

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/loudness"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// loudnessAnalyzeJob tracks a measure-only sweep. Analysis never modifies a
// file: it only records what each track currently is, which is the only way to
// capture the "before" state of tracks that have not been processed yet.
type loudnessAnalyzeJob struct {
	running   atomic.Bool
	stopping  atomic.Bool
	startedAt atomic.Int64
	total     atomic.Int64
	processed atomic.Int64
	failed    atomic.Int64
	stopMu    sync.Mutex
	stop      chan struct{}
}

var loudnessAnalyze loudnessAnalyzeJob

type loudnessAnalyzeStatus struct {
	Running   bool   `json:"running"`
	Stopping  bool   `json:"stopping"`
	StartedAt string `json:"startedAt,omitempty"`
	Total     int64  `json:"total"`
	Processed int64  `json:"processed"`
	Failed    int64  `json:"failed"`
	Message   string `json:"message,omitempty"`
}

type loudnessAnalyzePayload struct {
	IDs []string `json:"ids"`
	All bool     `json:"all"`
	// Mode "original" measures loudness only: one pass per file, skipping the
	// backup comparison and null test, and skipping tracks already measured.
	// Anything else runs the full audit.
	Mode string `json:"mode"`
}

const loudnessModeOriginal = "original"

func currentLoudnessAnalyzeStatus(msg string) loudnessAnalyzeStatus {
	st := loudnessAnalyzeStatus{
		Running:   loudnessAnalyze.running.Load(),
		Stopping:  loudnessAnalyze.stopping.Load(),
		Total:     loudnessAnalyze.total.Load(),
		Processed: loudnessAnalyze.processed.Load(),
		Failed:    loudnessAnalyze.failed.Load(),
		Message:   msg,
	}
	if ts := loudnessAnalyze.startedAt.Load(); ts > 0 {
		st.StartedAt = time.Unix(ts, 0).Format(time.RFC3339)
	}
	return st
}

func (n *Router) loudnessAnalyzeStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(currentLoudnessAnalyzeStatus(""))
	}
}

// beginLoudnessAnalyze prepares a fresh stop signal for one analysis run.
func beginLoudnessAnalyze() bool {
	loudnessAnalyze.stopMu.Lock()
	libraryLoudness.stopMu.Lock()
	defer loudnessAnalyze.stopMu.Unlock()
	defer libraryLoudness.stopMu.Unlock()
	if loudnessAnalyze.running.Load() || libraryLoudness.running.Load() {
		return false
	}
	loudnessAnalyze.running.Store(true)
	loudnessAnalyze.stopping.Store(false)
	loudnessAnalyze.stop = make(chan struct{})
	return true
}

// stopLoudnessAnalyze asks the producer to stop dispatching tracks. Workers
// finish only the tracks already in progress, so every persisted row is a
// complete analysis rather than a half-written result.
func stopLoudnessAnalyze() {
	loudnessAnalyze.stopMu.Lock()
	defer loudnessAnalyze.stopMu.Unlock()
	if !loudnessAnalyze.running.Load() || loudnessAnalyze.stopping.Load() {
		return
	}
	loudnessAnalyze.stopping.Store(true)
	if loudnessAnalyze.stop != nil {
		close(loudnessAnalyze.stop)
	}
}

func loudnessAnalyzeStopSignal() <-chan struct{} {
	loudnessAnalyze.stopMu.Lock()
	defer loudnessAnalyze.stopMu.Unlock()
	return loudnessAnalyze.stop
}

func finishLoudnessAnalyze() {
	loudnessAnalyze.stopMu.Lock()
	loudnessAnalyze.stop = nil
	loudnessAnalyze.stopping.Store(false)
	loudnessAnalyze.running.Store(false)
	loudnessAnalyze.stopMu.Unlock()
}

func (n *Router) stopLoudnessAnalyzeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !loudnessAnalyze.running.Load() {
			_ = json.NewEncoder(w).Encode(currentLoudnessAnalyzeStatus("No library analysis is running"))
			return
		}
		stopLoudnessAnalyze()
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(currentLoudnessAnalyzeStatus("Stopping analysis after active tracks finish"))
	}
}

// clearLoudnessAnalyzeResults deletes only the derived audit rows shown on the
// LUFS pages. Audio files, backups, and media-file metadata are untouched.
func (n *Router) clearLoudnessAnalyzeResults() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		loudnessAnalyze.stopMu.Lock()
		libraryLoudness.stopMu.Lock()
		defer loudnessAnalyze.stopMu.Unlock()
		defer libraryLoudness.stopMu.Unlock()
		if loudnessAnalyze.running.Load() || libraryLoudness.running.Load() {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"message": "Stop the active LUFS job before clearing analysis data",
			})
			return
		}
		count, err := n.ds.LoudnessAudit(r.Context()).Clear()
		if err != nil {
			log.Error(r.Context(), "Could not clear LUFS analysis data", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cleared": count,
			"message": "LUFS analysis data cleared",
		})
	}
}

func (n *Router) startLoudnessAnalyze() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var payload loudnessAnalyzePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(payload.IDs) == 0 && !payload.All {
			http.Error(w, "ids are required (or set all=true)", http.StatusBadRequest)
			return
		}
		if !beginLoudnessAnalyze() {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(currentLoudnessAnalyzeStatus("Another LUFS job is already running"))
			return
		}
		loudnessAnalyze.startedAt.Store(time.Now().Unix())
		loudnessAnalyze.total.Store(0)
		loudnessAnalyze.processed.Store(0)
		loudnessAnalyze.failed.Store(0)

		go n.runLoudnessAnalyze(context.Background(), payload)

		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(currentLoudnessAnalyzeStatus("Analysis started - no files are modified"))
	}
}

func (n *Router) runLoudnessAnalyze(ctx context.Context, payload loudnessAnalyzePayload) {
	defer finishLoudnessAnalyze()

	options := conf.Server.Scanner.LoudnessNormalization
	target := ffmpeg.LoudnessTarget{
		IntegratedLUFS: options.TargetLUFS,
		TruePeak:       options.TruePeak,
		LRA:            options.LRA,
	}
	tolerance := effectiveManualLoudnessTolerance(options.Tolerance)
	originalOnly := payload.Mode == loudnessModeOriginal

	tracks, err := n.collectAnalyzeTargets(ctx, payload)
	if err != nil {
		log.Error(ctx, "LUFS analysis: could not read media files", err)
		return
	}
	if loudnessAnalyze.stopping.Load() {
		log.Info(ctx, "LUFS analysis stopped before track processing began")
		return
	}
	loudnessAnalyze.total.Store(int64(len(tracks)))
	log.Info(ctx, "LUFS analysis started", "tracks", len(tracks), "targetLUFS", options.TargetLUFS,
		"mode", cmp.Or(payload.Mode, "full"))
	start := time.Now()

	work := make(chan model.MediaFile)
	var wg sync.WaitGroup
	workers := options.Parallelism
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > len(tracks) {
		workers = max(len(tracks), 1)
	}
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			normalizer := ffmpeg.NewLoudnessNormalizer()
			for mf := range work {
				trackPath := absoluteSelectedMediaPath(mf.LibraryPath, mf.Path)
				var audit *model.LoudnessAudit
				if originalOnly {
					audit = loudness.MeasureOriginal(ctx, normalizer, mf.ID, trackPath, target, tolerance)
				} else {
					audit = loudness.Audit(ctx, normalizer, mf.ID, mf.LibraryPath, trackPath, target, tolerance, options.BackupFolder)
				}
				if err := n.ds.LoudnessAudit(ctx).Put(audit); err != nil {
					log.Warn(ctx, "LUFS analysis: could not save audit record", "id", mf.ID, err)
				}
				if audit.Status == model.LoudnessStatusFailed {
					loudnessAnalyze.failed.Add(1)
				}
				if done := loudnessAnalyze.processed.Add(1); done%100 == 0 {
					log.Info(ctx, "LUFS analysis progress", "processed", done, "total", len(tracks), "elapsed", time.Since(start))
				}
			}
		}()
	}
	stop := loudnessAnalyzeStopSignal()
dispatch:
	for _, mf := range tracks {
		select {
		case <-stop:
			break dispatch
		case work <- mf:
		}
	}
	close(work)
	wg.Wait()

	log.Info(ctx, "LUFS analysis finished", "stopped", loudnessAnalyze.stopping.Load(),
		"processed", loudnessAnalyze.processed.Load(),
		"failed", loudnessAnalyze.failed.Load(), "elapsed", time.Since(start))
}

// alreadyMeasured lets the loudness-only pass resume: a track whose original
// loudness is already recorded is skipped, so re-running costs nothing and an
// interrupted sweep picks up where it left off. The full audit always re-runs,
// because it also re-checks the file against its stored original.
func alreadyMeasured(mf *model.MediaFile, payload loudnessAnalyzePayload) bool {
	if payload.Mode != loudnessModeOriginal {
		return false
	}
	audit := mf.LoudnessAudit
	return audit != nil && audit.LufsBefore != nil
}

func (n *Router) collectAnalyzeTargets(ctx context.Context, payload loudnessAnalyzePayload) ([]model.MediaFile, error) {
	repo := n.ds.MediaFile(ctx)
	var tracks []model.MediaFile

	usable := func(mf *model.MediaFile) bool {
		return !mf.Missing && strings.TrimSpace(mf.Path) != "" && strings.TrimSpace(mf.LibraryPath) != ""
	}

	if payload.All {
		cursor, err := repo.GetCursor()
		if err != nil {
			return nil, err
		}
		for mf, err := range cursor {
			if err != nil {
				return nil, err
			}
			if loudnessAnalyze.stopping.Load() {
				break
			}
			if usable(&mf) && !alreadyMeasured(&mf, payload) {
				tracks = append(tracks, mf)
			}
		}
		return tracks, nil
	}

	for _, id := range payload.IDs {
		mf, err := repo.Get(id)
		if err != nil {
			log.Warn(ctx, "LUFS analysis: could not read media file", "id", id, err)
			loudnessAnalyze.failed.Add(1)
			continue
		}
		if usable(mf) && !alreadyMeasured(mf, payload) {
			tracks = append(tracks, *mf)
		}
	}
	return tracks, nil
}
