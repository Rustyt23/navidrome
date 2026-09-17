package nativeapi

import (
	"encoding/json"
	"net/http"

	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// silenceSummary is the headline the page opens with.
//
// Counted in the database rather than over the page of rows on screen, so it
// describes the library instead of whatever happens to be listed - which is the
// difference between "142 songs have silence to remove" and "3 of the 20 rows
// you can see do".
type silenceSummary struct {
	Analyzed  int64 `json:"analyzed"`
	Trimmable int64 `json:"trimmable"`
	Clean     int64 `json:"clean"`
	Skipped   int64 `json:"skipped"`
	Failed    int64 `json:"failed"`
	// PendingSeconds is the total waiting to be removed across every trimmable
	// song not yet cut. This is the number the client asked the page to answer.
	PendingSeconds float64 `json:"pendingSeconds"`
}

func (n *Router) silenceSummaryHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo := n.ds.SilenceAudit(r.Context())

		counts, err := repo.CountByVerdict()
		if err != nil {
			log.Error(r.Context(), "Could not count silence verdicts", err)
			http.Error(w, "Could not read the silence summary", http.StatusInternalServerError)
			return
		}
		pending, err := repo.PendingTrimSeconds()
		if err != nil {
			log.Error(r.Context(), "Could not total the pending silence trim", err)
			http.Error(w, "Could not read the silence summary", http.StatusInternalServerError)
			return
		}

		summary := silenceSummary{
			Trimmable:      counts[model.SilenceVerdictTrimmable],
			Clean:          counts[model.SilenceVerdictClean],
			Skipped:        counts[model.SilenceVerdictSkipped],
			Failed:         counts[model.SilenceVerdictFailed],
			PendingSeconds: pending,
		}
		for _, count := range counts {
			summary.Analyzed += count
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(summary); err != nil {
			log.Error(r.Context(), "Error sending the silence summary", err)
		}
	}
}
