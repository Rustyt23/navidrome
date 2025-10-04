package nativeapi

import (
        "net/http"

        "github.com/deluan/rest"
        "github.com/go-chi/chi/v5"
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
		discID := chi.URLParam(r, "discoveryId")
		repo := ds.Discovery(r.Context()).Tracks(discID)
		tracks, err := repo.GetAll()
		if err != nil {
			http.Error(w, err.Error(), statusFor(err))
			return
		}
		rest.RespondWithJSON(w, http.StatusOK, tracks)
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
