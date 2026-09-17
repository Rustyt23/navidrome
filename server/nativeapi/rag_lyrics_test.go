package nativeapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navidrome/navidrome/conf"
)

func TestListQdrantLyricsSearchesStoredWords(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/collections":
			_, _ = w.Write([]byte(`{"result":{"collections":[{"name":"songs"}]},"status":"ok"}`))
		case "/collections/songs":
			_, _ = w.Write([]byte(`{"result":{"points_count":2,"config":{"params":{"vectors":{"size":768}}}},"status":"ok"}`))
		case "/collections/songs/points/scroll":
			var body struct {
				Filter map[string]any `json:"filter"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode Qdrant lyrics request: %v", err)
			}
			encoded, _ := json.Marshal(body.Filter)
			if !strings.Contains(string(encoded), `"text":"vikash"`) {
				t.Fatalf("expected exact lyrics text filter, got %s", encoded)
			}
			_, _ = w.Write([]byte(`{"result":{"points":[{"payload":{"songId":"song-1","title":"Vikash Song","artist":"Artist","hasLyrics":true,"lyricsText":"hello vikash"}}]},"status":"ok"}`))
		default:
			if strings.HasPrefix(request.URL.Path, "/collections/songs/points/") {
				_, _ = w.Write([]byte(`{"status":"ok","result":{"payload":{"indexVersion":3,"embeddingModel":"gemini:gemini-embedding-001","dimensions":768}}}`))
				return
			}
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	conf.Server.EnableRAG = true
	conf.Server.RAGVectorURL = server.URL
	conf.Server.RAGCollection = "songs"
	conf.Server.RAGEmbeddingURL = ""
	conf.Server.GeminiAPIKey = "test-key"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/ai/rag/lyrics?limit=10&query=vikash", nil)
	(&Router{}).handleListQdrantLyrics(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response qdrantLyricsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode lyrics response: %v", err)
	}
	if response.Collection != "songs" || response.Query != "vikash" || response.Count != 1 ||
		len(response.Songs) != 1 || response.Songs[0].LyricsText != "hello vikash" {
		t.Fatalf("unexpected Qdrant lyrics response: %+v", response)
	}
}

func TestAddQdrantLyricsRejectsInvalidLimit(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()
	conf.Server.EnableRAG = true
	conf.Server.GeminiAPIKey = "test-key"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/ai/rag/lyrics", strings.NewReader(`{"limit":501}`))
	(&Router{}).handleAddQdrantLyrics(recorder, request)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "limit must be between") {
		t.Fatalf("unexpected invalid limit response: %d %s", recorder.Code, recorder.Body.String())
	}
}
