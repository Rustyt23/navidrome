package nativeapi

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/log"
	"golang.org/x/time/rate"
)

const (
	spotifyTrackTitleDistanceThreshold = 4
	spotifyBatchDefaultLimit           = 0
)

var (
	spotifyReqLimiter = rate.NewLimiter(rate.Every(3*time.Second), 1)
	spotifyTokenMu    sync.Mutex
	spotifyToken      string
	spotifyTokenExp   time.Time
	errSpotify401     = errors.New("spotify search unauthorized (401)")
)

type SpotifyTrackResult struct {
	TrackName string
	Artist    string
	AlbumName string
	Year      int
	CoverURL  string
}

type spotifySearchResponse struct {
	Tracks struct {
		Items []struct {
			Name    string `json:"name"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"artists"`
			Album struct {
				Name        string `json:"name"`
				ReleaseDate string `json:"release_date"`
				Images      []struct {
					URL   string `json:"url"`
					Width int    `json:"width"`
				} `json:"images"`
			} `json:"album"`
		} `json:"items"`
	} `json:"tracks"`
}

type spotifyTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type spotifyMetadataTrack struct {
	ID          string
	Title       string
	Artist      string
	Album       string
	ReleaseYear int
	AlbumID     string
	EmbedArt    sql.NullString
}

type spotifyMetadataResponse struct {
	Processed int `json:"processed"`
	Updated   int `json:"updated"`
	Skipped   int `json:"skipped"`
	Failed    int `json:"failed"`
}

type spotifyFieldStats struct {
	AlreadyExist int `json:"alreadyExist"`
	Missing      int `json:"missing"`
	Fetching     int `json:"fetching"`
	Fetched      int `json:"fetched"`
	Updated      int `json:"updated"`
	ToBeFetch    int `json:"toBeFetch"`
	CouldntFetch int `json:"couldntFetch"`
}

type spotifyMetadataStats struct {
	Album    spotifyFieldStats `json:"album"`
	Year     spotifyFieldStats `json:"year"`
	CoverArt spotifyFieldStats `json:"coverArt"`
}

type spotifyFetchedItem struct {
	AlbumName string `json:"albumName"`
	Year      int    `json:"year"`
	CoverURL  string `json:"coverUrl"`
}

type spotifyMetadataJobState struct {
	Running     bool                    `json:"running"`
	StartedAt   time.Time               `json:"startedAt,omitempty"`
	EndedAt     *time.Time              `json:"endedAt,omitempty"`
	Response    spotifyMetadataResponse `json:"response"`
	Stats       spotifyMetadataStats    `json:"stats"`
	FetchedData []spotifyFetchedItem    `json:"fetchedData,omitempty"`
	Error       string                  `json:"error,omitempty"`
}

var (
	spotifyJobMu    sync.Mutex
	spotifyJobState spotifyMetadataJobState
)

func (n *Router) addSpotifyMetadataRoute(r chi.Router) {
	r.With(adminOnlyMiddleware).Post("/song/metadata/spotify", n.fetchMissingSpotifyMetadata())
	r.With(adminOnlyMiddleware).Get("/song/metadata/spotify/status", n.spotifyMetadataStatus())
}

func (n *Router) fetchMissingSpotifyMetadata() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		limit := spotifyBatchDefaultLimit
		if p := strings.TrimSpace(r.URL.Query().Get("limit")); p != "" {
			if v, err := strconv.Atoi(p); err == nil && v > 0 {
				limit = v
			}
		}

		if conf.Server.DbPath == "" {
			http.Error(w, "database path not configured", http.StatusInternalServerError)
			return
		}

		spotifyJobMu.Lock()
		if spotifyJobState.Running {
			state := spotifyJobState
			spotifyJobMu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(state)
			return
		}
		spotifyJobState = spotifyMetadataJobState{Running: true, StartedAt: time.Now(), FetchedData: []spotifyFetchedItem{}}
		state := spotifyJobState
		spotifyJobMu.Unlock()

		go runSpotifyMetadataJob(context.Background(), limit)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(state)
		log.Info(ctx, "Started Spotify metadata enrichment job", "limit", limit)
	}
}

