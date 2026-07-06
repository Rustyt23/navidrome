package rag

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAddLyricSnippetsHighlightsQuotedPhrase(t *testing.T) {
	results := AddLyricSnippets(`find the song that goes "we are the champions"`, []SongSearchResult{{
		SongID: "song-1", LyricsText: "A private opening line\nI've paid my dues\nWe are the champions, my friends\nAnd we'll keep on fighting\nA private closing line",
	}})
	if len(results) != 1 || !strings.Contains(results[0].LyricSnippet, "⟦We are the champions⟧") {
		t.Fatalf("expected highlighted lyric snippet, got %+v", results)
	}
	encoded, err := json.Marshal(results[0])
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(encoded), "private opening") || !strings.Contains(string(encoded), "lyricSnippet") {
		t.Fatalf("full lyrics must remain private while snippet is returned: %s", encoded)
	}
}

func TestAddLyricSnippetsUsesBestTokenLine(t *testing.T) {
	results := AddLyricSnippets("midnight train home", []SongSearchResult{{
		LyricsText: "Morning has broken\nI took the midnight train going home\nA different line",
	}})
	if !strings.Contains(results[0].LyricSnippet, "⟦midnight⟧") || !strings.Contains(results[0].LyricSnippet, "⟦train⟧") {
		t.Fatalf("expected token highlights, got %q", results[0].LyricSnippet)
	}
}
