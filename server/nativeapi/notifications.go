package nativeapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/log"
)

type missingTrackNotification struct {
	Title     string   `json:"title"`
	Artist    string   `json:"artist"`
	TrackPath string   `json:"-"`
	Playlists []string `json:"playlists,omitempty"`
	Folders   []string `json:"folders,omitempty"`
}

func (n *Router) addNotificationsRoute(r chi.Router) {
	r.Get("/notifications/missing-tracks", n.handleMissingTrackNotifications())
}

func (n *Router) handleMissingTrackNotifications() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		entries := []missingTrackNotification{}

		dbPath := conf.Server.DbPath
		if dbPath == "" {
			if conf.Server.DataFolder == "" {
				writeMissingTrackResponse(w, entries, ctx)
				return
			}
			dbPath = filepath.Join(conf.Server.DataFolder, consts.DefaultDbPath)
		}

		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			log.Warn(ctx, "Unable to open database for missing tracks", "path", dbPath, "err", err)
			writeMissingTrackResponse(w, entries, ctx)
			return
		}
		defer db.Close()

		rows, err := db.QueryContext(ctx, `SELECT track_path, playlists, playlist_paths FROM missing_playlist_tracks ORDER BY time_added DESC LIMIT 200`)
		if err != nil {
			if strings.Contains(err.Error(), "no such table") {
				writeMissingTrackResponse(w, entries, ctx)
				return
			}
			log.Error(ctx, "Unable to query missing tracks notifications", "path", dbPath, "err", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		entries = make([]missingTrackNotification, 0)

		for rows.Next() {
			var (
				trackPath         string
				playlistsJSON     sql.NullString
				playlistPathsJSON sql.NullString
			)

			if err := rows.Scan(&trackPath, &playlistsJSON, &playlistPathsJSON); err != nil {
				log.Warn(ctx, "Unable to scan missing track notification", "path", dbPath, "err", err)
				continue
			}

			playlists := decodeNotificationArray(playlistsJSON.String)
			playlistPaths := decodeNotificationArray(playlistPathsJSON.String)

			entries = append(entries, missingTrackNotification{
				Title:     deriveTrackName(trackPath),
				TrackPath: trackPath,
				Playlists: playlists,
				Folders:   derivePlaylistFolders(playlistPaths),
			})
		}

		if err := rows.Err(); err != nil {
			log.Warn(ctx, "Error iterating missing track notifications", "path", dbPath, "err", err)
		}

		if len(entries) > 0 {
			enrichMissingTrackMetadata(ctx, entries)
			for i := range entries {
				if entries[i].Title == "" {
					entries[i].Title = deriveTrackName(entries[i].TrackPath)
				}
			}
		}

		writeMissingTrackResponse(w, entries, ctx)
	}
}

func writeMissingTrackResponse(w http.ResponseWriter, entries []missingTrackNotification, ctx context.Context) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(entries); err != nil {
		log.Error(ctx, "Unable to encode missing track notifications", err)
	}
}

func deriveTrackName(trackPath string) string {
	if trackPath == "" {
		return ""
	}

	base := filepath.Base(trackPath)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return base
}

func enrichMissingTrackMetadata(ctx context.Context, entries []missingTrackNotification) {
	if conf.Server.DbPath == "" {
		return
	}

	mainDB, err := sql.Open("sqlite3", conf.Server.DbPath)
	if err != nil {
		log.Warn(ctx, "Unable to open main database for missing track metadata", "path", conf.Server.DbPath, "err", err)
		return
	}
	defer mainDB.Close()

	for i := range entries {
		populateTrackMetadata(ctx, mainDB, &entries[i])
	}
}

func populateTrackMetadata(ctx context.Context, db *sql.DB, track *missingTrackNotification) {
	if db == nil || track == nil || track.TrackPath == "" {
		return
	}

	var (
		title  sql.NullString
		artist sql.NullString
	)

	err := db.QueryRowContext(ctx, `SELECT title, artist FROM media_file WHERE path = ? LIMIT 1`, track.TrackPath).Scan(&title, &artist)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Debug(ctx, "Unable to lookup track metadata", "track", track.TrackPath, "err", err)
		}
		return
	}

	if title.Valid {
		track.Title = title.String
	}
	if artist.Valid {
		track.Artist = artist.String
	}
}

func decodeNotificationArray(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return values
}

func derivePlaylistFolders(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(paths))
	folders := make([]string, 0, len(paths))
	for _, p := range paths {
		folder := displayFolderFromPlaylistPath(p)
		if folder == "" {
			continue
		}
		if _, ok := seen[folder]; ok {
			continue
		}
		seen[folder] = struct{}{}
		folders = append(folders, folder)
	}
	sort.Strings(folders)
	return folders
}

func displayFolderFromPlaylistPath(path string) string {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" {
		return ""
	}

	cleaned = filepath.Clean(cleaned)
	dir := filepath.Dir(cleaned)
	if dir == "." || dir == "" {
		return ""
	}

	dir = filepath.Clean(dir)

	if conf.Server.PlaylistsPath != "" {
		for _, root := range strings.Split(conf.Server.PlaylistsPath, string(filepath.ListSeparator)) {
			root = strings.TrimSpace(root)
			if root == "" {
				continue
			}
			root = strings.TrimSuffix(root, "**")
			root = strings.TrimSuffix(root, string(os.PathSeparator))
			absRoot, err := filepath.Abs(root)
			if err != nil {
				continue
			}
			rel, err := filepath.Rel(absRoot, dir)
			if err == nil && !strings.HasPrefix(rel, "..") {
				rel = filepath.ToSlash(rel)
				if rel == "." {
					return filepath.Base(dir)
				}
				return rel
			}
		}
	}

	base := filepath.Base(dir)
	if base == "." || base == string(os.PathSeparator) || base == "" {
		return filepath.ToSlash(dir)
	}
	return base
}
