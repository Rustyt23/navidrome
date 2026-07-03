package nativeapi

import (
	"bytes"
	"context"
	"encoding/json"
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

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/model"
)

type aiChatRequest struct {
	Message  string `json:"message"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type aiChatResponse struct {
	Response string `json:"response"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

type aiLyricsResponse struct {
	Language string `json:"language"`
	Text     string `json:"text"`
}

type aiClassifyExplicitRequest struct {
	SongIDs  []string `json:"songIds"`
	Provider string   `json:"provider"`
}

type aiClassifyExplicitSong struct {
	ID             string `json:"id"`
	ExplicitStatus string `json:"explicitStatus"`
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
	Services []aiServiceStatus `json:"services"`
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

type gemmaChatResponse struct {
	Response string `json:"response"`
	Reply    string `json:"reply"`
	Model    string `json:"model"`
}

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
		client = &http.Client{Timeout: 60 * time.Second}
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

func (n *Router) addAIChatRoute(r chi.Router) {
	r.Get("/ai/status", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 4*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(aiStatusResponse{Services: getAIServiceStatuses(ctx)})
	})

	r.Post("/ai/chat", func(w http.ResponseWriter, req *http.Request) {
		var payload aiChatRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request payload", http.StatusBadRequest)
			return
		}
		payload.Message = strings.TrimSpace(payload.Message)
		if payload.Message == "" {
			http.Error(w, "message is required", http.StatusBadRequest)
			return
		}
		providerSpec := aiChatProviderSpec(payload.Provider, payload.Model)
		if providerSpec.ID == "" {
			http.Error(w, "unsupported AI provider", http.StatusBadRequest)
			return
		}

		provider, err := newAIChatProvider(providerSpec)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		answer, err := provider.Chat(req.Context(), payload.Message)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(aiChatResponse{
			Response: answer,
			Provider: providerSpec.ID,
			Model:    providerSpec.Model,
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

		lyrics, err := fetchWhisperLyrics(req.Context(), whisperURL, mf.AbsolutePath())
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
		songs, err := classifyExplicit(req.Context(), n.ds.MediaFile(req.Context()), provider, payload.SongIDs)
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
	case "gemma-26b", "gemma-26", "gemma-4", "gemma-3", "gemma3", "gemma3:4b":
		return aiProviderSpec{ID: "gemma-26b", Model: "gemma-26b"}
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
			client: &http.Client{Timeout: 60 * time.Second},
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
	apiKey := strings.TrimSpace(conf.Server.GeminiAPIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(conf.Server.AI.GeminiAPIKey)
	}
	return apiKey
}

func getAIServiceStatuses(ctx context.Context) []aiServiceStatus {
	client := &http.Client{Timeout: 4 * time.Second}
	gemmaURL, gemmaAPIKey := gemmaCredentials()
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

func fetchWhisperLyrics(ctx context.Context, whisperURL string, audioPath string) (aiLyricsResponse, error) {
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

func classifyExplicit(ctx context.Context, repo model.MediaFileRepository, provider aiChatProvider, songIDs []string) ([]aiClassifyExplicitSong, error) {
	results := make([]aiClassifyExplicitSong, 0, len(songIDs))
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

		mf, err := repo.Get(songID)
		if err != nil {
			results = append(results, aiClassifyExplicitSong{ID: songID, ExplicitStatus: ""})
			continue
		}

		status := strings.TrimSpace(mf.ExplicitStatus)
		if status != "" {
			results = append(results, aiClassifyExplicitSong{ID: songID, ExplicitStatus: status})
			continue
		}

		lyrics, _ := lyricsText(mf)
		if strings.TrimSpace(lyrics) == "" {
			results = append(results, aiClassifyExplicitSong{ID: songID, ExplicitStatus: ""})
			continue
		}

		classification, err := classifyLyricsExplicit(ctx, provider, lyrics)
		if err != nil {
			results = append(results, aiClassifyExplicitSong{ID: songID, ExplicitStatus: ""})
			continue
		}
		status = explicitStatusCode(classification)
		if status == "" {
			results = append(results, aiClassifyExplicitSong{ID: songID, ExplicitStatus: ""})
			continue
		}
		if err := repo.UpdateExplicitStatus(songID, status); err != nil {
			results = append(results, aiClassifyExplicitSong{ID: songID, ExplicitStatus: ""})
			continue
		}

		results = append(results, aiClassifyExplicitSong{ID: songID, ExplicitStatus: status})
	}

	return results, nil
}

func classifyLyricsExplicit(ctx context.Context, provider aiChatProvider, lyrics string) (string, error) {
	prompt := `Classify this song transcript or lyrics as exactly one value: explicit or clean.

Rules:
- explicit = strong profanity, sexual content, explicit violence, drug abuse, hate speech, or adult themes
- clean = no clear explicit content

Return only one word: explicit or clean.

Lyrics:
` + lyrics

	answer, err := provider.Chat(ctx, prompt)
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(answer)), nil
}

func explicitStatusCode(classification string) string {
	classification = strings.Trim(strings.ToLower(strings.TrimSpace(classification)), ".`\"' \n\t")
	if classification == "clean" {
		return "c"
	}
	if classification == "explicit" {
		return "e"
	}

	for _, token := range strings.FieldsFunc(classification, func(r rune) bool {
		return r < 'a' || r > 'z'
	}) {
		switch token {
		case "clean":
			return "c"
		case "explicit":
			return "e"
		}
	}
	return ""
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
