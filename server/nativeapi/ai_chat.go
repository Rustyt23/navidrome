package nativeapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

type aiChatRequest struct {
	Message  string `json:"message"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	UseRAG   *bool  `json:"useRag,omitempty"`
}

type aiChatResponse struct {
	Response string                 `json:"response"`
	Provider string                 `json:"provider,omitempty"`
	Model    string                 `json:"model,omitempty"`
	Sources  []rag.SongSearchResult `json:"sources,omitempty"`
	RAGError string                 `json:"ragError,omitempty"`
}

type aiLyricsResponse struct {
	Language string `json:"language"`
	Text     string `json:"text"`
}

type aiClassifyExplicitRequest struct {
	SongIDs       []string `json:"songIds"`
	Provider      string   `json:"provider"`
	IncludedWords []string `json:"includedWords,omitempty"`
	ExcludedWords []string `json:"excludedWords,omitempty"`
}

type aiClassifyExplicitSong struct {
	ID             string   `json:"id"`
	ExplicitStatus string   `json:"explicitStatus"`
	Reason         string   `json:"reason,omitempty"`
	Confidence     int      `json:"confidence,omitempty"`
	Evidence       []string `json:"evidence,omitempty"`
}

type aiClassifyExplicitResponse struct {
	Songs []aiClassifyExplicitSong `json:"songs"`
}

type aiFetchMetadataRequest struct {
	SongIDs  []string `json:"songIds"`
	Provider string   `json:"provider"`
	Force    bool     `json:"force"`
}

type aiFetchMetadataSong struct {
	ID              string `json:"id"`
	Album           string `json:"album,omitempty"`
	Year            int    `json:"year,omitempty"`
	AIGenre         string `json:"aiGenre,omitempty"`
	AlbumConfidence int    `json:"albumConfidence"`
	YearConfidence  int    `json:"yearConfidence"`
	GenreConfidence int    `json:"genreConfidence"`
}

type aiFetchMetadataResponse struct {
	Songs []aiFetchMetadataSong `json:"songs"`
}

type aiClearMetadataSong struct {
	ID    string `json:"id"`
	Album bool   `json:"album"`
	Year  bool   `json:"year"`
}

type aiClearMetadataRequest struct {
	Songs []aiClearMetadataSong `json:"songs"`
}

type aiClearMetadataResponse struct {
	SongIDs []string `json:"songIds"`
}

type geminiSongMetadata struct {
	Album           string `json:"album"`
	Year            int    `json:"year"`
	Genre           string `json:"genre"`
	AlbumConfidence int    `json:"albumConfidence"`
	YearConfidence  int    `json:"yearConfidence"`
	GenreConfidence int    `json:"genreConfidence"`
}

type aiChatProvider interface {
	Chat(ctx context.Context, message string) (string, error)
}

type aiProviderSpec struct {
	ID    string
	Model string
}

type aiServiceStatus struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Online bool   `json:"online"`
}

type aiStatusResponse struct {
	Services     []aiServiceStatus `json:"services"`
	WhisperModel string            `json:"whisperModel"`
}

type aiRAGStatusResponse struct {
	Enabled          bool   `json:"enabled"`
	VectorURL        string `json:"vectorUrl"`
	Collection       string `json:"collection"`
	TopK             int    `json:"topK"`
	VectorDBOnline   bool   `json:"vectorDbOnline"`
	CollectionExists bool   `json:"collectionExists"`
	IndexedCount     int64  `json:"indexedCount"`
	Error            string `json:"error,omitempty"`
}

type geminiClient struct {
	apiKey string
	model  string
	client *http.Client
}

type gemmaClient struct {
	apiURL string
	apiKey string
	client *http.Client
}

type ollamaGemmaClient struct {
	apiURL string
	model  string
	client *http.Client
}

type geminiGenerateContentRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerateContentResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
}

type gemmaChatRequest struct {
	Message string `json:"message"`
}

type ollamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type gemmaChatResponse struct {
	Response string `json:"response"`
	Reply    string `json:"reply"`
	Model    string `json:"model"`
}

const gemmaChatTimeout = 120 * time.Second

func (g geminiClient) Chat(ctx context.Context, message string) (string, error) {
	body, _ := json.Marshal(geminiGenerateContentRequest{
		Contents: []geminiContent{{Parts: []geminiPart{{Text: message}}}},
	})

	endpoint := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		url.PathEscape(g.model),
		url.QueryEscape(g.apiKey),
	)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := g.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to contact AI provider: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= http.StatusBadRequest {
		errBody, _ := io.ReadAll(httpResp.Body)
		return "", fmt.Errorf("AI provider error: %s", strings.TrimSpace(string(errBody)))
	}

	var apiResp geminiGenerateContentResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("invalid AI provider response: %w", err)
	}

	answer := ""
	for _, candidate := range apiResp.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				if answer != "" {
					answer += "\n"
				}
				answer += part.Text
			}
		}
	}

	if answer == "" {
		answer = "No response returned from AI provider."
	}

	return answer, nil
}

func (g gemmaClient) Chat(ctx context.Context, message string) (string, error) {
	if strings.TrimSpace(g.apiURL) == "" {
		return "", fmt.Errorf("Gemma API URL is not configured")
	}
	if strings.TrimSpace(g.apiKey) == "" {
		return "", fmt.Errorf("Gemma API key is not configured")
	}
	if _, err := url.ParseRequestURI(g.apiURL); err != nil {
		return "", fmt.Errorf("invalid Gemma API URL: %w", err)
	}

	body, _ := json.Marshal(gemmaChatRequest{Message: message})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.apiURL, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("invalid Gemma API URL: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.apiKey)

	client := g.client
	if client == nil {
		client = &http.Client{Timeout: gemmaChatTimeout}
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to contact Gemma API: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= http.StatusBadRequest {
		errBody, _ := io.ReadAll(httpResp.Body)
		return "", fmt.Errorf("Gemma API error: %s", strings.TrimSpace(string(errBody)))
	}

	var apiResp gemmaChatResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("invalid Gemma API response: %w", err)
	}
	answer := strings.TrimSpace(apiResp.Response)
	if answer == "" {
		answer = strings.TrimSpace(apiResp.Reply)
	}
	if answer == "" {
		return "", fmt.Errorf("Gemma API returned an empty response")
	}
	return answer, nil
}

func (g ollamaGemmaClient) Chat(ctx context.Context, message string) (string, error) {
	if strings.TrimSpace(g.apiURL) == "" {
		return "", fmt.Errorf("Gemma 3:4b API URL is not configured")
	}
	if _, err := url.ParseRequestURI(g.apiURL); err != nil {
		return "", fmt.Errorf("invalid Gemma 3:4b API URL: %w", err)
	}

	body, _ := json.Marshal(ollamaGenerateRequest{
		Model:  g.model,
		Prompt: message,
		Stream: false,
	})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.apiURL, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("invalid Gemma 3:4b API URL: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := g.client
	if client == nil {
		client = &http.Client{Timeout: gemmaChatTimeout}
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to contact Gemma 3:4b API: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= http.StatusBadRequest {
		errBody, _ := io.ReadAll(io.LimitReader(httpResp.Body, 4096))
		return "", fmt.Errorf("Gemma 3:4b API error: %s", strings.TrimSpace(string(errBody)))
	}

	var apiResp gemmaChatResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("invalid Gemma 3:4b API response: %w", err)
	}
	answer := strings.TrimSpace(apiResp.Response)
	if answer == "" {
		return "", fmt.Errorf("Gemma 3:4b API returned an empty response")
	}
	return answer, nil
}

func (n *Router) addAIChatRoute(r chi.Router) {
	r.Post("/ai/rag/search", n.handleRAGSearch)
	r.Post("/ai/rag/playlist/analyze", n.handleRAGPlaylistAnalyze)

	r.Get("/ai/rag/status", func(w http.ResponseWriter, request *http.Request) {
		response := aiRAGStatusResponse{
			Enabled:    ragEnabled(),
			VectorURL:  conf.Server.RAGVectorURL,
			Collection: conf.Server.RAGCollection,
			TopK:       conf.Server.RAGTopK,
		}
		if response.Enabled {
			ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
			defer cancel()

			qdrantStatus := rag.NewQdrantClient(response.VectorURL, response.Collection).Status(ctx, true)
			response.VectorDBOnline = qdrantStatus.VectorDBOnline
			response.CollectionExists = qdrantStatus.CollectionExists
			response.IndexedCount = qdrantStatus.IndexedCount
			response.Error = qdrantStatus.Error
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})

	r.Get("/ai/status", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 4*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(aiStatusResponse{
			Services:     getAIServiceStatuses(ctx),
			WhisperModel: selectedWhisperModel(),
		})
	})

	r.Post("/ai/chat", func(w http.ResponseWriter, req *http.Request) {
		var payload aiChatRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			writeAIChatError(w, http.StatusBadRequest, "invalid request payload")
			return
		}
		payload.Message = strings.TrimSpace(payload.Message)
		if payload.Message == "" {
			writeAIChatError(w, http.StatusBadRequest, "message is required")
			return
		}
		providerSpec := aiChatProviderSpec(payload.Provider, payload.Model)
		if providerSpec.ID == "" {
			writeAIChatError(w, http.StatusBadRequest, "unsupported AI provider")
			return
		}

		provider, err := newAIChatProvider(providerSpec)
		if err != nil {
			writeAIChatError(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		chatMessage := payload.Message
		var sources []rag.SongSearchResult
		ragError := ""
		if shouldUseRAG(payload) {
			chatMessage, sources, err = prepareAIChatMessage(req.Context(), payload.Message, searchRAG)
			if err != nil {
				ragError = err.Error()
				chatMessage = payload.Message
				sources = nil
			}
		}
		answer, err := provider.Chat(req.Context(), chatMessage)
		if err != nil {
			writeAIChatError(w, http.StatusBadGateway, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(aiChatResponse{
			Response: answer,
			Provider: providerSpec.ID,
			Model:    providerSpec.Model,
			Sources:  sources,
			RAGError: ragError,
		})
	})

	r.Post("/ai/songs/{id}/lyrics/fetch", func(w http.ResponseWriter, req *http.Request) {
		whisperURL := strings.TrimSpace(conf.Server.WhisperAPIURL)
		if whisperURL == "" {
			http.Error(w, "Whisper API URL is not configured", http.StatusServiceUnavailable)
			return
		}

		songID := strings.TrimSpace(chi.URLParam(req, "id"))
		if songID == "" {
			http.Error(w, "song id is required", http.StatusBadRequest)
			return
		}

		mf, err := n.ds.MediaFile(req.Context()).Get(songID)
		if err != nil {
			http.Error(w, "song not found", http.StatusNotFound)
			return
		}

		lyrics, err := fetchWhisperLyrics(
			req.Context(),
			whisperURL,
			mf.AbsolutePath(),
			selectedWhisperModel(),
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		structured, err := model.ToLyrics(lyrics.Language, lyrics.Text)
		if err != nil {
			http.Error(w, "invalid lyrics response", http.StatusBadGateway)
			return
		}
		lyricsJSON, err := json.Marshal(model.LyricList{*structured})
		if err != nil {
			http.Error(w, "could not save lyrics", http.StatusInternalServerError)
			return
		}
		if err := saveWhisperLyricsFile(conf.Server.WhisperLyricsFolder, songID, lyrics.Text); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := n.ds.MediaFile(req.Context()).UpdateLyrics(songID, string(lyricsJSON)); err != nil {
			http.Error(w, "could not save lyrics", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(lyrics)
	})

	r.Get("/ai/songs/{id}/lyrics", func(w http.ResponseWriter, req *http.Request) {
		songID := strings.TrimSpace(chi.URLParam(req, "id"))
		if songID == "" {
			http.Error(w, "song id is required", http.StatusBadRequest)
			return
		}

		mf, err := n.ds.MediaFile(req.Context()).Get(songID)
		if err != nil {
			http.Error(w, "song not found", http.StatusNotFound)
			return
		}

		text, language := lyricsText(mf)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(aiLyricsResponse{Language: language, Text: text})
	})

	r.Delete("/ai/songs/{id}/lyrics", func(w http.ResponseWriter, req *http.Request) {
		songID := strings.TrimSpace(chi.URLParam(req, "id"))
		if songID == "" {
			http.Error(w, "song id is required", http.StatusBadRequest)
			return
		}
		if _, err := n.ds.MediaFile(req.Context()).Get(songID); err != nil {
			http.Error(w, "song not found", http.StatusNotFound)
			return
		}
		if err := deleteSongLyrics(n.ds.MediaFile(req.Context()), conf.Server.WhisperLyricsFolder, songID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"deleted": true})
	})

	r.Post("/ai/classify-explicit", func(w http.ResponseWriter, req *http.Request) {
		var payload aiClassifyExplicitRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request payload", http.StatusBadRequest)
			return
		}
		if len(payload.SongIDs) == 0 {
			http.Error(w, "songIds is required", http.StatusBadRequest)
			return
		}

		selectedProvider := strings.TrimSpace(payload.Provider)
		if selectedProvider == "" {
			selectedProvider = "gemini-2.5"
		}
		providerSpec := aiChatProviderSpec(selectedProvider, "")
		if providerSpec.ID == "" {
			http.Error(w, "unsupported AI provider", http.StatusBadRequest)
			return
		}
		provider, err := newAIChatProvider(providerSpec)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		rules := normalizeExplicitWordRules(payload.IncludedWords, payload.ExcludedWords)
		songs, err := classifyExplicit(req.Context(), n.ds.MediaFile(req.Context()), provider, payload.SongIDs, rules)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(aiClassifyExplicitResponse{Songs: songs})
	})

	r.Post("/ai/fetch-metadata", func(w http.ResponseWriter, req *http.Request) {
		var payload aiFetchMetadataRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request payload", http.StatusBadRequest)
			return
		}
		if len(payload.SongIDs) == 0 {
			http.Error(w, "songIds is required", http.StatusBadRequest)
			return
		}

		selectedProvider := strings.TrimSpace(payload.Provider)
		if selectedProvider == "" {
			selectedProvider = "gemini-3.5"
		}
		providerSpec := aiChatProviderSpec(selectedProvider, "")
		if providerSpec.ID == "" {
			http.Error(w, "unsupported AI provider", http.StatusBadRequest)
			return
		}
		provider, err := newAIChatProvider(providerSpec)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		songs, err := fetchSongMetadata(req.Context(), n.ds.MediaFile(req.Context()), provider, payload.SongIDs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(aiFetchMetadataResponse{Songs: songs})
	})

	r.Post("/ai/clear-metadata", func(w http.ResponseWriter, req *http.Request) {
		var payload aiClearMetadataRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request payload", http.StatusBadRequest)
			return
		}
		if len(payload.Songs) == 0 {
			http.Error(w, "songs is required", http.StatusBadRequest)
			return
		}

		cleared, err := clearAIMetadata(n.ds.MediaFile(req.Context()), payload.Songs)
		if err != nil {
			http.Error(w, "could not clear AI metadata", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(aiClearMetadataResponse{SongIDs: cleared})
	})
}

func shouldUseRAG(request aiChatRequest) bool {
	return request.UseRAG == nil || *request.UseRAG
}

func writeAIChatError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}

func aiChatProviderSpec(provider string, modelName string) aiProviderSpec {
	selected := strings.TrimSpace(provider)
	if selected == "" {
		selected = strings.TrimSpace(modelName)
	}

	switch selected {
	case "", "gemini-2.5", "gemini-2.5-flash":
		return aiProviderSpec{ID: "gemini-2.5", Model: "gemini-2.5-flash"}
	case "gemini-3.5", "gemini-3.5-flash":
		return aiProviderSpec{ID: "gemini-3.5", Model: "gemini-3.5-flash"}
	case "gemma-26b", "gemma-26", "gemma-4":
		return aiProviderSpec{ID: "gemma-26b", Model: "gemma-26b"}
	case "gemma-3-4b", "gemma-3:4b", "gemma-3", "gemma3", "gemma3:4b":
		return aiProviderSpec{ID: "gemma-3-4b", Model: "gemma3:4b"}
	default:
		return aiProviderSpec{}
	}
}

func newAIChatProvider(spec aiProviderSpec) (aiChatProvider, error) {
	switch spec.ID {
	case "gemini-2.5", "gemini-3.5":
		apiKey := geminiAPIKey()
		if apiKey == "" {
			return nil, fmt.Errorf("Gemini API key is not configured")
		}
		return geminiClient{
			apiKey: apiKey,
			model:  spec.Model,
			client: http.DefaultClient,
		}, nil
	case "gemma-26b":
		apiURL, apiKey := gemmaCredentials()
		if apiURL == "" {
			return nil, fmt.Errorf("Gemma API URL is not configured")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("Gemma API key is not configured")
		}
		return gemmaClient{
			apiURL: apiURL,
			apiKey: apiKey,
			client: &http.Client{Timeout: gemmaChatTimeout},
		}, nil
	case "gemma-3-4b":
		apiURL := gemma4APIURL()
		if apiURL == "" {
			return nil, fmt.Errorf("Gemma 3:4b API URL is not configured")
		}
		return ollamaGemmaClient{
			apiURL: apiURL,
			model:  spec.Model,
			client: &http.Client{Timeout: gemmaChatTimeout},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported AI provider")
	}
}

func gemmaCredentials() (string, string) {
	apiURL := strings.TrimSpace(os.Getenv("ND_GEMMA_API_URL"))
	apiKey := strings.TrimSpace(os.Getenv("ND_GEMMA_API_KEY"))
	if apiURL == "" {
		apiURL = strings.TrimSpace(conf.Server.GemmaAPIURL)
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(conf.Server.GemmaAPIKey)
	}
	return apiURL, apiKey
}

func geminiAPIKey() string {
	return strings.TrimSpace(conf.Server.GeminiAPIKey)
}

func gemma4APIURL() string {
	apiURL := strings.TrimSpace(os.Getenv("ND_GEMMA4APIURL"))
	if apiURL == "" {
		apiURL = strings.TrimSpace(conf.Server.Gemma4APIURL)
	}
	return apiURL
}

func getAIServiceStatuses(ctx context.Context) []aiServiceStatus {
	client := &http.Client{Timeout: 4 * time.Second}
	gemmaURL, gemmaAPIKey := gemmaCredentials()
	gemma4URL := gemma4APIURL()
	whisperURL := strings.TrimSpace(conf.Server.WhisperAPIURL)
	geminiKey := geminiAPIKey()

	checks := []struct {
		id    string
		label string
		probe func() bool
	}{
		{
			id:    "gemma-26b",
			label: "Gemma 26B",
			probe: func() bool {
				return gemmaAPIKey != "" && probeGemmaEndpoint(ctx, client, gemmaURL, gemmaAPIKey)
			},
		},
		{
			id:    "gemma-3-4b",
			label: "Gemma 3:4b",
			probe: func() bool {
				return probeGemmaEndpoint(ctx, client, gemma4URL, "")
			},
		},
		{
			id:    "whisper",
			label: "Whisper",
			probe: func() bool { return probeAIEndpoint(ctx, client, whisperURL, "") },
		},
		{
			id:    "gemini-2.5",
			label: "Gemini 2.5",
			probe: func() bool { return probeGeminiModel(ctx, client, geminiKey, "gemini-2.5-flash") },
		},
		{
			id:    "gemini-3.5",
			label: "Gemini 3.5",
			probe: func() bool { return probeGeminiModel(ctx, client, geminiKey, "gemini-3.5-flash") },
		},
	}

	statuses := make([]aiServiceStatus, len(checks))
	var wg sync.WaitGroup
	for i := range checks {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			check := checks[index]
			statuses[index] = aiServiceStatus{
				ID:     check.id,
				Label:  check.label,
				Online: check.probe(),
			}
		}(i)
	}
	wg.Wait()
	return statuses
}

func probeAIEndpoint(ctx context.Context, client *http.Client, endpoint string, apiKey string) bool {
	return probeAIEndpointMethod(ctx, client, endpoint, apiKey, http.MethodHead)
}

func probeGemmaEndpoint(ctx context.Context, client *http.Client, endpoint string, apiKey string) bool {
	return probeAIEndpointMethod(ctx, client, endpoint, apiKey, http.MethodOptions)
}

func probeAIEndpointMethod(ctx context.Context, client *http.Client, endpoint string, apiKey string, method string) bool {
	if strings.TrimSpace(endpoint) == "" {
		return false
	}
	if _, err := url.ParseRequestURI(endpoint); err != nil {
		return false
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return false
	}
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusBadRequest ||
		resp.StatusCode == http.StatusMethodNotAllowed
}

func probeGeminiModel(ctx context.Context, client *http.Client, apiKey string, model string) bool {
	if strings.TrimSpace(apiKey) == "" {
		return false
	}
	endpoint := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s?key=%s",
		url.PathEscape(model),
		url.QueryEscape(apiKey),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusBadRequest
}

func fetchWhisperLyrics(ctx context.Context, whisperURL string, audioPath string, whisperModel string) (aiLyricsResponse, error) {
	file, err := os.Open(audioPath)
	if err != nil {
		return aiLyricsResponse{}, fmt.Errorf("could not open audio file: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return aiLyricsResponse{}, fmt.Errorf("could not prepare audio upload: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return aiLyricsResponse{}, fmt.Errorf("could not read audio file: %w", err)
	}
	if strings.TrimSpace(whisperModel) != "" {
		if err := writer.WriteField("model", strings.TrimSpace(whisperModel)); err != nil {
			return aiLyricsResponse{}, fmt.Errorf("could not add Whisper model: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return aiLyricsResponse{}, fmt.Errorf("could not finish audio upload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, whisperURL, &body)
	if err != nil {
		return aiLyricsResponse{}, fmt.Errorf("invalid Whisper API URL: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return aiLyricsResponse{}, fmt.Errorf("failed to contact Whisper API: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= http.StatusBadRequest {
		errBody, _ := io.ReadAll(httpResp.Body)
		return aiLyricsResponse{}, fmt.Errorf("Whisper API error: %s", strings.TrimSpace(string(errBody)))
	}

	var lyrics aiLyricsResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&lyrics); err != nil {
		return aiLyricsResponse{}, fmt.Errorf("invalid Whisper API response: %w", err)
	}
	lyrics.Language = strings.TrimSpace(lyrics.Language)
	lyrics.Text = strings.TrimSpace(lyrics.Text)
	if lyrics.Language == "" {
		lyrics.Language = "xxx"
	}
	if lyrics.Text == "" {
		return aiLyricsResponse{}, fmt.Errorf("Whisper API returned empty lyrics")
	}

	return lyrics, nil
}

func saveWhisperLyricsFile(folder string, songID string, text string) error {
	path, err := whisperLyricsFilePath(folder, songID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("could not create lyrics folder: %w", err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(text)+"\n"), 0o644); err != nil {
		return fmt.Errorf("could not save lyrics file: %w", err)
	}
	return nil
}

func whisperLyricsFilePath(folder string, songID string) (string, error) {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		folder = "./lyrics"
	}
	filename := strings.Map(func(value rune) rune {
		switch {
		case value >= 'a' && value <= 'z':
			return value
		case value >= 'A' && value <= 'Z':
			return value
		case value >= '0' && value <= '9':
			return value
		case value == '-' || value == '_':
			return value
		default:
			return '_'
		}
	}, strings.TrimSpace(songID))
	if filename == "" {
		return "", fmt.Errorf("could not save lyrics: song ID is empty")
	}
	return filepath.Join(folder, filename+".txt"), nil
}

func deleteSongLyrics(repo model.MediaFileRepository, folder, songID string) error {
	path, err := whisperLyricsFilePath(folder, songID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("could not delete lyrics file: %w", err)
	}
	if err := repo.UpdateLyrics(songID, ""); err != nil {
		return fmt.Errorf("could not delete lyrics: %w", err)
	}
	return nil
}

func lyricsText(mf *model.MediaFile) (string, string) {
	lyrics, err := mf.StructuredLyrics()
	if err != nil || len(lyrics) == 0 {
		return "", ""
	}

	var out strings.Builder
	language := lyrics[0].Lang
	for _, lyric := range lyrics {
		if language == "" {
			language = lyric.Lang
		}
		for _, line := range lyric.Line {
			if line.Value == "" {
				continue
			}
			if out.Len() > 0 {
				out.WriteString("\n")
			}
			out.WriteString(line.Value)
		}
	}
	return out.String(), language
}

type explicitWordRules struct {
	Included []string
	Excluded []string
}

type explicitClassificationResult struct {
	Classification string
	Confidence     int
	Reason         string
	Evidence       []string
}

var defaultExplicitWordRules = explicitWordRules{
	Included: []string{"fuck", "fucking", "motherfucker", "shit", "bitch", "cunt", "nigga", "nigger", "pussy", "dick", "cock"},
	Excluded: []string{"damn", "hell", "crap", "ass", "alcohol", "drunk", "weed", "marijuana", "kiss", "kissing", "sexy", "gun", "kill"},
}

func normalizeExplicitWordRules(included, excluded []string) explicitWordRules {
	if included == nil && excluded == nil {
		return defaultExplicitWordRules
	}
	normalize := func(values []string) []string {
		result := make([]string, 0, min(len(values), 200))
		seen := map[string]struct{}{}
		for _, value := range values {
			value = strings.ToLower(strings.TrimSpace(value))
			if value == "" || len(value) > 80 {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			result = append(result, value)
			if len(result) == 200 {
				break
			}
		}
		return result
	}
	return explicitWordRules{Included: normalize(included), Excluded: normalize(excluded)}
}

func classifyExplicit(ctx context.Context, repo model.MediaFileRepository, provider aiChatProvider, songIDs []string, configuredRules ...explicitWordRules) ([]aiClassifyExplicitSong, error) {
	results := make([]aiClassifyExplicitSong, 0, len(songIDs))
	seen := map[string]struct{}{}
	rules := defaultExplicitWordRules
	if len(configuredRules) > 0 {
		rules = configuredRules[0]
	}

	for _, rawID := range songIDs {
		songID := strings.TrimSpace(rawID)
		if songID == "" {
			continue
		}
		if _, ok := seen[songID]; ok {
			continue
		}
		seen[songID] = struct{}{}

		mf, err := repo.Get(songID)
		if err != nil {
			results = append(results, aiClassifyExplicitSong{ID: songID, ExplicitStatus: ""})
			continue
		}

		existingStatus := strings.TrimSpace(mf.ExplicitStatus)

		lyrics, _ := lyricsText(mf)
		if strings.TrimSpace(lyrics) == "" {
			results = append(results, aiClassifyExplicitSong{
				ID: songID, ExplicitStatus: existingStatus,
				Reason: "No saved lyrics were available, so the existing status was preserved.",
			})
			continue
		}

		classification, err := classifyLyricsExplicitDetailed(ctx, provider, lyrics, rules)
		if err != nil {
			results = append(results, aiClassifyExplicitSong{
				ID: songID, ExplicitStatus: existingStatus,
				Reason: "Classification failed, so the existing status was preserved.",
			})
			continue
		}
		status := explicitStatusCode(classification.Classification)
		if status == "" {
			reason := "The new result did not meet the confidence and evidence requirements, so the existing status was preserved."
			if classification.Reason != "" {
				reason += " Provider assessment: " + classification.Reason
			}
			results = append(results, aiClassifyExplicitSong{
				ID: songID, ExplicitStatus: existingStatus, Reason: reason,
				Confidence: classification.Confidence, Evidence: classification.Evidence,
			})
			continue
		}
		if status == existingStatus {
			results = append(results, aiClassifyExplicitSong{
				ID: songID, ExplicitStatus: status, Reason: classification.Reason,
				Confidence: classification.Confidence, Evidence: classification.Evidence,
			})
			continue
		}
		if err := repo.UpdateExplicitStatus(songID, status); err != nil {
			results = append(results, aiClassifyExplicitSong{
				ID: songID, ExplicitStatus: existingStatus,
				Reason: "The new result could not be saved, so the existing status was preserved.",
			})
			continue
		}

		results = append(results, aiClassifyExplicitSong{
			ID: songID, ExplicitStatus: status, Reason: classification.Reason,
			Confidence: classification.Confidence, Evidence: classification.Evidence,
		})
	}

	return results, nil
}

func classifyLyricsExplicit(ctx context.Context, provider aiChatProvider, lyrics string, configuredRules ...explicitWordRules) (string, error) {
	rules := defaultExplicitWordRules
	if len(configuredRules) > 0 {
		rules = configuredRules[0]
	}
	result, err := classifyLyricsExplicitDetailed(ctx, provider, lyrics, rules)
	return result.Classification, err
}

func classifyLyricsExplicitDetailed(ctx context.Context, provider aiChatProvider, lyrics string, rules explicitWordRules) (explicitClassificationResult, error) {
	includedJSON, _ := json.Marshal(rules.Included)
	excludedJSON, _ := json.Marshal(rules.Excluded)
	prompt := `Classify the supplied song lyrics conservatively as explicit, clean, or unknown.

Rules:
- explicit only when the lyrics contain clear, uncensored strong profanity, graphic or direct sexual language, hateful slurs, or graphic violence
- clean when the lyrics are understandable and contain none of those explicit signals
- unknown when the transcript is incomplete, corrupted, mostly non-lyrical, or the classification is uncertain
- do not mark a song explicit solely for romance, kissing, alcohol, partying, mild insults, innuendo, vague adult themes, non-graphic drug references, or non-graphic references to violence
- ignore transcription artifacts and explanatory text
- evidence must contain exact short quotations copied from the supplied lyrics
- configured explicit words are strong indicators when used with their normal explicit meaning
- configured excluded words must not make a song explicit by themselves, but the surrounding phrase may still be explicit for another clear reason

Configured explicit words:
` + string(includedJSON) + `

Configured excluded words:
` + string(excludedJSON) + `

Return only valid JSON in this exact shape, without markdown:
{"classification":"explicit|clean|unknown","confidence":0,"reason":"short explanation","evidence":["exact lyric quotation"]}

Confidence must be a whole number from 0 to 100. For clean or unknown, evidence may be empty.

Lyrics:
` + lyrics

	answer, err := provider.Chat(ctx, prompt)
	if err != nil {
		return explicitClassificationResult{}, err
	}
	return parseExplicitClassificationDetailed(answer, lyrics), nil
}

func explicitStatusCode(classification string) string {
	classification = strings.Trim(strings.ToLower(strings.TrimSpace(classification)), ".`\"' \n\t")
	if classification == "clean" {
		return "c"
	}
	if classification == "explicit" {
		return "e"
	}
	return ""
}

