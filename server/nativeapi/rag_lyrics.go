package nativeapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

type addQdrantLyricsRequest struct {
	Limit int `json:"limit"`
}

type qdrantLyricsResponse struct {
	Collection string            `json:"collection"`
	Query      string            `json:"query,omitempty"`
	Count      int               `json:"count"`
	Songs      []rag.IndexedSong `json:"songs"`
}

func (n *Router) handleAddQdrantLyrics(w http.ResponseWriter, request *http.Request) {
	if !ragEnabled() {
		writeRAGIndexError(w, http.StatusServiceUnavailable, "RAG is disabled")
		return
	}
	if !ragEmbeddingConfigured() {
		writeRAGIndexError(w, http.StatusServiceUnavailable, "no embedding backend is configured (set RAGEmbeddingURL or GeminiAPIKey)")
		return
	}
	payload := addQdrantLyricsRequest{Limit: rag.MaxIndexLimit}
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		writeRAGIndexError(w, http.StatusBadRequest, fmt.Sprintf("invalid request payload: %v", err))
		return
	}
	if payload.Limit <= 0 || payload.Limit > rag.MaxIndexLimit {
		writeRAGIndexError(w, http.StatusBadRequest, fmt.Sprintf("limit must be between 1 and %d", rag.MaxIndexLimit))
		return
	}
	if n.ds == nil {
		writeRAGIndexError(w, http.StatusInternalServerError, "media repository is unavailable")
		return
	}

	qdrant := newRAGQdrantClient()
	status := qdrant.Status(request.Context(), true)
	if !status.VectorDBOnline || !status.CollectionExists || status.Error != "" {
		message := status.Error
		if message == "" {
			message = "Qdrant collection is unavailable"
		}
		writeRAGIndexError(w, http.StatusServiceUnavailable, message)
		return
	}
	if err := qdrant.EnsureLyricsTextIndex(request.Context()); err != nil {
		writeRAGIndexError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	result, err := rag.IndexSongsWithLyrics(
		request.Context(),
		n.ds.MediaFile(request.Context()),
		ragDocumentEmbedder(),
		qdrant,
		payload.Limit,
		ragEmbedderTag(),
	)
	if err != nil {
		writeRAGIndexError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (n *Router) handleListQdrantLyrics(w http.ResponseWriter, request *http.Request) {
	if !ragEnabled() {
		writeRAGDocumentsError(w, http.StatusServiceUnavailable, "RAG is disabled")
		return
	}
	limit, err := parseRAGDocumentsLimit(request.URL.Query().Get("limit"))
	if err != nil {
		writeRAGDocumentsError(w, http.StatusBadRequest, err.Error())
		return
	}
	query := strings.TrimSpace(request.URL.Query().Get("query"))
	required := true
	filters, err := rag.NormalizeSearchFilters(rag.SearchFilters{
		LyricsContains: query,
		HasLyrics:      &required,
	})
	if err != nil {
		writeRAGDocumentsError(w, http.StatusBadRequest, err.Error())
		return
	}

	qdrant := newRAGQdrantClient()
	status := qdrant.Status(request.Context(), false)
	if !status.VectorDBOnline || !status.CollectionExists || status.Error != "" {
		message := status.Error
		if message == "" {
			message = "Qdrant collection is unavailable"
		}
		writeRAGDocumentsError(w, http.StatusServiceUnavailable, message)
		return
	}
	songs, err := qdrant.ListSongs(request.Context(), limit, filters)
	if err != nil {
		writeRAGDocumentsError(w, http.StatusBadGateway, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(qdrantLyricsResponse{
		Collection: conf.Server.RAGCollection,
		Query:      query,
		Count:      len(songs),
		Songs:      songs,
	})
}
