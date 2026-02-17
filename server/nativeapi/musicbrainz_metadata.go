package nativeapi

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
	fetched := 0
	strictMatched := 0
	updated := 0
	rejectedLowConfidence := 0
	ambiguous := 0
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

		album, year, genre, outcome, fetchErr := j.fetchMetadata(c.mf.Title, c.mf.Artist)
		if fetchErr != nil {
			failed++
			log.Warn(ctx, "Could not fetch metadata from MusicBrainz", "songId", c.mf.ID, "title", c.mf.Title, "artist", c.mf.Artist, fetchErr)
			j.finishFetch(c.flags, false, false, false)
			continue
		}

		fetched++
		switch outcome {
		case "strict":
			strictMatched++
		case "rejected":
			rejectedLowConfidence++
		case "ambiguous":
			ambiguous++
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
		}

		j.finishFetch(c.flags, album != "", year > 0, genre != "")
	}

	log.Info(ctx, "MusicBrainz metadata fetch completed",
		"totalScanned", len(candidates),
		"missingAlbums", missingAlbums,
		"fetched", fetched,
		"strictMatched", strictMatched,
		"rejectedLowConfidence", rejectedLowConfidence,
		"ambiguous", ambiguous,
		"updated", updated,
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
	Title        string           `json:"title"`
	Score        mbScore          `json:"score"`
	ArtistCredit []mbArtistCredit `json:"artist-credit"`
	Releases     []mbRelease      `json:"releases"`
	Tags         []mbTag          `json:"tags"`
	Genres       []mbTag          `json:"genres"`
}

type mbArtistCredit struct {
	Name string `json:"name"`
}

type mbRelease struct {
	Title        string  `json:"title"`
	Date         string  `json:"date"`
	Country      string  `json:"country"`
	Status       string  `json:"status"`
	ReleaseGroup mbGroup `json:"release-group"`
	Tags         []mbTag `json:"tags"`
	Genres       []mbTag `json:"genres"`
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

type mbTag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type mbGroup struct {
	PrimaryType   string   `json:"primary-type"`
	SecondaryType []string `json:"secondary-types"`
	FirstRelease  string   `json:"first-release-date"`
	Tags          []mbTag  `json:"tags"`
}

func (j *musicBrainzMetadataJob) fetchMetadata(title, artist string) (string, int, string, string, error) {
	inputTitle := strings.TrimSpace(title)
	normalizedTitle := normalizeRecordingQueryText(title)
	normalizedArtist := normalizeRecordingQueryText(artist)
	query := fmt.Sprintf("recording:\"%s\" AND artist:\"%s\"", normalizedTitle, normalizedArtist)
	u := "https://musicbrainz.org/ws/2/recording/?query=" + url.QueryEscape(query) + "&fmt=json&limit=10"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", 0, "", "", err
	}
	req.Header.Set("User-Agent", "Navidrome/metadata-fetcher (https://www.navidrome.org)")

	resp, err := j.client.Do(req)
	if err != nil {
		return "", 0, "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, "", "", fmt.Errorf("musicbrainz status %d", resp.StatusCode)
	}

	var payload mbSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", 0, "", "", err
	}
	if len(payload.Recordings) == 0 {
		return "", 0, "", "rejected", nil
	}

	bestRecording, ok := selectBestRecording(payload.Recordings, inputTitle, artist)
	if !ok {
		return "", 0, "", "rejected", nil
	}

	bestRelease, ok := selectBestRelease(bestRecording.Releases, "")
	if !ok {
		return "", 0, "", "ambiguous", nil
	}

	album := strings.TrimSpace(bestRelease.Title)
	year := yearFromDate(bestRelease.ReleaseGroup.FirstRelease)
	if year == 0 {
		year = yearFromDate(bestRelease.Date)
	}
	genre := collectGenre(bestRecording, bestRelease)
	return album, year, genre, "strict", nil
}

func selectBestRecording(recordings []mbRecording, title, artist string) (mbRecording, bool) {
	normalizedArtist := normalizeRecordingComparisonText(artist)
	normalizedTitle := normalizeRecordingComparisonText(title)
	originalHasLiveRemix := hasLiveOrRemixToken(title)

	bestScore := -1
	bestIdx := -1
	for i, rec := range recordings {
		if rec.Score.Int() < 85 {
			continue
		}
		if !artistCreditMatches(rec.ArtistCredit, normalizedArtist) {
			continue
		}

		recTitle := normalizeRecordingComparisonText(rec.Title)
		similarity := titleSimilarityScore(normalizedTitle, recTitle)
		if similarity < 90 {
			continue
		}
		if !originalHasLiveRemix && hasLiveOrRemixToken(rec.Title) {
			continue
		}

		strictScore := rec.Score.Int()*2 + similarity
		if strictScore > bestScore {
			bestScore = strictScore
			bestIdx = i
		}
	}

	if bestIdx < 0 {
		return mbRecording{}, false
	}
	return recordings[bestIdx], true
}

