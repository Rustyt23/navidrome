package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/loudness"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/slice"
)

type songLoudnessPayload struct {
	IDs []string `json:"ids"`
}

func (n *Router) addSongLoudnessRoute(r chi.Router) {
	r.Put("/song/loudness", n.optimizeSongLoudness())
	r.Post("/song/loudness/restore", n.restoreSongLoudness())
	// Not a loudness route, but it lives on the same selection toolbars and
	// there is nowhere better for it yet.
	r.Get("/song/download", n.downloadSongs())
	r.Get("/song/loudness/restore", n.restoreLoudnessStatusHandler())
	r.Post("/song/loudness/restore/stop", n.stopRestoreLoudnessHandler())
	r.Get("/song/loudness/backups", n.loudnessBackupReport())
	r.Post("/song/loudness/backups/cleanup", n.cleanupLoudnessBackups())
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
	r.Get("/song/loudness/summary", n.loudnessSummaryHandler())
	r.Get("/song/loudness/db", n.loudnessAuditDbList())
	r.Post("/song/loudness/db", n.loudnessAuditDbSnapshot())
	r.Post("/song/loudness/db/restore", n.loudnessAuditDbRestore())
}

// loudnessSettingsResponse describes the current loudness normalization state
// for the LUFS page. Enabled and Backup are writable from the UI; the remaining
// fields come from the configuration and are shown for reference.
type loudnessSettingsResponse struct {
	Enabled    bool    `json:"enabled"`
	TargetLUFS float64 `json:"targetLUFS"`
	Tolerance  float64 `json:"tolerance"`
	TruePeak   float64 `json:"truePeak"`
	LRA        float64 `json:"lra"`
	Backup     bool    `json:"backup"`
}

// Both fields are optional so each toggle can be set without disturbing the
// other. A request that sets neither is rejected rather than silently accepted.
type loudnessSettingsPayload struct {
	Enabled *bool `json:"enabled"`
	Backup  *bool `json:"backup"`
}

func (n *Router) currentLoudnessSettings(ctx context.Context) loudnessSettingsResponse {
	options := conf.Server.Scanner.LoudnessNormalization
	return loudnessSettingsResponse{
		Enabled:    loudness.Enabled(ctx, n.ds),
		TargetLUFS: options.TargetLUFS,
		Tolerance:  effectiveManualLoudnessTolerance(options.Tolerance),
		TruePeak:   options.TruePeak,
		LRA:        options.LRA,
		Backup:     loudness.BackupEnabled(ctx, n.ds),
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
		if payload.Enabled == nil && payload.Backup == nil {
			http.Error(w, "enabled or backup is required", http.StatusBadRequest)
			return
		}

		// Whether originals are kept decides what happens to files the moment
		// the next track is opened, and it cannot be applied to work already
		// done. Changing it underneath a run would leave one half of that run
		// restorable and the other half not, with nothing in the record saying
		// where the line falls.
		if payload.Backup != nil {
			if libraryLoudness.running.Load() {
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"message": "Stop the optimisation before changing whether originals are kept",
				})
				return
			}
			if err := loudness.SetBackupEnabled(ctx, n.ds, *payload.Backup); err != nil {
				log.Error(ctx, "Could not save loudness backup setting", "backup", *payload.Backup, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			log.Warn(ctx, "Keep original before optimising changed", "backup", *payload.Backup)
		}

		if payload.Enabled != nil {
			if err := loudness.SetEnabled(ctx, n.ds, *payload.Enabled); err != nil {
				log.Error(ctx, "Could not save loudness normalization setting", "enabled", *payload.Enabled, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Turning "Optimise all LUFS" on starts a whole-library run; turning
			// it off stops the run in progress. Optimizing a hand-picked
			// selection is an explicit action and stays available either way.
			if *payload.Enabled {
				if !n.beginLibraryLoudness(ctx, loudness.PhaseGain, nil) {
					// Nothing started, so nothing will turn the setting off
					// again - the run's own ending is what does that. Saying
					// "on" here would leave the switch claiming a run that does
					// not exist, and stay that way for ever.
					busy := loudnessFileWorkBusy()
					if busy == "" {
						busy = "another LUFS job"
					}
					if err := loudness.SetEnabled(ctx, n.ds, false); err != nil {
						log.Error(ctx, "Could not undo 'Optimise all LUFS' after a refused start", err)
					}
					log.Warn(ctx, "Optimise all LUFS could not start", "busy", busy)
					w.WriteHeader(http.StatusConflict)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"message": busy + " is already running - wait for it to finish",
					})
					return
				}
				log.Info(ctx, "Optimise all LUFS turned on, run started")
			} else {
				stopLibraryLoudness()
				log.Info(ctx, "Optimise all LUFS turned off")
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(n.currentLoudnessSettings(ctx)); err != nil {
			log.Error(ctx, "Error sending loudness settings", err)
		}
	}
}

// optimizeSongLoudness optimises a hand-picked selection.
//
// It starts the same background run the whole-library sweep uses, scoped to the
// chosen ids, and returns as soon as the run is going. It used to do the work
// inside the request instead: one song at a time, with no progress, no way to
// stop it, and a request that stayed open until the last song was finished -
// which on a large selection outlives the browser's patience while the server
// carries on working.
func (n *Router) optimizeSongLoudness() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		if !n.beginLibraryLoudness(r.Context(), loudness.PhaseGain, ids) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(currentLibraryLoudnessStatus("LUFS processing is already running"))
			return
		}

		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(currentLibraryLoudnessStatus(
			fmt.Sprintf("Optimising %d selected %s", len(ids), pluralSongs(len(ids)))))
	}
}

func pluralSongs(n int) string {
	if n == 1 {
		return "song"
	}
	return "songs"
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