func (n *Router) spotifyMetadataStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		spotifyJobMu.Lock()
		state := spotifyJobState
		spotifyJobMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(state)
	}
}

func runSpotifyMetadataJob(ctx context.Context, limit int) {
	db, err := sql.Open("sqlite3", conf.Server.DbPath)
	if err != nil {
		finishSpotifyMetadataJobWithError(fmt.Sprintf("unable to open database: %v", err))
		return
	}
	defer db.Close()

	tracks, err := loadTracksMissingSpotifyMetadata(ctx, db, limit)
	if err != nil {
		finishSpotifyMetadataJobWithError(fmt.Sprintf("unable to load tracks: %v", err))
		return
	}

	initializeSpotifyStats(tracks)
	for _, track := range tracks {
		status, err := processTrackSpotifyMetadata(ctx, db, track)
		updateSpotifyJobState(func(st *spotifyMetadataJobState) {
			st.Response.Processed++
		})
		if err != nil {
			updateSpotifyJobState(func(st *spotifyMetadataJobState) { st.Response.Failed++ })
			log.Error(ctx, "Spotify metadata enrichment failed", "trackID", track.ID, "title", track.Title, "artist", track.Artist, "status", status, "err", err)
			continue
		}
		updateSpotifyJobState(func(st *spotifyMetadataJobState) {
			switch status {
			case "updated":
				st.Response.Updated++
			case "failed":
				st.Response.Failed++
			default:
				st.Response.Skipped++
			}
		})
	}

	end := time.Now()
	updateSpotifyJobState(func(st *spotifyMetadataJobState) {
		st.Running = false
		st.EndedAt = &end
	})
}

func finishSpotifyMetadataJobWithError(msg string) {
	end := time.Now()
	updateSpotifyJobState(func(st *spotifyMetadataJobState) {
		st.Running = false
		st.EndedAt = &end
		st.Error = msg
	})
}

func updateSpotifyJobState(fn func(*spotifyMetadataJobState)) {
	spotifyJobMu.Lock()
	defer spotifyJobMu.Unlock()
	fn(&spotifyJobState)
}

func initializeSpotifyStats(tracks []spotifyMetadataTrack) {
	updateSpotifyJobState(func(st *spotifyMetadataJobState) {
		st.Stats = spotifyMetadataStats{}
		for _, tr := range tracks {
			if isUnknownAlbum(tr.Album) {
				st.Stats.Album.Missing++
				st.Stats.Album.ToBeFetch++
			} else {
				st.Stats.Album.AlreadyExist++
			}

			if tr.ReleaseYear == 0 {
				st.Stats.Year.Missing++
				st.Stats.Year.ToBeFetch++
			} else {
				st.Stats.Year.AlreadyExist++
			}

			if strings.TrimSpace(tr.EmbedArt.String) == "" {
				st.Stats.CoverArt.Missing++
				st.Stats.CoverArt.ToBeFetch++
			} else {
				st.Stats.CoverArt.AlreadyExist++
			}
		}
	})
}

func markTrackFetching(track spotifyMetadataTrack) {
	updateSpotifyJobState(func(st *spotifyMetadataJobState) {
		if isUnknownAlbum(track.Album) {
			st.Stats.Album.Fetching++
		}
		if track.ReleaseYear == 0 {
			st.Stats.Year.Fetching++
		}
		if strings.TrimSpace(track.EmbedArt.String) == "" {
			st.Stats.CoverArt.Fetching++
		}
	})
}

