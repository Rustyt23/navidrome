package rag

import (
	"fmt"
	"strings"
)

// SearchFilters contains the optional exact and numeric constraints that can
// be applied on top of semantic song search. Pointer fields distinguish an
// omitted filter from an explicit false or zero value.
type SearchFilters struct {
	Explicit string `json:"explicit,omitempty"`
	Genre    string `json:"genre,omitempty"`
	Mood     string `json:"mood,omitempty"`
	// LyricsContains restricts results to songs whose lyrics contain the given
	// word or phrase (a Qdrant full-text match on the lyricsText payload).
	LyricsContains string   `json:"lyricsContains,omitempty"`
	YearMin        *int     `json:"yearMin,omitempty"`
	YearMax        *int     `json:"yearMax,omitempty"`
	BPMMin         *float64 `json:"bpmMin,omitempty"`
	BPMMax         *float64 `json:"bpmMax,omitempty"`
	LUFSMin        *float64 `json:"lufsMin,omitempty"`
	LUFSMax        *float64 `json:"lufsMax,omitempty"`
	PlayCountMin   *int64   `json:"playCountMin,omitempty"`
	PlayCountMax   *int64   `json:"playCountMax,omitempty"`
	DurationMin    *float64 `json:"durationMin,omitempty"`
	DurationMax    *float64 `json:"durationMax,omitempty"`
	HasLyrics      *bool    `json:"hasLyrics,omitempty"`
	HasGenre       *bool    `json:"hasGenre,omitempty"`
	HasYear        *bool    `json:"hasYear,omitempty"`
	HasBPM         *bool    `json:"hasBpm,omitempty"`
	HasLUFS        *bool    `json:"hasLufs,omitempty"`
}

// maxLyricsContainsRunes bounds the lyric phrase filter so a whole pasted
// lyric sheet cannot be used as an exact-match condition.
const maxLyricsContainsRunes = 200

// The tri-state a song's explicit rating can be in. "unknown" is a real state,
// not a synonym for clean: it means nobody has classified the song yet, so it
// must be excluded from clean-only results rather than assumed safe.
const (
	ExplicitStatusExplicit = "explicit"
	ExplicitStatusClean    = "clean"
	ExplicitStatusUnknown  = "unknown"
)

// NormalizeExplicitStatus maps the stored single-letter status, the spelled-out
// form, and anything unrecognized onto the tri-state. An empty or unrecognized
// value is "unknown" so a missing rating can never read as clean.
func NormalizeExplicitStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "e", ExplicitStatusExplicit:
		return ExplicitStatusExplicit
	case "c", ExplicitStatusClean:
		return ExplicitStatusClean
	default:
		return ExplicitStatusUnknown
	}
}

// ExplicitStatusFilterValue converts a requested filter into the payload value
// to match, or "" when no explicit filter was requested. Only the two decided
// states are filterable; asking for "unknown" is supported so operators can
// audit what still needs classifying.
func ExplicitStatusFilterValue(filter string) string {
	switch strings.ToLower(strings.TrimSpace(filter)) {
	case ExplicitStatusClean:
		return ExplicitStatusClean
	case ExplicitStatusExplicit:
		return ExplicitStatusExplicit
	case ExplicitStatusUnknown:
		return ExplicitStatusUnknown
	default:
		return ""
	}
}

// NormalizeSearchFilters trims string values and validates bounded ranges.
func NormalizeSearchFilters(filters SearchFilters) (SearchFilters, error) {
	filters.Explicit = strings.ToLower(strings.TrimSpace(filters.Explicit))
	filters.Genre = strings.TrimSpace(filters.Genre)
	filters.Mood = strings.TrimSpace(filters.Mood)
	filters.LyricsContains = strings.TrimSpace(filters.LyricsContains)
	if len([]rune(filters.LyricsContains)) > maxLyricsContainsRunes {
		return SearchFilters{}, fmt.Errorf("lyricsContains must not exceed %d characters", maxLyricsContainsRunes)
	}
	if filters.Explicit != "" && ExplicitStatusFilterValue(filters.Explicit) == "" {
		return SearchFilters{}, fmt.Errorf("explicit must be clean, explicit, or unknown")
	}
	if filters.YearMin != nil && filters.YearMax != nil && *filters.YearMin > *filters.YearMax {
		return SearchFilters{}, fmt.Errorf("yearMin must not exceed yearMax")
	}
	if filters.BPMMin != nil && filters.BPMMax != nil && *filters.BPMMin > *filters.BPMMax {
		return SearchFilters{}, fmt.Errorf("bpmMin must not exceed bpmMax")
	}
	if filters.LUFSMin != nil && filters.LUFSMax != nil && *filters.LUFSMin > *filters.LUFSMax {
		return SearchFilters{}, fmt.Errorf("lufsMin must not exceed lufsMax")
	}
	if (filters.PlayCountMin != nil && *filters.PlayCountMin < 0) || (filters.PlayCountMax != nil && *filters.PlayCountMax < 0) {
		return SearchFilters{}, fmt.Errorf("play count filters must not be negative")
	}
	if filters.PlayCountMin != nil && filters.PlayCountMax != nil && *filters.PlayCountMin > *filters.PlayCountMax {
		return SearchFilters{}, fmt.Errorf("playCountMin must not exceed playCountMax")
	}
	if (filters.DurationMin != nil && *filters.DurationMin < 0) || (filters.DurationMax != nil && *filters.DurationMax < 0) {
		return SearchFilters{}, fmt.Errorf("duration filters must not be negative")
	}
	if filters.DurationMin != nil && filters.DurationMax != nil && *filters.DurationMin > *filters.DurationMax {
		return SearchFilters{}, fmt.Errorf("durationMin must not exceed durationMax")
	}
	return filters, nil
}

