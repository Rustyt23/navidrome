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
	mu       sync.RWMutex
	status   musicBrainzMetadataStatus
	client   *http.Client
	mbTicker *time.Ticker
}

func newMusicBrainzMetadataJob() *musicBrainzMetadataJob {
	return &musicBrainzMetadataJob{
		client: &http.Client{Timeout: 20 * time.Second},
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

func (j *musicBrainzMetadataJob) run(ds model.DataStore) {
	ctx := context.Background()
	j.mbTicker = time.NewTicker(time.Second)
	defer j.mbTicker.Stop()

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

	for _, c := range candidates {
		j.setFetching(c.flags, 1)

		album, year, genre, releaseMBID, fetchErr := j.fetchMetadata(c.mf.Title, c.mf.Artist)
		if fetchErr != nil {
			log.Warn(ctx, "Could not fetch metadata from MusicBrainz", "songId", c.mf.ID, "title", c.mf.Title, "artist", c.mf.Artist, fetchErr)
		}

		coverURL := ""
		if c.flags.coverArt && releaseMBID != "" {
			coverURL, _ = j.fetchCoverArtURL(releaseMBID)
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

func (j *musicBrainzMetadataJob) fetchMetadata(title, artist string) (string, int, string, string, error) {
	<-j.mbTicker.C
	query := fmt.Sprintf("recording:%s AND artist:%s", sanitizeQuery(title), sanitizeQuery(artist))
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
	pick := func(img struct {
		Front      bool   `json:"front"`
		Image      string `json:"image"`
		Thumbnails struct {
			Large string `json:"large"`
		} `json:"thumbnails"`
	}) string {
		if strings.TrimSpace(img.Thumbnails.Large) != "" {
			return strings.TrimSpace(img.Thumbnails.Large)
		}
		return strings.TrimSpace(img.Image)
	}
	for _, img := range payload.Images {
		if img.Front {
			return pick(img), nil
		}
	}
	return pick(payload.Images[0]), nil
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
