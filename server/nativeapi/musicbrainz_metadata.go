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
	Running    bool                  `json:"running"`
	StartedAt  *time.Time            `json:"startedAt,omitempty"`
	FinishedAt *time.Time            `json:"finishedAt,omitempty"`
	LastError  string                `json:"lastError,omitempty"`
	Album      metadataFieldProgress `json:"album"`
	Year       metadataFieldProgress `json:"year"`
	Genre      metadataFieldProgress `json:"genre"`
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
	album bool
	year  bool
	genre bool
}

type mbMetadataCandidate struct {
	mf    model.MediaFile
	flags missingFlags
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
	matched := 0
	fetched := 0
	updated := 0
	skipped := 0
	failed := 0

	for _, c := range candidates {
		if c.flags.album {
			missingAlbums++
		}
	}

	const batchSize = 50
	requestCount := 0
	for start := 0; start < len(candidates); start += batchSize {
		end := min(start+batchSize, len(candidates))
		batch := candidates[start:end]
		for _, c := range batch {
			if requestCount > 0 {
				<-ticker.C
			}
			requestCount++
			j.setFetching(c.flags, 1)

			album, year, genre, fetchErr := j.fetchMetadata(c.mf.Title, c.mf.Artist)
			if fetchErr != nil {
				failed++
				log.Warn(ctx, "Could not fetch metadata from MusicBrainz", "songId", c.mf.ID, "title", c.mf.Title, "artist", c.mf.Artist, fetchErr)
				j.finishFetch(c.flags, false, false, false)
				continue
			}

			fetched++
			if album != "" || year > 0 || genre != "" {
				matched++
			}

			var setAlbum *string
			var setYear *int
			var setGenre *string

			if c.flags.album && album != "" {
				setAlbum = &album
			}
			if c.flags.year && year > 0 {
				setYear = &year
			}
			if c.flags.genre && genre != "" {
				setGenre = &genre
			}

			if setAlbum != nil || setYear != nil || setGenre != nil {
				if err := ds.MediaFile(ctx).UpdateMissingMetadata(c.mf.ID, setAlbum, setYear, setGenre); err != nil {
					failed++
					log.Error(ctx, "Could not update fetched MusicBrainz metadata", "songId", c.mf.ID, err)
				} else {
					updated++
					j.setUpdated(setAlbum != nil, setYear != nil, setGenre != nil)
				}
			} else {
				skipped++
			}

			j.finishFetch(c.flags, album != "", year > 0, genre != "")
		}
	}

	log.Info(ctx, "MusicBrainz metadata fetch completed",
		"missingAlbums", missingAlbums,
		"matched", matched,
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
			album: isMissingAlbum(mf.Album),
			year:  mf.Year == 0,
			genre: strings.TrimSpace(mf.Genre) == "",
		}
		if !flags.album && !flags.year && !flags.genre {
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
}

func (j *musicBrainzMetadataJob) setUpdated(album, year, genre bool) {
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
}

func (j *musicBrainzMetadataJob) finishFetch(flags missingFlags, fetchedAlbum, fetchedYear, fetchedGenre bool) {
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
	Title            string      `json:"title"`
	Score            mbScore     `json:"score"`
	FirstReleaseDate string      `json:"first-release-date"`
	ArtistCredit     []mbCredit  `json:"artist-credit"`
	Releases         []mbRelease `json:"releases"`
	Tags             []mbName    `json:"tags"`
	Genres           []mbName    `json:"genres"`
}

type mbCredit struct {
	Name       string `json:"name"`
	JoinPhrase string `json:"joinphrase"`
}

type mbRelease struct {
	Title        string   `json:"title"`
	Date         string   `json:"date"`
	Status       string   `json:"status"`
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
	PrimaryType      string   `json:"primary-type"`
	SecondaryType    []string `json:"secondary-types"`
	FirstReleaseDate string   `json:"first-release-date"`
}

func (j *musicBrainzMetadataJob) fetchMetadata(title, artist string) (string, int, string, error) {
	normalizedTitle := normalizeRecordingSearchTitle(title)
	if normalizedTitle == "" {
		return "", 0, "", nil
	}

	query := fmt.Sprintf(`recording:"%s" AND artist:"%s"`, normalizedTitle, strings.TrimSpace(artist))
	u := "https://musicbrainz.org/ws/2/recording/?query=" + url.QueryEscape(query) + "&fmt=json&limit=5"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", 0, "", err
	}
	req.Header.Set("User-Agent", "Navidrome/metadata-fetcher (https://www.navidrome.org)")

	resp, err := j.client.Do(req)
	if err != nil {
		return "", 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, "", fmt.Errorf("musicbrainz status %d", resp.StatusCode)
	}

	var payload mbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", 0, "", err
	}
	if len(payload.Recordings) == 0 {
		return "", 0, "", nil
	}

	bestRecording := selectBestRecording(payload.Recordings, artist, title)
	if bestRecording == nil {
		return "", 0, "", nil
	}

	bestRelease := selectBestRelease(bestRecording.Releases, title)
	if bestRelease == nil {
		return "", yearFromDate(bestRecording.FirstReleaseDate), collectRecordingGenre(*bestRecording), nil
	}

	album := strings.TrimSpace(bestRelease.Title)
	year := yearFromDate(bestRelease.ReleaseGroup.FirstReleaseDate)
	if year == 0 {
		year = yearFromDate(bestRecording.FirstReleaseDate)
	}
	genre := collectGenre(*bestRecording, *bestRelease)
	return album, year, genre, nil
}