func markTrackDone(track spotifyMetadataTrack, failed bool) {
	updateSpotifyJobState(func(st *spotifyMetadataJobState) {
		if isUnknownAlbum(track.Album) {
			if st.Stats.Album.Fetching > 0 {
				st.Stats.Album.Fetching--
			}
			if st.Stats.Album.ToBeFetch > 0 {
				st.Stats.Album.ToBeFetch--
			}
			if failed {
				st.Stats.Album.CouldntFetch++
			}
		}
		if track.ReleaseYear == 0 {
			if st.Stats.Year.Fetching > 0 {
				st.Stats.Year.Fetching--
			}
			if st.Stats.Year.ToBeFetch > 0 {
				st.Stats.Year.ToBeFetch--
			}
			if failed {
				st.Stats.Year.CouldntFetch++
			}
		}
		if strings.TrimSpace(track.EmbedArt.String) == "" {
			if st.Stats.CoverArt.Fetching > 0 {
				st.Stats.CoverArt.Fetching--
			}
			if st.Stats.CoverArt.ToBeFetch > 0 {
				st.Stats.CoverArt.ToBeFetch--
			}
			if failed {
				st.Stats.CoverArt.CouldntFetch++
			}
		}
	})
}

func appendFetchedSpotifyData(item spotifyFetchedItem) {
	if strings.TrimSpace(item.AlbumName) == "" && item.Year == 0 && strings.TrimSpace(item.CoverURL) == "" {
		return
	}
	updateSpotifyJobState(func(st *spotifyMetadataJobState) {
		st.FetchedData = append(st.FetchedData, item)
		if len(st.FetchedData) > 50 {
			st.FetchedData = st.FetchedData[len(st.FetchedData)-50:]
		}
	})
}

