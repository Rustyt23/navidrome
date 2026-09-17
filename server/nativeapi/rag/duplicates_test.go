package rag

import "testing"

func TestDetectDuplicateSongsClassifiesAlternateVersion(t *testing.T) {
	items := []SongVector{
		{Song: IndexedSong{SongID: "studio", Title: "My Song", Artist: "Band", Duration: 210}, Vector: []float32{1, 0}},
		{Song: IndexedSong{SongID: "live", Title: "My Song (Live)", Artist: "Band", Duration: 260}, Vector: []float32{.99, .01}},
		{Song: IndexedSong{SongID: "other", Title: "Other", Artist: "Someone Else"}, Vector: []float32{1, 0}},
	}
	results := DetectDuplicateSongs(items, .92)
	if len(results) != 1 || results[0].Kind != DuplicateAlternate || results[0].FirstSongID != "studio" || results[0].SecondSongID != "live" {
		t.Fatalf("unexpected duplicate candidates: %+v", results)
	}
}

func TestDetectDuplicateSongsRejectsWeakOrDifferentArtistMatches(t *testing.T) {
	items := []SongVector{
		{Song: IndexedSong{SongID: "one", Title: "Song", Artist: "A"}, Vector: []float32{1, 0}},
		{Song: IndexedSong{SongID: "two", Title: "Song", Artist: "A"}, Vector: []float32{0, 1}},
		{Song: IndexedSong{SongID: "three", Title: "Song", Artist: "B"}, Vector: []float32{1, 0}},
	}
	if results := DetectDuplicateSongs(items, .92); len(results) != 0 {
		t.Fatalf("expected no duplicate candidates, got %+v", results)
	}
}
