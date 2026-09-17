package rag

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
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
	if strings.Contains(string(encoded), "private closing") || !strings.Contains(string(encoded), "lyricSnippet") {
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

func TestAddLyricSnippetsBoundsSingleLineWhisperTranscript(t *testing.T) {
	lyrics := "very beginning that must stay out " + strings.Repeat("opening context ", 40) +
		"I only wanted to say thank you before leaving " + strings.Repeat("closing context ", 40) +
		"very ending that must stay out"
	results := AddLyricSnippets("thank you", []SongSearchResult{{LyricsText: lyrics}})
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	snippet := results[0].LyricSnippet
	if !strings.Contains(snippet, "⟦thank you⟧") {
		t.Fatalf("expected highlighted phrase, got %q", snippet)
	}
	if strings.Contains(snippet, "very beginning") || strings.Contains(snippet, "very ending") {
		t.Fatalf("single-line transcript leaked distant lyrics: %q", snippet)
	}
	if utf8.RuneCountInString(snippet) > maxLyricSnippetRunes+2 {
		t.Fatalf("snippet exceeded hard limit: %d runes", utf8.RuneCountInString(snippet))
	}
}

func TestDeduplicateSongResultsNormalizesAccentSpelling(t *testing.T) {
	results := DeduplicateSongResults([]SongSearchResult{
		{SongID: "one", Title: "03' Bonnie & Clyde", Artist: "JAY-Z, Beyonce", Duration: 204, Album: "The Blueprint"},
		{SongID: "two", Title: "03 Bonnie and Clyde", Artist: "JAY-Z, Beyoncé", Duration: 204, Genre: "Hip Hop"},
	})
	if len(results) != 1 {
		t.Fatalf("expected equivalent recordings to be deduplicated, got %+v", results)
	}
	if results[0].Genre != "Hip Hop" {
		t.Fatalf("expected missing metadata to be merged, got %+v", results[0])
	}
}

func TestFilterExactLyricMatchesRequiresContiguousPhrase(t *testing.T) {
	results := FilterExactLyricMatches("thank you", []SongSearchResult{
		{SongID: "exact", LyricsText: "I want to thank, you for everything"},
		{SongID: "tokens-only", LyricsText: "thank everyone who helped, and you as well"},
	})
	if len(results) != 1 || results[0].SongID != "exact" {
		t.Fatalf("expected only the contiguous phrase match, got %+v", results)
	}
}

func TestBuildExactLyricsResponseIsCompact(t *testing.T) {
	results := AddLyricSnippets("thank you", []SongSearchResult{{
		SongID: "song-1", Title: "Gratitude", Artist: "Singer",
		LyricsText: strings.Repeat("distant words ", 40) + "thank you my friend" + strings.Repeat(" later words", 40),
		BPM:        120, LUFS: -10.5, PlayCount: 99,
	}})
	response := BuildExactLyricsResponse("thank you", results)
	for _, expected := range []string{"Here is 1 indexed song", "Gratitude — Singer", "⟦thank you⟧"} {
		if !strings.Contains(response, expected) {
			t.Fatalf("expected compact response to contain %q: %s", expected, response)
		}
	}
	for _, forbidden := range []string{strings.Repeat("distant words ", 12), "BPM", "LUFS", "play count"} {
		if strings.Contains(response, forbidden) {
			t.Fatalf("compact response contains unnecessary data %q: %s", forbidden, response)
		}
	}
}
