package nativeapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAIChatProviderSpecNormalizesGemma26B(t *testing.T) {
	for _, provider := range []string{"gemma-26b", "gemma-26", "gemma-4", "gemma-3", "gemma3", "gemma3:4b"} {
		t.Run(provider, func(t *testing.T) {
			spec := aiChatProviderSpec(provider, "")
			if spec.ID != "gemma-26b" {
				t.Fatalf("expected gemma-26b ID, got %q", spec.ID)
			}
			if spec.Model != "gemma-26b" {
				t.Fatalf("expected gemma-26b model, got %q", spec.Model)
			}
		})
	}
}

func TestProbeAIEndpoint(t *testing.T) {
	t.Run("online", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodHead {
				t.Errorf("expected HEAD request, got %s", req.Method)
			}
			if req.Header.Get("Authorization") != "Bearer secret" {
				t.Errorf("expected bearer authorization header")
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		if !probeAIEndpoint(context.Background(), server.Client(), server.URL, "secret") {
			t.Fatal("expected endpoint to be online")
		}
	})

	t.Run("post only endpoint is online", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}))
		defer server.Close()

		if !probeAIEndpoint(context.Background(), server.Client(), server.URL, "") {
			t.Fatal("expected reachable POST-only endpoint to be online")
		}
	})

	t.Run("server error is offline", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		if probeAIEndpoint(context.Background(), server.Client(), server.URL, "") {
			t.Fatal("expected endpoint to be offline")
		}
	})

	t.Run("missing endpoint is offline", func(t *testing.T) {
		if probeAIEndpoint(context.Background(), http.DefaultClient, "", "") {
			t.Fatal("expected missing endpoint to be offline")
		}
	})
}
