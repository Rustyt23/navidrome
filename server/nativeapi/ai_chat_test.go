package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
)

type staticAIChatProvider struct {
	answer string
}

func (p staticAIChatProvider) Chat(context.Context, string) (string, error) {
	return p.answer, nil
}

func TestRAGStatus(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()

	requestStatus := func(t *testing.T) aiRAGStatusResponse {
		t.Helper()
		router := chi.NewRouter()
		(&Router{}).addAIChatRoute(router)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/ai/rag/status", nil)

		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
		}
		if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
			t.Fatalf("expected JSON content type, got %q", contentType)
		}

		var response aiRAGStatusResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return response
	}

	t.Run("disabled does not contact Qdrant", func(t *testing.T) {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			requests++
		}))
		defer server.Close()

		conf.Server.EnableRAG = false
		conf.Server.RAGVectorURL = server.URL
		conf.Server.RAGCollection = "test_songs"
		conf.Server.RAGTopK = 12

		response := requestStatus(t)
		if response.Enabled || response.VectorDBOnline || response.CollectionExists || response.IndexedCount != 0 {
			t.Fatalf("unexpected disabled response: %+v", response)
		}
		if response.Error != "" {
			t.Fatalf("expected no disabled error, got %q", response.Error)
		}
		if requests != 0 {
			t.Fatalf("expected no Qdrant requests while disabled, got %d", requests)
		}
	})

	t.Run("reports online collection and point count", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/collections":
				_, _ = w.Write([]byte(`{"status":"ok","result":{"collections":[]}}`))
			case "/collections/test_songs":
				_, _ = w.Write([]byte(`{"status":"ok","result":{"points_count":7}}`))
			default:
				http.NotFound(w, request)
			}
		}))
		defer server.Close()

		conf.Server.EnableRAG = true
		conf.Server.RAGVectorURL = server.URL
		conf.Server.RAGCollection = "test_songs"
		conf.Server.RAGTopK = 12

		response := requestStatus(t)
		if !response.Enabled || !response.VectorDBOnline || !response.CollectionExists {
			t.Fatalf("unexpected online response: %+v", response)
		}
		if response.IndexedCount != 7 || response.Error != "" {
			t.Fatalf("unexpected online count/error: %+v", response)
		}
	})

	t.Run("returns JSON while Qdrant is offline", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		serverURL := server.URL
		server.Close()

		conf.Server.EnableRAG = true
		conf.Server.RAGVectorURL = serverURL
		conf.Server.RAGCollection = "test_songs"
		conf.Server.RAGTopK = 12

		response := requestStatus(t)
		if !response.Enabled || response.VectorDBOnline || response.CollectionExists {
			t.Fatalf("unexpected offline response: %+v", response)
		}
		if response.Error == "" {
			t.Fatal("expected an offline error message")
		}
	})
}

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

func TestOllamaGemmaClientChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", req.Method)
		}
		if req.Header.Get("Authorization") != "" {
			t.Errorf("did not expect an authorization header")
		}
		var payload ollamaGenerateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Model != "gemma3:4b" || payload.Prompt != "Say hello" || payload.Stream {
			t.Fatalf("unexpected Ollama payload: %+v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gemma3:4b","response":"Hello from 4b!","done":true}`))
	}))
	defer server.Close()

	client := ollamaGemmaClient{
		apiURL: server.URL,
		model:  "gemma3:4b",
		client: server.Client(),
	}
	answer, err := client.Chat(context.Background(), "Say hello")
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}
	if answer != "Hello from 4b!" {
		t.Fatalf("unexpected answer: %q", answer)
	}
}

func TestWriteAIChatErrorReturnsMessageJSON(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeAIChatError(recorder, http.StatusBadGateway, "Gemma connection timed out")

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", recorder.Code)
	}
	var response map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["message"] != "Gemma connection timed out" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestAIChatRAGMode(t *testing.T) {
	if !shouldUseRAG(aiChatRequest{}) {
		t.Fatal("expected backward-compatible RAG mode by default")
	}
	useRAG := true
	if !shouldUseRAG(aiChatRequest{UseRAG: &useRAG}) {
		t.Fatal("expected explicit RAG mode")
	}
	useRAG = false
	if shouldUseRAG(aiChatRequest{UseRAG: &useRAG}) {
		t.Fatal("expected normal chat to bypass RAG")
	}
}

func TestAIChatProviderSpecNormalizesGemma26B(t *testing.T) {
	for _, provider := range []string{"gemma-26b", "gemma-26", "gemma-4"} {
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

func TestAIChatProviderSpecNormalizesGemma3FourB(t *testing.T) {
	for _, provider := range []string{"gemma-3-4b", "gemma-3:4b", "gemma-3", "gemma3", "gemma3:4b"} {
		t.Run(provider, func(t *testing.T) {
			spec := aiChatProviderSpec(provider, "")
			if spec.ID != "gemma-3-4b" || spec.Model != "gemma3:4b" {
				t.Fatalf("unexpected Gemma 3:4b spec: %+v", spec)
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

func TestFetchWhisperLyricsSendsSelectedModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart form: %v", err)
		}
		if request.FormValue("model") != "small" {
			t.Fatalf("expected small model, got %q", request.FormValue("model"))
		}
		file, _, err := request.FormFile("file")
		if err != nil {
			t.Fatalf("expected audio file: %v", err)
		}
		_ = file.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"language":"eng","text":"Test lyrics"}`))
	}))
	defer server.Close()

	audioPath := filepath.Join(t.TempDir(), "song.mp3")
	if err := os.WriteFile(audioPath, []byte("audio"), 0o600); err != nil {
		t.Fatalf("write audio: %v", err)
	}
	lyrics, err := fetchWhisperLyrics(context.Background(), server.URL, audioPath, "small")
	if err != nil {
		t.Fatalf("fetch lyrics: %v", err)
	}
	if lyrics.Language != "eng" || lyrics.Text != "Test lyrics" {
		t.Fatalf("unexpected lyrics: %+v", lyrics)
	}
}

func TestSaveWhisperLyricsFile(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "lyrics")
	if err := saveWhisperLyricsFile(folder, "song/id", "Line one\nLine two"); err != nil {
		t.Fatalf("save lyrics: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(folder, "song_id.txt"))
	if err != nil {
		t.Fatalf("read lyrics: %v", err)
	}
	if string(content) != "Line one\nLine two\n" {
		t.Fatalf("unexpected lyrics file: %q", content)
	}
}

func TestDeleteSongLyricsClearsRepositoryAndSavedFile(t *testing.T) {
	folder := t.TempDir()
	repo := tests.CreateMockMediaFileRepo()
	repo.SetData(model.MediaFiles{{ID: "song-1", Lyrics: `[{"lang":"eng","line":[{"value":"lyrics"}]}]`}})
	if err := saveWhisperLyricsFile(folder, "song-1", "lyrics"); err != nil {
		t.Fatalf("save lyrics: %v", err)
	}
	if err := deleteSongLyrics(repo, folder, "song-1"); err != nil {
		t.Fatalf("delete lyrics: %v", err)
	}
	updated, _ := repo.Get("song-1")
	if updated.Lyrics != "" {
		t.Fatalf("expected repository lyrics to be empty, got %q", updated.Lyrics)
	}
	path, _ := whisperLyricsFilePath(folder, "song-1")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected lyrics file to be removed, err=%v", err)
	}
}

