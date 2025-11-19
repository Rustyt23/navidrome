package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/deluan/rest"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/server"
)

func (n *Router) addSongRoutes(r chi.Router) {
	constructor := func(ctx context.Context) rest.Repository {
		return n.ds.Resource(ctx, model.MediaFile{})
	}

	r.Route("/song", func(r chi.Router) {
		r.Get("/", rest.GetAll(constructor))
		r.Route("/{id}", func(r chi.Router) {
			r.Use(server.URLParamsMiddleware)
			r.Get("/", rest.Get(constructor))
			r.Put("/", n.handleUpdateSong())
		})
	})
}

type updateSongRequest struct {
	Comment *string `json:"comment"`
}

func (n *Router) handleUpdateSong() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := chi.URLParam(r, "id")
		if id == "" {
			rest.RespondWithError(w, http.StatusBadRequest, "missing song id")
			return
		}

		user, ok := request.UserFrom(ctx)
		if !ok || !user.IsAdmin {
			rest.RespondWithError(w, http.StatusForbidden, "Permission denied")
			return
		}

		var payload updateSongRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			rest.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}

		if payload.Comment == nil {
			rest.RespondWithError(w, http.StatusBadRequest, "comment is required")
			return
		}

		var updated *model.MediaFile
		err := n.ds.WithTxImmediate(func(tx model.DataStore) error {
			repo := tx.MediaFile(ctx)
			if err := repo.UpdateComment(id, *payload.Comment); err != nil {
				return err
			}
			var err error
			updated, err = repo.Get(id)
			return err
		})
		if errors.Is(err, rest.ErrPermissionDenied) {
			rest.RespondWithError(w, http.StatusForbidden, "Permission denied")
			return
		}
		if errors.Is(err, model.ErrNotFound) {
			rest.RespondWithError(w, http.StatusNotFound, "Song not found")
			return
		}
		if err != nil {
			log.Error(ctx, "Error updating song", "id", id, err)
			rest.RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		rest.RespondWithJSON(w, http.StatusOK, updated)
	}
}
