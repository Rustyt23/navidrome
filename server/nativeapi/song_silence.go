package nativeapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/core/silence"
	"github.com/navidrome/navidrome/log"
)

// addSongSilenceRoute mounts the silence-trim endpoints.
//
// Entirely separate from the loudness routes: the two features share the media
// files and nothing else, and a request to one must never be able to disturb
// the other's state.
func (n *Router) addSongSilenceRoute(r chi.Router) {
	r.Post("/song/silence/analyze", n.startSilenceAnalyze())
	r.Get("/song/silence/analyze", n.silenceAnalyzeStatusHandler())
	r.Post("/song/silence/analyze/stop", n.stopSilenceAnalyzeHandler())
	r.Delete("/song/silence/analyze/results", n.clearSilenceResults())

	r.Post("/song/silence/trim", n.startSilenceTrim())
	r.Get("/song/silence/trim", n.silenceTrimStatusHandler())
	r.Post("/song/silence/trim/stop", n.stopSilenceTrimHandler())

	r.Get("/song/silence/summary", n.silenceSummaryHandler())
	r.Get("/song/silence/settings", n.silenceSettings())
}

// songSilencePayload carries an explicit selection of songs.
type songSilencePayload struct {
	IDs []string `json:"ids"`
}

func decodeSilenceSelection(r *http.Request) []string {
	var payload songSilencePayload
	if r.Body == nil {
		return nil
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		// A body-less request means "the whole library", which is a normal way
		// to call this rather than an error.
		return nil
	}
	return payload.IDs
}

// silenceSettingsResponse describes the rules a run works under. Read-only:
// these are the guards, and changing them from the page would be changing what
// "safe" means from the same screen that reports safety.
type silenceSettingsResponse struct {
	MarginSeconds    float64 `json:"marginSeconds"`
	ThresholdDB      float64 `json:"thresholdDB"`
	OnsetThresholdDB float64 `json:"onsetThresholdDB"`
	MaxOnsetGap      float64 `json:"maxOnsetGap"`
	MaxTrimSeconds   float64 `json:"maxTrimSeconds"`
}

func (n *Router) silenceSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := silenceSettingsResponse{
			MarginSeconds:    silence.DefaultMarginSeconds,
			ThresholdDB:      silenceThresholdDB,
			OnsetThresholdDB: silenceOnsetThresholdDB,
			MaxOnsetGap:      silence.MaxOnsetGapSeconds,
			MaxTrimSeconds:   silence.MaxTrimSeconds,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Error(r.Context(), "Error sending silence settings", err)
		}
	}
}
