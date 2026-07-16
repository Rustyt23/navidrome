package nativeapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

func TestAIChatDeveloperTraceCapturesRAGPipeline(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()
	conf.Server.EnableRAG = true
	conf.Server.RAGTopK = 5
	conf.Server.RAGMinScore = 0

	collector := newAIChatTraceCollector(true, "gemma-3-4b", "gemma3:4b", true)
	ctx := withAIChatTrace(context.Background(), collector)
	provider := staticAIChatProvider{answer: `{"mode":"search","query":"upbeat rock","filters":{"genre":"Rock"}}`}
	prompt, sources, err := prepareAIChatMessage(
		ctx,
		"show upbeat rock songs",
		nil,
		provider,
		func(_ context.Context, query string, topK int, filters rag.SearchFilters) ([]rag.SongSearchResult, error) {
			if query != "upbeat rock" || topK != 5 || filters.Genre != "Rock" {
				t.Fatalf("unexpected retrieval: query=%q topK=%d filters=%+v", query, topK, filters)
			}
			return []rag.SongSearchResult{{
				SongID: "song-1", Title: "Bright Song", Artist: "Band", Score: 0.92,
				LyricsText: "complete lyrics must not be serialized in source details",
			}}, nil
		},
	)
	if err != nil || len(sources) != 1 {
		t.Fatalf("prepare chat: sources=%+v err=%v", sources, err)
	}

	trace := collector.snapshot(prompt, "Try Bright Song.")
	if trace == nil || trace.RequestID == "" || trace.FinalPrompt != prompt || trace.RawResponse != "Try Bright Song." {
		t.Fatalf("unexpected trace summary: %+v", trace)
	}
	stageIDs := make(map[string]bool, len(trace.Stages))
	for _, stage := range trace.Stages {
		stageIDs[stage.ID] = true
	}
	for _, expected := range []string{"history", "query_plan", "retrieval", "score_filter", "prompt"} {
		if !stageIDs[expected] {
			t.Fatalf("expected trace stage %q, got %+v", expected, trace.Stages)
		}
	}
	encoded, err := json.Marshal(trace)
	if err != nil {
		t.Fatalf("marshal trace: %v", err)
	}
	text := string(encoded)
	if !strings.Contains(text, "Qdrant vector similarity search") || !strings.Contains(text, "Bright Song") {
		t.Fatalf("expected retrieval details in trace: %s", text)
	}
	if strings.Contains(text, "complete lyrics must not be serialized") {
		t.Fatalf("full source lyrics leaked into trace: %s", text)
	}
}

func TestAIChatDeveloperTraceRedactsConfiguredCredentials(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()
	conf.Server.GeminiAPIKey = "gemini-super-secret"
	conf.Server.AWSBearerTokenBedrock = "bedrock-super-secret"
	conf.Server.GemmaAPIKey = "gemma-super-secret"

	collector := newAIChatTraceCollector(true, "gemini-2.5", "gemini-2.5-flash", false)
	collector.add(aiChatTraceStage{
		ID:       "answer",
		Label:    "Called AI",
		Status:   "failed",
		Prompt:   "key=gemini-super-secret authorization: Bearer token-value",
		Response: "gemma-super-secret bedrock-super-secret",
		Input: map[string]any{
			"message": "token=token-value",
			"apiKey":  "an-unconfigured-secret",
		},
	})
	encoded, err := json.Marshal(collector.snapshot("gemini-super-secret", "gemma-super-secret"))
	if err != nil {
		t.Fatalf("marshal trace: %v", err)
	}
	text := string(encoded)
	if strings.Contains(text, "super-secret") || strings.Contains(text, "token-value") || strings.Contains(text, "an-unconfigured-secret") {
		t.Fatalf("credential leaked into trace: %s", text)
	}
	if !strings.Contains(text, "[REDACTED]") {
		t.Fatalf("expected redaction marker: %s", text)
	}
}

func TestAIChatDeveloperTraceIsOptIn(t *testing.T) {
	if trace := newAIChatTraceCollector(false, "gemma-3-4b", "gemma3:4b", true); trace != nil {
		t.Fatalf("expected disabled trace collector to be nil, got %+v", trace)
	}
}
