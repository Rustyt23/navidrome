package rag

import (
	"fmt"
	"strings"
)

// PromptContext is the provider-independent input shape for RAG prompts.
type PromptContext struct {
	Question  string        `json:"question"`
	Documents []RAGDocument `json:"documents"`
}

// BuildChatPrompt constrains an AI answer to the retrieved music-library
// context and provides a deterministic fallback instruction for no matches.
func BuildChatPrompt(question string, results []SongSearchResult) string {
	var context strings.Builder
	if len(results) == 0 {
		context.WriteString("(no matching songs found)")
	} else {
		for index, result := range results {
			fmt.Fprintf(
				&context,
				"%d. %s — %s | album: %s | year: %d | genre: %s | explicit: %t | bpm: %d | lufs: %.2f | score: %.4f\n",
				index+1,
				result.Title,
				result.Artist,
				result.Album,
				result.Year,
				result.Genre,
				result.Explicit,
				result.BPM,
				result.LUFS,
				result.Score,
			)
		}
	}

	return `You are a music-library assistant. Answer only using the provided library context.
Do not invent songs or facts that are not present in the context.
If the context does not contain enough useful matching songs, say that you did not find enough matching songs in the library.

Library context:
` + strings.TrimSpace(context.String()) + `

User message:
` + strings.TrimSpace(question)
}
