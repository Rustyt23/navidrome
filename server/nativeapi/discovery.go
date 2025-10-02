package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/deluan/rest"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/core"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/req"
)

type restHandler = func(rest.RepositoryConstructor, ...rest.Logger) http.HandlerFunc

func getPlaylist(ds model.DataStore) http.HandlerFunc {
	// Add a middleware to capture the discoveryId
	wrapper := func(handler restHandler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			constructor := func(ctx context.Context) rest.Repository {
				plsRepo := ds.Discovery(ctx)
				plsId := chi.URLParam(r, "discoveryId")
				p := req.Params(r)
				start := p.Int64Or("_start", 0)
				return plsRepo.Tracks(plsId, start == 0)
			}

			handler(constructor).ServeHTTP(w, r)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("accept")
		if strings.ToLower(accept) == "audio/x-mpegurl" {
			handleExportPlaylist(ds)(w, r)
			return
		}
		wrapper(rest.GetAll)(w, r)
	}
}

func getPlaylistTrack(ds model.DataStore) http.HandlerFunc {
	// Add a middleware to capture the discoveryId
	wrapper := func(handler restHandler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			constructor := func(ctx context.Context) rest.Repository {
				plsRepo := ds.Discovery(ctx)
				plsId := chi.URLParam(r, "discoveryId")
				return plsRepo.Tracks(plsId, true)
			}

			handler(constructor).ServeHTTP(w, r)
		}
	}

	return wrapper(rest.Get)
}

