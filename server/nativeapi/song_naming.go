package nativeapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/gcpsync"
	"github.com/navidrome/navidrome/log"
)

// These endpoints report library filenames that differ from artist/title
// metadata and support an explicit rename. Sync now derives names from tags
// automatically; the legacy enforcement setting remains for client compatibility.

// namingIssue is one track whose filename disagrees with its tags.
type namingIssue struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Artist       string `json:"artist"`
	CurrentName  string `json:"currentName"`
	ExpectedName string `json:"expectedName"`
	Path         string `json:"-"`
	LibraryPath  string `json:"-"`
}

// The scan compares every track's filename against its tags, which cannot be
// expressed as a SQL predicate, so the whole list is built in Go. It is cached
// briefly so opening the panel, paging through it and the badge refresh do not
// each re-read the table; a rename drops the cache immediately.
var namingCache = struct {
	mu       sync.Mutex
	issues   []namingIssue
	loadedAt time.Time
}{}

const namingCacheTTL = 30 * time.Second

func invalidateNamingCache() {
	namingCache.mu.Lock()
	namingCache.issues = nil
	namingCache.loadedAt = time.Time{}
	namingCache.mu.Unlock()
}

func (n *Router) addSongNamingRoute(r chi.Router) {
	r.Get("/notifications/naming-issues", n.namingIssues())
	r.Post("/song/naming/rename", n.renameToConvention())
	r.Get("/song/naming/settings", n.namingSettings())
	r.Put("/song/naming/settings", n.updateNamingSettings())
}

