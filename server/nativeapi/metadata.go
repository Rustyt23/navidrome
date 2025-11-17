package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/log"
)

const (
	musicBrainzRecordingURL = "https://musicbrainz.org/ws/2/recording/"
	coverArtArchiveURL      = "https://coverartarchive.org/release/"
	metadataUserAgent       = "NavidromeMetadataFetcher/1.0 (+https://www.navidrome.org)"
)

type metadataFetchRequest struct {
	Songs []metadataSongPayload `json:"songs"`
}

type metadataFetchResponse struct {
	Songs []metadataSongPayload `json:"songs"`
}

type metadataSongPayload struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	Genre      string `json:"genre"`
	Year       *int   `json:"year"`
	CoverArt   string `json:"coverArt"`
	ArtworkURL string `json:"artworkUrl"`
}

type metadataSongPayloadDTO struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Artist     string          `json:"artist"`
	Album      string          `json:"album"`
	Genre      string          `json:"genre"`
	CoverArt   string          `json:"coverArt"`
	ArtworkURL string          `json:"artworkUrl"`
	Year       json.RawMessage `json:"year"`
}

func (m *metadataSongPayload) UnmarshalJSON(data []byte) error {
	var dto metadataSongPayloadDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return err
	}

	m.ID = dto.ID
	m.Title = dto.Title
	m.Artist = dto.Artist
	m.Album = dto.Album
	m.Genre = dto.Genre
	m.CoverArt = dto.CoverArt
	m.ArtworkURL = dto.ArtworkURL
	m.Year = parseYearRaw(dto.Year)
	return nil
}

func (n *Router) addMetadataRoute(r chi.Router) {
	r.Route("/metadata", func(r chi.Router) {
		r.Post("/fetch", n.fetchMetadataHandler())
	})
}

func (n *Router) fetchMetadataHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var req metadataFetchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if len(req.Songs) == 0 {
			writeJSON(w, metadataFetchResponse{Songs: []metadataSongPayload{}})
			return
		}

		updates := make([]metadataSongPayload, 0, len(req.Songs))
		for _, song := range req.Songs {
			normalized := normalizeSongPayload(song)
			enriched, err := metadataEnricher.Enrich(ctx, normalized)
			if err != nil {
				log.Warn(ctx, "Unable to enrich song metadata", "songId", normalized.ID, "err", err)
				updates = append(updates, normalized)
				continue
			}
			updates = append(updates, normalizeSongPayload(enriched))
		}

		writeJSON(w, metadataFetchResponse{Songs: updates})
	}
}

func writeJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

type metadataFetcher struct {
	client        *http.Client
	coverArt      *coverArtArchiveClient
	mu            sync.Mutex
	nextAvailable time.Time
}

var metadataEnricher = newMetadataFetcher()

func newMetadataFetcher() *metadataFetcher {
	timeout := 10 * time.Second
	coverClient := &http.Client{Timeout: timeout}
	return &metadataFetcher{
		client:   &http.Client{Timeout: timeout},
		coverArt: newCoverArtArchiveClient(coverClient),
	}
}

func (f *metadataFetcher) Enrich(ctx context.Context, song metadataSongPayload) (metadataSongPayload, error) {
	title := strings.TrimSpace(song.Title)
	artist := strings.TrimSpace(song.Artist)
	if title == "" || artist == "" {
		return song, nil
	}

	recording, err := f.lookupRecording(ctx, title, artist)
	if err != nil || recording == nil {
		return song, err
	}

	updated := song
	if strings.TrimSpace(updated.Artist) == "" && recording.PrimaryArtist() != "" {
		updated.Artist = recording.PrimaryArtist()
	}
	if strings.TrimSpace(updated.Title) == "" && recording.Title != "" {
		updated.Title = recording.Title
	}
	if needsAlbumUpdate(updated.Album) && recording.PrimaryReleaseTitle() != "" {
		updated.Album = recording.PrimaryReleaseTitle()
	}
	if strings.TrimSpace(updated.Genre) == "" && recording.PrimaryTag() != "" {
		updated.Genre = recording.PrimaryTag()
	}
	if updated.Year == nil || *updated.Year == 0 {
		if year := recording.ReleaseYear(); year != nil {
			updated.Year = year
		}
	}
	if strings.TrimSpace(updated.ArtworkURL) == "" {
		if releaseID := recording.PrimaryReleaseID(); releaseID != "" {
			artURL, artErr := f.coverArt.Fetch(ctx, releaseID)
			if artErr != nil {
				log.Warn(ctx, "Unable to fetch cover art", "songId", song.ID, "err", artErr)
			} else if artURL != "" {
				updated.ArtworkURL = artURL
			}
		}
	}

	return updated, nil
}

