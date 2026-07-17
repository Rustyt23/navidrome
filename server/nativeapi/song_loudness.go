package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/gcsync"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/slice"
)

const (
	maxManualLoudnessNormalizeAttempts = 3
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
	r.Post("/song/loudness/library", n.startLibraryLoudness())
	r.Get("/song/loudness/library", n.libraryLoudnessStatusHandler())
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

		res, err := ffmpeg.NormalizeToBest(ctx, normalizer, trackPath, target, ffmpeg.NormalizeOptions{
			Tolerance:    tolerance,
			MaxAttempts:  maxManualLoudnessNormalizeAttempts,
			Backup:       options.Backup,
			BackupSuffix: options.BackupSuffix,
		})
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			response.Failed = append(response.Failed, id)
			response.Results = append(response.Results, result)
			log.Warn(ctx, "Could not optimize selected song loudness", "id", id, "path", trackPath, err)
			continue
		}
		result.Before = &res.OldLUFS
		result.After = &res.FinalLUFS

		if !res.Changed {
			// Either already in range, or no attempt improved on the original
			// (best-result guarantee keeps the file untouched)
			result.Status = "skipped"
			if !res.InRange {
				result.Error = fmt.Sprintf("could not get closer to target than current %.2f LUFS; kept original", res.FinalLUFS)
				log.Warn(ctx, "Loudness optimization could not improve song, kept original", "id", id, "path", trackPath, "lufs", res.FinalLUFS, "targetLUFS", options.TargetLUFS, "attempts", res.Attempts)
			}
			result.UpdatedDB = updateSongLoudnessTag(ctx, repo, id, res.FinalLUFS)
			response.Skipped = append(response.Skipped, id)
			response.Results = append(response.Results, result)
			continue
		}

		log.Info(ctx, "Optimized selected song loudness", "id", id, "path", trackPath, "fromLUFS", res.OldLUFS, "finalLUFS", res.FinalLUFS, "targetLUFS", options.TargetLUFS, "minLUFS", minLUFS, "maxLUFS", maxLUFS, "attempts", res.Attempts, "inRange", res.InRange)
		result.Status = "normalized"
		result.UpdatedDB = updateSongLoudnessTag(ctx, repo, id, res.FinalLUFS)
		// Only overwrite the bucket copy when the new loudness is closer to
		// the target than the old one
		if gcsync.IsEligibleLUFS(res.OldLUFS, res.FinalLUFS, options.TargetLUFS) {
			gcsync.GetInstance().EnqueueMP3(trackPath,
				fmt.Sprintf("LUFS optimized: %.2f -> %.2f (target %.2f)", res.OldLUFS, res.FinalLUFS, options.TargetLUFS))
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

func effectiveManualLoudnessTolerance(tolerance float64) float64 {
	if tolerance <= 0 {
		return conf.DefaultLoudnessNormalizationTolerance
	}
	return tolerance
}
func absoluteSelectedMediaPath(libraryPath, mediaPath string) string {
	trackPath := filepath.FromSlash(mediaPath)
	if !filepath.IsAbs(trackPath) {
		trackPath = filepath.Join(libraryPath, trackPath)
	}
	return filepath.Clean(trackPath)
}
