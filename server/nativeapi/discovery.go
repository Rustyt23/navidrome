package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/deluan/rest"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/core"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server"
	"github.com/navidrome/navidrome/utils/req"
)

func (n *Router) addDiscoveryRoutes(r chi.Router) {
	r.Route("/discovery", func(r chi.Router) {
		constructor := func(ctx context.Context) rest.Repository {
			return n.ds.Resource(ctx, model.Discovery{})
		}

		r.Get("/", rest.GetAll(constructor))
		r.Post("/", rest.Post(constructor))

		r.Route("/folder", func(r chi.Router) {
			folderConstructor := func(ctx context.Context) rest.Repository {
				return n.ds.Resource(ctx, model.DiscoveryFolder{})
			}
			r.Get("/", rest.GetAll(folderConstructor))
			r.Post("/", rest.Post(folderConstructor))
			r.Route("/{id}", func(r chi.Router) {
				r.Use(server.URLParamsMiddleware)
				r.Get("/", rest.Get(folderConstructor))
				r.Put("/", rest.Put(folderConstructor))
				r.Delete("/", rest.Delete(folderConstructor))
				r.Patch("/parent", moveDiscoveryFolder(n.ds))
			})
			r.Patch("/move", bulkMoveDiscoveries(n.ds, n.discoveries))
		})

		r.Route("/{id}", func(r chi.Router) {
			r.Use(server.URLParamsMiddleware)
			r.Get("/", rest.Get(constructor))
			r.Put("/", rest.Put(constructor))
			r.Delete("/", rest.Delete(constructor))

			r.Patch("/folder", func(w http.ResponseWriter, r *http.Request) {
				id := chi.URLParam(r, "id")
				type reqBody struct {
					FolderID *string `json:"folderId"`
				}
				var body reqBody
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if err := n.discoveries.SetFolder(r.Context(), id, body.FolderID); err != nil {
					http.Error(w, err.Error(), statusFor(err))
					return
				}
				w.WriteHeader(http.StatusNoContent)
			})
		})

		r.Route("/{discoveryId}/songs", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				getDiscoverySongs(n.ds)(w, r)
			})
			r.With(server.URLParamsMiddleware).Route("/", func(r chi.Router) {
				r.Post("/", func(w http.ResponseWriter, r *http.Request) {
					addToDiscovery(n.ds, n.discoveries)(w, r)
				})
				r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
					deleteFromDiscovery(n.ds, n.discoveries)(w, r)
				})
			})
			r.Route("/{id}", func(r chi.Router) {
				r.Use(server.URLParamsMiddleware)
				r.Get("/", func(w http.ResponseWriter, r *http.Request) {
					getDiscoverySong(n.ds)(w, r)
				})
				r.Put("/", func(w http.ResponseWriter, r *http.Request) {
					reorderDiscoverySong(n.ds, n.discoveries)(w, r)
				})
				r.Delete("/", func(w http.ResponseWriter, r *http.Request) {
					deleteFromDiscovery(n.ds, n.discoveries)(w, r)
				})
			})
		})
	})
}

func getDiscoverySongs(ds model.DataStore) http.HandlerFunc {
	wrapper := func(handler restHandler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			constructor := func(ctx context.Context) rest.Repository {
				discoveryID := chi.URLParam(r, "discoveryId")
				p := req.Params(r)
				start := p.Int64Or("_start", 0)
				return ds.DiscoverySong(ctx, discoveryID, start == 0)
			}
			handler(constructor).ServeHTTP(w, r)
		}
	}
	return wrapper(rest.GetAll)
}

func getDiscoverySong(ds model.DataStore) http.HandlerFunc {
	wrapper := func(handler restHandler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			constructor := func(ctx context.Context) rest.Repository {
				discoveryID := chi.URLParam(r, "discoveryId")
				return ds.DiscoverySong(ctx, discoveryID, true)
			}
			handler(constructor).ServeHTTP(w, r)
		}
	}
	return wrapper(rest.Get)
}

