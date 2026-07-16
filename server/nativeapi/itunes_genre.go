package nativeapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Genre lookups use the iTunes Search API instead of MusicBrainz: Apple's
// primaryGenreName is an editorial, per-track genre, far more reliable than
// MusicBrainz's crowd-sourced tags, and it needs no API key. The API is rate
// limited to roughly 20 calls/minute, hence the enforced spacing between
// requests.

const itunesSearchURL = "https://itunes.apple.com/search"

type itunesSearchResponse struct {
	Results []itunesTrack `json:"results"`
}

type itunesTrack struct {
	TrackName        string `json:"trackName"`
	ArtistName       string `json:"artistName"`
	PrimaryGenreName string `json:"primaryGenreName"`
}

// fetchITunesGenre looks up the genre for a track. The search term uses the
// primary artist and a de-decorated title so decorated local tags still hit,
// while every result is validated against the full local title and artist
// before its genre is trusted. Returns "" when there is no confident match.
func (j *musicBrainzMetadataJob) fetchITunesGenre(title, artist string) string {
	genre, _ := j.fetchITunesGenreWithTrace(title, artist)
	return genre
}

// fetchITunesGenreWithTrace observes the same lookup used by fetchITunesGenre
// and returns the request and decoded candidates for the AI tool developer UI.
func (j *musicBrainzMetadataJob) fetchITunesGenreWithTrace(title, artist string) (string, *genreSourceDeveloperTrace) {
	trace := &genreSourceDeveloperTrace{Source: "itunes"}
	searchTitle := sanitizeSpotifyRetryTitle(title)
	searchArtist := primarySearchArtist(artist)
	if searchTitle == "" || searchArtist == "" {
		return "", trace
	}

	j.waitITunesSlot()

	q := url.Values{}
	q.Set("term", searchArtist+" "+searchTitle)
	q.Set("media", "music")
	q.Set("entity", "song")
	q.Set("limit", "10")
	req, err := http.NewRequest(http.MethodGet, itunesSearchURL+"?"+q.Encode(), nil)
	if err != nil {
		return "", trace
	}
	trace.Request = req.URL.String()
	req.Header.Set("User-Agent", "Navidrome/metadata-fetcher (https://www.navidrome.org)")

	resp, err := j.client.Do(req)
	if err != nil {
		return "", trace
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", trace
	}

	var payload itunesSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", trace
	}
	if response, err := json.MarshalIndent(payload, "", "  "); err == nil {
		trace.Response = string(response)
	}

	wantTitle := normalizeExternalString(searchTitle)
	wantArtist := normalizeExternalString(artist)
	for _, track := range payload.Results {
		genre := strings.TrimSpace(track.PrimaryGenreName)
		if genre == "" {
			continue
		}
		if !itunesTitleMatches(wantTitle, track.TrackName) {
			continue
		}
		if !itunesArtistMatches(wantArtist, track.ArtistName) {
			continue
		}
		trace.FetchedGenre = genre
		return genre, trace
	}
	return "", trace
}

// waitITunesSlot spaces iTunes requests out; the API blocks callers that
// exceed ~20 requests per minute.
func (j *musicBrainzMetadataJob) waitITunesSlot() {
	j.itunesMu.Lock()
	defer j.itunesMu.Unlock()
	if wait := j.itunesInterval - time.Since(j.lastITunesCall); wait > 0 {
		time.Sleep(wait)
	}
	j.lastITunesCall = time.Now()
}

func itunesTitleMatches(wantTitle, resultTitle string) bool {
	got := normalizeExternalString(sanitizeSpotifyRetryTitle(resultTitle))
	if got == "" || wantTitle == "" {
		return false
	}
	return got == wantTitle || containsAllTokens(got, wantTitle) || containsAllTokens(wantTitle, got)
}

func itunesArtistMatches(wantArtist, resultArtist string) bool {
	got := normalizeExternalString(resultArtist)
	if got == "" || wantArtist == "" {
		return false
	}
	return got == wantArtist ||
		containsAllTokens(wantArtist, got) ||
		containsAllTokens(got, wantArtist) ||
		stringSimilarity(got, wantArtist) >= 0.8
}

// normalizeExternalString folds diacritics on top of the usual normalization,
// so "Beyoncé" (Apple) matches a local tag spelled "Beyonce".
func normalizeExternalString(v string) string {
	folded := strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, norm.NFD.String(v))
	return normalizeMBString(folded)
}
