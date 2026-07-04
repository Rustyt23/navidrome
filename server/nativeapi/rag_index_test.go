package nativeapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/navidrome/navidrome/conf"
)

func TestDecodeRAGIndexRequest(t *testing.T) {
	payload, err := decodeRAGIndexRequest(bytes.NewBufferString(`{"limit":25,"force":true}`))
	if err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if payload.Limit != 25 || !payload.Force {
		t.Fatalf("unexpected payload: %+v", payload)
	}

	if _, err := decodeRAGIndexRequest(bytes.NewBufferString(`{"limit":501,"force":false}`)); err == nil {
		t.Fatal("expected a limit above 500 to fail")
	}
	if _, err := decodeRAGIndexRequest(bytes.NewBufferString(`{"limit":1,"unknown":true}`)); err == nil {
		t.Fatal("expected unknown field to fail")
	}
}

func TestRAGIndexDisabled(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()
	conf.Server.EnableRAG = false

	request := httptest.NewRequest(http.MethodPost, "/ai/rag/index", bytes.NewBufferString(`{"limit":50,"force":false}`))
	recorder := httptest.NewRecorder()
	(&Router{}).handleRAGIndex(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}
	var response map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["error"] != "RAG is disabled" {
		t.Fatalf("unexpected error response: %#v", response)
	}
}
