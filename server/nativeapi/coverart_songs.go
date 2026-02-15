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

	"github.com/Masterminds/squirrel"
	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
)

const (
	coverArtBatchSize    = 50
	coverArtPlaceholder  = "/musicmatters.png"
	musicBrainzUserAgent = "Navidrome-MusicMatters/1.0 (https://github.com/navidrome/navidrome)"
)

type coverArtSong struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Album       string  `json:"album"`
	Genre       string  `json:"genre"`
	Year        int     `json:"year"`
	Duration    float32 `json:"duration"`
	CoverArtURL string  `json:"coverArtUrl"`
}

type coverArtStatus struct {
	Running              bool   `json:"running"`
	Total                int    `json:"total"`
	Completed            int    `json:"completed"`
	Remaining            int    `json:"remaining"`
	Updated              int    `json:"updated"`
	NotFound             int    `json:"notFound"`
	MissingCoverSongs    int64  `json:"missingCoverSongs"`
	MissingMetadataSongs int64  `json:"missingMetadataSongs"`
	Message              string `json:"message,omitempty"`
}

type mbRecordingResponse struct {
	Recordings []struct {
		Length           int     `json:"length"`
		FirstReleaseDate string  `json:"first-release-date"`
		Tags             []mbTag `json:"tags"`
		Releases         []mbRel `json:"releases"`
	} `json:"recordings"`
}

type mbTag struct {
	Name string `json:"name"`
}

type mbRel struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type musicBrainzClient struct {
	httpClient  *http.Client
	mu          sync.Mutex
	lastRequest time.Time
}

type coverArtRefreshTracker struct {
	mu     sync.Mutex
	status coverArtStatus
}

func newMusicBrainzClient() *musicBrainzClient {
	return &musicBrainzClient{httpClient: &http.Client{Timeout: 10 * time.Second}}
}

func newCoverArtRefreshTracker() *coverArtRefreshTracker {
	return &coverArtRefreshTracker{status: coverArtStatus{}}
}

func (c *musicBrainzClient) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	wait := time.Second - time.Since(c.lastRequest)
	if wait > 0 {
		time.Sleep(wait)
	}
	c.lastRequest = time.Now()
}

func (c *musicBrainzClient) do(req *http.Request) (*http.Response, error) {
	c.throttle()
	req.Header.Set("User-Agent", musicBrainzUserAgent)
	req.Header.Set("Accept", "application/json")
	return c.httpClient.Do(req)
}

func (c *musicBrainzClient) searchRecording(ctx context.Context, title, artist string) (*mbRecordingResponse, error) {
	query := fmt.Sprintf(`recording:"%s" AND artist:"%s"`, title, artist)
	u := "https://musicbrainz.org/ws/2/recording/?query=" + url.QueryEscape(query) + "&fmt=json&limit=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz search failed: %s", resp.Status)
	}
	var data mbRecordingResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (c *musicBrainzClient) coverArtExists(ctx context.Context, releaseID string) bool {
	if releaseID == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "https://coverartarchive.org/release/"+releaseID+"/front", nil)
	if err != nil {
		return false
	}
	resp, err := c.do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (n *Router) addCoverArtSongsRoute(r chi.Router) {
	r.Route("/coverart-songs", func(r chi.Router) {
		r.Get("/", n.handleCoverArtSongs)
		r.Get("/status", n.handleCoverArtStatus)
		r.Post("/refresh", n.handleCoverArtRefresh)
	})
}

var (
	coverArtUpdaterMu sync.Mutex
	coverArtUpdater   = newMusicBrainzClient()
	coverArtTracker   = newCoverArtRefreshTracker()
)

func (t *coverArtRefreshTracker) snapshot() coverArtStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.status
	s.Remaining = max(0, s.Total-s.Completed)
	return s
}

func (t *coverArtRefreshTracker) setStart(total int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status = coverArtStatus{
		Running:   true,
		Total:     total,
		Completed: 0,
		Updated:   0,
		NotFound:  0,
		Message:   "Refresh started",
	}
}

func (t *coverArtRefreshTracker) setComplete(msg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.Running = false
	t.status.Message = msg
	t.status.Remaining = max(0, t.status.Total-t.status.Completed)
}

func (t *coverArtRefreshTracker) markSong(updated bool, found bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.Completed++
	if updated {
		t.status.Updated++
	}
	if !found {
		t.status.NotFound++
	}
	t.status.Remaining = max(0, t.status.Total-t.status.Completed)
}

