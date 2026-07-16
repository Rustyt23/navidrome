package rag

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildQdrantFilterExactFilters(t *testing.T) {
	truth := true
	falsehood := false
	filter := BuildQdrantFilter(SearchFilters{
		Explicit:  "clean",
		Genre:     "Pop",
		Mood:      "Energetic",
		HasLyrics: &truth,
		HasGenre:  &truth,
		HasYear:   &falsehood,
		HasBPM:    &truth,
		HasLUFS:   &truth,
	})
	want := map[string]any{"must": []any{
		map[string]any{"key": "explicit", "match": map[string]any{"value": false}},
		map[string]any{"key": "genres", "match": map[string]any{"value": "Pop"}},
		map[string]any{"key": "moods", "match": map[string]any{"value": "Energetic"}},
		map[string]any{"key": "hasLyrics", "match": map[string]any{"value": true}},
		map[string]any{"key": "hasGenre", "match": map[string]any{"value": true}},
		map[string]any{"key": "hasYear", "match": map[string]any{"value": false}},
		map[string]any{"key": "hasBpm", "match": map[string]any{"value": true}},
		map[string]any{"key": "hasLufs", "match": map[string]any{"value": true}},
	}}
	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("unexpected exact filter:\nwant %#v\n got %#v", want, filter)
	}
}

func TestBuildQdrantFilterRangeFilters(t *testing.T) {
	yearMin, yearMax := 2000, 2020
	bpmMin, bpmMax := 100.0, 140.0
	lufsMin, lufsMax := -13.5, -11.5
	playCountMin, playCountMax := int64(2), int64(20)
	durationMin, durationMax := 180.0, 240.0
	filter := BuildQdrantFilter(SearchFilters{
		YearMin: &yearMin, YearMax: &yearMax,
		BPMMin: &bpmMin, BPMMax: &bpmMax,
		LUFSMin: &lufsMin, LUFSMax: &lufsMax,
		PlayCountMin: &playCountMin, PlayCountMax: &playCountMax,
		DurationMin: &durationMin, DurationMax: &durationMax,
	})
	want := map[string]any{"must": []any{
		map[string]any{"key": "year", "range": map[string]any{"gte": 2000, "lte": 2020}},
		map[string]any{"key": "bpm", "range": map[string]any{"gte": 100.0, "lte": 140.0}},
		map[string]any{"key": "lufs", "range": map[string]any{"gte": -13.5, "lte": -11.5}},
		map[string]any{"key": "playCount", "range": map[string]any{"gte": int64(2), "lte": int64(20)}},
		map[string]any{"key": "duration", "range": map[string]any{"gte": 180.0, "lte": 240.0}},
	}}
	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("unexpected range filter:\nwant %#v\n got %#v", want, filter)
	}
}

func TestBuildQdrantFilterEmpty(t *testing.T) {
	if filter := BuildQdrantFilter(SearchFilters{}); filter != nil {
		t.Fatalf("expected no Qdrant filter, got %#v", filter)
	}
}

func TestNormalizeSearchFiltersLyricsContains(t *testing.T) {
	filters, err := NormalizeSearchFilters(SearchFilters{LyricsContains: "  rain  "})
	if err != nil || filters.LyricsContains != "rain" {
		t.Fatalf("expected trimmed lyricsContains, got %q err=%v", filters.LyricsContains, err)
	}
	long := strings.Repeat("a", maxLyricsContainsRunes+1)
	if _, err := NormalizeSearchFilters(SearchFilters{LyricsContains: long}); err == nil {
		t.Fatal("expected over-long lyricsContains to be rejected")
	}
}

func TestBuildQdrantFilterLyricsContains(t *testing.T) {
	filter := BuildQdrantFilter(SearchFilters{LyricsContains: "rain"})
	if filter == nil {
		t.Fatal("expected a filter")
	}
	must, _ := filter["must"].([]any)
	if len(must) != 1 {
		t.Fatalf("expected one condition, got %d", len(must))
	}
	condition, _ := must[0].(map[string]any)
	if condition["key"] != "lyricsText" {
		t.Fatalf("unexpected condition key: %+v", condition)
	}
	match, _ := condition["match"].(map[string]any)
	if match["text"] != "rain" {
		t.Fatalf("expected full-text match on rain, got %+v", match)
	}
}
