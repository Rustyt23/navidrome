package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

type ragSearchRequest struct {
	Query   string            `json:"query"`
	TopK    int               `json:"topK"`
	Filters rag.SearchFilters `json:"filters,omitempty"`
}

type ragSearchResponse struct {
	Results        []rag.SongSearchResult `json:"results"`
	AppliedFilters rag.SearchFilters      `json:"appliedFilters"`
	Count          int                    `json:"count"`
}

type ragSearchFunc func(context.Context, string, int, rag.SearchFilters) ([]rag.SongSearchResult, error)

func (n *Router) handleRAGSearch(w http.ResponseWriter, request *http.Request) {
	serveRAGSearch(w, request, searchRAG)
}

func serveRAGSearch(w http.ResponseWriter, request *http.Request, search ragSearchFunc) {
	if !ragEnabled() {
		writeRAGSearchError(w, http.StatusServiceUnavailable, "RAG is disabled")
		return
	}
	payload, err := decodeRAGSearchRequest(request.Body, conf.Server.RAGTopK)
	if err != nil {
		writeRAGSearchError(w, http.StatusBadRequest, err.Error())
		return
	}

	results, err := search(request.Context(), payload.Query, payload.TopK, payload.Filters)
	if err != nil {
		writeRAGSearchError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if results == nil {
		results = []rag.SongSearchResult{}
	}
	_ = json.NewEncoder(w).Encode(ragSearchResponse{
		Results: results, AppliedFilters: payload.Filters, Count: len(results),
	})
}

// ragEmbeddingConfigured reports whether an embedding backend is available:
// either a local Gemma (EmbeddingGemma) endpoint or a Gemini API key.
func ragEmbeddingConfigured() bool {
	if conf.Server.RAGOffline {
		return strings.TrimSpace(conf.Server.RAGEmbeddingURL) != ""
	}
	return strings.TrimSpace(conf.Server.RAGEmbeddingURL) != "" ||
		strings.TrimSpace(conf.Server.GeminiAPIKey) != ""
}

func ragEmbeddingStatus() (backend, model string, local bool) {
	if strings.TrimSpace(conf.Server.RAGEmbeddingURL) != "" {
		model = strings.TrimSpace(conf.Server.RAGEmbeddingModel)
		if model == "" {
			model = "embeddinggemma"
		}
		return "ollama", model, true
	}
	if strings.TrimSpace(conf.Server.GeminiAPIKey) != "" {
		return "gemini", "gemini-embedding-001", false
	}
	return "unconfigured", "", false
}

// ragDocumentEmbedder returns the embedder used to index documents. It prefers a
// local Gemma model when RAGEmbeddingURL is set, and falls back to Gemini.
func ragDocumentEmbedder() rag.Embedder {
	if url := strings.TrimSpace(conf.Server.RAGEmbeddingURL); url != "" {
		return rag.NewGemmaEmbedder(url, conf.Server.RAGEmbeddingModel, ragEmbeddingHTTPOptions())
	}
	return rag.NewGeminiEmbedder(conf.Server.GeminiAPIKey, ragEmbeddingHTTPOptions())
}

// ragQueryEmbedder returns the embedder used to embed search queries in the same
// vector space as the indexed documents.
func ragQueryEmbedder() rag.Embedder {
	if url := strings.TrimSpace(conf.Server.RAGEmbeddingURL); url != "" {
		return rag.NewGemmaEmbedder(url, conf.Server.RAGEmbeddingModel, ragEmbeddingHTTPOptions())
	}
	return rag.NewGeminiQueryEmbedder(conf.Server.GeminiAPIKey, ragEmbeddingHTTPOptions())
}

func ragEmbeddingHTTPOptions() rag.HTTPClientOptions {
	return rag.HTTPClientOptions{
		Timeout:      conf.Server.RAGEmbeddingTimeout,
		MaxRetries:   conf.Server.RAGRetryMax,
		RetryBackoff: conf.Server.RAGRetryBackoff,
	}
}

func newRAGQdrantClient() *rag.QdrantClient {
	schema := rag.ExpectedIndexSchema(ragEmbedderTag())
	return rag.NewQdrantClient(conf.Server.RAGVectorURL, conf.Server.RAGCollection, rag.QdrantClientOptions{
		HTTPClientOptions: rag.HTTPClientOptions{
			Timeout:      conf.Server.RAGQdrantTimeout,
			MaxRetries:   conf.Server.RAGRetryMax,
			RetryBackoff: conf.Server.RAGRetryBackoff,
		},
		ExpectedSchema: &schema,
	})
}

// ragEmbedderTag identifies the embedding backend so a change of model forces a
// re-index (vectors from different models are not comparable). It is stored in
// each point's content hash.
func ragEmbedderTag() string {
	if url := strings.TrimSpace(conf.Server.RAGEmbeddingURL); url != "" {
		model := strings.TrimSpace(conf.Server.RAGEmbeddingModel)
		if model == "" {
			model = "embeddinggemma"
		}
		return "gemma:" + model
	}
	return "gemini:gemini-embedding-001"
}

func searchRAG(ctx context.Context, query string, topK int, filters rag.SearchFilters) ([]rag.SongSearchResult, error) {
	started := time.Now()
	if !ragEnabled() {
		return nil, fmt.Errorf("RAG is disabled")
	}
	exactLyricsSearch := strings.TrimSpace(filters.LyricsContains) != ""
	if !exactLyricsSearch && !ragEmbeddingConfigured() {
		return nil, fmt.Errorf("no embedding backend is configured (set RAGEmbeddingURL or GeminiAPIKey)")
	}

	qdrant := newRAGQdrantClient()
	status := qdrant.Status(ctx, false)
	if !status.VectorDBOnline {
		if status.Error != "" {
			return nil, errors.New(status.Error)
		}
		return nil, fmt.Errorf("Qdrant is offline")
	}
	if !status.CollectionExists {
		return nil, fmt.Errorf("Qdrant collection %q does not exist", conf.Server.RAGCollection)
	}
	if status.Error != "" {
		return nil, errors.New(status.Error)
	}
	if exactLyricsSearch {
		results, err := qdrant.SearchSongsByPayload(ctx, topK, filters)
		if err != nil {
			return nil, err
		}
		log.Debug(ctx, "RAG exact lyrics retrieval completed",
			"duration", time.Since(started),
			"results", len(results),
			"lyricsContains", filters.LyricsContains,
			"topK", topK,
		)
		return results, nil
	}

	results, err := rag.SearchSongs(
		ctx,
		ragQueryEmbedder(),
		qdrant,
		query,
		topK,
		filters,
	)
	if err != nil {
		return nil, err
	}
	results = rag.FilterByMinScore(results, conf.Server.RAGMinScore)
	rag.ObserveRetrievalQuality(results)
	topScore := 0.0
	if len(results) > 0 {
		topScore = results[0].Score
	}
	log.Debug(ctx, "RAG retrieval completed",
		"duration", time.Since(started),
		"results", len(results),
		"topScore", topScore,
		"topK", topK,
	)
	return results, nil
}

func decodeRAGSearchRequest(reader io.Reader, defaultTopK int) (ragSearchRequest, error) {
	payload := ragSearchRequest{TopK: defaultTopK}
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return ragSearchRequest{}, fmt.Errorf("invalid request payload: %w", err)
	}
	payload.Query = strings.TrimSpace(payload.Query)
	if payload.Query == "" {
		return ragSearchRequest{}, fmt.Errorf("query is required")
	}
	if payload.TopK <= 0 || payload.TopK > rag.MaxSearchTopK {
		return ragSearchRequest{}, fmt.Errorf("topK must be between 1 and %d", rag.MaxSearchTopK)
	}
	normalizedFilters, err := rag.NormalizeSearchFilters(payload.Filters)
	if err != nil {
		return ragSearchRequest{}, err
	}
	payload.Filters = normalizedFilters
	return payload, nil
}

