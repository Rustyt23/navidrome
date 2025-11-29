package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddRemoteControlIDToRetailPlayerDeviceMapping, downAddRemoteControlIDToRetailPlayerDeviceMapping)
}

func upAddRemoteControlIDToRetailPlayerDeviceMapping(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
        ALTER TABLE retail_player_device_mapping
        ADD COLUMN IF NOT EXISTS remote_control_id TEXT DEFAULT '';
    `)
	return err
}

func downAddRemoteControlIDToRetailPlayerDeviceMapping(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
        ALTER TABLE retail_player_device_mapping
        DROP COLUMN IF EXISTS remote_control_id;
    `)
	return err
}