func TestNormalizeExplicitWordRules(t *testing.T) {
	rules := normalizeExplicitWordRules(
		[]string{" Strong Word ", "strong word", ""},
		[]string{" Mild Word ", "mild word"},
	)
	if len(rules.Included) != 1 || rules.Included[0] != "strong word" ||
		len(rules.Excluded) != 1 || rules.Excluded[0] != "mild word" {
		t.Fatalf("unexpected normalized rules: %+v", rules)
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

func TestParseExplicitClassificationRequiresConfidenceAndEvidence(t *testing.T) {
	lyrics := "We dance all night\nThis contains an uncensored explicit phrase\nThen we go home"

	tests := []struct {
		name   string
		answer string
		want   string
	}{
		{
			name:   "high confidence explicit with quoted evidence",
			answer: `{"classification":"explicit","confidence":95,"evidence":["This contains an uncensored explicit phrase"]}`,
			want:   "explicit",
		},
		{
			name:   "explicit without evidence abstains",
			answer: `{"classification":"explicit","confidence":99,"evidence":[]}`,
		},
		{
			name:   "hallucinated evidence abstains",
			answer: `{"classification":"explicit","confidence":99,"evidence":["words not present in the lyrics"]}`,
		},
		{
			name:   "low confidence explicit abstains",
			answer: `{"classification":"explicit","confidence":89,"evidence":["This contains an uncensored explicit phrase"]}`,
		},
		{
			name:   "high confidence clean",
			answer: `{"classification":"clean","confidence":92,"evidence":[]}`,
			want:   "clean",
		},
		{
			name:   "low confidence clean abstains",
			answer: `{"classification":"clean","confidence":70,"evidence":[]}`,
		},
		{
			name:   "single word clean compatibility",
			answer: "clean",
			want:   "clean",
		},
		{
			name:   "single word explicit is not trusted without evidence",
			answer: "explicit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseExplicitClassification(tt.answer, lyrics); got != tt.want {
				t.Fatalf("parseExplicitClassification() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExplicitStatusCodeDoesNotScanExplanatoryText(t *testing.T) {
	if got := explicitStatusCode("not explicit; this song is clean"); got != "" {
		t.Fatalf("expected ambiguous explanatory text to be ignored, got %q", got)
	}
	if got := explicitStatusCode("clean"); got != "c" {
		t.Fatalf("expected clean status, got %q", got)
	}
	if got := explicitStatusCode("explicit"); got != "e" {
		t.Fatalf("expected explicit status, got %q", got)
	}
}

func TestClassifyExplicitCorrectsExistingStatusOnlyWithValidatedVerdict(t *testing.T) {
	lyrics := `[{"lang":"eng","line":[{"value":"We dance together all night"},{"value":"Then watch the morning light"}]}]`

	t.Run("corrects an earlier explicit false positive", func(t *testing.T) {
		repo := tests.CreateMockMediaFileRepo()
		repo.SetData(model.MediaFiles{{ID: "song-1", Lyrics: lyrics, ExplicitStatus: "e"}})
		results, err := classifyExplicit(context.Background(), repo, staticAIChatProvider{
			answer: `{"classification":"clean","confidence":96,"evidence":[]}`,
		}, []string{"song-1"})
		if err != nil {
			t.Fatalf("classify explicit: %v", err)
		}
		updated, _ := repo.Get("song-1")
		if len(results) != 1 || results[0].ExplicitStatus != "c" || updated.ExplicitStatus != "c" {
			t.Fatalf("expected corrected clean status, results=%+v stored=%q", results, updated.ExplicitStatus)
		}
	})

	t.Run("preserves existing status when the new verdict is uncertain", func(t *testing.T) {
		repo := tests.CreateMockMediaFileRepo()
		repo.SetData(model.MediaFiles{{ID: "song-1", Lyrics: lyrics, ExplicitStatus: "e"}})
		results, err := classifyExplicit(context.Background(), repo, staticAIChatProvider{
			answer: `{"classification":"clean","confidence":50,"evidence":[]}`,
		}, []string{"song-1"})
		if err != nil {
			t.Fatalf("classify explicit: %v", err)
		}
		updated, _ := repo.Get("song-1")
		if len(results) != 1 || results[0].ExplicitStatus != "e" || updated.ExplicitStatus != "e" {
			t.Fatalf("expected existing status preserved, results=%+v stored=%q", results, updated.ExplicitStatus)
		}
	})
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