func writeRAGSearchError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func prepareAIChatMessage(
	ctx context.Context,
	message string,
	history []aiChatTurn,
	provider aiChatProvider,
	search ragSearchFunc,
) (string, []rag.SongSearchResult, error) {
	return prepareAIChatMessageWithFeatures(ctx, message, history, provider, search, ragChatFeatures{})
}

type ragChatFeatures struct {
	analytics  func(context.Context, rag.SearchFilters) (string, error)
	duplicates func(context.Context) (string, error)
}

func prepareAIChatMessageWithFeatures(
	ctx context.Context,
	message string,
	history []aiChatTurn,
	provider aiChatProvider,
	search ragSearchFunc,
	features ragChatFeatures,
) (string, []rag.SongSearchResult, error) {
	return prepareAIChatMessageWithFeaturesMode(ctx, message, history, provider, search, features, false)
}

// prepareAIChatMessageForResponse enables the exact-lyrics fast path used by
// the chat endpoint. Existing preparation callers still receive a prompt so
// their behavior remains backward compatible.
func prepareAIChatMessageForResponse(
	ctx context.Context,
	message string,
	history []aiChatTurn,
	provider aiChatProvider,
	search ragSearchFunc,
	features ragChatFeatures,
) (string, []rag.SongSearchResult, error) {
	return prepareAIChatMessageWithFeaturesMode(ctx, message, history, provider, search, features, true)
}

