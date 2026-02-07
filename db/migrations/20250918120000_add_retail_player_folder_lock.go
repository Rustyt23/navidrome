package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddRetailPlayerFolderLock, downAddRetailPlayerFolderLock)
}

func upAddRetailPlayerFolderLock(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
        ALTER TABLE retail_player_folder
        ADD COLUMN is_locked BOOLEAN NOT NULL DEFAULT 0;
    `)
	return err
}

func downAddRetailPlayerFolderLock(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
        ALTER TABLE retail_player_folder
        DROP COLUMN is_locked;
    `)
	return err
}
