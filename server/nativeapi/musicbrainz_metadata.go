package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/adapters/taglibwrite"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/pmezard/go-difflib/difflib"
)

type metadataFieldProgress struct {
	Existing     int `json:"existing"`
	Missing      int `json:"missing"`
	Fetching     int `json:"fetching"`
	Fetched      int `json:"fetched"`
	Updated      int `json:"updated"`
	Left         int `json:"left"`
	CouldntFetch int `json:"couldntFetch"`
}

type musicBrainzMetadataStatus struct {
	Running       bool                  `json:"running"`
	StartedAt     *time.Time            `json:"startedAt,omitempty"`
	FinishedAt    *time.Time            `json:"finishedAt,omitempty"`
	LastError     string                `json:"lastError,omitempty"`
	Album         metadataFieldProgress `json:"album"`
	Year          metadataFieldProgress `json:"year"`
	Genre         metadataFieldProgress `json:"genre"`
	RecordingMBID metadataFieldProgress `json:"recordingMbid"`
	ReleaseMBID   metadataFieldProgress `json:"releaseMbid"`
	CoverArt      metadataFieldProgress `json:"coverArt"`
}

type musicBrainzMetadataJob struct {
	mu          sync.RWMutex
	status      musicBrainzMetadataStatus
	client      *http.Client
	coverMisses sync.Map

	itunesMu       sync.Mutex
	itunesInterval time.Duration
	lastITunesCall time.Time
}

func newMusicBrainzMetadataJob() *musicBrainzMetadataJob {
	return &musicBrainzMetadataJob{
		client:         &http.Client{Timeout: 15 * time.Second},
		itunesInterval: 3 * time.Second,
	}
}

func (j *musicBrainzMetadataJob) getStatus() musicBrainzMetadataStatus {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.status
}

func (j *musicBrainzMetadataJob) start(ds model.DataStore, songIDs []string) bool {
	j.mu.Lock()
	if j.status.Running {
		j.mu.Unlock()
		return false
	}
	now := time.Now()
	j.status = musicBrainzMetadataStatus{Running: true, StartedAt: &now}
	j.mu.Unlock()

	go j.run(ds, songIDs)
	return true
}

type missingFlags struct {
	album         bool
	year          bool
	genre         bool
	recordingMBID bool
	releaseMBID   bool
	coverArt      bool
}

type mbMetadataCandidate struct {
	mf    model.MediaFile
	flags missingFlags
}

type metadataResult struct {
	Album         string
	Year          int
	Genre         string
	GenreTrace    *genreSourceDeveloperTrace
	RecordingMBID string
	ReleaseMBID   string
}

type spotifyMetadataStatus struct {
	Running    bool                  `json:"running"`
	StartedAt  *time.Time            `json:"startedAt,omitempty"`
	FinishedAt *time.Time            `json:"finishedAt,omitempty"`
	LastError  string                `json:"lastError,omitempty"`
	Album      metadataFieldProgress `json:"album"`
	CoverArt   metadataFieldProgress `json:"coverArt"`
}

type spotifyConfidenceEntry struct {
	SongID        string  `json:"songId"`
	Title         string  `json:"title"`
	Artist        string  `json:"artist"`
	Confidence    float64 `json:"confidence"`
	Album         string  `json:"album"`
	SpotifyMatch  string  `json:"spotifyMatch,omitempty"`
	SpotifyArtist string  `json:"spotifyArtist,omitempty"`
	SpotifyURL    string  `json:"spotifyUrl,omitempty"`
	CoverURL      string  `json:"coverUrl,omitempty"`
	Downloaded    bool    `json:"downloaded"`
}

type spotifyMetadataJob struct {
	mu               sync.RWMutex
	status           spotifyMetadataStatus
	client           *http.Client
	entries          map[string]spotifyConfidenceEntry
	coverMisses      sync.Map
	artistGenreCache sync.Map
	token            string
	tokenExpiresAt   time.Time
}

type metadataSaveSummary struct {
	Saved     int `json:"saved"`
	Remaining int `json:"remaining"`
	Saving    int `json:"saving"`
}

type selectedSongsPayload struct {
	SongIDs []string `json:"songIds"`
}

type spotifyCoverUpdatePayload struct {
	SongID      string   `json:"songId"`
	SongIDs     []string `json:"songIds"`
	SpotifyURL  string   `json:"spotifyUrl"`
	SpotifyLink string   `json:"spotifyLink"`
}

func decodeSelectedSongIDs(r *http.Request) ([]string, error) {
	if r == nil || r.Body == nil {
		return nil, nil
	}

	payload := selectedSongsPayload{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}

	return payload.SongIDs, nil
}

func newSpotifyMetadataJob() *spotifyMetadataJob {
	return &spotifyMetadataJob{
		client:  &http.Client{Timeout: 15 * time.Second},
		entries: map[string]spotifyConfidenceEntry{},
	}
}

func (j *musicBrainzMetadataJob) run(ds model.DataStore, songIDs []string) {
	ctx := context.Background()
	defer func() {
		j.mu.Lock()
		j.status.Running = false
		now := time.Now()
		j.status.FinishedAt = &now
		j.mu.Unlock()
	}()

	candidates, err := j.collectCandidates(ctx, ds, songIDs)
	if err != nil {
		j.setError(err)
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	missingAlbums := 0
	fetched := 0
	updated := 0
	skipped := 0
	failed := 0

	for _, c := range candidates {
		if c.flags.album {
			missingAlbums++
		}
	}

	for i, c := range candidates {
		if i > 0 {
			<-ticker.C
		}
		j.setFetching(c.flags, 1)

		metadata, fetchErr := j.fetchMetadata(itunesTrackQueryFor(c.mf))
		if fetchErr != nil {
			failed++
			log.Warn(ctx, "Could not fetch metadata from MusicBrainz", "songId", c.mf.ID, "title", c.mf.Title, "artist", c.mf.Artist, fetchErr)
			j.finishFetch(c.flags, false, false, false, false, false, false)
			continue
		}

		fetched++

		var setAlbum *string
		var setYear *int
		var setGenre *string

		if c.flags.album && metadata.Album != "" {
			setAlbum = &metadata.Album
		}
		if c.flags.year && metadata.Year > 0 {
			setYear = &metadata.Year
		}
		if c.flags.genre && metadata.Genre != "" {
			setGenre = &metadata.Genre
		}

		if setAlbum != nil || setYear != nil || setGenre != nil || metadata.RecordingMBID != "" || metadata.ReleaseMBID != "" {
			if err := ds.MediaFile(ctx).UpdateMissingMetadata(c.mf.ID, setAlbum, setYear, setGenre, valueOrNil(metadata.RecordingMBID), valueOrNil(metadata.ReleaseMBID)); err != nil {
				failed++
				log.Error(ctx, "Could not update fetched MusicBrainz metadata", "songId", c.mf.ID, err)
			} else {
				updated++
				j.setUpdated(setAlbum != nil, setYear != nil, setGenre != nil, c.flags.recordingMBID && metadata.RecordingMBID != "", c.flags.releaseMBID && metadata.ReleaseMBID != "", false)
			}
		} else {
			skipped++
		}

		coverFetched := false
		coverUpdated := false
		if c.flags.coverArt && !c.mf.HasCoverArt && strings.TrimSpace(c.mf.CoverPath) == "" {
			releaseMBID := strings.TrimSpace(c.mf.MbzReleaseID)
			if releaseMBID == "" {
				releaseMBID = strings.TrimSpace(metadata.ReleaseMBID)
			}
			if releaseMBID != "" && releaseMBIDRegex.MatchString(releaseMBID) {
				relPath, ok := j.ensureReleaseCover(ctx, releaseMBID)
				if ok {
					coverFetched = true
					if err := ds.MediaFile(ctx).UpdateCoverPath(c.mf.ID, relPath); err != nil {
						log.Warn(ctx, "Could not update cover path", "songId", c.mf.ID, "releaseMBID", releaseMBID, "err", err)
					} else {
						coverUpdated = true
					}
				}
			}
		}

		if coverUpdated {
			j.setUpdated(false, false, false, false, false, true)
		}
		j.finishFetch(c.flags, metadata.Album != "", metadata.Year > 0, metadata.Genre != "", metadata.RecordingMBID != "", metadata.ReleaseMBID != "", coverFetched)
	}

	log.Info(ctx, "MusicBrainz metadata fetch completed",
		"missingAlbums", missingAlbums,
		"fetched", fetched,
		"updated", updated,
		"skipped", skipped,
		"failed", failed,
	)
}

func (j *musicBrainzMetadataJob) collectCandidates(ctx context.Context, ds model.DataStore, songIDs []string) ([]mbMetadataCandidate, error) {
	cursor, err := ds.MediaFile(ctx).GetCursor()
	if err != nil {
		return nil, err
	}

	allowedSongs := make(map[string]struct{}, len(songIDs))
	for _, id := range songIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			allowedSongs[trimmed] = struct{}{}
		}
	}
	filterBySongIDs := len(allowedSongs) > 0

	res := make([]mbMetadataCandidate, 0)
	for mf, e := range cursor {
		if e != nil {
			return nil, e
		}
		if strings.TrimSpace(mf.Title) == "" || strings.TrimSpace(mf.Artist) == "" {
			continue
		}
		if filterBySongIDs {
			if _, ok := allowedSongs[mf.ID]; !ok {
				continue
			}
		}

		flags := missingFlags{
			album:         isMissingAlbum(mf.Album),
			year:          mf.Year == 0,
			genre:         strings.TrimSpace(mf.Genre) == "",
			recordingMBID: strings.TrimSpace(mf.MbzRecordingID) == "",
			releaseMBID:   strings.TrimSpace(mf.MbzReleaseID) == "",
			coverArt:      hasMissingCoverArt(mf),
		}

		if !flags.coverArt {
			continue
		}

		j.incrementExisting(flags)
		if !flags.album && !flags.year && !flags.genre && !flags.recordingMBID && !flags.releaseMBID && !flags.coverArt {
			continue
		}

		res = append(res, mbMetadataCandidate{mf: mf, flags: flags})
		j.incrementMissing(flags)
	}
	return res, nil
}

