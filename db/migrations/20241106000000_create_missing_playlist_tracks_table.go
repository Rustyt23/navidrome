package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upCreateMissingPlaylistTracksTable, downCreateMissingPlaylistTracksTable)
}

func upCreateMissingPlaylistTracksTable(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS missing_playlist_tracks (
    song_name TEXT PRIMARY KEY,
    track_path TEXT,
    playlist TEXT,
    time_added TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`)
	return err
}

func downCreateMissingPlaylistTracksTable(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS missing_playlist_tracks;`)
	return err
}
