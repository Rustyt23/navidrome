package nativeapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/navidrome/navidrome/model"
	"golang.org/x/text/unicode/norm"
)

// Genre lookups use the iTunes Search API instead of MusicBrainz: Apple's
// primaryGenreName is an editorial, per-track genre, far more reliable than
// MusicBrainz's crowd-sourced tags, and it needs no API key. The API is rate
// limited to roughly 20 calls/minute, hence the enforced spacing between
// requests.

const itunesSearchURL = "https://itunes.apple.com/search"

const (
	// A local file and Apple's copy of the same recording routinely differ by a
	// few seconds of encoder padding, so identity is judged on a loose window
	// rather than an exact duration.
	itunesDurationToleranceSeconds = 10
	// Below this many shared words, one title being a subset of another says
	// nothing: "Love" is a subset of "Love Story" but a different song.
	itunesMinSubsetTitleTokens = 3
)

type itunesSearchResponse struct {
	Results []itunesTrack `json:"results"`
}

type itunesTrack struct {
	TrackName        string `json:"trackName"`
	ArtistName       string `json:"artistName"`
	CollectionName   string `json:"collectionName"`
	PrimaryGenreName string `json:"primaryGenreName"`
	TrackTimeMillis  int    `json:"trackTimeMillis"`
	TrackViewURL     string `json:"trackViewUrl"`
}

// itunesTrackQuery is what the lookup knows about the local file. Album and
// Duration are optional; when present they separate candidates that the title
// and artist alone cannot.
type itunesTrackQuery struct {
	Title    string
	Artist   string
	Album    string
	Duration float32 // seconds, 0 when unknown
}

func itunesTrackQueryFor(mf model.MediaFile) itunesTrackQuery {
	return itunesTrackQuery{
		Title:    mf.Title,
		Artist:   mf.Artist,
		Album:    mf.Album,
		Duration: mf.Duration,
	}
}

// itunesTitleMatch grades how closely a result title matches the local one.
// The grade matters because Apple carries re-recordings, karaoke cuts and
// remixes under decorated variants of the same name, and they do not share the
// original's genre: "22" is Country, "22 (Taylor's Version)" is Pop. Stripping
// the decoration from both sides makes those indistinguishable, so only the
// closest grade present is allowed to vote.
type itunesTitleMatch int

const (
	itunesTitleNoMatch   itunesTitleMatch = iota
	itunesTitleExact                      // the result title is the local title
	itunesTitleSanitized                  // equal once release decoration is stripped
	itunesTitleLoose                      // masked profanity, or a token subset
)

// itunesCandidate is one search result that passed the title and artist gates,
// kept alongside its position in Apple's relevance ordering so ties can fall
// back to it.
type itunesCandidate struct {
	rank  int
	track itunesTrack
	genre string
	match itunesTitleMatch
}

// fetchITunesGenre looks up the genre for a track. The search term uses the
// primary artist and a de-decorated title so decorated local tags still hit,
// while every result is validated against the full local title and artist
// before its genre is trusted. Returns "" when there is no confident match.
func (j *musicBrainzMetadataJob) fetchITunesGenre(q itunesTrackQuery) string {
	genre, _ := j.fetchITunesGenreWithTrace(q)
	return genre
}

// fetchITunesGenreWithTrace observes the same lookup used by fetchITunesGenre
// and returns the request and decoded candidates for the AI tool developer UI.
func (j *musicBrainzMetadataJob) fetchITunesGenreWithTrace(q itunesTrackQuery) (string, *genreSourceDeveloperTrace) {
	trace := &genreSourceDeveloperTrace{Source: "itunes"}
	searchTitle := sanitizeSpotifyRetryTitle(q.Title)
	searchArtist := primarySearchArtist(q.Artist)
	if searchTitle == "" || searchArtist == "" {
		return "", trace
	}

	j.waitITunesSlot()

	params := url.Values{}
	params.Set("term", searchArtist+" "+searchTitle)
	params.Set("media", "music")
	params.Set("entity", "song")
	params.Set("limit", "10")
	req, err := http.NewRequest(http.MethodGet, itunesSearchURL+"?"+params.Encode(), nil)
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

	// Every result that could be this song is collected rather than returning
	// on the first hit: Apple lists the same recording several times (album,
	// single, deluxe reissue) and those copies do not always carry the same
	// genre, so the first one is an arbitrary winner.
	wantTitle := normalizeExternalString(searchTitle)
	wantArtist := normalizeExternalString(q.Artist)
	candidates := make([]itunesCandidate, 0, len(payload.Results))
	for i, track := range payload.Results {
		genre := strings.TrimSpace(track.PrimaryGenreName)
		if genre == "" {
			continue
		}
		match := classifyITunesTitle(searchTitle, wantTitle, track.TrackName)
		if match == itunesTitleNoMatch {
			continue
		}
		if !itunesArtistMatches(wantArtist, track.ArtistName) {
			continue
		}
		candidates = append(candidates, itunesCandidate{rank: i, track: track, genre: genre, match: match})
	}
	if len(candidates) == 0 {
		return "", trace
	}

	// Keep only the closest title grade, so a re-recording never outvotes the
	// original just because Apple stocks more copies of it.
	candidates = preferITunesTitleMatch(candidates)

	// Duration and album narrow the field to the local file's actual pressing.
	// Both only ever filter when something survives, so a track Apple lists
	// with no duration, or an album tag that disagrees with Apple's, degrades
	// to the unfiltered vote instead of losing the genre entirely.
	candidates = preferITunesDuration(candidates, q.Duration)
	candidates = preferITunesAlbum(candidates, q.Album)

	genre, best := voteITunesGenre(candidates)
	if genre == "" {
		return "", trace
	}
	trace.FetchedGenre = genre
	trace.SongURL = normalizeITunesSongURL(best.track.TrackViewURL)
	return genre, trace
}

