package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/log"
)

const (
	retailPlayerDefaultBaseURL = "https://rpp.jareddietch.com/broad/api/v1"
	retailPlayerDefaultAPIKey  = "f3894t28-aghj-cv50-453e-9dfr1s9h73s5"
	retailPlayerDefaultOrgID   = "1aa59b04-5365-4efe-afb3-deb23c414add"
)

// RetailPlayerProxy handles forwarding requests from the Navidrome server to the
// Retail Player Broad API, adding the required authentication headers and
// returning the upstream response to the client.
type RetailPlayerProxy struct {
	client  *http.Client
	baseURL string
	apiKey  string
	orgID   string
}

// NewRetailPlayerProxy creates a RetailPlayerProxy configured using environment
// variables with sensible defaults for development.
func NewRetailPlayerProxy() *RetailPlayerProxy {
	return &RetailPlayerProxy{
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: getEnvDefault("RETAILPLAYER_BASE_URL", retailPlayerDefaultBaseURL),
		apiKey:  getEnvDefault("RETAILPLAYER_API_KEY", retailPlayerDefaultAPIKey),
		orgID:   getEnvDefault("RETAILPLAYER_ORG_ID", retailPlayerDefaultOrgID),
	}
}

func getEnvDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Mount registers the Retail Player proxy endpoints in the provided router.
func (p *RetailPlayerProxy) Mount(r chi.Router) {
	r.Route("/retailplayer", func(r chi.Router) {
		r.Get("/devices", p.handleDevices)
		r.Get("/devices/{id}/status", p.handleDeviceStatus)
		r.Post("/devices/{id}/command", p.handleDeviceCommand)
	})
}

func (p *RetailPlayerProxy) handleDevices(w http.ResponseWriter, r *http.Request) {
	endpoint := fmt.Sprintf("/orgs/%s/devices", p.orgID)
	p.forward(w, r, http.MethodGet, endpoint, nil)
}

func (p *RetailPlayerProxy) handleDeviceStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	endpoint := fmt.Sprintf("/orgs/%s/devices/%s/status", p.orgID, id)
	p.forward(w, r, http.MethodGet, endpoint, nil)
}

func (p *RetailPlayerProxy) handleDeviceCommand(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	endpoint := fmt.Sprintf("/orgs/%s/devices/%s/command", p.orgID, id)
	defer func() { _ = r.Body.Close() }()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeRetailPlayerError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	p.forward(w, r, http.MethodPost, endpoint, body)
}

func (p *RetailPlayerProxy) forward(w http.ResponseWriter, r *http.Request, method, endpoint string, body []byte) {
	url := p.baseURL + endpoint

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(r.Context(), method, url, bodyReader)
	if err != nil {
		writeRetailPlayerError(w, http.StatusInternalServerError, "failed to build upstream request")
		return
	}

	req.Header.Set("x-retailplayer-apikey", p.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			log.Warn(r.Context(), "Retail Player proxy request cancelled or timed out", "endpoint", endpoint, err)
			writeRetailPlayerError(w, http.StatusGatewayTimeout, "upstream request timeout")
			return
		}
		log.Error(r.Context(), "Retail Player proxy request failed", "endpoint", endpoint, err)
		writeRetailPlayerError(w, http.StatusBadGateway, "upstream request failed")
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error(r.Context(), "Retail Player proxy failed to read upstream response", "endpoint", endpoint, err)
		writeRetailPlayerError(w, http.StatusBadGateway, "failed to read upstream response")
		return
	}

	for key, values := range resp.Header {
		if key == "Content-Length" {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}

	w.WriteHeader(resp.StatusCode)
	if _, err := w.Write(responseBody); err != nil {
		log.Warn(r.Context(), "Retail Player proxy failed to write response", "endpoint", endpoint, err)
	}
}

func writeRetailPlayerError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
