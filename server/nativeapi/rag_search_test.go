package nativeapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

func TestRAGSearchEndpoint(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()
	conf.Server.EnableRAG = true
	conf.Server.RAGTopK = 20

	var receivedQuery string
	var receivedTopK int
	search := func(_ context.Context, query string, topK int) ([]rag.SongSearchResult, error) {
		receivedQuery = query
		receivedTopK = topK
		return []rag.SongSearchResult{{
			SongID: "song-1", Title: "Bright Song", Artist: "Artist", Album: "Album",
			Year: 2020, Genre: "Pop", BPM: 100, LUFS: -12.5, Score: 0.87,
		}}, nil
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/ai/rag/search",
		bytes.NewBufferString(`{"query":"clean upbeat retail songs","topK":20}`),
	)
	recorder := httptest.NewRecorder()

	serveRAGSearch(recorder, request, search)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if receivedQuery != "clean upbeat retail songs" || receivedTopK != 20 {
		t.Fatalf("unexpected search input: query=%q topK=%d", receivedQuery, receivedTopK)
	}
	var response ragSearchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Results) != 1 || response.Results[0].SongID != "song-1" || response.Results[0].Score != 0.87 {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestRAGSearchDisabled(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()
	conf.Server.EnableRAG = false

	called := false
	request := httptest.NewRequest(http.MethodPost, "/api/ai/rag/search", bytes.NewBufferString(`{"query":"songs","topK":20}`))
	recorder := httptest.NewRecorder()
	serveRAGSearch(recorder, request, func(context.Context, string, int) ([]rag.SongSearchResult, error) {
		called = true
		return nil, nil
	})

	if recorder.Code != http.StatusServiceUnavailable || called {
		t.Fatalf("expected disabled response without search call, code=%d called=%t", recorder.Code, called)
	}
	if !strings.Contains(recorder.Body.String(), "RAG is disabled") {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
}

func TestRAGSearchQdrantOffline(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	serverURL := server.URL
	server.Close()

	conf.Server.EnableRAG = true
	conf.Server.GeminiAPIKey = "test-key"
	conf.Server.RAGVectorURL = serverURL
	conf.Server.RAGCollection = "songs"

	_, err := searchRAG(context.Background(), "songs", 20)
	if err == nil || !strings.Contains(err.Error(), "Qdrant unavailable") {
		t.Fatalf("expected useful Qdrant offline error, got %v", err)
	}
}

func TestPrepareAIChatMessage(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()

	t.Run("includes RAG context when enabled", func(t *testing.T) {
		conf.Server.EnableRAG = true
		conf.Server.RAGTopK = 5
		prompt, sources, err := prepareAIChatMessage(
			context.Background(),
			"find upbeat songs",
			func(_ context.Context, query string, topK int) ([]rag.SongSearchResult, error) {
				if query != "find upbeat songs" || topK != 5 {
					t.Fatalf("unexpected search input: %q %d", query, topK)
				}
				return []rag.SongSearchResult{{Title: "Bright Song", Artist: "Artist", Score: 0.9}}, nil
			},
		)
		if err != nil {
			t.Fatalf("prepare chat: %v", err)
		}
		if len(sources) != 1 || !strings.Contains(prompt, "Bright Song — Artist") {
			t.Fatalf("expected RAG prompt and sources, prompt=%q sources=%+v", prompt, sources)
		}
	})

	t.Run("keeps original chat message when disabled", func(t *testing.T) {
		conf.Server.EnableRAG = false
		called := false
		prompt, sources, err := prepareAIChatMessage(
			context.Background(),
			"normal chat",
			func(context.Context, string, int) ([]rag.SongSearchResult, error) {
				called = true
				return nil, nil
			},
		)
		if err != nil || prompt != "normal chat" || len(sources) != 0 || called {
			t.Fatalf("unexpected disabled chat behavior: prompt=%q sources=%+v called=%t err=%v", prompt, sources, called, err)
		}
	})

	t.Run("returns original chat message when retrieval fails", func(t *testing.T) {
		conf.Server.EnableRAG = true
		prompt, sources, err := prepareAIChatMessage(
			context.Background(),
			"normal chat",
			func(context.Context, string, int) ([]rag.SongSearchResult, error) {
				return nil, errors.New("embedding unavailable")
			},
		)
		if err == nil || prompt != "normal chat" || len(sources) != 0 {
			t.Fatalf("expected non-blocking chat fallback: prompt=%q sources=%+v err=%v", prompt, sources, err)
		}
	})
}