func prepareAIChatMessageWithFeaturesMode(
	ctx context.Context,
	message string,
	history []aiChatTurn,
	provider aiChatProvider,
	search ragSearchFunc,
	features ragChatFeatures,
	directExactResponse bool,
) (string, []rag.SongSearchResult, error) {
	if !ragEnabled() {
		recordAIChatTraceStage(ctx, aiChatTraceStage{
			ID: "rag", Label: "Skipped RAG retrieval", Status: "skipped",
			Detail: "RAG is disabled in the server configuration.",
		})
		return message, nil, nil
	}

	originalHistoryCount := len(history)
	history = normalizeAIChatHistory(history)
	recordAIChatTraceStage(ctx, aiChatTraceStage{
		ID: "history", Label: "Normalized conversation history", Status: "completed",
		Detail: "Only recent user and assistant turns are allowed into RAG prompts.",
		Input:  map[string]any{"turns": originalHistoryCount},
		Output: map[string]any{"turns": len(history), "history": history},
	})

	// For a follow-up ("only clean ones", "more like that"), condense the turn into a
	// standalone query so retrieval isn't tripped up by pronouns and ellipsis.
	retrievalQuery := message
	_, originalIsExactLyrics := directLyricsContains(message)
	if len(history) > 0 && provider != nil && !(directExactResponse && originalIsExactLyrics) {
		retrievalQuery = rewriteFollowUpQuery(ctx, provider, history, message)
	}

	plan := ragQueryPlan{}
	strictLyricsMatch := false
	if exactLyrics, ok := directLyricsContains(retrievalQuery); ok {
		strictLyricsMatch = true
		plan = ragQueryPlan{
			Mode:  ragQueryModeLyrics,
			Query: exactLyrics,
			Filters: rag.SearchFilters{
				LyricsContains: exactLyrics,
			},
		}
		recordAIChatTraceStage(ctx, aiChatTraceStage{
			ID: "query_plan", Label: "Detected an exact lyrics request", Status: "completed",
			Detail: "The phrase was extracted deterministically, so no planning-model call was needed.",
			Input:  map[string]any{"message": retrievalQuery},
			Output: plan,
		})
	} else {
		var filterErr error
		plan, filterErr = extractRAGQueryPlan(ctx, provider, retrievalQuery)
		if filterErr != nil {
			log.Debug(ctx, "RAG structured filter extraction failed; continuing without hard filters", "err", filterErr)
			plan = ragQueryPlan{Mode: ragQueryModeSearch}
			recordAIChatTraceStage(ctx, aiChatTraceStage{
				ID: "query_plan_fallback", Label: "Continued without structured filters", Status: "fallback",
				Detail: "The query-planning call failed, so retrieval continued using the user query.", Error: filterErr.Error(),
				Output: plan,
			})
		}
	}
	if plan.Query != "" {
		retrievalQuery = plan.Query
	}
	filters := plan.Filters
	if plan.Mode == ragQueryModeAnalytics && features.analytics != nil {
		started := time.Now()
		data, err := features.analytics(ctx, filters)
		if err != nil {
			recordAIChatTraceStage(ctx, aiChatTraceStage{
				ID: "analytics", Label: "Computed library analytics", Status: "failed",
				DurationMS: time.Since(started).Milliseconds(), Input: filters, Error: err.Error(),
			})
			return message, nil, err
		}
		prompt := rag.BuildLibraryDataPrompt(message, toRAGHistory(history), "library analytics", data)
		recordAIChatTraceStage(ctx, aiChatTraceStage{
			ID: "analytics", Label: "Computed library analytics", Status: "completed",
			DurationMS: time.Since(started).Milliseconds(), Input: filters,
			Output: map[string]any{"characters": len([]rune(data))},
		})
		recordAIChatTraceStage(ctx, aiChatTraceStage{
			ID: "prompt", Label: "Built the final answer prompt", Status: "completed",
			Detail: "The computed analytics and conversation were assembled for the final AI call.", Prompt: prompt,
		})
		return prompt, nil, nil
	}
	if plan.Mode == ragQueryModeDuplicates && features.duplicates != nil {
		started := time.Now()
		data, err := features.duplicates(ctx)
		if err != nil {
			recordAIChatTraceStage(ctx, aiChatTraceStage{
				ID: "duplicates", Label: "Computed duplicate candidates", Status: "failed",
				DurationMS: time.Since(started).Milliseconds(), Error: err.Error(),
			})
			return message, nil, err
		}
		prompt := rag.BuildLibraryDataPrompt(message, toRAGHistory(history), "duplicate and alternate-version candidates", data)
		recordAIChatTraceStage(ctx, aiChatTraceStage{
			ID: "duplicates", Label: "Computed duplicate candidates", Status: "completed",
			DurationMS: time.Since(started).Milliseconds(),
			Output:     map[string]any{"characters": len([]rune(data))},
		})
		recordAIChatTraceStage(ctx, aiChatTraceStage{
			ID: "prompt", Label: "Built the final answer prompt", Status: "completed",
			Detail: "Duplicate candidates and conversation were assembled for the final AI call.", Prompt: prompt,
		})
		return prompt, nil, nil
	}
	if plan.Mode == ragQueryModeLyrics {
		required := true
		filters.HasLyrics = &required
	} else {
		// Exact lyric matching is only meaningful for lyric-identification requests.
		filters.LyricsContains = ""
	}
	retrievalStarted := time.Now()
	retrievalTopK := conf.Server.RAGTopK
	if strictLyricsMatch && directExactResponse {
		retrievalTopK = rag.MaxSearchTopK
	}
	strategy := "Qdrant vector similarity search"
	if filters.LyricsContains != "" {
		strategy = "Qdrant lyrics full-text search"
	}
	results, err := search(ctx, retrievalQuery, retrievalTopK, filters)
	if err != nil {
		recordAIChatTraceStage(ctx, aiChatTraceStage{
			ID: "retrieval", Label: "Searched the indexed music library", Status: "failed",
			DurationMS: time.Since(retrievalStarted).Milliseconds(), Detail: strategy,
			Input: map[string]any{"query": retrievalQuery, "topK": retrievalTopK, "filters": filters}, Error: err.Error(),
		})
		return message, nil, err
	}
	rawResultCount := len(results)
	if strictLyricsMatch {
		results = rag.FilterExactLyricMatches(retrievalQuery, results)
	}
	exactResultCount := len(results)
	results = rag.DeduplicateSongResults(results)
	recordAIChatTraceStage(ctx, aiChatTraceStage{
		ID: "retrieval", Label: "Searched the indexed music library", Status: "completed",
		DurationMS: time.Since(retrievalStarted).Milliseconds(), Detail: strategy,
		Input: map[string]any{"query": retrievalQuery, "topK": retrievalTopK, "filters": filters},
		Output: map[string]any{
			"rawCount": rawResultCount, "exactCount": exactResultCount,
			"count": len(results), "results": results,
		},
	})
	recordAIChatTraceStage(ctx, aiChatTraceStage{
		ID: "deduplicate", Label: "Removed duplicate song matches", Status: "completed",
		Input: map[string]any{"count": exactResultCount},
		Output: map[string]any{
			"count": len(results), "removed": exactResultCount - len(results),
		},
	})
	// The exact lyric filter can be too strict (word variations, transcription
	// differences); fall back to semantic-only retrieval rather than answering
	// "nothing found" from an over-constrained search.
	if len(results) == 0 && filters.LyricsContains != "" && !strictLyricsMatch {
		relaxed := filters
		relaxed.LyricsContains = ""
		retryStarted := time.Now()
		if retried, retryErr := search(ctx, retrievalQuery, conf.Server.RAGTopK, relaxed); retryErr == nil {
			rawRetryCount := len(retried)
			results = rag.DeduplicateSongResults(retried)
			filters = relaxed
			recordAIChatTraceStage(ctx, aiChatTraceStage{
				ID: "retrieval_retry", Label: "Retried with semantic lyrics search", Status: "completed",
				DurationMS: time.Since(retryStarted).Milliseconds(),
				Detail:     "The exact lyric filter returned no results, so it was removed for one semantic retry.",
				Input:      map[string]any{"query": retrievalQuery, "topK": conf.Server.RAGTopK, "filters": relaxed},
				Output: map[string]any{
					"rawCount": rawRetryCount, "count": len(results), "results": results,
				},
			})
		} else if retryErr != nil {
			recordAIChatTraceStage(ctx, aiChatTraceStage{
				ID: "retrieval_retry", Label: "Retried with semantic lyrics search", Status: "failed",
				DurationMS: time.Since(retryStarted).Milliseconds(), Input: relaxed, Error: retryErr.Error(),
			})
		}
	}
	beforeMinScore := len(results)
	results = rag.FilterByMinScore(results, conf.Server.RAGMinScore)
	recordAIChatTraceStage(ctx, aiChatTraceStage{
		ID: "score_filter", Label: "Applied the minimum relevance score", Status: "completed",
		Input:  map[string]any{"count": beforeMinScore, "minimumScore": conf.Server.RAGMinScore},
		Output: map[string]any{"count": len(results)},
	})
	if plan.Mode == ragQueryModeLyrics {
		snippetNeedle := plan.Filters.LyricsContains
		if snippetNeedle == "" {
			snippetNeedle = retrievalQuery
		}
		results = rag.AddLyricSnippets(snippetNeedle, results)
		recordAIChatTraceStage(ctx, aiChatTraceStage{
			ID: "lyric_snippets", Label: "Selected matching lyric snippets", Status: "completed",
			Detail: "Only short matching snippets, not complete lyrics, are added to the final AI prompt.",
			Input:  map[string]any{"needle": snippetNeedle, "songs": len(results)}, Output: results,
		})
	}
	if strictLyricsMatch && directExactResponse {
		recordAIChatTraceStage(ctx, aiChatTraceStage{
			ID: "direct_response", Label: "Prepared an exact lyrics response", Status: "completed",
			Detail: "Exact Qdrant matches will be formatted directly; no answer prompt will be sent to an AI model.",
			Output: map[string]any{"songs": len(results)},
		})
		return "", results, nil
	}
	prompt := rag.BuildChatPromptWithHistory(message, toRAGHistory(history), results, filters)
	recordAIChatTraceStage(ctx, aiChatTraceStage{
		ID: "prompt", Label: "Built the final answer prompt", Status: "completed",
		Detail: "The user question, applied filters, conversation history, and retrieved sources were assembled for the final AI call.",
		Prompt: prompt, Output: map[string]any{"sources": len(results)},
	})
	return prompt, results, nil
}

