package rag

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxPromptRetrievedContextRunes = 12000
	maxPromptHistoryRunes          = 8000
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
		omitted := 0
		contextRunes := 0
		for index, result := range results {
			var item strings.Builder
			fmt.Fprintf(
				&item,
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
				fmt.Fprintf(&item, "   matching lyric: %s\n", result.LyricSnippet)
			}
			itemText := item.String()
			itemRunes := utf8.RuneCountInString(itemText)
			if contextRunes+itemRunes > maxPromptRetrievedContextRunes {
				omitted = len(results) - index
				break
			}
			context.WriteString(itemText)
			contextRunes += itemRunes
		}
		if omitted > 0 {
			fmt.Fprintf(&context, "[%d additional retrieved songs omitted to keep the prompt within its context budget]\n", omitted)
		}
	}

	searchFilters := SearchFilters{}
	if len(filters) > 0 {
		searchFilters = filters[0]
	}
	appliedFilters, _ := json.Marshal(searchFilters)

	historyBlock := boundedChatHistory(history)

	prompt := `You are a music-library assistant. Answer only using the provided library context.
Do not invent songs or facts that are not present in the context.
If the context does not contain enough useful matching songs, say that you did not find enough matching songs in the library.
`
	if historyBlock != "" {
		prompt += "\nConversation so far:\n" + historyBlock + "\n"
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
	historyBlock := boundedChatHistory(history)
	prompt := `You are a music-library analyst. Answer only from the computed library data below.
Do not estimate, invent counts, or treat a retrieved sample as the whole library.
State clearly when the computed data is empty.`
	if historyBlock != "" {
		prompt += "\n\nConversation so far:\n" + historyBlock
	}
	return prompt + "\n\nUser question:\n" + strings.TrimSpace(question) +
		"\n\nComputed " + strings.TrimSpace(label) + ":\n" + strings.TrimSpace(data)
}

func boundedChatHistory(history []ChatTurn) string {
	entries := make([]string, 0, len(history))
	used := 0
	omitted := false
	for index := len(history) - 1; index >= 0; index-- {
		content := strings.TrimSpace(history[index].Content)
		if content == "" {
			continue
		}
		role := strings.TrimSpace(history[index].Role)
		if role == "" {
			role = "user"
		}
		entry := role + ": " + content
		entryRunes := utf8.RuneCountInString(entry) + 1
		if used+entryRunes > maxPromptHistoryRunes {
			omitted = true
			break
		}
		entries = append(entries, entry)
		used += entryRunes
	}
	for left, right := 0, len(entries)-1; left < right; left, right = left+1, right-1 {
		entries[left], entries[right] = entries[right], entries[left]
	}
	if omitted {
		entries = append([]string{"[earlier conversation omitted to keep the prompt within its history budget]"}, entries...)
	}
	return strings.Join(entries, "\n")
}
