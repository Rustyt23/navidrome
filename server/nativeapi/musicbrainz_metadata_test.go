package nativeapi

import "testing"

func TestNormalizeMBString(t *testing.T) {
	got := normalizeMBString("  AC/DC - Live!  ")
	if got != "ac dc live" {
		t.Fatalf("unexpected normalized value: %q", got)
	}
}

func TestSelectBestRecording(t *testing.T) {
	payload := mbSearchResponse{Recordings: []mbRecording{
		{
			Score: "95",
			ArtistCredit: []struct {
				Name string `json:"name"`
			}{{Name: "Wrong Artist"}},
			Releases: []mbRelease{{Title: "Wrong Album", Date: "2010-01-01", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album"}}},
		},
		{
			Score: "92",
			ArtistCredit: []struct {
				Name string `json:"name"`
			}{{Name: "The Artist"}},
			Releases: []mbRelease{{Title: "Best Album", Date: "2001-01-01", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album"}}},
		},
		{
			Score: "79",
			ArtistCredit: []struct {
				Name string `json:"name"`
			}{{Name: "The Artist"}},
		},
	}}

	rec := selectBestRecording(payload.Recordings, "the artist")
	if rec == nil {
		t.Fatal("expected a recording")
	}
	if len(rec.Releases) == 0 || rec.Releases[0].Title != "Best Album" {
		t.Fatalf("unexpected selected recording: %+v", rec)
	}
}

func TestSelectBestRelease(t *testing.T) {
	releases := []mbRelease{
		{Title: "Live Cut", Date: "1990", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", SecondaryType: []string{"Live"}}},
		{Title: "US Album Cut", Date: "2001", Country: "US", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album"}},
		{Title: "Album Cut", Date: "2001", Country: "GB", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album"}},
	}

	rel := selectBestRelease(releases)
	if rel == nil {
		t.Fatal("expected release")
	}
	if rel.Title != "US Album Cut" {
		t.Fatalf("expected US Album Cut, got %q", rel.Title)
	}
}