var (
	quotedLyricsPattern = regexp.MustCompile(`["“‘']([^"”’']{1,200})["”’']`)
	wordLyricsPattern   = regexp.MustCompile(`(?i)\b(?:word|phrase)s?\s+["“‘']?([\p{L}\p{N}_-]+(?:\s+[\p{L}\p{N}_-]+){0,7})`)
	lyricsVerbPattern   = regexp.MustCompile(`(?i)\blyrics?\s+(?:contain|contains|containing|include|includes|including)\s+(?:the\s+)?(?:(?:word|phrase)s?\s+)?["“‘']?([\p{L}\p{N}_-]+(?:\s+[\p{L}\p{N}_-]+){0,7})`)
)

// directLyricsContains recognizes unambiguous lyric word/phrase requests
// without asking the chat model to plan the query. This makes requests such as
// "songs with the word vikash" deterministic and sends "vikash" directly to
// Qdrant's lyrics full-text index.
func directLyricsContains(message string) (string, bool) {
	lower := strings.ToLower(message)
	hasLyricsIntent := strings.Contains(lower, "lyric") ||
		strings.Contains(lower, "word") || strings.Contains(lower, "phrase") ||
		strings.Contains(lower, "contain") || strings.Contains(lower, "include")
	if !hasLyricsIntent {
		return "", false
	}
	for _, pattern := range []*regexp.Regexp{quotedLyricsPattern, wordLyricsPattern, lyricsVerbPattern} {
		match := pattern.FindStringSubmatch(message)
		if len(match) < 2 {
			continue
		}
		value := cleanDirectLyricsMatch(match[1])
		if value != "" && len([]rune(value)) <= 200 {
			return value, true
		}
	}
	return "", false
}