// preferITunesTitleMatch keeps only the candidates sharing the closest title
// grade any of them achieved.
func preferITunesTitleMatch(candidates []itunesCandidate) []itunesCandidate {
	best := itunesTitleLoose
	for _, c := range candidates {
		if c.match < best {
			best = c.match
		}
	}
	matched := make([]itunesCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.match == best {
			matched = append(matched, c)
		}
	}
	return matched
}

// preferITunesDuration keeps only the candidates whose length matches the local
// file, when any of them do.
func preferITunesDuration(candidates []itunesCandidate, seconds float32) []itunesCandidate {
	if seconds <= 0 {
		return candidates
	}
	matched := make([]itunesCandidate, 0, len(candidates))
	for _, c := range candidates {
		if itunesDurationMatches(seconds, c.track.TrackTimeMillis) {
			matched = append(matched, c)
		}
	}
	if len(matched) == 0 {
		return candidates
	}
	return matched
}

func itunesDurationMatches(seconds float32, trackMillis int) bool {
	if seconds <= 0 || trackMillis <= 0 {
		return false
	}
	diff := float64(seconds) - float64(trackMillis)/1000
	if diff < 0 {
		diff = -diff
	}
	return diff <= itunesDurationToleranceSeconds
}

// preferITunesAlbum keeps only the candidates released on the local file's
// album, when any of them are.
func preferITunesAlbum(candidates []itunesCandidate, album string) []itunesCandidate {
	want := normalizeExternalString(sanitizeSpotifyRetryTitle(album))
	if want == "" || strings.EqualFold(strings.TrimSpace(album), "[unknown album]") {
		return candidates
	}
	matched := make([]itunesCandidate, 0, len(candidates))
	for _, c := range candidates {
		got := normalizeExternalString(sanitizeSpotifyRetryTitle(c.track.CollectionName))
		if got == "" {
			continue
		}
		if got == want || containsAllTokens(got, want) || containsAllTokens(want, got) {
			matched = append(matched, c)
		}
	}
	if len(matched) == 0 {
		return candidates
	}
	return matched
}

// voteITunesGenre returns the genre the most candidates agree on, breaking ties
// towards Apple's own ordering. It also returns the best-ranked candidate
// carrying that genre, so the trace links to a song page that really has it.
func voteITunesGenre(candidates []itunesCandidate) (string, itunesCandidate) {
	type tally struct {
		display string
		count   int
		best    itunesCandidate
	}
	counts := map[string]*tally{}
	order := make([]string, 0, len(candidates))
	for _, c := range candidates {
		key := normalizeMBString(c.genre)
		if key == "" {
			continue
		}
		if t, ok := counts[key]; ok {
			t.count++
			continue
		}
		counts[key] = &tally{display: c.genre, count: 1, best: c}
		order = append(order, key)
	}

	var winner *tally
	for _, key := range order {
		// Strictly greater, so an equal count leaves the earlier — and
		// therefore better ranked — genre in place.
		if t := counts[key]; winner == nil || t.count > winner.count {
			winner = t
		}
	}
	if winner == nil {
		return "", itunesCandidate{}
	}
	return winner.display, winner.best
}

