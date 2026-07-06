package nativeapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

type ragIndexRequest struct {
	Limit            int   `json:"limit"`
	Force            bool  `json:"force"`
	Sync             bool  `json:"sync,omitempty"`
	IncludeSongs     *bool `json:"includeSongs,omitempty"`
	IncludePlaylists bool  `json:"includePlaylists"`
	PlaylistLimit    int   `json:"playlistLimit,omitempty"`
}

type ragIndexResponse struct {
	rag.IndexResult
	Deleted   int              `json:"deleted,omitempty"`
	Playlists *rag.IndexResult `json:"playlists,omitempty"`
}

func (n *Router) addRAGAdminRoute(router chi.Router) {
	router.Post("/ai/rag/index", n.handleRAGIndex)
	router.Get("/ai/rag/documents", n.handleRAGDocuments)
	router.Post("/ai/rag/enabled", n.handleRAGEnabled)
	router.Post("/ai/whisper/model", n.handleWhisperModel)
}

func (n *Router) handleRAGIndex(w http.ResponseWriter, request *http.Request) {
	if !ragEnabled() {
		writeRAGIndexError(w, http.StatusServiceUnavailable, "RAG is disabled")
		return
	}
	if !ragEmbeddingConfigured() {
		writeRAGIndexError(w, http.StatusServiceUnavailable, "no embedding backend is configured (set RAGEmbeddingURL or GeminiAPIKey)")
		return
	}
	if n.ds == nil {
		writeRAGIndexError(w, http.StatusInternalServerError, "media repository is unavailable")
		return
	}

	payload, err := decodeRAGIndexRequest(request.Body)
	if err != nil {
		writeRAGIndexError(w, http.StatusBadRequest, err.Error())
		return
	}
	includeSongs := payload.IncludeSongs == nil || *payload.IncludeSongs

	qdrant := newRAGQdrantClient()
	qdrantStatus := qdrant.Status(request.Context(), true)
	if qdrantStatus.ReindexRequired && payload.Force && payload.Sync && includeSongs {
		if err := qdrant.RecreateCollection(request.Context()); err != nil {
			writeRAGIndexError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		qdrantStatus = qdrant.Status(request.Context(), false)
	}
	if !qdrantStatus.VectorDBOnline || !qdrantStatus.CollectionExists || qdrantStatus.Error != "" {
		message := qdrantStatus.Error
		if message == "" {
			message = "Qdrant collection is unavailable"
		} else if qdrantStatus.ReindexRequired {
			message += "; retry the index request with both force=true and sync=true to rebuild it"
		}
		writeRAGIndexError(w, http.StatusServiceUnavailable, message)
		return
	}

	result := rag.IndexResult{}
	deleted := 0
	if includeSongs {
		if payload.Sync {
			// Full-library sync: index new/changed songs and remove orphaned points.
			syncResult, syncErr := rag.SyncSongs(
				request.Context(),
				n.ds.MediaFile(request.Context()),
				ragDocumentEmbedder(),
				qdrant,
				ragEmbedderTag(),
			)
			if syncErr != nil {
				writeRAGIndexError(w, http.StatusInternalServerError, syncErr.Error())
				return
			}
			result = syncResult.IndexResult
			deleted = syncResult.Deleted
		} else {
			result, err = rag.IndexSongs(
				request.Context(),
				n.ds.MediaFile(request.Context()),
				ragDocumentEmbedder(),
				qdrant,
				payload.Limit,
				payload.Force,
				ragEmbedderTag(),
			)
			if err != nil {
				writeRAGIndexError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	}

	response := ragIndexResponse{IndexResult: result, Deleted: deleted}
	if payload.IncludePlaylists {
		playlistLimit := payload.PlaylistLimit
		if playlistLimit == 0 {
			playlistLimit = payload.Limit
		}
		playlistResult, playlistErr := rag.IndexPlaylists(
			request.Context(),
			n.ds.Playlist(request.Context()),
			ragDocumentEmbedder(),
			qdrant,
			playlistLimit,
			payload.Force,
		)
		if playlistErr != nil {
			writeRAGIndexError(w, http.StatusInternalServerError, playlistErr.Error())
			return
		}
		response.Playlists = &playlistResult
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func decodeRAGIndexRequest(reader io.Reader) (ragIndexRequest, error) {
	payload := ragIndexRequest{Limit: rag.DefaultIndexLimit}
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		return ragIndexRequest{}, fmt.Errorf("invalid request payload: %w", err)
	}
	if payload.Limit <= 0 || payload.Limit > rag.MaxIndexLimit {
		return ragIndexRequest{}, fmt.Errorf("limit must be between 1 and %d", rag.MaxIndexLimit)
	}
	if payload.PlaylistLimit < 0 || payload.PlaylistLimit > rag.MaxIndexLimit {
		return ragIndexRequest{}, fmt.Errorf("playlistLimit must be between 1 and %d when provided", rag.MaxIndexLimit)
	}
	return payload, nil
}

func writeRAGIndexError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
