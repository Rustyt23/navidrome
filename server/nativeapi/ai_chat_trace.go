package nativeapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/navidrome/navidrome/conf"
)

// aiChatTrace is an opt-in, application-level account of one chat request. It
// intentionally reports only work performed by Navidrome; provider-internal
// reasoning is neither available nor represented here.
type aiChatTrace struct {
	RequestID   string             `json:"requestId"`
	StartedAt   string             `json:"startedAt"`
	DurationMS  int64              `json:"durationMs"`
	Provider    string             `json:"provider"`
	Model       string             `json:"model"`
	UseRAG      bool               `json:"useRag"`
	FinalPrompt string             `json:"finalPrompt,omitempty"`
	RawResponse string             `json:"rawResponse,omitempty"`
	Stages      []aiChatTraceStage `json:"stages"`
}

type aiChatTraceStage struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Status     string `json:"status"`
	DurationMS int64  `json:"durationMs,omitempty"`
	Detail     string `json:"detail,omitempty"`
	Prompt     string `json:"prompt,omitempty"`
	Response   string `json:"response,omitempty"`
	Error      string `json:"error,omitempty"`
	Input      any    `json:"input,omitempty"`
	Output     any    `json:"output,omitempty"`
}

type aiChatTraceContextKey struct{}

type aiChatTraceCollector struct {
	mu        sync.Mutex
	startedAt time.Time
	trace     aiChatTrace
}

func newAIChatTraceCollector(enabled bool, provider, model string, useRAG bool) *aiChatTraceCollector {
	if !enabled {
		return nil
	}
	startedAt := time.Now()
	return &aiChatTraceCollector{
		startedAt: startedAt,
		trace: aiChatTrace{
			RequestID:  newAIChatTraceRequestID(),
			StartedAt:  startedAt.UTC().Format(time.RFC3339Nano),
			Provider:   provider,
			Model:      model,
			UseRAG:     useRAG,
			Stages:     make([]aiChatTraceStage, 0, 8),
			DurationMS: 0,
		},
	}
}

func withAIChatTrace(ctx context.Context, collector *aiChatTraceCollector) context.Context {
	if collector == nil {
		return ctx
	}
	return context.WithValue(ctx, aiChatTraceContextKey{}, collector)
}

func recordAIChatTraceStage(ctx context.Context, stage aiChatTraceStage) {
	collector, _ := ctx.Value(aiChatTraceContextKey{}).(*aiChatTraceCollector)
	if collector == nil {
		return
	}
	collector.add(stage)
}

func (c *aiChatTraceCollector) add(stage aiChatTraceStage) {
	if c == nil {
		return
	}
	stage.Detail = redactAIChatTraceText(stage.Detail)
	stage.Prompt = redactAIChatTraceText(stage.Prompt)
	stage.Response = redactAIChatTraceText(stage.Response)
	stage.Error = redactAIChatTraceText(stage.Error)
	stage.Input = redactAIChatTraceValue(stage.Input)
	stage.Output = redactAIChatTraceValue(stage.Output)
	c.mu.Lock()
	c.trace.Stages = append(c.trace.Stages, stage)
	c.mu.Unlock()
}

func redactAIChatTraceValue(value any) any {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "[unavailable]"
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return "[unavailable]"
	}
	return redactAIChatTraceJSON(decoded)
}

func redactAIChatTraceJSON(value any) any {
	switch typed := value.(type) {
	case string:
		return redactAIChatTraceText(typed)
	case []any:
		for index := range typed {
			typed[index] = redactAIChatTraceJSON(typed[index])
		}
		return typed
	case map[string]any:
		for key := range typed {
			if isAIChatTraceSensitiveKey(key) {
				typed[key] = "[REDACTED]"
				continue
			}
			typed[key] = redactAIChatTraceJSON(typed[key])
		}
		return typed
	default:
		return value
	}
}

func isAIChatTraceSensitiveKey(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(key))
	switch normalized {
	case "authorization", "apikey", "token", "password", "secret":
		return true
	default:
		return false
	}
}

func (c *aiChatTraceCollector) snapshot(finalPrompt, rawResponse string) *aiChatTrace {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.trace.DurationMS = time.Since(c.startedAt).Milliseconds()
	c.trace.FinalPrompt = redactAIChatTraceText(finalPrompt)
	c.trace.RawResponse = redactAIChatTraceText(rawResponse)
	copyOfTrace := c.trace
	copyOfTrace.Stages = append([]aiChatTraceStage(nil), c.trace.Stages...)
	return &copyOfTrace
}

func newAIChatTraceRequestID() string {
	value := make([]byte, 8)
	if _, err := rand.Read(value); err == nil {
		return hex.EncodeToString(value)
	}
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

var aiChatTraceCredentialPattern = regexp.MustCompile(`(?i)(["']?(?:authorization|api[_-]?key|token|password|secret)["']?\s*[:=]\s*["']?(?:bearer\s+)?)[^\s,;"'}]+`)

func redactAIChatTraceText(value string) string {
	if value == "" {
		return ""
	}
	secrets := []string{
		strings.TrimSpace(conf.Server.GeminiAPIKey),
		strings.TrimSpace(conf.Server.AWSBearerTokenBedrock),
		strings.TrimSpace(conf.Server.GemmaAPIKey),
		strings.TrimSpace(os.Getenv("ND_GEMMA_API_KEY")),
	}
	for _, secret := range secrets {
		if len(secret) >= 4 {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return aiChatTraceCredentialPattern.ReplaceAllString(value, "${1}[REDACTED]")
}
