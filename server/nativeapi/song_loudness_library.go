package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/loudness"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// libraryLoudnessJob tracks the state of the full-library LUFS processing job.
// Only one run at a time. It is started by turning "Optimise all LUFS" on and
// stopped by turning it off.
type libraryLoudnessJob struct {
	running    atomic.Bool
	stopping   atomic.Bool
	phase      atomic.Int64
	startedAt  atomic.Int64
	total      atomic.Int64
	processed  atomic.Int64
	normalized atomic.Int64
	skipped    atomic.Int64
	failed     atomic.Int64
	stopMu     sync.Mutex
	stop       chan struct{}
}

var libraryLoudness libraryLoudnessJob

type libraryLoudnessStatus struct {
	Running    bool   `json:"running"`
	Stopping   bool   `json:"stopping"`
	Phase      int    `json:"phase"`
	StartedAt  string `json:"startedAt,omitempty"`
	Total      int64  `json:"total"`
	Processed  int64  `json:"processed"`
	Normalized int64  `json:"normalized"`
	Skipped    int64  `json:"skipped"`
	Failed     int64  `json:"failed"`
	Message    string `json:"message,omitempty"`
}

// beginLibraryLoudness starts a whole-library run unless one is already going.
// Returns false when a run was already in progress.
func (n *Router) beginLibraryLoudness(ctx context.Context, phase int) bool {
	loudnessAnalyze.stopMu.Lock()
	libraryLoudness.stopMu.Lock()
	defer loudnessAnalyze.stopMu.Unlock()
	defer libraryLoudness.stopMu.Unlock()
	// Analysis reads files but writes the same records; the other two rewrite
	// the files themselves. Any of them running means this run must wait.
	if loudnessAnalyze.running.Load() || loudnessFileWorkBusy() != "" {
		return false
	}
	libraryLoudness.running.Store(true)
	libraryLoudness.phase.Store(int64(phase))
	libraryLoudness.stopping.Store(false)
	libraryLoudness.stop = make(chan struct{})
	libraryLoudness.startedAt.Store(time.Now().Unix())
	libraryLoudness.total.Store(0)
	libraryLoudness.processed.Store(0)
	libraryLoudness.normalized.Store(0)
	libraryLoudness.skipped.Store(0)
	libraryLoudness.failed.Store(0)

	go n.runLibraryLoudness(context.WithoutCancel(ctx))
	return true
}

// stopLibraryLoudness asks a running whole-library run to finish the track it
// is on and stop. Tracks already rewritten are left as they are.
func stopLibraryLoudness() {
	libraryLoudness.stopMu.Lock()
	defer libraryLoudness.stopMu.Unlock()
	if !libraryLoudness.running.Load() || libraryLoudness.stopping.Load() {
		return
	}
	libraryLoudness.stopping.Store(true)
	if libraryLoudness.stop != nil {
		close(libraryLoudness.stop)
	}
}

func libraryLoudnessStopSignal() <-chan struct{} {
	libraryLoudness.stopMu.Lock()
	defer libraryLoudness.stopMu.Unlock()
	return libraryLoudness.stop
}

func finishLibraryLoudness() {
	libraryLoudness.stopMu.Lock()
	libraryLoudness.stop = nil
	libraryLoudness.stopping.Store(false)
	libraryLoudness.running.Store(false)
	libraryLoudness.stopMu.Unlock()
}

func (n *Router) stopLibraryLoudnessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !libraryLoudness.running.Load() {
			_ = json.NewEncoder(w).Encode(currentLibraryLoudnessStatus("No LUFS optimisation is running"))
			return
		}
		stopLibraryLoudness()
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(currentLibraryLoudnessStatus("Stopping optimisation after active tracks finish"))
	}
}

