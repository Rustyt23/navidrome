package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
				_, _ = w.Write([]byte(`{"status":"ok","result":{"points_count":8,"config":{"params":{"vectors":{"size":768}}}}}`))
			default:
				if strings.HasPrefix(request.URL.Path, "/collections/test_songs/points/") {
					_, _ = w.Write([]byte(`{"status":"ok","result":{"payload":{"indexVersion":2,"embeddingModel":"gemini:gemini-embedding-001","dimensions":768}}}`))
				} else {
					http.NotFound(w, request)
				}
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
		if request.FormValue("response_format") != "verbose_json" {
			t.Fatalf("expected verbose_json response format, got %q", request.FormValue("response_format"))
		}
		file, _, err := request.FormFile("file")
		if err != nil {
			t.Fatalf("expected audio file: %v", err)
		}
		_ = file.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"language":"eng","text":"Test lyrics","duration":100,"segments":[{"start":0,"end":90}]}`))
	}))
	defer server.Close()

	audioPath := filepath.Join(t.TempDir(), "song.mp3")
	if err := os.WriteFile(audioPath, []byte("audio"), 0o600); err != nil {
		t.Fatalf("write audio: %v", err)
	}
	result, err := fetchWhisperLyrics(context.Background(), server.URL, audioPath, "small", 0)
	if err != nil {
		t.Fatalf("fetch lyrics: %v", err)
	}
	if result.Language != "eng" || result.Text != "Test lyrics" {
		t.Fatalf("unexpected lyrics: %+v", result)
	}
	if result.Duration != 100 || result.LastSegmentEnd != 90 || result.SegmentCount != 1 {
		t.Fatalf("unexpected timing metadata: %+v", result)
	}
}

func TestFetchWhisperLyricsFallsBackWhenTuningRejected(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart form: %v", err)
		}
		attempts++
		// The first attempt includes the faster-whisper tuning fields; reject it
		// the way a strict OpenAI-compatible server would.
		if request.FormValue("vad_filter") != "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"unknown field vad_filter"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"language":"eng","text":"Fallback lyrics","duration":60,"segments":[{"start":0,"end":58}]}`))
	}))
	defer server.Close()

	audioPath := filepath.Join(t.TempDir(), "song.mp3")
	if err := os.WriteFile(audioPath, []byte("audio"), 0o600); err != nil {
		t.Fatalf("write audio: %v", err)
	}
	result, err := fetchWhisperLyrics(context.Background(), server.URL, audioPath, "small", 0)
	if err != nil {
		t.Fatalf("fetch lyrics: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected a retry without tuning fields, got %d attempts", attempts)
	}
	if result.Text != "Fallback lyrics" || result.LastSegmentEnd != 58 {
		t.Fatalf("unexpected fallback result: %+v", result)
	}
}