func hasMissingCoverArt(mf model.MediaFile) bool {
	return !mf.HasCoverArt && strings.TrimSpace(mf.CoverPath) == ""
}

func (j *musicBrainzMetadataJob) incrementExisting(flags missingFlags) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if !flags.album {
		j.status.Album.Existing++
	}
	if !flags.year {
		j.status.Year.Existing++
	}
	if !flags.genre {
		j.status.Genre.Existing++
	}
	if !flags.recordingMBID {
		j.status.RecordingMBID.Existing++
	}
	if !flags.releaseMBID {
		j.status.ReleaseMBID.Existing++
	}
	if !flags.coverArt {
		j.status.CoverArt.Existing++
	}
}

func isMissingAlbum(album string) bool {
	album = normalizeMBString(album)
	return album == "" || album == "unknown album"
}

func (j *musicBrainzMetadataJob) incrementMissing(flags missingFlags) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if flags.album {
		j.status.Album.Missing++
		j.status.Album.Left++
	}
	if flags.year {
		j.status.Year.Missing++
		j.status.Year.Left++
	}
	if flags.genre {
		j.status.Genre.Missing++
		j.status.Genre.Left++
	}
	if flags.recordingMBID {
		j.status.RecordingMBID.Missing++
		j.status.RecordingMBID.Left++
	}
	if flags.releaseMBID {
		j.status.ReleaseMBID.Missing++
		j.status.ReleaseMBID.Left++
	}
	if flags.coverArt {
		j.status.CoverArt.Missing++
		j.status.CoverArt.Left++
	}
}

func (j *musicBrainzMetadataJob) setFetching(flags missingFlags, delta int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if flags.album {
		j.status.Album.Fetching += delta
	}
	if flags.year {
		j.status.Year.Fetching += delta
	}
	if flags.genre {
		j.status.Genre.Fetching += delta
	}
	if flags.recordingMBID {
		j.status.RecordingMBID.Fetching += delta
	}
	if flags.releaseMBID {
		j.status.ReleaseMBID.Fetching += delta
	}
	if flags.coverArt {
		j.status.CoverArt.Fetching += delta
	}
}

func (j *musicBrainzMetadataJob) setUpdated(album, year, genre, recordingMBID, releaseMBID, coverArt bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if album {
		j.status.Album.Updated++
	}
	if year {
		j.status.Year.Updated++
	}
	if genre {
		j.status.Genre.Updated++
	}
	if recordingMBID {
		j.status.RecordingMBID.Updated++
	}
	if releaseMBID {
		j.status.ReleaseMBID.Updated++
	}
	if coverArt {
		j.status.CoverArt.Updated++
	}
}

func (j *musicBrainzMetadataJob) finishFetch(flags missingFlags, fetchedAlbum, fetchedYear, fetchedGenre, fetchedRecordingMBID, fetchedReleaseMBID, fetchedCoverArt bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if flags.album {
		j.status.Album.Fetching--
		j.status.Album.Left--
		if fetchedAlbum {
			j.status.Album.Fetched++
		} else {
			j.status.Album.CouldntFetch++
		}
	}
	if flags.year {
		j.status.Year.Fetching--
		j.status.Year.Left--
		if fetchedYear {
			j.status.Year.Fetched++
		} else {
			j.status.Year.CouldntFetch++
		}
	}
	if flags.genre {
		j.status.Genre.Fetching--
		j.status.Genre.Left--
		if fetchedGenre {
			j.status.Genre.Fetched++
		} else {
			j.status.Genre.CouldntFetch++
		}
	}
	if flags.recordingMBID {
		j.status.RecordingMBID.Fetching--
		j.status.RecordingMBID.Left--
		if fetchedRecordingMBID {
			j.status.RecordingMBID.Fetched++
		} else {
			j.status.RecordingMBID.CouldntFetch++
		}
	}
	if flags.releaseMBID {
		j.status.ReleaseMBID.Fetching--
		j.status.ReleaseMBID.Left--
		if fetchedReleaseMBID {
			j.status.ReleaseMBID.Fetched++
		} else {
			j.status.ReleaseMBID.CouldntFetch++
		}
	}
	if flags.coverArt {
		j.status.CoverArt.Fetching--
		j.status.CoverArt.Left--
		if fetchedCoverArt {
			j.status.CoverArt.Fetched++
		} else {
			j.status.CoverArt.CouldntFetch++
		}
	}
}

func (j *musicBrainzMetadataJob) setError(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status.LastError = err.Error()
}

func (j *musicBrainzMetadataJob) ensureReleaseCover(ctx context.Context, releaseMBID string) (string, bool) {
	if _, err := os.Stat(filepath.Join(conf.Server.DataFolder.String(), coverCacheDirName)); err != nil {
		if err := os.MkdirAll(filepath.Join(conf.Server.DataFolder.String(), coverCacheDirName), 0o755); err != nil {
			log.Warn(ctx, "Could not create cover cache directory", "err", err)
			return "", false
		}
	}

	coverPath := filepath.Join(conf.Server.DataFolder.String(), coverCacheDirName, releaseMBID+".jpg")
	if _, err := os.Stat(coverPath); err == nil {
		return filepath.ToSlash(filepath.Join(coverCacheDirName, releaseMBID+".jpg")), true
	}
	if _, missed := j.coverMisses.Load(releaseMBID); missed {
		return "", false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://coverartarchive.org/release/"+releaseMBID+"/front-500", nil)
	if err != nil {
		log.Warn(ctx, "Could not build cover request", "releaseMBID", releaseMBID, "err", err)
		return "", false
	}
	req.Header.Set("User-Agent", "Navidrome/metadata-fetcher (https://www.navidrome.org)")

	resp, err := j.client.Do(req)
	if err != nil {
		log.Warn(ctx, "Could not fetch cover art", "releaseMBID", releaseMBID, "err", err)
		return "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		j.coverMisses.Store(releaseMBID, true)
		return "", false
	}
	if resp.StatusCode != http.StatusOK {
		log.Warn(ctx, "Unexpected cover status", "releaseMBID", releaseMBID, "status", resp.StatusCode)
		return "", false
	}

	tmpPath := coverPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		log.Warn(ctx, "Could not create cover file", "path", tmpPath, "err", err)
		return "", false
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		log.Warn(ctx, "Could not save cover file", "path", tmpPath, "err", err)
		return "", false
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", false
	}
	if err := os.Rename(tmpPath, coverPath); err != nil {
		_ = os.Remove(tmpPath)
		log.Warn(ctx, "Could not store cover file", "path", coverPath, "err", err)
		return "", false
	}

	return filepath.ToSlash(filepath.Join(coverCacheDirName, releaseMBID+".jpg")), true
}

