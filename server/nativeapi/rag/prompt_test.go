package rag

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBuildChatPromptIncludesLibraryContext(t *testing.T) {
	prompt := BuildChatPrompt("find upbeat songs", []SongSearchResult{{
		SongID: "song-1", Title: "Bright Song", Artist: "Artist", Album: "Album",
		Year: 2020, Genre: "Pop", BPM: 100, LUFS: -12.5, Score: 0.87,
	}}, SearchFilters{Explicit: "clean"})

	for _, expected := range []string{
		"Answer only using the provided library context",
		"Bright Song — Artist",
		"genre: Pop",
		"score: 0.8700",
		"User question:\nfind upbeat songs",
		"Applied filters:\n{\"explicit\":\"clean\"}",
	} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("expected prompt to contain %q; got:\n%s", expected, prompt)
		}
	}
}

func TestBuildChatPromptBoundsRetrievedContext(t *testing.T) {
	results := make([]SongSearchResult, 50)
	for index := range results {
		results[index] = SongSearchResult{
			SongID: fmt.Sprintf("song-%d", index), Title: fmt.Sprintf("Song %d", index), Artist: "Artist",
			LyricSnippet: strings.Repeat("long lyric context ", 35), Score: .9,
		}
	}
	prompt := BuildChatPrompt("find something", results)
	if !strings.Contains(prompt, "additional retrieved songs omitted") {
		t.Fatalf("expected prompt context omission marker, got %d runes", utf8.RuneCountInString(prompt))
	}
	if utf8.RuneCountInString(prompt) > maxPromptRetrievedContextRunes+2000 {
		t.Fatalf("retrieved context budget was not enforced: %d runes", utf8.RuneCountInString(prompt))
	}
}

func TestBuildChatPromptKeepsRecentHistoryWithinBudget(t *testing.T) {
	history := make([]ChatTurn, 10)
	for index := range history {
		history[index] = ChatTurn{
			Role: "user", Content: fmt.Sprintf("turn-%d %s", index, strings.Repeat("history ", 400)),
		}
	}
	prompt := BuildChatPromptWithHistory("latest", history, nil)
	if !strings.Contains(prompt, "earlier conversation omitted") || !strings.Contains(prompt, "turn-9") {
		t.Fatalf("expected bounded recent history, got prompt length %d", utf8.RuneCountInString(prompt))
	}
	if strings.Contains(prompt, "turn-0") {
		t.Fatalf("oldest history should have been omitted")
	}
}

func TestBuildChatPromptIncludesNoContextFallback(t *testing.T) {
	prompt := BuildChatPrompt("find songs", nil)
	if !strings.Contains(prompt, "(no matching songs found)") ||
		!strings.Contains(prompt, "did not find enough matching songs") {
		t.Fatalf("expected no-context fallback instruction; got:\n%s", prompt)
	}
}

func TestBuildChatPromptIncludesConversationHistory(t *testing.T) {
	prompt := BuildChatPromptWithHistory(
		"only the clean ones",
		[]ChatTurn{
			{Role: "user", Content: "show upbeat rock"},
			{Role: "assistant", Content: "Try Song A."},
		},
		[]SongSearchResult{{SongID: "song-b", Title: "Song B", Artist: "Band", Score: 0.9}},
	)
	for _, expected := range []string{
		"Conversation so far:",
		"user: show upbeat rock",
		"assistant: Try Song A.",
		"User question:\nonly the clean ones",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected prompt to contain %q; got:\n%s", expected, prompt)
		}
	}
}
