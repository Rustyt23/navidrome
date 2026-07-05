package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

type ragSearchRequest struct {
	Query   string            `json:"query"`
	TopK    int               `json:"topK"`
	Filters rag.SearchFilters `json:"filters,omitempty"`
}

type ragSearchResponse struct {
	Results        []rag.SongSearchResult `json:"results"`
	AppliedFilters rag.SearchFilters      `json:"appliedFilters"`
	Count          int                    `json:"count"`
}

type ragSearchFunc func(context.Context, string, int, rag.SearchFilters) ([]rag.SongSearchResult, error)

func (n *Router) handleRAGSearch(w http.ResponseWriter, request *http.Request) {
	serveRAGSearch(w, request, searchRAG)
}

func serveRAGSearch(w http.ResponseWriter, request *http.Request, search ragSearchFunc) {
	if !ragEnabled() {
		writeRAGSearchError(w, http.StatusServiceUnavailable, "RAG is disabled")
		return
	}
	payload, err := decodeRAGSearchRequest(request.Body, conf.Server.RAGTopK)
	if err != nil {
		writeRAGSearchError(w, http.StatusBadRequest, err.Error())
		return
	}

	results, err := search(request.Context(), payload.Query, payload.TopK, payload.Filters)
	if err != nil {
		writeRAGSearchError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if results == nil {
		results = []rag.SongSearchResult{}
	}
	_ = json.NewEncoder(w).Encode(ragSearchResponse{
		Results: results, AppliedFilters: payload.Filters, Count: len(results),
	})
}

func searchRAG(ctx context.Context, query string, topK int, filters rag.SearchFilters) ([]rag.SongSearchResult, error) {
	if !ragEnabled() {
		return nil, fmt.Errorf("RAG is disabled")
	}
	if strings.TrimSpace(conf.Server.GeminiAPIKey) == "" {
		return nil, fmt.Errorf("Gemini API key is not configured")
	}

	qdrant := rag.NewQdrantClient(conf.Server.RAGVectorURL, conf.Server.RAGCollection)
	status := qdrant.Status(ctx, false)
	if !status.VectorDBOnline {
		if status.Error != "" {
			return nil, errors.New(status.Error)
		}
		return nil, fmt.Errorf("Qdrant is offline")
	}
	if !status.CollectionExists {
		return nil, fmt.Errorf("Qdrant collection %q does not exist", conf.Server.RAGCollection)
	}
	if status.Error != "" {
		return nil, errors.New(status.Error)
	}

	return rag.SearchSongs(
		ctx,
		rag.NewGeminiQueryEmbedder(conf.Server.GeminiAPIKey),
		qdrant,
		query,
		topK,
		filters,
	)
}

func decodeRAGSearchRequest(reader io.Reader, defaultTopK int) (ragSearchRequest, error) {
	payload := ragSearchRequest{TopK: defaultTopK}
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return ragSearchRequest{}, fmt.Errorf("invalid request payload: %w", err)
	}
	payload.Query = strings.TrimSpace(payload.Query)
	if payload.Query == "" {
		return ragSearchRequest{}, fmt.Errorf("query is required")
	}
	if payload.TopK <= 0 || payload.TopK > rag.MaxSearchTopK {
		return ragSearchRequest{}, fmt.Errorf("topK must be between 1 and %d", rag.MaxSearchTopK)
	}
	normalizedFilters, err := rag.NormalizeSearchFilters(payload.Filters)
	if err != nil {
		return ragSearchRequest{}, err
	}
	payload.Filters = normalizedFilters
	return payload, nil
}

func writeRAGSearchError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func prepareAIChatMessage(
	ctx context.Context,
	message string,
	search ragSearchFunc,
) (string, []rag.SongSearchResult, error) {
	if !ragEnabled() {
		return message, nil, nil
	}
	filters := extractRAGFilters(message)
	results, err := search(ctx, message, conf.Server.RAGTopK, filters)
	if err != nil {
		return message, nil, err
	}
	return rag.BuildChatPrompt(message, results, filters), results, nil
}

var (
	bpmMinimumPattern  = regexp.MustCompile(`(?i)\bbpm\s+(?:above|over)\s+(\d+(?:\.\d+)?)\b`)
	durationMaxPattern = regexp.MustCompile(`(?i)\bunder\s+(\d+(?:\.\d+)?)\s+minutes?\b`)
	yearRangePattern   = regexp.MustCompile(`(?i)\bfrom\s+(\d{4})\s+to\s+(\d{4})\b`)
	cleanPattern       = regexp.MustCompile(`(?i)\bclean\b`)
	explicitPattern    = regexp.MustCompile(`(?i)\bexplicit\b`)
)

func extractRAGFilters(message string) rag.SearchFilters {
	filters := rag.SearchFilters{}
	lower := strings.ToLower(message)
	if cleanPattern.MatchString(message) {
		filters.Explicit = "clean"
	} else if explicitPattern.MatchString(message) {
		filters.Explicit = "explicit"
	}
	if match := bpmMinimumPattern.FindStringSubmatch(message); len(match) == 2 {
		if value, err := strconv.ParseFloat(match[1], 64); err == nil {
			filters.BPMMin = &value
		}
	}
	if match := durationMaxPattern.FindStringSubmatch(message); len(match) == 2 {
		if value, err := strconv.ParseFloat(match[1], 64); err == nil {
			value *= 60
			filters.DurationMax = &value
		}
	}
	if match := yearRangePattern.FindStringSubmatch(message); len(match) == 3 {
		if minYear, minErr := strconv.Atoi(match[1]); minErr == nil {
			if maxYear, maxErr := strconv.Atoi(match[2]); maxErr == nil {
				filters.YearMin = &minYear
				filters.YearMax = &maxYear
			}
		}
	}
	if strings.Contains(lower, "not played too often") {
		value := int64(20)
		filters.PlayCountMax = &value
	}
	return filters
}