type mbSearchResponse struct {
	Recordings []mbRecording `json:"recordings"`
}

type mbRecording struct {
	ID               string  `json:"id"`
	Score            mbScore `json:"score"`
	FirstReleaseDate string  `json:"first-release-date"`
	ArtistCredit     []struct {
		Name string `json:"name"`
	} `json:"artist-credit"`
	Releases []mbRelease `json:"releases"`
}

type mbRelease struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Date         string  `json:"date"`
	Status       string  `json:"status"`
	Country      string  `json:"country"`
	ReleaseGroup mbGroup `json:"release-group"`
}

type mbScore string

func (s *mbScore) UnmarshalJSON(data []byte) error {
	v := strings.TrimSpace(string(data))
	if v == "" || v == "null" {
		*s = ""
		return nil
	}
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		*s = mbScore(strings.TrimSpace(v[1 : len(v)-1]))
		return nil
	}
	*s = mbScore(v)
	return nil
}

func (s mbScore) Int() int {
	v := strings.TrimSpace(string(s))
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return i
}

type mbGroup struct {
	PrimaryType   string   `json:"primary-type"`
	SecondaryType []string `json:"secondary-types"`
}

func valueOrNil(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

func floatValueOrNil(v float64) *float64 {
	if v <= 0 {
		return nil
	}
	return &v
}

var featSplitRegex = regexp.MustCompile(`(?i)\s+(feat\.?|ft\.?|featuring)\s+`)

// primarySearchArtist reduces a multi-artist tag like "JAY-Z, Beyoncé" or
// "The Carters (Beyonce & Jay-Z)" to its first credited artist. Local tags
// join collaborators with separators that never appear inside a MusicBrainz
// artist credit, so searching the full string as a quoted phrase finds nothing.
func primarySearchArtist(artist string) string {
	artist = strings.TrimSpace(artist)
	if i := strings.IndexAny(artist, ",;("); i >= 0 {
		artist = artist[:i]
	}
	artist = featSplitRegex.Split(artist, 2)[0]
	return strings.TrimSpace(artist)
}

// escapeLucenePhrase escapes the characters that carry meaning inside a quoted
// Lucene phrase so titles like `He said "no"` don't break the MusicBrainz query.
func escapeLucenePhrase(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	return strings.ReplaceAll(v, `"`, `\"`)
}

func (j *musicBrainzMetadataJob) searchRecordings(title, artist string) ([]mbRecording, error) {
	// Quoted phrases keep Lucene from splitting the field query on every space
	// and matching unrelated recordings that share a single word.
	query := fmt.Sprintf(`artist:"%s" AND recording:"%s"`, escapeLucenePhrase(artist), escapeLucenePhrase(title))
	u := "https://musicbrainz.org/ws/2/recording/?query=" + url.QueryEscape(query) + "&fmt=json&inc=releases+release-groups"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Navidrome/metadata-fetcher (https://www.navidrome.org)")

	resp, err := j.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("musicbrainz status %d", resp.StatusCode)
	}

	var payload mbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Recordings, nil
}

func (j *musicBrainzMetadataJob) fetchMetadata(q itunesTrackQuery) (metadataResult, error) {
	title, artist := q.Title, q.Artist
	recordings, err := j.searchRecordings(title, artist)
	if err != nil {
		return metadataResult{}, err
	}

	bestCandidates := collectReleaseCandidates(recordings, artist)
	if len(bestCandidates) == 0 {
		// Trailing "(Live)"/"[Remastered]"-style title tags and multi-artist
		// strings like "JAY-Z, Beyoncé" never match the quoted phrase query
		// (MusicBrainz credits collaborations as "JAY‐Z feat. Beyoncé"); retry
		// once with a simplified title and the primary artist only. The full
		// artist string still validates candidates via artistCreditMatches.
		retryTitle := sanitizeSpotifyRetryTitle(title)
		if retryTitle == "" {
			retryTitle = strings.TrimSpace(title)
		}
		retryArtist := primarySearchArtist(artist)
		if retryArtist == "" {
			retryArtist = strings.TrimSpace(artist)
		}
		if !strings.EqualFold(retryTitle, strings.TrimSpace(title)) || !strings.EqualFold(retryArtist, strings.TrimSpace(artist)) {
			time.Sleep(time.Second) // MusicBrainz allows 1 request per second
			if retryRecordings, retryErr := j.searchRecordings(retryTitle, retryArtist); retryErr == nil {
				bestCandidates = collectReleaseCandidates(retryRecordings, artist)
			}
		}
	}
	// Genre comes from iTunes, not MusicBrainz: Apple's editorial per-track
	// genre is reliable, and the lookup works even when no MusicBrainz release
	// matched, so collaboration tracks still get a genre.
	result := metadataResult{Genre: j.fetchITunesGenre(q)}

	if len(bestCandidates) == 0 {
		return result, nil
	}

	coverCache := make(map[string]bool, len(bestCandidates))
	best := selectBestReleaseCandidate(bestCandidates, func(releaseID string) bool {
		return j.releaseHasCover(releaseID, coverCache)
	})
	if best == nil {
		return result, nil
	}

	result.Album = strings.TrimSpace(best.release.Title)
	result.Year = yearFromDate(best.release.Date)
	if result.Year == 0 {
		result.Year = yearFromDate(best.recording.FirstReleaseDate)
	}
	result.RecordingMBID = strings.TrimSpace(best.recording.ID)
	result.ReleaseMBID = strings.TrimSpace(best.release.ID)
	return result, nil
}

func (j *musicBrainzMetadataJob) releaseHasCover(releaseID string, cache map[string]bool) bool {
	releaseID = strings.TrimSpace(releaseID)
	if releaseID == "" {
		return false
	}
	if cached, ok := cache[releaseID]; ok {
		return cached
	}

	u := "https://coverartarchive.org/release/" + url.PathEscape(releaseID)
	req, err := http.NewRequest(http.MethodHead, u, nil)
	if err != nil {
		cache[releaseID] = false
		return false
	}
	req.Header.Set("User-Agent", "Navidrome/metadata-fetcher (https://www.navidrome.org)")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := j.client.Do(req)
	if err != nil {
		cache[releaseID] = false
		return false
	}
	defer resp.Body.Close()

	hasCover := resp.StatusCode == http.StatusOK
	cache[releaseID] = hasCover
	return hasCover
}

func selectBestRecording(recordings []mbRecording, artist string) *mbRecording {
	normalizedArtist := normalizeMBString(artist)

	type recCandidate struct {
		rec      *mbRecording
		score    int
		releases int
		hasAlbum bool
		earliest int
	}

	candidates := make([]recCandidate, 0, len(recordings))
	for i := range recordings {
		rec := &recordings[i]
		score := rec.Score.Int()
		if score < 80 {
			continue
		}
		if !artistCreditMatches(rec.ArtistCredit, normalizedArtist) {
			continue
		}

		hasAlbum := false
		earliest := 9999
		for _, rel := range rec.Releases {
			if !isValidRelease(rel) {
				continue
			}
			if isAlbumRelease(rel) {
				hasAlbum = true
			}
			if y := yearFromDate(rel.Date); y > 0 && y < earliest {
				earliest = y
			}
		}
		candidates = append(candidates, recCandidate{rec: rec, score: score, releases: len(rec.Releases), hasAlbum: hasAlbum, earliest: earliest})
	}
	if len(candidates) == 0 {
		return nil
	}

	slices.SortFunc(candidates, func(a, b recCandidate) int {
		if a.releases > 0 && b.releases == 0 {
			return -1
		}
		if a.releases == 0 && b.releases > 0 {
			return 1
		}
		if a.score != b.score {
			return b.score - a.score
		}
		if a.hasAlbum && !b.hasAlbum {
			return -1
		}
		if !a.hasAlbum && b.hasAlbum {
			return 1
		}
		if a.earliest != b.earliest {
			return a.earliest - b.earliest
		}
		return 0
	})

	return candidates[0].rec
}

func artistCreditMatches(credits []struct {
	Name string `json:"name"`
}, normalizedArtist string) bool {
	if normalizedArtist == "" {
		return false
	}
	names := make([]string, 0, len(credits))
	for _, credit := range credits {
		name := normalizeMBString(credit.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
		// A single credit matching the whole local artist, or being one of the
		// artists in a local "A feat. B" string, is a match.
		if name == normalizedArtist || containsAllTokens(normalizedArtist, name) {
			return true
		}
	}
	// Local tag may credit only the primary artist of a collaboration.
	if len(names) > 1 {
		joined := strings.Join(names, " ")
		if containsAllTokens(joined, normalizedArtist) || stringSimilarity(joined, normalizedArtist) >= 0.8 {
			return true
		}
	}
	return false
}

// containsAllTokens reports whether every word of needle appears in haystack,
// so "daft punk" matches "daft punk feat pharrell" without the false positives
// of raw substring containment.
func containsAllTokens(haystack, needle string) bool {
	if needle == "" || haystack == "" {
		return false
	}
	words := map[string]bool{}
	for _, w := range strings.Fields(haystack) {
		words[w] = true
	}
	for _, w := range strings.Fields(needle) {
		if !words[w] {
			return false
		}
	}
	return true
}

type releaseCandidate struct {
	recording *mbRecording
	release   *mbRelease
}

func collectReleaseCandidates(recordings []mbRecording, artist string) []releaseCandidate {
	bestRecording := selectBestRecording(recordings, artist)
	if bestRecording == nil {
		return nil
	}

	matches := make([]releaseCandidate, 0, len(bestRecording.Releases))
	fallback := make([]releaseCandidate, 0, len(bestRecording.Releases))
	for i := range bestRecording.Releases {
		release := &bestRecording.Releases[i]
		if !isValidRelease(*release) {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(release.Status), "Official") {
			continue
		}

		candidate := releaseCandidate{recording: bestRecording, release: release}
		fallback = append(fallback, candidate)
		if isAlbumRelease(*release) && !hasDiscouragedSecondaryType(*release) {
			matches = append(matches, candidate)
		}
	}

	if len(matches) > 0 {
		return matches
	}
	return fallback
}

func selectBestReleaseCandidate(candidates []releaseCandidate, hasCover func(releaseID string) bool) *releaseCandidate {
	type relCandidate struct {
		candidate *releaseCandidate
		hasCover  bool
		usCountry bool
		year      int
	}

	sortedCandidates := make([]relCandidate, 0, len(candidates))
	for i := range candidates {
		candidate := &candidates[i]
		releaseID := strings.TrimSpace(candidate.release.ID)
		sortedCandidates = append(sortedCandidates, relCandidate{
			candidate: candidate,
			hasCover:  hasCover(releaseID),
			usCountry: strings.EqualFold(strings.TrimSpace(candidate.release.Country), "US"),
			year:      yearFromDate(candidate.release.Date),
		})
	}
	if len(sortedCandidates) == 0 {
		return nil
	}

	slices.SortFunc(sortedCandidates, func(a, b relCandidate) int {
		if a.hasCover && !b.hasCover {
			return -1
		}
		if !a.hasCover && b.hasCover {
			return 1
		}
		if a.year == 0 && b.year > 0 {
			return 1
		}
		if b.year == 0 && a.year > 0 {
			return -1
		}
		if a.year != b.year {
			return a.year - b.year
		}
		if a.usCountry && !b.usCountry {
			return -1
		}
		if !a.usCountry && b.usCountry {
			return 1
		}
		return 0
	})

	return sortedCandidates[0].candidate
}

func isValidRelease(release mbRelease) bool {
	if strings.TrimSpace(release.Title) == "" {
		return false
	}
	for _, t := range release.ReleaseGroup.SecondaryType {
		n := normalizeMBString(t)
		if n == "unofficial" || n == "promo" {
			return false
		}
	}
	return true
}

func hasDiscouragedSecondaryType(release mbRelease) bool {
	for _, t := range release.ReleaseGroup.SecondaryType {
		n := normalizeMBString(t)
		if n == "compilation" || n == "live" {
			return true
		}
	}
	return false
}

func isAlbumRelease(release mbRelease) bool {
	primaryType := normalizeMBString(release.ReleaseGroup.PrimaryType)
	return primaryType == "album"
}

var punctuationRegex = regexp.MustCompile(`[\p{P}\p{S}]`)
var spotifyParenRegex = regexp.MustCompile(`\(.*?\)`)
var spotifyBracketRegex = regexp.MustCompile(`\[.*?\]`)
var spotifyFeatRegex = regexp.MustCompile(`feat\.?|ft\.?`)
var spotifyNonAlphaNumSpaceRegex = regexp.MustCompile(`[^a-z0-9 ]`)
var spotifyTrackURLRegex = regexp.MustCompile(`open\.spotify\.com/track/([A-Za-z0-9]+)`)

var errSpotifyTokenExpired = errors.New("spotify access token expired")

func normalizeMBString(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = punctuationRegex.ReplaceAllString(v, " ")
	v = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, v)
	return strings.Join(strings.Fields(v), " ")
}

func normalizeSpotifyString(v string) string {
	v = strings.ToLower(v)
	v = spotifyParenRegex.ReplaceAllString(v, "")
	v = spotifyBracketRegex.ReplaceAllString(v, "")
	v = spotifyFeatRegex.ReplaceAllString(v, "")
	v = spotifyNonAlphaNumSpaceRegex.ReplaceAllString(v, "")
	return strings.TrimSpace(v)
}

func yearFromDate(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return y
}

func (j *spotifyMetadataJob) getStatus() spotifyMetadataStatus {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.status
}

func (j *spotifyMetadataJob) getConfidenceEntries(ctx context.Context, ds model.DataStore) []spotifyConfidenceEntry {
	cursor, err := ds.MediaFile(ctx).GetCursor()
	if err != nil {
		return nil
	}

	res := make([]spotifyConfidenceEntry, 0)
	for mf, e := range cursor {
		if e != nil {
			continue
		}
		spotifyMatch := strings.TrimSpace(mf.SpotifyMatch)
		spotifyURL := strings.TrimSpace(mf.SpotifyURL)
		if spotifyMatch == "" && spotifyURL == "" && mf.SpotifyConfidence <= 0 {
			continue
		}
		res = append(res, spotifyConfidenceEntry{
			SongID:        mf.ID,
			Title:         mf.Title,
			Artist:        mf.Artist,
			Confidence:    mf.SpotifyConfidence,
			Album:         mf.Album,
			SpotifyMatch:  spotifyMatch,
			SpotifyArtist: strings.TrimSpace(mf.SpotifyArtist),
			SpotifyURL:    spotifyURL,
			Downloaded:    strings.TrimSpace(mf.CoverPath) != "",
		})
	}

	slices.SortFunc(res, func(a, b spotifyConfidenceEntry) int {
		return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	})
	return res
}

func (j *spotifyMetadataJob) start(ds model.DataStore, songIDs []string) bool {
	j.mu.Lock()
	if j.status.Running {
		j.mu.Unlock()
		return false
	}
	now := time.Now()
	j.status = spotifyMetadataStatus{Running: true, StartedAt: &now}
	j.entries = map[string]spotifyConfidenceEntry{}
	j.mu.Unlock()

	go j.run(ds, songIDs)
	return true
}

func (j *spotifyMetadataJob) run(ds model.DataStore, songIDs []string) {
	ctx := context.Background()
	defer func() {
		j.mu.Lock()
		j.status.Running = false
		now := time.Now()
		j.status.FinishedAt = &now
		j.mu.Unlock()
	}()

	token, err := j.getToken(ctx)
	if err != nil {
		j.setError(err)
		return
	}

	cursor, err := ds.MediaFile(ctx).GetCursor()
	if err != nil {
		j.setError(err)
		return
	}

	allowedSongs := make(map[string]struct{}, len(songIDs))
	for _, id := range songIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			allowedSongs[trimmed] = struct{}{}
		}
	}
	filterBySongIDs := len(allowedSongs) > 0

	candidates := make([]model.MediaFile, 0)
	for mf, e := range cursor {
		if e != nil {
			j.setError(e)
			return
		}
		if strings.TrimSpace(mf.Title) == "" || strings.TrimSpace(mf.Artist) == "" {
			continue
		}
		if filterBySongIDs {
			if _, ok := allowedSongs[mf.ID]; !ok {
				continue
			}
		}
		missingCover := !mf.HasCoverArt && strings.TrimSpace(mf.CoverPath) == ""
		if !missingCover {
			j.mu.Lock()
			j.status.CoverArt.Existing++
			j.mu.Unlock()
			continue
		}
		j.mu.Lock()
		j.status.CoverArt.Missing++
		j.status.CoverArt.Left++
		if isMissingAlbum(mf.Album) {
			j.status.Album.Missing++
			j.status.Album.Left++
		} else {
			j.status.Album.Existing++
		}
		j.mu.Unlock()
		candidates = append(candidates, mf)
	}

	ticker := time.NewTicker(1400 * time.Millisecond)
	defer ticker.Stop()
	for i, mf := range candidates {
		if i > 0 {
			<-ticker.C
		}
		j.setFetching(true, isMissingAlbum(mf.Album), 1)

		track, confidence, searchErr := j.searchBestTrack(ctx, token, mf)
		if searchErr != nil && errors.Is(searchErr, errSpotifyTokenExpired) {
			token, err = j.refreshToken(ctx)
			if err != nil {
				j.setError(err)
				return
			}
			track, confidence, searchErr = j.searchBestTrack(ctx, token, mf)
		}
		if searchErr != nil || track == nil {
			if searchErr != nil {
				log.Debug(ctx, "Spotify metadata fetch failed", "songId", mf.ID, "title", mf.Title, "artist", mf.Artist, "err", searchErr)
			} else {
				log.Debug(ctx, "Spotify metadata fetch found no match", "songId", mf.ID, "title", mf.Title, "artist", mf.Artist)
			}
			j.finishFetch(true, isMissingAlbum(mf.Album), false, false)
			continue
		}

		albumName := strings.TrimSpace(track.Album.Name)
		releaseYear := spotifyReleaseYear(track.Album.ReleaseDate)
		coverURL := ""
		if len(track.Album.Images) > 0 {
			coverURL = strings.TrimSpace(track.Album.Images[0].URL)
		}

		setAlbum := isMissingAlbum(mf.Album) && albumName != ""
		setYear := mf.Year == 0 && releaseYear > 0
		if setAlbum || setYear {
			var year *int
			if setYear {
				year = &releaseYear
			}
			if err := ds.MediaFile(ctx).UpdateMissingMetadata(mf.ID, valueOrNil(albumName), year, nil, nil, nil); err == nil {
				j.setUpdated(setAlbum, false)
			} else {
				setAlbum = false
				setYear = false
			}
		}

		downloaded := false
		if confidence > conf.Server.Spotify.MinScore && coverURL != "" {
			if relPath, ok := j.ensureSpotifyCover(ctx, track.ID, coverURL, false); ok {
				if err := ds.MediaFile(ctx).UpdateCoverPath(mf.ID, relPath); err == nil {
					downloaded = true
					j.setUpdated(false, true)
				}
			}
		} else {
			log.Debug(ctx, "Spotify metadata fetch skipped cover download", "songId", mf.ID, "trackId", track.ID, "confidence", confidence, "minScore", conf.Server.Spotify.MinScore, "coverURLPresent", coverURL != "")
		}

		spotifyMatch := strings.TrimSpace(track.Name)
		spotifyArtist := spotifyPrimaryArtist(*track)
		spotifyURL := "https://open.spotify.com/track/" + strings.TrimSpace(track.ID)
		if err := ds.MediaFile(ctx).UpdateSpotifyMetadata(mf.ID, floatValueOrNil(confidence), valueOrNil(spotifyMatch), valueOrNil(spotifyArtist), valueOrNil(spotifyURL)); err != nil {
			log.Debug(ctx, "Could not persist Spotify confidence metadata", "songId", mf.ID, "err", err)
		}

		j.storeEntry(spotifyConfidenceEntry{
			SongID:        mf.ID,
			Title:         mf.Title,
			Artist:        mf.Artist,
			Confidence:    confidence,
			Album:         albumName,
			SpotifyMatch:  spotifyMatch,
			SpotifyArtist: spotifyArtist,
			SpotifyURL:    spotifyURL,
			CoverURL:      coverURL,
			Downloaded:    downloaded,
		})

		j.finishFetch(downloaded || coverURL != "", isMissingAlbum(mf.Album), setAlbum || albumName != "", downloaded)
	}
}

