package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
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
	Missing             int `json:"missing"`
	Fetching            int `json:"fetching"`
	Fetched             int `json:"fetched"`
	Updated             int `json:"updated"`
	Left                int `json:"left"`
	MissingAfterUpdates int `json:"missingAfterUpdates"`
}

type musicBrainzMetadataStatus struct {
	Running    bool                  `json:"running"`
	StartedAt  *time.Time            `json:"startedAt,omitempty"`
	FinishedAt *time.Time            `json:"finishedAt,omitempty"`
	LastError  string                `json:"lastError,omitempty"`
	Album      metadataFieldProgress `json:"album"`
	Year       metadataFieldProgress `json:"year"`
	Genre      metadataFieldProgress `json:"genre"`
	CoverArt   metadataFieldProgress `json:"coverArt"`
}

type musicBrainzMetadataJob struct {
	mu                     sync.RWMutex
	status                 musicBrainzMetadataStatus
	client                 *http.Client
	discogsToken           string
	discogsReleaseIDCache  map[string]int
	discogsReleaseURLCache map[int]string
}

func newMusicBrainzMetadataJob() *musicBrainzMetadataJob {
	return &musicBrainzMetadataJob{
		client:                 &http.Client{Timeout: 20 * time.Second},
		discogsToken:           strings.TrimSpace(os.Getenv("ND_DISCOGS_TOKEN")),
		discogsReleaseIDCache:  map[string]int{},
		discogsReleaseURLCache: map[int]string{},
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
	album    bool
	year     bool
	genre    bool
	coverArt bool
}

type mbMetadataCandidate struct {
	mf    model.MediaFile
	flags missingFlags
}

type coverArtRunStats struct {
	totalMissing int
	fetching     int
	matched      int
	failed       int
	skipped      int
}

func (j *musicBrainzMetadataJob) run(ds model.DataStore) {
	ctx := context.Background()
	mbTicker := time.NewTicker(time.Second)
	discogsTicker := time.NewTicker(time.Second)
	defer mbTicker.Stop()
	defer discogsTicker.Stop()

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

	stats := coverArtRunStats{}
	for _, c := range candidates {
		if c.flags.coverArt {
			stats.totalMissing++
		} else {
			stats.skipped++
		}
		j.setFetching(c.flags, 1)

		album, year, genre, releaseMBID, fetchErr := j.fetchMetadata(mbTicker, c.mf.Title, c.mf.Artist)
		if fetchErr != nil {
			log.Warn(ctx, "Could not fetch metadata from MusicBrainz", "songId", c.mf.ID, "title", c.mf.Title, "artist", c.mf.Artist, fetchErr)
		}

		coverURL := ""
		if c.flags.coverArt {
			stats.fetching++
			if releaseMBID != "" {
				coverURL, _ = j.fetchCoverArtURL(releaseMBID)
			}
			if coverURL == "" {
				coverURL, _ = j.fetchCoverArtFromDiscogs(discogsTicker, c.mf)
			}
			if coverURL != "" {
				stats.matched++
			} else {
				stats.failed++
			}
		}

		var setAlbum *string
		var setYear *int
		var setGenre *string
		var setCoverArtURL *string

		if c.flags.album && album != "" {
			setAlbum = &album
		}
		if c.flags.year && year > 0 {
			setYear = &year
		}
		if c.flags.genre && genre != "" {
			setGenre = &genre
		}
		if c.flags.coverArt && coverURL != "" {
			setCoverArtURL = &coverURL
		}

		updated := missingFlags{}
		if setAlbum != nil || setYear != nil || setGenre != nil || setCoverArtURL != nil {
			if err := ds.MediaFile(ctx).UpdateMissingMetadata(c.mf.ID, setAlbum, setYear, setGenre, setCoverArtURL); err != nil {
				log.Error(ctx, "Could not update fetched metadata", "songId", c.mf.ID, err)
			} else {
				updated = missingFlags{album: setAlbum != nil, year: setYear != nil, genre: setGenre != nil, coverArt: setCoverArtURL != nil}
				j.setUpdated(updated)
			}
		}

		fetched := missingFlags{album: album != "", year: year > 0, genre: genre != "", coverArt: coverURL != ""}
		j.finishFetch(c.flags, fetched, updated)
	}

	log.Info(ctx, "Cover art metadata run summary",
		"total_missing", stats.totalMissing,
		"fetching", stats.fetching,
		"matched", stats.matched,
		"failed", stats.failed,
		"skipped", stats.skipped,
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
			album:    strings.TrimSpace(mf.Album) == "",
			year:     mf.Year == 0,
			genre:    strings.TrimSpace(mf.Genre) == "",
			coverArt: !mf.HasCoverArt && strings.TrimSpace(mf.CoverArtURL) == "",
		}
		if !flags.album && !flags.year && !flags.genre && !flags.coverArt {
			continue
		}

		res = append(res, mbMetadataCandidate{mf: mf, flags: flags})
		j.incrementMissing(flags)
	}
	return res, nil
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
	if flags.coverArt {
		j.status.CoverArt.Fetching += delta
	}
}

