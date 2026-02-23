package nativeapi

import "testing"

func TestSpotifyMatchValid(t *testing.T) {
	ok := spotifyMatchValid("Amidinine", "Bombino", &SpotifyTrackResult{TrackName: "Amidinine", Artist: "Bombino"})
	if !ok {
		t.Fatal("expected exact match to be valid")
	}

	ok = spotifyMatchValid("Amidine", "Bombino", &SpotifyTrackResult{TrackName: "Amidinine", Artist: "Bombino"})
	if !ok {
		t.Fatal("expected close title match to be valid")
	}

	ok = spotifyMatchValid("Totally Different", "Bombino", &SpotifyTrackResult{TrackName: "Amidinine", Artist: "Bombino"})
	if ok {
		t.Fatal("expected distant title mismatch to be invalid")
	}

	ok = spotifyMatchValid("Amidinine", "Bombino", &SpotifyTrackResult{TrackName: "Amidinine", Artist: "Another Artist"})
	if ok {
		t.Fatal("expected artist mismatch to be invalid")
	}
}

func TestLevenshteinDistance(t *testing.T) {
	if got := levenshteinDistance("kitten", "sitting"); got != 3 {
		t.Fatalf("expected distance 3, got %d", got)
	}
	if got := levenshteinDistance("", "abc"); got != 3 {
		t.Fatalf("expected distance 3, got %d", got)
	}
}