func (j *spotifyMetadataJob) setError(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status.LastError = err.Error()
}

func (j *spotifyMetadataJob) setFetching(cover, album bool, delta int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if cover {
		j.status.CoverArt.Fetching += delta
	}
	if album {
		j.status.Album.Fetching += delta
	}
}

func (j *spotifyMetadataJob) setUpdated(album, cover bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if album {
		j.status.Album.Updated++
	}
	if cover {
		j.status.CoverArt.Updated++
	}
}

func (j *spotifyMetadataJob) finishFetch(coverProcessed, albumMissing, albumFetched, coverFetched bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status.CoverArt.Fetching--
	j.status.CoverArt.Left--
	if coverFetched {
		j.status.CoverArt.Fetched++
	} else if coverProcessed {
		j.status.CoverArt.CouldntFetch++
	}
	if albumMissing {
		j.status.Album.Fetching--
		j.status.Album.Left--
		if albumFetched {
			j.status.Album.Fetched++
		} else {
			j.status.Album.CouldntFetch++
		}
	}
}

func (j *spotifyMetadataJob) storeEntry(entry spotifyConfidenceEntry) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.entries[entry.SongID] = entry
}

func (j *spotifyMetadataJob) getToken(ctx context.Context) (string, error) {
	if token := strings.TrimSpace(conf.Server.Spotify.Token); token != "" {
		return token, nil
	}

	j.mu.RLock()
	cachedToken := strings.TrimSpace(j.token)
	expiresAt := j.tokenExpiresAt
	j.mu.RUnlock()
	if cachedToken != "" && time.Now().Before(expiresAt.Add(-15*time.Second)) {
		return cachedToken, nil
	}

	return j.refreshToken(ctx)
}

