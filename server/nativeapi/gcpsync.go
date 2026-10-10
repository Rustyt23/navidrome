package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/gcpsync"
	"github.com/navidrome/navidrome/log"
)

// addGCPSyncRoute exposes a manual trigger for the GCS sync sweep (admin only).
func (api *Router) addGCPSyncRoute(r chi.Router) {
	r.Post("/gcpsync/sweep", func(w http.ResponseWriter, req *http.Request) {
		if reason := syncUnavailableReason(); reason != "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": reason})
			return
		}
		go func() {
			if err := api.gcpSyncService().Sweep(context.Background()); err != nil {
				log.Error("GCPSync: manual sweep failed", err)
			}
		}()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"sweep started"}`))
	})
}

func (api *Router) gcpSyncService() gcpsync.Service {
	if api.gcpSync != nil {
		return api.gcpSync
	}
	return gcpsync.GetInstance()
}

func syncUnavailableReason() string {
	if !conf.Server.GCPSync.Enabled {
		return "GCS sync is disabled (set GCPSync.Enabled in the config)"
	}
	if strings.TrimSpace(conf.Server.GCPSync.Bucket) == "" {
		return "GCS sync needs GCPSync.Bucket in the config"
	}
	if strings.TrimSpace(conf.Server.SyncFolder) == "" {
		return "GCS sync needs SyncFolder in the config"
	}
	return ""
}
