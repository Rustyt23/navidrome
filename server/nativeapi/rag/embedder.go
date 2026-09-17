package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	GeminiEmbeddingDimensions    = 768
	geminiEmbeddingModel         = "gemini-embedding-001"
	geminiEmbeddingEndpoint      = "https://generativelanguage.googleapis.com/v1beta/models/" + geminiEmbeddingModel + ":embedContent"
	geminiBatchEmbeddingEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/" + geminiEmbeddingModel + ":batchEmbedContents"
	// geminiMaxBatchSize is the request cap of the batchEmbedContents API.
	geminiMaxBatchSize = 100
)

// Embedder converts text into a vector without coupling the indexing pipeline
// to a specific embedding provider. EmbedTexts embeds a batch in one call where
// the provider supports it, which is the fast path for indexing.
type Embedder interface {
	EmbedText(ctx context.Context, text string) ([]float32, error)
	EmbedTexts(ctx context.Context, texts []string) ([][]float32, error)
}

type geminiEmbeddingPart struct {
	Text string `json:"text"`
}

// GeminiEmbedder implements Embedder using the existing Gemini API key.
type GeminiEmbedder struct {
	apiKey        string
	endpoint      string
	batchEndpoint string
	taskType      string
	httpClient    *http.Client
	httpOptions   HTTPClientOptions
}

// NewGeminiEmbedder creates a text embedder. The API key is validated when an
// embedding is requested so configuration errors are returned, never panicked.
func NewGeminiEmbedder(apiKey string, options ...HTTPClientOptions) *GeminiEmbedder {
	httpOptions := HTTPClientOptions{MaxRetries: 2}
	if len(options) > 0 {
		httpOptions = options[0]
	}
	httpOptions = normalizeHTTPClientOptions(httpOptions, 30*time.Second)
	return &GeminiEmbedder{
		apiKey:        strings.TrimSpace(apiKey),
		endpoint:      geminiEmbeddingEndpoint,
		batchEndpoint: geminiBatchEmbeddingEndpoint,
		taskType:      "RETRIEVAL_DOCUMENT",
		httpClient:    &http.Client{Timeout: httpOptions.Timeout},
		httpOptions:   httpOptions,
	}
}

// NewGeminiQueryEmbedder creates an embedder for search queries in the same
// vector space as retrieval-document embeddings.
func NewGeminiQueryEmbedder(apiKey string, options ...HTTPClientOptions) *GeminiEmbedder {
	embedder := NewGeminiEmbedder(apiKey, options...)
	embedder.taskType = "RETRIEVAL_QUERY"
	return embedder
}

// EmbedText generates an embedding with a fixed output size that matches the
// Qdrant collection created by this RAG package.
func (g *GeminiEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	if g == nil || strings.TrimSpace(g.apiKey) == "" {
		return nil, fmt.Errorf("Gemini API key is not configured")
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("embedding text is empty")
	}

	payload := struct {
		Content struct {
			Parts []geminiEmbeddingPart `json:"parts"`
		} `json:"content"`
		TaskType             string `json:"taskType"`
		OutputDimensionality int    `json:"outputDimensionality"`
	}{}
	payload.Content.Parts = []geminiEmbeddingPart{{Text: text}}
	payload.TaskType = g.taskType
	payload.OutputDimensionality = GeminiEmbeddingDimensions
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("could not encode Gemini embedding request: %w", err)
	}

	client := g.httpClient
	if client == nil {
		client = &http.Client{Timeout: g.httpOptions.Timeout}
	}
	started := time.Now()
	response, err := DoWithRetry(ctx, client, "gemini", g.httpOptions, func() (*http.Request, error) {
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(body))
		if requestErr != nil {
			return nil, requestCreationError("Gemini embedding", requestErr)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("x-goog-api-key", g.apiKey)
		return request, nil
	})
	if err != nil {
		observeRAGOperation("embedding_gemini", started, err)
		return nil, fmt.Errorf("Gemini embedding request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		errorBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		err = fmt.Errorf(
			"Gemini embedding API returned HTTP %d: %s",
			response.StatusCode,
			strings.TrimSpace(string(errorBody)),
		)
		observeRAGOperation("embedding_gemini", started, err)
		return nil, err
	}
	var result struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		err = fmt.Errorf("invalid Gemini embedding response: %w", err)
		observeRAGOperation("embedding_gemini", started, err)
		return nil, err
	}
	if len(result.Embedding.Values) == 0 {
		err = fmt.Errorf("Gemini embedding API returned an empty vector")
		observeRAGOperation("embedding_gemini", started, err)
		return nil, err
	}
	if len(result.Embedding.Values) != GeminiEmbeddingDimensions {
		err = fmt.Errorf(
			"Gemini embedding API returned %d dimensions; expected %d",
			len(result.Embedding.Values),
			GeminiEmbeddingDimensions,
		)
		observeRAGOperation("embedding_gemini", started, err)
		return nil, err
	}
	observeRAGOperation("embedding_gemini", started, nil)
	return result.Embedding.Values, nil
}

