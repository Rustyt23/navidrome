package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/core/silencetrim"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/slice"
)

type silenceTrimPayload struct {
	IDs        []string `json:"ids"`
	LibraryIDs []int    `json:"libraryIds"`
	All        bool     `json:"all"`
}

type silenceTrimDecisionPayload struct {
	IDs      []string `json:"ids"`
	Decision string   `json:"decision"`
}

func (n *Router) addSongSilenceTrimRoute(r chi.Router) {
	r.Get("/song/silence-trim/settings", n.silenceTrimSettings())
	r.Post("/song/silence-trim/analyze", n.startSilenceTrimAnalyze())
	r.Get("/song/silence-trim/analyze", n.silenceTrimAnalyzeStatusHandler())
	r.Post("/song/silence-trim/analyze/stop", n.stopSilenceTrimAnalyzeHandler())
	r.Delete("/song/silence-trim/analyze/results", n.clearSilenceTrimAnalysis())
	r.Put("/song/silence-trim/decision", n.setSilenceTrimDecision())
	r.Post("/song/silence-trim/apply", n.startSilenceTrimApply())
	r.Get("/song/silence-trim/apply", n.silenceTrimApplyStatusHandler())
	r.Post("/song/silence-trim/apply/stop", n.stopSilenceTrimApplyHandler())
	r.Post("/song/silence-trim/restore", n.restoreSongSilenceTrim())
}

func (n *Router) silenceTrimSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(silencetrim.CurrentSettings()); err != nil {
			log.Error(r.Context(), "Error sending silence-trim settings", err)
		}
	}
}

func validSilenceTrimDecision(decision string) bool {
	switch decision {
	case model.SilenceTrimDecisionPending,
		model.SilenceTrimDecisionApprove,
		model.SilenceTrimDecisionSkip:
		return true
	}
	return false
}

func (n *Router) setSilenceTrimDecision() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload silenceTrimDecisionPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload.IDs = slice.Unique(payload.IDs)
		payload.Decision = strings.TrimSpace(payload.Decision)
		if len(payload.IDs) == 0 {
			http.Error(w, "ids are required", http.StatusBadRequest)
			return
		}
		if !validSilenceTrimDecision(payload.Decision) {
			http.Error(w, "invalid decision", http.StatusBadRequest)
			return
		}
		release, busy := claimLoudnessFileWork(&silenceDecisionRunning)
		if busy != "" {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"message": busy + " is in progress",
			})
			return
		}
		defer release()

		repo := n.ds.SilenceTrimAudit(r.Context())
		updated := 0
		for _, id := range payload.IDs {
			if err := repo.SetDecision(id, payload.Decision); err != nil {
				log.Warn(r.Context(), "Could not save silence-trim decision", "id", id, err)
				continue
			}
			updated++
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"updated":  updated,
			"decision": payload.Decision,
		})
	}
}

func (n *Router) clearSilenceTrimAnalysis() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		release, busy := claimLoudnessFileWork(&silenceClearRunning)
		if busy != "" {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"message": busy + " is in progress",
			})
			return
		}
		defer release()
		count, err := n.ds.SilenceTrimAudit(r.Context()).Clear()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cleared": count,
			"message": "Dry-run analysis cleared; songs, backups, and restore provenance were not touched",
		})
	}
}

func collectSilenceTrimTargets(
	ctx context.Context,
	ds model.DataStore,
	payload silenceTrimPayload,
) ([]model.MediaFile, error) {
	repo := ds.MediaFile(ctx)
	usable := func(mf *model.MediaFile) bool {
		return mf != nil &&
			!mf.Missing &&
			strings.TrimSpace(mf.Path) != "" &&
			strings.TrimSpace(mf.LibraryPath) != ""
	}

	if payload.All {
		libraryIDs := slice.Unique(payload.LibraryIDs)
		if len(libraryIDs) == 0 {
			return nil, nil
		}
		cursor, err := repo.GetCursor(model.QueryOptions{
			Filters: squirrel.Eq{"media_file.library_id": libraryIDs},
		})
		if err != nil {
			return nil, err
		}
		var tracks []model.MediaFile
		for mf, err := range cursor {
			if err != nil {
				return nil, err
			}
			if usable(&mf) {
				tracks = append(tracks, mf)
			}
		}
		return tracks, nil
	}

	var tracks []model.MediaFile
	for _, id := range slice.Unique(payload.IDs) {
		mf, err := repo.Get(id)
		if err != nil {
			log.Warn(ctx, "Could not read silence-trim target", "id", id, err)
			continue
		}
		if usable(mf) {
			tracks = append(tracks, *mf)
		}
	}
	return tracks, nil
}
