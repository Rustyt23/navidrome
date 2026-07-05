package rag

import (
	"fmt"
	"strings"
)

// SearchFilters contains the optional exact and numeric constraints that can
// be applied on top of semantic song search. Pointer fields distinguish an
// omitted filter from an explicit false or zero value.
type SearchFilters struct {
	Explicit     string   `json:"explicit,omitempty"`
	Genre        string   `json:"genre,omitempty"`
	YearMin      *int     `json:"yearMin,omitempty"`
	YearMax      *int     `json:"yearMax,omitempty"`
	BPMMin       *float64 `json:"bpmMin,omitempty"`
	BPMMax       *float64 `json:"bpmMax,omitempty"`
	LUFSMin      *float64 `json:"lufsMin,omitempty"`
	LUFSMax      *float64 `json:"lufsMax,omitempty"`
	PlayCountMax *int64   `json:"playCountMax,omitempty"`
	DurationMax  *float64 `json:"durationMax,omitempty"`
	HasLyrics    *bool    `json:"hasLyrics,omitempty"`
	HasGenre     *bool    `json:"hasGenre,omitempty"`
	HasYear      *bool    `json:"hasYear,omitempty"`
	HasBPM       *bool    `json:"hasBpm,omitempty"`
	HasLUFS      *bool    `json:"hasLufs,omitempty"`
}

// NormalizeSearchFilters trims string values and validates bounded ranges.
func NormalizeSearchFilters(filters SearchFilters) (SearchFilters, error) {
	filters.Explicit = strings.ToLower(strings.TrimSpace(filters.Explicit))
	filters.Genre = strings.TrimSpace(filters.Genre)
	if filters.Explicit != "" && filters.Explicit != "clean" && filters.Explicit != "explicit" {
		return SearchFilters{}, fmt.Errorf("explicit must be clean or explicit")
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
	if filters.PlayCountMax != nil && *filters.PlayCountMax < 0 {
		return SearchFilters{}, fmt.Errorf("playCountMax must not be negative")
	}
	if filters.DurationMax != nil && *filters.DurationMax < 0 {
		return SearchFilters{}, fmt.Errorf("durationMax must not be negative")
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

	if filters.Explicit == "clean" {
		addMatch("explicit", false)
	} else if filters.Explicit == "explicit" {
		addMatch("explicit", true)
	}
	if filters.Genre != "" {
		addMatch("genre", filters.Genre)
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

	if filters.PlayCountMax != nil {
		addRange("playCount", map[string]any{"lte": *filters.PlayCountMax})
	}
	if filters.DurationMax != nil {
		addRange("duration", map[string]any{"lte": *filters.DurationMax})
	}

	if len(must) == 0 {
		return nil
	}
	return map[string]any{"must": must}
}
