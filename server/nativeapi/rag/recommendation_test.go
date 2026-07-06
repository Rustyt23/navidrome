package rag

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/navidrome/navidrome/model"
)

func recommendationResult(id string, plays int64, score float64) SongSearchResult {
	return SongSearchResult{
		SongID: id, Title: "Song " + id, Artist: "Artist", Album: "Album", Genre: "Pop",
		Year: 2022, BPM: 120, LUFS: -12, PlayCount: plays, Score: score,
	}
}

func TestRecommendSimilarSongsUsesSongDocumentAndProfile(t *testing.T) {
	song := model.MediaFile{
		ID: "seed", Title: "Seed Song", Artist: "Seed Artist", Album: "Seed Album",
		Genre: "Pop", BPM: 120, Tags: model.Tags{"lufs": {"-12"}},
	}
	var query string
	var filters SearchFilters
	search := func(_ context.Context, value string, _ int, valueFilters SearchFilters) ([]SongSearchResult, error) {
		query, filters = value, valueFilters
		return []SongSearchResult{recommendationResult("seed", 1, .99), recommendationResult("other", 3, .91)}, nil
	}

	response, err := Recommend(context.Background(), RecommendationInput{
		Type: RecommendationSimilarSongs, Limit: 10, Song: &song,
	}, search)
	if err != nil {
		t.Fatalf("recommend similar songs: %v", err)
	}
	if !strings.Contains(query, "Title: Seed Song") || !strings.Contains(query, "Artist: Seed Artist") {
		t.Fatalf("expected song document query, got %q", query)
	}
	if filters.Genre != "Pop" || filters.BPMMin == nil || *filters.BPMMin != 100 || filters.LUFSMax == nil || *filters.LUFSMax != -9.5 {
		t.Fatalf("unexpected similarity filters: %+v", filters)
	}
	if response.Count != 1 || response.Results[0].SongID != "other" || !strings.Contains(response.Results[0].Reason, "Seed Song") {
		t.Fatalf("unexpected similar recommendations: %+v", response)
	}
}

func TestRecommendUnderusedSongsUsesCleanCompleteMetadataFilters(t *testing.T) {
	var filters SearchFilters
	search := func(_ context.Context, _ string, _ int, valueFilters SearchFilters) ([]SongSearchResult, error) {
		filters = valueFilters
		return []SongSearchResult{recommendationResult("ten", 10, .8), recommendationResult("two", 2, .7)}, nil
	}
	response, err := Recommend(context.Background(), RecommendationInput{Type: RecommendationUnderusedSongs, Limit: 5}, search)
	if err != nil {
		t.Fatalf("recommend underused songs: %v", err)
	}
	if filters.Explicit != "clean" || filters.PlayCountMax == nil || *filters.PlayCountMax != 20 ||
		filters.HasLyrics == nil || !*filters.HasLyrics || filters.HasGenre == nil || !*filters.HasGenre ||
		filters.HasYear == nil || !*filters.HasYear || filters.HasBPM == nil || !*filters.HasBPM || filters.HasLUFS == nil || !*filters.HasLUFS {
		t.Fatalf("unexpected underused filters: %+v", filters)
	}
	if response.Results[0].SongID != "two" || !strings.Contains(response.Results[0].Reason, "2 plays") {
		t.Fatalf("expected lowest play count first: %+v", response.Results)
	}
}

func TestRecommendOverplayedSongsRanksPlayCountAndExplainsRecency(t *testing.T) {
	recent := time.Now().Add(-24 * time.Hour)
	high := recommendationResult("high", 100, .7)
	high.LastPlayedAt = &recent
	low := recommendationResult("low", 10, .9)
	search := func(context.Context, string, int, SearchFilters) ([]SongSearchResult, error) {
		return []SongSearchResult{low, high}, nil
	}
	response, err := Recommend(context.Background(), RecommendationInput{Type: RecommendationOverplayedSongs, Limit: 5}, search)
	if err != nil {
		t.Fatalf("recommend overplayed songs: %v", err)
	}
	if response.Results[0].SongID != "high" || !strings.Contains(response.Results[0].Reason, "last 30 days") {
		t.Fatalf("unexpected overplayed ranking: %+v", response.Results)
	}
}

