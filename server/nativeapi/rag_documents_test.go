package nativeapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navidrome/navidrome/conf"
)

func TestRAGDocumentsDisabled(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()
	conf.Server.EnableRAG = false

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/ai/rag/documents", nil)
	(&Router{}).handleRAGDocuments(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "RAG is disabled") {
		t.Fatalf("unexpected disabled response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestRAGDocumentsListsCollectionPayloads(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/collections":
			_, _ = w.Write([]byte(`{"result":{"collections":[{"name":"songs"}]},"status":"ok"}`))
		case "/collections/songs":
			_, _ = w.Write([]byte(`{"result":{"points_count":8,"config":{"params":{"vectors":{"size":768}}}},"status":"ok"}`))
		case "/collections/songs/points/scroll":
			_, _ = w.Write([]byte(`{"result":{"points":[{"payload":{"songId":"song-1","title":"Bright Song","artist":"Artist","album":"Album","year":2020,"genre":"Pop","explicit":false,"bpm":100,"lufs":-12.5}}]},"status":"ok"}`))
		default:
			if strings.HasPrefix(request.URL.Path, "/collections/songs/points/") {
				_, _ = w.Write([]byte(`{"status":"ok","result":{"payload":{"indexVersion":2,"embeddingModel":"gemini:gemini-embedding-001","dimensions":768}}}`))
			} else {
				http.NotFound(w, request)
			}
		}
	}))
	defer server.Close()

	conf.Server.EnableRAG = true
	conf.Server.RAGVectorURL = server.URL
	conf.Server.RAGCollection = "songs"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/ai/rag/documents?limit=10", nil)
	(&Router{}).handleRAGDocuments(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response ragDocumentsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Collection != "songs" || response.IndexedCount != 7 || len(response.Songs) != 1 || response.Songs[0].SongID != "song-1" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestParseRAGDocumentsLimit(t *testing.T) {
	if limit, err := parseRAGDocumentsLimit(""); err != nil || limit != defaultRAGDocumentsLimit {
		t.Fatalf("unexpected default limit: %d %v", limit, err)
	}
	if _, err := parseRAGDocumentsLimit("501"); err == nil {
		t.Fatal("expected oversized limit error")
	}
}