func (j *musicBrainzMetadataJob) setUpdated(flags missingFlags) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if flags.album {
		j.status.Album.Updated++
	}
	if flags.year {
		j.status.Year.Updated++
	}
	if flags.genre {
		j.status.Genre.Updated++
	}
	if flags.coverArt {
		j.status.CoverArt.Updated++
	}
}

func (j *musicBrainzMetadataJob) finishFetch(requested, fetched, updated missingFlags) {
	j.mu.Lock()
	defer j.mu.Unlock()
	finish := func(field *metadataFieldProgress, req, got, upd bool) {
		if !req {
			return
		}
		field.Fetching--
		field.Left--
		if got {
			field.Fetched++
		}
		if !upd {
			field.MissingAfterUpdates++
		}
	}
	finish(&j.status.Album, requested.album, fetched.album, updated.album)
	finish(&j.status.Year, requested.year, fetched.year, updated.year)
	finish(&j.status.Genre, requested.genre, fetched.genre, updated.genre)
	finish(&j.status.CoverArt, requested.coverArt, fetched.coverArt, updated.coverArt)
}

func (j *musicBrainzMetadataJob) setError(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status.LastError = err.Error()
}

type mbSearchResponse struct {
	Recordings []struct {
		FirstReleaseDate string `json:"first-release-date"`
		Releases         []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Date  string `json:"date"`
		} `json:"releases"`
		Tags []struct {
			Name string `json:"name"`
		} `json:"tags"`
		Genres []struct {
			Name string `json:"name"`
		} `json:"genres"`
	} `json:"recordings"`
}

func (j *musicBrainzMetadataJob) fetchMetadata(mbTicker *time.Ticker, title, artist string) (string, int, string, string, error) {
	<-mbTicker.C
	query := fmt.Sprintf("recording:\"%s\" AND artist:\"%s\"", sanitizeQuery(title), sanitizeQuery(artist))
	u := "https://musicbrainz.org/ws/2/recording/?query=" + url.QueryEscape(query) + "&fmt=json"
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
		return "", 0, "", "", nil
	}
	rec := payload.Recordings[0]

	album := ""
	releaseMBID := ""
	if len(rec.Releases) > 0 {
		album = strings.TrimSpace(rec.Releases[0].Title)
		releaseMBID = strings.TrimSpace(rec.Releases[0].ID)
	}
	year := yearFromDate(rec.FirstReleaseDate)
	if year == 0 && len(rec.Releases) > 0 {
		year = yearFromDate(rec.Releases[0].Date)
	}
	genre := collectGenre(rec)
	return album, year, genre, releaseMBID, nil
}

func sanitizeQuery(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, `"`, "")
	return s
}

type caaResponse struct {
	Images []struct {
		Front      bool   `json:"front"`
		Image      string `json:"image"`
		Thumbnails struct {
			Large string `json:"large"`
		} `json:"thumbnails"`
	} `json:"images"`
}

func (j *musicBrainzMetadataJob) fetchCoverArtURL(releaseMBID string) (string, error) {
	u := "https://coverartarchive.org/release/" + url.PathEscape(releaseMBID)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Navidrome/metadata-fetcher (https://www.navidrome.org)")
	resp, err := j.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("coverartarchive status %d", resp.StatusCode)
	}

	var payload caaResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if len(payload.Images) == 0 {
		return "", nil
	}
	for _, img := range payload.Images {
		if img.Front {
			return pickCoverArtURL(img.Image, img.Thumbnails.Large), nil
		}
	}
	return pickCoverArtURL(payload.Images[0].Image, payload.Images[0].Thumbnails.Large), nil
}