func cleanDirectLyricsMatch(value string) string {
	value = strings.Trim(strings.TrimSpace(value), `"'“”‘’.,!?`)
	lower := strings.ToLower(value)
	for _, suffix := range []string{
		" inside it", " in it", " inside the lyrics", " in the lyrics",
		" in their lyrics", " from the lyrics", " inside the song", " in the song",
	} {
		if strings.HasSuffix(lower, suffix) {
			value = strings.TrimSpace(value[:len(value)-len(suffix)])
			break
		}
	}
	return strings.Trim(value, `"'“”‘’.,!?`)
}

func toRAGHistory(history []aiChatTurn) []rag.ChatTurn {
	turns := make([]rag.ChatTurn, 0, len(history))
	for _, turn := range history {
		turns = append(turns, rag.ChatTurn{Role: turn.Role, Content: turn.Content})
	}
	return turns
}

const (
	maxAIChatHistoryTurns = 8
	maxAIChatTurnRunes    = 4000
)

// normalizeAIChatHistory bounds client-provided history and only accepts the
// two roles that can occur in this chat. This keeps both rewrite and answer
// prompts predictable even when the endpoint is called outside the web UI.
func normalizeAIChatHistory(history []aiChatTurn) []aiChatTurn {
	normalized := make([]aiChatTurn, 0, min(len(history), maxAIChatHistoryTurns))
	for index := len(history) - 1; index >= 0 && len(normalized) < maxAIChatHistoryTurns; index-- {
		role := strings.ToLower(strings.TrimSpace(history[index].Role))
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(history[index].Content)
		if content == "" {
			continue
		}
		runes := []rune(content)
		if len(runes) > maxAIChatTurnRunes {
			content = string(runes[:maxAIChatTurnRunes])
		}
		normalized = append(normalized, aiChatTurn{Role: role, Content: content})
	}
	for left, right := 0, len(normalized)-1; left < right; left, right = left+1, right-1 {
		normalized[left], normalized[right] = normalized[right], normalized[left]
	}
	return normalized
}