func (j *spotifyMetadataJob) refreshToken(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", conf.Server.Spotify.ID)
	form.Set("client_secret", conf.Server.Spotify.Secret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://accounts.spotify.com/api/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := j.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("spotify token status: %d, body: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return "", fmt.Errorf("spotify access token is empty")
	}
	expiresIn := payload.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	j.mu.Lock()
	j.token = strings.TrimSpace(payload.AccessToken)
	j.tokenExpiresAt = expiresAt
	j.mu.Unlock()

	return strings.TrimSpace(payload.AccessToken), nil
}

type spotifySearchResponse struct {
	Tracks struct {
		Items []spotifyTrack `json:"items"`
	} `json:"tracks"`
}

type spotifyTrack struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	DurationMS int    `json:"duration_ms"`
	Artists    []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		ReleaseDate string `json:"release_date"`
		Images      []struct {
			URL string `json:"url"`
		} `json:"images"`
	} `json:"album"`
}

func spotifyReleaseYear(releaseDate string) int {
	releaseDate = strings.TrimSpace(releaseDate)
	if len(releaseDate) < 4 {
		return 0
	}
	year, err := strconv.Atoi(releaseDate[:4])
	if err != nil || year <= 0 {
		return 0
	}
	return year
}

func sanitizeSpotifyRetryTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	trailingParen := regexp.MustCompile(`\s*\([^)]*\)\s*$`)
	trailingBracket := regexp.MustCompile(`\s*\[[^\]]*\]\s*$`)

	stripped := title

	// Keep removing trailing tags (handles multiple like "(Live) (Remix)")
	for {
		newStr := trailingParen.ReplaceAllString(stripped, "")
		newStr = trailingBracket.ReplaceAllString(newStr, "")

		newStr = strings.TrimSpace(newStr)

		if newStr == stripped {
			break
		}
		stripped = newStr
	}

	stripped = strings.Join(strings.Fields(stripped), " ")

	if stripped == "" {
		return title
	}
	return stripped
}

