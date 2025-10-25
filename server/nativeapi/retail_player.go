package nativeapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/server"
)

const retailPlayerAPIKeyHeader = "x-retailplayer-apikey"

func (n *Router) addRetailPlayerRoutes(r chi.Router) {
	r.Route("/retailplayer", func(r chi.Router) {
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
	})
}

type retailPlayerTargetBuilder func(r *http.Request) (string, error)

func (n *Router) retailPlayerProxyHandler(buildTarget retailPlayerTargetBuilder, method string, forwardBody bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := conf.Server.RetailPlayer
		if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.OrgID == "" {
			http.Error(w, "Retail Player proxy is not configured", http.StatusServiceUnavailable)
			return
		}

		targetSuffix, err := buildTarget(r)
		if err != nil {
			http.Error(w, "Invalid Retail Player request", http.StatusBadRequest)
			return
		}

		baseURL := strings.TrimRight(cfg.BaseURL, "/")
		targetURL := fmt.Sprintf("%s/orgs/%s%s", baseURL, cfg.OrgID, targetSuffix)

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
		proxiedReq.Header.Set(retailPlayerAPIKeyHeader, cfg.APIKey)
		proxiedReq.URL.RawQuery = r.URL.RawQuery

		if forwardBody && r.Header.Get("Content-Type") != "" {
			proxiedReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))
			proxiedReq.ContentLength = r.ContentLength
		}

		resp, err := n.httpClient.Do(proxiedReq)
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