func TestWhisperCoverageDetectsTruncation(t *testing.T) {
	// Transcription stopped less than 75% through with plenty of song left.
	_, _, _, truncated := whisperCoverage(
		whisperResult{Duration: 200, LastSegmentEnd: 100, SegmentCount: 5}, 200,
	)
	if !truncated {
		t.Fatalf("expected truncation to be detected")
	}

	// Near-full coverage is not flagged.
	coverage, _, _, truncated := whisperCoverage(
		whisperResult{Duration: 200, LastSegmentEnd: 190, SegmentCount: 20}, 200,
	)
	if truncated {
		t.Fatalf("expected full coverage not to be flagged, coverage=%v", coverage)
	}

	// Falls back to the song's stored duration when Whisper omits it, and does
	// not flag when there is no segment timing to judge by.
	_, _, total, truncated := whisperCoverage(
		whisperResult{LastSegmentEnd: 0, SegmentCount: 0}, 180,
	)
	if total != 180 || truncated {
		t.Fatalf("expected fallback duration 180 and no truncation, total=%v truncated=%v", total, truncated)
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

func TestParseGeminiSongMetadataMatchedIdentity(t *testing.T) {
	metadata, err := parseGeminiSongMetadata(
		`{"matchedTitle":"One More Time","matchedArtist":"Daft Punk","album":"Discovery","year":2001,"genre":"French house","albumConfidence":95,"yearConfidence":90,"genreConfidence":80}`,
	)
	if err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if metadata.MatchedTitle != "One More Time" || metadata.MatchedArtist != "Daft Punk" {
		t.Fatalf("unexpected matched identity: %+v", metadata)
	}
}

func TestResolveMetadataField(t *testing.T) {
	// Missing value, Spotify only -> chosen from Spotify at Spotify confidence.
	if fill, val, conf, expl := resolveMetadataField("", false, "Discovery", "", "", metadataValuesMatch); fill != "Discovery" || val != "Discovery" || conf != metadataConfidenceSpotify || expl.Source != "spotify" {
		t.Fatalf("spotify-only: got fill=%q val=%q conf=%d src=%q", fill, val, conf, expl.Source)
	}
	// Two sources agree -> verified.
	if _, _, conf, expl := resolveMetadataField("", false, "Discovery", "discovery", "", metadataValuesMatch); conf != metadataConfidenceVerified || expl.Source != "verified" {
		t.Fatalf("agreement: got conf=%d src=%q", conf, expl.Source)
	}
	// MusicBrainz only -> authoritative.
	if _, _, conf, expl := resolveMetadataField("", false, "", "Discovery", "", metadataValuesMatch); conf != metadataConfidenceAuthoritative || expl.Source != "musicbrainz" {
		t.Fatalf("mb-only: got conf=%d src=%q", conf, expl.Source)
	}
	// AI only -> low, honest.
	if _, _, conf, expl := resolveMetadataField("", false, "", "", "Guess", metadataValuesMatch); conf != metadataConfidenceAIOnly || expl.Source != "ai-only" {
		t.Fatalf("ai-only: got conf=%d src=%q", conf, expl.Source)
	}
	// Existing value that a source confirms -> verified, not overwritten.
	if fill, val, conf, expl := resolveMetadataField("Discovery", true, "Discovery", "", "", metadataValuesMatch); fill != "" || val != "Discovery" || conf != metadataConfidenceVerified || expl.Source != "verified" {
		t.Fatalf("existing-verified: got fill=%q val=%q conf=%d src=%q", fill, val, conf, expl.Source)
	}
	// Existing value that disagrees with a source -> conflict.
	if _, _, conf, expl := resolveMetadataField("Wrong Album", true, "Discovery", "", "", metadataValuesMatch); conf != metadataConfidenceConflict || expl.Source != "conflict" {
		t.Fatalf("existing-conflict: got conf=%d src=%q", conf, expl.Source)
	}
	// Nothing available -> none.
	if _, _, conf, expl := resolveMetadataField("", false, "", "", "", metadataValuesMatch); conf != 0 || expl.Source != "none" {
		t.Fatalf("none: got conf=%d src=%q", conf, expl.Source)
	}
}

func TestResolveGenreConsensus(t *testing.T) {
	// Two sources share a genre -> consensus, verified.
	if value, conf, expl := resolveGenreConsensus("Indie Pop", "indie pop", "Rock"); value != "Indie Pop" || conf != metadataConfidenceVerified || expl.Source != "verified" {
		t.Fatalf("consensus: got value=%q conf=%d src=%q", value, conf, expl.Source)
	}
	// All three disagree -> fall back to MusicBrainz (Spotify trusted last).
	if value, conf, expl := resolveGenreConsensus("Dream Pop", "Shoegaze", "Rock"); value != "Shoegaze" || conf != metadataConfidenceAuthoritative || expl.Source != "musicbrainz" {
		t.Fatalf("no-consensus: got value=%q conf=%d src=%q", value, conf, expl.Source)
	}
	// Only Spotify has a genre -> used, but at Spotify's lower confidence.
	if value, conf, expl := resolveGenreConsensus("Indie Pop", "", ""); value != "Indie Pop" || conf != metadataConfidenceSpotify || expl.Source != "spotify" {
		t.Fatalf("spotify-only: got value=%q conf=%d src=%q", value, conf, expl.Source)
	}
	// Nothing available -> none.
	if value, conf, expl := resolveGenreConsensus("", "", ""); value != "" || conf != 0 || expl.Source != "none" {
		t.Fatalf("none: got value=%q conf=%d src=%q", value, conf, expl.Source)
	}
}

func TestMetadataValuesMatch(t *testing.T) {
	if !metadataValuesMatch("Discovery", "discovery") {
		t.Fatal("expected case-insensitive match")
	}
	if !metadataValuesMatch("Discovery", "Discovery (Deluxe Edition)") {
		t.Fatal("expected subset match")
	}
	if metadataValuesMatch("Discovery", "Random Access Memories") {
		t.Fatal("expected different albums not to match")
	}
}

func TestFetchSongMetadataUnverifiedAIScoresLow(t *testing.T) {
	repo := tests.CreateMockMediaFileRepo()
	repo.SetData(model.MediaFiles{{ID: "song-1", Title: "Some Song", Artist: "Some Artist", Album: "[Unknown Album]"}})
	// No Spotify, no MusicBrainz: the AI's guess must score low even though the
	// model claims 100.
	songs, err := fetchSongMetadata(context.Background(), repo, staticAIChatProvider{
		answer: `{"album":"Guessed Album","albumConfidence":100,"year":2010,"yearConfidence":100,"genre":"Pop","genreConfidence":100}`,
	}, nil, nil, []string{"song-1"})
	if err != nil {
		t.Fatalf("fetch metadata: %v", err)
	}
	if len(songs) != 1 {
		t.Fatalf("expected 1 result, got %d", len(songs))
	}
	if songs[0].Album != "Guessed Album" {
		t.Fatalf("expected album filled from AI, got %q", songs[0].Album)
	}
	if songs[0].AlbumConfidence != metadataConfidenceAIOnly {
		t.Fatalf("expected unverified AI album to score %d, got %d", metadataConfidenceAIOnly, songs[0].AlbumConfidence)
	}
}

func TestFetchSongMetadataPrefersSpotifyAndVerifies(t *testing.T) {
	repo := tests.CreateMockMediaFileRepo()
	repo.SetData(model.MediaFiles{{ID: "song-1", Title: "One More Time", Artist: "Daft Punk", Album: "[Unknown Album]"}})
	spotify := func(_ context.Context, _ model.MediaFile) (spotifyLookupResult, error) {
		return spotifyLookupResult{Album: "Discovery", Year: 2001, Genre: "French House", Confidence: 0.95, Found: true}, nil
	}
	verify := func(_ context.Context, _, _ string) (metadataResult, error) {
		return metadataResult{Album: "Discovery", Year: 2001, Genre: "French house"}, nil
	}
	// The AI hallucinates a wrong album/genre, but Spotify + MusicBrainz agree, so
	// the AI's values are overridden for album/year and outvoted for genre.
	songs, err := fetchSongMetadata(context.Background(), repo, staticAIChatProvider{
		answer: `{"album":"Greatest Hits","year":1999,"genre":"Pop"}`,
	}, verify, spotify, []string{"song-1"})
	if err != nil {
		t.Fatalf("fetch metadata: %v", err)
	}
	if songs[0].Album != "Discovery" {
		t.Fatalf("expected Spotify album Discovery, got %q", songs[0].Album)
	}
	if songs[0].AlbumConfidence != metadataConfidenceVerified || songs[0].YearConfidence != metadataConfidenceVerified {
		t.Fatalf("expected Spotify+MusicBrainz agreement to be verified, got album=%d year=%d", songs[0].AlbumConfidence, songs[0].YearConfidence)
	}
	// Each source's genre is reported separately; Spotify and MusicBrainz agree,
	// so the genre confidence is verified even though the AI guessed "Pop".
	if songs[0].SpotifyGenre != "French House" || songs[0].MusicBrainzGenre != "French House" || songs[0].AIGenre != "Pop" {
		t.Fatalf("unexpected per-source genres: spotify=%q mb=%q ai=%q", songs[0].SpotifyGenre, songs[0].MusicBrainzGenre, songs[0].AIGenre)
	}
	if songs[0].GenreConfidence != metadataConfidenceVerified {
		t.Fatalf("expected verified genre confidence from Spotify+MusicBrainz agreement, got %d", songs[0].GenreConfidence)
	}
	updated, _ := repo.Get("song-1")
	if updated.Album != "Discovery" || updated.Year != 2001 {
		t.Fatalf("expected DB updated with Spotify values, got album=%q year=%d", updated.Album, updated.Year)
	}
}

func TestFetchSongMetadataSpotifyOnlyScore(t *testing.T) {
	repo := tests.CreateMockMediaFileRepo()
	repo.SetData(model.MediaFiles{{ID: "song-1", Title: "One More Time", Artist: "Daft Punk", Album: "[Unknown Album]"}})
	spotify := func(_ context.Context, _ model.MediaFile) (spotifyLookupResult, error) {
		return spotifyLookupResult{Album: "Discovery", Year: 2001, Genre: "House", Confidence: 0.9, Found: true}, nil
	}
	// No AI provider and no MusicBrainz: Spotify alone fills album/year at the
	// Spotify confidence level.
	songs, err := fetchSongMetadata(context.Background(), repo, nil, nil, spotify, []string{"song-1"})
	if err != nil {
		t.Fatalf("fetch metadata: %v", err)
	}
	if songs[0].Album != "Discovery" || songs[0].AlbumConfidence != metadataConfidenceSpotify {
		t.Fatalf("expected Spotify album at score %d, got %q %d", metadataConfidenceSpotify, songs[0].Album, songs[0].AlbumConfidence)
	}
	if songs[0].Year != 2001 || songs[0].YearConfidence != metadataConfidenceSpotify {
		t.Fatalf("expected Spotify year at score %d, got %d %d", metadataConfidenceSpotify, songs[0].Year, songs[0].YearConfidence)
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

func TestExplicitMatcherDetectsUncensoredAndObfuscated(t *testing.T) {
	matcher := newExplicitMatcher(defaultExplicitWordRules)

	cases := []struct {
		name   string
		lyrics string
		want   bool
	}{
		{"uncensored", "this is some fucking noise", true},
		{"masked vowels", "what the f**k is this", true},
		{"leetspeak", "you little sh1t", true},
		{"bang mask", "she is a b!tch to me", true},
		{"slur masked", "n*gga please", true},
		{"clean romance", "i love the way you kiss me tonight", false},
		{"substring safe", "the assassin walked past the dock", false},
		{"innocuous near-miss", "we drove the truck and had some funk", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := len(matcher.matches(tc.lyrics)) > 0
			if got != tc.want {
				t.Fatalf("matches(%q) = %v, want %v (hits=%v)", tc.lyrics, got, tc.want, matcher.matches(tc.lyrics))
			}
		})
	}
}

func TestApplyDeterministicExplicitEvidenceOverridesFalseClean(t *testing.T) {
	// The model confidently returns clean, but the lyrics contain masked profanity.
	result := parseExplicitClassificationDetailed(
		`{"classification":"clean","confidence":97,"evidence":[]}`,
		"what the f**k is going on",
	)
	if result.Classification != "clean" {
		t.Fatalf("precondition failed, expected model clean, got %q", result.Classification)
	}
	merged := applyDeterministicExplicitEvidence(result, "what the f**k is going on", defaultExplicitWordRules)
	if merged.Classification != "explicit" {
		t.Fatalf("expected deterministic override to explicit, got %q", merged.Classification)
	}
	if len(merged.Evidence) == 0 {
		t.Fatalf("expected explicit evidence to be attached")
	}
	if merged.Confidence < minimumExplicitConfidence {
		t.Fatalf("expected confidence raised to at least %d, got %d", minimumExplicitConfidence, merged.Confidence)
	}
}

func TestExplicitTitleMarkerFallbackWhenLyricsMissing(t *testing.T) {
	repo := tests.CreateMockMediaFileRepo()
	repo.SetData(model.MediaFiles{
		{ID: "song-1", Title: "Bad Song [Explicit]", ExplicitStatus: ""},
	})
	results, err := classifyExplicit(context.Background(), repo, staticAIChatProvider{
		answer: `{"classification":"unknown","confidence":0,"evidence":[]}`,
	}, []string{"song-1"})
	if err != nil {
		t.Fatalf("classify explicit: %v", err)
	}
	updated, _ := repo.Get("song-1")
	if len(results) != 1 || results[0].ExplicitStatus != "e" || updated.ExplicitStatus != "e" {
		t.Fatalf("expected explicit status from title marker, results=%+v stored=%q", results, updated.ExplicitStatus)
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
