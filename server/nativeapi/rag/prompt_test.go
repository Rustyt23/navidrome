package rag

import (
	"strings"
	"testing"
)

func TestBuildChatPromptIncludesLibraryContext(t *testing.T) {
	prompt := BuildChatPrompt("find upbeat songs", []SongSearchResult{{
		SongID: "song-1", Title: "Bright Song", Artist: "Artist", Album: "Album",
		Year: 2020, Genre: "Pop", BPM: 100, LUFS: -12.5, Score: 0.87,
	}})

	for _, expected := range []string{
		"Answer only using the provided library context",
		"Bright Song — Artist",
		"genre: Pop",
		"score: 0.8700",
		"User message:\nfind upbeat songs",
	} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("expected prompt to contain %q; got:\n%s", expected, prompt)
		}
	}
}

func TestBuildChatPromptIncludesNoContextFallback(t *testing.T) {
	prompt := BuildChatPrompt("find songs", nil)
	if !strings.Contains(prompt, "(no matching songs found)") ||
		!strings.Contains(prompt, "did not find enough matching songs") {
		t.Fatalf("expected no-context fallback instruction; got:\n%s", prompt)
	}
}
