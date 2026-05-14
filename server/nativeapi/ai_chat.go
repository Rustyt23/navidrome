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
)

type aiChatRequest struct {
	Message string `json:"message"`
}

type aiChatResponse struct {
	Answer string `json:"answer"`
}

type openAIResponsesRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type openAIResponsesResponse struct {
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

func (n *Router) addAIChatRoute(r chi.Router) {
	r.Post("/ai/chat", func(w http.ResponseWriter, req *http.Request) {
		apiKey := strings.TrimSpace(conf.Server.AI.OpenAIAPIKey)
		if apiKey == "" {
			http.Error(w, "AI API key is not configured", http.StatusServiceUnavailable)
			return
		}

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

		model := strings.TrimSpace(conf.Server.AI.OpenAIModel)
		if model == "" {
			model = "gpt-4.1-mini"
		}

		body, _ := json.Marshal(openAIResponsesRequest{Model: model, Input: payload.Message})
		httpReq, _ := http.NewRequestWithContext(req.Context(), http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewBuffer(body))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)

		httpResp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			http.Error(w, "failed to contact AI provider", http.StatusBadGateway)
			return
		}
		defer httpResp.Body.Close()

		if httpResp.StatusCode >= http.StatusBadRequest {
			errBody, _ := io.ReadAll(httpResp.Body)
			http.Error(w, fmt.Sprintf("AI provider error: %s", strings.TrimSpace(string(errBody))), http.StatusBadGateway)
			return
		}

		var apiResp openAIResponsesResponse
		if err := json.NewDecoder(httpResp.Body).Decode(&apiResp); err != nil {
			http.Error(w, "invalid AI provider response", http.StatusBadGateway)
			return
		}

		answer := ""
		for _, out := range apiResp.Output {
			for _, c := range out.Content {
				if c.Text != "" {
					if answer != "" {
						answer += "\n"
					}
					answer += c.Text
				}
			}
		}

		if answer == "" {
			answer = "No response returned from AI provider."
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(aiChatResponse{Answer: answer})
	})
}