// EmbedTexts embeds a batch through the batchEmbedContents API, one request per
// geminiMaxBatchSize texts instead of one per text. A batch failure is returned
// as-is; the indexing pipeline already isolates bad items by falling back to
// per-song EmbedText calls.
func (g *GeminiEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if g == nil || strings.TrimSpace(g.apiKey) == "" {
		return nil, fmt.Errorf("Gemini API key is not configured")
	}
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	if strings.TrimSpace(g.batchEndpoint) == "" {
		// No batch endpoint configured: keep the sequential path working.
		vectors := make([][]float32, 0, len(texts))
		for _, text := range texts {
			vector, err := g.EmbedText(ctx, text)
			if err != nil {
				return nil, err
			}
			vectors = append(vectors, vector)
		}
		return vectors, nil
	}

	vectors := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += geminiMaxBatchSize {
		end := min(start+geminiMaxBatchSize, len(texts))
		chunk, err := g.embedTextsBatch(ctx, texts[start:end])
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, chunk...)
	}
	return vectors, nil
}

func (g *GeminiEmbedder) embedTextsBatch(ctx context.Context, texts []string) ([][]float32, error) {
	type batchRequest struct {
		Model   string `json:"model"`
		Content struct {
			Parts []geminiEmbeddingPart `json:"parts"`
		} `json:"content"`
		TaskType             string `json:"taskType"`
		OutputDimensionality int    `json:"outputDimensionality"`
	}
	requests := make([]batchRequest, 0, len(texts))
	for _, text := range texts {
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("embedding text is empty")
		}
		request := batchRequest{
			Model:                "models/" + geminiEmbeddingModel,
			TaskType:             g.taskType,
			OutputDimensionality: GeminiEmbeddingDimensions,
		}
		request.Content.Parts = []geminiEmbeddingPart{{Text: text}}
		requests = append(requests, request)
	}
	body, err := json.Marshal(map[string]any{"requests": requests})
	if err != nil {
		return nil, fmt.Errorf("could not encode Gemini batch embedding request: %w", err)
	}

	client := g.httpClient
	if client == nil {
		client = &http.Client{Timeout: g.httpOptions.Timeout}
	}
	started := time.Now()
	response, err := DoWithRetry(ctx, client, "gemini", g.httpOptions, func() (*http.Request, error) {
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, g.batchEndpoint, bytes.NewReader(body))
		if requestErr != nil {
			return nil, requestCreationError("Gemini batch embedding", requestErr)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("x-goog-api-key", g.apiKey)
		return request, nil
	})
	if err != nil {
		observeRAGOperation("embedding_gemini_batch", started, err)
		return nil, fmt.Errorf("Gemini batch embedding request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		errorBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		err = fmt.Errorf(
			"Gemini batch embedding API returned HTTP %d: %s",
			response.StatusCode,
			strings.TrimSpace(string(errorBody)),
		)
		observeRAGOperation("embedding_gemini_batch", started, err)
		return nil, err
	}
	var result struct {
		Embeddings []struct {
			Values []float32 `json:"values"`
		} `json:"embeddings"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		err = fmt.Errorf("invalid Gemini batch embedding response: %w", err)
		observeRAGOperation("embedding_gemini_batch", started, err)
		return nil, err
	}
	if len(result.Embeddings) != len(texts) {
		err = fmt.Errorf("Gemini batch embedding API returned %d vectors; expected %d", len(result.Embeddings), len(texts))
		observeRAGOperation("embedding_gemini_batch", started, err)
		return nil, err
	}
	vectors := make([][]float32, 0, len(texts))
	for i, embedding := range result.Embeddings {
		if len(embedding.Values) != GeminiEmbeddingDimensions {
			err = fmt.Errorf(
				"Gemini batch embedding vector %d has %d dimensions; expected %d",
				i,
				len(embedding.Values),
				GeminiEmbeddingDimensions,
			)
			observeRAGOperation("embedding_gemini_batch", started, err)
			return nil, err
		}
		vectors = append(vectors, embedding.Values)
	}
	observeRAGOperation("embedding_gemini_batch", started, nil)
	return vectors, nil
}

const gemmaEmbeddingTimeout = 60 * time.Second

// GemmaEmbedder produces embeddings from a local Gemma model (EmbeddingGemma)
// served through an Ollama-compatible /api/embed endpoint. EmbeddingGemma is
// built on Gemma 3 and outputs 768-dimensional vectors, matching the Qdrant
// collection, so it is a drop-in replacement for the Gemini embedder.
type GemmaEmbedder struct {
	endpoint    string
	model       string
	httpClient  *http.Client
	httpOptions HTTPClientOptions
}

// NewGemmaEmbedder creates a batch-capable embedder. endpoint should be the
// Ollama /api/embed URL; model defaults to "embeddinggemma".
func NewGemmaEmbedder(endpoint, model string, options ...HTTPClientOptions) *GemmaEmbedder {
	model = strings.TrimSpace(model)
	if model == "" {
		model = "embeddinggemma"
	}
	httpOptions := HTTPClientOptions{MaxRetries: 2}
	if len(options) > 0 {
		httpOptions = options[0]
	}
	httpOptions = normalizeHTTPClientOptions(httpOptions, gemmaEmbeddingTimeout)
	return &GemmaEmbedder{
		endpoint:    strings.TrimSpace(endpoint),
		model:       model,
		httpClient:  &http.Client{Timeout: httpOptions.Timeout},
		httpOptions: httpOptions,
	}
}

func (g *GemmaEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("embedding text is empty")
	}
	vectors, err := g.EmbedTexts(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("Gemma embedding API returned no vector")
	}
	return vectors[0], nil
}

// EmbedTexts embeds all texts in a single Ollama /api/embed request.
func (g *GemmaEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if g == nil || strings.TrimSpace(g.endpoint) == "" {
		return nil, fmt.Errorf("Gemma embedding URL is not configured")
	}
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	body, err := json.Marshal(map[string]any{"model": g.model, "input": texts})
	if err != nil {
		return nil, fmt.Errorf("could not encode Gemma embedding request: %w", err)
	}
	client := g.httpClient
	if client == nil {
		client = &http.Client{Timeout: g.httpOptions.Timeout}
	}
	started := time.Now()
	response, err := DoWithRetry(ctx, client, "gemma", g.httpOptions, func() (*http.Request, error) {
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(body))
		if requestErr != nil {
			return nil, requestCreationError("Gemma embedding", requestErr)
		}
		request.Header.Set("Content-Type", "application/json")
		return request, nil
	})
	if err != nil {
		observeRAGOperation("embedding_gemma", started, err)
		return nil, fmt.Errorf("Gemma embedding request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		errorBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		err = fmt.Errorf("Gemma embedding API returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(errorBody)))
		observeRAGOperation("embedding_gemma", started, err)
		return nil, err
	}
	var result struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		err = fmt.Errorf("invalid Gemma embedding response: %w", err)
		observeRAGOperation("embedding_gemma", started, err)
		return nil, err
	}
	if len(result.Embeddings) != len(texts) {
		err = fmt.Errorf("Gemma embedding API returned %d vectors; expected %d", len(result.Embeddings), len(texts))
		observeRAGOperation("embedding_gemma", started, err)
		return nil, err
	}
	for i, vector := range result.Embeddings {
		if len(vector) != GeminiEmbeddingDimensions {
			err = fmt.Errorf("Gemma embedding vector %d has %d dimensions; expected %d", i, len(vector), GeminiEmbeddingDimensions)
			observeRAGOperation("embedding_gemma", started, err)
			return nil, err
		}
	}
	observeRAGOperation("embedding_gemma", started, nil)
	return result.Embeddings, nil
}

var (
	_ Embedder = (*GeminiEmbedder)(nil)
	_ Embedder = (*GemmaEmbedder)(nil)
)