const (
	minimumExplicitConfidence = 90
	minimumCleanConfidence    = 80
)

func parseExplicitClassification(answer, lyrics string) string {
	return parseExplicitClassificationDetailed(answer, lyrics).Classification
}

func parseExplicitClassificationDetailed(answer, lyrics string) explicitClassificationResult {
	answer = strings.TrimSpace(answer)
	if strings.EqualFold(strings.Trim(answer, ".`\"' \n\t"), "clean") {
		// Conservative compatibility for providers that ignore the JSON format:
		// accepting a clean verdict cannot create an explicit false positive.
		return explicitClassificationResult{
			Classification: "clean",
			Reason:         "The provider returned a clean verdict and no explicit evidence.",
		}
	}

	if strings.HasPrefix(answer, "```") {
		answer = strings.TrimPrefix(answer, "```json")
		answer = strings.TrimPrefix(answer, "```")
		answer = strings.TrimSuffix(answer, "```")
		answer = strings.TrimSpace(answer)
	}
	if start := strings.Index(answer, "{"); start >= 0 {
		if end := strings.LastIndex(answer, "}"); end > start {
			answer = answer[start : end+1]
		}
	}

	var response struct {
		Classification string      `json:"classification"`
		Confidence     interface{} `json:"confidence"`
		Reason         string      `json:"reason"`
		Evidence       []string    `json:"evidence"`
	}
	if err := json.Unmarshal([]byte(answer), &response); err != nil {
		return explicitClassificationResult{Reason: "The provider returned an invalid classification response."}
	}

	classification := strings.ToLower(strings.TrimSpace(response.Classification))
	confidence := parseMetadataConfidence(response.Confidence)
	result := explicitClassificationResult{
		Confidence: confidence,
		Reason:     strings.TrimSpace(response.Reason),
		Evidence:   response.Evidence,
	}
	switch classification {
	case "clean":
		if confidence >= minimumCleanConfidence {
			result.Classification = "clean"
			if result.Reason == "" {
				result.Reason = "No qualifying explicit language was found in the supplied lyrics."
			}
			return result
		}
	case "explicit":
		verifiedEvidence := verifiableExplicitEvidence(lyrics, response.Evidence)
		result.Evidence = verifiedEvidence
		if confidence >= minimumExplicitConfidence && len(verifiedEvidence) > 0 {
			result.Classification = "explicit"
			if result.Reason == "" {
				result.Reason = "The lyrics contain verified explicit language."
			}
			return result
		}
	}
	if result.Reason == "" {
		result.Reason = "The classification was uncertain or lacked verifiable lyric evidence."
	}
	return result
}

