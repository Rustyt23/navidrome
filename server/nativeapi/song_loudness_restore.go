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
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/loudness"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/slice"
)

// restoreLoudnessJob tracks a restore sweep.
//
// Restoring used to happen inside the request that asked for it: one song at a
// time, with no progress, no way to stop it, and the browser left waiting until
// the last song was done. On a selection of any size that reads as a hung page,
// and a browser that gives up does not stop the server - it just stops anyone
// seeing what is happening. It is a background job now, like the optimisation
// run and the analysis sweep, and reports itself the same way.
type restoreLoudnessJob struct {
	running   atomic.Bool
	stopping  atomic.Bool
	startedAt atomic.Int64
	total     atomic.Int64
	processed atomic.Int64
	restored  atomic.Int64
	skipped   atomic.Int64
	failed    atomic.Int64
	inFlight  atomic.Int64
	cancelled atomic.Int64
	stopMu    sync.Mutex
	stop      chan struct{}
	// cancel kills the ffmpeg processes of songs still being measured when the
	// grace period runs out. The copy itself is not interruptible and does not
	// need to be: it finishes in milliseconds, and a song is either replaced or
	// it is not. Only the measurement afterwards is worth abandoning.
	cancel context.CancelFunc
}

var restoreLoudness restoreLoudnessJob

type restoreLoudnessStatus struct {
	Running   bool   `json:"running"`
	Stopping  bool   `json:"stopping"`
	StartedAt string `json:"startedAt,omitempty"`
	Total     int64  `json:"total"`
	Processed int64  `json:"processed"`
	Restored  int64  `json:"restored"`
	Skipped   int64  `json:"skipped"`
	Failed    int64  `json:"failed"`
	InFlight  int64  `json:"inFlight"`
	Cancelled int64  `json:"cancelled"`
	Message   string `json:"message,omitempty"`
}

func currentRestoreLoudnessStatus(msg string) restoreLoudnessStatus {
	st := restoreLoudnessStatus{
		Running:   restoreLoudness.running.Load(),
		Stopping:  restoreLoudness.stopping.Load(),
		Total:     restoreLoudness.total.Load(),
		Processed: restoreLoudness.processed.Load(),
		Restored:  restoreLoudness.restored.Load(),
		Skipped:   restoreLoudness.skipped.Load(),
		Failed:    restoreLoudness.failed.Load(),
		InFlight:  restoreLoudness.inFlight.Load(),
		Cancelled: restoreLoudness.cancelled.Load(),
		Message:   msg,
	}
	if ts := restoreLoudness.startedAt.Load(); ts > 0 {
		st.StartedAt = time.Unix(ts, 0).Format(time.RFC3339)
	}
	return st
}

// beginRestoreLoudness claims the right to work on library songs and prepares a
// fresh stop signal. The release function is discarded on purpose:
// finishRestoreLoudness clears the same flag and is the one path that also tears
// down the stop channel and the run context.
func beginRestoreLoudness(ctx context.Context, total int) (context.Context, bool) {
	restoreLoudness.stopMu.Lock()
	defer restoreLoudness.stopMu.Unlock()
	if _, busy := claimLoudnessFileWork(&restoreLoudness.running); busy != "" {
		return nil, false
	}
	restoreLoudness.stopping.Store(false)
	restoreLoudness.stop = make(chan struct{})
	restoreLoudness.startedAt.Store(time.Now().Unix())
	restoreLoudness.total.Store(int64(total))
	restoreLoudness.processed.Store(0)
	restoreLoudness.restored.Store(0)
	restoreLoudness.skipped.Store(0)
	restoreLoudness.failed.Store(0)
	restoreLoudness.inFlight.Store(0)
	restoreLoudness.cancelled.Store(0)

	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	restoreLoudness.cancel = cancel
	return runCtx, true
}

