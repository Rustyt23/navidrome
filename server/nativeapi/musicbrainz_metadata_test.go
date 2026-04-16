package nativeapi

import (
	"testing"

	"github.com/navidrome/navidrome/model"
)

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

	candidates := collectReleaseCandidates(recordings, "the artist")
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

	candidates := collectReleaseCandidates(recordings, "the artist")
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

func TestHasMissingCoverArt(t *testing.T) {
	tests := []struct {
		name string
		mf   model.MediaFile
		want bool
	}{
		{
			name: "missing when embedded and external cover are absent",
			mf:   model.MediaFile{},
			want: true,
		},
		{
			name: "not missing when embedded cover exists",
			mf: model.MediaFile{
				HasCoverArt: true,
			},
			want: false,
		},
		{
			name: "not missing when external cover path exists",
			mf: model.MediaFile{
				CoverPath: "covers/release.jpg",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasMissingCoverArt(tt.mf); got != tt.want {
				t.Fatalf("hasMissingCoverArt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSanitizeSpotifyRetryTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  string
	}{
		{name: "removes parenthesized text", title: "Song Title (Live)", want: "Song Title"},
		{name: "removes bracketed text", title: "Song Title [Remastered 2019]", want: "Song Title"},
		{name: "removes mixed suffixes", title: "Song (Live) [Remastered]", want: "Song"},
		{name: "keeps original when fully stripped", title: "(Live)", want: "(Live)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeSpotifyRetryTitle(tt.title); got != tt.want {
				t.Fatalf("sanitizeSpotifyRetryTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}
