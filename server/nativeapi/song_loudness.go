package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/loudness"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/slice"
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
	r.Get("/song/loudness/settings", n.loudnessSettings())
	r.Put("/song/loudness/settings", n.updateLoudnessSettings())
	r.Post("/song/loudness/analyze", n.startLoudnessAnalyze())
	r.Get("/song/loudness/analyze", n.loudnessAnalyzeStatusHandler())
	r.Post("/song/loudness/analyze/stop", n.stopLoudnessAnalyzeHandler())
	r.Delete("/song/loudness/analyze/results", n.clearLoudnessAnalyzeResults())
	r.Post("/song/loudness/library/stop", n.stopLibraryLoudnessHandler())
	r.Put("/song/loudness/decision", n.setLoudnessDecision())
}

// loudnessSettingsResponse describes the current loudness normalization state
// for the LUFS page. Only Enabled is writable from the UI; the remaining
// fields come from the configuration and are shown for reference.
type loudnessSettingsResponse struct {
	Enabled    bool    `json:"enabled"`
	TargetLUFS float64 `json:"targetLUFS"`
	Tolerance  float64 `json:"tolerance"`
	TruePeak   float64 `json:"truePeak"`
	LRA        float64 `json:"lra"`
	Backup     bool    `json:"backup"`
}

type loudnessSettingsPayload struct {
	Enabled *bool `json:"enabled"`
}

func (n *Router) currentLoudnessSettings(ctx context.Context) loudnessSettingsResponse {
	options := conf.Server.Scanner.LoudnessNormalization
	return loudnessSettingsResponse{
		Enabled:    loudness.Enabled(ctx, n.ds),
		TargetLUFS: options.TargetLUFS,
		Tolerance:  effectiveManualLoudnessTolerance(options.Tolerance),
		TruePeak:   options.TruePeak,
		LRA:        options.LRA,
		Backup:     options.Backup,
	}
}

func (n *Router) loudnessSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(n.currentLoudnessSettings(r.Context())); err != nil {
			log.Error(r.Context(), "Error sending loudness settings", err)
		}
	}
}

func (n *Router) updateLoudnessSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var payload loudnessSettingsPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if payload.Enabled == nil {
			http.Error(w, "enabled is required", http.StatusBadRequest)
			return
		}
		if err := loudness.SetEnabled(ctx, n.ds, *payload.Enabled); err != nil {
			log.Error(ctx, "Could not save loudness normalization setting", "enabled", *payload.Enabled, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Turning "Optimise all LUFS" on starts a whole-library run; turning it
		// off stops the run in progress. Optimizing a hand-picked selection is
		// an explicit action and stays available either way.
		if *payload.Enabled {
			started := n.beginLibraryLoudness(ctx, loudness.PhaseGain)
			log.Info(ctx, "Optimise all LUFS turned on", "startedRun", started)
		} else {
			stopLibraryLoudness()
			log.Info(ctx, "Optimise all LUFS turned off")
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(n.currentLoudnessSettings(ctx)); err != nil {
			log.Error(ctx, "Error sending loudness settings", err)
		}
	}
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
	normalizer := ffmpeg.NewLoudnessNormalizer()
	repo := ds.MediaFile(ctx)

	response := songLoudnessResponse{IDs: ids}
	for _, id := range ids {
		result := songLoudnessResult{ID: id}
		fail := func(msg string) {
			result.Status = "failed"
			result.Error = msg
			response.Failed = append(response.Failed, id)
			response.Results = append(response.Results, result)
		}

		mf, err := repo.Get(id)
		if err != nil {
			fail(err.Error())
			continue
		}
		if mf.Path == "" || mf.LibraryPath == "" || mf.Missing {
			fail("song file is missing")
			continue
		}

		res, err := optimizeOneTrack(ctx, ds, normalizer, mf)
		if err != nil {
			log.Warn(ctx, "Could not optimize selected song loudness", "id", id, err)
			fail(err.Error())
			continue
		}
		result.Before = &res.OldLUFS
		result.After = &res.NewLUFS
		result.UpdatedDB = true

		if !res.Changed {
			result.Status = "skipped"
			switch {
			case res.Phase == loudness.PhaseReview:
				result.Error = "not enough headroom: review it on the LUFS 2 page"
			case res.Rejected != "":
				result.Error = res.Rejected
			}
			response.Skipped = append(response.Skipped, id)
			response.Results = append(response.Results, result)
			continue
		}

		log.Info(ctx, "Optimized selected song loudness", "id", id, "fromLUFS", res.OldLUFS,
			"finalLUFS", res.NewLUFS, "gain", res.GainDB, "phase", res.Phase)
		result.Status = "normalized"
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
