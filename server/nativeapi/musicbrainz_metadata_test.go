package nativeapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
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

func TestSelectBestRecordingReleasePrefersCover(t *testing.T) {
	job := newMusicBrainzMetadataJob()
	job.coverClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "rel-no-cover") {
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		if strings.Contains(r.URL.Path, "rel-cover") {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}

	recordings := []mbRecording{{
		ID:    "rec-1",
		Score: "99",
		ArtistCredit: []struct {
			Name string `json:"name"`
		}{{Name: "Artist"}},
		Releases: []mbRelease{
			{ID: "rel-no-cover", Title: "No Cover", Date: "1999-01-01", Status: "Official", Country: "US", ReleaseGroup: mbGroup{PrimaryType: "Album"}},
			{ID: "rel-cover", Title: "Has Cover", Date: "2005-01-01", Status: "Official", Country: "GB", ReleaseGroup: mbGroup{PrimaryType: "Album"}},
		},
	}}

	best := job.selectBestRecordingRelease(context.Background(), filterCandidateRecordings(recordings, "Artist"))
	if best == nil {
		t.Fatal("expected release candidate")
	}
	if best.release.ID != "rel-cover" {
		t.Fatalf("expected rel-cover, got %q", best.release.ID)
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
