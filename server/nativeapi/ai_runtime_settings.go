package nativeapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/navidrome/navidrome/conf"
)

var ragEnabledOverride atomic.Int32
var whisperModelOverride atomic.Pointer[string]

var supportedWhisperModels = []string{
	"tiny",
	"base",
	"small",
	"medium",
	"large-v3",
	"turbo",
}

func ragEnabled() bool {
	switch ragEnabledOverride.Load() {
	case 1:
		return false
	case 2:
		return true
	default:
		return conf.Server.EnableRAG
	}
}

func setRAGEnabled(enabled bool) {
	if enabled {
		ragEnabledOverride.Store(2)
		return
	}
	ragEnabledOverride.Store(1)
}

func selectedWhisperModel() string {
	if model := whisperModelOverride.Load(); model != nil {
		return *model
	}
	model := strings.TrimSpace(conf.Server.WhisperModel)
	if model == "" {
		return "large-v3"
	}
	return model
}

func setWhisperModel(model string) error {
	model = strings.TrimSpace(model)
	for _, supported := range supportedWhisperModels {
		if model == supported {
			value := model
			whisperModelOverride.Store(&value)
			return nil
		}
	}
	return fmt.Errorf("unsupported Whisper model %q", model)
}

func resetAIRuntimeSettingsForTest() {
	ragEnabledOverride.Store(0)
	whisperModelOverride.Store(nil)
}

func (n *Router) handleRAGEnabled(w http.ResponseWriter, request *http.Request) {
	var payload struct {
		Enabled *bool `json:"enabled"`
	}
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil || payload.Enabled == nil {
		writeRuntimeSettingError(w, http.StatusBadRequest, "enabled is required")
		return
	}

	setRAGEnabled(*payload.Enabled)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"enabled": ragEnabled()})
}

func (n *Router) handleWhisperModel(w http.ResponseWriter, request *http.Request) {
	var payload struct {
		Model string `json:"model"`
	}
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeRuntimeSettingError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	if err := setWhisperModel(payload.Model); err != nil {
		writeRuntimeSettingError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"model": selectedWhisperModel()})
}

func writeRuntimeSettingError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}
