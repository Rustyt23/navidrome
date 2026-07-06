package rag

import (
	"regexp"
	"sort"
	"strings"
)

var quotedLyricPattern = regexp.MustCompile(`["'“”‘’]([^"'“”‘’]{2,})["'“”‘’]`)

// AddLyricSnippets annotates search results with the best matching lyric line
// and visible highlight markers. The full lyrics remain internal and are never
// serialized in a search response.
func AddLyricSnippets(query string, results []SongSearchResult) []SongSearchResult {
	needle := lyricNeedle(query)
	annotated := make([]SongSearchResult, len(results))
	copy(annotated, results)
	for index := range annotated {
		annotated[index].LyricSnippet = bestLyricSnippet(needle, annotated[index].LyricsText)
	}
	return annotated
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
	lines := strings.Split(lyrics, "\n")
	tokens := uniqueLyricTokens(needle)
	type candidate struct {
		index int
		score int
	}
	candidates := make([]candidate, 0, len(lines))
	lowerNeedle := strings.ToLower(strings.TrimSpace(needle))
	for index, line := range lines {
		lowerLine := strings.ToLower(line)
		score := 0
		if lowerNeedle != "" && strings.Contains(lowerLine, lowerNeedle) {
			score += 100
		}
		for _, token := range tokens {
			if strings.Contains(lowerLine, token) {
				score++
			}
		}
		candidates = append(candidates, candidate{index: index, score: score})
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	best := candidates[0].index
	start, end := max(0, best-1), min(len(lines), best+2)
	snippet := strings.Join(lines[start:end], " / ")
	if lowerNeedle != "" && strings.Contains(strings.ToLower(snippet), lowerNeedle) {
		return highlightInsensitive(snippet, needle)
	}
	for _, token := range tokens {
		snippet = highlightInsensitive(snippet, token)
	}
	return snippet
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
