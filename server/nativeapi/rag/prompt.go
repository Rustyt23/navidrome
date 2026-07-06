package rag

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PromptContext is the provider-independent input shape for RAG prompts.
type PromptContext struct {
	Question  string        `json:"question"`
	Documents []RAGDocument `json:"documents"`
}

// ChatTurn is one message in a conversation history.
type ChatTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// BuildChatPrompt constrains an AI answer to the retrieved music-library
// context and provides a deterministic fallback instruction for no matches.
func BuildChatPrompt(question string, results []SongSearchResult, filters ...SearchFilters) string {
	return BuildChatPromptWithHistory(question, nil, results, filters...)
}

// BuildChatPromptWithHistory is BuildChatPrompt with prior conversation turns
// included so the assistant can resolve follow-up questions ("cheaper ones",
// "more like that") against the ongoing conversation.
func BuildChatPromptWithHistory(question string, history []ChatTurn, results []SongSearchResult, filters ...SearchFilters) string {
	var context strings.Builder
	if len(results) == 0 {
		context.WriteString("(no matching songs found)")
	} else {
		for index, result := range results {
			fmt.Fprintf(
				&context,
				"%d. %s — %s | album: %s | year: %d | genre: %s | explicit: %t | bpm: %d | lufs: %.2f | duration: %.0fs | play count: %d | score: %.4f\n",
				index+1,
				result.Title,
				result.Artist,
				result.Album,
				result.Year,
				result.Genre,
				result.Explicit,
				result.BPM,
				result.LUFS,
				result.Duration,
				result.PlayCount,
				result.Score,
			)
			if result.LyricSnippet != "" {
				fmt.Fprintf(&context, "   matching lyric: %s\n", result.LyricSnippet)
			}
		}
	}

	searchFilters := SearchFilters{}
	if len(filters) > 0 {
		searchFilters = filters[0]
	}
	appliedFilters, _ := json.Marshal(searchFilters)

	var historyBlock strings.Builder
	for _, turn := range history {
		role := strings.TrimSpace(turn.Role)
		content := strings.TrimSpace(turn.Content)
		if content == "" {
			continue
		}
		if role == "" {
			role = "user"
		}
		fmt.Fprintf(&historyBlock, "%s: %s\n", role, content)
	}

	prompt := `You are a music-library assistant. Answer only using the provided library context.
Do not invent songs or facts that are not present in the context.
If the context does not contain enough useful matching songs, say that you did not find enough matching songs in the library.
`
	if historyBlock.Len() > 0 {
		prompt += "\nConversation so far:\n" + strings.TrimSpace(historyBlock.String()) + "\n"
	}
	return prompt + `
User question:
` + strings.TrimSpace(question) + `

Applied filters:
` + string(appliedFilters) + `

Retrieved songs:
` + strings.TrimSpace(context.String())
}

// BuildLibraryDataPrompt answers analytics and cleanup questions from computed
// JSON rather than asking the model to estimate counts from retrieved samples.
func BuildLibraryDataPrompt(question string, history []ChatTurn, label, data string) string {
	var historyBlock strings.Builder
	for _, turn := range history {
		if content := strings.TrimSpace(turn.Content); content != "" {
			role := strings.TrimSpace(turn.Role)
			if role == "" {
				role = "user"
			}
			fmt.Fprintf(&historyBlock, "%s: %s\n", role, content)
		}
	}
	prompt := `You are a music-library analyst. Answer only from the computed library data below.
Do not estimate, invent counts, or treat a retrieved sample as the whole library.
State clearly when the computed data is empty.`
	if historyBlock.Len() > 0 {
		prompt += "\n\nConversation so far:\n" + strings.TrimSpace(historyBlock.String())
	}
	return prompt + "\n\nUser question:\n" + strings.TrimSpace(question) +
		"\n\nComputed " + strings.TrimSpace(label) + ":\n" + strings.TrimSpace(data)
}
