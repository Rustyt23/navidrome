package nativeapi

import "testing"

func TestNormalizeMBString(t *testing.T) {
	got := normalizeMBString("  AC/DC - Live!  ")
	if got != "ac dc live" {
		t.Fatalf("unexpected normalized value: %q", got)
	}
}

func TestSelectBestRecording_Strict(t *testing.T) {
	recordings := []mbRecording{
		{Title: "Song A", Score: "90", ArtistCredit: []mbArtistCredit{{Name: "Wrong Artist"}}},
		{Title: "Song A (Live)", Score: "99", ArtistCredit: []mbArtistCredit{{Name: "The Artist"}}},
		{Title: "Song A", Score: "91", ArtistCredit: []mbArtistCredit{{Name: "The Artist"}}},
	}

	rec, ok := selectBestRecording(recordings, "Song A", "the artist")
	if !ok {
		t.Fatal("expected strict recording match")
	}
	if rec.Title != "Song A" {
		t.Fatalf("unexpected selected recording: %+v", rec)
	}
}

func TestSelectBestRelease_StrictScoring(t *testing.T) {
	releases := []mbRelease{
		{Title: "Live Cut", Date: "1990", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", SecondaryType: []string{"Live"}, FirstRelease: "1990-01-01"}},
		{Title: "Single Cut", Date: "2003", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Single", FirstRelease: "2003-01-01"}},
		{Title: "Album Cut", Date: "2001", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", FirstRelease: "2001-01-01"}},
	}

	rel, ok := selectBestRelease(releases, "")
	if !ok {
		t.Fatal("expected release")
	}
	if rel.Title != "Album Cut" {
		t.Fatalf("expected Album Cut, got %q", rel.Title)
	}
}

func TestCollectGenres_CountThreshold(t *testing.T) {
	genres := collectGenres([]mbTag{{Name: "Rock", Count: 2}, {Name: "Pop", Count: 1}}, []mbTag{{Name: "Alt Rock", Count: 3}})
	if genres != "rock, alt rock" {
		t.Fatalf("unexpected genres: %q", genres)
	}
}
