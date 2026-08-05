package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
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
	// inFlight is how many tracks are open right now. It is what makes stopping
	// legible: the UI can say "finishing 3" and count down, instead of showing a
	// bar that has stopped moving for reasons nobody can see.
	inFlight atomic.Int64
	// cancelled counts tracks dropped when the grace period ran out. Kept apart
	// from failures deliberately - nothing is wrong with those tracks, they were
	// simply not finished, and reporting eight failures every time someone
	// presses stop would be both alarming and untrue.
	cancelled atomic.Int64
	stopMu    sync.Mutex
	stop      chan struct{}
	// cancel kills the ffmpeg processes of tracks still being worked on when
	// the grace period runs out. Without it a stop can only stop handing out
	// new tracks, and has to wait for whatever is already in flight.
	cancel context.CancelFunc
}

// stopGracePeriod is how long a stopping run lets the tracks already in flight
// finish before their ffmpeg processes are killed.
//
// A stop used to mean "stop dispatching", which sounds immediate but is not: one
// track is in flight per worker - eight on a typical machine - and each is
// several full decodes, so an unlucky one holds a worker for minutes. Waiting
// for the slowest of eight is what made stopping feel like nothing happened.
//
// Killing them is safe. An encode is written to a temp file and the library file
// is only ever touched by the atomic rename at the very end, so a track killed
// mid-encode is simply a track that was not processed. The grace period exists
// so the ones about to finish do, rather than throwing away nearly-complete work
// for the sake of a couple of seconds.
// A var rather than a const only so tests need not wait ten real seconds.
var stopGracePeriod = 10 * time.Second

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
	InFlight   int64  `json:"inFlight"`
	Cancelled  int64  `json:"cancelled"`
	Message    string `json:"message,omitempty"`
}

// beginLibraryLoudness starts a run unless one is already going. Returns false
// when a run was already in progress.
//
// With ids, the run covers exactly those songs; without, the whole library at
// the given phase. Both are the same job - one background run, one status, one
// stop button - because a selection that behaves differently from a sweep is a
// second implementation of the same thing, and was the reason optimising a
// selection had no progress and could not be stopped.
func (n *Router) beginLibraryLoudness(ctx context.Context, phase int, ids []string) bool {
	loudnessAnalyze.stopMu.Lock()
	libraryLoudness.stopMu.Lock()
	defer loudnessAnalyze.stopMu.Unlock()
	defer libraryLoudness.stopMu.Unlock()
	// Analysis reads files but writes the same records; the other two rewrite
	// the files themselves. Any of them running means this run must wait.
	//
	// The release function is discarded on purpose: finishLibraryLoudness clears
	// the same flag, and it is the one path that also tears down the stop channel
	// and the run context.
	if _, busy := claimLoudnessFileWork(&libraryLoudness.running); busy != "" {
		return false
	}
	libraryLoudness.phase.Store(int64(phase))
	libraryLoudness.stopping.Store(false)
	libraryLoudness.stop = make(chan struct{})
	libraryLoudness.startedAt.Store(time.Now().Unix())
	libraryLoudness.total.Store(0)
	libraryLoudness.processed.Store(0)
	libraryLoudness.normalized.Store(0)
	libraryLoudness.skipped.Store(0)
	libraryLoudness.failed.Store(0)
	libraryLoudness.inFlight.Store(0)
	libraryLoudness.cancelled.Store(0)

	// Detached from the request that started it - the run outlives the HTTP
	// call - but cancellable on its own terms, so a stop can reach the ffmpeg
	// processes rather than only the loop that hands out work.
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	libraryLoudness.cancel = cancel

	go n.runLibraryLoudness(runCtx, phase, ids)
	return true
}

// stopLibraryLoudness stops a running whole-library run: no further tracks are
// handed out, the tracks in flight get stopGracePeriod to finish, and whatever
// is still going after that has its ffmpeg processes killed. Tracks already
// rewritten are left as they are; a track killed part-way is left untouched.
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
	// Cancelling a run that has already finished on its own does nothing, so
	// the timer needs no coordination with the run ending first.
	if cancel := libraryLoudness.cancel; cancel != nil {
		time.AfterFunc(stopGracePeriod, cancel)
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
	// Releases the context whether or not a stop ever cancelled it. A pending
	// grace-period timer may still fire afterwards, which is harmless: the
	// function it holds is this same one, and cancelling twice does nothing.
	if cancel := libraryLoudness.cancel; cancel != nil {
		cancel()
		libraryLoudness.cancel = nil
	}
	libraryLoudness.inFlight.Store(0)
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
		// No check that "Optimise all LUFS" is on. That switch now means "a run
		// is happening" and turns itself off when one ends, so requiring it here
		// would refuse the phase 2 run every time - and applying decisions the
		// client has already made is an explicit action that should not need a
		// library-wide switch turned on first.
		phase := loudnessPhaseFromRequest(r)
		if !n.beginLibraryLoudness(r.Context(), phase, nil) {
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
		InFlight:   libraryLoudness.inFlight.Load(),
		Cancelled:  libraryLoudness.cancelled.Load(),
		Message:    msg,
	}
	if ts := libraryLoudness.startedAt.Load(); ts > 0 {
		st.StartedAt = time.Unix(ts, 0).Format(time.RFC3339)
	}
	return st
}

