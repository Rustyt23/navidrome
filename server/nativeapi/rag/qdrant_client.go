package rag

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // Used only for deterministic UUID generation, not security.
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const qdrantRequestTimeout = 4 * time.Second

// QdrantStatus describes only connection and collection state. It does not
// perform embeddings, indexing, or vector search.
type QdrantStatus struct {
	VectorDBOnline   bool
	CollectionExists bool
	IndexedCount     int64
	Error            string
}

// QdrantClient is the small REST client used by the RAG status endpoint.
type QdrantClient struct {
	baseURL    string
	collection string
	httpClient *http.Client
}

// NewQdrantClient creates a Qdrant REST client with a short timeout so an
// unavailable vector database cannot block or crash Navidrome.
func NewQdrantClient(baseURL, collection string) *QdrantClient {
	return &QdrantClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		collection: strings.TrimSpace(collection),
		httpClient: &http.Client{Timeout: qdrantRequestTimeout},
	}
}

// Status checks Qdrant and optionally creates the missing collection using the
// fixed vector size shared with GeminiEmbedder.
func (c *QdrantClient) Status(ctx context.Context, createIfMissing bool) QdrantStatus {
	status := QdrantStatus{}
	if err := c.CheckReachable(ctx); err != nil {
		status.Error = fmt.Sprintf("Qdrant unavailable: %v", err)
		return status
	}
	status.VectorDBOnline = true

	exists, count, err := c.CollectionInfo(ctx)
	if err != nil {
		status.Error = fmt.Sprintf("Could not check Qdrant collection: %v", err)
		return status
	}
	status.CollectionExists = exists
	status.IndexedCount = count

	if !exists && createIfMissing {
		if err := c.CreateCollection(ctx); err != nil {
			status.Error = fmt.Sprintf("Could not create Qdrant collection: %v", err)
			return status
		}
		status.CollectionExists = true
		status.IndexedCount = 0
	}

	return status
}

