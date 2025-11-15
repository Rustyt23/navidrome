package nativeapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/slice"
)

type playlistCommentPayload struct {
	IDs     []string `json:"ids"`
	Comment string   `json:"comment"`
}

func updatePlaylistComments(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var payload playlistCommentPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ids := slice.Unique(payload.IDs)
		if len(ids) == 0 {
			http.Error(w, "ids are required", http.StatusBadRequest)
			return
		}

		repo := ds.Playlist(ctx)
		if err := repo.UpdateComment(ids, payload.Comment); err != nil {
			switch {
			case errors.Is(err, rest.ErrPermissionDenied):
				http.Error(w, "Access denied", http.StatusForbidden)
			default:
				log.Error(ctx, "Error updating playlist comments", "ids", ids, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		response := struct {
			Ids []string `json:"ids"`
		}{Ids: ids}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Error(ctx, "Error sending playlist comment response", err)
		}
	}
}