func addToDiscovery(ds model.DataStore, discoveries core.Discoveries) http.HandlerFunc {
	type addSongsPayload struct {
		Ids       []string       `json:"ids"`
		AlbumIds  []string       `json:"albumIds"`
		ArtistIds []string       `json:"artistIds"`
		Discs     []model.DiscID `json:"discs"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		p := req.Params(r)
		discoveryID, _ := p.String(":discoveryId")
		var payload addSongsPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		tracksRepo := ds.DiscoverySong(r.Context(), discoveryID, true)
		count := 0
		var err error
		if len(payload.Ids) > 0 {
			var added int
			if added, err = tracksRepo.Add(payload.Ids); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			count += added
		}
		if len(payload.AlbumIds) > 0 {
			var added int
			if added, err = tracksRepo.AddAlbums(payload.AlbumIds); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			count += added
		}
		if len(payload.ArtistIds) > 0 {
			var added int
			if added, err = tracksRepo.AddArtists(payload.ArtistIds); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			count += added
		}
		if len(payload.Discs) > 0 {
			var added int
			if added, err = tracksRepo.AddDiscs(payload.Discs); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			count += added
		}

		if err := syncDiscovery(discoveries, ds, r.Context(), discoveryID); err != nil {
			http.Error(w, err.Error(), statusFor(err))
			return
		}

		_, err = fmt.Fprintf(w, `{"added":%d}`, count)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func deleteFromDiscovery(ds model.DataStore, discoveries core.Discoveries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := req.Params(r)
		discoveryID, _ := p.String(":discoveryId")
		ids, _ := p.Strings("id")
		err := ds.WithTxImmediate(func(tx model.DataStore) error {
			return tx.DiscoverySong(r.Context(), discoveryID, true).Delete(ids...)
		})
		if len(ids) == 1 && errors.Is(err, model.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := syncDiscovery(discoveries, ds, r.Context(), discoveryID); err != nil {
			http.Error(w, err.Error(), statusFor(err))
			return
		}
		writeDeleteManyResponse(w, r, ids)
	}
}

func reorderDiscoverySong(ds model.DataStore, discoveries core.Discoveries) http.HandlerFunc {
	type reorderPayload struct {
		InsertBefore string `json:"insert_before"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		p := req.Params(r)
		discoveryID, _ := p.String(":discoveryId")
		id := p.IntOr(":id", 0)
		if id == 0 {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var payload reorderPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		newPos, err := strconv.Atoi(payload.InsertBefore)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		tracksRepo := ds.DiscoverySong(r.Context(), discoveryID, true)
		if err := tracksRepo.Reorder(id, newPos); err != nil {
			if errors.Is(err, rest.ErrPermissionDenied) {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := syncDiscovery(discoveries, ds, r.Context(), discoveryID); err != nil {
			http.Error(w, err.Error(), statusFor(err))
			return
		}

		_, err = w.Write([]byte(fmt.Sprintf(`{"id":"%d"}`, id)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func syncDiscovery(discoveries core.Discoveries, ds model.DataStore, ctx context.Context, discoveryID string) error {
	discovery, err := ds.Discovery(ctx).Get(discoveryID)
	if err != nil {
		return err
	}
	return discoveries.Update(ctx, discoveryID, &discovery.Name, &discovery.Comment, &discovery.Public, nil, nil)
}

func moveDiscoveryFolder(ds model.DataStore) http.HandlerFunc {
	type reqBody struct {
		ParentID *string `json:"parentId"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var body reqBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body.ParentID != nil && *body.ParentID == "" {
			http.Error(w, "parentId cannot be empty; use null for root", http.StatusBadRequest)
			return
		}
		if err := ds.DiscoveryFolder(r.Context()).UpdateParent(id, body.ParentID); err != nil {
			http.Error(w, err.Error(), statusFor(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func bulkMoveDiscoveries(ds model.DataStore, discoveries core.Discoveries) http.HandlerFunc {
	type movePayload struct {
		Ids      []string `json:"ids"`
		Types    []string `json:"types"`
		FolderID *string  `json:"folderId"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var payload movePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(payload.Ids) != len(payload.Types) {
			http.Error(w, "ids and types length mismatch", http.StatusBadRequest)
			return
		}
		if payload.FolderID != nil && *payload.FolderID == "" {
			http.Error(w, "folderId cannot be empty; use null for root", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		updated := 0
		for i, id := range payload.Ids {
			switch payload.Types[i] {
			case "discovery":
				if err := discoveries.SetFolder(ctx, id, payload.FolderID); err != nil {
					http.Error(w, err.Error(), statusFor(err))
					return
				}
				updated++
			case "folder":
				if err := ds.DiscoveryFolder(ctx).UpdateParent(id, payload.FolderID); err != nil {
					http.Error(w, err.Error(), statusFor(err))
					return
				}
				updated++
			default:
				http.Error(w, "invalid type "+payload.Types[i], http.StatusBadRequest)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(fmt.Sprintf(`{"updated":%d}`, updated)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