type discogsSearchResponse struct {
	Results []struct {
		ID          int      `json:"id"`
		Title       string   `json:"title"`
		Country     string   `json:"country"`
		Year        int      `json:"year"`
		Type        string   `json:"type"`
		Format      []string `json:"format"`
		CoverImage  string   `json:"cover_image"`
		ResourceURL string   `json:"resource_url"`
		Community   struct {
			Want int `json:"want"`
			Have int `json:"have"`
		} `json:"community"`
	} `json:"results"`
}

type discogsReleaseResponse struct {
	Images []struct {
		Type string `json:"type"`
		URI  string `json:"uri"`
	} `json:"images"`
}

func (j *musicBrainzMetadataJob) fetchCoverArtFromDiscogs(discogsTicker *time.Ticker, mf model.MediaFile) (string, error) {
	if j.discogsToken == "" {
		return "", nil
	}
	cacheKey := strings.ToLower(strings.TrimSpace(mf.Artist)) + "|" + strings.ToLower(strings.TrimSpace(mf.Title))
	if releaseID, ok := j.getCachedDiscogsRelease(cacheKey); ok {
		if u, ok := j.getCachedDiscogsReleaseURL(releaseID); ok {
			return u, nil
		}
		<-discogsTicker.C
		releaseURL := fmt.Sprintf("https://api.discogs.com/releases/%d", releaseID)
		img, err := j.fetchDiscogsReleaseImage(releaseURL)
		if err == nil && img != "" {
			j.setCachedDiscogsReleaseURL(releaseID, img)
			return img, nil
		}
	}

	<-discogsTicker.C
	u := fmt.Sprintf("https://api.discogs.com/database/search?q=%s&type=release&token=%s", url.QueryEscape(strings.TrimSpace(mf.Artist)+" "+strings.TrimSpace(mf.Title)), url.QueryEscape(j.discogsToken))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Navidrome/metadata-fetcher (https://www.navidrome.org)")
	resp, err := j.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("discogs search status %d", resp.StatusCode)
	}

	var payload discogsSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if len(payload.Results) == 0 {
		return "", nil
	}

	best := j.selectBestDiscogsResult(payload.Results, mf)
	if best == nil {
		return "", nil
	}
	j.setCachedDiscogsRelease(cacheKey, best.ID)

	if strings.TrimSpace(best.CoverImage) != "" {
		j.setCachedDiscogsReleaseURL(best.ID, strings.TrimSpace(best.CoverImage))
		return strings.TrimSpace(best.CoverImage), nil
	}
	if strings.TrimSpace(best.ResourceURL) == "" {
		return "", nil
	}

	<-discogsTicker.C
	img, err := j.fetchDiscogsReleaseImage(best.ResourceURL)
	if err != nil {
		return "", err
	}
	if img != "" {
		j.setCachedDiscogsReleaseURL(best.ID, img)
	}
	return img, nil
}

func (j *musicBrainzMetadataJob) selectBestDiscogsResult(results []struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Country     string   `json:"country"`
	Year        int      `json:"year"`
	Type        string   `json:"type"`
	Format      []string `json:"format"`
	CoverImage  string   `json:"cover_image"`
	ResourceURL string   `json:"resource_url"`
	Community   struct {
		Want int `json:"want"`
		Have int `json:"have"`
	} `json:"community"`
}, mf model.MediaFile) *struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Country     string   `json:"country"`
	Year        int      `json:"year"`
	Type        string   `json:"type"`
	Format      []string `json:"format"`
	CoverImage  string   `json:"cover_image"`
	ResourceURL string   `json:"resource_url"`
	Community   struct {
		Want int `json:"want"`
		Have int `json:"have"`
	} `json:"community"`
} {
	artistNorm := normalizeText(mf.Artist)
	titleNorm := normalizeText(mf.Title)
	targetCountry := normalizeText(firstTagValue(mf.Tags, "releasecountry", "country"))
	targetYear := mf.Year

	var best *struct {
		ID          int      `json:"id"`
		Title       string   `json:"title"`
		Country     string   `json:"country"`
		Year        int      `json:"year"`
		Type        string   `json:"type"`
		Format      []string `json:"format"`
		CoverImage  string   `json:"cover_image"`
		ResourceURL string   `json:"resource_url"`
		Community   struct {
			Want int `json:"want"`
			Have int `json:"have"`
		} `json:"community"`
	}
	bestScore := -999.0
	for i := range results {
		r := &results[i]
		if !strings.EqualFold(strings.TrimSpace(r.Type), "release") {
			continue
		}
		if isUnofficialOrPromo(r.Format) {
			continue
		}

		resArtist, resTitle := splitDiscogsTitle(r.Title)
		score := 0.0
		if normalizeText(resArtist) == artistNorm {
			score += 100
		} else if strings.Contains(normalizeText(resArtist), artistNorm) {
			score += 70
		}

		tsim := titleSimilarity(titleNorm, normalizeText(resTitle))
		score += tsim * 50

		score += formatPriority(r.Format)

		if targetCountry != "" && normalizeText(r.Country) == targetCountry {
			score += 10
		}
		if targetYear > 0 && r.Year > 0 {
			delta := absInt(targetYear - r.Year)
			score += float64(maxInt(0, 10-delta))
		}
		score += float64(r.Community.Have+r.Community.Want) / 1000.0

		if score > bestScore {
			bestScore = score
			best = r
		}
	}
	return best
}

