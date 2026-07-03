package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/gcsync"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/slice"
)

const (
	maxManualLoudnessNormalizeAttempts = 3
	closeManualLoudnessMissLUFS        = 1.0
	minManualLoudnessImprovementLUFS   = 0.02
)

type songLoudnessPayload struct {
	IDs []string `json:"ids"`
}

type songLoudnessResult struct {
	ID        string   `json:"id"`
	Status    string   `json:"status"`
	Before    *float64 `json:"before,omitempty"`
	After     *float64 `json:"after,omitempty"`
	Error     string   `json:"error,omitempty"`
	UpdatedDB bool     `json:"updatedDb,omitempty"`
}

type songLoudnessResponse struct {
	IDs        []string             `json:"ids"`
	Normalized []string             `json:"normalized"`
	Skipped    []string             `json:"skipped"`
	Failed     []string             `json:"failed"`
	Results    []songLoudnessResult `json:"results"`
}

func (n *Router) addSongLoudnessRoute(r chi.Router) {
	r.Put("/song/loudness", n.optimizeSongLoudness())
}

func (n *Router) optimizeSongLoudness() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

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

		response := optimizeSelectedSongLoudness(ctx, n.ds, ids)
		status := http.StatusOK
		if len(response.Normalized) == 0 && len(response.Skipped) == 0 && len(response.Failed) > 0 {
			status = http.StatusInternalServerError
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Error(ctx, "Error sending song loudness response", err)
		}
	}
}

func optimizeSelectedSongLoudness(ctx context.Context, ds model.DataStore, ids []string) songLoudnessResponse {
	options := conf.Server.Scanner.LoudnessNormalization
	target := ffmpeg.LoudnessTarget{
		IntegratedLUFS: options.TargetLUFS,
		TruePeak:       options.TruePeak,
		LRA:            options.LRA,
	}
	tolerance := effectiveManualLoudnessTolerance(options.Tolerance)
	minLUFS := options.TargetLUFS - tolerance
	maxLUFS := options.TargetLUFS + tolerance
	normalizer := ffmpeg.NewLoudnessNormalizer()
	repo := ds.MediaFile(ctx)

	response := songLoudnessResponse{IDs: ids}
	for _, id := range ids {
		result := songLoudnessResult{ID: id}
		mf, err := repo.Get(id)
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			response.Failed = append(response.Failed, id)
			response.Results = append(response.Results, result)
			continue
		}
		trackPath := absoluteSelectedMediaPath(mf.LibraryPath, mf.Path)
		if mf.Path == "" || mf.LibraryPath == "" || mf.Missing {
			result.Status = "failed"
			result.Error = "song file is missing"
			response.Failed = append(response.Failed, id)
			response.Results = append(response.Results, result)
			continue
		}

		analysis, err := normalizer.AnalyzeLoudness(ctx, trackPath, target)
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			response.Failed = append(response.Failed, id)
			response.Results = append(response.Results, result)
			log.Warn(ctx, "Could not analyze selected song loudness", "id", id, "path", trackPath, err)
			continue
		}
		result.Before = &analysis.InputIntegrated

		if !shouldNormalizeManualLoudness(analysis.InputIntegrated, options.TargetLUFS, tolerance) {
			result.Status = "skipped"
			result.After = &analysis.InputIntegrated
			result.UpdatedDB = updateSongLoudnessTag(ctx, repo, id, analysis.InputIntegrated)
			response.Skipped = append(response.Skipped, id)
			response.Results = append(response.Results, result)
			continue
		}

		finalLUFS, err := normalizeSelectedTrackLoudness(ctx, normalizer, trackPath, target, *analysis, tolerance, minLUFS, maxLUFS, options.Backup, options.BackupSuffix)
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			response.Failed = append(response.Failed, id)
			response.Results = append(response.Results, result)
			log.Warn(ctx, "Could not optimize selected song loudness", "id", id, "path", trackPath, "lufs", analysis.InputIntegrated, err)
			continue
		}
		result.Status = "normalized"
		result.After = &finalLUFS
		result.UpdatedDB = updateSongLoudnessTag(ctx, repo, id, finalLUFS)
		// Only overwrite the bucket copy when the new loudness is closer to
		// the target than the old one
		if gcsync.IsEligibleLUFS(*result.Before, finalLUFS, options.TargetLUFS) {
			gcsync.GetInstance().EnqueueMP3(trackPath,
				fmt.Sprintf("LUFS optimized: %.2f -> %.2f (target %.2f)", *result.Before, finalLUFS, options.TargetLUFS))
		}
		response.Normalized = append(response.Normalized, id)
		response.Results = append(response.Results, result)
	}
	return response
}

func updateSongLoudnessTag(ctx context.Context, repo model.MediaFileRepository, id string, lufs float64) bool {
	if err := repo.UpdateLoudnessTags(id, lufs); err != nil {
		log.Warn(ctx, "Could not update selected song LUFS tag", "id", id, "lufs", lufs, err)
		return false
	}
	return true
}

