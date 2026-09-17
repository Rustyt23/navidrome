package rag

import (
	"context"
	"testing"
	"time"

	"github.com/navidrome/navidrome/model"
)

func dashboardSong(id, status string, plays int64) model.MediaFile {
	return model.MediaFile{
		ID: id, Title: id, Artist: "Artist", Genre: "Pop", Year: 2024, BPM: 120,
		ExplicitStatus: status, Lyrics: `[{"lang":"eng","line":[{"value":"line"}]}]`,
		Tags: model.Tags{"lufs": {"-12"}}, Annotations: model.Annotations{PlayCount: plays},
	}
}

func TestBuildDashboardReportCalculatesLibraryAndPlaylistHealth(t *testing.T) {
	songs := model.MediaFiles{
		dashboardSong("underused", "c", 2),
		{ID: "risk", Title: "Risk", Artist: "Artist", ExplicitStatus: "e"},
		dashboardSong("overplayed", "c", 75),
	}
	playlist := model.Playlist{
		ID: "playlist-1", Name: "Test",
		Tracks: model.PlaylistTracks{
			{MediaFile: songs[0]}, {MediaFile: songs[0]}, {MediaFile: songs[1]},
		},
	}
	report := BuildDashboardReport(
		context.Background(), songs, model.Playlists{playlist},
		DashboardRAGStatus{Enabled: true, VectorDBOnline: true, CollectionExists: true, IndexedSongs: 2},
		time.Now(),
	)
	if report.TotalSongs != 3 || report.IndexedSongs != 2 || report.IndexedCoveragePercent != 66.7 {
		t.Fatalf("unexpected library/index totals: %+v", report)
	}
	if report.SongsWithLyrics != 2 || report.SongsMissingLyrics != 1 || report.SongsWithGenre != 2 || report.SongsMissingGenre != 1 {
		t.Fatalf("unexpected metadata coverage: %+v", report)
	}
	if report.CleanSongs != 2 || report.ExplicitSongs != 1 || report.ExplicitRiskSongs != 1 || report.ReviewNeededSongs != 1 {
		t.Fatalf("unexpected explicit/review counts: %+v", report)
	}
	if report.UnderusedSongs != 1 || report.OverplayedSongs != 1 {
		t.Fatalf("unexpected usage insights: %+v", report)
	}
	if report.PlaylistsTotal != 1 || report.PlaylistsAnalyzed != 1 || report.PlaylistsWithIssues != 1 || report.DuplicateRiskCount != 1 {
		t.Fatalf("unexpected playlist quality: %+v", report)
	}
	if report.RecommendationsAvailable < 2 || report.LibraryHealthScore <= 0 || report.LibraryHealthScore > 100 {
		t.Fatalf("unexpected recommendations/health score: %+v", report)
	}
}

func TestBuildDashboardReportHandlesEmptyLibrary(t *testing.T) {
	report := BuildDashboardReport(context.Background(), nil, nil, DashboardRAGStatus{Enabled: true}, time.Now())
	if report.TotalSongs != 0 || report.LibraryHealthScore != 0 || report.IndexedCoveragePercent != 0 || report.PlaylistsTotal != 0 {
		t.Fatalf("unexpected empty report: %+v", report)
	}
}

func TestBuildDashboardReportTreatsOfflineQdrantAsNoCoverage(t *testing.T) {
	report := BuildDashboardReport(
		context.Background(), model.MediaFiles{dashboardSong("song", "c", 1)}, nil,
		DashboardRAGStatus{Enabled: true, VectorDBOnline: false, CollectionExists: true, IndexedSongs: 99, Error: "offline"},
		time.Now(),
	)
	if report.IndexedSongs != 0 || report.IndexedCoveragePercent != 0 || report.RAGError != "offline" {
		t.Fatalf("offline Qdrant must have zero coverage: %+v", report)
	}
}