func selectBestRecording(recordings []mbRecording, artist, title string) *mbRecording {
	normalizedArtist := normalizeMBString(artist)
	normalizedTitle := normalizeRecordingSearchTitle(title)

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
		if !recordingTitleMatches(rec.Title, normalizedTitle) {
			continue
		}

		hasAlbum := false
		earliest := 9999
		for _, rel := range rec.Releases {
			if !isValidRelease(rel, title) {
				continue
			}
			if isAlbumRelease(rel) {
				hasAlbum = true
			}
			if y := yearFromDate(rel.ReleaseGroup.FirstReleaseDate); y > 0 && y < earliest {
				earliest = y
			}
		}
		candidates = append(candidates, recCandidate{rec: rec, score: score, releases: len(rec.Releases), hasAlbum: hasAlbum, earliest: earliest})
	}
	if len(candidates) == 0 {
		return nil
	}

	slices.SortFunc(candidates, func(a, b recCandidate) int {
		if a.score != b.score {
			return b.score - a.score
		}
		if a.releases > 0 && b.releases == 0 {
			return -1
		}
		if a.releases == 0 && b.releases > 0 {
			return 1
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

func artistCreditMatches(credits []mbCredit, normalizedArtist string) bool {
	if normalizedArtist == "" {
		return false
	}
	return normalizeMBString(joinArtistCredits(credits)) == normalizedArtist
}

func joinArtistCredits(credits []mbCredit) string {
	var b strings.Builder
	for _, credit := range credits {
		b.WriteString(credit.Name)
		b.WriteString(credit.JoinPhrase)
	}
	return b.String()
}

func selectBestRelease(releases []mbRelease, trackTitle string) *mbRelease {
	type relCandidate struct {
		release  *mbRelease
		official bool
		album    bool
		year     int
	}

	candidates := make([]relCandidate, 0, len(releases))
	for i := range releases {
		release := &releases[i]
		if !isValidRelease(*release, trackTitle) {
			continue
		}
		candidates = append(candidates, relCandidate{
			release:  release,
			official: strings.EqualFold(strings.TrimSpace(release.Status), "Official"),
			album:    isAlbumRelease(*release),
			year:     yearFromDate(release.ReleaseGroup.FirstReleaseDate),
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
		if a.year == 0 && b.year > 0 {
			return 1
		}
		if b.year == 0 && a.year > 0 {
			return -1
		}
		if a.year != b.year {
			return a.year - b.year
		}
		return 0
	})

	return candidates[0].release
}

func isValidRelease(release mbRelease, trackTitle string) bool {
	if strings.TrimSpace(release.Title) == "" {
		return false
	}
	if isCompilationRelease(release) {
		return false
	}
	if isLiveRelease(release) && !strings.Contains(normalizeMBString(trackTitle), "live") {
		return false
	}
	return true
}

func isCompilationRelease(release mbRelease) bool {
	if normalizeMBString(release.ReleaseGroup.PrimaryType) == "compilation" {
		return true
	}
	for _, t := range release.ReleaseGroup.SecondaryType {
		if normalizeMBString(t) == "compilation" {
			return true
		}
	}
	return false
}

func isLiveRelease(release mbRelease) bool {
	if normalizeMBString(release.ReleaseGroup.PrimaryType) == "live" {
		return true
	}
	for _, t := range release.ReleaseGroup.SecondaryType {
		if normalizeMBString(t) == "live" {
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
var normalizeRemixLiveRegex = regexp.MustCompile(`(?i)[(\[]\s*(remix|live)\s*[)\]]`)
var normalizeFeatRegex = regexp.MustCompile(`(?i)\bfeat\.?\b[^\[(]*`)

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

func normalizeRecordingSearchTitle(v string) string {
	v = strings.TrimSpace(v)
	v = normalizeRemixLiveRegex.ReplaceAllString(v, " ")
	v = normalizeFeatRegex.ReplaceAllString(v, " ")
	v = strings.NewReplacer("(", " ", ")", " ", "[", " ", "]", " ", "{", " ", "}", " ").Replace(v)
	return normalizeMBString(v)
}

func recordingTitleMatches(recordingTitle, normalizedTrackTitle string) bool {
	return normalizeRecordingSearchTitle(recordingTitle) == normalizedTrackTitle
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