func (n *Router) startLibraryLoudness() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !loudness.Enabled(r.Context(), n.ds) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(libraryLoudnessStatus{Message: "Turn on 'Optimise all LUFS' first"})
			return
		}
		phase := loudnessPhaseFromRequest(r)
		if !n.beginLibraryLoudness(r.Context(), phase) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(currentLibraryLoudnessStatus("LUFS processing is already running"))
			return
		}

		// A phase 2 run opens only the songs the client has decided on, so
		// saying it covers the library is both wrong and alarming.
		started := "LUFS processing started for the entire library"
		if phase == loudness.PhaseReview {
			started = "Applying decisions to the songs you chose"
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(currentLibraryLoudnessStatus(started))
	}
}

// loudnessPhaseFromRequest reads ?phase=2, defaulting to the phase 1 run.
func loudnessPhaseFromRequest(r *http.Request) int {
	if r.URL.Query().Get("phase") == "2" {
		return loudness.PhaseReview
	}
	return loudness.PhaseGain
}

func (n *Router) libraryLoudnessStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(currentLibraryLoudnessStatus(""))
	}
}

func currentLibraryLoudnessStatus(msg string) libraryLoudnessStatus {
	st := libraryLoudnessStatus{
		Running:    libraryLoudness.running.Load(),
		Stopping:   libraryLoudness.stopping.Load(),
		Phase:      int(libraryLoudness.phase.Load()),
		Total:      libraryLoudness.total.Load(),
		Processed:  libraryLoudness.processed.Load(),
		Normalized: libraryLoudness.normalized.Load(),
		Skipped:    libraryLoudness.skipped.Load(),
		Failed:     libraryLoudness.failed.Load(),
		Message:    msg,
	}
	if ts := libraryLoudness.startedAt.Load(); ts > 0 {
		st.StartedAt = time.Unix(ts, 0).Format(time.RFC3339)
	}
	return st
}

func (n *Router) runLibraryLoudness(ctx context.Context) {
	defer finishLibraryLoudness()

	phase := int(libraryLoudness.phase.Load())
	options := conf.Server.Scanner.LoudnessNormalization
	log.Info(ctx, "LUFS run started", "phase", phase, "targetLUFS", options.TargetLUFS)
	start := time.Now()

	if total, err := n.countLoudnessTargets(ctx, phase); err != nil {
		log.Warn(ctx, "LUFS run: could not count tracks, progress will have no total", err)
	} else {
		libraryLoudness.total.Store(total)
		log.Info(ctx, "LUFS run: tracks to process", "total", total, "phase", phase)
	}

	cursor, err := n.ds.MediaFile(ctx).GetCursor(model.QueryOptions{Filters: loudnessRunFilter(phase)})
	if err != nil {
		log.Error(ctx, "LUFS run: could not read media files", err)
		return
	}

	workers := options.Parallelism
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	work := make(chan model.MediaFile)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			normalizer := ffmpeg.NewLoudnessNormalizer()
			for mf := range work {
				res, err := optimizeOneTrack(ctx, n.ds, normalizer, &mf)
				switch {
				case err != nil:
					libraryLoudness.failed.Add(1)
					log.Warn(ctx, "LUFS run: could not optimise track", "path", mf.Path, err)
				case res.Changed:
					libraryLoudness.normalized.Add(1)
				default:
					libraryLoudness.skipped.Add(1)
				}
				libraryLoudness.processed.Add(1)
			}
		}()
	}

	stop := libraryLoudnessStopSignal()
dispatch:
	for mf, err := range cursor {
		if err != nil {
			log.Error(ctx, "LUFS run aborted: error reading media files", err)
			break
		}
		if libraryLoudness.stopping.Load() {
			log.Info(ctx, "LUFS run stopped on request", "processed", libraryLoudness.processed.Load())
			break
		}
		if strings.TrimSpace(mf.Path) == "" || strings.TrimSpace(mf.LibraryPath) == "" {
			libraryLoudness.skipped.Add(1)
			continue
		}
		select {
		case <-stop:
			break dispatch
		case work <- mf:
		}
	}
	close(work)
	wg.Wait()

	log.Info(ctx, "LUFS run finished", "phase", phase, "processed", libraryLoudness.processed.Load(),
		"normalized", libraryLoudness.normalized.Load(), "skipped", libraryLoudness.skipped.Load(),
		"failed", libraryLoudness.failed.Load(), "elapsed", time.Since(start))
}

