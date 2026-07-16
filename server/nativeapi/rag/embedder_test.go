package rag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGeminiEmbedderMissingAPIKey(t *testing.T) {
	var embedder Embedder = NewGeminiEmbedder("")

	_, err := embedder.EmbedText(context.Background(), "song text")
	if err == nil || !strings.Contains(err.Error(), "Gemini API key is not configured") {
		t.Fatalf("expected missing API key error, got %v", err)
	}
}

func TestGeminiEmbedderRequestAndResponse(t *testing.T) {
	testGeminiEmbedderTaskType(t, NewGeminiEmbedder("test-key"), "RETRIEVAL_DOCUMENT")
}

func TestGeminiQueryEmbedderUsesRetrievalQueryTask(t *testing.T) {
	testGeminiEmbedderTaskType(t, NewGeminiQueryEmbedder("test-key"), "RETRIEVAL_QUERY")
}

func testGeminiEmbedderTaskType(t *testing.T, embedder *GeminiEmbedder, expectedTaskType string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", request.Method)
		}
		if request.Header.Get("x-goog-api-key") != "test-key" {
			t.Errorf("expected Gemini API key header")
		}
		var payload struct {
			Content struct {
				Parts []geminiEmbeddingPart `json:"parts"`
			} `json:"content"`
			TaskType             string `json:"taskType"`
			OutputDimensionality int    `json:"outputDimensionality"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(payload.Content.Parts) != 1 || payload.Content.Parts[0].Text != "song text" {
			t.Fatalf("unexpected content: %+v", payload.Content.Parts)
		}
		if payload.TaskType != expectedTaskType || payload.OutputDimensionality != GeminiEmbeddingDimensions {
			t.Fatalf("unexpected embedding config: %+v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		values := make([]float32, GeminiEmbeddingDimensions)
		values[0] = 0.1
		values[len(values)-1] = 0.3
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embedding": map[string]any{"values": values},
		})
	}))
	defer server.Close()

	embedder.endpoint = server.URL
	embedder.httpClient = server.Client()

	vector, err := embedder.EmbedText(context.Background(), "song text")
	if err != nil {
		t.Fatalf("embed text: %v", err)
	}
	if len(vector) != GeminiEmbeddingDimensions || vector[0] != float32(0.1) || vector[len(vector)-1] != float32(0.3) {
		t.Fatalf("unexpected vector: %#v", vector)
	}
}

func TestGeminiEmbedderRejectsUnexpectedDimensions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"embedding":{"values":[0.1,0.2,0.3]}}`))
	}))
	defer server.Close()

	embedder := NewGeminiEmbedder("test-key")
	embedder.endpoint = server.URL
	embedder.httpClient = server.Client()

	_, err := embedder.EmbedText(context.Background(), "song text")
	if err == nil || !strings.Contains(err.Error(), "returned 3 dimensions; expected 768") {
		t.Fatalf("expected dimension error, got %v", err)
	}
}

func TestGeminiEmbedderRetriesRateLimit(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		values := make([]float32, GeminiEmbeddingDimensions)
		_ = json.NewEncoder(w).Encode(map[string]any{"embedding": map[string]any{"values": values}})
	}))
	defer server.Close()

	embedder := NewGeminiEmbedder("test-key", HTTPClientOptions{
		Timeout: time.Second, MaxRetries: 1, RetryBackoff: time.Millisecond,
	})
	embedder.endpoint = server.URL
	embedder.httpClient = server.Client()
	if _, err := embedder.EmbedText(context.Background(), "song text"); err != nil || attempts != 2 {
		t.Fatalf("expected Gemini rate-limit retry, attempts=%d err=%v", attempts, err)
	}
}

func TestGeminiEmbedderBatchEmbedsInOneRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		requests++
		var payload struct {
			Requests []struct {
				Model   string `json:"model"`
				Content struct {
					Parts []geminiEmbeddingPart `json:"parts"`
				} `json:"content"`
				TaskType             string `json:"taskType"`
				OutputDimensionality int    `json:"outputDimensionality"`
			} `json:"requests"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode batch request: %v", err)
		}
		if len(payload.Requests) != 2 {
			t.Fatalf("expected 2 batched requests, got %d", len(payload.Requests))
		}
		for _, item := range payload.Requests {
			if item.Model != "models/"+geminiEmbeddingModel ||
				item.TaskType != "RETRIEVAL_DOCUMENT" ||
				item.OutputDimensionality != GeminiEmbeddingDimensions {
				t.Fatalf("unexpected batch item: %+v", item)
			}
		}
		first := make([]float32, GeminiEmbeddingDimensions)
		first[0] = 0.1
		second := make([]float32, GeminiEmbeddingDimensions)
		second[0] = 0.2
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embeddings": []map[string]any{{"values": first}, {"values": second}},
		})
	}))
	defer server.Close()

	embedder := NewGeminiEmbedder("test-key")
	embedder.batchEndpoint = server.URL
	embedder.httpClient = server.Client()

	vectors, err := embedder.EmbedTexts(context.Background(), []string{"song one", "song two"})
	if err != nil {
		t.Fatalf("embed texts: %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected a single batch request, got %d", requests)
	}
	if len(vectors) != 2 || vectors[0][0] != float32(0.1) || vectors[1][0] != float32(0.2) {
		t.Fatalf("unexpected vectors: %v %v", vectors[0][0], vectors[1][0])
	}
}

func TestGeminiEmbedderBatchRejectsCountMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		values := make([]float32, GeminiEmbeddingDimensions)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embeddings": []map[string]any{{"values": values}},
		})
	}))
	defer server.Close()

	embedder := NewGeminiEmbedder("test-key")
	embedder.batchEndpoint = server.URL
	embedder.httpClient = server.Client()

	_, err := embedder.EmbedTexts(context.Background(), []string{"song one", "song two"})
	if err == nil || !strings.Contains(err.Error(), "returned 1 vectors; expected 2") {
		t.Fatalf("expected count mismatch error, got %v", err)
	}
}
