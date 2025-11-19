package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/deluan/rest"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/server"
)

func (n *Router) addSongRoute(r chi.Router) {
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
			http.Error(w, "missing song id", http.StatusBadRequest)
			return
		}

		var payload updateSongRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if payload.Comment == nil {
			http.Error(w, "comment is required", http.StatusBadRequest)
			return
		}

		var updated *model.MediaFile
		err := n.ds.WithTxImmediate(func(tx model.DataStore) error {
			repo := tx.MediaFile(ctx)
			mediaFile, err := repo.Get(id)
			if err != nil {
				return err
			}

			if n.libs != nil {
				if user, ok := request.UserFrom(ctx); ok {
					if err := n.libs.ValidateLibraryAccess(ctx, user.ID, mediaFile.LibraryID); err != nil {
						return err
					}
				}
			}

			mediaFile.Comment = *payload.Comment
			mediaFile.UpdatedAt = time.Now()

			if err := repo.Put(mediaFile); err != nil {
				return err
			}

			if err := n.tagWriter.Update(mediaFile.AbsolutePath(), mediaFile.Comment); err != nil {
				log.Error(ctx, "Unable to write song comment to file", "song", mediaFile.ID, "path", mediaFile.AbsolutePath(), "err", err)
				return err
			}

			updated = mediaFile
			return nil
		})

		if err != nil {
			switch {
			case errors.Is(err, model.ErrNotFound):
				http.Error(w, err.Error(), http.StatusNotFound)
			case errors.Is(err, model.ErrNotAuthorized):
				http.Error(w, err.Error(), http.StatusForbidden)
			default:
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		if updated == nil {
			http.Error(w, "unable to update song", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(updated); err != nil {
			log.Error(ctx, "Unable to write response", "err", err)
		}
	}
}