// BuildQdrantFilter converts public search filters to Qdrant's payload-filter
// shape. An empty filter returns nil so unfiltered Phase 1 searches retain the
// same request semantics.
func BuildQdrantFilter(filters SearchFilters) map[string]any {
	must := make([]any, 0, 12)
	addMatch := func(key string, value any) {
		must = append(must, map[string]any{
			"key":   key,
			"match": map[string]any{"value": value},
		})
	}
	addRange := func(key string, rangeValue map[string]any) {
		if len(rangeValue) == 0 {
			return
		}
		must = append(must, map[string]any{"key": key, "range": rangeValue})
	}

	// Matched against the tri-state explicitStatus payload, never the legacy
	// boolean: `explicit` is false for both a verified-clean song and one that
	// was never classified, so filtering on it silently admitted unrated songs
	// into clean-only results such as retail-safe picks. A song whose status is
	// unknown now matches neither "clean" nor "explicit".
	if status := ExplicitStatusFilterValue(filters.Explicit); status != "" {
		addMatch("explicitStatus", status)
	}
	if filters.Genre != "" {
		addMatch("genres", filters.Genre)
	}
	if filters.Mood != "" {
		addMatch("moods", filters.Mood)
	}
	if filters.LyricsContains != "" {
		// Full-text match: with the lyricsText index this requires every token of
		// the phrase to appear in the song's lyrics.
		must = append(must, map[string]any{
			"key":   "lyricsText",
			"match": map[string]any{"text": filters.LyricsContains},
		})
	}
	for _, exact := range []struct {
		key   string
		value *bool
	}{
		{"hasLyrics", filters.HasLyrics},
		{"hasGenre", filters.HasGenre},
		{"hasYear", filters.HasYear},
		{"hasBpm", filters.HasBPM},
		{"hasLufs", filters.HasLUFS},
	} {
		if exact.value != nil {
			addMatch(exact.key, *exact.value)
		}
	}

	yearRange := map[string]any{}
	if filters.YearMin != nil {
		yearRange["gte"] = *filters.YearMin
	}
	if filters.YearMax != nil {
		yearRange["lte"] = *filters.YearMax
	}
	addRange("year", yearRange)

	bpmRange := map[string]any{}
	if filters.BPMMin != nil {
		bpmRange["gte"] = *filters.BPMMin
	}
	if filters.BPMMax != nil {
		bpmRange["lte"] = *filters.BPMMax
	}
	addRange("bpm", bpmRange)

	lufsRange := map[string]any{}
	if filters.LUFSMin != nil {
		lufsRange["gte"] = *filters.LUFSMin
	}
	if filters.LUFSMax != nil {
		lufsRange["lte"] = *filters.LUFSMax
	}
	addRange("lufs", lufsRange)

	playCountRange := map[string]any{}
	if filters.PlayCountMin != nil {
		playCountRange["gte"] = *filters.PlayCountMin
	}
	if filters.PlayCountMax != nil {
		playCountRange["lte"] = *filters.PlayCountMax
	}
	addRange("playCount", playCountRange)

	durationRange := map[string]any{}
	if filters.DurationMin != nil {
		durationRange["gte"] = *filters.DurationMin
	}
	if filters.DurationMax != nil {
		durationRange["lte"] = *filters.DurationMax
	}
	addRange("duration", durationRange)

	if len(must) == 0 {
		return nil
	}
	return map[string]any{"must": must}
}
