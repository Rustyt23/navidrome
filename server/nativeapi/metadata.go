package nativeapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server/events"
	"github.com/navidrome/navidrome/utils/str"
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
	Persisted  bool   `json:"persisted,omitempty"`
}

type metadataSongPayloadDTO struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Artist     string          `json:"artist"`
	Album      string          `json:"album"`
	Genre      string          `json:"genre"`
	CoverArt   string          `json:"coverArt"`
	ArtworkURL string          `json:"artworkUrl,omitempty"`
	Persisted  bool            `json:"persisted,omitempty"`
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
	m.Persisted = dto.Persisted
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

		repo := n.ds.MediaFile(ctx)
		updates := make([]metadataSongPayload, 0, len(req.Songs))
		changedIDs := make(map[string]struct{})
		for _, song := range req.Songs {
			normalized := normalizeSongPayload(song)
			enriched, err := metadataEnricher.Enrich(ctx, normalized)
			if err != nil {
				log.Warn(ctx, "Unable to enrich song metadata", "songId", normalized.ID, "err", err)
				updates = append(updates, normalized)
				continue
			}
			saved := normalizeSongPayload(enriched)
			if repo != nil && saved.ID != "" {
				existing, err := repo.Get(saved.ID)
				if err != nil {
					log.Warn(ctx, "Unable to load song metadata for persistence", "songId", saved.ID, "err", err)
				} else {
					if wrote, writeErr := writeTagsToFile(ctx, existing, saved); writeErr != nil {
						log.Warn(ctx, "Unable to write metadata tags to file", "songId", saved.ID, "err", writeErr)
					} else if wrote {
						saved.Persisted = true
					}
					if changed, persistErr := persistMetadataSongWithCurrent(repo, existing, saved); persistErr != nil {
						log.Warn(ctx, "Unable to persist song metadata", "songId", saved.ID, "err", persistErr)
					} else if changed {
						changedIDs[saved.ID] = struct{}{}
					}
				}
			}
			if saved.Persisted {
				changedIDs[saved.ID] = struct{}{}
			}
			updates = append(updates, saved)
		}

		if len(changedIDs) > 0 && n.broker != nil {
			ids := make([]string, 0, len(changedIDs))
			for id := range changedIDs {
				if id != "" {
					ids = append(ids, id)
				}
			}
			if len(ids) > 0 {
				event := &events.RefreshResource{}
				n.broker.SendMessage(ctx, event.With("song", ids...))
			}
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
	coverClient   *http.Client
	mu            sync.Mutex
	nextAvailable time.Time
}

var metadataEnricher = newMetadataFetcher()
var artworkDownloadClient = &http.Client{Timeout: 20 * time.Second}