func finishRestoreLoudness() {
	restoreLoudness.stopMu.Lock()
	defer restoreLoudness.stopMu.Unlock()
	restoreLoudness.running.Store(false)
	restoreLoudness.stopping.Store(false)
	if cancel := restoreLoudness.cancel; cancel != nil {
		cancel()
		restoreLoudness.cancel = nil
	}
	restoreLoudness.stop = nil
	restoreLoudness.inFlight.Store(0)
}

// stopRestoreLoudness stops the sweep: no further songs are handed out, and the
// ones in flight get stopGracePeriod before their measurement is killed.
//
// Nothing is left half-restored by this. A song is replaced by a whole-file copy
// that either happened or did not, so the worst a stop can do is leave a song
// still normalized - which is where it started.
func stopRestoreLoudness() {
	restoreLoudness.stopMu.Lock()
	defer restoreLoudness.stopMu.Unlock()
	if !restoreLoudness.running.Load() || restoreLoudness.stopping.Load() {
		return
	}
	restoreLoudness.stopping.Store(true)
	if restoreLoudness.stop != nil {
		close(restoreLoudness.stop)
	}
	if cancel := restoreLoudness.cancel; cancel != nil {
		time.AfterFunc(stopGracePeriod, cancel)
	}
}

func restoreLoudnessStopSignal() <-chan struct{} {
	restoreLoudness.stopMu.Lock()
	defer restoreLoudness.stopMu.Unlock()
	return restoreLoudness.stop
}

func (n *Router) restoreLoudnessStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(currentRestoreLoudnessStatus(""))
	}
}

func (n *Router) stopRestoreLoudnessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !restoreLoudness.running.Load() {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(currentRestoreLoudnessStatus("No restore is running"))
			return
		}
		stopRestoreLoudness()
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(currentRestoreLoudnessStatus("Stopping the restore"))
	}
}

// restoreSongLoudness puts the stored originals back for the selected songs.
//
// This is the promise the backups exist to keep: whatever normalization did to
// a song can be undone on request, exactly, without re-encoding it back - the
// file the client started with is copied into place unchanged.
func (n *Router) restoreSongLoudness() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		w.Header().Set("Content-Type", "application/json")

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

		// Restoring a song something else is in the middle of rewriting would
		// have the two racing for the same file, and whichever finished last
		// would win - which for a restore means it silently does not stick.
		runCtx, ok := beginRestoreLoudness(ctx, len(ids))
		if !ok {
			busy := loudnessFileWorkBusy()
			if busy == "" {
				busy = "another LUFS job"
			}
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(currentRestoreLoudnessStatus(
				busy + " is in progress - wait for it to finish before restoring"))
			return
		}

		go n.runRestoreLoudness(runCtx, ids)

		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(currentRestoreLoudnessStatus("Restore started"))
	}
}

func (n *Router) runRestoreLoudness(ctx context.Context, ids []string) {
	defer finishRestoreLoudness()

	options := conf.Server.Scanner.LoudnessNormalization
	target := ffmpeg.LoudnessTarget{
		IntegratedLUFS: options.TargetLUFS,
		TruePeak:       options.TruePeak,
		LRA:            options.LRA,
	}
	tolerance := effectiveManualLoudnessTolerance(options.Tolerance)
	normalizer := ffmpeg.NewLoudnessNormalizer()

	log.Info(ctx, "LUFS restore started", "songs", len(ids))
	start := time.Now()

	work := make(chan string)
	var wg sync.WaitGroup
	workers := options.Parallelism
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > len(ids) {
		workers = max(len(ids), 1)
	}
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for id := range work {
				restoreLoudness.inFlight.Add(1)
				n.restoreOneSong(ctx, normalizer, id, target, tolerance, options.BackupFolder)
				restoreLoudness.inFlight.Add(-1)
				if done := restoreLoudness.processed.Add(1); done%50 == 0 {
					log.Info(ctx, "LUFS restore progress", "processed", done, "total", len(ids),
						"elapsed", time.Since(start))
				}
			}
		}()
	}

	stop := restoreLoudnessStopSignal()
