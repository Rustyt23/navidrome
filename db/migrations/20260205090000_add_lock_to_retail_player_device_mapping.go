package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddLockToRetailPlayerDeviceMapping, downAddLockToRetailPlayerDeviceMapping)
}

func upAddLockToRetailPlayerDeviceMapping(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		ALTER TABLE retail_player_device_mapping
		ADD COLUMN is_locked BOOLEAN NOT NULL DEFAULT FALSE;
	`)
	return err
}

func downAddLockToRetailPlayerDeviceMapping(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		ALTER TABLE retail_player_device_mapping
		DROP COLUMN is_locked;
	`)
	return err
}
