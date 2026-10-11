package loudness

import (
	"context"
	"sync"
)

// fileWork excludes jobs whose measurements, audio changes, and audit writes
// must describe the same version of a song, including automatic scanner work.
var fileWork = struct {
	sync.Mutex
	name    string
	changed chan struct{}
}{changed: make(chan struct{})}

// ClaimFileWork reserves the library for one job, or reports the current job.
// The returned release is idempotent and must be called when the job finishes.
func ClaimFileWork(name string) (release func(), busy string) {
	fileWork.Lock()
	defer fileWork.Unlock()
	return claimFileWork(name)
}

func claimFileWork(name string) (func(), string) {
	if fileWork.name != "" {
		return nil, fileWork.name
	}
	if name == "" {
		name = "a LUFS job"
	}
	fileWork.name = name
	var once sync.Once
	return func() {
		once.Do(func() {
			fileWork.Lock()
			defer fileWork.Unlock()
			fileWork.name = ""
			close(fileWork.changed)
			fileWork.changed = make(chan struct{})
		})
	}, ""
}

// WaitForFileWork waits for a claim without dropping pending scanner work.
// Cancellation stops waiting without acquiring or releasing another job's claim.
func WaitForFileWork(ctx context.Context, name string) (func(), error) {
	for {
		fileWork.Lock()
		if err := ctx.Err(); err != nil {
			fileWork.Unlock()
			return nil, err
		}
		release, busy := claimFileWork(name)
		changed := fileWork.changed
		fileWork.Unlock()
		if busy == "" {
			return release, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-changed:
		}
	}
}

// FileWorkBusy names the job currently holding the library, if any.
func FileWorkBusy() string {
	fileWork.Lock()
	defer fileWork.Unlock()
	return fileWork.name
}
