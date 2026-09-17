package nativeapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
)

type missingTrackNotification struct {
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	TrackPath string `json:"-"`
}

func (n *Router) addNotificationsRoute(r chi.Router) {
	r.Get("/notifications/missing-tracks", n.handleMissingTrackNotifications())
}

func (n *Router) handleMissingTrackNotifications() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		entries := []missingTrackNotification{}
		total := 0

		limit := 200
		if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
			if parsed, err := strconv.Atoi(limitParam); err == nil && parsed > 0 {
				if parsed > 500 {
					parsed = 500
				}
				limit = parsed
			}
		}

		offset := 0
		if offsetParam := r.URL.Query().Get("offset"); offsetParam != "" {
			if parsed, err := strconv.Atoi(offsetParam); err == nil && parsed >= 0 {
				offset = parsed
			}
		}

		if conf.Server.DataFolder.String() == "" {
			writeMissingTrackResponse(w, entries, total, ctx)
			return
		}

		dbFile := filepath.Join(conf.Server.DataFolder.String(), "missing_tracks.db")
		dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_journal_mode=WAL", filepath.ToSlash(dbFile))

		db, err := sql.Open("sqlite3", dsn)
		if err != nil {
			log.Warn(ctx, "Unable to open missing tracks database", "path", dbFile, "err", err)
			writeMissingTrackResponse(w, entries, total, ctx)
			return
		}
		defer db.Close()

		if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
			log.Debug(ctx, "Unable to enable WAL for missing tracks database", "path", dbFile, "err", err)
		}

		// Load distinct recorded paths (most recent first), bounded so a runaway
		// table can't unbound this request. Reconciliation and de-duplication then
		// happen in Go, which keeps the badge count honest.
		rows, err := db.QueryContext(ctx, `SELECT track_path FROM missing_playlist_tracks GROUP BY track_path ORDER BY MAX(created_at) DESC LIMIT ?`, missingTrackScanCap)
		if err != nil {
			if strings.Contains(err.Error(), "no such table") {
				writeMissingTrackResponse(w, entries, total, ctx)
				return
			}
			log.Error(ctx, "Unable to query missing tracks notifications", "path", dbFile, "err", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		scanned := make([]string, 0, 256)
		for rows.Next() {
			var trackPath string
			if err := rows.Scan(&trackPath); err != nil {
				log.Warn(ctx, "Unable to scan missing track notification", "path", dbFile, "err", err)
				continue
			}
			scanned = append(scanned, trackPath)
		}
		if err := rows.Err(); err != nil {
			log.Warn(ctx, "Error iterating missing track notifications", "path", dbFile, "err", err)
		}
		rows.Close()

		// Reconcile against the library and collapse different spellings of the
		// same file: a track is only still "missing" if no library file shares its
		// base name. This makes added songs disappear and stops the same song being
		// counted twice when two playlists reference it via different paths.
		reconciled := reconcileMissingTracks(ctx, scanned)
		total = len(reconciled)

		if offset >= len(reconciled) {
			reconciled = nil
		} else {
			end := offset + limit
			if end > len(reconciled) {
				end = len(reconciled)
			}
			reconciled = reconciled[offset:end]
		}

		entries = make([]missingTrackNotification, 0, len(reconciled))
		for _, trackPath := range reconciled {
			entries = append(entries, missingTrackNotification{
				Title:     deriveTrackName(trackPath),
				TrackPath: trackPath,
			})
		}

		if len(entries) > 0 {
			enrichMissingTrackMetadata(ctx, entries)
			for i := range entries {
				if entries[i].Title == "" {
					entries[i].Title = deriveTrackName(entries[i].TrackPath)
				}
			}
		}

		writeMissingTrackResponse(w, entries, total, ctx)
	}
}

// missingTrackScanCap bounds how many recorded paths are loaded for
// reconciliation so the notifications endpoint stays cheap even if the side
// table has grown large.
const missingTrackScanCap = 5000

// reconcileMissingTracks drops recorded paths whose base name now exists in the
// library and de-duplicates the remainder by base name, preserving the input
// order (most-recent first). The base name is the reliable identity here because
// the playlist importer itself falls back to base-name matching when a path does
// not resolve directly.
func reconcileMissingTracks(ctx context.Context, trackPaths []string) []string {
	var mainDB *sql.DB
	if conf.Server.DbPath != "" {
		if db, err := sql.Open("sqlite3", conf.Server.DbPath); err == nil {
			mainDB = db
			defer mainDB.Close()
		} else {
			log.Warn(ctx, "Unable to open main database for missing track reconciliation", "path", conf.Server.DbPath, "err", err)
		}
	}

	result := make([]string, 0, len(trackPaths))
	seen := make(map[string]struct{}, len(trackPaths))
	for _, trackPath := range trackPaths {
		base := strings.ToLower(deriveTrackBase(trackPath))
		if base == "" {
			continue
		}
		if _, ok := seen[base]; ok {
			continue // same file already accounted for under another spelling
		}
		seen[base] = struct{}{}
		if libraryHasTrack(ctx, mainDB, trackPath) {
			continue // the file now exists in the library; no longer missing
		}
		result = append(result, trackPath)
	}
	return result
}

// deriveTrackBase returns the file base name (with extension) of a recorded
// playlist path, tolerating both separators.
func deriveTrackBase(trackPath string) string {
	trackPath = strings.TrimSpace(trackPath)
	if trackPath == "" {
		return ""
	}
	trackPath = strings.ReplaceAll(trackPath, "\\", "/")
	return path.Base(trackPath)
}

// libraryHasTrack reports whether the library contains a media file matching the
// recorded path either exactly or by base name (media_file.path is stored
// relative to the library, so it ends with the base name).
func libraryHasTrack(ctx context.Context, db *sql.DB, trackPath string) bool {
	if db == nil {
		return false
	}
	base := deriveTrackBase(trackPath)
	if base == "" {
		return false
	}
	// Escape LIKE wildcards in the base name so titles containing % or _ don't
	// match unrelated files.
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(base)
	var found int
	err := db.QueryRowContext(ctx,
		`SELECT 1 FROM media_file WHERE path = ? OR path = ? OR path LIKE ? ESCAPE '\' LIMIT 1`,
		trackPath, base, "%/"+escaped,
	).Scan(&found)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) && !strings.Contains(err.Error(), "no such table") {
			log.Debug(ctx, "Unable to reconcile missing track against library", "track", trackPath, "err", err)
		}
		return false
	}
	return true
}

func writeMissingTrackResponse(w http.ResponseWriter, entries []missingTrackNotification, total int, ctx context.Context) {
	w.Header().Set("Content-Type", "application/json")
	if total >= 0 {
		w.Header().Set("X-Total-Count", strconv.Itoa(total))
	}
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
