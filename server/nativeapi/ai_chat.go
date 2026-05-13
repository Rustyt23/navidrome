package nativeapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
)

type aiChatRequest struct {
	Message string `json:"message"`
}

type aiChatResponse struct {
	Answer string `json:"answer"`
}

type openAIChatRequest struct {
	Model    string              `json:"model"`
	Messages []map[string]string `json:"messages"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (n *Router) addAIRoute(r chi.Router) {
	r.Post("/ai/chat", n.handleAIChat())
}

func (n *Router) handleAIChat() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(conf.Server.AI.OpenAIAPIKey) == "" {
			http.Error(w, "AI is not configured", http.StatusServiceUnavailable)
			return
		}

		var req aiChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
		message := strings.TrimSpace(req.Message)
		if message == "" {
			http.Error(w, "Message is required", http.StatusBadRequest)
			return
		}

		body, _ := json.Marshal(openAIChatRequest{
			Model:    conf.Server.AI.OpenAIModel,
			Messages: []map[string]string{{"role": "user", "content": message}},
		})
		openAIReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
		if err != nil {
			http.Error(w, "Could not create AI request", http.StatusInternalServerError)
			return
		}
		openAIReq.Header.Set("Content-Type", "application/json")
		openAIReq.Header.Set("Authorization", "Bearer "+conf.Server.AI.OpenAIAPIKey)

		res, err := http.DefaultClient.Do(openAIReq)
		if err != nil {
			log.Error(r.Context(), "Error calling OpenAI", "err", err)
			http.Error(w, "Unable to process AI request", http.StatusBadGateway)
			return
		}
		defer res.Body.Close()

		if res.StatusCode >= 300 {
			data, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
			log.Warn(r.Context(), "OpenAI returned error", "status", res.StatusCode, "body", string(data))
			http.Error(w, "AI provider returned an error", http.StatusBadGateway)
			return
		}

		var openAIRes openAIChatResponse
		if err := json.NewDecoder(res.Body).Decode(&openAIRes); err != nil {
			http.Error(w, "Invalid AI response", http.StatusBadGateway)
			return
		}
		if len(openAIRes.Choices) == 0 {
			http.Error(w, "AI returned an empty response", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(aiChatResponse{Answer: strings.TrimSpace(openAIRes.Choices[0].Message.Content)})
	}
}
