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

	rec := selectBestRecording(payload.Recordings, "the artist", 0)
	if rec == nil {
		t.Fatal("expected a recording")
	}
	if len(rec.Releases) == 0 || rec.Releases[0].Title != "Best Album" {
		t.Fatalf("unexpected selected recording: %+v", rec)
	}
}

func TestCollectReleaseCandidates(t *testing.T) {
	recordings := []mbRecording{{
		Score: "95",
		ArtistCredit: []struct {
			Name string `json:"name"`
		}{{Name: "The Artist"}},
		Releases: []mbRelease{
			{ID: "live", Title: "Live Cut", Date: "1990", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", SecondaryType: []string{"Live"}}},
			{ID: "comp", Title: "Compilation", Date: "1992", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", SecondaryType: []string{"Compilation"}}},
			{ID: "album", Title: "Studio Album", Date: "2001", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album"}},
		},
	}}

	candidates := collectReleaseCandidates(recordings, "the artist", 0)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 preferred candidate, got %d", len(candidates))
	}
	if candidates[0].release.ID != "album" {
		t.Fatalf("expected album candidate, got %q", candidates[0].release.ID)
	}
}

func TestCollectReleaseCandidatesFallsBackToOfficial(t *testing.T) {
	recordings := []mbRecording{{
		Score: "95",
		ArtistCredit: []struct {
			Name string `json:"name"`
		}{{Name: "The Artist"}},
		Releases: []mbRelease{
			{ID: "live", Title: "Live Cut", Date: "1990", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", SecondaryType: []string{"Live"}}},
			{ID: "single", Title: "Official Single", Date: "1991", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Single"}},
		},
	}}

	candidates := collectReleaseCandidates(recordings, "the artist", 0)
	if len(candidates) != 2 {
		t.Fatalf("expected official fallback candidates, got %d", len(candidates))
	}
}

func TestSelectBestReleaseCandidatePrefersCover(t *testing.T) {
	candidates := []releaseCandidate{
		{release: &mbRelease{ID: "a", Title: "No Cover", Date: "1980", Country: "US"}},
		{release: &mbRelease{ID: "b", Title: "Has Cover", Date: "2001", Country: "GB"}},
	}

	rel := selectBestReleaseCandidate(candidates, func(releaseID string) bool { return releaseID == "b" })
	if rel == nil {
		t.Fatal("expected release")
	}
	if rel.release.ID != "b" {
		t.Fatalf("expected release with cover, got %q", rel.release.ID)
	}
}

func TestSelectBestReleaseCandidatePrefersEarliestThenUS(t *testing.T) {
	candidates := []releaseCandidate{
		{release: &mbRelease{ID: "a", Title: "Album A", Date: "2001", Country: "GB"}},
		{release: &mbRelease{ID: "b", Title: "Album B", Date: "2001", Country: "US"}},
		{release: &mbRelease{ID: "c", Title: "Album C", Date: "1999", Country: "GB"}},
	}

	rel := selectBestReleaseCandidate(candidates, func(string) bool { return false })
	if rel == nil {
		t.Fatal("expected release")
	}
	if rel.release.ID != "c" {
		t.Fatalf("expected earliest release when no cover exists, got %q", rel.release.ID)
	}
}

func TestSelectBestRecordingFiltersExcludedAndPrefersOfficial(t *testing.T) {
	recordings := []mbRecording{
		{ID: "video", Score: "100", Video: true, ArtistCredit: []struct {
			Name string `json:"name"`
		}{{Name: "The Artist"}}, Releases: []mbRelease{{Title: "Video Album", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album"}}}},
		{ID: "acoustic", Score: "100", Disambiguation: "acoustic", ArtistCredit: []struct {
			Name string `json:"name"`
		}{{Name: "The Artist"}}, Releases: []mbRelease{{Title: "Acoustic Album", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album"}}}},
		{ID: "demo", Title: "Song (demo)", Score: "100", ArtistCredit: []struct {
			Name string `json:"name"`
		}{{Name: "The Artist"}}, Releases: []mbRelease{{Title: "Demo Album", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album"}}}},
		{ID: "non-official", Score: "100", ArtistCredit: []struct {
			Name string `json:"name"`
		}{{Name: "The Artist"}}, Releases: []mbRelease{{Title: "Unofficial", Status: "Bootleg", ReleaseGroup: mbGroup{PrimaryType: "Album"}}}},
		{ID: "official", Score: "99", ArtistCredit: []struct {
			Name string `json:"name"`
		}{{Name: "The Artist"}}, Releases: []mbRelease{{Title: "Official", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album"}}}},
	}

	rec := selectBestRecording(recordings, "the artist", 0)
	if rec == nil {
		t.Fatal("expected recording")
	}
	if rec.ID != "official" {
		t.Fatalf("expected official recording, got %q", rec.ID)
	}
}

func TestSelectBestRecordingPrefersClosestDurationWithinWindow(t *testing.T) {
	recordings := []mbRecording{
		{ID: "far", Score: "100", Length: 211000, ArtistCredit: []struct {
			Name string `json:"name"`
		}{{Name: "The Artist"}}, Releases: []mbRelease{{Title: "Album", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album"}}}},
		{ID: "close", Score: "100", Length: 212500, ArtistCredit: []struct {
			Name string `json:"name"`
		}{{Name: "The Artist"}}, Releases: []mbRelease{{Title: "Album", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album"}}}},
	}

	rec := selectBestRecording(recordings, "the artist", 213)
	if rec == nil {
		t.Fatal("expected recording")
	}
	if rec.ID != "close" {
		t.Fatalf("expected closest duration recording, got %q", rec.ID)
	}
}