// rewriteFollowUpQuery asks the model to turn a conversational follow-up into a
// self-contained library search query. On any failure it falls back to the raw
// message so retrieval always proceeds.
func rewriteFollowUpQuery(ctx context.Context, provider aiChatProvider, history []aiChatTurn, message string) string {
	var conversation strings.Builder
	for _, turn := range history {
		content := strings.TrimSpace(turn.Content)
		if content == "" {
			continue
		}
		role := strings.TrimSpace(turn.Role)
		if role == "" {
			role = "user"
		}
		fmt.Fprintf(&conversation, "%s: %s\n", role, content)
	}
	if conversation.Len() == 0 {
		return message
	}

	prompt := `Rewrite the user's latest message into a single standalone search query about their music library, resolving any references to the earlier conversation. Return only the rewritten query with no quotes or explanation.

Conversation:
` + strings.TrimSpace(conversation.String()) + `

Latest message: ` + strings.TrimSpace(message) + `

Standalone query:`

	started := time.Now()
	answer, err := provider.Chat(ctx, prompt)
	if err != nil {
		recordAIChatTraceStage(ctx, aiChatTraceStage{
			ID: "rewrite", Label: "Rewrote the conversational follow-up", Status: "fallback",
			DurationMS: time.Since(started).Milliseconds(),
			Detail:     "The rewrite call failed, so the original message was used for retrieval.",
			Prompt:     prompt, Error: err.Error(), Output: map[string]any{"query": message},
		})
		return message
	}
	rewritten := strings.TrimSpace(answer)
	if idx := strings.IndexAny(rewritten, "\n\r"); idx >= 0 {
		rewritten = strings.TrimSpace(rewritten[:idx])
	}
	rewritten = strings.Trim(rewritten, `"'`)
	if rewritten == "" || len([]rune(rewritten)) > 300 {
		recordAIChatTraceStage(ctx, aiChatTraceStage{
			ID: "rewrite", Label: "Rewrote the conversational follow-up", Status: "fallback",
			DurationMS: time.Since(started).Milliseconds(),
			Detail:     "The rewrite response was empty or too long, so the original message was used for retrieval.",
			Prompt:     prompt, Response: answer, Output: map[string]any{"query": message},
		})
		return message
	}
	recordAIChatTraceStage(ctx, aiChatTraceStage{
		ID: "rewrite", Label: "Rewrote the conversational follow-up", Status: "completed",
		DurationMS: time.Since(started).Milliseconds(),
		Detail:     "The model resolved references to earlier messages before library retrieval.",
		Prompt:     prompt, Response: answer, Output: map[string]any{"query": rewritten},
	})
	return rewritten
}

