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

	for i, c := range candidates {
		if i > 0 {
			<-ticker.C
		}
		j.setFetching(c.flags, 1)

		album, year, genre, fetchErr := j.fetchMetadata(c.mf.Title, c.mf.Artist)
		if fetchErr != nil {
			log.Warn(ctx, "Could not fetch metadata from MusicBrainz", "songId", c.mf.ID, "title", c.mf.Title, "artist", c.mf.Artist, fetchErr)
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
				log.Error(ctx, "Could not update fetched MusicBrainz metadata", "songId", c.mf.ID, err)
			} else {
				j.setUpdated(setAlbum != nil, setYear != nil, setGenre != nil)
			}
		}

		j.finishFetch(c.flags, album != "", year > 0, genre != "")
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
			album: strings.TrimSpace(mf.Album) == "",
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
	Recordings []struct {
		FirstReleaseDate string `json:"first-release-date"`
		Releases         []struct {
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

func (j *musicBrainzMetadataJob) fetchMetadata(title, artist string) (string, int, string, error) {
	query := fmt.Sprintf("recording:\"%s\" AND artist:\"%s\"", title, artist)
	u := "https://musicbrainz.org/ws/2/recording/?query=" + url.QueryEscape(query) + "&fmt=json"
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
	rec := payload.Recordings[0]

	album := ""
	if len(rec.Releases) > 0 {
		album = strings.TrimSpace(rec.Releases[0].Title)
	}
	year := yearFromDate(rec.FirstReleaseDate)
	if year == 0 && len(rec.Releases) > 0 {
		year = yearFromDate(rec.Releases[0].Date)
	}
	genre := collectGenre(rec)
	return album, year, genre, nil
}

func collectGenre(rec struct {
	FirstReleaseDate string `json:"first-release-date"`
	Releases         []struct {
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