// CheckReachable verifies that Qdrant's collection API responds successfully.
func (c *QdrantClient) CheckReachable(ctx context.Context) error {
	response, err := c.do(ctx, http.MethodGet, "/collections", nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return qdrantResponseError(response)
}

// CollectionInfo reports whether the configured collection exists and returns
// its point count when Qdrant includes points_count in the response.
func (c *QdrantClient) CollectionInfo(ctx context.Context) (bool, int64, error) {
	if c.collection == "" {
		return false, 0, fmt.Errorf("collection name is empty")
	}

	response, err := c.do(
		ctx,
		http.MethodGet,
		"/collections/"+url.PathEscape(c.collection),
		nil,
	)
	if err != nil {
		return false, 0, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return false, 0, nil
	}
	if err := qdrantResponseError(response); err != nil {
		return false, 0, err
	}

	var payload struct {
		Result struct {
			PointsCount *int64 `json:"points_count"`
		} `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return false, 0, fmt.Errorf("invalid collection response: %w", err)
	}
	if payload.Result.PointsCount == nil {
		return true, 0, nil
	}
	return true, *payload.Result.PointsCount, nil
}

// CreateCollection creates the dense vector collection used by the Gemini
// embedder. This is one of only two Qdrant writes in Phase 1; the other is
// upserting song points through UpsertPoint.
func (c *QdrantClient) CreateCollection(ctx context.Context) error {
	if c.collection == "" {
		return fmt.Errorf("collection name is empty")
	}

	body, err := json.Marshal(map[string]any{
		"vectors": map[string]any{
			"size":     GeminiEmbeddingDimensions,
			"distance": "Cosine",
		},
	})
	if err != nil {
		return fmt.Errorf("could not encode Qdrant collection request: %w", err)
	}

	response, err := c.do(
		ctx,
		http.MethodPut,
		"/collections/"+url.PathEscape(c.collection),
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return qdrantResponseError(response)
}

// PointExists checks for a stable logical point ID. Qdrant accepts UUID or
// uint64 physical IDs, so the logical ID is deterministically converted to a
// UUID by qdrantPointID.
func (c *QdrantClient) PointExists(ctx context.Context, logicalID string) (bool, error) {
	response, err := c.do(
		ctx,
		http.MethodGet,
		"/collections/"+url.PathEscape(c.collection)+"/points/"+url.PathEscape(qdrantPointID(logicalID)),
		nil,
	)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if err := qdrantResponseError(response); err != nil {
		return false, err
	}
	return true, nil
}

// UpsertPoint writes one vector and payload to Qdrant. It does not modify any
// Navidrome database record.
func (c *QdrantClient) UpsertPoint(
	ctx context.Context,
	logicalID string,
	vector []float32,
	payload map[string]any,
) error {
	if strings.TrimSpace(logicalID) == "" {
		return fmt.Errorf("point ID is empty")
	}
	if len(vector) == 0 {
		return fmt.Errorf("point vector is empty")
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["ragId"] = logicalID

	body, err := json.Marshal(map[string]any{
		"points": []any{
			map[string]any{
				"id":      qdrantPointID(logicalID),
				"vector":  vector,
				"payload": payload,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("could not encode Qdrant point: %w", err)
	}

	response, err := c.do(
		ctx,
		http.MethodPut,
		"/collections/"+url.PathEscape(c.collection)+"/points?wait=true",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return qdrantResponseError(response)
}

// Search performs a read-only nearest-neighbor query and returns song payloads
// with their Qdrant similarity scores.
func (c *QdrantClient) Search(
	ctx context.Context,
	vector []float32,
	topK int,
) ([]SongSearchResult, error) {
	if len(vector) == 0 {
		return nil, fmt.Errorf("query vector is empty")
	}
	if topK <= 0 || topK > MaxSearchTopK {
		return nil, fmt.Errorf("topK must be between 1 and %d", MaxSearchTopK)
	}
	body, err := json.Marshal(map[string]any{
		"query":        vector,
		"limit":        topK,
		"with_payload": true,
		"with_vector":  false,
	})
	if err != nil {
		return nil, fmt.Errorf("could not encode Qdrant search request: %w", err)
	}

	response, err := c.do(
		ctx,
		http.MethodPost,
		"/collections/"+url.PathEscape(c.collection)+"/points/query",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if err := qdrantResponseError(response); err != nil {
		return nil, err
	}

	var result struct {
		Result struct {
			Points []struct {
				Score   float64 `json:"score"`
				Payload struct {
					SongID   string  `json:"songId"`
					Title    string  `json:"title"`
					Artist   string  `json:"artist"`
					Album    string  `json:"album"`
					Year     int     `json:"year"`
					Genre    string  `json:"genre"`
					Explicit bool    `json:"explicit"`
					BPM      int     `json:"bpm"`
					LUFS     float64 `json:"lufs"`
				} `json:"payload"`
			} `json:"points"`
		} `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid Qdrant search response: %w", err)
	}

	results := make([]SongSearchResult, 0, len(result.Result.Points))
	for _, point := range result.Result.Points {
		if strings.TrimSpace(point.Payload.SongID) == "" {
			continue
		}
		results = append(results, SongSearchResult{
			SongID:   point.Payload.SongID,
			Title:    point.Payload.Title,
			Artist:   point.Payload.Artist,
			Album:    point.Payload.Album,
			Year:     point.Payload.Year,
			Genre:    point.Payload.Genre,
			Explicit: point.Payload.Explicit,
			BPM:      point.Payload.BPM,
			LUFS:     point.Payload.LUFS,
			Score:    point.Score,
		})
	}
	return results, nil
}

// ListSongs scrolls indexed song payloads without returning their vectors.
func (c *QdrantClient) ListSongs(ctx context.Context, limit int) ([]IndexedSong, error) {
	if limit <= 0 || limit > MaxListLimit {
		return nil, fmt.Errorf("limit must be between 1 and %d", MaxListLimit)
	}
	body, err := json.Marshal(map[string]any{
		"limit":        limit,
		"with_payload": true,
		"with_vector":  false,
		"filter": map[string]any{
			"must": []any{
				map[string]any{
					"key":   "type",
					"match": map[string]any{"value": "song"},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not encode Qdrant scroll request: %w", err)
	}

	response, err := c.do(
		ctx,
		http.MethodPost,
		"/collections/"+url.PathEscape(c.collection)+"/points/scroll",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if err := qdrantResponseError(response); err != nil {
		return nil, err
	}

	var result struct {
		Result struct {
			Points []struct {
				Payload IndexedSong `json:"payload"`
			} `json:"points"`
		} `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid Qdrant scroll response: %w", err)
	}

	songs := make([]IndexedSong, 0, len(result.Result.Points))
	for _, point := range result.Result.Points {
		if strings.TrimSpace(point.Payload.SongID) == "" {
			continue
		}
		songs = append(songs, point.Payload)
	}
	return songs, nil
}

// StableSongPointID is the provider-independent point identity required by
// the RAG index. It is also stored in each Qdrant payload as ragId.
func StableSongPointID(songID string) string {
	return "song:" + songID
}

func qdrantPointID(logicalID string) string {
	digest := sha1.Sum([]byte(logicalID))
	id := digest[:16]
	id[6] = (id[6] & 0x0f) | 0x50 // UUID version 5
	id[8] = (id[8] & 0x3f) | 0x80 // RFC 4122 variant
	hexID := fmt.Sprintf("%x", id)
	return hexID[0:8] + "-" + hexID[8:12] + "-" + hexID[12:16] + "-" + hexID[16:20] + "-" + hexID[20:32]
}

func (c *QdrantClient) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	endpoint := c.baseURL + path
	parsed, err := url.ParseRequestURI(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid Qdrant URL %q", c.baseURL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("invalid Qdrant URL scheme %q", parsed.Scheme)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("could not create Qdrant request: %w", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: qdrantRequestTimeout}
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	return response, nil
}

func qdrantResponseError(response *http.Response) error {
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(response.StatusCode)
	}
	return fmt.Errorf("HTTP %d: %s", response.StatusCode, message)
}
