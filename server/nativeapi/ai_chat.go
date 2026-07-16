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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

type aiChatTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type aiChatRequest struct {
	Message        string       `json:"message"`
	Provider       string       `json:"provider"`
	Model          string       `json:"model"`
	UseRAG         *bool        `json:"useRag,omitempty"`
	History        []aiChatTurn `json:"history,omitempty"`
	DeveloperTrace bool         `json:"developerTrace,omitempty"`
}

type aiChatResponse struct {
	Response string                 `json:"response"`
	Provider string                 `json:"provider,omitempty"`
	Model    string                 `json:"model,omitempty"`
	Sources  []rag.SongSearchResult `json:"sources,omitempty"`
	RAGError string                 `json:"ragError,omitempty"`
	Direct   bool                   `json:"direct,omitempty"`
	Trace    *aiChatTrace           `json:"trace,omitempty"`
}

type aiLyricsResponse struct {
	Language string `json:"language"`
	Text     string `json:"text"`
	Status   string `json:"status,omitempty"`
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
	ID                  string                         `json:"id"`
	AIGenre             string                         `json:"aiGenre,omitempty"`
	SpotifyGenre        string                         `json:"spotifyGenre,omitempty"`
	MusicBrainzGenre    string                         `json:"musicBrainzGenre,omitempty"`
	GenreConfidence     int                            `json:"genreConfidence"`
	ConfidenceBreakdown *aiMetadataConfidenceBreakdown `json:"confidenceBreakdown,omitempty"`
	GenreDeveloperTrace *genreDeveloperTrace           `json:"genreDeveloperTrace,omitempty"`
}

type genreDeveloperTrace struct {
	ITunes *genreSourceDeveloperTrace `json:"itunes,omitempty"`
	AI     *genreSourceDeveloperTrace `json:"ai,omitempty"`
}

type genreSourceDeveloperTrace struct {
	Source       string                  `json:"source"`
	Provider     string                  `json:"provider,omitempty"`
	Model        string                  `json:"model,omitempty"`
	Request      string                  `json:"request,omitempty"`
	Prompt       string                  `json:"prompt,omitempty"`
	Response     string                  `json:"response,omitempty"`
	FetchedGenre string                  `json:"fetchedGenre,omitempty"`
	Attempts     []genreDeveloperAttempt `json:"attempts,omitempty"`
}

type genreDeveloperAttempt struct {
	Number   int    `json:"number"`
	Prompt   string `json:"prompt,omitempty"`
	Response string `json:"response,omitempty"`
	Error    string `json:"error,omitempty"`
}

// aiMetadataConfidenceBreakdown explains, per field, how the value and its
// confidence were derived from the available sources, so the UI can show it when
// a score is clicked.
type aiMetadataConfidenceBreakdown struct {
	Genre aiMetadataFieldExplanation `json:"genre"`
}

// aiMetadataFieldExplanation describes how a single metadata field was resolved.
// Source is one of: "verified" (two sources agree), "spotify", "musicbrainz",
// "ai-only" (only the language model provided it), "conflict" (the stored value
// disagrees with the sources), or "none".
type aiMetadataFieldExplanation struct {
	Source      string `json:"source"`
	Spotify     string `json:"spotify,omitempty"`
	MusicBrainz string `json:"musicBrainz,omitempty"`
	AI          string `json:"ai,omitempty"`
	Confidence  int    `json:"confidence"`
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

// Confidence scores by how a field was resolved. AI-only values score low
// because a language model recalling metadata from memory cannot be trusted
// without an independent cross-check; Spotify and MusicBrainz are external
// sources, and agreement between any two sources is treated as verified.
const (
	metadataConfidenceVerified      = 100 // two independent sources agree
	metadataConfidenceAuthoritative = 85  // taken from MusicBrainz alone
	metadataConfidenceSpotify       = 80  // taken from Spotify alone
	metadataConfidenceConflict      = 55  // stored value disagrees with sources
	metadataConfidenceAIOnly        = 35  // language-model guess, unverified
)

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
	State  string `json:"state"`
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
	ReindexRequired  bool   `json:"reindexRequired,omitempty"`
	EmbeddingBackend string `json:"embeddingBackend"`
	EmbeddingModel   string `json:"embeddingModel"`
	EmbeddingLocal   bool   `json:"embeddingLocal"`
	OfflineMode      bool   `json:"offlineMode"`
	Error            string `json:"error,omitempty"`
}

type geminiClient struct {
	apiKey string
	model  string
	client *http.Client
}

