package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
)

func TestGemmaClientChat(t *testing.T) {
	t.Run("reads response field returned by Gemma service", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodPost {
				t.Errorf("expected POST request, got %s", req.Method)
			}
			if req.Header.Get("Authorization") != "Bearer secret" {
				t.Errorf("expected bearer authorization header")
			}

			var payload gemmaChatRequest
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if payload.Message != "Say hello" {
				t.Errorf("expected message %q, got %q", "Say hello", payload.Message)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"gemma4:26b","response":"Hello!"}`))
		}))
		defer server.Close()

		client := gemmaClient{apiURL: server.URL, apiKey: "secret", client: server.Client()}
		answer, err := client.Chat(context.Background(), "Say hello")
		if err != nil {
			t.Fatalf("chat failed: %v", err)
		}
		if answer != "Hello!" {
			t.Fatalf("expected %q, got %q", "Hello!", answer)
		}
	})

	t.Run("supports legacy reply field", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"gemma-26b","reply":"Legacy hello"}`))
		}))
		defer server.Close()

		client := gemmaClient{apiURL: server.URL, apiKey: "secret", client: server.Client()}
		answer, err := client.Chat(context.Background(), "Say hello")
		if err != nil {
			t.Fatalf("chat failed: %v", err)
		}
		if answer != "Legacy hello" {
			t.Fatalf("expected %q, got %q", "Legacy hello", answer)
		}
	})
}

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

func TestProbeGemmaEndpointUsesOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodOptions {
			t.Errorf("expected OPTIONS request, got %s", req.Method)
		}
		if req.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("expected bearer authorization header")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if !probeGemmaEndpoint(context.Background(), server.Client(), server.URL, "secret") {
		t.Fatal("expected Gemma endpoint to be online")
	}
}

func TestParseGeminiSongMetadataConfidence(t *testing.T) {
	metadata, err := parseGeminiSongMetadata(`{
		"album":"Discovery",
		"albumConfidence":92,
		"year":2001,
		"yearConfidence":"84%",
		"genre":"French house",
		"genreConfidence":150
	}`)
	if err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if metadata.AlbumConfidence != 92 {
		t.Fatalf("expected album confidence 92, got %d", metadata.AlbumConfidence)
	}
	if metadata.YearConfidence != 84 {
		t.Fatalf("expected year confidence 84, got %d", metadata.YearConfidence)
	}
	if metadata.GenreConfidence != 100 {
		t.Fatalf("expected clamped genre confidence 100, got %d", metadata.GenreConfidence)
	}
}

func TestParseGeminiSongMetadataLegacyConfidence(t *testing.T) {
	metadata, err := parseGeminiSongMetadata(
		`{"album":"Discovery","year":2001,"genre":"House","confidence":76}`,
	)
	if err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if metadata.AlbumConfidence != 76 || metadata.YearConfidence != 76 || metadata.GenreConfidence != 76 {
		t.Fatalf("expected legacy confidence for all fields, got %+v", metadata)
	}
}

func TestClearAIMetadata(t *testing.T) {
	repo := tests.CreateMockMediaFileRepo()
	repo.SetData(model.MediaFiles{
		{ID: "song-1", Album: "AI Album", Year: 2024},
		{ID: "song-2", Album: "Original Album", Year: 1999},
	})

	cleared, err := clearAIMetadata(repo, []aiClearMetadataSong{
		{ID: "song-1", Album: true, Year: true},
		{ID: "song-2"},
	})
	if err != nil {
		t.Fatalf("clear metadata: %v", err)
	}
	if len(cleared) != 2 {
		t.Fatalf("expected 2 cleared songs, got %d", len(cleared))
	}

	first, _ := repo.Get("song-1")
	if first.Album != "[Unknown Album]" || first.Year != 0 {
		t.Fatalf("expected AI fields cleared, got album=%q year=%d", first.Album, first.Year)
	}
	second, _ := repo.Get("song-2")
	if second.Album != "Original Album" || second.Year != 1999 {
		t.Fatalf("expected original fields preserved, got album=%q year=%d", second.Album, second.Year)
	}
}
