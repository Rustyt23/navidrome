package rag

import (
	"testing"

	"github.com/navidrome/navidrome/model"
)

func TestBuildLibraryAnalyticsReportUsesCrossFilteredCounts(t *testing.T) {
	songs := model.MediaFiles{
		{ID: "1", Artist: "Artist A", Genre: "Rock", Year: 2015, ExplicitStatus: "c", Lyrics: `[{"line":[{"value":"hello"}]}]`},
		{ID: "2", Artist: "Artist A", Genre: "Rock", Year: 2015, ExplicitStatus: "e"},
		{ID: "3", Artist: "Artist B", Genre: "Pop", Year: 2016, ExplicitStatus: "c"},
	}
	year := 2015
	report := BuildLibraryAnalyticsReport(songs, SearchFilters{Explicit: "clean", YearMin: &year, YearMax: &year})
	if report.TotalSongs != 3 || report.MatchingSongs != 1 || report.CleanSongs != 1 || report.SongsWithLyrics != 1 {
		t.Fatalf("unexpected filtered analytics: %+v", report)
	}
	if len(report.TopArtists) != 1 || report.TopArtists[0].Name != "Artist A" || report.TopArtists[0].Count != 1 {
		t.Fatalf("unexpected artist analytics: %+v", report.TopArtists)
	}
}
