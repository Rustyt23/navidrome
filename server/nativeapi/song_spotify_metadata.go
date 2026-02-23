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
	spotifyBatchDefaultLimit           = 50
)

var (
	spotifyReqLimiter = rate.NewLimiter(rate.Limit(5), 1)
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

func (n *Router) addSpotifyMetadataRoute(r chi.Router) {
	r.With(adminOnlyMiddleware).Post("/song/metadata/spotify", n.fetchMissingSpotifyMetadata())
}

func (n *Router) fetchMissingSpotifyMetadata() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		limit := spotifyBatchDefaultLimit
		if p := strings.TrimSpace(r.URL.Query().Get("limit")); p != "" {
			if v, err := strconv.Atoi(p); err == nil && v > 0 && v <= 200 {
				limit = v
			}
		}

		if conf.Server.DbPath == "" {
			http.Error(w, "database path not configured", http.StatusInternalServerError)
			return
		}

		db, err := sql.Open("sqlite3", conf.Server.DbPath)
		if err != nil {
			log.Error(ctx, "Unable to open database for Spotify metadata enrichment", "path", conf.Server.DbPath, "err", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer db.Close()

		tracks, err := loadTracksMissingSpotifyMetadata(ctx, db, limit)
		if err != nil {
			log.Error(ctx, "Unable to load tracks for Spotify metadata enrichment", "err", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		resp := spotifyMetadataResponse{}
		for _, track := range tracks {
			resp.Processed++
			updatedFields, status, err := processTrackSpotifyMetadata(ctx, db, track)
			if err != nil {
				resp.Failed++
				log.Error(ctx, "Spotify metadata enrichment failed", "trackID", track.ID, "title", track.Title, "artist", track.Artist, "updatedFields", strings.Join(updatedFields, ","), "status", status, "err", err)
				continue
			}
			switch status {
			case "updated":
				resp.Updated++
			case "skipped":
				resp.Skipped++
			default:
				resp.Failed++
			}
			log.Info(ctx, "Spotify metadata enrichment", "trackID", track.ID, "title", track.Title, "artist", track.Artist, "updatedFields", strings.Join(updatedFields, ","), "status", status)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func loadTracksMissingSpotifyMetadata(ctx context.Context, db *sql.DB, limit int) ([]spotifyMetadataTrack, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT mf.id, mf.title, mf.artist, mf.album, mf.release_year, mf.album_id, a.embed_art_path
		FROM media_file mf
		LEFT JOIN album a ON a.id = mf.album_id
		WHERE mf.missing = 0 AND (
			TRIM(COALESCE(mf.album, '')) = '' OR LOWER(TRIM(COALESCE(mf.album, ''))) = LOWER(?) OR
			COALESCE(mf.release_year, 0) = 0 OR
			TRIM(COALESCE(a.embed_art_path, '')) = ''
		)
		ORDER BY mf.updated_at ASC
		LIMIT ?
	`, consts.UnknownAlbum, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tracks := make([]spotifyMetadataTrack, 0, limit)
	for rows.Next() {
		var t spotifyMetadataTrack
		if err := rows.Scan(&t.ID, &t.Title, &t.Artist, &t.Album, &t.ReleaseYear, &t.AlbumID, &t.EmbedArt); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

func processTrackSpotifyMetadata(ctx context.Context, db *sql.DB, track spotifyMetadataTrack) ([]string, string, error) {
	if strings.TrimSpace(track.Title) == "" || strings.TrimSpace(track.Artist) == "" {
		return nil, "skipped", nil
	}

	result, err := SearchSpotifyTrack(track.Title, track.Artist)
	if err != nil {
		return nil, "failed", err
	}
	if result == nil {
		return nil, "skipped", nil
	}

	if !spotifyMatchValid(track.Title, track.Artist, result) {
		return nil, "skipped", nil
	}

	updatedFields := make([]string, 0, 3)
	albumMissing := isUnknownAlbum(track.Album)
	yearMissing := track.ReleaseYear == 0
	coverMissing := strings.TrimSpace(track.EmbedArt.String) == ""

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return updatedFields, "failed", err
	}
	defer tx.Rollback()

	if albumMissing && strings.TrimSpace(result.AlbumName) != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE media_file SET album = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, result.AlbumName, track.ID); err != nil {
			return updatedFields, "failed", err
		}
		updatedFields = append(updatedFields, "album")
	}

	if yearMissing && result.Year > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE media_file SET release_year = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, result.Year, track.ID); err != nil {
			return updatedFields, "failed", err
		}
		updatedFields = append(updatedFields, "release_year")
	}

	if coverMissing && strings.TrimSpace(result.CoverURL) != "" && strings.TrimSpace(track.AlbumID) != "" {
		coverPath, err := downloadSpotifyCover(ctx, track.ID, result.CoverURL)
		if err != nil {
			return updatedFields, "failed", err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE album SET embed_art_path = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, coverPath, track.AlbumID); err != nil {
			return updatedFields, "failed", err
		}
		updatedFields = append(updatedFields, "cover_art_path")
	}

	if len(updatedFields) == 0 {
		return updatedFields, "skipped", tx.Commit()
	}
	if err := tx.Commit(); err != nil {
		return updatedFields, "failed", err
	}
	return updatedFields, "updated", nil
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
