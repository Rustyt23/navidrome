package nativeapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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

		if conf.Server.DataFolder == "" {
			writeMissingTrackResponse(w, entries, total, ctx)
			return
		}

		dbFile := filepath.Join(conf.Server.DataFolder, "missing_tracks.db")
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

		err = db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT track_path) FROM missing_playlist_tracks`).Scan(&total)
		if err != nil {
			if strings.Contains(err.Error(), "no such table") {
				writeMissingTrackResponse(w, entries, total, ctx)
				return
			}
			log.Warn(ctx, "Unable to count missing tracks notifications", "path", dbFile, "err", err)
			total = 0
		}

		rows, err := db.QueryContext(ctx, `SELECT track_path FROM missing_playlist_tracks GROUP BY track_path ORDER BY MAX(created_at) DESC LIMIT ? OFFSET ?`, limit, offset)
		if err != nil {
			if strings.Contains(err.Error(), "no such table") {
				writeMissingTrackResponse(w, entries, total, ctx)
				return
			}
			log.Error(ctx, "Unable to query missing tracks notifications", "path", dbFile, "err", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		entries = make([]missingTrackNotification, 0)

		for rows.Next() {
			var trackPath string

			if err := rows.Scan(&trackPath); err != nil {
				log.Warn(ctx, "Unable to scan missing track notification", "path", dbFile, "err", err)
				continue
			}

			entries = append(entries, missingTrackNotification{
				Title:     deriveTrackName(trackPath),
				TrackPath: trackPath,
			})
		}

		if err := rows.Err(); err != nil {
			log.Warn(ctx, "Error iterating missing track notifications", "path", dbFile, "err", err)
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