func (j *spotifyMetadataJob) spotifySearch(ctx context.Context, token, query string, limit int) ([]spotifyTrack, error) {
	endpoint, _ := url.Parse("https://api.spotify.com/v1/search")
	params := endpoint.Query()
	params.Set("q", query)
	params.Set("type", "track")
	params.Set("limit", strconv.Itoa(limit))
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := j.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		trimmedBody := strings.TrimSpace(string(body))
		if resp.StatusCode == http.StatusUnauthorized && strings.Contains(strings.ToLower(trimmedBody), "access token expired") {
			return nil, fmt.Errorf("%w: %s", errSpotifyTokenExpired, trimmedBody)
		}
		return nil, fmt.Errorf("spotify search status: %d, body: %s", resp.StatusCode, trimmedBody)
	}
	var payload spotifySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Tracks.Items, nil
}

// spotifyFieldValue strips the quote characters that would terminate a quoted
// track:/artist: field filter early.
func spotifyFieldValue(v string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(v, `"`, " ")), " ")
}

// bestArtistSimilarity scores the local artist against every credited artist
// and against all of them joined, so collaborations and "feat." credits aren't
// penalized for matching a non-primary artist.
func bestArtistSimilarity(localArtistNorm string, track *spotifyTrack) float64 {
	best := 0.0
	names := make([]string, 0, len(track.Artists))
	for _, artist := range track.Artists {
		names = append(names, artist.Name)
		if score := stringSimilarity(localArtistNorm, normalizeSpotifyString(artist.Name)); score > best {
			best = score
		}
	}
	if len(names) > 1 {
		if score := stringSimilarity(localArtistNorm, normalizeSpotifyString(strings.Join(names, " "))); score > best {
			best = score
		}
	}
	return best
}

func (j *spotifyMetadataJob) searchSpotifyTrack(ctx context.Context, token string, mf model.MediaFile, title string) (*spotifyTrack, float64, error) {
	localArtist := strings.TrimSpace(mf.Artist)
	localTitle := strings.TrimSpace(title)
	if localArtist == "" || localTitle == "" {
		return nil, 0, nil
	}

	// Field-filtered search is far more precise than a bag-of-words query; the
	// loose query is only a fallback when the strict one finds nothing.
	strict := fmt.Sprintf(`track:"%s" artist:"%s"`, spotifyFieldValue(localTitle), spotifyFieldValue(localArtist))
	log.Debug(ctx, "Spotify metadata search request", "songId", mf.ID, "title", localTitle, "artist", localArtist, "query", strict)
	items, err := j.spotifySearch(ctx, token, strict, 10)
	if err != nil {
		return nil, 0, err
	}
	if len(items) == 0 {
		items, err = j.spotifySearch(ctx, token, localTitle+" "+localArtist, 10)
		if err != nil {
			return nil, 0, err
		}
	}
	if len(items) == 0 {
		return nil, 0, nil
	}

	localTitleNorm := normalizeSpotifyString(localTitle)
	localArtistNorm := normalizeSpotifyString(localArtist)
	localDuration := int(mf.Duration)

	var best *spotifyTrack
	bestScore := 0.0
	for i := range items {
		candidate := &items[i]
		if len(candidate.Artists) == 0 {
			continue
		}
		titleScore := stringSimilarity(localTitleNorm, normalizeSpotifyString(candidate.Name))
		artistScore := bestArtistSimilarity(localArtistNorm, candidate)
		durationScore := 0.0
		if localDuration > 0 {
			diff := localDuration - (candidate.DurationMS / 1000)
			if diff < 0 {
				diff = -diff
			}
			if diff <= 10 {
				durationScore = 1
			}
		}
		finalScore := (titleScore * conf.Server.Spotify.TitleScoreWeight) +
			(artistScore * conf.Server.Spotify.ArtistScoreWeight) +
			(durationScore * conf.Server.Spotify.DurationScoreWeight)
		if finalScore > bestScore {
			bestScore = finalScore
			best = candidate
		}
	}
	return best, bestScore, nil
}

func (j *spotifyMetadataJob) searchBestTrack(ctx context.Context, token string, mf model.MediaFile) (*spotifyTrack, float64, error) {
	track, confidence, err := j.searchSpotifyTrack(ctx, token, mf, mf.Title)
	if err != nil || track == nil {
		return track, confidence, err
	}

	if confidence >= conf.Server.Spotify.CoverRetryMinScore {
		return track, confidence, nil
	}

	retryTitle := sanitizeSpotifyRetryTitle(mf.Title)
	if retryTitle == "" || strings.EqualFold(strings.TrimSpace(mf.Title), retryTitle) {
		return track, confidence, nil
	}

	retryTrack, retryConfidence, retryErr := j.searchSpotifyTrack(ctx, token, mf, retryTitle)
	if retryErr != nil {
		return nil, 0, retryErr
	}
	if retryTrack == nil {
		return track, confidence, nil
	}
	return retryTrack, retryConfidence, nil
}

// spotifyLookupResult carries the read-only metadata Spotify can provide for a
// track: album name, release year, the primary artist's genres, and the match
// confidence for the search.
type spotifyLookupResult struct {
	Album      string
	Year       int
	Genre      string
	Confidence float64
	Found      bool
}

// lookupMetadata searches Spotify for the best-matching track and returns its
// album, release year, primary-artist genres, and match confidence. It is
// read-only and never writes to the database.
func (j *spotifyMetadataJob) lookupMetadata(ctx context.Context, mf model.MediaFile) (spotifyLookupResult, error) {
	if strings.TrimSpace(mf.Title) == "" || strings.TrimSpace(mf.Artist) == "" {
		return spotifyLookupResult{}, nil
	}
	token, err := j.getToken(ctx)
	if err != nil {
		return spotifyLookupResult{}, err
	}
	track, confidence, err := j.searchBestTrack(ctx, token, mf)
	if err != nil && errors.Is(err, errSpotifyTokenExpired) {
		if token, err = j.refreshToken(ctx); err == nil {
			track, confidence, err = j.searchBestTrack(ctx, token, mf)
		}
	}
	if err != nil {
		return spotifyLookupResult{}, err
	}
	if track == nil {
		return spotifyLookupResult{}, nil
	}

	result := spotifyLookupResult{
		Album:      strings.TrimSpace(track.Album.Name),
		Year:       spotifyReleaseYear(track.Album.ReleaseDate),
		Confidence: confidence,
		Found:      true,
	}
	// Spotify genres are attached to the artist, not the track, so they require
	// a second lookup. This is best-effort; a failure just leaves genre empty.
	if genre := j.trackArtistGenre(ctx, token, track, mf.Artist, confidence); genre != "" {
		result.Genre = genre
	}
	return result, nil
}

