package rag

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // Used only for deterministic UUID generation, not security.
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const qdrantRequestTimeout = 15 * time.Second

type QdrantClientOptions struct {
	HTTPClientOptions
	ExpectedSchema *IndexSchema
}

// QdrantStatus describes only connection and collection state. It does not
// perform embeddings, indexing, or vector search.
type QdrantStatus struct {
	VectorDBOnline   bool
	CollectionExists bool
	IndexedCount     int64
	ReindexRequired  bool
	Error            string
}

// QdrantClient is the small REST client used by the RAG status endpoint.
type QdrantClient struct {
	baseURL        string
	collection     string
	httpClient     *http.Client
	httpOptions    HTTPClientOptions
	expectedSchema *IndexSchema
}

// NewQdrantClient creates a bounded, retrying Qdrant REST client.
func NewQdrantClient(baseURL, collection string, options ...QdrantClientOptions) *QdrantClient {
	clientOptions := QdrantClientOptions{HTTPClientOptions: HTTPClientOptions{MaxRetries: 2}}
	if len(options) > 0 {
		clientOptions = options[0]
	}
	clientOptions.HTTPClientOptions = normalizeHTTPClientOptions(clientOptions.HTTPClientOptions, qdrantRequestTimeout)
	return &QdrantClient{
		baseURL:        strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		collection:     strings.TrimSpace(collection),
		httpClient:     &http.Client{Timeout: clientOptions.Timeout},
		httpOptions:    clientOptions.HTTPClientOptions,
		expectedSchema: clientOptions.ExpectedSchema,
	}
}

