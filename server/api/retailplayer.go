package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
)

const retailPlayerBaseURL = "https://rpp.jareddietch.com"

// AddRetailPlayerRoutes registers proxy endpoints used by the UI to interact with RetailPlayer.
func AddRetailPlayerRoutes(r chi.Router) {
	handler := retailPlayerHandler{client: &http.Client{Timeout: 10 * time.Second}}
	r.Get("/retailplayer/triggers", handler.getTriggers)
	r.Post("/retailplayer/play", handler.playCue)
	r.Post("/retailplayer/stop", handler.stopCue)
}

type retailPlayerHandler struct {
	client *http.Client
}

type playCueRequest struct {
	Device string `json:"device"`
	Cue    string `json:"cue"`
}

type stopCueRequest struct {
	Device string `json:"device"`
}

func (h retailPlayerHandler) getTriggers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	deviceID := r.URL.Query().Get("device")
	if deviceID == "" {
		http.Error(w, "device is required", http.StatusBadRequest)
		return
	}

	upstreamURL := fmt.Sprintf("%s/rest/v1/device-control/%s/triggers", retailPlayerBaseURL, deviceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstreamURL, nil)
	if err != nil {
		log.Error(ctx, "creating retailplayer triggers request", err)
		http.Error(w, "unable to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("x-retailplayer-rc-apikey", conf.Server.RetailPlayerApiKey)

	h.forward(w, req)
}

func (h retailPlayerHandler) playCue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var body playCueRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Device == "" || body.Cue == "" {
		http.Error(w, "device and cue are required", http.StatusBadRequest)
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"action": "PLAY",
		"value":  body.Cue,
	})
	upstreamURL := fmt.Sprintf("%s/rest/v1/device-control/%s/triggers", retailPlayerBaseURL, body.Device)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(payload))
	if err != nil {
		log.Error(ctx, "creating retailplayer play request", err)
		http.Error(w, "unable to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("x-retailplayer-rc-apikey", conf.Server.RetailPlayerApiKey)
	req.Header.Set("Content-Type", "application/json")

	h.forward(w, req)
}

func (h retailPlayerHandler) stopCue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var body stopCueRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Device == "" {
		http.Error(w, "device is required", http.StatusBadRequest)
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"action": "STOP",
	})
	upstreamURL := fmt.Sprintf("%s/rest/v1/device-control/%s/triggers", retailPlayerBaseURL, body.Device)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(payload))
	if err != nil {
		log.Error(ctx, "creating retailplayer stop request", err)
		http.Error(w, "unable to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("x-retailplayer-rc-apikey", conf.Server.RetailPlayerApiKey)
	req.Header.Set("Content-Type", "application/json")

	h.forward(w, req)
}

func (h retailPlayerHandler) forward(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	resp, err := h.client.Do(req)
	if err != nil {
		log.Error(ctx, "forwarding retailplayer request", err)
		http.Error(w, "retailplayer request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
