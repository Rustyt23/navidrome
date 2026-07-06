package rag

import (
	"sort"
	"strings"
	"unicode"
)

// rrfK is the Reciprocal Rank Fusion constant. 60 is the widely used default.
const rrfK = 60

// hybridRerank re-orders dense (semantic) search results by fusing their vector
// rank with a lexical rank, so exact title/artist/word matches that cosine
// similarity ranked low are pulled back up. It returns at most topK results.
func hybridRerank(query string, results []SongSearchResult, topK int) []SongSearchResult {
	if len(results) <= 1 {
		return capResults(results, topK)
	}
	queryTokens := tokenizeText(query)
	if len(queryTokens) == 0 {
		return capResults(results, topK)
	}

	denseRank := make(map[string]int, len(results))
	lexScoreByID := make(map[string]float64, len(results))
	for i := range results {
		denseRank[results[i].SongID] = i
		lexScoreByID[results[i].SongID] = lexicalScore(queryTokens, query, results[i])
	}

	// Lexical rank (used by RRF for the non-exact tier).
	lexOrder := make([]int, len(results))
	for i := range lexOrder {
		lexOrder[i] = i
	}
	sort.SliceStable(lexOrder, func(a, b int) bool {
		return lexScoreByID[results[lexOrder[a]].SongID] > lexScoreByID[results[lexOrder[b]].SongID]
	})
	lexRank := make(map[string]int, len(results))
	for rank, idx := range lexOrder {
		lexRank[results[idx].SongID] = rank
	}

	fused := make([]SongSearchResult, len(results))
	copy(fused, results)
	// A score >= strongLexicalMatch means the whole query appears in the title or
	// artist — a near-exact hit that should outrank pure-semantic neighbors.
	rrf := func(r SongSearchResult) float64 {
		return 1.0/float64(rrfK+denseRank[r.SongID]) + 1.0/float64(rrfK+lexRank[r.SongID])
	}
	sort.SliceStable(fused, func(a, b int) bool {
		sa := lexScoreByID[fused[a].SongID] >= strongLexicalMatch
		sb := lexScoreByID[fused[b].SongID] >= strongLexicalMatch
		if sa != sb {
			return sa
		}
		if sa && sb {
			la, lb := lexScoreByID[fused[a].SongID], lexScoreByID[fused[b].SongID]
			if la != lb {
				return la > lb
			}
			return fused[a].Score > fused[b].Score
		}
		ra, rb := rrf(fused[a]), rrf(fused[b])
		if ra != rb {
			return ra > rb
		}
		return fused[a].Score > fused[b].Score
	})
	return capResults(fused, topK)
}

// strongLexicalMatch is the lexical score at which a result is treated as a
// near-exact match (the whole query appears in the title or artist).
const strongLexicalMatch = 3

func capResults(results []SongSearchResult, topK int) []SongSearchResult {
	if topK > 0 && len(results) > topK {
		return results[:topK]
	}
	return results
}

// lexicalScore counts how many query tokens appear in the result's searchable
// fields, with a strong boost when the whole query is contained in the title or
// artist (i.e. a near-exact match).
func lexicalScore(queryTokens []string, rawQuery string, result SongSearchResult) float64 {
	haystack := strings.ToLower(strings.Join([]string{result.Title, result.Artist, result.Album, result.Genre}, " "))
	tokenSet := make(map[string]struct{})
	for _, token := range tokenizeText(haystack) {
		tokenSet[token] = struct{}{}
	}
	score := 0.0
	for _, token := range queryTokens {
		if _, ok := tokenSet[token]; ok {
			score++
		}
	}
	normalizedQuery := normalizeLex(rawQuery)
	if normalizedQuery != "" {
		if strings.Contains(normalizeLex(result.Title), normalizedQuery) {
			score += 3
		}
		if strings.Contains(normalizeLex(result.Artist), normalizedQuery) {
			score += 3
		}
	}
	return score
}

func tokenizeText(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if len([]rune(field)) >= 2 {
			tokens = append(tokens, field)
		}
	}
	return tokens
}

func normalizeLex(text string) string {
	return strings.Join(tokenizeText(text), " ")
}

// FilterByMinScore drops results whose semantic score is below min so obviously
// irrelevant matches are not fed to the language model. A min of 0 (or less)
// disables filtering, which keeps behavior safe across embedding models whose
// score scales differ.
func FilterByMinScore(results []SongSearchResult, min float64) []SongSearchResult {
	if min <= 0 {
		return results
	}
	kept := make([]SongSearchResult, 0, len(results))
	for _, result := range results {
		if result.Score >= min {
			kept = append(kept, result)
		}
	}
	return kept
}

// selectDiverseRecommendations preserves relevance order while enforcing a
// hard per-artist cap. It intentionally returns fewer than limit when the
// candidate pool lacks enough artist variety instead of padding the response
// with one artist's overflow.
func selectDiverseRecommendations(items []RecommendationResult, limit int) []RecommendationResult {
	if limit <= 0 {
		return nil
	}
	maxPerArtist := max(1, (limit+3)/4) // at most roughly 25% of the result set
	counts := make(map[string]int, len(items))
	selected := make([]RecommendationResult, 0, min(len(items), limit))
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item.Artist))
		if key != "" && counts[key] >= maxPerArtist {
			continue
		}
		selected = append(selected, item)
		if key != "" {
			counts[key]++
		}
		if len(selected) == limit {
			break
		}
	}
	return selected
}