func artistCreditMatches(credits []mbArtistCredit, normalizedArtist string) bool {
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

func selectBestRelease(releases []mbRelease, artistCountry string) (mbRelease, bool) {
	bestScore := -1000
	secondBestScore := -1000
	bestIdx := -1
	hasAlbum := false

	for _, release := range releases {
		if isAlbumRelease(release) {
			hasAlbum = true
			break
		}
	}

	for i, release := range releases {
		score := scoreRelease(release, artistCountry, hasAlbum)
		if score > bestScore {
			secondBestScore = bestScore
			bestScore = score
			bestIdx = i
		} else if score > secondBestScore {
			secondBestScore = score
		}
	}

	if bestIdx < 0 || bestScore <= 60 {
		return mbRelease{}, false
	}
	if secondBestScore > -1000 && bestScore-secondBestScore < 5 {
		return mbRelease{}, false
	}
	return releases[bestIdx], true
}

func isAlbumRelease(release mbRelease) bool {
	primaryType := normalizeMBString(release.ReleaseGroup.PrimaryType)
	return primaryType == "album"
}

func scoreRelease(release mbRelease, artistCountry string, hasAlbum bool) int {
	if strings.TrimSpace(release.Title) == "" {
		return -1000
	}

	score := 0
	primaryType := normalizeMBString(release.ReleaseGroup.PrimaryType)
	if primaryType == "album" {
		score += 50
	}
	if strings.EqualFold(strings.TrimSpace(release.Status), "official") {
		score += 20
	}
	if strings.TrimSpace(release.ReleaseGroup.FirstRelease) != "" {
		score += 15
	}
	if artistCountry != "" && strings.EqualFold(strings.TrimSpace(release.Country), strings.TrimSpace(artistCountry)) {
		score += 10
	}
	if primaryType == "compilation" {
		score -= 30
	}
	for _, st := range release.ReleaseGroup.SecondaryType {
		if normalizeMBString(st) == "live" {
			score -= 25
			break
		}
	}
	if primaryType == "single" && hasAlbum {
		score -= 20
	}
	return score
}

func collectRecordingGenre(rec mbRecording) string {
	return collectGenres(rec.Genres, rec.Tags)
}

func collectGenre(rec mbRecording, release mbRelease) string {
	genre := collectGenres(release.Genres, release.Tags)
	if genre != "" {
		return genre
	}
	genre = collectGenres(nil, release.ReleaseGroup.Tags)
	if genre != "" {
		return genre
	}
	return collectRecordingGenre(rec)
}

func collectGenres(genres, tags []mbTag) string {
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
		if g.Count <= 1 {
			continue
		}
		appendName(mapMusicBrainzGenre(g.Name))
	}
	for _, t := range tags {
		if t.Count <= 1 {
			continue
		}
		appendName(mapMusicBrainzGenre(t.Name))
	}
	if len(ordered) == 0 {
		return ""
	}
	if len(ordered) > 5 {
		ordered = ordered[:5]
	}
	return strings.Join(ordered, ", ")
}

func mapMusicBrainzGenre(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "hip hop", "hip-hop":
		return "Hip-Hop"
	case "r&b", "rhythm and blues":
		return "R&B"
	case "electronica":
		return "Electronic"
	default:
		return strings.TrimSpace(v)
	}
}

func normalizeRecordingQueryText(v string) string {
	v = stripMixDescriptors(v)
	v = normalizeMBString(v)
	return strings.TrimSpace(v)
}

func normalizeRecordingComparisonText(v string) string {
	v = stripMixDescriptors(v)
	v = normalizeMBString(v)
	return strings.TrimSpace(v)
}

func stripMixDescriptors(v string) string {
	replacer := strings.NewReplacer("(", " ", ")", " ", "[", " ", "]", " ", "{", " ", "}", " ")
	v = replacer.Replace(v)
	patterns := []string{"remix", "live", "radio edit", "remastered", "feat", "ft"}
	for _, p := range patterns {
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(p) + `\b\.?[^\s]*`)
		v = re.ReplaceAllString(v, " ")
	}
	return v
}

func hasLiveOrRemixToken(v string) bool {
	n := normalizeMBString(v)
	return strings.Contains(n, " live") || strings.Contains(n, " remix") || strings.HasPrefix(n, "live ") || strings.HasPrefix(n, "remix ")
}

func titleSimilarityScore(a, b string) int {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 100
	}
	distance := levenshteinDistance(a, b)
	maxLen := max(len([]rune(a)), len([]rune(b)))
	if maxLen == 0 {
		return 100
	}
	score := int(math.Round((1 - float64(distance)/float64(maxLen)) * 100))
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func levenshteinDistance(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}
	prev := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr := make([]int, len(br)+1)
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 0
			if ar[i-1] != br[j-1] {
				cost = 1
			}
			curr[j] = min(min(curr[j-1]+1, prev[j]+1), prev[j-1]+cost)
		}
		prev = curr
	}
	return prev[len(br)]
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
	if y < 1900 || y > time.Now().Year() {
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
