package nativeapi

import "testing"

func TestNormalizeMBString(t *testing.T) {
	got := normalizeMBString("  AC/DC - Live!  ")
	if got != "ac dc live" {
		t.Fatalf("unexpected normalized value: %q", got)
	}
}

func TestNormalizeRecordingSearchTitle(t *testing.T) {
	got := normalizeRecordingSearchTitle(" 1998 (Sonny Alven Remix) feat. Alice [Live] ")
	if got != "1998" {
		t.Fatalf("unexpected normalized search title: %q", got)
	}
}

func TestSelectBestRecording(t *testing.T) {
	payload := mbSearchResponse{Recordings: []mbRecording{
		{
			Title:        "Track Name",
			Score:        "95",
			ArtistCredit: []mbCredit{{Name: "Wrong Artist"}},
			Releases:     []mbRelease{{Title: "Wrong Album", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", FirstReleaseDate: "2010-01-01"}}},
		},
		{
			Title:        "Track Name",
			Score:        "89",
			ArtistCredit: []mbCredit{{Name: "The Artist"}},
			Releases:     []mbRelease{{Title: "Good Album", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", FirstReleaseDate: "2003-01-01"}}},
		},
		{
			Title:        "Track Name",
			Score:        "92",
			ArtistCredit: []mbCredit{{Name: "The Artist"}},
			Releases:     []mbRelease{{Title: "Best Album", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", FirstReleaseDate: "2001-01-01"}}},
		},
		{
			Title:        "Track Name",
			Score:        "79",
			ArtistCredit: []mbCredit{{Name: "The Artist"}},
		},
	}}

	rec := selectBestRecording(payload.Recordings, "the artist", "track name")
	if rec == nil {
		t.Fatal("expected a recording")
	}
	if len(rec.Releases) == 0 || rec.Releases[0].Title != "Best Album" {
		t.Fatalf("unexpected selected recording: %+v", rec)
	}
}

func TestSelectBestRelease(t *testing.T) {
	releases := []mbRelease{
		{Title: "Compilation", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", SecondaryType: []string{"Compilation"}, FirstReleaseDate: "1990"}},
		{Title: "Live Cut", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", SecondaryType: []string{"Live"}, FirstReleaseDate: "1995"}},
		{Title: "Single Cut", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Single", FirstReleaseDate: "2003"}},
		{Title: "Album Cut", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", FirstReleaseDate: "2001"}},
	}

	rel := selectBestRelease(releases, "Track Name")
	if rel == nil {
		t.Fatal("expected release")
	}
	if rel.Title != "Album Cut" {
		t.Fatalf("expected Album Cut, got %q", rel.Title)
	}
}

func TestSelectBestRelease_AllowsLiveForLiveTracks(t *testing.T) {
	releases := []mbRelease{
		{Title: "Live Album", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", SecondaryType: []string{"Live"}, FirstReleaseDate: "2004"}},
		{Title: "Studio Album", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", SecondaryType: []string{"Compilation"}, FirstReleaseDate: "2001"}},
	}

	rel := selectBestRelease(releases, "My Song Live")
	if rel == nil {
		t.Fatal("expected live release for live track")
	}
	if rel.Title != "Live Album" {
		t.Fatalf("expected Live Album, got %q", rel.Title)
	}
}