func createPlaylist(ds model.DataStore, playlists core.Discoveries) http.HandlerFunc {
	constructor := func(ctx context.Context) rest.Repository {
		return ds.Resource(ctx, model.Discovery{})
	}
	return func(w http.ResponseWriter, r *http.Request) {
		c := rest.Controller{Repository: constructor(r.Context())}
		rp, ok := c.Repository.(rest.Persistable)
		if !ok {
			rest.RespondWithError(w, http.StatusMethodNotAllowed, "405 Method Not Allowed")
			return
		}
		entity := c.Repository.NewInstance().(*model.Discovery)
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(entity); err != nil {
			rest.RespondWithError(w, http.StatusUnprocessableEntity, "Invalid request payload")
			return
		}
		entity.Sync = true
		id, err := rp.Save(entity)
		if err == rest.ErrPermissionDenied {
			rest.RespondWithError(w, http.StatusForbidden, "Saving playlist: Permission denied")
			return
		}
		if err != nil {
			rest.RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := syncPlaylist(playlists, ds, r.Context(), id); err != nil {
			rest.RespondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		rest.RespondWithJSON(w, http.StatusOK, &map[string]string{"id": id})
	}
}

func createPlaylistFromM3U(playlists core.Discoveries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		pls, err := playlists.ImportM3U(ctx, r.Body)
		if err != nil {
			log.Error(r.Context(), "Error parsing playlist", err)
			// TODO: consider returning StatusBadRequest for playlists that are malformed
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, err = w.Write([]byte(pls.ToM3U8()))
		if err != nil {
			log.Error(ctx, "Error sending m3u contents", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func handleExportPlaylist(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		plsRepo := ds.Discovery(ctx)
		plsId := chi.URLParam(r, "discoveryId")
		pls, err := plsRepo.GetWithTracks(plsId, true, false)
		if errors.Is(err, model.ErrNotFound) {
			log.Warn(r.Context(), "Discovery not found", "discoveryId", plsId)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Error(r.Context(), "Error retrieving the playlist", "discoveryId", plsId, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Debug(ctx, "Exporting playlist as M3U", "discoveryId", plsId, "name", pls.Name)
		w.Header().Set("Content-Type", "audio/x-mpegurl")
		disposition := fmt.Sprintf("attachment; filename=\"%s.m3u\"", pls.Name)
		w.Header().Set("Content-Disposition", disposition)

		_, err = w.Write([]byte(pls.ToM3U8()))
		if err != nil {
			log.Error(ctx, "Error sending playlist", "name", pls.Name)
			return
		}
	}
}

func publishPlaylist(ds model.DataStore, playlists core.Discoveries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		ctx := r.Context()
		if err := syncPlaylist(playlists, ds, ctx, id); err != nil {
			http.Error(w, err.Error(), statusFor(err))
			return
		}
		if err := playlists.Publish(ctx, id); err != nil {
			http.Error(w, err.Error(), statusFor(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteFromPlaylist(ds model.DataStore, playlists core.Discoveries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := req.Params(r)
		discoveryId, _ := p.String(":discoveryId")
		ids, _ := p.Strings("id")
		err := ds.WithTxImmediate(func(tx model.DataStore) error {
			tracksRepo := tx.Discovery(r.Context()).Tracks(discoveryId, true)
			return tracksRepo.Delete(ids...)
		})
		if len(ids) == 1 && errors.Is(err, model.ErrNotFound) {
			log.Warn(r.Context(), "Track not found in playlist", "discoveryId", discoveryId, "id", ids[0])
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Error(r.Context(), "Error deleting tracks from playlist", "discoveryId", discoveryId, "ids", ids, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := syncPlaylist(playlists, ds, r.Context(), discoveryId); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeDeleteManyResponse(w, r, ids)
	}
}

func addToPlaylist(ds model.DataStore, playlists core.Discoveries) http.HandlerFunc {
	type addTracksPayload struct {
		Ids       []string       `json:"ids"`
		AlbumIds  []string       `json:"albumIds"`
		ArtistIds []string       `json:"artistIds"`
		Discs     []model.DiscID `json:"discs"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		p := req.Params(r)
		discoveryId, _ := p.String(":discoveryId")
		var payload addTracksPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		tracksRepo := ds.Discovery(r.Context()).Tracks(discoveryId, true)
		count, c := 0, 0
		if c, err = tracksRepo.Add(payload.Ids); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		count += c
		if c, err = tracksRepo.AddAlbums(payload.AlbumIds); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		count += c
		if c, err = tracksRepo.AddArtists(payload.ArtistIds); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		count += c
		if c, err = tracksRepo.AddDiscs(payload.Discs); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		count += c

		if err := syncPlaylist(playlists, ds, r.Context(), discoveryId); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Must return an object with an ID, to satisfy ReactAdmin `create` call
		_, err = fmt.Fprintf(w, `{"added":%d}`, count)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func reorderItem(ds model.DataStore, playlists core.Discoveries) http.HandlerFunc {
	type reorderPayload struct {
		InsertBefore string `json:"insert_before"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		p := req.Params(r)
		discoveryId, _ := p.String(":discoveryId")
		id := p.IntOr(":id", 0)
		if id == 0 {
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}
		var payload reorderPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		newPos, err := strconv.Atoi(payload.InsertBefore)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		tracksRepo := ds.Discovery(r.Context()).Tracks(discoveryId, true)
		err = tracksRepo.Reorder(id, newPos)
		if errors.Is(err, rest.ErrPermissionDenied) {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := syncPlaylist(playlists, ds, r.Context(), discoveryId); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_, err = w.Write([]byte(fmt.Sprintf(`{"id":"%d"}`, id)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func getSongPlaylists(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := req.Params(r)
		trackId, _ := p.String(":id")
		playlists, err := ds.Discovery(r.Context()).GetPlaylists(trackId)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data, err := json.Marshal(playlists)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(data)
	}
}

func syncPlaylist(playlists core.Discoveries, ds model.DataStore, ctx context.Context, discoveryId string) error {
	pls, err := ds.Discovery(ctx).Get(discoveryId)
	if err != nil {
		return err
	}
	return playlists.Update(ctx, discoveryId, &pls.Name, &pls.Comment, &pls.Public, nil, nil)
}
