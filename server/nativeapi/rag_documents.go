package nativeapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

const defaultRAGDocumentsLimit = 100

type ragDocumentsResponse struct {
	Collection   string            `json:"collection"`
	IndexedCount int64             `json:"indexedCount"`
	Songs        []rag.IndexedSong `json:"songs"`
}

func (n *Router) handleRAGDocuments(w http.ResponseWriter, request *http.Request) {
	if !ragEnabled() {
		writeRAGDocumentsError(w, http.StatusServiceUnavailable, "RAG is disabled")
		return
	}

	limit, err := parseRAGDocumentsLimit(request.URL.Query().Get("limit"))
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

	songs, err := qdrant.ListSongs(request.Context(), limit)
	if err != nil {
		writeRAGDocumentsError(w, http.StatusBadGateway, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ragDocumentsResponse{
		Collection:   conf.Server.RAGCollection,
		IndexedCount: status.IndexedCount,
		Songs:        songs,
	})
}

func parseRAGDocumentsLimit(value string) (int, error) {
	if value == "" {
		return defaultRAGDocumentsLimit, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 || limit > rag.MaxListLimit {
		return 0, fmt.Errorf("limit must be between 1 and %d", rag.MaxListLimit)
	}
	return limit, nil
}

func writeRAGDocumentsError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}
