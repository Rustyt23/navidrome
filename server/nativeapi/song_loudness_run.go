package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/loudness"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// These wrappers keep the native jobs on the same processing path as automatic imports.
func loudnessRunOptions(ctx context.Context, ds model.DataStore, mf *model.MediaFile) loudness.OptimizeOptions {
	return loudness.RunOptions(ctx, ds, mf)
}

func optimizeOneTrack(ctx context.Context, ds model.DataStore, normalizer ffmpeg.LoudnessNormalizer,
	mf *model.MediaFile) (loudness.OptimizeResult, error) {
	return loudness.OptimizeTrack(ctx, ds, normalizer, mf)
}

type loudnessDecisionPayload struct {
	IDs      []string `json:"ids"`
	Decision string   `json:"decision"`
}

func validLoudnessDecision(d string) bool {
	switch d {
	case loudness.DecisionPending, loudness.DecisionLimit, loudness.DecisionCeiling, loudness.DecisionSkip:
		return true
	}
	return false
}

// setLoudnessDecision records the client's choice for phase 2 tracks. It only
// stores the intent; nothing is applied until a phase 2 run is started.
func (n *Router) setLoudnessDecision() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var payload loudnessDecisionPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload.Decision = strings.TrimSpace(payload.Decision)
		if !validLoudnessDecision(payload.Decision) {
			http.Error(w, "invalid decision", http.StatusBadRequest)
			return
		}
		if len(payload.IDs) == 0 {
			http.Error(w, "ids are required", http.StatusBadRequest)
			return
		}

		repo := n.ds.LoudnessAudit(ctx)
		updated := 0
		for _, id := range payload.IDs {
			if err := repo.SetDecision(id, payload.Decision); err != nil {
				log.Warn(ctx, "Could not save loudness decision", "id", id, err)
				continue
			}
			updated++
		}
		log.Info(ctx, "Loudness decisions saved", "decision", payload.Decision, "tracks", updated)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"updated": updated, "decision": payload.Decision})
	}
}
