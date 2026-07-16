package rag

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var quotedLyricPattern = regexp.MustCompile(`["'“”‘’]([^"'“”‘’]{2,})["'“”‘’]`)
var lyricWordPattern = regexp.MustCompile(`\S+`)

const (
	lyricSnippetContextWords = 8
	maxLyricSnippetRunes     = 240
)

// AddLyricSnippets annotates search results with a small window around the best
// match and visible highlight markers. The window is word-position based, so a
// Whisper transcript stored as one long line cannot become the whole snippet.
// Full lyrics remain internal and are never serialized in a search response.
func AddLyricSnippets(query string, results []SongSearchResult) []SongSearchResult {
	needle := lyricNeedle(query)
	annotated := make([]SongSearchResult, len(results))
	copy(annotated, results)
	for index := range annotated {
		annotated[index].LyricSnippet = bestLyricSnippet(needle, annotated[index].LyricsText)
	}
	return annotated
}

// FilterExactLyricMatches verifies that a multi-word Qdrant token match is an
// actual contiguous word/phrase match in the stored lyrics. Qdrant's text
// filter guarantees the tokens are present, but not that they are adjacent.
func FilterExactLyricMatches(needle string, results []SongSearchResult) []SongSearchResult {
	normalizedNeedle := normalizeExactLyricText(needle)
	if normalizedNeedle == "" {
		return nil
	}
	filtered := make([]SongSearchResult, 0, len(results))
	for _, result := range results {
		lyrics := normalizeExactLyricText(result.LyricsText)
		if strings.Contains(lyrics, normalizedNeedle) {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

func normalizeExactLyricText(value string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(value), func(char rune) bool {
		return !unicode.IsLetter(char) && !unicode.IsNumber(char)
	}), " ")
}

// DeduplicateSongResults collapses equivalent indexed recordings while
// preserving result order. Song IDs catch ordinary duplicates; the normalized
// recording key also catches re-imported copies whose IDs or accent spelling
// differ (for example, Beyonce and Beyoncé).
func DeduplicateSongResults(results []SongSearchResult) []SongSearchResult {
	deduplicated := make([]SongSearchResult, 0, len(results))
	positions := make(map[string]int, len(results)*2)
	for _, result := range results {
		keys := []string{"id:" + strings.TrimSpace(result.SongID), "recording:" + songRecordingKey(result)}
		duplicateAt := -1
		for _, key := range keys {
			if key == "id:" || key == "recording:" {
				continue
			}
			if index, exists := positions[key]; exists {
				duplicateAt = index
				break
			}
		}
		if duplicateAt >= 0 {
			deduplicated[duplicateAt] = mergeSongSearchResult(deduplicated[duplicateAt], result)
			for _, key := range keys {
				if key != "id:" && key != "recording:" {
					positions[key] = duplicateAt
				}
			}
			continue
		}
		index := len(deduplicated)
		deduplicated = append(deduplicated, result)
		for _, key := range keys {
			if key != "id:" && key != "recording:" {
				positions[key] = index
			}
		}
	}
	return deduplicated
}

// BuildExactLyricsResponse formats an exact Qdrant lyrics match without
// involving a chat model. It deliberately includes only compact identity data
// and one bounded snippet per song.
func BuildExactLyricsResponse(needle string, results []SongSearchResult) string {
	results = DeduplicateSongResults(results)
	needle = strings.TrimSpace(needle)
	if len(results) == 0 {
		return fmt.Sprintf("I found no indexed songs containing %q in their lyrics.", needle)
	}
	var response strings.Builder
	verb := "are"
	if len(results) == 1 {
		verb = "is"
	}
	fmt.Fprintf(&response, "Here %s %d indexed %s containing %q in the lyrics:\n", verb, len(results), pluralizeSong(len(results)), needle)
	for index, result := range results {
		title := strings.TrimSpace(result.Title)
		if title == "" {
			title = "Unknown title"
		}
		artist := strings.TrimSpace(result.Artist)
		if artist == "" {
			artist = "Unknown artist"
		}
		fmt.Fprintf(&response, "\n%d. %s — %s", index+1, title, artist)
		if snippet := strings.TrimSpace(result.LyricSnippet); snippet != "" {
			fmt.Fprintf(&response, "\n   %s", snippet)
		}
	}
	return response.String()
}

func lyricNeedle(query string) string {
	if match := quotedLyricPattern.FindStringSubmatch(query); len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	query = strings.TrimSpace(query)
	for _, prefix := range []string{
		"find the song that goes", "find a song that goes", "what song goes",
		"which song goes", "song that goes", "lyrics", "lyric",
	} {
		if strings.HasPrefix(strings.ToLower(query), prefix) {
			query = strings.TrimSpace(query[len(prefix):])
			break
		}
	}
	return strings.Trim(query, " \t\n\r:;,.!?\"'")
}

func bestLyricSnippet(needle, lyrics string) string {
	if strings.TrimSpace(lyrics) == "" {
		return ""
	}
	spans := lyricWordPattern.FindAllStringIndex(lyrics, -1)
	if len(spans) == 0 {
		return ""
	}
	tokens := uniqueLyricTokens(needle)
	lowerNeedle := strings.ToLower(strings.TrimSpace(needle))
	target := bestLyricWordIndex(lyrics, spans, lowerNeedle, tokens)
	start := max(0, target-lyricSnippetContextWords)
	phraseWords := max(1, len(strings.Fields(needle)))
	end := min(len(spans), target+phraseWords+lyricSnippetContextWords)
	snippet := strings.Join(strings.Fields(lyrics[spans[start][0]:spans[end-1][1]]), " ")
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(spans) {
		snippet += "…"
	}
	snippet = limitLyricSnippet(snippet, needle, maxLyricSnippetRunes)
	if lowerNeedle != "" && strings.Contains(strings.ToLower(snippet), lowerNeedle) {
		return highlightInsensitive(snippet, needle)
	}
	for _, token := range tokens {
		snippet = highlightInsensitive(snippet, token)
	}
	return snippet
}

func bestLyricWordIndex(lyrics string, spans [][]int, lowerNeedle string, tokens []string) int {
	lowerLyrics := strings.ToLower(lyrics)
	if lowerNeedle != "" {
		if match := strings.Index(lowerLyrics, lowerNeedle); match >= 0 {
			for index, span := range spans {
				if span[1] > match {
					return index
				}
			}
		}
	}

	type candidate struct {
		index int
		score int
	}
	candidates := make([]candidate, 0, len(spans))
	for index := range spans {
		start := max(0, index-lyricSnippetContextWords)
		end := min(len(spans), index+lyricSnippetContextWords+1)
		window := strings.ToLower(lyrics[spans[start][0]:spans[end-1][1]])
		score := 0
		for _, token := range tokens {
			if strings.Contains(window, token) {
				score++
			}
		}
		candidates = append(candidates, candidate{index: index, score: score})
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	return candidates[0].index
}

func limitLyricSnippet(snippet, needle string, limit int) string {
	runes := []rune(snippet)
	if len(runes) <= limit {
		return snippet
	}
	matchRune := 0
	if match := strings.Index(strings.ToLower(snippet), strings.ToLower(strings.TrimSpace(needle))); match >= 0 {
		matchRune = len([]rune(snippet[:match]))
	}
	start := max(0, matchRune-limit/2)
	end := min(len(runes), start+limit)
	if end-start < limit {
		start = max(0, end-limit)
	}
	trimmed := string(runes[start:end])
	if start > 0 {
		trimmed = "…" + trimmed
	}
	if end < len(runes) {
		trimmed += "…"
	}
	return trimmed
}

func songRecordingKey(result SongSearchResult) string {
	title := normalizeSongIdentityPart(result.Title)
	artist := normalizeSongIdentityPart(result.Artist)
	if title == "" || artist == "" {
		return ""
	}
	duration := int(math.Round(result.Duration))
	if duration <= 0 {
		return strings.Join([]string{title, artist, normalizeSongIdentityPart(result.Album)}, "|")
	}
	return strings.Join([]string{title, artist, strconv.Itoa(duration)}, "|")
}

func normalizeSongIdentityPart(value string) string {
	var normalized strings.Builder
	for _, char := range norm.NFD.String(strings.ToLower(strings.TrimSpace(value))) {
		if unicode.Is(unicode.Mn, char) {
			continue
		}
		if char == '&' {
			normalized.WriteString("and")
			continue
		}
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			normalized.WriteRune(char)
		}
	}
	return normalized.String()
}

func mergeSongSearchResult(existing, duplicate SongSearchResult) SongSearchResult {
	if existing.Album == "" {
		existing.Album = duplicate.Album
	}
	if existing.Year == 0 {
		existing.Year = duplicate.Year
	}
	if existing.Genre == "" {
		existing.Genre = duplicate.Genre
	}
	if existing.LyricsText == "" {
		existing.LyricsText = duplicate.LyricsText
	}
	if existing.LyricSnippet == "" {
		existing.LyricSnippet = duplicate.LyricSnippet
	}
	return existing
}

func pluralizeSong(count int) string {
	if count == 1 {
		return "song"
	}
	return "songs"
}

func uniqueLyricTokens(value string) []string {
	seen := map[string]struct{}{}
	tokens := make([]string, 0)
	for _, token := range tokenizeText(value) {
		if len([]rune(token)) < 3 {
			continue
		}
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		tokens = append(tokens, token)
	}
	return tokens
}

func highlightInsensitive(value, match string) string {
	if strings.TrimSpace(match) == "" {
		return value
	}
	pattern, err := regexp.Compile(`(?i)` + regexp.QuoteMeta(match))
	if err != nil {
		return value
	}
	return pattern.ReplaceAllStringFunc(value, func(found string) string { return "⟦" + found + "⟧" })
}
