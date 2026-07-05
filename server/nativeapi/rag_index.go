package nativeapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

type ragIndexRequest struct {
	Limit            int   `json:"limit"`
	Force            bool  `json:"force"`
	IncludeSongs     *bool `json:"includeSongs,omitempty"`
	IncludePlaylists bool  `json:"includePlaylists"`
	PlaylistLimit    int   `json:"playlistLimit,omitempty"`
}

type ragIndexResponse struct {
	rag.IndexResult
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
	if strings.TrimSpace(conf.Server.GeminiAPIKey) == "" {
		writeRAGIndexError(w, http.StatusServiceUnavailable, "Gemini API key is not configured")
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

	qdrant := rag.NewQdrantClient(conf.Server.RAGVectorURL, conf.Server.RAGCollection)
	qdrantStatus := qdrant.Status(request.Context(), true)
	if !qdrantStatus.VectorDBOnline || !qdrantStatus.CollectionExists || qdrantStatus.Error != "" {
		message := qdrantStatus.Error
		if message == "" {
			message = "Qdrant collection is unavailable"
		}
		writeRAGIndexError(w, http.StatusServiceUnavailable, message)
		return
	}

	result := rag.IndexResult{}
	includeSongs := payload.IncludeSongs == nil || *payload.IncludeSongs
	if includeSongs {
		result, err = rag.IndexSongs(
			request.Context(),
			n.ds.MediaFile(request.Context()),
			rag.NewGeminiEmbedder(conf.Server.GeminiAPIKey),
			qdrant,
			payload.Limit,
			payload.Force,
		)
		if err != nil {
			writeRAGIndexError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	response := ragIndexResponse{IndexResult: result}
	if payload.IncludePlaylists {
		playlistLimit := payload.PlaylistLimit
		if playlistLimit == 0 {
			playlistLimit = payload.Limit
		}
		playlistResult, playlistErr := rag.IndexPlaylists(
			request.Context(),
			n.ds.Playlist(request.Context()),
			rag.NewGeminiEmbedder(conf.Server.GeminiAPIKey),
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
