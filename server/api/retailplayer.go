package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
)

const retailPlayerBaseURL = "https://rpp.jareddietch.com"

var retailPlayerClient = &http.Client{Timeout: 15 * time.Second}

type triggerPlayRequest struct {
	Device string `json:"device"`
	Cue    string `json:"cue"`
}

type triggerStopRequest struct {
	Device string `json:"device"`
}

type retailPlayerCommand struct {
	Action string `json:"action"`
	Value  string `json:"value,omitempty"`
}

func (s *Server) RetailPlayerGetTriggers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	device := r.URL.Query().Get("device")
	if device == "" {
		http.Error(w, "device is required", http.StatusBadRequest)
		return
	}

	url := fmt.Sprintf("%s/rest/v1/device-control/%s/triggers", retailPlayerBaseURL, device)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		http.Error(w, "unable to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("x-retailplayer-rc-apikey", conf.Server.RetailPlayerApiKey)

	resp, err := retailPlayerClient.Do(req)
	if err != nil {
		log.Error(ctx, "Unable to fetch retail player triggers", "err", err)
		http.Error(w, "unable to fetch triggers", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyRetailPlayerResponse(w, resp)
}

func (s *Server) RetailPlayerPlayCue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var payload triggerPlayRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if payload.Device == "" || payload.Cue == "" {
		http.Error(w, "device and cue are required", http.StatusBadRequest)
		return
	}

	command := retailPlayerCommand{Action: "PLAY", Value: payload.Cue}
	if err := s.sendRetailPlayerCommand(ctx, payload.Device, command, w); err != nil {
		log.Error(ctx, "Unable to play retail player cue", "err", err)
	}
}

func (s *Server) RetailPlayerStopCue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var payload triggerStopRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if payload.Device == "" {
		http.Error(w, "device is required", http.StatusBadRequest)
		return
	}

	command := retailPlayerCommand{Action: "STOP"}
	if err := s.sendRetailPlayerCommand(ctx, payload.Device, command, w); err != nil {
		log.Error(ctx, "Unable to stop retail player cue", "err", err)
	}
}

func (s *Server) sendRetailPlayerCommand(ctx context.Context, device string, command retailPlayerCommand, w http.ResponseWriter) error {
	body, err := json.Marshal(command)
	if err != nil {
		http.Error(w, "unable to marshal command", http.StatusInternalServerError)
		return err
	}

	url := fmt.Sprintf("%s/rest/v1/device-control/%s/triggers", retailPlayerBaseURL, device)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "unable to create request", http.StatusInternalServerError)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-retailplayer-rc-apikey", conf.Server.RetailPlayerApiKey)

	resp, err := retailPlayerClient.Do(req)
	if err != nil {
		http.Error(w, "unable to contact retail player", http.StatusBadGateway)
		return err
	}
	defer resp.Body.Close()

	copyRetailPlayerResponse(w, resp)
	return nil
}

func copyRetailPlayerResponse(w http.ResponseWriter, resp *http.Response) {
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