func hasVerifiableExplicitEvidence(lyrics string, evidence []string) bool {
	return len(verifiableExplicitEvidence(lyrics, evidence)) > 0
}

func verifiableExplicitEvidence(lyrics string, evidence []string) []string {
	normalizedLyrics := normalizeExplicitEvidence(lyrics)
	verified := make([]string, 0, len(evidence))
	for _, excerpt := range evidence {
		normalizedExcerpt := normalizeExplicitEvidence(excerpt)
		if len(normalizedExcerpt) >= 3 && strings.Contains(normalizedLyrics, normalizedExcerpt) {
			verified = append(verified, strings.TrimSpace(excerpt))
		}
	}
	return verified
}

func normalizeExplicitEvidence(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, value)
	return strings.Join(strings.Fields(value), " ")
}

func fetchSongMetadata(ctx context.Context, repo model.MediaFileRepository, provider aiChatProvider, songIDs []string) ([]aiFetchMetadataSong, error) {
	results := make([]aiFetchMetadataSong, 0, len(songIDs))
	seen := map[string]struct{}{}

	for _, rawID := range songIDs {
		songID := strings.TrimSpace(rawID)
		if songID == "" {
			continue
		}
		if _, ok := seen[songID]; ok {
			continue
		}
		seen[songID] = struct{}{}

		result := aiFetchMetadataSong{ID: songID}
		mf, err := repo.Get(songID)
		if err != nil {
			results = append(results, result)
			continue
		}

		metadata, err := fetchGeminiSongMetadata(ctx, provider, mf)
		if err != nil {
			results = append(results, result)
			continue
		}

		var album *string
		var year *int
		if metadata.Album != "" && strings.EqualFold(strings.TrimSpace(metadata.Album), strings.TrimSpace(mf.Album)) {
			result.AlbumConfidence = metadata.AlbumConfidence
		}
		if metadata.Year > 0 && metadata.Year == mf.Year {
			result.YearConfidence = metadata.YearConfidence
		}
		if albumNeedsFetch(mf.Album) && metadata.Album != "" {
			album = &metadata.Album
			result.Album = metadata.Album
			result.AlbumConfidence = metadata.AlbumConfidence
		}
		if yearNeedsFetch(mf.Year) && metadata.Year > 0 {
			year = &metadata.Year
			result.Year = metadata.Year
			result.YearConfidence = metadata.YearConfidence
		}
		if album != nil || year != nil {
			if err := repo.UpdateMissingMetadata(songID, album, year, nil, nil, nil); err != nil {
				result.Album = ""
				result.Year = 0
				result.AlbumConfidence = 0
				result.YearConfidence = 0
			}
		}

		result.AIGenre = metadata.Genre
		if result.AIGenre != "" {
			result.GenreConfidence = metadata.GenreConfidence
		}
		results = append(results, result)
	}

	return results, nil
}

