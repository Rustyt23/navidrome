package nativeapi

import (
	"context"
	"encoding/base64"
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
	"github.com/navidrome/navidrome/adapters/taglib"
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
}

func newMusicBrainzMetadataJob() *musicBrainzMetadataJob {
	return &musicBrainzMetadataJob{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (j *musicBrainzMetadataJob) getStatus() musicBrainzMetadataStatus {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.status
}

func (j *musicBrainzMetadataJob) start(ds model.DataStore) bool {
	j.mu.Lock()
	if j.status.Running {
		j.mu.Unlock()
		return false
	}
	now := time.Now()
	j.status = musicBrainzMetadataStatus{Running: true, StartedAt: &now}
	j.mu.Unlock()

	go j.run(ds)
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
	SongID      string  `json:"songId"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	MatchedName string  `json:"matchedName"`
	Confidence  float64 `json:"confidence"`
	Album       string  `json:"album"`
	CoverURL    string  `json:"coverUrl,omitempty"`
	Downloaded  bool    `json:"downloaded"`
}

type spotifyMetadataJob struct {
	mu          sync.RWMutex
	status      spotifyMetadataStatus
	client      *http.Client
	entries     map[string]spotifyConfidenceEntry
	coverMisses sync.Map
}

func newSpotifyMetadataJob() *spotifyMetadataJob {
	return &spotifyMetadataJob{
		client:  &http.Client{Timeout: 15 * time.Second},
		entries: map[string]spotifyConfidenceEntry{},
	}
}

const (
	spotifyMinScore            = 0.69
	spotifyTokenRefreshSeconds = 3500
)

type spotifyHTTPError struct {
	status int
	op     string
}

func (e spotifyHTTPError) Error() string {
	return fmt.Sprintf("spotify %s status: %d", e.op, e.status)
}

func (j *musicBrainzMetadataJob) run(ds model.DataStore) {
	ctx := context.Background()
	defer func() {
		j.mu.Lock()
		j.status.Running = false
		now := time.Now()
		j.status.FinishedAt = &now
		j.mu.Unlock()
	}()

	candidates, err := j.collectCandidates(ctx, ds)
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

		metadata, fetchErr := j.fetchMetadata(c.mf.Title, c.mf.Artist)
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

func (j *musicBrainzMetadataJob) collectCandidates(ctx context.Context, ds model.DataStore) ([]mbMetadataCandidate, error) {
	cursor, err := ds.MediaFile(ctx).GetCursor()
	if err != nil {
		return nil, err
	}

	res := make([]mbMetadataCandidate, 0)
	for mf, e := range cursor {
		if e != nil {
			return nil, e
		}
		if strings.TrimSpace(mf.Title) == "" || strings.TrimSpace(mf.Artist) == "" {
			continue
		}

		flags := missingFlags{
			album:         isMissingAlbum(mf.Album),
			year:          mf.Year == 0,
			genre:         strings.TrimSpace(mf.Genre) == "",
			recordingMBID: strings.TrimSpace(mf.MbzRecordingID) == "",
			releaseMBID:   strings.TrimSpace(mf.MbzReleaseID) == "",
			coverArt:      !mf.HasCoverArt && strings.TrimSpace(mf.CoverPath) == "",
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
	if _, err := os.Stat(filepath.Join(conf.Server.DataFolder, coverCacheDirName)); err != nil {
		if err := os.MkdirAll(filepath.Join(conf.Server.DataFolder, coverCacheDirName), 0o755); err != nil {
			log.Warn(ctx, "Could not create cover cache directory", "err", err)
			return "", false
		}
	}

	coverPath := filepath.Join(conf.Server.DataFolder, coverCacheDirName, releaseMBID+".jpg")
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
	Tags     []mbName    `json:"tags"`
	Genres   []mbName    `json:"genres"`
}

type mbRelease struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Date         string   `json:"date"`
	Status       string   `json:"status"`
	Country      string   `json:"country"`
	ReleaseGroup mbGroup  `json:"release-group"`
	Tags         []mbName `json:"tags"`
	Genres       []mbName `json:"genres"`
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

type mbName struct {
	Name string `json:"name"`
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

func (j *musicBrainzMetadataJob) fetchMetadata(title, artist string) (metadataResult, error) {
	query := fmt.Sprintf("artist:%s AND recording:%s", artist, title)
	u := "https://musicbrainz.org/ws/2/recording/?query=" + url.QueryEscape(query) + "&fmt=json&inc=releases+release-groups"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return metadataResult{}, err
	}
	req.Header.Set("User-Agent", "Navidrome/metadata-fetcher (https://www.navidrome.org)")

	resp, err := j.client.Do(req)
	if err != nil {
		return metadataResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return metadataResult{}, fmt.Errorf("musicbrainz status %d", resp.StatusCode)
	}

	var payload mbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return metadataResult{}, err
	}
	if len(payload.Recordings) == 0 {
		return metadataResult{}, nil
	}

	bestCandidates := collectReleaseCandidates(payload.Recordings, artist)
	if len(bestCandidates) == 0 {
		return metadataResult{}, nil
	}

	coverCache := make(map[string]bool, len(bestCandidates))
	best := selectBestReleaseCandidate(bestCandidates, func(releaseID string) bool {
		return j.releaseHasCover(releaseID, coverCache)
	})
	if best == nil {
		return metadataResult{}, nil
	}

	album := strings.TrimSpace(best.release.Title)
	year := yearFromDate(best.release.Date)
	if year == 0 {
		year = yearFromDate(best.recording.FirstReleaseDate)
	}
	genre := collectGenre(*best.recording, *best.release)
	return metadataResult{
		Album:         album,
		Year:          year,
		Genre:         genre,
		RecordingMBID: strings.TrimSpace(best.recording.ID),
		ReleaseMBID:   strings.TrimSpace(best.release.ID),
	}, nil
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
	for _, credit := range credits {
		if normalizeMBString(credit.Name) == normalizedArtist {
			return true
		}
	}
	return false
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

func collectRecordingGenre(rec mbRecording) string {
	return collectGenres(rec.Genres, rec.Tags)
}

func collectGenre(rec mbRecording, release mbRelease) string {
	genre := collectGenres(release.Genres, release.Tags)
	if genre != "" {
		return genre
	}
	return collectRecordingGenre(rec)
}

func collectGenres(genres, tags []mbName) string {
	unique := map[string]bool{}
	ordered := make([]string, 0, 4)
	appendName := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		key := strings.ToLower(v)
		if unique[key] {
			return
		}
		unique[key] = true
		ordered = append(ordered, v)
	}

	for _, g := range genres {
		appendName(g.Name)
	}
	for _, t := range tags {
		appendName(t.Name)
	}
	if len(ordered) == 0 {
		return ""
	}
	if len(ordered) > 5 {
		ordered = ordered[:5]
	}
	return strings.Join(ordered, ", ")
}

var punctuationRegex = regexp.MustCompile(`[\p{P}\p{S}]`)
var spotifyParenRegex = regexp.MustCompile(`\(.*?\)`)
var spotifyBracketRegex = regexp.MustCompile(`\[.*?\]`)
var spotifyFeatRegex = regexp.MustCompile(`feat\.?|ft\.?`)
var spotifyNonAlphaNumSpaceRegex = regexp.MustCompile(`[^a-z0-9 ]`)

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

func (j *spotifyMetadataJob) getConfidenceEntries() []spotifyConfidenceEntry {
	j.mu.RLock()
	defer j.mu.RUnlock()
	res := make([]spotifyConfidenceEntry, 0, len(j.entries))
	for _, v := range j.entries {
		res = append(res, v)
	}
	slices.SortFunc(res, func(a, b spotifyConfidenceEntry) int {
		return strings.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	})
	return res
}

func (j *spotifyMetadataJob) start(ds model.DataStore) bool {
	j.mu.Lock()
	if j.status.Running {
		j.mu.Unlock()
		return false
	}
	now := time.Now()
	j.status = spotifyMetadataStatus{Running: true, StartedAt: &now}
	j.entries = map[string]spotifyConfidenceEntry{}
	j.mu.Unlock()

	go j.run(ds)
	return true
}

func (j *spotifyMetadataJob) run(ds model.DataStore) {
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
	tokenFetchedAt := time.Now()

	cursor, err := ds.MediaFile(ctx).GetCursor()
	if err != nil {
		j.setError(err)
		return
	}

	candidates := make([]model.MediaFile, 0)
	for mf, e := range cursor {
		if e != nil {
			j.setError(e)
			return
		}
		if strings.TrimSpace(mf.Title) == "" || strings.TrimSpace(mf.Artist) == "" {
			continue
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

		if time.Since(tokenFetchedAt) >= time.Duration(spotifyTokenRefreshSeconds)*time.Second {
			refreshedToken, tokenErr := j.getToken(ctx)
			if tokenErr != nil {
				j.setError(tokenErr)
				j.finishFetch(true, isMissingAlbum(mf.Album), false, false)
				continue
			}
			token = refreshedToken
			tokenFetchedAt = time.Now()
		}

		track, confidence, searchErr := j.searchBestTrack(ctx, token, mf)
		if searchErr != nil {
			var httpErr spotifyHTTPError
			if errors.As(searchErr, &httpErr) && httpErr.status == http.StatusUnauthorized {
				refreshedToken, tokenErr := j.getToken(ctx)
				if tokenErr == nil {
					token = refreshedToken
					tokenFetchedAt = time.Now()
					track, confidence, searchErr = j.searchBestTrack(ctx, token, mf)
				} else {
					j.setError(tokenErr)
				}
			}
		}
		if searchErr != nil || track == nil {
			j.finishFetch(true, isMissingAlbum(mf.Album), false, false)
			continue
		}

		albumName := strings.TrimSpace(track.Album.Name)
		coverURL := ""
		if len(track.Album.Images) > 0 {
			coverURL = strings.TrimSpace(track.Album.Images[0].URL)
		}

		setAlbum := isMissingAlbum(mf.Album) && albumName != ""
		if setAlbum {
			if err := ds.MediaFile(ctx).UpdateMissingMetadata(mf.ID, &albumName, nil, nil, nil, nil); err == nil {
				j.setUpdated(true, false)
			} else {
				setAlbum = false
			}
		}

		downloaded := false
		if confidence > spotifyMinScore && coverURL != "" {
			if relPath, ok := j.ensureSpotifyCover(ctx, track.ID, coverURL); ok {
				if err := ds.MediaFile(ctx).UpdateCoverPath(mf.ID, relPath); err == nil {
					downloaded = true
					j.setUpdated(false, true)
				}
			}
		}

		j.storeEntry(spotifyConfidenceEntry{
			SongID:      mf.ID,
			Title:       mf.Title,
			Artist:      mf.Artist,
			MatchedName: strings.TrimSpace(track.Name),
			Confidence:  confidence,
			Album:       albumName,
			CoverURL:    coverURL,
			Downloaded:  downloaded,
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
	clientID := strings.TrimSpace(conf.Server.Spotify.ID)
	clientSecret := strings.TrimSpace(conf.Server.Spotify.Secret)
	if clientID != "" && clientSecret != "" {
		form := url.Values{}
		form.Set("grant_type", "client_credentials")
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://accounts.spotify.com/api/token", strings.NewReader(form.Encode()))
		if err != nil {
			return "", err
		}
		basic := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
		req.Header.Set("Authorization", "Basic "+basic)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := j.client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", spotifyHTTPError{status: resp.StatusCode, op: "token"}
		}
		var payload struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return "", err
		}
		if payload.AccessToken == "" {
			return "", fmt.Errorf("spotify access token is empty")
		}
		return payload.AccessToken, nil
	}

	manualToken := strings.TrimSpace(conf.Server.Spotify.APIToken)
	manualToken = strings.TrimPrefix(manualToken, "Bearer ")
	manualToken = strings.TrimPrefix(manualToken, "bearer ")
	manualToken = strings.TrimSpace(manualToken)
	if manualToken != "" {
		return manualToken, nil
	}

	return "", fmt.Errorf("spotify credentials are not configured (set Spotify.ID + Spotify.Secret or Spotify.APIToken)")
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
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
	} `json:"album"`
}

func (j *spotifyMetadataJob) searchBestTrack(ctx context.Context, token string, mf model.MediaFile) (*spotifyTrack, float64, error) {
	q := strings.TrimSpace(mf.Title + " " + mf.Artist)
	if q == "" {
		return nil, 0, nil
	}
	endpoint, _ := url.Parse("https://api.spotify.com/v1/search")
	params := endpoint.Query()
	params.Set("q", q)
	params.Set("type", "track")
	params.Set("limit", "5")
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := j.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, spotifyHTTPError{status: resp.StatusCode, op: "search"}
	}
	var payload spotifySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, 0, err
	}
	if len(payload.Tracks.Items) == 0 {
		return nil, 0, nil
	}

	localTitle := normalizeSpotifyString(mf.Title)
	localArtist := normalizeSpotifyString(mf.Artist)
	localDuration := int(mf.Duration)

	var best *spotifyTrack
	bestScore := 0.0
	for i := range payload.Tracks.Items {
		candidate := &payload.Tracks.Items[i]
		if len(candidate.Artists) == 0 {
			continue
		}
		titleScore := stringSimilarity(localTitle, normalizeSpotifyString(candidate.Name))
		artistScore := stringSimilarity(localArtist, normalizeSpotifyString(candidate.Artists[0].Name))
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
		finalScore := (titleScore * 0.5) + (artistScore * 0.45) + (durationScore * 0.05)
		if finalScore > bestScore {
			bestScore = finalScore
			best = candidate
		}
	}
	return best, bestScore, nil
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

func (j *spotifyMetadataJob) ensureSpotifyCover(ctx context.Context, trackID, coverURL string) (string, bool) {
	if trackID == "" || coverURL == "" {
		return "", false
	}
	if _, missed := j.coverMisses.Load(trackID); missed {
		return "", false
	}
	if err := os.MkdirAll(filepath.Join(conf.Server.DataFolder, coverCacheDirName), 0o755); err != nil {
		log.Warn(ctx, "Could not create cover cache directory", "err", err)
		return "", false
	}

	fileName := "spotify-" + trackID + ".jpg"
	coverPath := filepath.Join(conf.Server.DataFolder, coverCacheDirName, fileName)
	if _, err := os.Stat(coverPath); err == nil {
		return filepath.ToSlash(filepath.Join(coverCacheDirName, fileName)), true
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
		r.Post("/fetch", func(w http.ResponseWriter, _ *http.Request) {
			if n.metadataJob.start(n.ds) {
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
			_ = json.NewEncoder(w).Encode(map[string]any{"items": n.spotifyJob.getConfidenceEntries()})
		})
		r.Post("/spotify/fetch", func(w http.ResponseWriter, _ *http.Request) {
			if n.spotifyJob.start(n.ds) {
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{"status":"started"}`))
				return
			}
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"status":"already_running"}`))
		})
		r.Post("/save", func(w http.ResponseWriter, _ *http.Request) {
			status := n.metadataJob.getStatus()
			if status.Running || n.spotifyJob.getStatus().Running {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"status":"still_running"}`))
				return
			}

			if err := n.saveFetchedMetadataToSongs(); err != nil {
				log.Warn("Could not persist fetched metadata to song files", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"status":"save_failed"}`))
				return
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"saved"}`))
		})
	})
}