type ragQueryMode string

const (
	ragQueryModeSearch     ragQueryMode = "search"
	ragQueryModeLyrics     ragQueryMode = "lyrics"
	ragQueryModeAnalytics  ragQueryMode = "analytics"
	ragQueryModeDuplicates ragQueryMode = "duplicates"
)

type ragQueryPlan struct {
	Mode    ragQueryMode      `json:"mode"`
	Query   string            `json:"query"`
	Filters rag.SearchFilters `json:"filters"`
}

func extractRAGFilters(ctx context.Context, provider aiChatProvider, message string) (rag.SearchFilters, error) {
	plan, err := extractRAGQueryPlan(ctx, provider, message)
	return plan.Filters, err
}

func extractRAGQueryPlan(ctx context.Context, provider aiChatProvider, message string) (payload ragQueryPlan, returnErr error) {
	if provider == nil {
		return ragQueryPlan{}, fmt.Errorf("AI provider is unavailable")
	}
	prompt := `Call the plan_library_query function by returning exactly one JSON object containing its arguments. Classify the request and extract only constraints explicitly requested by the user.

Function schema:
{"mode":"search|lyrics|analytics|duplicates","query":"standalone semantic query","filters":{"explicit":"clean|explicit","genre":"string","mood":"string","lyricsContains":"string","yearMin":integer,"yearMax":integer,"bpmMin":number,"bpmMax":number,"lufsMin":number,"lufsMax":number,"playCountMin":integer,"playCountMax":integer,"durationMin":number,"durationMax":number,"hasLyrics":boolean,"hasGenre":boolean,"hasYear":boolean,"hasBpm":boolean,"hasLufs":boolean}}

Rules:
- mode="lyrics" for requests identifying a song from quoted or remembered lyrics, or asking which songs contain a word or phrase in their lyrics.
- For mode="lyrics", set filters.lyricsContains to the exact remembered word or phrase only (e.g. "which song includes the word rain" gives lyricsContains "rain"); never include surrounding words like "the word" or "lyrics".
- mode="analytics" for counts, totals, distributions, dominant artists/genres, coverage, or library-wide comparisons.
- mode="duplicates" for duplicate rips, repeated recordings, live/remix/edit variants, or cleanup candidates.
- mode="search" for ordinary song discovery and recommendations.
- query should preserve descriptive terms needed for semantic retrieval; for lyrics, use only the remembered lyric phrase when possible.
- Omit every field that the user did not request. Never emit null values.
- Convert decades such as "90s" to yearMin 1990 and yearMax 1999.
- Convert durations to seconds. "Longer than 4 minutes" is durationMin 240.
- Use explicit="clean" for clean/family-safe and explicit="explicit" only when explicit content is requested.
- Genre and mood must be short canonical tag values. Map high-energy/energetic requests to mood="Energetic" when appropriate. Leave acoustic and other descriptive qualities to semantic retrieval unless they are clearly a genre or mood.
- Do not invent numeric thresholds for subjective language.
- Return an empty filters object when there are no hard constraints; always include mode and query.

User query:
` + strings.TrimSpace(message)

	started := time.Now()
	answer := ""
	defer func() {
		status := "completed"
		errorText := ""
		if returnErr != nil {
			status = "failed"
			errorText = returnErr.Error()
		}
		recordAIChatTraceStage(ctx, aiChatTraceStage{
			ID: "query_plan", Label: "Planned the library query", Status: status,
			DurationMS: time.Since(started).Milliseconds(),
			Detail:     "The model classified the request and extracted only explicit search filters.",
			Prompt:     prompt, Response: answer, Error: errorText, Output: payload,
		})
	}()
	var err error
	answer, err = provider.Chat(ctx, prompt)
	log.Debug(ctx, "RAG structured filter extraction completed", "duration", time.Since(started), "success", err == nil)
	if err != nil {
		return ragQueryPlan{}, fmt.Errorf("query planning failed: %w", err)
	}
	answer = strings.TrimSpace(answer)
	if len(answer) > 16*1024 {
		return ragQueryPlan{}, fmt.Errorf("query planning response is too large")
	}
	start, end := strings.Index(answer, "{"), strings.LastIndex(answer, "}")
	if start < 0 || end < start {
		return ragQueryPlan{}, fmt.Errorf("query planning did not return JSON")
	}
	payload = ragQueryPlan{}
	decoder := json.NewDecoder(strings.NewReader(answer[start : end+1]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return ragQueryPlan{}, fmt.Errorf("invalid query planning JSON: %w", err)
	}
	filters, err := rag.NormalizeSearchFilters(payload.Filters)
	if err != nil {
		return ragQueryPlan{}, fmt.Errorf("invalid extracted filters: %w", err)
	}
	payload.Filters = filters
	payload.Query = strings.TrimSpace(payload.Query)
	switch payload.Mode {
	case ragQueryModeLyrics, ragQueryModeAnalytics, ragQueryModeDuplicates, ragQueryModeSearch:
	default:
		payload.Mode = ragQueryModeSearch
	}
	return payload, nil
}