func (f *metadataFetcher) lookupRecording(ctx context.Context, title, artist string) (*musicBrainzRecording, error) {
	query := url.Values{}
	query.Set("query", fmt.Sprintf("%s AND artist:%s", title, artist))
	query.Set("fmt", "json")
	query.Set("limit", "1")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, musicBrainzRecordingURL+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	f.applyRateLimit()
	req.Header.Set("User-Agent", metadataUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		time.Sleep(time.Second)
		return f.lookupRecording(ctx, title, artist)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz search failed: %s", resp.Status)
	}

	var result musicBrainzRecordingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Recordings) == 0 {
		return nil, nil
	}

	return &result.Recordings[0], nil
}

func (f *metadataFetcher) applyRateLimit() {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()
	if wait := f.nextAvailable.Sub(now); wait > 0 {
		time.Sleep(wait)
	}
	f.nextAvailable = time.Now().Add(time.Second)
}

type musicBrainzRecordingResponse struct {
	Recordings []musicBrainzRecording `json:"recordings"`
}

type musicBrainzRecording struct {
	Title            string                    `json:"title"`
	ArtistCredit     []musicBrainzArtistCredit `json:"artist-credit"`
	Releases         []musicBrainzRelease      `json:"releases"`
	FirstReleaseDate string                    `json:"first-release-date"`
	Tags             []musicBrainzRecordingTag `json:"tags"`
}

type musicBrainzArtistCredit struct {
	Name string `json:"name"`
}

type musicBrainzRelease struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Date  string `json:"date"`
}

type musicBrainzRecordingTag struct {
	Name string `json:"name"`
}

type coverArtResponse struct {
	Images []coverArtImage `json:"images"`
}

type coverArtImage struct {
	Image string `json:"image"`
	Front bool   `json:"front"`
}

func (m musicBrainzRecording) PrimaryArtist() string {
	if len(m.ArtistCredit) == 0 {
		return ""
	}
	return strings.TrimSpace(m.ArtistCredit[0].Name)
}

func (m musicBrainzRecording) PrimaryReleaseTitle() string {
	if len(m.Releases) == 0 {
		return ""
	}
	return strings.TrimSpace(m.Releases[0].Title)
}

func (m musicBrainzRecording) PrimaryReleaseID() string {
	if len(m.Releases) == 0 {
		return ""
	}
	return strings.TrimSpace(m.Releases[0].ID)
}

func (m musicBrainzRecording) ReleaseYear() *int {
	if len(m.Releases) > 0 {
		if year := extractYear(m.Releases[0].Date); year != nil {
			return year
		}
	}
	if year := extractYear(m.FirstReleaseDate); year != nil {
		return year
	}
	return nil
}

func (m musicBrainzRecording) PrimaryTag() string {
	if len(m.Tags) == 0 {
		return ""
	}
	return strings.TrimSpace(m.Tags[0].Name)
}

func extractYear(date string) *int {
	date = strings.TrimSpace(date)
	if len(date) < 4 {
		return nil
	}
	year, err := strconv.Atoi(date[:4])
	if err != nil {
		return nil
	}
	return &year
}

func needsAlbumUpdate(album string) bool {
	album = strings.TrimSpace(album)
	if album == "" {
		return true
	}
	return strings.EqualFold(album, "[Unknown Album]")
}

func parseYearRaw(raw json.RawMessage) *int {
	if raw == nil || len(raw) == 0 {
		return nil
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || strings.EqualFold(trimmed, "null") {
		return nil
	}
	var asInt int
	if err := json.Unmarshal(raw, &asInt); err == nil {
		return &asInt
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString == "" {
			return nil
		}
		if parsed, err := strconv.Atoi(asString); err == nil {
			return &parsed
		}
	}
	return nil
}

func normalizeSongPayload(song metadataSongPayload) metadataSongPayload {
	song.ID = strings.TrimSpace(song.ID)
	song.Title = strings.TrimSpace(song.Title)
	song.Artist = strings.TrimSpace(song.Artist)
	song.Album = strings.TrimSpace(song.Album)
	song.Genre = strings.TrimSpace(song.Genre)
	song.CoverArt = strings.TrimSpace(song.CoverArt)
	song.ArtworkURL = ensureHTTPSURL(song.ArtworkURL)
	if song.Year != nil {
		year := *song.Year
		if year == 0 {
			song.Year = nil
		}
	}
	return song
}

func ensureHTTPSURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	switch {
	case trimmed == "":
		return ""
	case strings.HasPrefix(trimmed, "https://"):
		return trimmed
	case strings.HasPrefix(trimmed, "http://"):
		return "https://" + strings.TrimPrefix(trimmed, "http://")
	case strings.HasPrefix(trimmed, "//"):
		return "https:" + trimmed
	default:
		return trimmed
	}
}
