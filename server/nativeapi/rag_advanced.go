package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

type ragLyricSearchRequest struct {
	Query string `json:"query"`
	TopK  int    `json:"topK"`
}

func (n *Router) handleRAGLyricSearch(w http.ResponseWriter, request *http.Request) {
	serveRAGLyricSearch(w, request, searchRAG)
}

func serveRAGLyricSearch(w http.ResponseWriter, request *http.Request, search ragSearchFunc) {
	var payload ragLyricSearchRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeRAGSearchError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	payload.Query = strings.TrimSpace(payload.Query)
	if payload.Query == "" {
		writeRAGSearchError(w, http.StatusBadRequest, "query is required")
		return
	}
	if payload.TopK == 0 {
		payload.TopK = 10
	}
	if payload.TopK < 1 || payload.TopK > rag.MaxSearchTopK {
		writeRAGSearchError(w, http.StatusBadRequest, fmt.Sprintf("topK must be between 1 and %d", rag.MaxSearchTopK))
		return
	}
	required := true
	results, err := search(request.Context(), payload.Query, payload.TopK, rag.SearchFilters{HasLyrics: &required})
	if err != nil {
		writeRAGSearchError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	results = rag.AddLyricSnippets(payload.Query, results)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"query": payload.Query, "count": len(results), "results": results})
}

func (n *Router) handleRAGDuplicates(w http.ResponseWriter, request *http.Request) {
	limit := 1000
	if raw := strings.TrimSpace(request.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 2 || parsed > 5000 {
			writeRAGSearchError(w, http.StatusBadRequest, "limit must be between 2 and 5000")
			return
		}
		limit = parsed
	}
	threshold := 0.92
	if raw := strings.TrimSpace(request.URL.Query().Get("threshold")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || parsed < 0.5 || parsed > 1 {
			writeRAGSearchError(w, http.StatusBadRequest, "threshold must be between 0.5 and 1")
			return
		}
		threshold = parsed
	}
	candidates, err := loadDuplicateCandidates(request.Context(), limit, threshold)
	if err != nil {
		writeRAGSearchError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"count": len(candidates), "results": candidates})
}

func loadDuplicateCandidates(ctx context.Context, limit int, threshold float64) ([]rag.DuplicateCandidate, error) {
	client := newRAGQdrantClient()
	status := client.Status(ctx, false)
	if !status.VectorDBOnline || !status.CollectionExists || status.Error != "" {
		if status.Error != "" {
			return nil, fmt.Errorf("duplicate analysis unavailable: %s", status.Error)
		}
		return nil, fmt.Errorf("duplicate analysis unavailable: Qdrant collection is offline")
	}
	items, err := client.ListSongVectors(ctx, limit)
	if err != nil {
		return nil, err
	}
	return rag.DetectDuplicateSongs(items, threshold), nil
}

func (n *Router) ragChatFeatures() ragChatFeatures {
	features := ragChatFeatures{}
	if n.ds != nil {
		features.analytics = func(ctx context.Context, filters rag.SearchFilters) (string, error) {
			songs, err := loadDashboardSongs(n.ds.MediaFile(ctx))
			if err != nil {
				return "", err
			}
			data, err := json.Marshal(rag.BuildLibraryAnalyticsReport(songs, filters))
			if err != nil {
				return "", fmt.Errorf("could not encode library analytics: %w", err)
			}
			return string(data), nil
		}
	}
	features.duplicates = func(ctx context.Context) (string, error) {
		candidates, err := loadDuplicateCandidates(ctx, 1000, 0.92)
		if err != nil {
			return "", err
		}
		total := len(candidates)
		if total > 100 {
			candidates = candidates[:100]
		}
		data, err := json.Marshal(map[string]any{"count": total, "returned": len(candidates), "candidates": candidates})
		if err != nil {
			return "", fmt.Errorf("could not encode duplicate analysis: %w", err)
		}
		return string(data), nil
	}
	return features
}
