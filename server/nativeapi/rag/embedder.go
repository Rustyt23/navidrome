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
	GeminiEmbeddingDimensions = 768
	geminiEmbeddingModel      = "gemini-embedding-001"
	geminiEmbeddingEndpoint   = "https://generativelanguage.googleapis.com/v1beta/models/" + geminiEmbeddingModel + ":embedContent"
)

// Embedder converts text into a vector without coupling the indexing pipeline
// to a specific embedding provider.
type Embedder interface {
	EmbedText(ctx context.Context, text string) ([]float32, error)
}

type geminiEmbeddingPart struct {
	Text string `json:"text"`
}

// GeminiEmbedder implements Embedder using the existing Gemini API key.
type GeminiEmbedder struct {
	apiKey     string
	endpoint   string
	taskType   string
	httpClient *http.Client
}

// NewGeminiEmbedder creates a text embedder. The API key is validated when an
// embedding is requested so configuration errors are returned, never panicked.
func NewGeminiEmbedder(apiKey string) *GeminiEmbedder {
	return &GeminiEmbedder{
		apiKey:     strings.TrimSpace(apiKey),
		endpoint:   geminiEmbeddingEndpoint,
		taskType:   "RETRIEVAL_DOCUMENT",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// NewGeminiQueryEmbedder creates an embedder for search queries in the same
// vector space as retrieval-document embeddings.
func NewGeminiQueryEmbedder(apiKey string) *GeminiEmbedder {
	embedder := NewGeminiEmbedder(apiKey)
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

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create Gemini embedding request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-goog-api-key", g.apiKey)

	client := g.httpClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Gemini embedding request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		errorBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf(
			"Gemini embedding API returned HTTP %d: %s",
			response.StatusCode,
			strings.TrimSpace(string(errorBody)),
		)
	}

	var result struct {
		Embedding struct {
			Values []float32 `json:"values"`
		} `json:"embedding"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid Gemini embedding response: %w", err)
	}
	if len(result.Embedding.Values) == 0 {
		return nil, fmt.Errorf("Gemini embedding API returned an empty vector")
	}
	if len(result.Embedding.Values) != GeminiEmbeddingDimensions {
		return nil, fmt.Errorf(
			"Gemini embedding API returned %d dimensions; expected %d",
			len(result.Embedding.Values),
			GeminiEmbeddingDimensions,
		)
	}
	return result.Embedding.Values, nil
}

var _ Embedder = (*GeminiEmbedder)(nil)
