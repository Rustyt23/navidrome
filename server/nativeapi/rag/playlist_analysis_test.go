package rag

import (
	"context"
	"strings"
	"testing"

	"github.com/navidrome/navidrome/model"
)

func analysisSong(id, title, artist, genre, explicit string, bpm int, lufs string) model.MediaFile {
	tags := model.Tags{}
	if lufs != "" {
		tags["lufs"] = []string{lufs}
	}
	return model.MediaFile{
		ID: id, Title: title, Artist: artist, Genre: genre, ExplicitStatus: explicit,
		Year: 2020, BPM: bpm, Duration: 180, Lyrics: `[{"lang":"eng","line":[{"value":"lyrics"}]}]`, Tags: tags,
	}
}

func TestAnalyzePlaylistDetectsDuplicatesMetadataAndOutliers(t *testing.T) {
	base := analysisSong("song-1", "One", "Repeated Artist", "Pop", "c", 120, "-12")
	missing := model.MediaFile{ID: "song-2", Title: "Missing", Artist: "Repeated Artist", Duration: 180}
	playlist := &model.Playlist{ID: "playlist-1", Name: "Test", Tracks: model.PlaylistTracks{
		{MediaFile: base},
		{MediaFile: base},
		{MediaFile: analysisSong("song-3", "Explicit", "Repeated Artist", "Pop", "e", 122, "-11.5")},
		{MediaFile: analysisSong("song-4", "Loud", "Other", "Metal", "c", 190, "-4")},
		{MediaFile: missing},
	}}

	analysis := AnalyzePlaylist(context.Background(), playlist, nil)
	if len(analysis.DuplicateSongs) != 1 || analysis.DuplicateSongs[0].Count != 2 {
		t.Fatalf("expected duplicate detection, got %+v", analysis.DuplicateSongs)
	}
	if len(analysis.MetadataIssues) != 1 || len(analysis.MetadataIssues[0].Missing) != 5 {
		t.Fatalf("expected all metadata gaps, got %+v", analysis.MetadataIssues)
	}
	if analysis.ExplicitRisk.Count != 1 || analysis.ExplicitRisk.Level != "high" {
		t.Fatalf("unexpected explicit risk: %+v", analysis.ExplicitRisk)
	}
	if len(analysis.LoudnessIssues) != 1 || analysis.LoudnessIssues[0].SongID != "song-4" {
		t.Fatalf("expected LUFS outlier, got %+v", analysis.LoudnessIssues)
	}
	if len(analysis.BadFitSongs) == 0 {
		t.Fatal("expected bad-fit songs")
	}
}

func TestAnalyzePlaylistSuggestsFilteredCleanReplacements(t *testing.T) {
	explicit := analysisSong("explicit", "Risk", "Artist", "Pop", "e", 120, "-12")
	playlist := &model.Playlist{ID: "playlist-1", Name: "Clean Store", Tracks: model.PlaylistTracks{{MediaFile: explicit}}}
	var received SearchFilters
	search := func(_ context.Context, query string, topK int, filters SearchFilters) ([]SongSearchResult, error) {
		received = filters
		if !strings.Contains(query, "Risk") || topK != playlistReplacementTopK {
			t.Fatalf("unexpected replacement query: %q topK=%d", query, topK)
		}
		return []SongSearchResult{
			{SongID: "explicit", Title: "Risk", Explicit: false},
			{SongID: "replacement", Title: "Safe", Artist: "Other", Genre: "Pop", BPM: 121, LUFS: -12.2, Score: .91},
		}, nil
	}

	analysis := AnalyzePlaylist(context.Background(), playlist, search)
	if received.Explicit != "clean" || received.Genre != "Pop" || received.BPMMin == nil || *received.BPMMin != 105 || received.LUFSMax == nil || *received.LUFSMax != -10 {
		t.Fatalf("unexpected replacement filters: %+v", received)
	}
	if len(analysis.SuggestedReplacements) != 1 || len(analysis.SuggestedReplacements[0].Suggestions) != 1 || analysis.SuggestedReplacements[0].Suggestions[0].SongID != "replacement" {
		t.Fatalf("unexpected replacement suggestions: %+v", analysis.SuggestedReplacements)
	}
}
