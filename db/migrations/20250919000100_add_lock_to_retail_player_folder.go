package migrations

import (
	"context"
	"database/sql"
	"strings"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddRetailPlayerFolderLock, downAddRetailPlayerFolderLock)
}

func upAddRetailPlayerFolderLock(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
ALTER TABLE retail_player_folder
ADD COLUMN is_locked boolean NOT NULL DEFAULT false;
`)
	if err != nil {
		lowerErr := strings.ToLower(err.Error())
		if strings.Contains(lowerErr, "duplicate column name") || strings.Contains(lowerErr, "already exists") {
			return nil
		}
	}
	return err
}

func downAddRetailPlayerFolderLock(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
ALTER TABLE retail_player_folder
DROP COLUMN is_locked;
`)
	if err != nil {
		lowerErr := strings.ToLower(err.Error())
		if strings.Contains(lowerErr, "no such column") || strings.Contains(lowerErr, "does not exist") || strings.Contains(lowerErr, "syntax error") {
			return nil
		}
	}
	return err
}