// Status checks Qdrant and optionally creates a compatible missing collection.
func (c *QdrantClient) Status(ctx context.Context, createIfMissing bool) QdrantStatus {
	status := QdrantStatus{}
	if err := c.CheckReachable(ctx); err != nil {
		status.Error = fmt.Sprintf("Qdrant unavailable: %v", err)
		return status
	}
	status.VectorDBOnline = true

	details, err := c.collectionDetails(ctx)
	if err != nil {
		status.Error = fmt.Sprintf("Could not check Qdrant collection: %v", err)
		return status
	}
	status.CollectionExists = details.Exists
	status.IndexedCount = details.PointsCount

	if !details.Exists && createIfMissing {
		if err := c.CreateCollection(ctx); err != nil {
			status.Error = fmt.Sprintf("Could not create Qdrant collection: %v", err)
			return status
		}
		status.CollectionExists = true
		status.IndexedCount = 0
		return status
	}

	if details.Exists && c.expectedSchema != nil {
		if err := c.validateIndexSchema(ctx, details, createIfMissing); err != nil {
			status.ReindexRequired = errors.Is(err, ErrIndexSchemaDrift)
			status.Error = err.Error()
			return status
		}
		// The collection-level metadata sentinel is not an indexed song.
		if status.IndexedCount > 0 {
			status.IndexedCount--
		}
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
	details, err := c.collectionDetails(ctx)
	return details.Exists, details.PointsCount, err
}

type collectionDetails struct {
	Exists      bool
	PointsCount int64
	Dimensions  int
}

func (c *QdrantClient) collectionDetails(ctx context.Context) (collectionDetails, error) {
	if c.collection == "" {
		return collectionDetails{}, fmt.Errorf("collection name is empty")
	}

	response, err := c.do(
		ctx,
		http.MethodGet,
		"/collections/"+url.PathEscape(c.collection),
		nil,
	)
	if err != nil {
		return collectionDetails{}, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return collectionDetails{}, nil
	}
	if err := qdrantResponseError(response); err != nil {
		return collectionDetails{}, err
	}

	var payload struct {
		Result struct {
			PointsCount *int64 `json:"points_count"`
			Config      struct {
				Params struct {
					Vectors struct {
						Size int `json:"size"`
					} `json:"vectors"`
				} `json:"params"`
			} `json:"config"`
		} `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return collectionDetails{}, fmt.Errorf("invalid collection response: %w", err)
	}
	details := collectionDetails{Exists: true, Dimensions: payload.Result.Config.Params.Vectors.Size}
	if payload.Result.PointsCount == nil {
		return details, nil
	}
	details.PointsCount = *payload.Result.PointsCount
	return details, nil
}

// CreateCollection creates the dense vector collection and its schema sentinel.
func (c *QdrantClient) CreateCollection(ctx context.Context) error {
	if c.collection == "" {
		return fmt.Errorf("collection name is empty")
	}

	dimensions := GeminiEmbeddingDimensions
	if c.expectedSchema != nil {
		dimensions = c.expectedSchema.normalized().Dimensions
	}
	body, err := json.Marshal(map[string]any{
		"vectors": map[string]any{
			"size":     dimensions,
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
	if err := qdrantResponseError(response); err != nil {
		return err
	}
	if c.expectedSchema != nil {
		return c.writeIndexMetadata(ctx, c.expectedSchema.normalized())
	}
	return nil
}

// RecreateCollection is the explicit migration path for an incompatible index.
// It deletes only the configured Qdrant collection and immediately recreates
// it with the expected schema metadata.
func (c *QdrantClient) RecreateCollection(ctx context.Context) error {
	response, err := c.do(ctx, http.MethodDelete, "/collections/"+url.PathEscape(c.collection), nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		if err := qdrantResponseError(response); err != nil {
			return fmt.Errorf("could not delete incompatible Qdrant collection: %w", err)
		}
	}
	return c.CreateCollection(ctx)
}

func (c *QdrantClient) validateIndexSchema(ctx context.Context, details collectionDetails, initializeEmpty bool) error {
	expected := c.expectedSchema.normalized()
	if details.Dimensions > 0 && details.Dimensions != expected.Dimensions {
		return fmt.Errorf(
			"%w: RAG index dimension drift detected for collection %q (stored=%d expected=%d); delete and recreate the collection, then run a full RAG reindex",
			ErrIndexSchemaDrift,
			c.collection,
			details.Dimensions,
			expected.Dimensions,
		)
	}
	actual, found, err := c.readIndexMetadata(ctx)
	if err != nil {
		return fmt.Errorf("could not read RAG index metadata: %w", err)
	}
	if !found {
		if details.PointsCount == 0 && initializeEmpty {
			if err := c.writeIndexMetadata(ctx, expected); err != nil {
				return fmt.Errorf("could not initialize RAG index metadata: %w", err)
			}
			return nil
		}
		return fmt.Errorf(
			"%w: RAG index metadata is missing for collection %q; delete and recreate the collection, then run a full RAG reindex",
			ErrIndexSchemaDrift,
			c.collection,
		)
	}
	if err := expected.compatibilityError(actual); err != nil {
		return fmt.Errorf(
			"%w: RAG index schema drift detected for collection %q (%v); delete and recreate the collection, then run a full RAG reindex",
			ErrIndexSchemaDrift,
			c.collection,
			err,
		)
	}
	return nil
}

func (c *QdrantClient) readIndexMetadata(ctx context.Context) (IndexSchema, bool, error) {
	response, err := c.do(
		ctx,
		http.MethodGet,
		"/collections/"+url.PathEscape(c.collection)+"/points/"+url.PathEscape(qdrantPointID(indexMetadataPointID)),
		nil,
	)
	if err != nil {
		return IndexSchema{}, false, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return IndexSchema{}, false, nil
	}
	if err := qdrantResponseError(response); err != nil {
		return IndexSchema{}, false, err
	}
	var result struct {
		Result struct {
			Payload IndexSchema `json:"payload"`
		} `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return IndexSchema{}, false, fmt.Errorf("invalid index metadata response: %w", err)
	}
	return result.Result.Payload, true, nil
}

func (c *QdrantClient) writeIndexMetadata(ctx context.Context, schema IndexSchema) error {
	payload := map[string]any{
		"type":           "rag_index_metadata",
		"indexVersion":   schema.Version,
		"embeddingModel": schema.EmbeddingModel,
		"dimensions":     schema.Dimensions,
	}
	vector := make([]float32, schema.Dimensions)
	vector[0] = 1 // valid cosine vector; metadata is excluded from every song query
	return c.UpsertPoint(ctx, indexMetadataPointID, vector, payload)
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

// PointUpsert is a single vector + payload to write in a batch.
type PointUpsert struct {
	LogicalID string
	Vector    []float32
	Payload   map[string]any
}

// UpsertPoints writes many points in one request. It is the batch fast path for
// indexing and does not modify any Navidrome record.
func (c *QdrantClient) UpsertPoints(ctx context.Context, points []PointUpsert) error {
	if len(points) == 0 {
		return nil
	}
	encoded := make([]any, 0, len(points))
	for _, point := range points {
		if strings.TrimSpace(point.LogicalID) == "" || len(point.Vector) == 0 {
			return fmt.Errorf("point ID or vector is empty")
		}
		payload := point.Payload
		if payload == nil {
			payload = map[string]any{}
		}
		payload["ragId"] = point.LogicalID
		encoded = append(encoded, map[string]any{
			"id":      qdrantPointID(point.LogicalID),
			"vector":  point.Vector,
			"payload": payload,
		})
	}
	body, err := json.Marshal(map[string]any{"points": encoded})
	if err != nil {
		return fmt.Errorf("could not encode Qdrant points: %w", err)
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

// ExistingContentHashes retrieves the stored contentHash for the given logical
// IDs in a single request. The returned map is keyed by logical ID; missing IDs
// are simply absent, so the caller can tell new songs (absent) from changed
// songs (present but different hash).
func (c *QdrantClient) ExistingContentHashes(ctx context.Context, logicalIDs []string) (map[string]string, error) {
	hashes := make(map[string]string, len(logicalIDs))
	if len(logicalIDs) == 0 {
		return hashes, nil
	}
	ids := make([]string, 0, len(logicalIDs))
	for _, id := range logicalIDs {
		ids = append(ids, qdrantPointID(id))
	}
	body, err := json.Marshal(map[string]any{
		"ids":          ids,
		"with_payload": []string{"ragId", "contentHash"},
		"with_vector":  false,
	})
	if err != nil {
		return nil, fmt.Errorf("could not encode Qdrant retrieve request: %w", err)
	}
	response, err := c.do(
		ctx,
		http.MethodPost,
		"/collections/"+url.PathEscape(c.collection)+"/points",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if err := qdrantResponseError(response); err != nil {
		return nil, err
	}
	var parsed struct {
		Result []struct {
			Payload struct {
				RagID       string `json:"ragId"`
				ContentHash string `json:"contentHash"`
			} `json:"payload"`
		} `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("invalid Qdrant retrieve response: %w", err)
	}
	for _, point := range parsed.Result {
		if point.Payload.RagID != "" {
			hashes[point.Payload.RagID] = point.Payload.ContentHash
		}
	}
	return hashes, nil
}

// DeletePoints removes the given logical IDs from the collection.
func (c *QdrantClient) DeletePoints(ctx context.Context, logicalIDs []string) error {
	if len(logicalIDs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(logicalIDs))
	for _, id := range logicalIDs {
		ids = append(ids, qdrantPointID(id))
	}
	body, err := json.Marshal(map[string]any{"points": ids})
	if err != nil {
		return fmt.Errorf("could not encode Qdrant delete request: %w", err)
	}
	response, err := c.do(
		ctx,
		http.MethodPost,
		"/collections/"+url.PathEscape(c.collection)+"/points/delete?wait=true",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return qdrantResponseError(response)
}

// AllIndexedSongIDs scrolls the collection and returns the logical IDs of every
// indexed song point, paging until the collection is exhausted or maxTotal is
// reached. It is used to detect points whose songs no longer exist.
func (c *QdrantClient) AllIndexedSongIDs(ctx context.Context, maxTotal int) ([]string, error) {
	if maxTotal <= 0 {
		maxTotal = 100000
	}
	ids := make([]string, 0, 256)
	var offset any
	for len(ids) < maxTotal {
		request := map[string]any{
			"limit":        512,
			"with_payload": []string{"ragId"},
			"with_vector":  false,
			"filter": map[string]any{
				"must": []any{map[string]any{"key": "type", "match": map[string]any{"value": "song"}}},
			},
		}
		if offset != nil {
			request["offset"] = offset
		}
		body, err := json.Marshal(request)
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
		if err := qdrantResponseError(response); err != nil {
			response.Body.Close()
			return nil, err
		}
		var parsed struct {
			Result struct {
				Points []struct {
					Payload struct {
						RagID string `json:"ragId"`
					} `json:"payload"`
				} `json:"points"`
				NextPageOffset any `json:"next_page_offset"`
			} `json:"result"`
		}
		if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
			response.Body.Close()
			return nil, fmt.Errorf("invalid Qdrant scroll response: %w", err)
		}
		response.Body.Close()
		for _, point := range parsed.Result.Points {
			if point.Payload.RagID != "" {
				ids = append(ids, point.Payload.RagID)
			}
		}
		if parsed.Result.NextPageOffset == nil || len(parsed.Result.Points) == 0 {
			break
		}
		offset = parsed.Result.NextPageOffset
	}
	return ids, nil
}

// Search performs a read-only nearest-neighbor query and returns song payloads
// with their Qdrant similarity scores.
func (c *QdrantClient) Search(
	ctx context.Context,
	vector []float32,
	topK int,
	filters ...SearchFilters,
) ([]SongSearchResult, error) {
	if len(vector) == 0 {
		return nil, fmt.Errorf("query vector is empty")
	}
	if topK <= 0 || topK > MaxSearchTopK {
		return nil, fmt.Errorf("topK must be between 1 and %d", MaxSearchTopK)
	}
	requestBody := map[string]any{
		"query":        vector,
		"limit":        topK,
		"with_payload": true,
		"with_vector":  false,
	}
	searchFilters := SearchFilters{}
	if len(filters) > 0 {
		searchFilters = filters[0]
	}
	typeCondition := map[string]any{
		"key":   "type",
		"match": map[string]any{"value": "song"},
	}
	if filter := BuildQdrantFilter(searchFilters); filter != nil {
		must, _ := filter["must"].([]any)
		filter["must"] = append([]any{typeCondition}, must...)
		requestBody["filter"] = filter
	} else {
		requestBody["filter"] = map[string]any{"must": []any{typeCondition}}
	}
	body, err := json.Marshal(requestBody)
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
				Score   float64     `json:"score"`
				Payload IndexedSong `json:"payload"`
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
			SongID:       point.Payload.SongID,
			Title:        point.Payload.Title,
			Artist:       point.Payload.Artist,
			Album:        point.Payload.Album,
			Year:         point.Payload.Year,
			Genre:        point.Payload.Genre,
			Explicit:     point.Payload.Explicit,
			BPM:          point.Payload.BPM,
			LUFS:         point.Payload.LUFS,
			Duration:     point.Payload.Duration,
			PlayCount:    point.Payload.PlayCount,
			LastPlayedAt: point.Payload.LastPlayedAt,
			HasLyrics:    point.Payload.HasLyrics,
			HasGenre:     point.Payload.HasGenre,
			HasYear:      point.Payload.HasYear,
			HasBPM:       point.Payload.HasBPM,
			HasLUFS:      point.Payload.HasLUFS,
			Score:        point.Score,
			LyricsText:   point.Payload.LyricsText,
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

// SongVector is an indexed song plus its embedding, used for read-only
// duplicate and alternate-version analysis.
type SongVector struct {
	Song   IndexedSong
	Vector []float32
}

func (c *QdrantClient) ListSongVectors(ctx context.Context, limit int) ([]SongVector, error) {
	if limit <= 0 || limit > 5000 {
		return nil, fmt.Errorf("vector list limit must be between 1 and 5000")
	}
	items := make([]SongVector, 0, min(limit, 512))
	var offset any
	for len(items) < limit {
		request := map[string]any{
			"limit":        min(512, limit-len(items)),
			"with_payload": true,
			"with_vector":  true,
			"filter": map[string]any{"must": []any{map[string]any{
				"key": "type", "match": map[string]any{"value": "song"},
			}}},
		}
		if offset != nil {
			request["offset"] = offset
		}
		body, err := json.Marshal(request)
		if err != nil {
			return nil, fmt.Errorf("could not encode Qdrant vector scroll request: %w", err)
		}
		response, err := c.do(ctx, http.MethodPost, "/collections/"+url.PathEscape(c.collection)+"/points/scroll", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		if err := qdrantResponseError(response); err != nil {
			response.Body.Close()
			return nil, err
		}
		var result struct {
			Result struct {
				Points []struct {
					Vector  []float32   `json:"vector"`
					Payload IndexedSong `json:"payload"`
				} `json:"points"`
				NextPageOffset any `json:"next_page_offset"`
			} `json:"result"`
		}
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
			response.Body.Close()
			return nil, fmt.Errorf("invalid Qdrant vector scroll response: %w", err)
		}
		response.Body.Close()
		for _, point := range result.Result.Points {
			if point.Payload.SongID != "" && len(point.Vector) > 0 {
				items = append(items, SongVector{Song: point.Payload, Vector: point.Vector})
			}
		}
		if result.Result.NextPageOffset == nil || len(result.Result.Points) == 0 {
			break
		}
		offset = result.Result.NextPageOffset
	}
	return items, nil
}

// CountDocumentsByType returns an exact filtered point count without loading
// vectors or payloads. This keeps playlist points from inflating song coverage.
func (c *QdrantClient) CountDocumentsByType(ctx context.Context, documentType string) (int64, error) {
	documentType = strings.TrimSpace(documentType)
	if documentType == "" {
		return 0, fmt.Errorf("document type is empty")
	}
	body, err := json.Marshal(map[string]any{
		"exact": true,
		"filter": map[string]any{
			"must": []any{map[string]any{
				"key": "type", "match": map[string]any{"value": documentType},
			}},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("could not encode Qdrant count request: %w", err)
	}
	response, err := c.do(
		ctx,
		http.MethodPost,
		"/collections/"+url.PathEscape(c.collection)+"/points/count",
		bytes.NewReader(body),
	)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if err := qdrantResponseError(response); err != nil {
		return 0, err
	}
	var result struct {
		Result struct {
			Count int64 `json:"count"`
		} `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("invalid Qdrant count response: %w", err)
	}
	return result.Result.Count, nil
}

// StableSongPointID is the provider-independent point identity required by
// the RAG index. It is also stored in each Qdrant payload as ragId.
func StableSongPointID(songID string) string {
	return "song:" + songID
}

// StablePlaylistPointID prevents playlist IDs from colliding with song IDs in
// the shared collection.
func StablePlaylistPointID(playlistID string) string {
	return "playlist:" + playlistID
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

	var bodyBytes []byte
	if body != nil {
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("could not read Qdrant request body: %w", err)
		}
	}

	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: c.httpOptions.Timeout}
	}
	started := time.Now()
	response, err := DoWithRetry(ctx, client, "qdrant", c.httpOptions, func() (*http.Request, error) {
		var requestBody io.Reader
		if body != nil {
			requestBody = bytes.NewReader(bodyBytes)
		}
		request, requestErr := http.NewRequestWithContext(ctx, method, endpoint, requestBody)
		if requestErr != nil {
			return nil, requestCreationError("Qdrant", requestErr)
		}
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		return request, nil
	})
	if err != nil {
		observeRAGOperation("qdrant_http", started, err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var statusErr error
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		statusErr = fmt.Errorf("HTTP %d", response.StatusCode)
	}
	observeRAGOperation("qdrant_http", started, statusErr)
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
