package nativeapi

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
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
	inFlight  atomic.Int64
	cancelled atomic.Int64
	stopMu    sync.Mutex
	stop      chan struct{}
	// cancel kills the ffmpeg processes of tracks still being measured when the
	// grace period runs out. Analysis never writes to a library file, so there
	// is nothing to leave half-done: an abandoned track is simply one that was
	// not measured, and the next sweep measures it.
	cancel context.CancelFunc
}

var loudnessAnalyze loudnessAnalyzeJob

type loudnessAnalyzeStatus struct {
	Running   bool   `json:"running"`
	Stopping  bool   `json:"stopping"`
	StartedAt string `json:"startedAt,omitempty"`
	Total     int64  `json:"total"`
	Processed int64  `json:"processed"`
	Failed    int64  `json:"failed"`
	InFlight  int64  `json:"inFlight"`
	Cancelled int64  `json:"cancelled"`
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
		InFlight:  loudnessAnalyze.inFlight.Load(),
		Cancelled: loudnessAnalyze.cancelled.Load(),
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

// beginLoudnessAnalyze prepares a fresh stop signal for one analysis run, and
// returns the context the run must use so a stop can reach its ffmpeg processes.
func beginLoudnessAnalyze(ctx context.Context) (context.Context, bool) {
	loudnessAnalyze.stopMu.Lock()
	libraryLoudness.stopMu.Lock()
	defer loudnessAnalyze.stopMu.Unlock()
	defer libraryLoudness.stopMu.Unlock()
	// The release function is discarded on purpose: finishLoudnessAnalyze clears
	// the same flag, and it is the one path that also tears down the stop channel
	// and the run context.
	if _, busy := claimLoudnessFileWork(&loudnessAnalyze.running); busy != "" {
		return nil, false
	}
	loudnessAnalyze.stopping.Store(false)
	loudnessAnalyze.stop = make(chan struct{})
	loudnessAnalyze.inFlight.Store(0)
	loudnessAnalyze.cancelled.Store(0)

	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	loudnessAnalyze.cancel = cancel
	return runCtx, true
}

// stopLoudnessAnalyze stops the sweep: no further tracks are handed out, the
// ones in flight get stopGracePeriod to finish, and whatever is still going
// after that is killed. Every persisted row is a complete analysis either way -
// a track that does not finish measuring writes nothing at all.
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
	if cancel := loudnessAnalyze.cancel; cancel != nil {
		time.AfterFunc(stopGracePeriod, cancel)
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
	if cancel := loudnessAnalyze.cancel; cancel != nil {
		cancel()
		loudnessAnalyze.cancel = nil
	}
	loudnessAnalyze.inFlight.Store(0)
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
		ctx := r.Context()
		w.Header().Set("Content-Type", "application/json")
		loudnessAnalyze.stopMu.Lock()
		libraryLoudness.stopMu.Lock()
		defer loudnessAnalyze.stopMu.Unlock()
		defer libraryLoudness.stopMu.Unlock()
		// A restore counts too: it writes the very record this would delete.
		if busy := loudnessFileWorkBusy(); busy != "" {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"message": "Stop " + busy + " before clearing analysis data",
			})
			return
		}
		// Copy it before destroying it.
		//
		// This wipes every measurement in the library and there is no undo. A
		// copy costs a second and turns an irreversible click into a reversible
		// one; without it the only safety net was whatever snapshot happened to
		// exist already, which the same button's aftermath then aged out of the
		// keep-window.
		//
		// A failure here does not block the clear. The button was pressed on
		// purpose, and refusing to do the thing that was asked because the
		// courtesy backup did not work would be its own surprise - but it is
		// said plainly in the response so nobody assumes a copy exists.
		var savedTo string
		if dir := loudnessAuditDbFolder(ctx, n.ds); dir != "" {
			keep := conf.Server.Scanner.LoudnessNormalization.AuditDbKeep
			if snapshot, snapErr := n.ds.LoudnessAudit(ctx).Snapshot(dir, keep); snapErr != nil {
				if !errors.Is(snapErr, model.ErrNoLoudnessAuditData) {
					log.Warn(ctx, "Could not copy LUFS data before clearing it", "folder", dir, snapErr)
				}
			} else {
				savedTo = snapshot.File
				log.Info(ctx, "LUFS data copied before clearing", "file", snapshot.File, "rows", snapshot.Rows)
			}
		}

		count, err := n.ds.LoudnessAudit(ctx).Clear()
		if err != nil {
			log.Error(ctx, "Could not clear LUFS analysis data", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		message := "LUFS analysis data cleared"
		if savedTo != "" {
			message += " - a copy was saved to " + savedTo + " first"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cleared":  count,
			"snapshot": savedTo,
			"message":  message,
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
		runCtx, ok := beginLoudnessAnalyze(r.Context())
		if !ok {
			// Named rather than "another job", so someone who started a restore
			// is not left guessing what is holding the library. Read after the
			// fact, so it may have finished in between - hence the fallback.
			busy := loudnessFileWorkBusy()
			if busy == "" {
				busy = "another LUFS job"
			}
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(currentLoudnessAnalyzeStatus(
				busy + " is already running - wait for it to finish"))
			return
		}
		loudnessAnalyze.startedAt.Store(time.Now().Unix())
		loudnessAnalyze.total.Store(0)
		loudnessAnalyze.processed.Store(0)
		loudnessAnalyze.failed.Store(0)

		go n.runLoudnessAnalyze(runCtx, payload)

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

	// A hand-picked list is read up front: it is short, and reading it once is
	// what reports which ids could not be read at all. The whole library is
	// counted and then streamed - see eachLibraryAnalyzeTarget.
	var selected []model.MediaFile
	if payload.All {
		total, err := n.countAnalyzeTargets(ctx, payload)
		if err != nil {
			log.Error(ctx, "LUFS analysis: could not read media files", err)
			return
		}
		loudnessAnalyze.total.Store(total)
	} else {
		var err error
		selected, err = n.collectAnalyzeTargets(ctx, payload)
		if err != nil {
			log.Error(ctx, "LUFS analysis: could not read media files", err)
			return
		}
		loudnessAnalyze.total.Store(int64(len(selected)))
	}
	if loudnessAnalyze.stopping.Load() {
		log.Info(ctx, "LUFS analysis stopped before track processing began")
		return
	}
	total := loudnessAnalyze.total.Load()
	log.Info(ctx, "LUFS analysis started", "tracks", total, "targetLUFS", options.TargetLUFS,
		"mode", cmp.Or(payload.Mode, "full"))
	start := time.Now()

	work := make(chan model.MediaFile)
	var wg sync.WaitGroup
	workers := options.Parallelism
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if int64(workers) > total {
		workers = max(int(total), 1)
	}
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			normalizer := ffmpeg.NewLoudnessNormalizer()
			for mf := range work {
				trackPath := absoluteSelectedMediaPath(mf.LibraryPath, mf.Path)
				loudnessAnalyze.inFlight.Add(1)
				var audit *model.LoudnessAudit
				if originalOnly {
					audit = loudness.MeasureOriginal(ctx, normalizer, mf.ID, mf.LibraryPath, trackPath,
						target, tolerance, options.BackupFolder)
				} else {
					audit = loudness.Audit(ctx, normalizer, mf.ID, mf.LibraryPath, trackPath, target, tolerance, options.BackupFolder)
				}
				loudnessAnalyze.inFlight.Add(-1)
				// A measurement cut short by a stop is not a failed measurement.
				// Recording it would replace a good reading with an error, and
				// mark a track as analysed when nothing was learned about it.
				if ctx.Err() != nil {
					loudnessAnalyze.cancelled.Add(1)
					log.Debug(ctx, "LUFS analysis: track abandoned on stop", "path", mf.Path)
					continue
				}
				// A sweep must not cancel a restore.
				//
				// restored_at is what stops the next optimise run from
				// re-normalizing a song somebody deliberately put back. A fresh
				// audit carries no such mark, and Put writes every column, so a
				// whole-library analysis silently cleared it for every restored
				// song in the library - and the next run undid every restore.
				//
				// Only carried forward on a sweep. Re-analysing one track by
				// hand is someone looking at that track and asking for it to be
				// reconsidered, and clearing the mark there is the documented
				// intent.
				if payload.All && mf.LoudnessAudit != nil && mf.LoudnessAudit.RestoredAt != nil {
					audit.RestoredAt = mf.LoudnessAudit.RestoredAt
				}
				if err := n.ds.LoudnessAudit(ctx).Put(audit); err != nil {
					log.Warn(ctx, "LUFS analysis: could not save audit record", "id", mf.ID, err)
				}
				if audit.Status == model.LoudnessStatusFailed {
					loudnessAnalyze.failed.Add(1)
				}
				if done := loudnessAnalyze.processed.Add(1); done%100 == 0 {
					log.Info(ctx, "LUFS analysis progress", "processed", done, "total", total, "elapsed", time.Since(start))
				}
			}
		}()
	}
	stop := loudnessAnalyzeStopSignal()
	dispatch := func(mf model.MediaFile) bool {
		select {
		case <-stop:
			return false
		case work <- mf:
			return true
		}
	}
	if payload.All {
		if err := n.eachLibraryAnalyzeTarget(ctx, payload, dispatch); err != nil {
			log.Error(ctx, "LUFS analysis: could not read media files", err)
		}
	} else {
		for _, mf := range selected {
			if !dispatch(mf) {
				break
			}
		}
	}
	close(work)
	wg.Wait()

	log.Info(ctx, "LUFS analysis finished", "stopped", loudnessAnalyze.stopping.Load(),
		"processed", loudnessAnalyze.processed.Load(),
		"failed", loudnessAnalyze.failed.Load(), "elapsed", time.Since(start))

	// Measurements of the untouched originals are the one snapshot that cannot
	// be taken again once a song has been rewritten.
	snapshotLoudnessAudit(context.WithoutCancel(ctx), n.ds, "analysis finished")
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

// usableAnalyzeTarget: a song with no file behind it cannot be measured.
func usableAnalyzeTarget(mf *model.MediaFile) bool {
	return !mf.Missing && strings.TrimSpace(mf.Path) != "" && strings.TrimSpace(mf.LibraryPath) != ""
}

// eachLibraryAnalyzeTarget walks every song a whole-library sweep will measure,
// calling fn for each until fn returns false.
//
// Walked rather than collected. The sweep used to build a slice holding every
// model.MediaFile in the library before measuring any of them - a wide struct,
// tags and joined audit columns included - and then held it for the entire run,
// which on a large library is hours. The optimisation run has always streamed
// its cursor; this now does the same.
//
// The cost is reading the table twice, once to count and once to dispatch. That
// is a few seconds against a run that decodes every file in the library, and it
// buys an exact total for the progress bar rather than an estimate.
func (n *Router) eachLibraryAnalyzeTarget(ctx context.Context, payload loudnessAnalyzePayload,
	fn func(model.MediaFile) bool) error {
	cursor, err := n.ds.MediaFile(ctx).GetCursor()
	if err != nil {
		return err
	}
	for mf, err := range cursor {
		if err != nil {
			return err
		}
		if loudnessAnalyze.stopping.Load() {
			return nil
		}
		if !usableAnalyzeTarget(&mf) || alreadyMeasured(&mf, payload) {
			continue
		}
		if !fn(mf) {
			return nil
		}
	}
	return nil
}

// countAnalyzeTargets counts what a whole-library sweep will measure, so the
// progress bar has a denominator. Counting by walking rather than by SQL
// because "already measured" depends on the joined audit row and "usable"
// depends on the library path, and the two together are easier to keep honest
// in one place than to reproduce as a query that must not drift from it.
func (n *Router) countAnalyzeTargets(ctx context.Context, payload loudnessAnalyzePayload) (int64, error) {
	var total int64
	err := n.eachLibraryAnalyzeTarget(ctx, payload, func(model.MediaFile) bool {
		total++
		return true
	})
	return total, err
}

// collectAnalyzeTargets reads a hand-picked selection. Short by definition, and
// read once so that ids which cannot be read at all are reported exactly once.
func (n *Router) collectAnalyzeTargets(ctx context.Context, payload loudnessAnalyzePayload) ([]model.MediaFile, error) {
	repo := n.ds.MediaFile(ctx)
	var tracks []model.MediaFile
	for _, id := range payload.IDs {
		mf, err := repo.Get(id)
		if err != nil {
			log.Warn(ctx, "LUFS analysis: could not read media file", "id", id, err)
			loudnessAnalyze.failed.Add(1)
			continue
		}
		if usableAnalyzeTarget(mf) && !alreadyMeasured(mf, payload) {
			tracks = append(tracks, *mf)
		}
	}
	return tracks, nil
}
