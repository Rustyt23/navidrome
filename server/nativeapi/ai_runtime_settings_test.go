package nativeapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/navidrome/navidrome/conf"
)

func TestRAGEnabledRuntimeSetting(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()
	defer resetAIRuntimeSettingsForTest()
	resetAIRuntimeSettingsForTest()
	conf.Server.EnableRAG = true

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/ai/rag/enabled",
		bytes.NewBufferString(`{"enabled":false}`),
	)
	(&Router{}).handleRAGEnabled(recorder, request)

	if recorder.Code != http.StatusOK || ragEnabled() {
		t.Fatalf("expected disabled runtime RAG, code=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response map[string]bool
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["enabled"] {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestWhisperModelRuntimeSetting(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()
	defer resetAIRuntimeSettingsForTest()
	resetAIRuntimeSettingsForTest()
	conf.Server.WhisperModel = "large-v3"

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/ai/whisper/model",
		bytes.NewBufferString(`{"model":"small"}`),
	)
	(&Router{}).handleWhisperModel(recorder, request)

	if recorder.Code != http.StatusOK || selectedWhisperModel() != "small" {
		t.Fatalf("expected small model, code=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestWhisperModelRejectsUnknownModel(t *testing.T) {
	defer resetAIRuntimeSettingsForTest()
	resetAIRuntimeSettingsForTest()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/ai/whisper/model",
		bytes.NewBufferString(`{"model":"unknown"}`),
	)
	(&Router{}).handleWhisperModel(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
