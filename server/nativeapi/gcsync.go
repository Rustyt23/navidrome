package nativeapi

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/gcsync"
	"github.com/navidrome/navidrome/log"
)

// addGCSyncRoute exposes a manual trigger for the GCS sync sweep (admin only).
func (api *Router) addGCSyncRoute(r chi.Router) {
	r.Post("/gcsync/sweep", func(w http.ResponseWriter, req *http.Request) {
		if !conf.Server.GCSync.Enabled {
			http.Error(w, `{"error":"GCSync is disabled"}`, http.StatusServiceUnavailable)
			return
		}
		go func() {
			if err := gcsync.GetInstance().Sweep(context.Background()); err != nil {
				log.Error("GCSync: manual sweep failed", err)
			}
		}()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"sweep started"}`))
	})
}