// loadNamingIssues scans the library for tracks whose filename does not match
// "Artist - Title.mp3".
func loadNamingIssues(ctx context.Context) ([]namingIssue, error) {
	namingCache.mu.Lock()
	if namingCache.issues != nil && time.Since(namingCache.loadedAt) < namingCacheTTL {
		cached := namingCache.issues
		namingCache.mu.Unlock()
		return cached, nil
	}
	namingCache.mu.Unlock()

	if conf.Server.DbPath == "" {
		return nil, nil
	}
	db, err := sql.Open("sqlite3", conf.Server.DbPath)
	if err != nil {
		return nil, fmt.Errorf("opening library database: %w", err)
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `
		SELECT media_file.id, media_file.path, media_file.artist, media_file.title, library.path
		FROM media_file
		JOIN library ON library.id = media_file.library_id
		WHERE media_file.missing = 0
		ORDER BY media_file.artist, media_file.title`)
	if err != nil {
		return nil, fmt.Errorf("querying tracks: %w", err)
	}
	defer rows.Close()

	issues := []namingIssue{}
	for rows.Next() {
		var id, path, libPath string
		var artist, title sql.NullString
		if err := rows.Scan(&id, &path, &artist, &title, &libPath); err != nil {
			log.Warn(ctx, "Unable to scan track while checking naming", err)
			continue
		}
		if gcpsync.Conforms(path, artist.String, title.String) {
			continue
		}
		expected := gcpsync.ExpectedName(artist.String, title.String)
		if expected == "" {
			continue
		}
		issues = append(issues, namingIssue{
			ID:           id,
			Title:        title.String,
			Artist:       artist.String,
			CurrentName:  filepath.Base(path),
			ExpectedName: expected,
			Path:         path,
			LibraryPath:  libPath,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating tracks: %w", err)
	}

	namingCache.mu.Lock()
	namingCache.issues = issues
	namingCache.loadedAt = time.Now()
	namingCache.mu.Unlock()
	return issues, nil
}

func (n *Router) namingIssues() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		issues, err := loadNamingIssues(ctx)
		if err != nil {
			log.Error(ctx, "Unable to list naming issues", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		limit := 200
		if v := r.URL.Query().Get("limit"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
				limit = min(parsed, 500)
			}
		}
		offset := 0
		if v := r.URL.Query().Get("offset"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
				offset = parsed
			}
		}

		page := []namingIssue{}
		if offset < len(issues) {
			page = issues[offset:min(offset+limit, len(issues))]
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", strconv.Itoa(len(issues)))
		if err := json.NewEncoder(w).Encode(page); err != nil {
			log.Error(ctx, "Unable to encode naming issues", err)
		}
	}
}

type renamePayload struct {
	IDs []string `json:"ids"`
}

type renameResult struct {
	ID    string `json:"id"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
	Error string `json:"error,omitempty"`
}

// renameToConvention renames the given tracks on disk to "Artist - Title.mp3"
// and points their library rows at the new path.
func (n *Router) renameToConvention() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var payload renamePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(payload.IDs) == 0 {
			http.Error(w, "ids are required", http.StatusBadRequest)
			return
		}

		repo := n.ds.MediaFile(ctx)
		results := make([]renameResult, 0, len(payload.IDs))
		renamed := 0

		for _, id := range payload.IDs {
			mf, err := repo.Get(id)
			if err != nil {
				results = append(results, renameResult{ID: id, Error: "track not found"})
				continue
			}
			expected := gcpsync.ExpectedName(mf.Artist, mf.Title)
			if expected == "" {
				results = append(results, renameResult{ID: id, Error: "track has no artist or title to name it after"})
				continue
			}
			oldName := filepath.Base(mf.Path)
			if strings.EqualFold(oldName, expected) {
				results = append(results, renameResult{ID: id, From: oldName, To: expected})
				continue
			}

			oldAbs := mf.AbsolutePath()
			newAbs := filepath.Join(filepath.Dir(oldAbs), expected)

			// Never rename over another track. The two files are different
			// songs that happen to share tags, and silently replacing one with
			// the other would destroy audio the library still has a row for.
			if _, err := os.Stat(newAbs); err == nil && !strings.EqualFold(oldAbs, newAbs) {
				results = append(results, renameResult{
					ID: id, From: oldName, To: expected,
					Error: "a file with that name already exists in the same folder",
				})
				continue
			}

			if err := os.Rename(oldAbs, newAbs); err != nil {
				log.Warn(ctx, "Could not rename track", "id", id, "from", oldAbs, "to", newAbs, err)
				results = append(results, renameResult{ID: id, From: oldName, To: expected, Error: err.Error()})
				continue
			}

			newPath := filepath.Join(filepath.Dir(mf.Path), expected)
			if err := repo.UpdatePath(id, newPath); err != nil {
				// Put the file back rather than leave disk and database
				// disagreeing - the scanner would report a missing track and a
				// duplicate for the same song.
				if rollbackErr := os.Rename(newAbs, oldAbs); rollbackErr != nil {
					log.Error(ctx, "Renamed file but could not update the library row, and could not roll back",
						"id", id, "path", newAbs, "err", err, "rollbackErr", rollbackErr)
				}
				results = append(results, renameResult{ID: id, From: oldName, To: expected, Error: err.Error()})
				continue
			}

			renameStagedCopy(ctx, mf.LibraryPath, oldAbs, newAbs)

			log.Info(ctx, "Renamed track to naming convention", "id", id, "from", oldName, "to", expected)
			results = append(results, renameResult{ID: id, From: oldName, To: expected})
			renamed++
		}

		if renamed > 0 {
			invalidateNamingCache()
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"renamed": renamed, "results": results})
	}
}

// renameStagedCopy renames a copy of the track waiting in the sync staging
// folder, if there is one. A track held back by the naming gate has a staged
// copy under the old name; leaving it there would keep it blocked forever and
// leak a file into the sync folder.
func renameStagedCopy(ctx context.Context, libraryRoot, oldAbs, newAbs string) {
	staging := gcpsync.StagingPath(libraryRoot, oldAbs)
	if staging == "" {
		return
	}
	if _, err := os.Stat(staging); err != nil {
		return
	}
	dest := gcpsync.StagingPath(libraryRoot, newAbs)
	if dest == "" {
		return
	}
	if err := gcpsync.RenameStagedCopy(staging, dest); err != nil {
		log.Warn(ctx, "Could not rename staged copy after track rename", "from", staging, "to", dest, err)
		return
	}
	log.Info(ctx, "Renamed staged copy to match", "from", staging, "to", dest)
}

func (n *Router) namingSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"enforced": gcpsync.NamingEnforced(r.Context()),
		})
	}
}

func (n *Router) updateNamingSettings() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var payload struct {
			Enforced *bool `json:"enforced"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if payload.Enforced == nil {
			http.Error(w, "enforced is required", http.StatusBadRequest)
			return
		}
		if err := gcpsync.SetNamingEnforced(ctx, n.ds, *payload.Enforced); err != nil {
			log.Error(ctx, "Could not save naming enforcement setting", "enforced", *payload.Enforced, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Warn(ctx, "Naming enforcement changed", "enforced", *payload.Enforced)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"enforced": *payload.Enforced})
	}
}