func newMetadataFetcher() *metadataFetcher {
	timeout := 10 * time.Second
	return &metadataFetcher{
		client:      &http.Client{Timeout: timeout},
		coverClient: &http.Client{Timeout: timeout},
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
	if strings.TrimSpace(updated.CoverArt) == "" {
		if releaseID := recording.PrimaryReleaseID(); releaseID != "" {
			if artURL, artErr := f.fetchCoverArt(ctx, releaseID); artErr == nil && artURL != "" {
				updated.ArtworkURL = artURL
			} else if artErr != nil {
				log.Warn(ctx, "Unable to fetch cover art", "songId", song.ID, "err", artErr)
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

func (f *metadataFetcher) fetchCoverArt(ctx context.Context, releaseID string) (string, error) {
	if releaseID == "" {
		return "", nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coverArtArchiveURL+releaseID, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", metadataUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := f.coverClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		log.Warn(ctx, "Cover Art Archive request failed", "status", resp.Status)
		return "", nil
	}

	var payload coverArtResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		log.Warn(ctx, "Unable to decode cover art response", "err", err)
		return "", nil
	}
	for _, image := range payload.Images {
		if image.Front && image.Image != "" {
			return ensureHTTPSURL(image.Image), nil
		}
	}
	if len(payload.Images) > 0 {
		return ensureHTTPSURL(payload.Images[0].Image), nil
	}
	return "", nil
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

func persistMetadataSong(repo model.MediaFileRepository, update metadataSongPayload) (bool, error) {
	if repo == nil {
		return false, nil
	}
	if strings.TrimSpace(update.ID) == "" {
		return false, nil
	}
	current, err := repo.Get(update.ID)
	if err != nil {
		return false, err
	}
	return persistMetadataSongWithCurrent(repo, current, update)
}

func persistMetadataSongWithCurrent(repo model.MediaFileRepository, current *model.MediaFile, update metadataSongPayload) (bool, error) {
	if repo == nil || current == nil {
		return false, nil
	}
	if !applyMetadataUpdate(current, update) {
		return false, nil
	}
	current.UpdatedAt = time.Now()
	return true, repo.Put(current)
}

func applyMetadataUpdate(mf *model.MediaFile, update metadataSongPayload) bool {
	if mf == nil {
		return false
	}
	changed := false
	if title := strings.TrimSpace(update.Title); title != "" && mf.Title != title {
		mf.Title = title
		mf.OrderTitle = str.SanitizeFieldForSorting(title)
		if mf.SortTitle == "" || mf.SortTitle == mf.Title {
			mf.SortTitle = title
		}
		changed = true
	}
	if artist := strings.TrimSpace(update.Artist); artist != "" && mf.Artist != artist {
		mf.Artist = artist
		mf.OrderArtistName = str.SanitizeFieldForSorting(artist)
		if mf.SortArtistName == "" || mf.SortArtistName == mf.Artist {
			mf.SortArtistName = artist
		}
		changed = true
	}
	if album := strings.TrimSpace(update.Album); album != "" && mf.Album != album {
		mf.Album = album
		mf.OrderAlbumName = str.SanitizeFieldForSortingNoArticle(album)
		if mf.SortAlbumName == "" || mf.SortAlbumName == mf.Album {
			mf.SortAlbumName = album
		}
		changed = true
	}
	if genre := strings.TrimSpace(update.Genre); genre != "" && mf.Genre != genre {
		mf.Genre = genre
		if mf.Tags == nil {
			mf.Tags = make(model.Tags)
		}
		mf.Tags[model.TagGenre] = []string{genre}
		tag := model.NewTag(model.TagGenre, genre)
		mf.Genres = model.Genres{{ID: tag.ID, Name: genre}}
		changed = true
	}
	if update.Year != nil {
		year := *update.Year
		if mf.Year != year {
			mf.Year = year
			changed = true
		}
	}
	return changed
}

func writeTagsToFile(ctx context.Context, mf *model.MediaFile, update metadataSongPayload) (bool, error) {
	if mf == nil {
		return false, nil
	}
	originalPath := strings.TrimSpace(mf.AbsolutePath())
	if originalPath == "" {
		return false, fmt.Errorf("media file path is empty for %s", mf.ID)
	}
	args, cleanup, err := buildExiftoolArgs(ctx, mf, update)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return false, err
	}
	if len(args) == 0 {
		return false, nil
	}
	tempPath, err := createTempCopy(originalPath)
	if err != nil {
		return false, err
	}
	replaced := false
	defer func() {
		if !replaced {
			_ = os.Remove(tempPath)
		}
	}()
	cmdArgs := append([]string{"-overwrite_original", "-q", "-q"}, args...)
	cmdArgs = append(cmdArgs, tempPath)
	cmd := exec.CommandContext(ctx, "exiftool", cmdArgs...) // #nosec G204
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("exiftool failed: %w %s", err, strings.TrimSpace(stderr.String()))
	}
	if err := replaceOriginalFile(originalPath, tempPath); err != nil {
		return false, err
	}
	replaced = true
	return true, nil
}

func buildExiftoolArgs(ctx context.Context, mf *model.MediaFile, update metadataSongPayload) ([]string, func(), error) {
	args := make([]string, 0, 6)
	cleanup := func() {}
	addField := func(flag, current, next string) {
		next = strings.TrimSpace(next)
		if next == "" {
			return
		}
		if strings.TrimSpace(current) == next {
			return
		}
		args = append(args, fmt.Sprintf("-%s=%s", flag, next))
	}
	addField("Title", mf.Title, update.Title)
	addField("Artist", mf.Artist, update.Artist)
	addField("Album", mf.Album, update.Album)
	addField("Genre", mf.Genre, update.Genre)
	if update.Year != nil {
		year := *update.Year
		if year > 0 && mf.Year != year {
			args = append(args, fmt.Sprintf("-Year=%d", year))
		}
	}
	trimmedArtwork := strings.TrimSpace(update.ArtworkURL)
	if trimmedArtwork != "" {
		artPath, err := downloadArtwork(ctx, trimmedArtwork)
		if err != nil {
			return nil, nil, err
		}
		if artPath != "" {
			cleanup = func() { _ = os.Remove(artPath) }
			args = append(args, fmt.Sprintf("-Picture=@%s", artPath))
		}
	}
	if len(args) == 0 {
		return args, nil, nil
	}
	return args, cleanup, nil
}

func downloadArtwork(ctx context.Context, artworkURL string) (string, error) {
	client := artworkDownloadClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, artworkURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unable to download artwork: %s", resp.Status)
	}
	file, err := os.CreateTemp("", "navidrome-art-")
	if err != nil {
		return "", err
	}
	defer file.Close()
	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

func createTempCopy(src string) (string, error) {
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(src)
	ext := filepath.Ext(src)
	tempName := filepath.Join(dir, fmt.Sprintf(".nd-meta-%d%s", time.Now().UnixNano(), ext))
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.OpenFile(tempName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return "", err
	}
	if err := out.Sync(); err != nil {
		return "", err
	}
	return tempName, nil
}

func replaceOriginalFile(originalPath, tempPath string) error {
	backupPath := fmt.Sprintf("%s.ndbackup", originalPath)
	if err := os.Rename(originalPath, backupPath); err != nil {
		return err
	}
	if err := os.Rename(tempPath, originalPath); err != nil {
		_ = os.Rename(backupPath, originalPath)
		return err
	}
	return os.Remove(backupPath)
}
