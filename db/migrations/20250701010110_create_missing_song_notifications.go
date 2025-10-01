package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upCreateMissingSongNotifications, downCreateMissingSongNotifications)
}

func upCreateMissingSongNotifications(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS missing_song_notification (
            media_file_id TEXT PRIMARY KEY REFERENCES media_file(id) ON DELETE CASCADE,
            playlist_names TEXT NOT NULL DEFAULT '[]',
            detected_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        );
        CREATE INDEX IF NOT EXISTS missing_song_notification_detected_at_idx
            ON missing_song_notification(detected_at DESC);
    `)
	return err
}

func downCreateMissingSongNotifications(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
        DROP TABLE IF EXISTS missing_song_notification;
    `)
	return err
}
