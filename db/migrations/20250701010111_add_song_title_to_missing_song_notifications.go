package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddSongTitleToMissingSongNotifications, downAddSongTitleToMissingSongNotifications)
}

func upAddSongTitleToMissingSongNotifications(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
        ALTER TABLE missing_song_notification
            ADD COLUMN song_title TEXT NOT NULL DEFAULT '';
    `)
	return err
}

func downAddSongTitleToMissingSongNotifications(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
        ALTER TABLE missing_song_notification
            DROP COLUMN IF EXISTS song_title;
    `)
	return err
}