// countLoudnessTargets counts how many tracks this run will actually touch, so
// the UI can show "processed of total" rather than an open-ended counter. It
// asks the database rather than walking every row, and so applies the same
// filter the run itself does.
func (n *Router) countLoudnessTargets(ctx context.Context, phase int) (int64, error) {
	return n.ds.MediaFile(ctx).CountAll(model.QueryOptions{Filters: loudnessRunFilter(phase)})
}

// loudnessRunFilter narrows a run to the tracks it will actually touch.
//
// The selection belongs in the query rather than in Go because the alternative
// is to hand every track to the engine and let it find out: the engine's first
// act is a full loudness measurement, which decodes the whole file. On a large
// library that means re-measuring everything already finished on every run -
// the most expensive thing the pipeline does, spent to rediscover something
// already recorded. It also means an interrupted run resumes where it stopped
// instead of starting over.
//
// Phase 1 covers everything not yet known to need review, including tracks
// never analysed - the engine measures and classifies those itself. It leaves
// out tracks already measured as on target (phase 0). That verdict is only as
// current as the last analysis: neither a change of tolerance nor a file
// replaced on disk updates it, so re-run Analyze after either. Phase 2 covers
// only tracks the client has given a decision for.
func loudnessRunFilter(phase int) squirrel.Sqlizer {
	notMissing := squirrel.Eq{"media_file.missing": false}
	if phase == loudness.PhaseReview {
		return squirrel.And{
			notMissing,
			squirrel.Eq{"media_file_loudness.phase": loudness.PhaseReview},
			squirrel.Eq{"media_file_loudness.decision": []string{
				loudness.DecisionLimit, loudness.DecisionCeiling,
			}},
		}
	}
	// A track with no audit row at all joins as NULL, which no comparison
	// matches, so the absent case is spelled out with the same "never planned"
	// value the column defaults to.
	return squirrel.And{
		notMissing,
		squirrel.Expr("coalesce(media_file_loudness.phase, ?) <> ?",
			loudness.PhaseUnplanned, loudness.PhaseReview),
		squirrel.Expr("not (coalesce(media_file_loudness.phase, ?) = ? and media_file_loudness.lufs_before is not null)",
			loudness.PhaseUnplanned, loudness.PhaseDone),
		// A track whose last attempt was built and then rejected is left out.
		// Nothing about it or the settings has changed since, so rebuilding the
		// same file would reach the same refusal - on every run, for ever.
		// Re-analysing clears the mark and the track is tried again.
		squirrel.Expr("coalesce(media_file_loudness.action, '') <> ?", model.LoudnessActionRefused),
	}
}

// copyTrackToSyncMP3Folder copies a LUFS-updated track into SyncFolder/mp3,
// preserving its library-relative path (mirrors the scanner's behavior).
// Returns the destination path, or "" when no SyncFolder is configured.
func copyTrackToSyncMP3Folder(libraryPath, trackPath string) (string, error) {
	syncRoot := strings.TrimSpace(conf.Server.SyncFolder)
	if syncRoot == "" {
		return "", nil
	}
	rel := filepath.Base(trackPath)
	if libraryPath != "" {
		if r, err := filepath.Rel(filepath.Clean(libraryPath), trackPath); err == nil && r != "." && !strings.HasPrefix(r, "..") {
			rel = r
		}
	}
	dest := filepath.Join(syncRoot, "mp3", rel)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}
	data, err := os.ReadFile(trackPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "", err
	}
	return dest, nil
}