// trackArtistGenre returns the genre list for a matched track. Spotify attaches
// genres to artists, not tracks, so the credited artists are fetched in one
// batched request. Genres from the artist whose name matches the library
// artist are preferred; when no name matches, the credited artists are only
// trusted if the track match itself was confident, so a wrong-track match
// can't donate an unrelated artist's genres.
func (j *spotifyMetadataJob) trackArtistGenre(ctx context.Context, token string, track *spotifyTrack, localArtist string, confidence float64) string {
	ids := make([]string, 0, len(track.Artists))
	for _, artist := range track.Artists {
		if id := strings.TrimSpace(artist.ID); id != "" {
			ids = append(ids, id)
		}
		if len(ids) == 5 {
			break
		}
	}
	if len(ids) == 0 {
		return ""
	}
	artists, err := j.fetchArtists(ctx, token, ids)
	if err != nil {
		log.Debug(ctx, "Could not fetch Spotify artist genres", "artistIds", ids, "err", err)
		return ""
	}

	localNorm := normalizeSpotifyString(localArtist)
	var matched, primary, any []string
	for i, artist := range artists {
		if len(artist.Genres) == 0 {
			continue
		}
		if any == nil {
			any = artist.Genres
		}
		if i == 0 {
			primary = artist.Genres
		}
		if matched == nil && localNorm != "" && stringSimilarity(localNorm, normalizeSpotifyString(artist.Name)) >= 0.8 {
			matched = artist.Genres
		}
	}

	genres := matched
	if genres == nil && confidence >= conf.Server.Spotify.MinScore {
		genres = primary
		if genres == nil {
			genres = any
		}
	}
	if len(genres) > 2 {
		genres = genres[:2]
	}
	titled := make([]string, 0, len(genres))
	for _, genre := range genres {
		if genre = strings.TrimSpace(genre); genre != "" {
			titled = append(titled, titleCaseGenre(genre))
		}
	}
	return strings.Join(titled, ", ")
}

type spotifyArtist struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Genres []string `json:"genres"`
}