func (n *Router) saveFetchedMetadataToSongs() error {
	ctx := context.Background()
	cursor, err := n.ds.MediaFile(ctx).GetCursor()
	if err != nil {
		return err
	}

	for mf, e := range cursor {
		if e != nil {
			return e
		}

		if strings.TrimSpace(mf.Path) == "" || strings.TrimSpace(mf.LibraryPath) == "" {
			continue
		}

		coverFile := ""
		if strings.HasPrefix(strings.TrimSpace(mf.CoverPath), coverCacheDirName+"/") {
			coverFile = filepath.Join(conf.Server.DataFolder, filepath.FromSlash(mf.CoverPath))
		}

		hasFetchedIDs := strings.TrimSpace(mf.MbzRecordingID) != "" || strings.TrimSpace(mf.MbzReleaseID) != ""
		if !hasFetchedIDs && coverFile == "" {
			continue
		}

		if err := taglib.WriteFetchedMetadata(mf.AbsolutePath(), taglib.FetchedMetadata{
			Album:         mf.Album,
			Year:          mf.Year,
			Genre:         mf.Genre,
			RecordingMBID: mf.MbzRecordingID,
			ReleaseMBID:   mf.MbzReleaseID,
			CoverPath:     coverFile,
		}); err != nil {
			log.Warn(ctx, "Could not write fetched metadata to song", "songId", mf.ID, "path", mf.Path, "err", err)
		}
	}

	return nil
}