func TestRecommendPlaylistExpansionUsesSummaryAndExcludesCurrentSongs(t *testing.T) {
	playlist := &model.Playlist{
		ID: "playlist-1", Name: "Store Mix",
		Tracks: model.PlaylistTracks{{MediaFile: model.MediaFile{
			ID: "existing", Title: "Current", Artist: "Artist", Genre: "Pop",
			Tags: model.Tags{model.TagMood: {"Upbeat"}},
		}}},
	}
	var query string
	search := func(_ context.Context, value string, _ int, _ SearchFilters) ([]SongSearchResult, error) {
		query = value
		return []SongSearchResult{recommendationResult("existing", 1, .9), recommendationResult("new", 2, .8)}, nil
	}
	response, err := Recommend(context.Background(), RecommendationInput{
		Type: RecommendationPlaylistExpansion, Limit: 5, Playlist: playlist,
	}, search)
	if err != nil {
		t.Fatalf("recommend playlist expansion: %v", err)
	}
	if !strings.Contains(query, "Playlist: Store Mix") || !strings.Contains(query, "Genres: Pop") || !strings.Contains(query, "Moods: Upbeat") {
		t.Fatalf("expected playlist summary query, got %q", query)
	}
	if response.Count != 1 || response.Results[0].SongID != "new" {
		t.Fatalf("expected existing playlist song to be excluded: %+v", response)
	}
}

func TestRecommendPlaylistReplacementsUsesAnalysisSuggestions(t *testing.T) {
	playlist := &model.Playlist{
		ID: "playlist-1", Name: "Retail Mix",
		Tracks: model.PlaylistTracks{{MediaFile: model.MediaFile{
			ID: "risk", Title: "Risky", Artist: "Artist", Genre: "Pop", ExplicitStatus: "e",
		}}},
	}
	search := func(context.Context, string, int, SearchFilters) ([]SongSearchResult, error) {
		return []SongSearchResult{{
			SongID: "safe", Title: "Safe", Artist: "Other", Album: "Album", Genre: "Pop",
			Year: 2024, BPM: 120, LUFS: -12, PlayCount: 4, Score: .9,
		}}, nil
	}
	response, err := Recommend(context.Background(), RecommendationInput{
		Type: RecommendationPlaylistReplacements, Limit: 5, Playlist: playlist,
	}, search)
	if err != nil {
		t.Fatalf("recommend playlist replacements: %v", err)
	}
	if response.Count != 1 || response.Results[0].SongID != "safe" || !strings.Contains(response.Results[0].Reason, "Risky") {
		t.Fatalf("unexpected playlist replacement response: %+v", response)
	}
}

func TestRecommendRetailSafeSongsUsesSafetyAndLUFSFilters(t *testing.T) {
	var filters SearchFilters
	search := func(_ context.Context, _ string, _ int, valueFilters SearchFilters) ([]SongSearchResult, error) {
		filters = valueFilters
		return []SongSearchResult{recommendationResult("safe", 5, .9)}, nil
	}
	response, err := Recommend(context.Background(), RecommendationInput{Type: RecommendationRetailSafeSongs, Limit: 5}, search)
	if err != nil {
		t.Fatalf("recommend retail-safe songs: %v", err)
	}
	if filters.Explicit != "clean" || filters.LUFSMin == nil || *filters.LUFSMin != -14 || filters.LUFSMax == nil || *filters.LUFSMax != -10 ||
		filters.HasLyrics == nil || !*filters.HasLyrics || filters.HasGenre == nil || !*filters.HasGenre || filters.HasYear == nil || !*filters.HasYear ||
		filters.HasBPM == nil || !*filters.HasBPM || filters.HasLUFS == nil || !*filters.HasLUFS {
		t.Fatalf("unexpected retail-safe filters: %+v", filters)
	}
	if response.Count != 1 || !strings.Contains(response.Results[0].Reason, "retail-friendly") {
		t.Fatalf("unexpected retail-safe response: %+v", response)
	}
}

func TestRecommendCapsResultsPerArtist(t *testing.T) {
	search := func(context.Context, string, int, SearchFilters) ([]SongSearchResult, error) {
		results := []SongSearchResult{
			recommendationResult("a1", 1, .99),
			recommendationResult("a2", 2, .98),
			recommendationResult("a3", 3, .97),
			recommendationResult("b1", 4, .96),
			recommendationResult("c1", 5, .95),
			recommendationResult("d1", 6, .94),
		}
		results[3].Artist = "Artist B"
		results[4].Artist = "Artist C"
		results[5].Artist = "Artist D"
		return results, nil
	}
	response, err := Recommend(context.Background(), RecommendationInput{Type: RecommendationRetailSafeSongs, Limit: 4}, search)
	if err != nil {
		t.Fatalf("recommend retail-safe songs: %v", err)
	}
	if response.Count != 4 {
		t.Fatalf("expected expanded pool to provide four artists, got %+v", response.Results)
	}
	seen := map[string]bool{}
	for _, result := range response.Results {
		if seen[result.Artist] {
			t.Fatalf("artist cap was not enforced: %+v", response.Results)
		}
		seen[result.Artist] = true
	}
}
