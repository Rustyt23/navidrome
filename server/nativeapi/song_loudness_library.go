package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/gcsync"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// libraryLoudnessJob tracks the state of the full-library LUFS processing job
// (triggered from Personal settings). Only one run at a time.
type libraryLoudnessJob struct {
	running    atomic.Bool
	startedAt  atomic.Int64
	processed  atomic.Int64
	normalized atomic.Int64
	skipped    atomic.Int64
	failed     atomic.Int64
}

var libraryLoudness libraryLoudnessJob

type libraryLoudnessStatus struct {
	Running    bool   `json:"running"`
	StartedAt  string `json:"startedAt,omitempty"`
	Processed  int64  `json:"processed"`
	Normalized int64  `json:"normalized"`
	Skipped    int64  `json:"skipped"`
	Failed     int64  `json:"failed"`
	Message    string `json:"message,omitempty"`
}

func (n *Router) startLibraryLoudness() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !conf.Server.Scanner.LoudnessNormalization.Enabled {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(libraryLoudnessStatus{Message: "Loudness normalization is disabled in the server configuration"})
			return
		}
		if !libraryLoudness.running.CompareAndSwap(false, true) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(currentLibraryLoudnessStatus("LUFS processing is already running"))
			return
		}
		libraryLoudness.startedAt.Store(time.Now().Unix())
		libraryLoudness.processed.Store(0)
		libraryLoudness.normalized.Store(0)
		libraryLoudness.skipped.Store(0)
		libraryLoudness.failed.Store(0)

		go n.runLibraryLoudness(context.Background())

		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(currentLibraryLoudnessStatus("LUFS processing started for the entire library"))
	}
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
	defer libraryLoudness.running.Store(false)

	options := conf.Server.Scanner.LoudnessNormalization
	target := ffmpeg.LoudnessTarget{
		IntegratedLUFS: options.TargetLUFS,
		TruePeak:       options.TruePeak,
		LRA:            options.LRA,
	}
	tolerance := effectiveManualLoudnessTolerance(options.Tolerance)
	normalizer := ffmpeg.NewLoudnessNormalizer()
	repo := n.ds.MediaFile(ctx)

	log.Info(ctx, "LUFS library processing started", "targetLUFS", options.TargetLUFS, "tolerance", tolerance)
	start := time.Now()

	cursor, err := repo.GetCursor()
	if err != nil {
		log.Error(ctx, "LUFS library processing: could not read media files", err)
		return
	}

	for mf, err := range cursor {
		if err != nil {
			log.Error(ctx, "LUFS library processing aborted: error reading media files", err)
			return
		}
		processed := libraryLoudness.processed.Add(1)
		if processed%200 == 0 {
			log.Info(ctx, "LUFS library processing progress", "processed", processed,
				"normalized", libraryLoudness.normalized.Load(), "skipped", libraryLoudness.skipped.Load(),
				"failed", libraryLoudness.failed.Load(), "elapsed", time.Since(start))
		}
		if mf.Missing || strings.TrimSpace(mf.Path) == "" || strings.TrimSpace(mf.LibraryPath) == "" {
			libraryLoudness.skipped.Add(1)
			continue
		}
		// Fast path: stored LUFS tag already within tolerance - no ffmpeg run
		if lufs, ok := storedLoudness(&mf); ok && math.Abs(lufs-options.TargetLUFS) <= tolerance {
			libraryLoudness.skipped.Add(1)
			continue
		}

		trackPath := absoluteSelectedMediaPath(mf.LibraryPath, mf.Path)
		res, err := ffmpeg.NormalizeToBest(ctx, normalizer, trackPath, target, ffmpeg.NormalizeOptions{
			Tolerance:    tolerance,
			MaxAttempts:  maxManualLoudnessNormalizeAttempts,
			Backup:       options.Backup,
			BackupSuffix: options.BackupSuffix,
		})
		if err != nil {
			libraryLoudness.failed.Add(1)
			log.Warn(ctx, "LUFS library processing: could not normalize track", "path", trackPath, err)
			continue
		}
		updateSongLoudnessTag(ctx, repo, mf.ID, res.FinalLUFS)
		if !res.Changed {
			libraryLoudness.skipped.Add(1)
			continue
		}
		libraryLoudness.normalized.Add(1)

		uploadPath := trackPath
		if dest, err := copyTrackToSyncMP3Folder(mf.LibraryPath, trackPath); err != nil {
			log.Warn(ctx, "LUFS library processing: could not copy track to sync folder", "path", trackPath, err)
		} else if dest != "" {
			uploadPath = dest
		}
		if gcsync.IsEligibleLUFS(res.OldLUFS, res.FinalLUFS, options.TargetLUFS) {
			gcsync.GetInstance().EnqueueMP3(uploadPath,
				fmt.Sprintf("LUFS improved: %.2f -> %.2f (target %.2f)", res.OldLUFS, res.FinalLUFS, options.TargetLUFS))
		}
	}

	log.Info(ctx, "LUFS library processing finished", "processed", libraryLoudness.processed.Load(),
		"normalized", libraryLoudness.normalized.Load(), "skipped", libraryLoudness.skipped.Load(),
		"failed", libraryLoudness.failed.Load(), "elapsed", time.Since(start))
}

// storedLoudness reads the LUFS value previously written to the media file's
// tags, if any.
func storedLoudness(mf *model.MediaFile) (float64, bool) {
	for _, name := range []string{"loudnorm_final_lufs", "final_lufs", "finallufs", "lufs"} {
		if values, ok := mf.Tags[model.TagName(name)]; ok && len(values) > 0 {
			if v, err := strconv.ParseFloat(strings.TrimSpace(values[0]), 64); err == nil {
				return v, true
			}
		}
	}
	return 0, false
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
