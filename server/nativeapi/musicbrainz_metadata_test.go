package nativeapi

import "testing"

func TestNormalizeMBString(t *testing.T) {
	got := normalizeMBString("  AC/DC - Live!  ")
	if got != "ac dc live" {
		t.Fatalf("unexpected normalized value: %q", got)
	}
}

func TestSelectBestRecording(t *testing.T) {
	payload := mbSearchResponse{Recordings: []struct {
		Score            mbScore `json:"score"`
		FirstReleaseDate string  `json:"first-release-date"`
		ArtistCredit     []struct {
			Name string `json:"name"`
		} `json:"artist-credit"`
		Releases []struct {
			Title        string   `json:"title"`
			Date         string   `json:"date"`
			Status       string   `json:"status"`
			ReleaseGroup mbGroup  `json:"release-group"`
			Tags         []mbName `json:"tags"`
			Genres       []mbName `json:"genres"`
		} `json:"releases"`
		Tags   []mbName `json:"tags"`
		Genres []mbName `json:"genres"`
	}{
		{
			Score: "95",
			ArtistCredit: []struct {
				Name string `json:"name"`
			}{{Name: "Wrong Artist"}},
			Releases: []struct {
				Title        string   `json:"title"`
				Date         string   `json:"date"`
				Status       string   `json:"status"`
				ReleaseGroup mbGroup  `json:"release-group"`
				Tags         []mbName `json:"tags"`
				Genres       []mbName `json:"genres"`
			}{{Title: "Wrong Album", Date: "2010-01-01", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album"}}},
		},
		{
			Score: "92",
			ArtistCredit: []struct {
				Name string `json:"name"`
			}{{Name: "The Artist"}},
			Releases: []struct {
				Title        string   `json:"title"`
				Date         string   `json:"date"`
				Status       string   `json:"status"`
				ReleaseGroup mbGroup  `json:"release-group"`
				Tags         []mbName `json:"tags"`
				Genres       []mbName `json:"genres"`
			}{{Title: "Best Album", Date: "2001-01-01", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album"}}},
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
	releases := []struct {
		Title        string   `json:"title"`
		Date         string   `json:"date"`
		Status       string   `json:"status"`
		ReleaseGroup mbGroup  `json:"release-group"`
		Tags         []mbName `json:"tags"`
		Genres       []mbName `json:"genres"`
	}{
		{Title: "Live Cut", Date: "1990", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", SecondaryType: []string{"Live"}}},
		{Title: "Single Cut", Date: "2003", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Single"}},
		{Title: "Album Cut", Date: "2001", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album"}},
	}

	rel := selectBestRelease(releases)
	if rel == nil {
		t.Fatal("expected release")
	}
	if rel.Title != "Album Cut" {
		t.Fatalf("expected Album Cut, got %q", rel.Title)
	}
}
