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
			Score: "100",
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

func TestSelectBestReleaseFromRecording(t *testing.T) {
	releases := []mbRelease{
		{ID: "bootleg", Title: "Bootleg", Status: "Bootleg", ReleaseGroup: mbGroup{PrimaryType: "Album"}},
		{ID: "comp", Title: "Compilation", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", SecondaryType: []string{"Compilation"}}},
		{ID: "remix", Title: "Remix", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album", SecondaryType: []string{"Remix"}}},
		{ID: "video", Title: "Video", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Single"}, Media: []mbMedium{{Format: "DVD-Video"}}},
		{ID: "ep", Title: "EP", Status: "Official", Date: "2010-01-01", ReleaseGroup: mbGroup{PrimaryType: "EP"}},
		{ID: "album", Title: "Album", Status: "Official", Date: "2010-01-01", ReleaseGroup: mbGroup{PrimaryType: "Album"}},
		{ID: "single", Title: "Single", Status: "Official", Date: "2010-01-01", ReleaseGroup: mbGroup{PrimaryType: "Single"}},
	}

	best := selectBestReleaseFromRecording(releases)
	if best == nil {
		t.Fatal("expected a release")
	}
	if best.ID != "single" {
		t.Fatalf("expected single to be prioritized, got %q", best.ID)
	}
}

func TestSelectBestReleaseFromRecordingPrefersEarliestDate(t *testing.T) {
	releases := []mbRelease{
		{ID: "later", Title: "Later", Status: "Official", Country: "US", Date: "2011-01-01", ReleaseGroup: mbGroup{PrimaryType: "Album"}},
		{ID: "earlier", Title: "Earlier", Status: "Official", Country: "GB", Date: "2010-01-01", ReleaseGroup: mbGroup{PrimaryType: "Album"}},
	}

	best := selectBestReleaseFromRecording(releases)
	if best == nil {
		t.Fatal("expected a release")
	}
	if best.ID != "earlier" {
		t.Fatalf("expected earliest date to be preferred, got %q", best.ID)
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
		{ID: "official", Score: "100", ArtistCredit: []struct {
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

func TestSelectBestRecordingPrefersClosestDurationWhenProvided(t *testing.T) {
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

func TestSelectBestRecordingRejectsVeryLowScore(t *testing.T) {
	recordings := []mbRecording{
		{ID: "low", Score: "50", ArtistCredit: []struct {
			Name string `json:"name"`
		}{{Name: "The Artist"}}, Releases: []mbRelease{{Title: "Album", Status: "Official", ReleaseGroup: mbGroup{PrimaryType: "Album"}}}},
	}

	rec := selectBestRecording(recordings, "the artist", 0)
	if rec != nil {
		t.Fatalf("expected no recording for very low score, got %q", rec.ID)
	}
}

func TestArtistCreditLooselyMatches(t *testing.T) {
	credits := []struct {
		Name string `json:"name"`
	}{{Name: "Edward Sharpe & the Magnetic Zeros"}}

	if !artistCreditLooselyMatches(credits, normalizeMBString("Edward Sharpe and the Magnetic Zeros")) {
		t.Fatal("expected loose artist matching to succeed")
	}
}
