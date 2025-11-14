package nativeapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

type songCommentRequest struct {
	IDs     []string `json:"ids"`
	Comment *string  `json:"comment"`
}

func setSongComment(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req songCommentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(req.IDs) == 0 {
			http.Error(w, "ids are required", http.StatusBadRequest)
			return
		}

		repo := ds.MediaFile(r.Context())
		if repo == nil {
			http.Error(w, "media repository not available", http.StatusInternalServerError)
			return
		}

		if err := repo.SetComment(req.Comment, req.IDs...); err != nil {
			if errors.Is(err, rest.ErrPermissionDenied) {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}
			log.Error(r.Context(), "Error setting song comments", "ids", req.IDs, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		resp := struct {
			IDs []string `json:"ids"`
		}{IDs: req.IDs}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
