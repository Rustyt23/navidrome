package nativeapi

import (
	"context"
	"encoding/json"
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

		metadata, fetchErr := j.fetchMetadata(c.mf.Title, c.mf.Artist, c.mf.Duration)
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
	Title            string  `json:"title"`
	Score            mbScore `json:"score"`
	Length           int     `json:"length"`
	Video            bool    `json:"video"`
	Disambiguation   string  `json:"disambiguation"`
	FirstReleaseDate string  `json:"first-release-date"`
	ArtistCredit     []struct {
		Name string `json:"name"`
	} `json:"artist-credit"`
	Releases []mbRelease `json:"releases"`
	Tags     []mbName    `json:"tags"`
	Genres   []mbName    `json:"genres"`
}

type mbRelease struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Date         string     `json:"date"`
	Status       string     `json:"status"`
	Country      string     `json:"country"`
	ReleaseGroup mbGroup    `json:"release-group"`
	Media        []mbMedium `json:"media"`
	Tags         []mbName   `json:"tags"`
	Genres       []mbName   `json:"genres"`
}

type mbMedium struct {
	Format string `json:"format"`
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

func (j *musicBrainzMetadataJob) fetchMetadata(title, artist string, localDuration float32) (metadataResult, error) {
	queries := []string{
		fmt.Sprintf("recording:\"%s\" AND artist:\"%s\"", title, artist),
		fmt.Sprintf("recording:%s AND artist:%s", title, artist),
		fmt.Sprintf("recording:\"%s\"", title),
	}

	var bestRecording *mbRecording
	for _, query := range queries {
		recordings, err := j.searchRecordings(query)
		if err != nil {
			continue
		}
		bestRecording = selectBestRecording(recordings, artist, localDuration)
		if bestRecording != nil {
			break
		}
	}
	if bestRecording == nil {
		return metadataResult{}, nil
	}

	details, err := j.fetchRecordingDetails(bestRecording.ID)
	if err != nil {
		return metadataResult{RecordingMBID: strings.TrimSpace(bestRecording.ID)}, nil
	}

	coverCache := make(map[string]bool)
	bestRelease := selectBestReleaseFromRecording(details.Releases, func(releaseID string) bool {
		return j.releaseHasCover(releaseID, coverCache)
	})
	if bestRelease == nil {
		return metadataResult{RecordingMBID: strings.TrimSpace(bestRecording.ID)}, nil
	}

	album := strings.TrimSpace(bestRelease.Title)
	year := yearFromDate(bestRelease.Date)
	if year == 0 {
		year = yearFromDate(bestRecording.FirstReleaseDate)
	}
	genre := collectGenre(*bestRecording, *bestRelease)
	return metadataResult{
		Album:         album,
		Year:          year,
		Genre:         genre,
		RecordingMBID: strings.TrimSpace(bestRecording.ID),
		ReleaseMBID:   strings.TrimSpace(bestRelease.ID),
	}, nil
}

func (j *musicBrainzMetadataJob) searchRecordings(query string) ([]mbRecording, error) {
	u := "https://musicbrainz.org/ws/2/recording/?query=" + url.QueryEscape(query) + "&fmt=json&limit=100&inc=releases+release-groups"
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

func (j *musicBrainzMetadataJob) releaseHasCover(releaseID string, cache map[string]bool) bool {
	releaseID = strings.TrimSpace(releaseID)
	if releaseID == "" {
		return false
	}
	if v, ok := cache[releaseID]; ok {
		return v
	}

	req, err := http.NewRequest(http.MethodHead, "https://coverartarchive.org/release/"+url.PathEscape(releaseID), nil)
	if err != nil {
		cache[releaseID] = false
		return false
	}
	req.Header.Set("User-Agent", "Navidrome/metadata-fetcher (https://www.navidrome.org)")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := j.client.Do(req.WithContext(ctx))
	if err != nil {
		cache[releaseID] = false
		return false
	}
	defer resp.Body.Close()

	hasCover := resp.StatusCode == http.StatusOK
	cache[releaseID] = hasCover
	return hasCover
}

func (j *musicBrainzMetadataJob) fetchRecordingDetails(recordingID string) (mbRecording, error) {
	recordingID = strings.TrimSpace(recordingID)
	if recordingID == "" {
		return mbRecording{}, nil
	}

	u := "https://musicbrainz.org/ws/2/recording/" + url.PathEscape(recordingID) + "?inc=releases+release-groups&fmt=json"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return mbRecording{}, err
	}
	req.Header.Set("User-Agent", "Navidrome/metadata-fetcher (https://www.navidrome.org)")

	resp, err := j.client.Do(req)
	if err != nil {
		return mbRecording{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return mbRecording{}, fmt.Errorf("musicbrainz status %d", resp.StatusCode)
	}

	var details mbRecording
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return mbRecording{}, err
	}
	return details, nil
}

func selectBestRecording(recordings []mbRecording, artist string, localDuration float32) *mbRecording {
	normalizedArtist := normalizeMBString(artist)

	type recCandidate struct {
		rec             *mbRecording
		score           int
		releases        int
		hasAlbum        bool
		hasOfficial     bool
		earliest        int
		durationDeltaMS int
	}

	candidates := make([]recCandidate, 0, len(recordings))
	for i := range recordings {
		rec := &recordings[i]
		score := rec.Score.Int()
		if score < 60 {
			continue
		}
		if !artistCreditMatches(rec.ArtistCredit, normalizedArtist) && !artistCreditLooselyMatches(rec.ArtistCredit, normalizedArtist) {
			continue
		}
		if isExcludedRecording(*rec) {
			continue
		}

		hasAlbum := false
		hasOfficial := false
		earliest := 9999
		for _, rel := range rec.Releases {
			if !isValidRelease(rel) {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(rel.Status), "Official") {
				hasOfficial = true
			}
			if isAlbumRelease(rel) {
				hasAlbum = true
			}
			if y := yearFromDate(rel.Date); y > 0 && y < earliest {
				earliest = y
			}
		}

		delta := int(^uint(0) >> 1)
		if localDuration > 0 && rec.Length > 0 {
			delta = absInt(rec.Length - int(localDuration*1000))
		}

		candidates = append(candidates, recCandidate{rec: rec, score: score, releases: len(rec.Releases), hasAlbum: hasAlbum, hasOfficial: hasOfficial, earliest: earliest, durationDeltaMS: delta})
	}
	if len(candidates) == 0 {
		return nil
	}

	slices.SortFunc(candidates, func(a, b recCandidate) int {
		if a.hasOfficial && !b.hasOfficial {
			return -1
		}
		if !a.hasOfficial && b.hasOfficial {
			return 1
		}
		if a.releases > 0 && b.releases == 0 {
			return -1
		}
		if a.releases == 0 && b.releases > 0 {
			return 1
		}
		if a.score != b.score {
			return b.score - a.score
		}
		if localDuration > 0 && a.durationDeltaMS != b.durationDeltaMS {
			return a.durationDeltaMS - b.durationDeltaMS
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

func artistCreditLooselyMatches(credits []struct {
	Name string `json:"name"`
}, normalizedArtist string) bool {
	if normalizedArtist == "" {
		return false
	}
	artistTokens := strings.Fields(normalizedArtist)
	if len(artistTokens) == 0 {
		return false
	}
	for _, credit := range credits {
		n := normalizeMBString(credit.Name)
		if n == "" {
			continue
		}
		if strings.Contains(n, normalizedArtist) || strings.Contains(normalizedArtist, n) {
			return true
		}
		for _, tok := range artistTokens {
			if tok != "" && strings.Contains(n, tok) {
				return true
			}
		}
	}
	return false
}

func selectBestReleaseFromRecording(releases []mbRelease, hasCover func(releaseID string) bool) *mbRelease {
	ordered := rankReleases(releases, false)
	for _, release := range ordered {
		if hasCover == nil || hasCover(release.ID) {
			return release
		}
	}

	fallbackOrdered := rankReleases(releases, true)
	for _, release := range fallbackOrdered {
		if hasCover == nil || hasCover(release.ID) {
			return release
		}
	}

	return nil
}

func rankReleases(releases []mbRelease, includeExcluded bool) []*mbRelease {
	filtered := make([]*mbRelease, 0, len(releases))
	for i := range releases {
		release := &releases[i]
		if !isValidRelease(*release) {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(release.Status), "Official") {
			continue
		}
		if !includeExcluded {
			if hasExcludedSecondaryType(*release) {
				continue
			}
			if hasExcludedMediaFormat(*release) {
				continue
			}
		}
		filtered = append(filtered, release)
	}
	if len(filtered) == 0 {
		return nil
	}

	slices.SortFunc(filtered, func(a, b *mbRelease) int {
		aRank := releasePrimaryTypeRank(a.ReleaseGroup.PrimaryType)
		bRank := releasePrimaryTypeRank(b.ReleaseGroup.PrimaryType)
		if aRank != bRank {
			return aRank - bRank
		}

		aYear := yearFromDate(a.Date)
		bYear := yearFromDate(b.Date)
		if aYear == 0 && bYear > 0 {
			return 1
		}
		if bYear == 0 && aYear > 0 {
			return -1
		}
		if aYear != bYear {
			return aYear - bYear
		}
		return 0
	})

	return filtered
}

func releasePrimaryTypeRank(primaryType string) int {
	switch normalizeMBString(primaryType) {
	case "single":
		return 0
	case "album":
		return 1
	case "ep":
		return 2
	default:
		return 3
	}
}

func hasExcludedSecondaryType(release mbRelease) bool {
	for _, t := range release.ReleaseGroup.SecondaryType {
		n := normalizeMBString(t)
		if n == "compilation" || n == "remix" || n == "soundtrack" {
			return true
		}
	}
	return false
}

func hasExcludedMediaFormat(release mbRelease) bool {
	for _, media := range release.Media {
		format := normalizeMBString(media.Format)
		if strings.Contains(format, "dvd") || strings.Contains(format, "video") {
			return true
		}
	}
	return false
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

func isExcludedRecording(rec mbRecording) bool {
	if rec.Video {
		return true
	}
	for _, marker := range []string{"live", "remix", "acoustic", "demo", "instrumental", "video"} {
		if hasMarker(rec.Title, marker) || hasMarker(rec.Disambiguation, marker) {
			return true
		}
	}
	return false
}

func hasMarker(value, marker string) bool {
	for _, token := range strings.Fields(normalizeMBString(value)) {
		if token == marker {
			return true
		}
	}
	return false
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
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
		r.Post("/save", func(w http.ResponseWriter, _ *http.Request) {
			status := n.metadataJob.getStatus()
			if status.Running {
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
