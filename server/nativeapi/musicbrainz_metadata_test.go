package nativeapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/navidrome/navidrome/conf"
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

func TestPrimarySearchArtist(t *testing.T) {
	tests := []struct {
		name   string
		artist string
		want   string
	}{
		{name: "single artist unchanged", artist: "Taylor Swift", want: "Taylor Swift"},
		{name: "comma separated collaboration", artist: "JAY-Z, Beyoncé", want: "JAY-Z"},
		{name: "parenthesized members", artist: "The Carters (Beyonce & Jay-Z)", want: "The Carters"},
		{name: "feat separator", artist: "Ed Sheeran feat. Lil Baby", want: "Ed Sheeran"},
		{name: "ft separator", artist: "Drake ft Rihanna", want: "Drake"},
		{name: "keeps ampersand duos", artist: "Simon & Garfunkel", want: "Simon & Garfunkel"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := primarySearchArtist(tt.artist); got != tt.want {
				t.Fatalf("primarySearchArtist(%q) = %q, want %q", tt.artist, got, tt.want)
			}
		})
	}
}

type fakeTransport func(*http.Request) (*http.Response, error)

func (f fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
}

func TestEscapeLucenePhrase(t *testing.T) {
	got := escapeLucenePhrase(`say "hi" \now`)
	if got != `say \"hi\" \\now` {
		t.Fatalf("unexpected escaped phrase: %q", got)
	}
}

func TestArtistCreditMatchesCollaborations(t *testing.T) {
	type credit = struct {
		Name string `json:"name"`
	}

	// Exact match still works.
	if !artistCreditMatches([]credit{{Name: "The Artist"}}, "the artist") {
		t.Fatal("expected exact credit match")
	}
	// A single credit that is part of a local "A feat. B" artist matches.
	if !artistCreditMatches([]credit{{Name: "Daft Punk"}}, "daft punk feat pharrell williams") {
		t.Fatal("expected credit to match local collaboration string")
	}
	// A local tag crediting only the primary artist matches joined credits.
	if !artistCreditMatches([]credit{{Name: "Daft Punk"}, {Name: "Pharrell Williams"}}, "daft punk") {
		t.Fatal("expected local primary artist to match joined credits")
	}
	// Substring noise must not match.
	if artistCreditMatches([]credit{{Name: "Ye"}}, "beyonce") {
		t.Fatal("expected substring credit not to match")
	}
}

func TestFetchITunesGenreMatchesDecoratedTags(t *testing.T) {
	j := newMusicBrainzMetadataJob()
	j.itunesInterval = 0
	j.client = &http.Client{Transport: fakeTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "itunes.apple.com" {
			t.Fatalf("unexpected host %q", req.URL.Host)
		}
		return jsonResponse(`{"results": [
			{"trackName": "Some Other Song", "artistName": "Beyoncé", "primaryGenreName": "Pop"},
			{"trackName": "***Flawless (feat. Chimamanda Ngozi Adichie)", "artistName": "Beyoncé", "primaryGenreName": "R&B/Soul"}
		]}`), nil
	})}

	got, trace := j.fetchITunesGenreWithTrace("***Flawless (feat. Chimamanda Ngozi Adichie)", "Beyonce, Chimamanda Ngozi Adichie")
	if got != "R&B/Soul" {
		t.Fatalf("expected diacritic-folded validated match, got %q", got)
	}
	if trace == nil || trace.Source != "itunes" || trace.FetchedGenre != got {
		t.Fatalf("unexpected iTunes genre trace: %+v", trace)
	}
	if !strings.Contains(trace.Request, "itunes.apple.com/search?") ||
		!strings.Contains(trace.Request, "entity=song") ||
		!strings.Contains(trace.Response, `"trackName": "***Flawless (feat. Chimamanda Ngozi Adichie)"`) {
		t.Fatalf("trace did not capture the iTunes request and candidates: %+v", trace)
	}
}

func TestFetchITunesGenreRejectsWrongMatches(t *testing.T) {
	j := newMusicBrainzMetadataJob()
	j.itunesInterval = 0
	j.client = &http.Client{Transport: fakeTransport(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(`{"results": [
			{"trackName": "Halo", "artistName": "Some Cover Band", "primaryGenreName": "Karaoke"},
			{"trackName": "Completely Different", "artistName": "Beyoncé", "primaryGenreName": "Pop"}
		]}`), nil
	})}

	if got := j.fetchITunesGenre("Halo", "Beyoncé"); got != "" {
		t.Fatalf("expected no genre when neither title+artist pair matches, got %q", got)
	}
}

func TestSpotifyFieldValue(t *testing.T) {
	if got := spotifyFieldValue(`Song "Live" Version`); got != "Song Live Version" {
		t.Fatalf("unexpected field value: %q", got)
	}
}

func TestBestArtistSimilarityUsesAllCredits(t *testing.T) {
	track := &spotifyTrack{}
	track.Artists = append(track.Artists, struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}{ID: "a1", Name: "Some Guy"}, struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}{ID: "a2", Name: "Daft Punk"})

	if got := bestArtistSimilarity(normalizeSpotifyString("Daft Punk"), track); got < 0.99 {
		t.Fatalf("expected non-primary credited artist to score, got %f", got)
	}
}

func TestTrackArtistGenrePrefersNameMatchedArtist(t *testing.T) {
	requests := 0
	j := newSpotifyMetadataJob()
	j.client = &http.Client{Transport: fakeTransport(func(req *http.Request) (*http.Response, error) {
		requests++
		return jsonResponse(`{"artists": [
			{"id": "a1", "name": "Wrong Artist", "genres": ["pop"]},
			{"id": "a2", "name": "Daft Punk", "genres": ["french house", "electronic", "disco"]}
		]}`), nil
	})}

	track := &spotifyTrack{}
	track.Artists = append(track.Artists, struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}{ID: "a1", Name: "Wrong Artist"}, struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}{ID: "a2", Name: "Daft Punk"})

	// The name-matched artist's genres win even with a low match confidence.
	got := j.trackArtistGenre(context.Background(), "tok", track, "Daft Punk", 0)
	if got != "French House, Electronic" {
		t.Fatalf("expected name-matched artist genres, got %q", got)
	}

	// Cached artists must not trigger a second request.
	_ = j.trackArtistGenre(context.Background(), "tok", track, "Daft Punk", 0)
	if requests != 1 {
		t.Fatalf("expected artist genres to be cached, got %d requests", requests)
	}
}

func TestTrackArtistGenreGatesUnmatchedArtistOnConfidence(t *testing.T) {
	restore := conf.Server.Spotify.MinScore
	conf.Server.Spotify.MinScore = 0.6
	defer func() { conf.Server.Spotify.MinScore = restore }()

	j := newSpotifyMetadataJob()
	j.client = &http.Client{Transport: fakeTransport(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(`{"artists": [{"id": "a1", "name": "Somebody Else", "genres": ["pop"]}]}`), nil
	})}

	track := &spotifyTrack{}
	track.Artists = append(track.Artists, struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}{ID: "a1", Name: "Somebody Else"})

	// No credited artist matches the library artist and the track match was
	// weak: genres from the wrong artist must not be used.
	if got := j.trackArtistGenre(context.Background(), "tok", track, "Daft Punk", 0.3); got != "" {
		t.Fatalf("expected no genre for weak unmatched match, got %q", got)
	}
	// A confident track match may fall back to the primary artist.
	if got := j.trackArtistGenre(context.Background(), "tok", track, "Daft Punk", 0.9); got != "Pop" {
		t.Fatalf("expected primary artist fallback for confident match, got %q", got)
	}
}
