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

type missingPlaylistNotification struct {
	PlaylistID   string               `json:"playlist_id"`
	PlaylistName string               `json:"playlist_name,omitempty"`
	MissingCount int                  `json:"missing_count"`
	Tracks       []missingTrackDetail `json:"tracks"`
}

type missingTrackDetail struct {
	TrackPath       string  `json:"track_path"`
	CreatedAt       string  `json:"created_at"`
	Title           string  `json:"title,omitempty"`
	Artist          string  `json:"artist,omitempty"`
	DurationSeconds float64 `json:"duration_seconds,omitempty"`
}

func (n *Router) addNotificationsRoute(r chi.Router) {
	r.Get("/notifications/missing-tracks", n.handleMissingTrackNotifications())
}

func (n *Router) handleMissingTrackNotifications() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		entries := []missingPlaylistNotification{}

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

		rows, err := db.QueryContext(ctx, `WITH recent AS (
        SELECT playlist_id, track_path, created_at
        FROM missing_playlist_tracks
        ORDER BY created_at DESC
        LIMIT 200
)
SELECT
        playlist_id,
        COUNT(*) AS missing_count,
        MAX(created_at) AS latest_created_at,
        json_group_array(json_object('track_path', track_path, 'created_at', created_at)) AS tracks_json
FROM recent
GROUP BY playlist_id
ORDER BY latest_created_at DESC
LIMIT 50`)
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

		entries = make([]missingPlaylistNotification, 0)

		for rows.Next() {
			var (
				playlistID      string
				missingCount    int
				latestCreatedAt string
				tracksJSON      sql.NullString
				playlistEntries []missingTrackDetail
			)

			if err := rows.Scan(&playlistID, &missingCount, &latestCreatedAt, &tracksJSON); err != nil {
				log.Warn(ctx, "Unable to scan missing track notification", "path", dbFile, "err", err)
				continue
			}

			if tracksJSON.Valid {
				if err := json.Unmarshal([]byte(tracksJSON.String), &playlistEntries); err != nil {
					log.Warn(ctx, "Unable to parse missing track list", "path", dbFile, "playlist", playlistID, "err", err)
				}
			}

			entries = append(entries, missingPlaylistNotification{
				PlaylistID:   playlistID,
				PlaylistName: derivePlaylistName(playlistID),
				MissingCount: missingCount,
				Tracks:       playlistEntries,
			})
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

func writeMissingTrackResponse(w http.ResponseWriter, entries []missingPlaylistNotification, ctx context.Context) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(entries); err != nil {
		log.Error(ctx, "Unable to encode missing track notifications", err)
	}
}

func derivePlaylistName(playlistID string) string {
	if playlistID == "" {
		return ""
	}

	base := filepath.Base(playlistID)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return base
}

func enrichMissingTrackMetadata(ctx context.Context, entries []missingPlaylistNotification) {
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
		for j := range entries[i].Tracks {
			populateTrackMetadata(ctx, mainDB, &entries[i].Tracks[j])
		}
	}
}

func populateTrackMetadata(ctx context.Context, db *sql.DB, track *missingTrackDetail) {
	if db == nil || track == nil || track.TrackPath == "" {
		return
	}

	var (
		title    sql.NullString
		artist   sql.NullString
		duration sql.NullFloat64
	)

	err := db.QueryRowContext(ctx, `SELECT title, artist, duration FROM media_file WHERE path = ? LIMIT 1`, track.TrackPath).Scan(&title, &artist, &duration)
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
	if duration.Valid {
		track.DurationSeconds = duration.Float64
	}
}
