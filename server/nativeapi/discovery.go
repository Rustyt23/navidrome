package nativeapi

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/deluan/rest"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/core"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/req"
)

func getDiscovery(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "discoveryId")
		if id == "" {
			id = chi.URLParam(r, "id")
		}
		disc, err := ds.Discovery(r.Context()).GetWithTracks(id)
		if err != nil {
			http.Error(w, err.Error(), statusFor(err))
			return
		}
		rest.RespondWithJSON(w, http.StatusOK, disc)
	}
}

func getDiscoveryTracks(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Header.Get("accept"), "audio/x-mpegurl") {
			exportDiscovery(ds)(w, r)
			return
		}
		discID := chi.URLParam(r, "discoveryId")
		if discID == "" {
			discID = chi.URLParam(r, "id")
		}
		repo := ds.Discovery(r.Context()).Tracks(discID)
		controller := rest.Controller{Repository: repo}
		controller.GetAll(w, r)
	}
}

func getSongDiscoveries(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		trackID := chi.URLParam(r, "id")
		discs, err := ds.Discovery(r.Context()).GetDiscoveries(trackID)
		if err != nil {
			log.Error(r.Context(), "Error getting song discoveries", "songId", trackID, err)
			http.Error(w, err.Error(), statusFor(err))
			return
		}
		rest.RespondWithJSON(w, http.StatusOK, discs)
	}
}

func exportDiscovery(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		discID := chi.URLParam(r, "discoveryId")
		if discID == "" {
			discID = chi.URLParam(r, "id")
		}
		disc, err := ds.Discovery(ctx).GetWithTracks(discID)
		if errors.Is(err, model.ErrNotFound) {
			log.Warn(ctx, "Discovery not found", "id", discID)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Error(ctx, "Error retrieving discovery", "id", discID, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "audio/x-mpegurl")
		disposition := fmt.Sprintf("attachment; filename=\"%s.m3u\"", disc.Name)
		w.Header().Set("Content-Disposition", disposition)

		if _, err := w.Write([]byte(disc.ToM3U8())); err != nil {
			log.Error(ctx, "Error exporting discovery", "id", discID, err)
		}
	}
}

func publishDiscovery(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		discID := chi.URLParam(r, "id")
		if discID == "" {
			discID = chi.URLParam(r, "discoveryId")
		}
		discoveries := core.NewDiscoveries(ds)
		if err := discoveries.Publish(ctx, discID); err != nil {
			switch {
			case errors.Is(err, model.ErrNotFound):
				http.Error(w, "not found", http.StatusNotFound)
			case errors.Is(err, os.ErrNotExist):
				http.Error(w, "not found", http.StatusNotFound)
			default:
				http.Error(w, err.Error(), statusFor(err))
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func syncDiscoveries(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		discoveries := core.NewDiscoveries(ds)
		if err := discoveries.Sync(ctx); err != nil {
			log.Error(ctx, "Error syncing discoveries", err)
			http.Error(w, err.Error(), statusFor(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteDiscoveryTracks(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		params := req.Params(r)
		discID, _ := params.String(":discoveryId")
		if discID == "" {
			discID = chi.URLParam(r, "discoveryId")
		}
		if discID == "" {
			discID = chi.URLParam(r, "id")
		}
		ids, _ := params.Strings("id")
		if len(ids) == 0 {
			http.Error(w, "no tracks selected", http.StatusBadRequest)
			return
		}

		err := ds.WithTxImmediate(func(tx model.DataStore) error {
			repo := tx.Discovery(ctx).Tracks(discID)
			return repo.Delete(ids...)
		})
		if len(ids) == 1 && errors.Is(err, model.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Error(ctx, "Error deleting discovery tracks", "id", discID, "trackIds", ids, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeDeleteManyResponse(w, r, ids)
	}
}