func (n *Router) runLibraryLoudness(ctx context.Context, phase int, ids []string) {
	// "Optimise all LUFS" describes a run, not a preference, so it goes off when
	// the run ends. Persisting it - rather than leaving the UI to infer it from
	// the job status - is what makes a page reload agree with what is actually
	// happening; otherwise the stored setting says on for ever after the first
	// run, and the switch has to be turned off and on again to start another.
	//
	// The stored context may be cancelled by now, which would fail the write.
	defer func() {
		saveCtx := context.WithoutCancel(ctx)
		if err := loudness.SetEnabled(saveCtx, n.ds, false); err != nil {
			log.Error(saveCtx, "Could not turn 'Optimise all LUFS' off after the run", err)
		}
		finishLibraryLoudness()
	}()

	options := conf.Server.Scanner.LoudnessNormalization
	log.Info(ctx, "LUFS run started", "phase", phase, "selected", len(ids),
		"targetLUFS", options.TargetLUFS)
	start := time.Now()

	if total, err := n.countLoudnessTargets(ctx, phase, ids); err != nil {
		log.Warn(ctx, "LUFS run: could not count tracks, progress will have no total", err)
	} else {
		libraryLoudness.total.Store(total)
		log.Info(ctx, "LUFS run: tracks to process", "total", total, "phase", phase)
	}

	cursor, err := n.ds.MediaFile(ctx).GetCursor(model.QueryOptions{Filters: loudnessRunFilter(phase, ids)})
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
				libraryLoudness.inFlight.Add(1)
				res, err := optimizeOneTrack(ctx, n.ds, normalizer, &mf)
				libraryLoudness.inFlight.Add(-1)
				switch {
				case err != nil && ctx.Err() != nil:
					// Abandoned when the grace period ran out. The file was not
					// touched and no audit was written, so the next run selects
					// it again - it is postponed, not lost, and counting it as
					// processed or failed would misreport both.
					libraryLoudness.cancelled.Add(1)
					log.Debug(ctx, "LUFS run: track abandoned on stop", "path", mf.Path)
					continue
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
			// Counted as processed as well as skipped: the total came from the
			// same query this loop reads, so a track dropped here without being
			// counted leaves the progress bar permanently short of its total and
			// the estimated time never resolving.
			libraryLoudness.skipped.Add(1)
			libraryLoudness.processed.Add(1)
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
		"failed", libraryLoudness.failed.Load(), "cancelled", libraryLoudness.cancelled.Load(),
		"elapsed", time.Since(start))

	// The run has just rewritten files and the records describing them. Copy
	// those records somewhere the data directory cannot take with it.
	snapshotLoudnessAudit(context.WithoutCancel(ctx), n.ds, "optimisation run finished")
}

// countLoudnessTargets counts how many tracks this run will actually touch, so
// the UI can show "processed of total" rather than an open-ended counter. It
// asks the database rather than walking every row, and so applies the same
// filter the run itself does.
func (n *Router) countLoudnessTargets(ctx context.Context, phase int, ids []string) (int64, error) {
	return n.ds.MediaFile(ctx).CountAll(model.QueryOptions{Filters: loudnessRunFilter(phase, ids)})
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
func loudnessRunFilter(phase int, ids []string) squirrel.Sqlizer {
	notMissing := squirrel.Eq{"media_file.missing": false}
	// An explicit selection overrides the planner. Every other clause below
	// exists to stop a library sweep from redoing work it has already done or
	// already refused - reasonable for a sweep, wrong for a list of songs a
	// person picked out by hand. Applying them here would quietly drop the very
	// tracks someone selected in order to try again.
	if len(ids) > 0 {
		return squirrel.And{notMissing, squirrel.Eq{"media_file.id": ids}}
	}
	if phase == loudness.PhaseReview {
		return squirrel.And{
			notMissing,
			// Restoring clears the decision, so a restored song should not reach
			// here at all. It is excluded anyway: clearing the decision is a
			// separate write that only logs if it fails, and the cost of it
			// having failed is re-applying a choice the client has just undone.
			squirrel.Expr("media_file_loudness.restored_at is null"),
			// The decision is the gate, not the phase.
			//
			// This used to require phase = 2 as well, on the assumption that a
			// decision only ever exists on a song the planner sent to review.
			// That stopped being true once refused songs were listed on the
			// exceptions page: those are phase 1, a person can decide on them,
			// and the extra clause then matched nothing at all. "Apply
			// decisions" started a run, selected zero songs and reported
			// success - the worst possible way to do nothing.
			//
			// Every guard that matters is still here. The decision itself means
			// a person chose this song, which is what the phase was standing in
			// for, and skip is excluded below because leaving a song alone is
			// not work to be done.
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
		// A song whose original was put back is left alone. Restoring is someone
		// saying "I want this one as it was", and a sweep that normalized it
		// again would undo that silently - every time. Re-analysing the song
		// clears the mark, and picking it out by hand skips this filter entirely,
		// so neither way of saying "do this one" is blocked.
		squirrel.Expr("media_file_loudness.restored_at is null"),
		squirrel.Expr("coalesce(media_file_loudness.phase, ?) <> ?",
			loudness.PhaseUnplanned, loudness.PhaseReview),
		// Deliberately left alone: near enough to target that correcting it is
		// not worth what it would cost. Retrying it every sweep would spend a
		// full encode per run to arrive at the same conclusion.
		squirrel.Expr("coalesce(media_file_loudness.phase, ?) <> ?",
			loudness.PhaseUnplanned, loudness.PhaseCloseEnough),
		squirrel.Expr("not (coalesce(media_file_loudness.phase, ?) = ? and media_file_loudness.lufs_before is not null)",
			loudness.PhaseUnplanned, loudness.PhaseDone),
		// A track whose last attempt was built and then rejected is left out.
		// Nothing about it or the settings has changed since, so rebuilding the
		// same file would reach the same refusal - on every run, for ever.
		// Re-analysing clears the mark and the track is tried again.
		squirrel.Expr("coalesce(media_file_loudness.action, '') <> ?", model.LoudnessActionRefused),
	}
}