func normalizeSelectedTrackLoudness(ctx context.Context, normalizer ffmpeg.LoudnessNormalizer, trackPath string, target ffmpeg.LoudnessTarget, analysis ffmpeg.LoudnessAnalysis, tolerance, minLUFS, maxLUFS float64, backup bool, backupSuffix string) (float64, error) {
	fromLUFS := analysis.InputIntegrated
	attemptAnalysis := analysis
	attemptTarget := target
	previousDistance := math.Abs(analysis.InputIntegrated - target.IntegratedLUFS)
	if math.Abs(target.IntegratedLUFS-analysis.InputIntegrated) <= closeManualLoudnessMissLUFS {
		var err error
		attemptTarget = adjustedManualLoudnessTarget(target, analysis.InputIntegrated, minLUFS, maxLUFS)
		attemptAnalysis, err = analyzeSelectedLoudnessForTarget(ctx, normalizer, trackPath, target, attemptTarget, 1)
		if err != nil {
			return 0, err
		}
	}

	for attempt := 1; attempt <= maxManualLoudnessNormalizeAttempts; attempt++ {
		if err := writeNormalizedTrackLoudness(ctx, normalizer, trackPath, attemptTarget, attemptAnalysis, backup, backupSuffix); err != nil {
			return 0, err
		}

		finalAnalysis, err := normalizer.AnalyzeLoudness(ctx, trackPath, target)
		if err != nil {
			return 0, fmt.Errorf("normalized track loudness but could not verify final LUFS: %w", err)
		}
		log.Info(ctx, "Optimized selected song loudness", "path", trackPath, "fromLUFS", fromLUFS, "finalLUFS", finalAnalysis.InputIntegrated, "targetLUFS", target.IntegratedLUFS, "minLUFS", minLUFS, "maxLUFS", maxLUFS, "attempt", attempt)
		if !shouldNormalizeManualLoudness(finalAnalysis.InputIntegrated, target.IntegratedLUFS, tolerance) {
			return finalAnalysis.InputIntegrated, nil
		}

		currentDistance := math.Abs(finalAnalysis.InputIntegrated - target.IntegratedLUFS)
		if isAdjustedManualLoudnessTarget(target, attemptTarget) && currentDistance > previousDistance-minManualLoudnessImprovementLUFS {
			return finalAnalysis.InputIntegrated, nil
		}
		if attempt == maxManualLoudnessNormalizeAttempts {
			return finalAnalysis.InputIntegrated, nil
		}

		previousDistance = currentDistance
		attemptTarget = adjustedManualLoudnessTarget(target, finalAnalysis.InputIntegrated, minLUFS, maxLUFS)
		attemptAnalysis, err = analyzeSelectedLoudnessForTarget(ctx, normalizer, trackPath, target, attemptTarget, attempt+1)
		if err != nil {
			return 0, err
		}
	}

	return attemptAnalysis.InputIntegrated, nil
}

func effectiveManualLoudnessTolerance(tolerance float64) float64 {
	if tolerance <= 0 {
		return conf.DefaultLoudnessNormalizationTolerance
	}
	return tolerance
}

func adjustedManualLoudnessTarget(target ffmpeg.LoudnessTarget, measuredLUFS, minLUFS, maxLUFS float64) ffmpeg.LoudnessTarget {
	target.IntegratedLUFS += target.IntegratedLUFS - measuredLUFS
	target.IntegratedLUFS = min(max(target.IntegratedLUFS, minLUFS), maxLUFS)
	return target
}

func isAdjustedManualLoudnessTarget(target, attemptTarget ffmpeg.LoudnessTarget) bool {
	return attemptTarget.IntegratedLUFS != target.IntegratedLUFS
}

func analyzeSelectedLoudnessForTarget(ctx context.Context, normalizer ffmpeg.LoudnessNormalizer, trackPath string, target, attemptTarget ffmpeg.LoudnessTarget, attempt int) (ffmpeg.LoudnessAnalysis, error) {
	analysis, err := normalizer.AnalyzeLoudness(ctx, trackPath, attemptTarget)
	if err != nil {
		log.Warn(ctx, "Could not analyze selected song loudness for adjusted target", "path", trackPath, "targetLUFS", target.IntegratedLUFS, "attemptTargetLUFS", attemptTarget.IntegratedLUFS, "attempt", attempt, err)
		return ffmpeg.LoudnessAnalysis{}, err
	}
	return *analysis, nil
}

func shouldNormalizeManualLoudness(lufs, targetLUFS, tolerance float64) bool {
	return math.Abs(lufs-targetLUFS) > tolerance
}

func absoluteSelectedMediaPath(libraryPath, mediaPath string) string {
	trackPath := filepath.FromSlash(mediaPath)
	if !filepath.IsAbs(trackPath) {
		trackPath = filepath.Join(libraryPath, trackPath)
	}
	return filepath.Clean(trackPath)
}

func writeNormalizedTrackLoudness(ctx context.Context, normalizer ffmpeg.LoudnessNormalizer, trackPath string, target ffmpeg.LoudnessTarget, analysis ffmpeg.LoudnessAnalysis, backup bool, backupSuffix string) error {
	stat, err := os.Stat(trackPath)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(trackPath), "."+trimManualExt(filepath.Base(trackPath))+".loudnorm-*.tmp"+filepath.Ext(trackPath))
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Remove(tmpPath); err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	if err := normalizer.NormalizeLoudness(ctx, trackPath, tmpPath, target, analysis); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, stat.Mode()); err != nil {
		return err
	}
	if backup {
		if backupSuffix == "" {
			backupSuffix = ".before_loudnorm"
		}
		backupPath := trackPath + backupSuffix
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			if err := copyManualLoudnessFile(trackPath, backupPath, stat); err != nil {
				return fmt.Errorf("creating loudness backup: %w", err)
			}
		} else if err != nil {
			return err
		}
	}
	return os.Rename(tmpPath, trackPath)
}

func copyManualLoudnessFile(srcPath, dstPath string, stat os.FileInfo) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, stat.Mode())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		_ = os.Remove(dstPath)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dstPath)
		return closeErr
	}
	return os.Chtimes(dstPath, stat.ModTime(), stat.ModTime())
}

func trimManualExt(name string) string {
	return name[:len(name)-len(filepath.Ext(name))]
}
