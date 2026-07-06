package rag

import "testing"

func TestHybridRerankLiftsExactMatch(t *testing.T) {
	// The exact-title match is ranked last by the dense score; hybrid rerank must
	// pull it to the top because the query matches its title verbatim.
	results := []SongSearchResult{
		{SongID: "a", Title: "Something Else", Artist: "Other", Score: 0.90},
		{SongID: "b", Title: "Random Track", Artist: "Nobody", Score: 0.85},
		{SongID: "c", Title: "Bohemian Rhapsody", Artist: "Queen", Score: 0.60},
	}
	reranked := hybridRerank("bohemian rhapsody", results, 3)
	if reranked[0].SongID != "c" {
		t.Fatalf("expected exact title match first, got %q", reranked[0].SongID)
	}
}

func TestHybridRerankCapsTopK(t *testing.T) {
	results := []SongSearchResult{
		{SongID: "a", Title: "One", Score: 0.9},
		{SongID: "b", Title: "Two", Score: 0.8},
		{SongID: "c", Title: "Three", Score: 0.7},
	}
	if got := hybridRerank("something", results, 2); len(got) != 2 {
		t.Fatalf("expected topK=2, got %d", len(got))
	}
}

func TestFilterByMinScore(t *testing.T) {
	results := []SongSearchResult{
		{SongID: "a", Score: 0.8},
		{SongID: "b", Score: 0.2},
	}
	if got := FilterByMinScore(results, 0); len(got) != 2 {
		t.Fatalf("min 0 should disable filtering, got %d", len(got))
	}
	got := FilterByMinScore(results, 0.5)
	if len(got) != 1 || got[0].SongID != "a" {
		t.Fatalf("expected only high-score result, got %+v", got)
	}
}

func TestSelectDiverseRecommendationsCapsArtists(t *testing.T) {
	items := []RecommendationResult{
		{SongID: "1", Artist: "A"},
		{SongID: "2", Artist: "A"},
		{SongID: "3", Artist: "A"},
		{SongID: "4", Artist: "B"},
		{SongID: "5", Artist: "C"},
	}
	got := selectDiverseRecommendations(items, 4)
	if len(got) != 3 {
		t.Fatalf("expected one result per available artist, got %d", len(got))
	}
	if got[0].SongID != "1" || got[1].SongID != "4" || got[2].SongID != "5" {
		t.Fatalf("expected relevance order with artist overflow removed, got %+v", got)
	}
}

func TestSelectDiverseRecommendationsUsesExpandedPool(t *testing.T) {
	items := []RecommendationResult{
		{SongID: "1", Artist: "A"},
		{SongID: "2", Artist: "A"},
		{SongID: "3", Artist: "B"},
		{SongID: "4", Artist: "C"},
		{SongID: "5", Artist: "D"},
	}
	got := selectDiverseRecommendations(items, 4)
	if len(got) != 4 || got[3].SongID != "5" {
		t.Fatalf("expected lower-ranked diverse candidate to fill the limit, got %+v", got)
	}
}
