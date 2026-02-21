package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

type metadataFieldProgress struct {
	Missing  int `json:"missing"`
	Fetching int `json:"fetching"`
	Fetched  int `json:"fetched"`
	Updated  int `json:"updated"`
	Left     int `json:"left"`
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
}

type musicBrainzMetadataJob struct {
	mu     sync.RWMutex
	status musicBrainzMetadataStatus
	client *http.Client
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

		metadata, fetchErr := j.fetchMetadata(c.mf.Title, c.mf.Artist)
		if fetchErr != nil {
			failed++
			log.Warn(ctx, "Could not fetch metadata from MusicBrainz", "songId", c.mf.ID, "title", c.mf.Title, "artist", c.mf.Artist, fetchErr)
			j.finishFetch(c.flags, false, false, false, false, false)
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
				j.setUpdated(setAlbum != nil, setYear != nil, setGenre != nil, c.flags.recordingMBID && metadata.RecordingMBID != "", c.flags.releaseMBID && metadata.ReleaseMBID != "")
			}
		} else {
			skipped++
		}

		j.finishFetch(c.flags, metadata.Album != "", metadata.Year > 0, metadata.Genre != "", metadata.RecordingMBID != "", metadata.ReleaseMBID != "")
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
		}
		if !flags.album && !flags.year && !flags.genre && !flags.recordingMBID && !flags.releaseMBID {
			continue
		}

		res = append(res, mbMetadataCandidate{mf: mf, flags: flags})
		j.incrementMissing(flags)
	}
	return res, nil
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
}

func (j *musicBrainzMetadataJob) setUpdated(album, year, genre, recordingMBID, releaseMBID bool) {
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
}

func (j *musicBrainzMetadataJob) finishFetch(flags missingFlags, fetchedAlbum, fetchedYear, fetchedGenre, fetchedRecordingMBID, fetchedReleaseMBID bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if flags.album {
		j.status.Album.Fetching--
		j.status.Album.Left--
		if fetchedAlbum {
			j.status.Album.Fetched++
		}
	}
	if flags.year {
		j.status.Year.Fetching--
		j.status.Year.Left--
		if fetchedYear {
			j.status.Year.Fetched++
		}
	}
	if flags.genre {
		j.status.Genre.Fetching--
		j.status.Genre.Left--
		if fetchedGenre {
			j.status.Genre.Fetched++
		}
	}
	if flags.recordingMBID {
		j.status.RecordingMBID.Fetching--
		j.status.RecordingMBID.Left--
		if fetchedRecordingMBID {
			j.status.RecordingMBID.Fetched++
		}
	}
	if flags.releaseMBID {
		j.status.ReleaseMBID.Fetching--
		j.status.ReleaseMBID.Left--
		if fetchedReleaseMBID {
			j.status.ReleaseMBID.Fetched++
		}
	}
}

func (j *musicBrainzMetadataJob) setError(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status.LastError = err.Error()
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

	bestRecording := selectBestRecording(payload.Recordings, artist)
	if bestRecording == nil {
		return metadataResult{}, nil
	}

	bestRelease := selectBestRelease(bestRecording.Releases)
	if bestRelease == nil {
		return metadataResult{Genre: collectRecordingGenre(*bestRecording), RecordingMBID: strings.TrimSpace(bestRecording.ID)}, nil
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

func selectBestRelease(releases []mbRelease) *mbRelease {
	type relCandidate struct {
		release               *mbRelease
		official              bool
		album                 bool
		hasDiscouragedSubtype bool
		usCountry             bool
		year                  int
	}

	candidates := make([]relCandidate, 0, len(releases))
	for i := range releases {
		release := &releases[i]
		if !isValidRelease(*release) {
			continue
		}
		candidates = append(candidates, relCandidate{
			release:               release,
			official:              strings.EqualFold(strings.TrimSpace(release.Status), "Official"),
			album:                 isAlbumRelease(*release),
			hasDiscouragedSubtype: hasDiscouragedSecondaryType(*release),
			usCountry:             strings.EqualFold(strings.TrimSpace(release.Country), "US"),
			year:                  yearFromDate(release.Date),
		})
	}
	if len(candidates) == 0 {
		return nil
	}

	slices.SortFunc(candidates, func(a, b relCandidate) int {
		if a.official && !b.official {
			return -1
		}
		if !a.official && b.official {
			return 1
		}
		if a.album && !b.album {
			return -1
		}
		if !a.album && b.album {
			return 1
		}
		if !a.hasDiscouragedSubtype && b.hasDiscouragedSubtype {
			return -1
		}
		if a.hasDiscouragedSubtype && !b.hasDiscouragedSubtype {
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

	return candidates[0].release
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
	})
}
