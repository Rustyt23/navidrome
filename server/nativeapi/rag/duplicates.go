package rag

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

type DuplicateKind string

const (
	DuplicateExact     DuplicateKind = "likely_duplicate"
	DuplicateAlternate DuplicateKind = "alternate_version"
)

type DuplicateCandidate struct {
	FirstSongID  string        `json:"firstSongId"`
	SecondSongID string        `json:"secondSongId"`
	FirstTitle   string        `json:"firstTitle"`
	SecondTitle  string        `json:"secondTitle"`
	Artist       string        `json:"artist"`
	Similarity   float64       `json:"similarity"`
	Kind         DuplicateKind `json:"kind"`
	Reason       string        `json:"reason"`
}

var versionSuffixPattern = regexp.MustCompile(`(?i)\s*[\[(\-–—]\s*(live|remix|mix|radio edit|edit|remaster(?:ed)?|acoustic|demo|instrumental|mono|stereo|version).*?[\])]?\s*$`)
var versionTermPattern = regexp.MustCompile(`(?i)\b(live|remix|mix|radio edit|edit|remaster(?:ed)?|acoustic|demo|instrumental|mono|stereo|version)\b`)

func DetectDuplicateSongs(items []SongVector, threshold float64) []DuplicateCandidate {
	if threshold <= 0 || threshold > 1 {
		threshold = 0.92
	}
	candidates := make([]DuplicateCandidate, 0)
	for left := 0; left < len(items); left++ {
		for right := left + 1; right < len(items); right++ {
			first, second := items[left].Song, items[right].Song
			if !strings.EqualFold(strings.TrimSpace(first.Artist), strings.TrimSpace(second.Artist)) {
				continue
			}
			similarity := cosineSimilarity(items[left].Vector, items[right].Vector)
			if similarity < threshold {
				continue
			}
			baseFirst, baseSecond := baseVersionTitle(first.Title), baseVersionTitle(second.Title)
			sameBase := baseFirst != "" && baseFirst == baseSecond
			durationDelta := math.Abs(first.Duration - second.Duration)
			alternate := sameBase && (versionTermPattern.MatchString(first.Title) || versionTermPattern.MatchString(second.Title) || durationDelta > 3)
			if !sameBase && similarity < max(threshold, 0.97) {
				continue
			}
			kind := DuplicateExact
			reason := "Very similar audio/text embedding with matching artist and title"
			if alternate {
				kind = DuplicateAlternate
				reason = "Matching base title with version marker or meaningful duration difference"
			}
			candidates = append(candidates, DuplicateCandidate{
				FirstSongID: first.SongID, SecondSongID: second.SongID,
				FirstTitle: first.Title, SecondTitle: second.Title, Artist: first.Artist,
				Similarity: math.Round(similarity*10000) / 10000, Kind: kind, Reason: reason,
			})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Similarity > candidates[j].Similarity })
	return candidates
}

func baseVersionTitle(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = versionSuffixPattern.ReplaceAllString(value, "")
	return normalizeLex(value)
}

func cosineSimilarity(first, second []float32) float64 {
	if len(first) == 0 || len(first) != len(second) {
		return 0
	}
	dot, firstNorm, secondNorm := 0.0, 0.0, 0.0
	for index := range first {
		a, b := float64(first[index]), float64(second[index])
		dot += a * b
		firstNorm += a * a
		secondNorm += b * b
	}
	if firstNorm == 0 || secondNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(firstNorm) * math.Sqrt(secondNorm))
}
