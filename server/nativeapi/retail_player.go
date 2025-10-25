package nativeapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/server"
)

const (
	retailPlayerDefaultBaseURL = "https://rpp.jareddietch.com/broad/api/v1"
	retailPlayerDefaultAPIKey  = "f3894t28-aghj-cv50-453e-9dfr1s9h73s5"
	retailPlayerDefaultOrgID   = "1aa59b04-5365-4efe-afb3-deb23c414add"
	retailPlayerMisconfigMsg   = "retail player proxy misconfigured: missing api key/org id"
)

const retailPlayerAPIKeyHeader = "x-retailplayer-apikey"

func (n *Router) addRetailPlayerRoutes(r chi.Router) {
	r.Route("/api/retailplayer", func(r chi.Router) {
		r.Get("/devices", n.retailPlayerProxyHandler(func(*http.Request) (string, error) {
			return "/devices", nil
		}, http.MethodGet, false))

		r.Route("/devices/{id}", func(r chi.Router) {
			r.Use(server.URLParamsMiddleware)

			r.Get("/", n.retailPlayerProxyHandler(func(r *http.Request) (string, error) {
				deviceID := chi.URLParam(r, "id")
				if deviceID == "" {
					return "", errors.New("missing device id")
				}
				return fmt.Sprintf("/devices/%s", url.PathEscape(deviceID)), nil
			}, http.MethodGet, false))

			r.Get("/status", n.retailPlayerProxyHandler(func(r *http.Request) (string, error) {
				deviceID := chi.URLParam(r, "id")
				if deviceID == "" {
					return "", errors.New("missing device id")
				}
				return fmt.Sprintf("/devices/%s/status", url.PathEscape(deviceID)), nil
			}, http.MethodGet, false))

			r.Post("/command", n.retailPlayerProxyHandler(func(r *http.Request) (string, error) {
				deviceID := chi.URLParam(r, "id")
				if deviceID == "" {
					return "", errors.New("missing device id")
				}
				return fmt.Sprintf("/devices/%s/command", url.PathEscape(deviceID)), nil
			}, http.MethodPost, true))
		})

		r.Get("/ping", n.retailPlayerProxyHandler(func(*http.Request) (string, error) {
			return "/hello", nil
		}, http.MethodGet, false))
	})
}

type retailPlayerTargetBuilder func(r *http.Request) (string, error)

func (n *Router) retailPlayerProxyHandler(buildTarget retailPlayerTargetBuilder, method string, forwardBody bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := loadRetailPlayerConfig()
		if cfg.apiKey == "" || cfg.orgID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": retailPlayerMisconfigMsg})
			return
		}

		targetSuffix, err := buildTarget(r)
		if err != nil {
			http.Error(w, "Invalid Retail Player request", http.StatusBadRequest)
			return
		}

		targetURL := cfg.urlFor(targetSuffix)
		log.Debug(r.Context(), "Retail Player proxy forwarding request", "method", method, "url", targetURL)

		var body io.Reader
		if forwardBody {
			body = r.Body
		}

		proxiedReq, err := http.NewRequestWithContext(r.Context(), method, targetURL, body)
		if err != nil {
			log.Error(r.Context(), "Unable to create Retail Player proxy request", "method", method, "url", targetURL, err)
			http.Error(w, "Retail Player request failed", http.StatusBadGateway)
			return
		}

		proxiedReq.Header.Set("Accept", "application/json")
		proxiedReq.Header.Set("Content-Type", "application/json")
		proxiedReq.Header.Set(retailPlayerAPIKeyHeader, cfg.apiKey)
		proxiedReq.URL.RawQuery = r.URL.RawQuery

		if forwardBody && r.ContentLength >= 0 {
			proxiedReq.ContentLength = r.ContentLength
		}

		resp, err := retailPlayerHTTPClient().Do(proxiedReq)
		if err != nil {
			log.Error(r.Context(), "Retail Player proxy request failed", "method", method, "url", targetURL, err)
			http.Error(w, "Retail Player request failed", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for key, values := range resp.Header {
			if strings.EqualFold(key, "Content-Length") {
				continue
			}
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		w.WriteHeader(resp.StatusCode)
		if resp.Body != nil {
			_, _ = io.Copy(w, resp.Body)
		}
	}
}

type retailPlayerConfig struct {
	baseURL string
	apiKey  string
	orgID   string
}

func loadRetailPlayerConfig() retailPlayerConfig {
	cfg := conf.Server.RetailPlayer

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = retailPlayerDefaultBaseURL
	}

	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		apiKey = retailPlayerDefaultAPIKey
	}

	orgID := strings.TrimSpace(cfg.OrgID)
	if orgID == "" {
		orgID = retailPlayerDefaultOrgID
	}

	return retailPlayerConfig{baseURL: baseURL, apiKey: apiKey, orgID: orgID}
}

func (c retailPlayerConfig) urlFor(path string) string {
	trimmedBase := strings.TrimRight(c.baseURL, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return fmt.Sprintf("%s/orgs/%s%s", trimmedBase, c.orgID, path)
}

var retailPlayerClient = &http.Client{Timeout: 10 * time.Second}

func retailPlayerHTTPClient() *http.Client {
	return retailPlayerClient
}
