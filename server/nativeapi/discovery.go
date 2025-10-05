package nativeapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/deluan/rest"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server"
	"github.com/navidrome/navidrome/utils/req"
)

func (n *Router) addDiscoveryRoute(r chi.Router) {
	r.Route("/discovery", func(r chi.Router) {
		r.Get("/", n.listDiscovery())
		r.Post("/refresh", n.refreshDiscovery())
		r.Route("/{id}", func(r chi.Router) {
			r.Use(server.URLParamsMiddleware)
			r.Get("/", n.getDiscovery())
			r.Get("/tracks", n.getDiscoveryTracks())
			r.Get("/export", n.exportDiscovery())
			r.Delete("/tracks", n.deleteDiscoveryTracks())
			r.Post("/publish", n.publishDiscovery())
		})
	})
}

func (n *Router) listDiscovery() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		refresh := strings.EqualFold(r.URL.Query().Get("refresh"), "true")
		playlists, err := n.discovery.List(r.Context(), refresh)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rest.RespondWithJSON(w, http.StatusOK, playlists)
	}
}

func (n *Router) refreshDiscovery() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playlists, err := n.discovery.Refresh(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rest.RespondWithJSON(w, http.StatusOK, playlists)
	}
}

func (n *Router) getDiscovery() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		entry, err := n.discovery.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, rest.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			log.Error(r.Context(), "Error retrieving discovery playlist", "id", id, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rest.RespondWithJSON(w, http.StatusOK, entry)
	}
}

func (n *Router) getDiscoveryTracks() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		tracks, err := n.discovery.Tracks(r.Context(), id)
		if err != nil {
			if errors.Is(err, rest.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			log.Error(r.Context(), "Error retrieving discovery playlist tracks", "id", id, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rest.RespondWithJSON(w, http.StatusOK, tracks)
	}
}

func (n *Router) exportDiscovery() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		entry, err := n.discovery.Get(r.Context(), id)
		if err != nil {
			if errors.Is(err, rest.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			log.Error(r.Context(), "Error retrieving discovery playlist for export", "id", id, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "audio/x-mpegurl")
		filename := entry.Name
		if filename == "" {
			filename = "discovery-" + id
		}
		w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+".m3u\"")
		if err := n.discovery.Export(r.Context(), id, w); err != nil {
			log.Error(r.Context(), "Error exporting discovery playlist", "id", id, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (n *Router) deleteDiscoveryTracks() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := req.Params(r)
		id, _ := p.String(":id")
		ids, _ := p.Strings("id")
		if len(ids) == 0 {
			writeDeleteManyResponse(w, r, ids)
			return
		}
		if err := n.discovery.DeleteTracks(r.Context(), id, ids); err != nil {
			if errors.Is(err, rest.ErrNotFound) || errors.Is(err, sql.ErrNoRows) || errors.Is(err, model.ErrNotFound) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			log.Error(r.Context(), "Error deleting discovery tracks", "id", id, "tracks", ids, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeDeleteManyResponse(w, r, ids)
	}
}

func (n *Router) publishDiscovery() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if err := n.discovery.Publish(r.Context(), id); err != nil {
			if errors.Is(err, rest.ErrNotFound) || errors.Is(err, sql.ErrNoRows) || errors.Is(err, model.ErrNotFound) {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			log.Error(r.Context(), "Error publishing discovery playlist", "id", id, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
