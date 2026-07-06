package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	if !ragEmbeddingConfigured() {
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
	if !ragEnabled() {
		return message, nil, nil
	}

	history = normalizeAIChatHistory(history)

	// For a follow-up ("only clean ones", "more like that"), condense the turn into a
	// standalone query so retrieval isn't tripped up by pronouns and ellipsis.
	retrievalQuery := message
	if len(history) > 0 && provider != nil {
		retrievalQuery = rewriteFollowUpQuery(ctx, provider, history, message)
	}

	plan, filterErr := extractRAGQueryPlan(ctx, provider, retrievalQuery)
	if filterErr != nil {
		log.Debug(ctx, "RAG structured filter extraction failed; continuing without hard filters", "err", filterErr)
		plan = ragQueryPlan{Mode: ragQueryModeSearch}
	}
	if plan.Query != "" {
		retrievalQuery = plan.Query
	}
	filters := plan.Filters
	if plan.Mode == ragQueryModeAnalytics && features.analytics != nil {
		data, err := features.analytics(ctx, filters)
		if err != nil {
			return message, nil, err
		}
		return rag.BuildLibraryDataPrompt(message, toRAGHistory(history), "library analytics", data), nil, nil
	}
	if plan.Mode == ragQueryModeDuplicates && features.duplicates != nil {
		data, err := features.duplicates(ctx)
		if err != nil {
			return message, nil, err
		}
		return rag.BuildLibraryDataPrompt(message, toRAGHistory(history), "duplicate and alternate-version candidates", data), nil, nil
	}
	if plan.Mode == ragQueryModeLyrics {
		required := true
		filters.HasLyrics = &required
	}
	results, err := search(ctx, retrievalQuery, conf.Server.RAGTopK, filters)
	if err != nil {
		return message, nil, err
	}
	results = rag.FilterByMinScore(results, conf.Server.RAGMinScore)
	if plan.Mode == ragQueryModeLyrics {
		results = rag.AddLyricSnippets(retrievalQuery, results)
	}
	return rag.BuildChatPromptWithHistory(message, toRAGHistory(history), results, filters), results, nil
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

	answer, err := provider.Chat(ctx, prompt)
	if err != nil {
		return message
	}
	rewritten := strings.TrimSpace(answer)
	if idx := strings.IndexAny(rewritten, "\n\r"); idx >= 0 {
		rewritten = strings.TrimSpace(rewritten[:idx])
	}
	rewritten = strings.Trim(rewritten, `"'`)
	if rewritten == "" || len([]rune(rewritten)) > 300 {
		return message
	}
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

func extractRAGQueryPlan(ctx context.Context, provider aiChatProvider, message string) (ragQueryPlan, error) {
	if provider == nil {
		return ragQueryPlan{}, fmt.Errorf("AI provider is unavailable")
	}
	prompt := `Call the plan_library_query function by returning exactly one JSON object containing its arguments. Classify the request and extract only constraints explicitly requested by the user.

Function schema:
{"mode":"search|lyrics|analytics|duplicates","query":"standalone semantic query","filters":{"explicit":"clean|explicit","genre":"string","mood":"string","yearMin":integer,"yearMax":integer,"bpmMin":number,"bpmMax":number,"lufsMin":number,"lufsMax":number,"playCountMin":integer,"playCountMax":integer,"durationMin":number,"durationMax":number,"hasLyrics":boolean,"hasGenre":boolean,"hasYear":boolean,"hasBpm":boolean,"hasLufs":boolean}}

Rules:
- mode="lyrics" for requests identifying a song from quoted or remembered lyrics.
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
	answer, err := provider.Chat(ctx, prompt)
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
	payload := ragQueryPlan{}
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
