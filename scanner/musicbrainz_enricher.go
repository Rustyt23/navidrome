package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

var bracketContentRegex = regexp.MustCompile(`\([^)]*\)|\[[^\]]*\]|\{[^}]*\}`)
var punctuationRegex = regexp.MustCompile(`[^\p{L}\p{N}\s]+`)
var featuredArtistRegex = regexp.MustCompile(`\b(feat|ft)\.?\b.*$`)

type musicBrainzEnricher struct {
	client      *http.Client
	rateMu      sync.Mutex
	nextAllowed time.Time

	totalSongsProcessed  atomic.Int64
	strictMatchesUpdated atomic.Int64
	strictMatchesSkipped atomic.Int64
}

func newMusicBrainzEnricher() *musicBrainzEnricher {
	return &musicBrainzEnricher{client: &http.Client{Timeout: 10 * time.Second}}
}

func (m *musicBrainzEnricher) enrichTracks(ctx context.Context, entry *folderEntry) error {
	for i := range entry.tracks {
		track := &entry.tracks[i]
		if !needsEnrichment(track) {
			continue
		}
		m.totalSongsProcessed.Add(1)

		updated, lowConfidence, err := m.enrichTrack(ctx, track)
		if err != nil {
			return err
		}
		if lowConfidence {
			m.strictMatchesSkipped.Add(1)
		}
		if updated {
			m.strictMatchesUpdated.Add(1)
			if track.Genre != "" {
				entry.tags = append(entry.tags, model.NewTag(model.TagGenre, track.Genre))
			}
		}
	}
	return nil
}

func needsEnrichment(track *model.MediaFile) bool {
	missingAlbum := track.Album == "" || track.Album == consts.UnknownAlbum
	missingYear := track.ReleaseYear == 0
	missingGenre := strings.TrimSpace(track.Genre) == ""
	return missingAlbum || missingYear || missingGenre
}

func (m *musicBrainzEnricher) enrichTrack(ctx context.Context, track *model.MediaFile) (updated bool, lowConfidence bool, err error) {
	title := normalizeMusicBrainzTerm(track.Title)
	artist := normalizeMusicBrainzTerm(track.Artist)
	if title == "" || artist == "" {
		return false, true, nil
	}
	result, err := m.searchRecording(ctx, title, artist)
	if err != nil {
		return false, false, err
	}

	filtered := make([]musicBrainzRecording, 0, len(result.Recordings))
	for _, rec := range result.Recordings {
		if rec.Score < 85 {
			continue
		}
		if normalizeMusicBrainzTerm(rec.artistCreditString()) != artist {
			continue
		}
		filtered = append(filtered, rec)
	}
	if len(filtered) == 0 {
		return false, true, nil
	}

	knownYear := knownTrackYear(track)
	var candidate *musicBrainzRelease
	for _, rec := range filtered {
		for _, rel := range rec.Releases {
			if rel.ReleaseGroup.PrimaryType != "Album" || rel.Status != "Official" {
				continue
			}
			if candidate == nil || chooseRelease(rel, *candidate, knownYear) {
				r := rel
				candidate = &r
			}
		}
	}
	if candidate == nil {
		return false, false, nil
	}

	if (track.Album == "" || track.Album == consts.UnknownAlbum) && candidate.Title != "" {
		track.Album = candidate.Title
		updated = true
	}
	if track.ReleaseYear == 0 {
		if year := releaseYear(candidate.ReleaseGroup.FirstReleaseDate); year > 0 {
			track.ReleaseYear = year
			updated = true
		}
	}
	if strings.TrimSpace(track.Genre) == "" {
		genre := firstTag(candidate.ReleaseGroup.Tags)
		if genre == "" {
			for _, rec := range filtered {
				genre = firstTag(rec.Tags)
				if genre != "" {
					break
				}
			}
		}
		if genre != "" {
			track.Genre = genre
			updated = true
		}
	}
	return updated, false, nil
}

