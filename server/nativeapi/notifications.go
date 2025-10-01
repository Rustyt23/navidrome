package nativeapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
)

type missingTrackDetail struct {
	TrackPath string `json:"-"`
	Title     string `json:"title,omitempty"`
	Artist    string `json:"artist,omitempty"`
}

func (n *Router) addNotificationsRoute(r chi.Router) {
	r.Get("/notifications/missing-tracks", n.handleMissingTrackNotifications())
}

func (n *Router) handleMissingTrackNotifications() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		entries := []missingTrackDetail{}

		if conf.Server.DataFolder == "" {
			writeMissingTrackResponse(w, entries, ctx)
			return
		}

		dbFile := filepath.Join(conf.Server.DataFolder, "missing_tracks.db")
		dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_journal_mode=WAL", filepath.ToSlash(dbFile))

		db, err := sql.Open("sqlite3", dsn)
		if err != nil {
			log.Warn(ctx, "Unable to open missing tracks database", "path", dbFile, "err", err)
			writeMissingTrackResponse(w, entries, ctx)
			return
		}
		defer db.Close()

		if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
			log.Debug(ctx, "Unable to enable WAL for missing tracks database", "path", dbFile, "err", err)
		}

		rows, err := db.QueryContext(ctx, `SELECT track_path FROM missing_playlist_tracks ORDER BY created_at DESC LIMIT 200`)
		if err != nil {
			if strings.Contains(err.Error(), "no such table") {
				writeMissingTrackResponse(w, entries, ctx)
				return
			}
			log.Error(ctx, "Unable to query missing tracks notifications", "path", dbFile, "err", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		entries = make([]missingTrackDetail, 0)

		for rows.Next() {
			var trackPath sql.NullString

			if err := rows.Scan(&trackPath); err != nil {
				log.Warn(ctx, "Unable to scan missing track entry", "path", dbFile, "err", err)
				continue
			}

			if !trackPath.Valid || trackPath.String == "" {
				continue
			}

			entries = append(entries, missingTrackDetail{TrackPath: trackPath.String})
		}

		if err := rows.Err(); err != nil {
			log.Warn(ctx, "Error iterating missing track notifications", "path", dbFile, "err", err)
		}

		if len(entries) > 0 {
			enrichMissingTrackMetadata(ctx, entries)
		}

		writeMissingTrackResponse(w, entries, ctx)
	}
}

func writeMissingTrackResponse(w http.ResponseWriter, entries []missingTrackDetail, ctx context.Context) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(entries); err != nil {
		log.Error(ctx, "Unable to encode missing track notifications", err)
	}
}

func enrichMissingTrackMetadata(ctx context.Context, entries []missingTrackDetail) {
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

func populateTrackMetadata(ctx context.Context, db *sql.DB, track *missingTrackDetail) {
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
	} else {
		if title.Valid && title.String != "" {
			track.Title = title.String
		}
		if artist.Valid && artist.String != "" {
			track.Artist = artist.String
		}
	}

	if track.Title == "" {
		base := filepath.Base(track.TrackPath)
		if ext := filepath.Ext(base); ext != "" {
			base = strings.TrimSuffix(base, ext)
		}
		if base != "" {
			track.Title = base
		}
	}
}