// fetchArtists resolves artists via the batched artists endpoint, with a
// per-job cache so libraries with many tracks by the same artist don't repeat
// requests and run into rate limits. The result preserves the order of ids.
func (j *spotifyMetadataJob) fetchArtists(ctx context.Context, token string, ids []string) ([]spotifyArtist, error) {
	missing := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := j.artistGenreCache.Load(id); !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		endpoint := "https://api.spotify.com/v1/artists?ids=" + url.QueryEscape(strings.Join(missing, ","))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := j.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			return nil, fmt.Errorf("spotify artists status: %d, body: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		var payload struct {
			Artists []spotifyArtist `json:"artists"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, err
		}
		for _, artist := range payload.Artists {
			if strings.TrimSpace(artist.ID) != "" {
				j.artistGenreCache.Store(artist.ID, artist)
			}
		}
	}

	res := make([]spotifyArtist, 0, len(ids))
	for _, id := range ids {
		if cached, ok := j.artistGenreCache.Load(id); ok {
			res = append(res, cached.(spotifyArtist))
		}
	}
	return res, nil
}

// titleCaseGenre upper-cases the first letter of each word so lowercase Spotify
// genres (e.g. "indie pop") display consistently with the rest of the UI. It
// also capitalizes after the separators used inside genre names — "/", "-",
// "&" — so taxonomy genres like "Hip-Hop/Rap", "R&B/Soul", and "K-Pop" keep
// their canonical casing.
func titleCaseGenre(genre string) string {
	runes := []rune(strings.ToLower(genre))
	capitalizeNext := true
	for i, r := range runes {
		switch r {
		case ' ', '/', '-', '&':
			capitalizeNext = true
		default:
			if capitalizeNext {
				runes[i] = unicode.ToUpper(r)
			}
			capitalizeNext = false
		}
	}
	return strings.Join(strings.Fields(string(runes)), " ")
}

func (j *spotifyMetadataJob) fetchAndSetCoverFromURL(ctx context.Context, ds model.DataStore, songID, spotifyURL string) (spotifyConfidenceEntry, error) {
	songID = strings.TrimSpace(songID)
	trackID, err := spotifyTrackIDFromURL(spotifyURL)
	if err != nil {
		return spotifyConfidenceEntry{}, err
	}

	token, err := j.getToken(ctx)
	if err != nil {
		return spotifyConfidenceEntry{}, err
	}

	track, err := j.fetchTrackByID(ctx, token, trackID)
	if err != nil && errors.Is(err, errSpotifyTokenExpired) {
		token, err = j.refreshToken(ctx)
		if err != nil {
			return spotifyConfidenceEntry{}, err
		}
		track, err = j.fetchTrackByID(ctx, token, trackID)
	}
	if err != nil {
		return spotifyConfidenceEntry{}, err
	}

	coverURL := ""
	if len(track.Album.Images) > 0 {
		coverURL = strings.TrimSpace(track.Album.Images[0].URL)
	}
	if coverURL == "" {
		return spotifyConfidenceEntry{}, fmt.Errorf("spotify track has no album cover image")
	}

	relPath, ok := j.ensureSpotifyCover(ctx, track.ID, coverURL, true)
	if !ok {
		return spotifyConfidenceEntry{}, fmt.Errorf("could not cache spotify cover image")
	}
	if err := ds.MediaFile(ctx).UpdateCoverPath(songID, relPath); err != nil {
		return spotifyConfidenceEntry{}, err
	}

	mf, err := ds.MediaFile(ctx).Get(songID)
	if err != nil {
		return spotifyConfidenceEntry{}, err
	}

	albumName := strings.TrimSpace(track.Album.Name)
	releaseYear := spotifyReleaseYear(track.Album.ReleaseDate)
	if (isMissingAlbum(mf.Album) && albumName != "") || (mf.Year == 0 && releaseYear > 0) {
		var year *int
		if mf.Year == 0 && releaseYear > 0 {
			year = &releaseYear
		}
		if err := ds.MediaFile(ctx).UpdateMissingMetadata(songID, valueOrNil(albumName), year, nil, nil, nil); err != nil {
			return spotifyConfidenceEntry{}, err
		}
	}

	spotifyMatch := strings.TrimSpace(track.Name)
	spotifyArtist := spotifyPrimaryArtist(track)
	spotifyURL = "https://open.spotify.com/track/" + strings.TrimSpace(track.ID)
	if err := ds.MediaFile(ctx).UpdateSpotifyMetadata(songID, nil, valueOrNil(spotifyMatch), valueOrNil(spotifyArtist), valueOrNil(spotifyURL)); err != nil {
		return spotifyConfidenceEntry{}, err
	}

	entry := spotifyConfidenceEntry{
		SongID:        songID,
		Album:         albumName,
		SpotifyMatch:  spotifyMatch,
		SpotifyArtist: spotifyArtist,
		SpotifyURL:    spotifyURL,
		CoverURL:      coverURL,
		Downloaded:    true,
	}
	entry.Title = mf.Title
	entry.Artist = mf.Artist
	j.storeEntry(entry)

	return entry, nil
}

func spotifyPrimaryArtist(track spotifyTrack) string {
	if len(track.Artists) == 0 {
		return ""
	}
	return strings.TrimSpace(track.Artists[0].Name)
}

func spotifyTrackIDFromURL(spotifyURL string) (string, error) {
	trimmed := strings.TrimSpace(spotifyURL)
	if trimmed == "" {
		return "", fmt.Errorf("spotify url is required")
	}
	matches := spotifyTrackURLRegex.FindStringSubmatch(trimmed)
	if len(matches) > 1 {
		return matches[1], nil
	}
	if !strings.Contains(trimmed, "/") && len(trimmed) >= 10 {
		return trimmed, nil
	}
	return "", fmt.Errorf("invalid spotify track url")
}

func (j *spotifyMetadataJob) fetchTrackByID(ctx context.Context, token, trackID string) (spotifyTrack, error) {
	endpoint := "https://api.spotify.com/v1/tracks/" + url.PathEscape(strings.TrimSpace(trackID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return spotifyTrack{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := j.client.Do(req)
	if err != nil {
		return spotifyTrack{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		trimmedBody := strings.TrimSpace(string(body))
		if resp.StatusCode == http.StatusUnauthorized && strings.Contains(strings.ToLower(trimmedBody), "access token expired") {
			return spotifyTrack{}, fmt.Errorf("%w: %s", errSpotifyTokenExpired, trimmedBody)
		}
		return spotifyTrack{}, fmt.Errorf("spotify track status: %d, body: %s", resp.StatusCode, trimmedBody)
	}

	var track spotifyTrack
	if err := json.NewDecoder(resp.Body).Decode(&track); err != nil {
		return spotifyTrack{}, err
	}
	if strings.TrimSpace(track.ID) == "" {
		return spotifyTrack{}, fmt.Errorf("spotify track response missing id")
	}
	return track, nil
}

func stringSimilarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	matcher := difflib.NewMatcher(toRuneStrings(a), toRuneStrings(b))
	return matcher.Ratio()
}

func toRuneStrings(s string) []string {
	chars := make([]string, 0, len(s))
	for _, r := range s {
		chars = append(chars, string(r))
	}
	return chars
}

func (j *spotifyMetadataJob) ensureSpotifyCover(ctx context.Context, trackID, coverURL string, force bool) (string, bool) {
	if trackID == "" || coverURL == "" {
		return "", false
	}
	if _, missed := j.coverMisses.Load(trackID); missed && !force {
		return "", false
	}
	if force {
		j.coverMisses.Delete(trackID)
	}
	if err := os.MkdirAll(filepath.Join(conf.Server.DataFolder.String(), coverCacheDirName), 0o755); err != nil {
		log.Warn(ctx, "Could not create cover cache directory", "err", err)
		return "", false
	}

	fileName := "spotify-" + trackID + ".jpg"
	coverPath := filepath.Join(conf.Server.DataFolder.String(), coverCacheDirName, fileName)
	if !force {
		if _, err := os.Stat(coverPath); err == nil {
			return filepath.ToSlash(filepath.Join(coverCacheDirName, fileName)), true
		}
	} else {
		_ = os.Remove(coverPath)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coverURL, nil)
	if err != nil {
		return "", false
	}
	resp, err := j.client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		j.coverMisses.Store(trackID, true)
		return "", false
	}
	tmpPath := coverPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return "", false
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return "", false
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", false
	}
	if err := os.Rename(tmpPath, coverPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", false
	}
	return filepath.ToSlash(filepath.Join(coverCacheDirName, fileName)), true
}

func (n *Router) addMusicBrainzMetadataRoute(r chi.Router) {
	r.Route("/metadata/musicbrainz", func(r chi.Router) {
		r.Get("/status", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(n.metadataJob.getStatus())
		})
		r.Post("/fetch", func(w http.ResponseWriter, r *http.Request) {
			songIDs, err := decodeSelectedSongIDs(r)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"status":"invalid_payload"}`))
				return
			}

			if n.metadataJob.start(n.ds, songIDs) {
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{"status":"started"}`))
				return
			}
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"status":"already_running"}`))
		})
		r.Get("/spotify/status", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(n.spotifyJob.getStatus())
		})
		r.Get("/spotify/confidence", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": n.spotifyJob.getConfidenceEntries(context.Background(), n.ds)})
		})
		r.Post("/spotify/fetch", func(w http.ResponseWriter, r *http.Request) {
			songIDs, err := decodeSelectedSongIDs(r)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"status":"invalid_payload"}`))
				return
			}

			if n.spotifyJob.start(n.ds, songIDs) {
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{"status":"started"}`))
				return
			}
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"status":"already_running"}`))
		})
		r.Post("/spotify/cover", func(w http.ResponseWriter, r *http.Request) {
			payload := spotifyCoverUpdatePayload{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"status":"invalid_payload"}`))
				return
			}
			spotifyURL := strings.TrimSpace(payload.SpotifyURL)
			if spotifyURL == "" {
				spotifyURL = strings.TrimSpace(payload.SpotifyLink)
			}
			targetSongIDs := make([]string, 0, len(payload.SongIDs)+1)
			for _, id := range payload.SongIDs {
				if trimmed := strings.TrimSpace(id); trimmed != "" {
					targetSongIDs = append(targetSongIDs, trimmed)
				}
			}
			if trimmed := strings.TrimSpace(payload.SongID); trimmed != "" {
				targetSongIDs = append(targetSongIDs, trimmed)
			}

			if len(targetSongIDs) == 0 || spotifyURL == "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"status":"invalid_payload"}`))
				return
			}

			results := make([]spotifyConfidenceEntry, 0, len(targetSongIDs))
			for _, songID := range targetSongIDs {
				entry, err := n.spotifyJob.fetchAndSetCoverFromURL(context.Background(), n.ds, songID, spotifyURL)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]any{"status": "failed", "error": err.Error(), "songId": songID})
					return
				}
				results = append(results, entry)
			}

			if len(results) == 0 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "failed", "error": "no songs updated"})
				return
			}

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "updated", "items": results})
		})
		r.Post("/save", func(w http.ResponseWriter, _ *http.Request) {
			status := n.metadataJob.getStatus()
			if status.Running || n.spotifyJob.getStatus().Running {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"status":"still_running"}`))
				return
			}

			summary, err := n.saveFetchedMetadataToSongs()
			if err != nil {
				log.Warn("Could not persist fetched metadata to song files", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"status":"save_failed"}`))
				return
			}

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":    "saved",
				"saved":     summary.Saved,
				"remaining": summary.Remaining,
				"saving":    summary.Saving,
			})
		})
	})
}

func (n *Router) saveFetchedMetadataToSongs() (metadataSaveSummary, error) {
	ctx := context.Background()
	summary := metadataSaveSummary{}
	cursor, err := n.ds.MediaFile(ctx).GetCursor()
	if err != nil {
		return summary, err
	}

	for mf, e := range cursor {
		if e != nil {
			return summary, e
		}

		if strings.TrimSpace(mf.Path) == "" || strings.TrimSpace(mf.LibraryPath) == "" {
			continue
		}

		coverFile := ""
		if strings.HasPrefix(strings.TrimSpace(mf.CoverPath), coverCacheDirName+"/") {
			coverFile = filepath.Join(conf.Server.DataFolder.String(), filepath.FromSlash(mf.CoverPath))
		}

		hasFetchedIDs := strings.TrimSpace(mf.MbzRecordingID) != "" || strings.TrimSpace(mf.MbzReleaseID) != ""
		if !hasFetchedIDs && coverFile == "" {
			continue
		}

		summary.Remaining++

		if err := taglibwrite.WriteFetchedMetadata(mf.AbsolutePath(), taglibwrite.FetchedMetadata{
			Album:         mf.Album,
			Year:          mf.Year,
			Genre:         mf.Genre,
			RecordingMBID: mf.MbzRecordingID,
			ReleaseMBID:   mf.MbzReleaseID,
			CoverPath:     coverFile,
		}); err != nil {
			log.Warn(ctx, "Could not write fetched metadata to song", "songId", mf.ID, "path", mf.Path, "err", err)
			continue
		}

		summary.Saved++
		summary.Remaining--
	}

	return summary, nil
}