func (j *musicBrainzMetadataJob) fetchDiscogsReleaseImage(resourceURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, resourceURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Navidrome/metadata-fetcher (https://www.navidrome.org)")
	resp, err := j.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("discogs release status %d", resp.StatusCode)
	}
	var rel discogsReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	if len(rel.Images) == 0 {
		return "", nil
	}
	for _, img := range rel.Images {
		if strings.EqualFold(strings.TrimSpace(img.Type), "primary") && strings.TrimSpace(img.URI) != "" {
			return strings.TrimSpace(img.URI), nil
		}
	}
	return strings.TrimSpace(rel.Images[0].URI), nil
}

func splitDiscogsTitle(s string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(s), " - ", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", s
}

func titleSimilarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	ta := strings.Fields(a)
	tb := strings.Fields(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0
	}
	setA := map[string]bool{}
	for _, t := range ta {
		setA[t] = true
	}
	inter := 0
	for _, t := range tb {
		if setA[t] {
			inter++
		}
	}
	den := len(ta) + len(tb) - inter
	if den <= 0 {
		return 0
	}
	return float64(inter) / float64(den)
}

func formatPriority(formats []string) float64 {
	score := 0.0
	for _, f := range formats {
		n := normalizeText(f)
		switch {
		case n == "album":
			score += 20
		case n == "ep":
			score += 15
		case n == "single":
			score += 5
		case n == "compilation":
			score -= 10
		case n == "live":
			score -= 10
		}
	}
	return score
}

func isUnofficialOrPromo(formats []string) bool {
	for _, f := range formats {
		n := normalizeText(f)
		if n == "unofficial release" || n == "promo" || n == "promotional" || n == "bootleg" {
			return true
		}
	}
	return false
}

func normalizeText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	b := strings.Builder{}
	lastSpace := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func firstTagValue(tags model.Tags, keys ...string) string {
	for _, k := range keys {
		vals := tags[model.TagName(k)]
		if len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

func pickCoverArtURL(image, large string) string {
	if strings.TrimSpace(large) != "" {
		return strings.TrimSpace(large)
	}
	return strings.TrimSpace(image)
}

func collectGenre(rec struct {
	FirstReleaseDate string `json:"first-release-date"`
	Releases         []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Date  string `json:"date"`
	} `json:"releases"`
	Tags []struct {
		Name string `json:"name"`
	} `json:"tags"`
	Genres []struct {
		Name string `json:"name"`
	} `json:"genres"`
}) string {
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

	for _, g := range rec.Genres {
		appendName(g.Name)
	}
	for _, t := range rec.Tags {
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

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (j *musicBrainzMetadataJob) getCachedDiscogsRelease(key string) (int, bool) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	id, ok := j.discogsReleaseIDCache[key]
	return id, ok
}

func (j *musicBrainzMetadataJob) setCachedDiscogsRelease(key string, releaseID int) {
	if releaseID <= 0 {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.discogsReleaseIDCache[key] = releaseID
}

func (j *musicBrainzMetadataJob) getCachedDiscogsReleaseURL(releaseID int) (string, bool) {
	j.mu.RLock()
	defer j.mu.RUnlock()
	u, ok := j.discogsReleaseURLCache[releaseID]
	return u, ok
}

func (j *musicBrainzMetadataJob) setCachedDiscogsReleaseURL(releaseID int, coverURL string) {
	if releaseID <= 0 || strings.TrimSpace(coverURL) == "" {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.discogsReleaseURLCache[releaseID] = strings.TrimSpace(coverURL)
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
