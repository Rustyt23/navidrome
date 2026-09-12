package nativeapi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/navidrome/navidrome/model"
)

// Retain the latest error in job status, including after completion and page
// reload. A failed database write must not look like a successful audio change.
type loudnessJobError struct {
	mu      sync.Mutex
	message string
}

func (e *loudnessJobError) set(message string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.message = message
}

func (e *loudnessJobError) get() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.message
}

// Audio may already have changed. Retry transient database failures without
// reprocessing the song, and commit its audit and media metadata together.
// Cancellation of the audio job must not cancel this recording step.
func saveLoudnessRecord(ctx context.Context, ds model.DataStore, id string,
	write func(context.Context, model.DataStore) error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = ds.WithTx(func(tx model.DataStore) error { return write(ctx, tx) })
		if err == nil {
			return nil
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return fmt.Errorf("song %s: could not save LUFS records; audio may have changed. Re-analyse this song before processing again: %w", id, err)
			case <-time.After(time.Duration(attempt+1) * 100 * time.Millisecond):
			}
		}
	}
	return fmt.Errorf("song %s: could not save LUFS records; audio may have changed. Re-analyse this song before processing again: %w", id, err)
}