type bedrockDeepSeekClient struct {
	apiURL      string
	bearerToken string
	model       string
	maxTokens   int
	client      *http.Client
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

type bedrockConverseRequest struct {
	Messages        []bedrockMessage       `json:"messages"`
	System          []bedrockContent       `json:"system"`
	InferenceConfig bedrockInferenceConfig `json:"inferenceConfig"`
}

type bedrockMessage struct {
	Role    string           `json:"role"`
	Content []bedrockContent `json:"content"`
}

type bedrockContent struct {
	Text string `json:"text"`
}

type bedrockInferenceConfig struct {
	MaxTokens   int     `json:"maxTokens"`
	Temperature float64 `json:"temperature"`
}

type bedrockConverseResponse struct {
	Output struct {
		Message bedrockMessage `json:"message"`
	} `json:"output"`
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

const (
	gemmaChatTimeout        = 120 * time.Second
	bedrockDeepSeekAPIURL   = "https://bedrock-runtime.us-east-1.amazonaws.com"
	bedrockDeepSeekModel    = "deepseek.v3.2"
	deepSeekChatMaxTokens   = 4096
	deepSeekChatTemperature = 0.2
	deepSeekEnglishPrompt   = "Always respond only in English. Do not respond in Chinese or any other language."
)

func (g geminiClient) Chat(ctx context.Context, message string) (string, error) {
	body, _ := json.Marshal(geminiGenerateContentRequest{
		Contents: []geminiContent{{Parts: []geminiPart{{Text: message}}}},
	})

	endpoint := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		url.PathEscape(g.model),
		url.QueryEscape(g.apiKey),
	)
	client := g.client
	if client == nil {
		client = &http.Client{Timeout: gemmaChatTimeout}
	}
	httpResp, err := rag.DoWithRetry(ctx, client, "gemini_chat", rag.HTTPClientOptions{
		MaxRetries:   conf.Server.RAGRetryMax,
		RetryBackoff: conf.Server.RAGRetryBackoff,
	}, func() (*http.Request, error) {
		httpReq, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if requestErr != nil {
			return nil, requestErr
		}
		httpReq.Header.Set("Content-Type", "application/json")
		return httpReq, nil
	})
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

func (b bedrockDeepSeekClient) Chat(ctx context.Context, message string) (string, error) {
	maxTokens := b.maxTokens
	if maxTokens <= 0 {
		maxTokens = deepSeekChatMaxTokens
	}
	body, err := json.Marshal(bedrockConverseRequest{
		System: []bedrockContent{{Text: deepSeekEnglishPrompt}},
		Messages: []bedrockMessage{{
			Role:    "user",
			Content: []bedrockContent{{Text: message}},
		}},
		InferenceConfig: bedrockInferenceConfig{
			MaxTokens:   maxTokens,
			Temperature: deepSeekChatTemperature,
		},
	})
	if err != nil {
		return "", fmt.Errorf("could not encode DeepSeek request: %w", err)
	}

	endpoint := fmt.Sprintf(
		"%s/model/%s/converse",
		strings.TrimRight(b.apiURL, "/"),
		url.PathEscape(b.model),
	)
	client := b.client
	if client == nil {
		client = &http.Client{Timeout: gemmaChatTimeout}
	}
	httpResp, err := rag.DoWithRetry(ctx, client, "bedrock_deepseek_chat", rag.HTTPClientOptions{
		MaxRetries:   conf.Server.RAGRetryMax,
		RetryBackoff: conf.Server.RAGRetryBackoff,
	}, func() (*http.Request, error) {
		httpReq, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if requestErr != nil {
			return nil, requestErr
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+b.bearerToken)
		return httpReq, nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to contact DeepSeek on Amazon Bedrock: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= http.StatusBadRequest {
		errBody, _ := io.ReadAll(httpResp.Body)
		return "", fmt.Errorf("DeepSeek on Amazon Bedrock returned HTTP %d: %s", httpResp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	var apiResp bedrockConverseResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("invalid DeepSeek response: %w", err)
	}
	answerParts := make([]string, 0, len(apiResp.Output.Message.Content))
	for _, content := range apiResp.Output.Message.Content {
		if text := strings.TrimSpace(content.Text); text != "" {
			answerParts = append(answerParts, text)
		}
	}
	if len(answerParts) == 0 {
		return "", fmt.Errorf("DeepSeek on Amazon Bedrock returned an empty response")
	}
	return strings.Join(answerParts, "\n"), nil
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
	r.Post("/ai/rag/lyrics/search", n.handleRAGLyricSearch)
	r.Get("/ai/rag/duplicates", n.handleRAGDuplicates)
	r.Post("/ai/rag/playlist/analyze", n.handleRAGPlaylistAnalyze)
	r.Post("/ai/rag/recommend", n.handleRAGRecommendation)
	r.Get("/ai/rag/reports/dashboard", n.handleRAGDashboardReport)

	r.Get("/ai/rag/status", func(w http.ResponseWriter, request *http.Request) {
		backend, model, local := ragEmbeddingStatus()
		response := aiRAGStatusResponse{
			Enabled:          ragEnabled(),
			VectorURL:        conf.Server.RAGVectorURL,
			Collection:       conf.Server.RAGCollection,
			TopK:             conf.Server.RAGTopK,
			EmbeddingBackend: backend,
			EmbeddingModel:   model,
			EmbeddingLocal:   local,
			OfflineMode:      conf.Server.RAGOffline,
		}
		if response.Enabled {
			if conf.Server.RAGOffline && strings.TrimSpace(conf.Server.RAGEmbeddingURL) == "" {
				response.Error = "RAG offline mode requires RAGEmbeddingURL"
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)
				return
			}
			ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
			defer cancel()

			qdrantStatus := newRAGQdrantClient().Status(ctx, true)
			response.VectorDBOnline = qdrantStatus.VectorDBOnline
			response.CollectionExists = qdrantStatus.CollectionExists
			response.IndexedCount = qdrantStatus.IndexedCount
			response.ReindexRequired = qdrantStatus.ReindexRequired
			response.Error = qdrantStatus.Error
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	})

	r.Get("/ai/status", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 4*time.Second)
		defer cancel()
		whisperBusy := n.lyricsJobManager().Busy()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(aiStatusResponse{
			Services:     getAIServiceStatuses(ctx, whisperBusy),
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
		useRAG := shouldUseRAG(payload)
		exactNeedle, directExactResponse := "", false
		if useRAG {
			exactNeedle, directExactResponse = directLyricsContains(payload.Message)
		}
		trace := newAIChatTraceCollector(payload.DeveloperTrace, providerSpec.ID, providerSpec.Model, useRAG)
		ctx := withAIChatTrace(req.Context(), trace)
		recordAIChatTraceStage(ctx, aiChatTraceStage{
			ID:     "request",
			Label:  "Received chat request",
			Status: "completed",
			Input: map[string]any{
				"message": payload.Message,
				"history": payload.History,
				"useRag":  useRAG,
			},
		})
		if useRAG && conf.Server.RAGOffline && !directExactResponse {
			if strings.TrimSpace(conf.Server.RAGEmbeddingURL) == "" {
				writeAIChatErrorWithTrace(w, http.StatusServiceUnavailable, "RAG offline mode requires RAGEmbeddingURL", trace)
				return
			}
			if providerSpec.ID != "gemma-3-4b" {
				writeAIChatErrorWithTrace(w, http.StatusBadRequest, "RAG offline mode requires the local Gemma 3:4b chat provider", trace)
				return
			}
		}

		var provider aiChatProvider
		var err error
		if directExactResponse {
			recordAIChatTraceStage(ctx, aiChatTraceStage{
				ID: "provider", Label: "Skipped AI provider", Status: "skipped",
				Detail: "An exact lyrics query can be answered directly from Qdrant without an AI provider.",
			})
		} else {
			providerStarted := time.Now()
			provider, err = newAIChatProvider(providerSpec)
			if err != nil {
				recordAIChatTraceStage(ctx, aiChatTraceStage{
					ID: "provider", Label: "Selected AI provider", Status: "failed",
					DurationMS: time.Since(providerStarted).Milliseconds(), Error: err.Error(),
					Input: map[string]any{"provider": providerSpec.ID, "model": providerSpec.Model},
				})
				writeAIChatErrorWithTrace(w, http.StatusServiceUnavailable, err.Error(), trace)
				return
			}
			recordAIChatTraceStage(ctx, aiChatTraceStage{
				ID: "provider", Label: "Selected AI provider", Status: "completed",
				DurationMS: time.Since(providerStarted).Milliseconds(),
				Output:     map[string]any{"provider": providerSpec.ID, "model": providerSpec.Model},
			})
		}
		chatMessage := payload.Message
		var sources []rag.SongSearchResult
		ragError := ""
		if useRAG {
			chatMessage, sources, err = prepareAIChatMessageForResponse(ctx, payload.Message, payload.History, provider, searchRAG, n.ragChatFeatures())
			if err != nil {
				if directExactResponse {
					recordAIChatTraceStage(ctx, aiChatTraceStage{
						ID: "answer", Label: "Could not retrieve exact Qdrant lyrics matches", Status: "failed",
						Error: err.Error(),
					})
					writeAIChatErrorWithTrace(w, http.StatusServiceUnavailable, err.Error(), trace)
					return
				}
				ragError = err.Error()
				directExactResponse = false
				recordAIChatTraceStage(ctx, aiChatTraceStage{
					ID: "rag_fallback", Label: "Fell back to direct chat", Status: "fallback",
					Detail: "RAG preparation failed, so the original user message was sent without library context.", Error: err.Error(),
				})
				chatMessage = payload.Message
				sources = nil
			}
		} else {
			recordAIChatTraceStage(ctx, aiChatTraceStage{
				ID: "rag", Label: "Skipped RAG retrieval", Status: "skipped",
				Detail: "This request used normal chat, so no Qdrant search or library context was added.",
			})
		}
		answerStarted := time.Now()
		answer, directResponse, err := resolveAIChatAnswer(ctx, provider, chatMessage, exactNeedle, directExactResponse, sources)
		if err != nil {
			recordAIChatTraceStage(ctx, aiChatTraceStage{
				ID: "answer", Label: "Called AI for the final answer", Status: "failed",
				DurationMS: time.Since(answerStarted).Milliseconds(), Prompt: chatMessage, Error: err.Error(),
			})
			writeAIChatErrorWithTrace(w, http.StatusBadGateway, err.Error(), trace)
			return
		}
		if directResponse {
			recordAIChatTraceStage(ctx, aiChatTraceStage{
				ID: "answer", Label: "Returned exact Qdrant lyrics matches", Status: "completed",
				DurationMS: time.Since(answerStarted).Milliseconds(),
				Detail:     "The response was formatted deterministically without sending a final prompt to an AI model.",
				Response:   answer, Output: map[string]any{"characters": len([]rune(answer)), "songs": len(sources)},
			})
		} else {
			recordAIChatTraceStage(ctx, aiChatTraceStage{
				ID: "answer", Label: "Called AI for the final answer", Status: "completed",
				DurationMS: time.Since(answerStarted).Milliseconds(), Prompt: chatMessage, Response: answer,
				Output: map[string]any{"characters": len([]rune(answer))},
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(aiChatResponse{
			Response: answer,
			Provider: providerSpec.ID,
			Model:    providerSpec.Model,
			Sources:  sources,
			RAGError: ragError,
			Direct:   directResponse,
			Trace:    trace.snapshot(chatMessage, answer),
		})
	})

	r.Post("/ai/lyrics/fetch-job", n.handleLyricsJobStart)
	r.Get("/ai/lyrics/fetch-job/status", n.handleLyricsJobStatus)
	r.Delete("/ai/lyrics/fetch-job", n.handleLyricsJobStop)

	r.Post("/ai/songs/{id}/lyrics/fetch", func(w http.ResponseWriter, req *http.Request) {
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

		n.lyricsJobManager().BeginWhisperRequest()
		defer n.lyricsJobManager().EndWhisperRequest()
		lyrics, err := fetchAndSaveWhisperLyrics(req.Context(), n.ds.MediaFile(req.Context()), mf)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
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
		status := lyricsResultFailed
		if strings.TrimSpace(text) != "" {
			status = lyricsResultAvailable
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(aiLyricsResponse{
			Language: language, Text: text, Status: status,
		})
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
		var verify metadataVerifier
		if n.metadataJob != nil {
			// The AI tool page only fetches genre, so only the iTunes genre
			// lookup runs — no MusicBrainz album/year search or cover checks.
			verify = func(_ context.Context, title, artist string) (metadataResult, error) {
				genre, trace := n.metadataJob.fetchITunesGenreWithTrace(title, artist)
				return metadataResult{Genre: genre, GenreTrace: trace}, nil
			}
		}
		var spotify spotifyLookupFunc
		if n.spotifyJob != nil && spotifyConfigured() {
			spotify = func(ctx context.Context, mf model.MediaFile) (spotifyLookupResult, error) {
				return n.spotifyJob.lookupMetadata(ctx, mf)
			}
		}
		songs, err := fetchSongMetadata(req.Context(), n.ds.MediaFile(req.Context()), provider, verify, spotify, payload.SongIDs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		for index := range songs {
			if trace := songs[index].GenreDeveloperTrace; trace != nil && trace.AI != nil {
				trace.AI.Provider = providerSpec.ID
				trace.AI.Model = providerSpec.Model
			}
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

func resolveAIChatAnswer(
	ctx context.Context,
	provider aiChatProvider,
	prompt string,
	exactNeedle string,
	directExactResponse bool,
	sources []rag.SongSearchResult,
) (string, bool, error) {
	if directExactResponse {
		return rag.BuildExactLyricsResponse(exactNeedle, sources), true, nil
	}
	if provider == nil {
		return "", false, fmt.Errorf("AI provider is unavailable")
	}
	answer, err := provider.Chat(ctx, prompt)
	return answer, false, err
}

func writeAIChatError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}

func writeAIChatErrorWithTrace(w http.ResponseWriter, status int, message string, trace *aiChatTraceCollector) {
	if trace == nil {
		writeAIChatError(w, status, message)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message": message,
		"trace":   trace.snapshot("", ""),
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
	case "deepseek-v3.2", "deepseek.v3.2", "deepseek":
		return aiProviderSpec{ID: "deepseek-v3.2", Model: bedrockDeepSeekModel}
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
			client: &http.Client{Timeout: gemmaChatTimeout},
		}, nil
	case "deepseek-v3.2":
		bearerToken := bedrockBearerToken()
		if bearerToken == "" {
			return nil, fmt.Errorf("Amazon Bedrock bearer token is not configured")
		}
		return bedrockDeepSeekClient{
			apiURL:      bedrockDeepSeekAPIURL,
			bearerToken: bearerToken,
			model:       spec.Model,
			maxTokens:   deepSeekChatMaxTokens,
			client:      &http.Client{Timeout: gemmaChatTimeout},
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

func bedrockBearerToken() string {
	return strings.TrimSpace(conf.Server.AWSBearerTokenBedrock)
}

func gemma4APIURL() string {
	apiURL := strings.TrimSpace(os.Getenv("ND_GEMMA4APIURL"))
	if apiURL == "" {
		apiURL = strings.TrimSpace(conf.Server.Gemma4APIURL)
	}
	return apiURL
}

func getAIServiceStatuses(ctx context.Context, whisperBusy bool) []aiServiceStatus {
	client := &http.Client{Timeout: 4 * time.Second}
	gemmaURL, gemmaAPIKey := gemmaCredentials()
	gemma4URL := gemma4APIURL()
	whisperURL := strings.TrimSpace(conf.Server.WhisperAPIURL)
	geminiKey := geminiAPIKey()
	bedrockToken := bedrockBearerToken()

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
			probe: func() bool { return probeWhisperEndpoint(ctx, client, whisperURL) },
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
		{
			id:    "deepseek-v3.2",
			label: "DeepSeek V3.2",
			probe: func() bool { return probeBedrockDeepSeekModel(ctx, client, bedrockToken) },
		},
	}

	statuses := make([]aiServiceStatus, len(checks))
	var wg sync.WaitGroup
	for i := range checks {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			check := checks[index]
			statuses[index] = resolveAIServiceStatus(
				check.id,
				check.label,
				check.id == "whisper" && whisperBusy,
				check.probe,
			)
		}(i)
	}
	wg.Wait()
	return statuses
}

func resolveAIServiceStatus(id, label string, busy bool, probe func() bool) aiServiceStatus {
	if busy {
		return aiServiceStatus{ID: id, Label: label, Online: true, State: "busy"}
	}
	online := probe()
	state := "offline"
	if online {
		state = "online"
	}
	return aiServiceStatus{ID: id, Label: label, Online: online, State: state}
}

func probeAIEndpoint(ctx context.Context, client *http.Client, endpoint string, apiKey string) bool {
	return probeAIEndpointMethod(ctx, client, endpoint, apiKey, http.MethodHead)
}

func probeWhisperEndpoint(ctx context.Context, client *http.Client, endpoint string) bool {
	healthURL, err := url.Parse(strings.TrimSpace(endpoint))
	if err == nil && healthURL.Scheme != "" && healthURL.Host != "" {
		basePath := strings.TrimSuffix(healthURL.Path, "/")
		basePath = strings.TrimSuffix(basePath, "/transcribe")
		healthURL.Path = strings.TrimSuffix(basePath, "/") + "/health"
		healthURL.RawPath = ""
		healthURL.RawQuery = ""
		healthURL.Fragment = ""
		if probeAIEndpointMethod(ctx, client, healthURL.String(), "", http.MethodGet) {
			return true
		}
	}
	return probeAIEndpoint(ctx, client, endpoint, "")
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

func probeBedrockDeepSeekModel(ctx context.Context, client *http.Client, bearerToken string) bool {
	if strings.TrimSpace(bearerToken) == "" {
		return false
	}
	_, err := (bedrockDeepSeekClient{
		apiURL:      bedrockDeepSeekAPIURL,
		bearerToken: bearerToken,
		model:       bedrockDeepSeekModel,
		maxTokens:   1,
		client:      client,
	}).Chat(ctx, "Reply OK")
	return err == nil
}

type whisperResult struct {
	Language string
	Text     string
	Duration float64
}

// whisperTimeoutForDuration scales the request timeout with the song length so
// slow CPU transcription is allowed to finish, while still bounding the request.
func whisperTimeoutForDuration(duration float64) time.Duration {
	timeout := time.Duration(duration*6) * time.Second
	if timeout < 5*time.Minute {
		timeout = 5 * time.Minute
	}
	if timeout > 30*time.Minute {
		timeout = 30 * time.Minute
	}
	return timeout
}

// errWhisperRejectedTuning signals that the server rejected the request, most
// likely because of the optional tuning fields, so it is worth retrying with a
// minimal request.
var errWhisperRejectedTuning = errors.New("whisper rejected tuning fields")

func fetchWhisperLyrics(ctx context.Context, whisperURL string, audioPath string, whisperModel string, temperature float64) (whisperResult, error) {
	// First try with the optional Whisper tuning fields. If the server rejects
	// them (some minimal OpenAI-compatible servers 4xx on unknown fields), fall
	// back to a plain request so whole-song transcription still works.
	result, err := postWhisperTranscription(ctx, whisperURL, audioPath, whisperModel, temperature, true)
	if err != nil && errors.Is(err, errWhisperRejectedTuning) {
		log.Debug(ctx, "Whisper rejected tuning fields; retrying with a minimal request", "err", err)
		return postWhisperTranscription(ctx, whisperURL, audioPath, whisperModel, temperature, false)
	}
	return result, err
}

func postWhisperTranscription(ctx context.Context, whisperURL string, audioPath string, whisperModel string, temperature float64, withTuning bool) (whisperResult, error) {
	file, err := os.Open(audioPath)
	if err != nil {
		return whisperResult{}, fmt.Errorf("could not open audio file: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return whisperResult{}, fmt.Errorf("could not prepare audio upload: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return whisperResult{}, fmt.Errorf("could not read audio file: %w", err)
	}
	if strings.TrimSpace(whisperModel) != "" {
		if err := writer.WriteField("model", strings.TrimSpace(whisperModel)); err != nil {
			return whisperResult{}, fmt.Errorf("could not add Whisper model: %w", err)
		}
	}
	// verbose_json returns per-segment timestamps and total duration, which is
	// what makes coverage measurable. It is part of the standard OpenAI audio
	// API, so it is always sent.
	whisperFields := map[string]string{
		"response_format": "verbose_json",
		"temperature":     strconv.FormatFloat(temperature, 'f', -1, 64),
	}
	// These reduce Whisper's mid-song "collapse" but are faster-whisper
	// extensions, so they are only sent on the first attempt.
	if withTuning {
		whisperFields["vad_filter"] = "true"
		whisperFields["condition_on_previous_text"] = "false"
	}
	for field, value := range whisperFields {
		if err := writer.WriteField(field, value); err != nil {
			return whisperResult{}, fmt.Errorf("could not add Whisper field %q: %w", field, err)
		}
	}
	if err := writer.Close(); err != nil {
		return whisperResult{}, fmt.Errorf("could not finish audio upload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, whisperURL, &body)
	if err != nil {
		return whisperResult{}, fmt.Errorf("invalid Whisper API URL: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return whisperResult{}, fmt.Errorf("failed to contact Whisper API: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= http.StatusBadRequest {
		errBody, _ := io.ReadAll(httpResp.Body)
		message := strings.TrimSpace(string(errBody))
		if withTuning && httpResp.StatusCode < http.StatusInternalServerError {
			return whisperResult{}, fmt.Errorf("%w: HTTP %d: %s", errWhisperRejectedTuning, httpResp.StatusCode, message)
		}
		return whisperResult{}, fmt.Errorf("Whisper API error: %s", message)
	}

	var payload struct {
		Language string  `json:"language"`
		Text     string  `json:"text"`
		Duration float64 `json:"duration"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&payload); err != nil {
		return whisperResult{}, fmt.Errorf("invalid Whisper API response: %w", err)
	}

	result := whisperResult{
		Language: strings.TrimSpace(payload.Language),
		Text:     strings.TrimSpace(payload.Text),
		Duration: payload.Duration,
	}
	if result.Language == "" {
		result.Language = "xxx"
	}
	if result.Text == "" {
		return whisperResult{}, fmt.Errorf("Whisper API returned empty lyrics")
	}

	return result, nil
}

func saveWhisperLyricsFile(folder string, songID string, text string) error {
	path, err := whisperLyricsFilePath(folder, songID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("could not create lyrics folder: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".lyrics-*.tmp")
	if err != nil {
		return fmt.Errorf("could not create temporary lyrics file: %w", err)
	}
	tempPath := temp.Name()
	committed := false
	defer func() {
		_ = temp.Close()
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(0o644); err != nil {
		return fmt.Errorf("could not set lyrics file permissions: %w", err)
	}
	if _, err := temp.WriteString(strings.TrimSpace(text) + "\n"); err != nil {
		return fmt.Errorf("could not write lyrics file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("could not sync lyrics file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("could not close lyrics file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("could not publish lyrics file: %w", err)
	}
	committed = true
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
			if marker := explicitTitleMarker(mf); marker != "" {
				markerStatus := explicitStatusCode(marker)
				reason := "Derived from the explicit tag in the track's title or album because no lyrics were available."
				if markerStatus != "" && markerStatus != existingStatus {
					if err := repo.UpdateExplicitStatus(songID, markerStatus); err != nil {
						results = append(results, aiClassifyExplicitSong{
							ID: songID, ExplicitStatus: existingStatus,
							Reason: "A title/album explicit tag was found but could not be saved, so the existing status was preserved.",
						})
						continue
					}
				}
				if markerStatus != "" {
					results = append(results, aiClassifyExplicitSong{
						ID: songID, ExplicitStatus: markerStatus, Reason: reason,
					})
					continue
				}
			}
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
	result := parseExplicitClassificationDetailed(answer, lyrics)
	return applyDeterministicExplicitEvidence(result, lyrics, rules), nil
}

// applyDeterministicExplicitEvidence overrides the model's verdict to explicit
// when the lyrics contain uncensored or lightly obfuscated configured explicit
// words. This is a conservative, high-precision backstop: the configured words
// are strong profanity/slurs whose plain presence is essentially always
// explicit, so it fixes confident false-clean verdicts without adding model
// guesswork.
func applyDeterministicExplicitEvidence(result explicitClassificationResult, lyrics string, rules explicitWordRules) explicitClassificationResult {
	terms := newExplicitMatcher(rules).matches(lyrics)
	if len(terms) == 0 {
		return result
	}
	result.Classification = "explicit"
	if result.Confidence < minimumExplicitConfidence {
		result.Confidence = minimumExplicitConfidence
	}
	result.Evidence = dedupeExplicitEvidence(append(result.Evidence, terms...))
	result.Reason = "The lyrics contain explicit language: " + strings.Join(terms, ", ") + "."
	return result
}

func dedupeExplicitEvidence(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result
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

// Leetspeak substitutions and masking characters commonly used to obscure
// profanity in lyric transcripts (e.g. "f**k", "sh1t", "b!tch").
var explicitLeetVariants = map[rune]string{
	'a': "@4", 'b': "8", 'e': "3", 'g': "9", 'i': "1!",
	'l': "1", 'o': "0", 's': "5$", 't': "7",
}

// Characters used to censor interior letters. Kept literal inside a regex
// character class; '-' is placed last so it is not read as a range.
const explicitMaskChars = `*#@$%!.+_-`

// explicitMatcher deterministically finds configured explicit words in lyrics,
// both uncensored and lightly obfuscated. It is a backstop that does not depend
// on the model's own judgment, closing the "confident false clean" gap.
type explicitMatcher struct {
	plain      *regexp.Regexp
	obfuscated []*regexp.Regexp
}

func newExplicitMatcher(rules explicitWordRules) *explicitMatcher {
	m := &explicitMatcher{}
	plainAlternatives := make([]string, 0, len(rules.Included))
	for _, word := range rules.Included {
		word = strings.ToLower(strings.TrimSpace(word))
		if word == "" {
			continue
		}
		plainAlternatives = append(plainAlternatives, regexp.QuoteMeta(word))
		if pattern := obfuscatedWordPattern(word); pattern != "" {
			if re, err := regexp.Compile(pattern); err == nil {
				m.obfuscated = append(m.obfuscated, re)
			}
		}
	}
	if len(plainAlternatives) > 0 {
		m.plain = regexp.MustCompile(`(?i)\b(?:` + strings.Join(plainAlternatives, "|") + `)\b`)
	}
	return m
}

// obfuscatedWordPattern builds a regex that matches a censored spelling of word.
// The first and last letters must be present (leetspeak allowed); interior
// letters may be replaced by leetspeak or a masking character. Only ASCII words
// of length >= 3 are supported; anything else returns "" and relies on the
// plain matcher.
func obfuscatedWordPattern(word string) string {
	runes := []rune(word)
	if len(runes) < 3 {
		return ""
	}
	for _, r := range runes {
		if r < 'a' || r > 'z' {
			return ""
		}
	}
	var b strings.Builder
	b.WriteString(`(?i)\b`)
	for i, r := range runes {
		b.WriteByte('[')
		b.WriteRune(r)
		b.WriteString(explicitLeetVariants[r])
		if i != 0 && i != len(runes)-1 {
			b.WriteString(explicitMaskChars)
		}
		b.WriteByte(']')
	}
	b.WriteString(`\b`)
	return b.String()
}

// matches returns the distinct explicit terms found in the lyrics. Obfuscated
// hits are only reported when they actually contain a non-letter, so that plain
// spellings are attributed to the plain matcher and innocuous words never match.
func (m *explicitMatcher) matches(lyrics string) []string {
	if m == nil || strings.TrimSpace(lyrics) == "" {
		return nil
	}
	found := make([]string, 0, 4)
	seen := map[string]struct{}{}
	add := func(term string) {
		term = strings.TrimSpace(term)
		if term == "" {
			return
		}
		key := strings.ToLower(term)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		found = append(found, term)
	}
	if m.plain != nil {
		for _, hit := range m.plain.FindAllString(lyrics, -1) {
			add(hit)
		}
	}
	for _, re := range m.obfuscated {
		for _, hit := range re.FindAllString(lyrics, -1) {
			if containsNonLetter(hit) {
				add(hit)
			}
		}
	}
	return found
}

func containsNonLetter(value string) bool {
	for _, r := range value {
		if !unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

var (
	explicitTitleMarkerRegex = regexp.MustCompile(`(?i)[\[(]\s*explicit\s*[\])]`)
	cleanTitleMarkerRegex    = regexp.MustCompile(`(?i)[\[(]\s*clean\s*[\])]|\b(?:radio edit|clean version)\b`)
)

// explicitTitleMarker reads the distributor-supplied explicit/clean tag that is
// often embedded in a track's title or album (e.g. "Song [Explicit]"). These
// markers are highly reliable and used as a fallback when lyrics are missing.
func explicitTitleMarker(mf *model.MediaFile) string {
	haystack := mf.Title + " " + mf.Album
	if explicitTitleMarkerRegex.MatchString(haystack) {
		return "explicit"
	}
	if cleanTitleMarkerRegex.MatchString(haystack) {
		return "clean"
	}
	return ""
}

// metadataVerifier looks up an authoritative genre for a track (the iTunes
// Search API). It returns an empty result and no error when there is no
// confident match.
type metadataVerifier func(ctx context.Context, title, artist string) (metadataResult, error)

// spotifyLookupFunc looks up Spotify metadata (artist genres) for a track.
type spotifyLookupFunc func(ctx context.Context, mf model.MediaFile) (spotifyLookupResult, error)

func fetchSongMetadata(ctx context.Context, repo model.MediaFileRepository, provider aiChatProvider, verify metadataVerifier, spotify spotifyLookupFunc, songIDs []string) ([]aiFetchMetadataSong, error) {
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

		// 1. Spotify — artist-level genres.
		var sp spotifyLookupResult
		if spotify != nil {
			if r, spErr := spotify(ctx, *mf); spErr == nil {
				sp = r
			} else {
				log.Debug(ctx, "Spotify metadata lookup unavailable", "songId", songID, "err", spErr)
			}
		}

		// 2. iTunes — editorial per-track genre.
		var mb metadataResult
		if verify != nil {
			if r, mbErr := verify(ctx, mf.Title, mf.Artist); mbErr == nil {
				mb = r
			} else {
				log.Debug(ctx, "iTunes genre lookup unavailable", "songId", songID, "err", mbErr)
			}
		}

		// 3. AI — a dedicated per-song classification (never batched: one
		// request per song keeps the model focused on that recording) from the
		// song's own title, artist, album, and year.
		var aiGenre string
		var aiTrace *genreSourceDeveloperTrace
		if provider != nil {
			tracingProvider := &genreTracingProvider{provider: provider}
			aiGenre = fetchAIGenre(ctx, tracingProvider, mf)
			aiTrace = tracingProvider.trace(aiGenre)
		}

		breakdown := &aiMetadataConfidenceBreakdown{}

		// --- Genre: fetched from all three sources and shown per source; the
		// consensus among them drives the confidence score. ---
		_, genreConf, genreExpl := resolveGenreConsensus(sp.Genre, mb.Genre, aiGenre)
		result.SpotifyGenre = titleCaseGenreList(genreExpl.Spotify)
		result.MusicBrainzGenre = titleCaseGenreList(genreExpl.MusicBrainz)
		result.AIGenre = titleCaseGenreList(genreExpl.AI)
		result.GenreConfidence = genreConf
		breakdown.Genre = genreExpl
		genreTrace := &genreDeveloperTrace{}
		if result.MusicBrainzGenre != "" && mb.GenreTrace != nil {
			genreTrace.ITunes = mb.GenreTrace
		}
		if result.AIGenre != "" && aiTrace != nil {
			genreTrace.AI = aiTrace
		}
		if genreTrace.ITunes != nil || genreTrace.AI != nil {
			result.GenreDeveloperTrace = genreTrace
		}

		result.ConfidenceBreakdown = breakdown
		results = append(results, result)
	}

	return results, nil
}

func yearAsString(year int) string {
	if year <= 0 {
		return ""
	}
	return strconv.Itoa(year)
}

// spotifyConfigured reports whether Spotify credentials are available so the
// metadata lookup can authenticate.
func spotifyConfigured() bool {
	if strings.TrimSpace(conf.Server.Spotify.Token) != "" {
		return true
	}
	return strings.TrimSpace(conf.Server.Spotify.ID) != "" && strings.TrimSpace(conf.Server.Spotify.Secret) != ""
}

// resolveGenreConsensus combines the genre reported by Spotify, MusicBrainz, and
// the AI and prefers the genre that more than one source agrees on. Spotify's
// genre is artist-level and not always accurate, so a single source is only used
// when there is no agreement; in that case Spotify is trusted last.
func resolveGenreConsensus(spotify, musicBrainz, ai string) (string, int, aiMetadataFieldExplanation) {
	spotify = strings.TrimSpace(spotify)
	musicBrainz = strings.TrimSpace(musicBrainz)
	ai = strings.TrimSpace(ai)
	expl := aiMetadataFieldExplanation{Spotify: spotify, MusicBrainz: musicBrainz, AI: ai}

	type tokenInfo struct {
		display string
		sources map[string]bool
	}
	tokens := map[string]*tokenInfo{}
	order := make([]string, 0, 6)
	addTokens := func(source, genre string) {
		for _, raw := range strings.Split(genre, ",") {
			raw = strings.TrimSpace(raw)
			key := normalizeMBString(raw)
			if key == "" {
				continue
			}
			info, ok := tokens[key]
			if !ok {
				info = &tokenInfo{display: titleCaseGenre(raw), sources: map[string]bool{}}
				tokens[key] = info
				order = append(order, key)
			}
			info.sources[source] = true
		}
	}
	addTokens("spotify", spotify)
	addTokens("musicbrainz", musicBrainz)
	addTokens("ai", ai)

	// A genre token confirmed by two or more sources is the consensus.
	common := make([]string, 0, 2)
	for _, key := range order {
		if len(tokens[key].sources) >= 2 {
			common = append(common, tokens[key].display)
		}
	}
	if len(common) > 0 {
		if len(common) > 2 {
			common = common[:2]
		}
		expl.Source, expl.Confidence = "verified", metadataConfidenceVerified
		return strings.Join(common, ", "), metadataConfidenceVerified, expl
	}

	// No agreement: fall back to a single source, trusting Spotify's genre last.
	switch {
	case musicBrainz != "":
		expl.Source, expl.Confidence = "musicbrainz", metadataConfidenceAuthoritative
		return titleCaseGenreList(musicBrainz), metadataConfidenceAuthoritative, expl
	case ai != "":
		expl.Source, expl.Confidence = "ai-only", metadataConfidenceAIOnly
		return titleCaseGenreList(ai), metadataConfidenceAIOnly, expl
	case spotify != "":
		expl.Source, expl.Confidence = "spotify", metadataConfidenceSpotify
		return titleCaseGenreList(spotify), metadataConfidenceSpotify, expl
	default:
		expl.Source, expl.Confidence = "none", 0
		return "", 0, expl
	}
}

func titleCaseGenreList(genre string) string {
	parts := strings.Split(genre, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, titleCaseGenre(part))
		}
	}
	return strings.Join(out, ", ")
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