// normalizeITunesSongURL only exposes Apple-owned song pages to the UI. Older
// Search API responses can contain http links, which are safe to upgrade to
// https because both supported hosts serve the same canonical store pages.
func normalizeITunesSongURL(value string) string {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.User != nil || parsed.Hostname() == "" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "itunes.apple.com" && !strings.HasSuffix(host, ".itunes.apple.com") &&
		host != "music.apple.com" && !strings.HasSuffix(host, ".music.apple.com") {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	parsed.Scheme = "https"
	return parsed.String()
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

// classifyITunesTitle grades a result title against the local one. localTitle is
// the local title with its own tag noise already stripped; wantTitle is that
// same value normalized.
func classifyITunesTitle(localTitle, wantTitle, resultTitle string) itunesTitleMatch {
	if wantTitle == "" {
		return itunesTitleNoMatch
	}
	// Compared before stripping anything from the result, so a decorated
	// variant cannot pass as the plain original.
	if normalizeExternalString(resultTitle) == wantTitle {
		return itunesTitleExact
	}
	sanitized := sanitizeSpotifyRetryTitle(resultTitle)
	got := normalizeExternalString(sanitized)
	if got == "" {
		return itunesTitleNoMatch
	}
	if got == wantTitle {
		return itunesTitleSanitized
	}
	if itunesMaskedTitleMatches(localTitle, sanitized) || itunesSubsetTitleMatches(got, wantTitle) {
		return itunesTitleLoose
	}
	return itunesTitleNoMatch
}

// itunesMaskedTitleMatches reports whether the result is the local title with
// Apple's profanity masking applied. Apple ships "APESHIT" as "APES**T", and
// normalization turns those asterisks into a space, so the correct — often
// top-ranked — result would otherwise be rejected over its censoring. Each '*'
// stands for exactly one character, and at least half the title has to remain
// visible so a heavily masked name cannot match everything.
func itunesMaskedTitleMatches(localTitle, resultTitle string) bool {
	got := []rune(foldITunesTitle(resultTitle))
	want := []rune(foldITunesTitle(localTitle))
	if len(got) == 0 || len(got) != len(want) {
		return false
	}
	masked := 0
	for i, r := range got {
		if r == '*' {
			masked++
			continue
		}
		if r != want[i] {
			return false
		}
	}
	return masked > 0 && masked*2 <= len(got)
}

// foldITunesTitle lower-cases and drops diacritics but keeps every other
// character, so the masking asterisks survive for itunesMaskedTitleMatches.
func foldITunesTitle(v string) string {
	folded := strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, norm.NFD.String(v))
	return strings.Join(strings.Fields(strings.ToLower(folded)), " ")
}

// itunesSubsetTitleMatches allows one title to merely add words to the other,
// but only when that cannot be coincidence. Accepting any subset lets a local
// tag of "Love" bind to Apple's "Love Story" and import a genre from an
// entirely different song, so the extra words must either be release
// decoration ("remastered", "2011") or the shared part must be long enough to
// identify the song on its own.
func itunesSubsetTitleMatches(got, wantTitle string) bool {
	if extra, ok := titleTokenDifference(got, wantTitle); ok {
		return itunesDecorationOnly(extra) || len(strings.Fields(wantTitle)) >= itunesMinSubsetTitleTokens
	}
	if extra, ok := titleTokenDifference(wantTitle, got); ok {
		return itunesDecorationOnly(extra) || len(strings.Fields(got)) >= itunesMinSubsetTitleTokens
	}
	return false
}

// titleTokenDifference returns the words haystack adds over needle, and whether
// needle is contained in haystack at all.
func titleTokenDifference(haystack, needle string) ([]string, bool) {
	if haystack == "" || needle == "" {
		return nil, false
	}
	haystackWords := map[string]bool{}
	for _, w := range strings.Fields(haystack) {
		haystackWords[w] = true
	}
	needleWords := map[string]bool{}
	for _, w := range strings.Fields(needle) {
		if !haystackWords[w] {
			return nil, false
		}
		needleWords[w] = true
	}
	var extra []string
	for _, w := range strings.Fields(haystack) {
		if !needleWords[w] {
			extra = append(extra, w)
		}
	}
	return extra, true
}

// itunesDecorationTokens are the words a store listing adds to a title without
// making it a different song.
var itunesDecorationTokens = map[string]bool{
	"a": true, "the": true, "with": true, "feat": true, "featuring": true,
	"ft": true, "remaster": true, "remastered": true, "remasters": true,
	"version": true, "edit": true, "edited": true, "mix": true, "remix": true,
	"live": true, "mono": true, "stereo": true, "deluxe": true, "bonus": true,
	"radio": true, "single": true, "album": true, "acoustic": true,
	"instrumental": true, "explicit": true, "clean": true, "extended": true,
	"original": true, "anniversary": true, "edition": true, "reissue": true,
	"demo": true, "reprise": true, "intro": true, "outro": true,
}

var itunesYearTokenRegex = regexp.MustCompile(`^(19|20)\d{2}$`)

func itunesDecorationOnly(tokens []string) bool {
	for _, token := range tokens {
		if itunesDecorationTokens[token] || itunesYearTokenRegex.MatchString(token) {
			continue
		}
		return false
	}
	return true
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
