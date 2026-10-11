package scanner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/loudness"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"golang.org/x/sync/errgroup"
)

// Imports and their pending work commit together. The main process consumes
// this queue even when directory scanning runs in a separate process.
const autoLoudnessPendingKey = "ScannerAutoLoudnessPending"

func pendingNewSongs(ctx context.Context, ds model.DataStore) ([]string, error) {
	value, err := ds.Property(ctx).DefaultGet(autoLoudnessPendingKey, "[]")
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal([]byte(value), &ids); err != nil {
		return nil, fmt.Errorf("reading pending LUFS imports: %w", err)
	}
	return ids, nil
}

func savePendingNewSongs(ctx context.Context, ds model.DataStore, ids []string) error {
	if ids == nil {
		ids = []string{}
	}
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	return ds.Property(ctx).Put(autoLoudnessPendingKey, string(data))
}

func queueNewSongs(ctx context.Context, ds model.DataStore, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	pending, err := pendingNewSongs(ctx, ds)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(pending)+len(ids))
	for _, id := range pending {
		seen[id] = true
	}
	for _, id := range ids {
		if !seen[id] {
			pending = append(pending, id)
			seen[id] = true
		}
	}
	return savePendingNewSongs(ctx, ds, pending)
}

// normalizeNewSongs deliberately reads the configured option, not the
// one-shot "Optimise all LUFS" UI toggle, which resets after each manual run.
func normalizeNewSongs(ctx context.Context, ds model.DataStore) []string {
	if !conf.Server.Scanner.LoudnessNormalization.Enabled || ctx.Err() != nil {
		return nil
	}
	ids, err := pendingNewSongs(ctx, ds)
	if err != nil {
		return []string{fmt.Sprintf("Automatic LUFS normalization: %v", err)}
	}
	if len(ids) == 0 {
		return nil
	}
	release, err := loudness.WaitForFileWork(ctx, "automatic LUFS normalization")
	if err != nil {
		return []string{fmt.Sprintf("Automatic LUFS normalization deferred: %v", err)}
	}
	defer release()

	parallelism := conf.Server.Scanner.LoudnessNormalization.Parallelism
	if parallelism < 1 {
		parallelism = 1
	}
	group := new(errgroup.Group)
	group.SetLimit(parallelism)
	completed := make([]bool, len(ids))
	var mu sync.Mutex
	var warnings []string
	warn := func(id string, err error) {
		mu.Lock()
		defer mu.Unlock()
		warnings = append(warnings, fmt.Sprintf("Automatic LUFS normalization for %s: %v", id, err))
		log.Warn(ctx, "Scanner: Automatic LUFS normalization failed", "id", id, err)
	}
	log.Info(ctx, "Scanner: Normalizing new songs", "tracks", len(ids), "parallelism", parallelism)
	for i, id := range ids {
		if ctx.Err() != nil {
			break
		}
		group.Go(func() error {
			if ctx.Err() != nil {
				return nil
			}
			mf, err := ds.MediaFile(ctx).Get(id)
			if errors.Is(err, model.ErrNotFound) || (err == nil && (mf == nil || mf.Missing)) {
				// Move reconciliation may have merged this import into an older
				// song. Keep that song's original identity and normalization state.
				completed[i] = true
				return nil
			}
			if err != nil {
				warn(id, err)
				return nil // Database failures retain work for the next scan.
			}
			if strings.Contains(mf.LibraryPath, "://") {
				warn(id, fmt.Errorf("automatic normalization requires a local library"))
				completed[i] = true
				return nil
			}
			if mf.LoudnessAudit != nil && mf.LoudnessAudit.IsRestored() {
				completed[i] = true // Never undo an explicit Restore Original.
				return nil
			}
			result, err := loudness.OptimizeTrack(ctx, ds, ffmpeg.NewLoudnessNormalizer(), mf)
			if ctx.Err() != nil && !result.Changed {
				return nil // Retry work canceled before the file was replaced.
			}
			completed[i] = true
			if err != nil {
				warn(id, err)
			} else {
				log.Info(ctx, "Scanner: New song LUFS processed", "id", id, "path", mf.Path,
					"changed", result.Changed, "fromLUFS", result.OldLUFS, "finalLUFS", result.NewLUFS)
			}
			return nil
		})
	}
	_ = group.Wait()
	finished := make(map[string]bool, len(ids))
	for i, id := range ids {
		if completed[i] {
			finished[id] = true
		}
	}
	// A separate CLI scan may have appended imports while this batch was
	// working. Acknowledge only our finished IDs against the current queue.
	err = ds.WithTx(func(tx model.DataStore) error {
		recordCtx := context.WithoutCancel(ctx)
		pending, err := pendingNewSongs(recordCtx, tx)
		if err != nil {
			return err
		}
		pending = slices.DeleteFunc(pending, func(id string) bool { return finished[id] })
		return savePendingNewSongs(recordCtx, tx, pending)
	}, "scanner: acknowledge automatic LUFS work")
	if err != nil {
		warn("pending queue", err)
	}
	return warnings
}