func clearAIMetadata(repo model.MediaFileRepository, songs []aiClearMetadataSong) ([]string, error) {
	cleared := make([]string, 0, len(songs))
	seen := map[string]struct{}{}
	for _, song := range songs {
		songID := strings.TrimSpace(song.ID)
		if songID == "" {
			continue
		}
		if _, ok := seen[songID]; ok {
			continue
		}
		seen[songID] = struct{}{}
		if err := repo.ClearAIMetadata(songID, song.Album, song.Year); err != nil {
			return cleared, err
		}
		cleared = append(cleared, songID)
	}
	return cleared, nil
}

func fetchGeminiSongMetadata(ctx context.Context, provider aiChatProvider, mf *model.MediaFile) (geminiSongMetadata, error) {
	lyrics, _ := lyricsText(mf)
	answer, err := provider.Chat(ctx, songMetadataPrompt(mf, lyrics))
	if err != nil {
		return geminiSongMetadata{}, err
	}
	return parseGeminiSongMetadata(answer)
}

func songMetadataPrompt(mf *model.MediaFile, lyrics string) string {
	var b strings.Builder
	b.WriteString(`Find the most likely album, release year, and concise genre for this song.

Return only valid JSON in this exact shape:
{"album":"album name or empty string","albumConfidence":0,"year":0,"yearConfidence":0,"genre":"genre or empty string","genreConfidence":0}

Confidence values must be whole numbers from 0 to 100 for each individual field. If current metadata is supplied and appears correct, return it unchanged and score it. Use 0 or an empty string when you are not confident. Do not include markdown.

Song:
`)
	b.WriteString("Title: ")
	b.WriteString(mf.Title)
	b.WriteString("\nArtist: ")
	b.WriteString(mf.Artist)
	b.WriteString("\nCurrent album: ")
	b.WriteString(mf.Album)
	b.WriteString("\nCurrent year: ")
	b.WriteString(fmt.Sprint(mf.Year))
	b.WriteString("\nCurrent genre: ")
	b.WriteString(mf.Genre)
	if strings.TrimSpace(lyrics) != "" {
		b.WriteString("\nLyrics/transcript:\n")
		b.WriteString(lyrics)
	}
	return b.String()
}