func loadTracksMissingSpotifyMetadata(ctx context.Context, db *sql.DB, limit int) ([]spotifyMetadataTrack, error) {
	query := `
		SELECT mf.id, mf.title, mf.artist, mf.album, mf.release_year, mf.album_id, a.embed_art_path
		FROM media_file mf
		LEFT JOIN album a ON a.id = mf.album_id
		WHERE mf.missing = 0 AND (
			TRIM(COALESCE(mf.album, '')) = '' OR LOWER(TRIM(COALESCE(mf.album, ''))) = LOWER(?) OR
			COALESCE(mf.release_year, 0) = 0 OR
			TRIM(COALESCE(a.embed_art_path, '')) = ''
		)
		ORDER BY mf.updated_at ASC`

	args := []interface{}{consts.UnknownAlbum}
	if limit > 0 {
		query += `
		LIMIT ?`
		args = append(args, limit)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	capacity := limit
	if capacity <= 0 {
		capacity = 64
	}
	tracks := make([]spotifyMetadataTrack, 0, capacity)
	for rows.Next() {
		var t spotifyMetadataTrack
		if err := rows.Scan(&t.ID, &t.Title, &t.Artist, &t.Album, &t.ReleaseYear, &t.AlbumID, &t.EmbedArt); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

func processTrackSpotifyMetadata(ctx context.Context, db *sql.DB, track spotifyMetadataTrack) (string, error) {
	markTrackFetching(track)

	if strings.TrimSpace(track.Title) == "" || strings.TrimSpace(track.Artist) == "" {
		markTrackDone(track, true)
		return "skipped", nil
	}

	result, err := SearchSpotifyTrack(track.Title, track.Artist)
	if err != nil {
		markTrackDone(track, true)
		return "failed", err
	}
	if result == nil {
		markTrackDone(track, true)
		return "skipped", nil
	}

	if !spotifyMatchValid(track.Title, track.Artist, result) {
		markTrackDone(track, true)
		return "skipped", nil
	}

	appendFetchedSpotifyData(spotifyFetchedItem{
		AlbumName: result.AlbumName,
		Year:      result.Year,
		CoverURL:  result.CoverURL,
	})

	albumMissing := isUnknownAlbum(track.Album)
	yearMissing := track.ReleaseYear == 0
	coverMissing := strings.TrimSpace(track.EmbedArt.String) == ""

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		markTrackDone(track, true)
		return "failed", err
	}
	defer tx.Rollback()

	updatedAny := false
	failed := false

	if albumMissing {
		if strings.TrimSpace(result.AlbumName) != "" {
			if _, err := tx.ExecContext(ctx, `UPDATE media_file SET album = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, result.AlbumName, track.ID); err != nil {
				markTrackDone(track, true)
				return "failed", err
			}
			updatedAny = true
			updateSpotifyJobState(func(st *spotifyMetadataJobState) {
				st.Stats.Album.Fetched++
				st.Stats.Album.Updated++
			})
		} else {
			failed = true
		}
	}

	if yearMissing {
		if result.Year > 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE media_file SET release_year = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, result.Year, track.ID); err != nil {
				markTrackDone(track, true)
				return "failed", err
			}
			updatedAny = true
			updateSpotifyJobState(func(st *spotifyMetadataJobState) {
				st.Stats.Year.Fetched++
				st.Stats.Year.Updated++
			})
		} else {
			failed = true
		}
	}

	if coverMissing {
		if strings.TrimSpace(result.CoverURL) != "" && strings.TrimSpace(track.AlbumID) != "" {
			coverPath, err := downloadSpotifyCover(ctx, track.ID, result.CoverURL)
			if err != nil {
				markTrackDone(track, true)
				return "failed", err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE album SET embed_art_path = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, coverPath, track.AlbumID); err != nil {
				markTrackDone(track, true)
				return "failed", err
			}
			updatedAny = true
			updateSpotifyJobState(func(st *spotifyMetadataJobState) {
				st.Stats.CoverArt.Fetched++
				st.Stats.CoverArt.Updated++
			})
		} else {
			failed = true
		}
	}

	if err := tx.Commit(); err != nil {
		markTrackDone(track, true)
		return "failed", err
	}

	markTrackDone(track, failed)
	if updatedAny {
		log.Info(ctx, "Spotify metadata enrichment", "trackID", track.ID, "title", track.Title, "artist", track.Artist, "status", "updated")
		return "updated", nil
	}
	log.Info(ctx, "Spotify metadata enrichment", "trackID", track.ID, "title", track.Title, "artist", track.Artist, "status", "skipped")
	return "skipped", nil
}

func isUnknownAlbum(album string) bool {
	a := normalizeSpotifyString(album)
	return a == "" || a == normalizeSpotifyString(consts.UnknownAlbum)
}

func spotifyMatchValid(title string, artist string, result *SpotifyTrackResult) bool {
	expectedTitle := normalizeSpotifyString(title)
	actualTitle := normalizeSpotifyString(result.TrackName)
	if expectedTitle == "" || actualTitle == "" {
		return false
	}

	titleMatch := expectedTitle == actualTitle || levenshteinDistance(expectedTitle, actualTitle) < spotifyTrackTitleDistanceThreshold
	if !titleMatch {
		return false
	}

	expectedArtist := normalizeSpotifyString(artist)
	actualArtist := normalizeSpotifyString(result.Artist)
	if expectedArtist == "" || actualArtist == "" {
		return false
	}

	return strings.Contains(actualArtist, expectedArtist) || strings.Contains(expectedArtist, actualArtist)
}

func normalizeSpotifyString(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func levenshteinDistance(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	if len(ra) == 0 {
		return len(rb)
	}
	if len(rb) == 0 {
		return len(ra)
	}

	dp := make([]int, len(rb)+1)
	for j := range dp {
		dp[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		prev := dp[0]
		dp[0] = i
		for j := 1; j <= len(rb); j++ {
			temp := dp[j]
			cost := 0
			if ra[i-1] != rb[j-1] {
				cost = 1
			}
			dp[j] = min3(
				dp[j]+1,
				dp[j-1]+1,
				prev+cost,
			)
			prev = temp
		}
	}
	return dp[len(rb)]
}

func min3(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}

func SearchSpotifyTrack(title string, artist string) (*SpotifyTrackResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := spotifyReqLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	token, err := spotifyAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	result, err := searchSpotifyTrackWithToken(ctx, title, artist, token)
	if errors.Is(err, errSpotify401) {
		invalidateSpotifyCachedToken()
		fallbackToken, tokenErr := spotifyAccessToken(ctx)
		if tokenErr != nil {
			return nil, tokenErr
		}
		if fallbackToken != token {
			return searchSpotifyTrackWithToken(ctx, title, artist, fallbackToken)
		}
	}
	return result, err
}

func searchSpotifyTrackWithToken(ctx context.Context, title string, artist string, token string) (*SpotifyTrackResult, error) {

	params := url.Values{}
	params.Set("q", fmt.Sprintf(`track:"%s" artist:"%s"`, strings.TrimSpace(title), strings.TrimSpace(artist)))
	params.Set("type", "track")
	params.Set("limit", "1")

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.spotify.com/v1/search", nil)
	req.URL.RawQuery = params.Encode()
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleSpotifySearchStatus(resp); err != nil {
		return nil, err
	}

	var payload spotifySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if len(payload.Tracks.Items) == 0 {
		return nil, nil
	}
	item := payload.Tracks.Items[0]

	result := &SpotifyTrackResult{
		TrackName: item.Name,
		AlbumName: item.Album.Name,
	}
	if len(item.Artists) > 0 {
		result.Artist = item.Artists[0].Name
	}
	if len(item.Album.ReleaseDate) >= 4 {
		if year, err := strconv.Atoi(item.Album.ReleaseDate[:4]); err == nil {
			result.Year = year
		}
	}
	if len(item.Album.Images) > 0 {
		sort.Slice(item.Album.Images, func(i, j int) bool {
			return item.Album.Images[i].Width < item.Album.Images[j].Width
		})
		result.CoverURL = item.Album.Images[len(item.Album.Images)-1].URL
	}
	return result, nil
}

func handleSpotifySearchStatus(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return errSpotify401
	case http.StatusForbidden:
		return errors.New("spotify search forbidden (403)")
	case http.StatusTooManyRequests:
		return errors.New("spotify search rate limited (429)")
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("spotify search failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func spotifyAccessToken(ctx context.Context) (string, error) {
	configuredToken := strings.TrimSpace(conf.Server.Spotify.Token)
	if configuredToken != "" {
		return configuredToken, nil
	}

	spotifyTokenMu.Lock()
	if spotifyToken != "" && time.Now().Before(spotifyTokenExp.Add(-30*time.Second)) {
		tok := spotifyToken
		spotifyTokenMu.Unlock()
		return tok, nil
	}
	spotifyTokenMu.Unlock()

	id := strings.TrimSpace(conf.Server.Spotify.ID)
	secret := strings.TrimSpace(conf.Server.Spotify.Secret)
	if id == "" || secret == "" {
		return "", errors.New("spotify credentials are not configured")
	}

	payload := url.Values{}
	payload.Set("grant_type", "client_credentials")
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://accounts.spotify.com/api/token", strings.NewReader(payload.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(id+":"+secret)))

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("spotify auth failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tr spotifyTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	if strings.TrimSpace(tr.AccessToken) == "" {
		return "", errors.New("spotify auth succeeded but token is empty")
	}

	expires := tr.ExpiresIn
	if expires <= 0 {
		expires = 3600
	}

	spotifyTokenMu.Lock()
	spotifyToken = tr.AccessToken
	spotifyTokenExp = time.Now().Add(time.Duration(expires) * time.Second)
	tok := spotifyToken
	spotifyTokenMu.Unlock()

	return tok, nil
}

func invalidateSpotifyCachedToken() {
	spotifyTokenMu.Lock()
	spotifyToken = ""
	spotifyTokenExp = time.Time{}
	spotifyTokenMu.Unlock()
}

func downloadSpotifyCover(ctx context.Context, trackID string, coverURL string) (string, error) {
	if conf.Server.DataFolder == "" {
		return "", errors.New("data folder is not configured")
	}
	if err := spotifyReqLimiter.Wait(ctx); err != nil {
		return "", err
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, coverURL, nil)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("spotify cover download failed: status=%d", resp.StatusCode)
	}

	dir := filepath.Join(conf.Server.DataFolder, "spotify_coverart")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	coverPath := filepath.Join(dir, trackID+".jpg")
	f, err := os.Create(coverPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}
	return filepath.ToSlash(coverPath), nil
}
