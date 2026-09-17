package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// silenceJob is the shared state of one long-running silence job.
//
// Analysis and trimming are both one of these. They are separate instances
// rather than one shared job because they are genuinely different work - one
// only reads, the other rewrites files - and the page shows their progress
// separately. They exclude each other, though: see claimSilenceWork.
type silenceJob struct {
	name      string
	running   atomic.Bool
	stopping  atomic.Bool
	startedAt atomic.Int64
	total     atomic.Int64
	processed atomic.Int64
	// changed counts tracks the job actually did something to: analysed tracks
	// found trimmable, or tracks trimmed.
	changed atomic.Int64
	skipped atomic.Int64
	failed  atomic.Int64
	// secondsRemoved is the running total of audio removed, in milliseconds so
	// it can live in an atomic integer. Reported in seconds.
	millisRemoved atomic.Int64
	inFlight      atomic.Int64
	cancelled     atomic.Int64

	stopMu sync.Mutex
	stop   chan struct{}
	cancel context.CancelFunc
}

// silenceStopGracePeriod is how long a stopping run lets tracks already in
// flight finish before their ffmpeg processes are killed.
//
// Safe to kill: a trim is written to a temp file beside the original and only
// swapped in by an atomic rename at the very end, so a track killed part-way is
// simply a track that was not trimmed. Nothing is left half-written.
var silenceStopGracePeriod = 10 * time.Second

var (
	silenceAnalyze = silenceJob{name: "silence analysis"}
	silenceTrim    = silenceJob{name: "silence trim"}
)

type silenceJobStatus struct {
	Running        bool    `json:"running"`
	Stopping       bool    `json:"stopping"`
	StartedAt      string  `json:"startedAt,omitempty"`
	Total          int64   `json:"total"`
	Processed      int64   `json:"processed"`
	Changed        int64   `json:"changed"`
	Skipped        int64   `json:"skipped"`
	Failed         int64   `json:"failed"`
	SecondsRemoved float64 `json:"secondsRemoved"`
	InFlight       int64   `json:"inFlight"`
	Cancelled      int64   `json:"cancelled"`
	Message        string  `json:"message,omitempty"`
}

// claimSilenceWork refuses to start a job while any silence job is going.
//
// Analysis reads the same rows trimming writes, and trimming rewrites the very
// files analysis is measuring. Running both would have the analysis record the
// head and tail of a file that is being cut underneath it, and the resulting
// plan would describe audio that no longer exists.
//
// The loudness jobs are deliberately NOT consulted. They are a separate feature
// with separate state, and the client is entitled to run one while the other
// works. The two do touch the same files, which is why a trim re-probes and
// refuses a track whose duration has moved since it was measured.
func claimSilenceWork(job *silenceJob) bool {
	if silenceAnalyze.running.Load() || silenceTrim.running.Load() {
		return false
	}
	job.running.Store(true)
	return true
}

// begin resets the counters and starts the job's goroutine.
func (j *silenceJob) begin(ctx context.Context, total int64, run func(context.Context)) bool {
	j.stopMu.Lock()
	defer j.stopMu.Unlock()
	if !claimSilenceWork(j) {
		return false
	}
	j.stopping.Store(false)
	j.stop = make(chan struct{})
	j.startedAt.Store(time.Now().Unix())
	j.total.Store(total)
	j.processed.Store(0)
	j.changed.Store(0)
	j.skipped.Store(0)
	j.failed.Store(0)
	j.millisRemoved.Store(0)
	j.inFlight.Store(0)
	j.cancelled.Store(0)

	// Detached from the request that started it - the run outlives the HTTP
	// call - but cancellable, so a stop reaches the ffmpeg processes and not
	// only the loop handing out work.
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	j.cancel = cancel
	go run(runCtx)
	return true
}

func (j *silenceJob) requestStop() {
	j.stopMu.Lock()
	defer j.stopMu.Unlock()
	if !j.running.Load() || j.stopping.Load() {
		return
	}
	j.stopping.Store(true)
	if j.stop != nil {
		close(j.stop)
	}
	if cancel := j.cancel; cancel != nil {
		time.AfterFunc(silenceStopGracePeriod, cancel)
	}
}

func (j *silenceJob) stopSignal() <-chan struct{} {
	j.stopMu.Lock()
	defer j.stopMu.Unlock()
	return j.stop
}

func (j *silenceJob) finish() {
	j.stopMu.Lock()
	defer j.stopMu.Unlock()
	j.stop = nil
	if cancel := j.cancel; cancel != nil {
		cancel()
		j.cancel = nil
	}
	j.inFlight.Store(0)
	j.stopping.Store(false)
	j.running.Store(false)
}

func (j *silenceJob) addRemoved(seconds float64) {
	j.millisRemoved.Add(int64(seconds * 1000))
}

func (j *silenceJob) status(msg string) silenceJobStatus {
	st := silenceJobStatus{
		Running:        j.running.Load(),
		Stopping:       j.stopping.Load(),
		Total:          j.total.Load(),
		Processed:      j.processed.Load(),
		Changed:        j.changed.Load(),
		Skipped:        j.skipped.Load(),
		Failed:         j.failed.Load(),
		SecondsRemoved: float64(j.millisRemoved.Load()) / 1000,
		InFlight:       j.inFlight.Load(),
		Cancelled:      j.cancelled.Load(),
		Message:        msg,
	}
	if ts := j.startedAt.Load(); ts > 0 {
		st.StartedAt = time.Unix(ts, 0).Format(time.RFC3339)
	}
	return st
}

func writeSilenceStatus(w http.ResponseWriter, code int, status silenceJobStatus) {
	w.Header().Set("Content-Type", "application/json")
	if code != http.StatusOK {
		w.WriteHeader(code)
	}
	_ = json.NewEncoder(w).Encode(status)
}
