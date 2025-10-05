package nativeapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/deluan/rest"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/core"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

func getDiscovery(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
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

		var builder strings.Builder
		builder.WriteString("#EXTM3U\n")
		for _, track := range disc.Tracks {
			title := track.Title
			if track.Artist != "" {
				title = fmt.Sprintf("%s - %s", track.Artist, track.Title)
			}
			builder.WriteString(fmt.Sprintf("#EXTINF:%d,%s\n", int(track.Duration+0.5), title))
			builder.WriteString(track.Path)
			builder.WriteString("\n")
		}

		if _, err := w.Write([]byte(builder.String())); err != nil {
			log.Error(ctx, "Error exporting discovery", "id", discID, err)
		}
	}
}

func publishDiscovery(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		discID := chi.URLParam(r, "id")
		repo := ds.Discovery(ctx)
		if _, err := repo.Get(discID); err != nil {
			if errors.Is(err, model.ErrNotFound) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), statusFor(err))
			return
		}
		if err := core.NewDiscoveries(ds).Sync(ctx); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