dispatch:
	for _, id := range ids {
		select {
		case <-stop:
			break dispatch
		case work <- id:
		}
	}
	close(work)
	wg.Wait()

	log.Info(ctx, "LUFS restore finished", "stopped", restoreLoudness.stopping.Load(),
		"restored", restoreLoudness.restored.Load(), "skipped", restoreLoudness.skipped.Load(),
		"failed", restoreLoudness.failed.Load(), "elapsed", time.Since(start))

	if restoreLoudness.restored.Load() > 0 {
		snapshotLoudnessAudit(context.WithoutCancel(ctx), n.ds, "songs restored")
	}
}

func (n *Router) restoreOneSong(ctx context.Context, normalizer ffmpeg.LoudnessNormalizer,
	id string, target ffmpeg.LoudnessTarget, tolerance float64, backupFolder string) {
	mf, err := n.ds.MediaFile(ctx).Get(id)
	if err != nil {
		log.Warn(ctx, "LUFS restore: could not read media file", "id", id, err)
		restoreLoudness.failed.Add(1)
		return
	}
	if mf.Path == "" || mf.LibraryPath == "" {
		log.Warn(ctx, "LUFS restore: song file is missing", "id", id)
		restoreLoudness.failed.Add(1)
		return
	}
	trackPath := absoluteSelectedMediaPath(mf.LibraryPath, mf.Path)

	// Counted as skipped rather than failed: a song that was never rewritten has
	// nothing to restore, and selecting a whole page should not look like an
	// error.
	if ffmpeg.FindLoudnessBackup(backupFolder, mf.LibraryPath, id, trackPath) == "" {
		restoreLoudness.skipped.Add(1)
		return
	}

	previous, err := n.ds.LoudnessAudit(ctx).Get(id)
	if err != nil {
		log.Warn(ctx, "Could not read loudness audit before restoring", "id", id, err)
		previous = nil
	}

	res, err := loudness.Restore(ctx, normalizer, id, mf.LibraryPath, trackPath,
		previous, target, tolerance, backupFolder)
	if err != nil {
		log.Warn(ctx, "Could not restore original song", "id", id, "path", trackPath, err)
		restoreLoudness.failed.Add(1)
		return
	}

	// The song is already back on disk, so the record of it is written even if
	// the request that asked has gone: losing the record of a change that did
	// happen is worse than doing the work twice.
	recordCtx := context.WithoutCancel(ctx)
	repo := n.ds.MediaFile(recordCtx)
	audits := n.ds.LoudnessAudit(recordCtx)

	if err := audits.Put(res.Audit); err != nil {
		log.Warn(recordCtx, "Restored the song but could not update its audit record", "id", id, err)
	}
	// Put deliberately preserves the client's decision, but a decision that has
	// just been undone must not be reapplied by the next phase 2 run.
	if err := audits.SetDecision(id, loudness.DecisionPending); err != nil {
		log.Warn(recordCtx, "Restored the song but could not clear its decision", "id", id, err)
	}
	updateSongLoudnessTag(recordCtx, repo, id, res.LUFS)

	// The song's own row still describes the file that was just replaced. Left
	// alone it reports the normalized size and bitrate until some later scan
	// happens to notice, so the library disagrees with the disk in the one place
	// a person is most likely to look.
	if res.Probe != nil {
		if err := repo.UpdateAudioProperties(id, model.AudioFileProperties{
			BitRate:    res.Probe.BitRate,
			SampleRate: res.Probe.SampleRate,
			BitDepth:   res.Probe.BitDepth,
			Channels:   res.Probe.Channels,
			Duration:   float32(res.Probe.Duration),
			Size:       res.Probe.Size,
		}); err != nil {
			log.Warn(recordCtx, "Restored the song but could not refresh its stored details", "id", id, err)
		}
	}

	log.Info(recordCtx, "Restored original song", "id", id, "path", trackPath, "lufs", res.LUFS)
	restoreLoudness.restored.Add(1)
}