func chooseRelease(a, b musicBrainzRelease, knownYear int) bool {
	ya := releaseYear(a.ReleaseGroup.FirstReleaseDate)
	yb := releaseYear(b.ReleaseGroup.FirstReleaseDate)
	if knownYear > 0 {
		da := absInt(knownYear - ya)
		db := absInt(knownYear - yb)
		if da != db {
			return da < db
		}
	}
	if ya == 0 {
		return false
	}
	if yb == 0 {
		return true
	}
	return ya < yb
}

func (m *musicBrainzEnricher) searchRecording(ctx context.Context, title, artist string) (*musicBrainzRecordingSearchResult, error) {
	query := fmt.Sprintf(`recording:"%s" AND artist:"%s"`, title, artist)
	u := "https://musicbrainz.org/ws/2/recording/?fmt=json&limit=5&query=" + url.QueryEscape(query)

	if err := m.waitForRateLimit(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Navidrome/metadata-enrichment")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz request failed with status %d", resp.StatusCode)
	}
	var out musicBrainzRecordingSearchResult
	if err = json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (m *musicBrainzEnricher) waitForRateLimit(ctx context.Context) error {
	m.rateMu.Lock()
	defer m.rateMu.Unlock()

	now := time.Now()
	if now.Before(m.nextAllowed) {
		t := time.NewTimer(time.Until(m.nextAllowed))
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
	m.nextAllowed = time.Now().Add(time.Second)
	return nil
}

func (m *musicBrainzEnricher) logStats(ctx context.Context) {
	log.Info(ctx, "Scanner: MusicBrainz strict enrichment stats",
		"totalSongsProcessed", m.totalSongsProcessed.Load(),
		"strictMatchesUpdated", m.strictMatchesUpdated.Load(),
		"strictMatchesSkippedLowConfidence", m.strictMatchesSkipped.Load(),
	)
}

func normalizeMusicBrainzTerm(v string) string {
	v = strings.ToLower(v)
	v = bracketContentRegex.ReplaceAllString(v, " ")
	v = featuredArtistRegex.ReplaceAllString(v, " ")
	v = punctuationRegex.ReplaceAllString(v, " ")
	v = strings.Join(strings.Fields(v), " ")
	return strings.TrimSpace(v)
}

func knownTrackYear(track *model.MediaFile) int {
	for _, y := range []int{track.ReleaseYear, track.Year, track.OriginalYear} {
		if y > 0 {
			return y
		}
	}
	return 0
}

func releaseYear(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return y
}

func firstTag(tags []musicBrainzTag) string {
	if len(tags) == 0 {
		return ""
	}
	return tags[0].Name
}

func absInt(v int) int {
	return int(math.Abs(float64(v)))
}

type musicBrainzRecordingSearchResult struct {
	Recordings []musicBrainzRecording `json:"recordings"`
}

type musicBrainzRecording struct {
	Score        int                    `json:"score,string"`
	ArtistCredit []musicBrainzArtistRef `json:"artist-credit"`
	Releases     []musicBrainzRelease   `json:"releases"`
	Tags         []musicBrainzTag       `json:"tags"`
}

func (r musicBrainzRecording) artistCreditString() string {
	if len(r.ArtistCredit) == 0 {
		return ""
	}
	var b strings.Builder
	for _, a := range r.ArtistCredit {
		name := strings.TrimSpace(a.Name)
		if name == "" {
			name = strings.TrimSpace(a.Artist.Name)
		}
		b.WriteString(name)
		b.WriteString(a.JoinPhrase)
	}
	return b.String()
}

type musicBrainzArtistRef struct {
	Name       string `json:"name"`
	JoinPhrase string `json:"joinphrase"`
	Artist     struct {
		Name string `json:"name"`
	} `json:"artist"`
}

type musicBrainzRelease struct {
	Title        string `json:"title"`
	Status       string `json:"status"`
	ReleaseGroup struct {
		PrimaryType      string           `json:"primary-type"`
		FirstReleaseDate string           `json:"first-release-date"`
		Tags             []musicBrainzTag `json:"tags"`
	} `json:"release-group"`
}

type musicBrainzTag struct {
	Name string `json:"name"`
}