func parseGeminiSongMetadata(answer string) (geminiSongMetadata, error) {
	answer = strings.TrimSpace(answer)
	if strings.HasPrefix(answer, "```") {
		answer = strings.TrimPrefix(answer, "```json")
		answer = strings.TrimPrefix(answer, "```")
		answer = strings.TrimSuffix(answer, "```")
		answer = strings.TrimSpace(answer)
	}
	if start := strings.Index(answer, "{"); start >= 0 {
		if end := strings.LastIndex(answer, "}"); end > start {
			answer = answer[start : end+1]
		}
	}

	var raw struct {
		Album           string      `json:"album"`
		Year            interface{} `json:"year"`
		Genre           string      `json:"genre"`
		AlbumConfidence interface{} `json:"albumConfidence"`
		YearConfidence  interface{} `json:"yearConfidence"`
		GenreConfidence interface{} `json:"genreConfidence"`
		Confidence      interface{} `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(answer), &raw); err != nil {
		return geminiSongMetadata{}, err
	}

	legacyConfidence := parseMetadataConfidence(raw.Confidence)
	metadata := geminiSongMetadata{
		Album:           strings.TrimSpace(raw.Album),
		Genre:           strings.TrimSpace(raw.Genre),
		AlbumConfidence: metadataConfidenceOr(raw.AlbumConfidence, legacyConfidence),
		YearConfidence:  metadataConfidenceOr(raw.YearConfidence, legacyConfidence),
		GenreConfidence: metadataConfidenceOr(raw.GenreConfidence, legacyConfidence),
	}
	if albumNeedsFetch(metadata.Album) {
		metadata.Album = ""
		metadata.AlbumConfidence = 0
	}
	switch year := raw.Year.(type) {
	case float64:
		metadata.Year = int(year)
	case string:
		_, _ = fmt.Sscanf(strings.TrimSpace(year), "%d", &metadata.Year)
	}
	if metadata.Year < 1900 || metadata.Year > time.Now().Year()+1 {
		metadata.Year = 0
		metadata.YearConfidence = 0
	}
	if metadata.Genre == "" {
		metadata.GenreConfidence = 0
	}
	return metadata, nil
}

func metadataConfidenceOr(value interface{}, fallback int) int {
	confidence := parseMetadataConfidence(value)
	if confidence == 0 {
		return fallback
	}
	return confidence
}

func parseMetadataConfidence(value interface{}) int {
	confidence := 0.0
	switch typed := value.(type) {
	case float64:
		confidence = typed
	case string:
		_, _ = fmt.Sscanf(strings.TrimSpace(strings.TrimSuffix(typed, "%")), "%f", &confidence)
	}
	if confidence < 0 {
		return 0
	}
	if confidence > 100 {
		return 100
	}
	return int(confidence + 0.5)
}

func albumNeedsFetch(album string) bool {
	album = strings.ToLower(strings.TrimSpace(album))
	return album == "" || album == "unknown" || album == "unknown album" || album == "[unknown album]"
}

func yearNeedsFetch(year int) bool {
	return year <= 0
}