func (n *Router) handleCoverArtSongs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start, end := parseRange(r)
	limit := end - start
	if limit <= 0 || limit > coverArtBatchSize {
		limit = coverArtBatchSize
	}

	search := strings.TrimSpace(r.URL.Query().Get("q"))
	refreshMissingOnly := strings.EqualFold(r.URL.Query().Get("refreshMissingCoverArt"), "true")
	filters := listFilters(search)

	repo := n.ds.MediaFile(ctx)
	total, err := repo.CountAll(model.QueryOptions{Filters: filters})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	songs, err := repo.GetAll(model.QueryOptions{
		Sort:    "title",
		Order:   "ASC",
		Offset:  start,
		Max:     limit,
		Filters: filters,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := make([]coverArtSong, 0, len(songs))
	toUpdate := make([]model.MediaFile, 0, coverArtBatchSize)
	for i := range songs {
		song := songs[i]
		n.populateSongArtwork(r, &song)
		coverURL := song.ArtworkURL
		if coverURL == "" && song.MbzAlbumID != "" {
			coverURL = "https://coverartarchive.org/release/" + song.MbzAlbumID + "/front"
		}
		if coverURL == "" {
			coverURL = coverArtPlaceholder
		}
		resp = append(resp, coverArtSong{
			ID:          song.ID,
			Title:       song.Title,
			Artist:      song.Artist,
			Album:       song.Album,
			Genre:       song.Genre,
			Year:        song.Year,
			Duration:    song.Duration,
			CoverArtURL: coverURL,
		})

		if len(toUpdate) < coverArtBatchSize && needsMetadataUpdate(song, refreshMissingOnly) {
			toUpdate = append(toUpdate, song)
		}
	}

	if len(toUpdate) > 0 {
		go n.updateSongsFromMusicBrainz(request.AddValues(context.Background(), ctx), toUpdate, refreshMissingOnly, nil)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Total-Count", strconv.FormatInt(total, 10))
	w.Header().Set("Access-Control-Expose-Headers", "X-Total-Count")
	_ = json.NewEncoder(w).Encode(resp)
}

func (n *Router) handleCoverArtRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := coverArtTracker.snapshot()
	if status.Running {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(status)
		return
	}

	songs, err := n.ds.MediaFile(ctx).GetAll(model.QueryOptions{
		Sort:  "title",
		Order: "ASC",
		Max:   coverArtBatchSize,
		Filters: squirrel.And{
			squirrel.Eq{"missing": false},
			squirrel.Eq{"has_cover_art": false},
		},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	coverArtTracker.setStart(len(songs))
	go func(items []model.MediaFile) {
		if len(items) == 0 {
			coverArtTracker.setComplete("No songs with missing cover art in current batch")
			return
		}
		n.updateSongsFromMusicBrainz(request.AddValues(context.Background(), ctx), items, true, coverArtTracker)
		coverArtTracker.setComplete("Refresh finished")
	}(songs)

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(coverArtTracker.snapshot())
}

func (n *Router) handleCoverArtStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := coverArtTracker.snapshot()
	status.MissingCoverSongs = n.countMissingCoverSongs(ctx)
	status.MissingMetadataSongs = n.countMissingMetadataSongs(ctx)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (n *Router) countMissingCoverSongs(ctx context.Context) int64 {
	count, err := n.ds.MediaFile(ctx).CountAll(model.QueryOptions{Filters: squirrel.And{
		squirrel.Eq{"missing": false},
		squirrel.Eq{"has_cover_art": false},
	}})
	if err != nil {
		log.Debug(ctx, "Could not count missing cover songs", "err", err)
		return 0
	}
	return count
}

func (n *Router) countMissingMetadataSongs(ctx context.Context) int64 {
	count, err := n.ds.MediaFile(ctx).CountAll(model.QueryOptions{Filters: squirrel.And{
		squirrel.Eq{"missing": false},
		squirrel.Or{
			squirrel.Eq{"album": ""},
			squirrel.Eq{"genre": ""},
			squirrel.Eq{"year": 0},
			squirrel.Eq{"duration": 0},
		},
	}})
	if err != nil {
		log.Debug(ctx, "Could not count missing metadata songs", "err", err)
		return 0
	}
	return count
}

func listFilters(search string) squirrel.And {
	filters := squirrel.And{squirrel.Eq{"missing": false}}
	if search == "" {
		return filters
	}
	like := "%" + strings.ToLower(search) + "%"
	return append(filters, squirrel.Or{
		squirrel.Expr("lower(title) like ?", like),
		squirrel.Expr("lower(artist) like ?", like),
		squirrel.Expr("lower(album) like ?", like),
	})
}

func parseRange(r *http.Request) (int, int) {
	q := r.URL.Query()
	start, _ := strconv.Atoi(q.Get("_start"))
	end, _ := strconv.Atoi(q.Get("_end"))
	if end <= start {
		end = start + coverArtBatchSize
	}
	if start < 0 {
		start = 0
	}
	return start, end
}

func needsMetadataUpdate(song model.MediaFile, refreshMissingOnly bool) bool {
	if refreshMissingOnly {
		return !song.HasCoverArt
	}
	return song.Album == "" || song.Genre == "" || song.Year == 0 || song.Duration == 0 || !song.HasCoverArt
}

func (n *Router) updateSongsFromMusicBrainz(
	ctx context.Context,
	songs []model.MediaFile,
	refreshMissingOnly bool,
	tracker *coverArtRefreshTracker,
) {
	coverArtUpdaterMu.Lock()
	defer coverArtUpdaterMu.Unlock()

	for _, song := range songs {
		updated, found := n.updateSingleSong(ctx, song, refreshMissingOnly)
		if tracker != nil {
			tracker.markSong(updated, found)
		}
	}
}

func (n *Router) updateSingleSong(ctx context.Context, song model.MediaFile, refreshMissingOnly bool) (bool, bool) {
	updated := song
	changed := false
	found := false

	if (!refreshMissingOnly && (updated.Album == "" || updated.Genre == "" || updated.Year == 0 || updated.Duration == 0 || updated.MbzAlbumID == "")) || (refreshMissingOnly && updated.MbzAlbumID == "") {
		mbData, err := coverArtUpdater.searchRecording(ctx, updated.Title, updated.Artist)
		if err != nil {
			log.Debug(ctx, "MusicBrainz lookup failed", "songId", updated.ID, "err", err)
			return false, false
		}
		if len(mbData.Recordings) > 0 {
			found = true
			rec := mbData.Recordings[0]
			if !refreshMissingOnly && updated.Album == "" && len(rec.Releases) > 0 && rec.Releases[0].Title != "" {
				updated.Album = rec.Releases[0].Title
				changed = true
			}
			if !refreshMissingOnly && updated.Year == 0 && len(rec.FirstReleaseDate) >= 4 {
				if y, err := strconv.Atoi(rec.FirstReleaseDate[:4]); err == nil {
					updated.Year = y
					changed = true
				}
			}
			if !refreshMissingOnly && updated.Genre == "" && len(rec.Tags) > 0 {
				updated.Genre = rec.Tags[0].Name
				changed = true
			}
			if !refreshMissingOnly && updated.Duration == 0 && rec.Length > 0 {
				updated.Duration = float32(rec.Length) / 1000
				changed = true
			}
			if updated.MbzAlbumID == "" && len(rec.Releases) > 0 && rec.Releases[0].ID != "" {
				updated.MbzAlbumID = rec.Releases[0].ID
				changed = true
			}
		}
	}

	if !updated.HasCoverArt && updated.MbzAlbumID != "" {
		if coverArtUpdater.coverArtExists(ctx, updated.MbzAlbumID) {
			found = true
			changed = true
		}
	}

	if !changed {
		return false, found
	}

	err := n.ds.WithTx(func(tx model.DataStore) error {
		repo := tx.MediaFile(ctx)
		stored, err := repo.Get(updated.ID)
		if err != nil {
			return err
		}
		// Never overwrite title/artist
		updated.Title = stored.Title
		updated.Artist = stored.Artist

		if stored.Album != "" {
			updated.Album = stored.Album
		}
		if stored.Genre != "" {
			updated.Genre = stored.Genre
		}
		if stored.Year != 0 {
			updated.Year = stored.Year
		}
		if stored.Duration != 0 {
			updated.Duration = stored.Duration
		}
		if stored.MbzAlbumID != "" {
			updated.MbzAlbumID = stored.MbzAlbumID
		}
		return repo.Put(&updated)
	})
	if err != nil {
		log.Debug(ctx, "Could not persist MusicBrainz data", "songId", updated.ID, "err", err)
		return false, found
	}
	return true, true
}
