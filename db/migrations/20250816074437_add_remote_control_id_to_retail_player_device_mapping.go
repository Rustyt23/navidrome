package migrations

import (
	"context"
	"database/sql"
	"strings"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddRemoteControlIDToRetailPlayerDeviceMapping, downAddRemoteControlIDToRetailPlayerDeviceMapping)
}

func upAddRemoteControlIDToRetailPlayerDeviceMapping(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
ALTER TABLE retail_player_device_mapping
ADD COLUMN remote_control_id TEXT DEFAULT '';
`)
	if err != nil {
		lowerErr := strings.ToLower(err.Error())
		if strings.Contains(lowerErr, "duplicate column name") || strings.Contains(lowerErr, "already exists") {
			return nil
		}
	}
	return err
}

func downAddRemoteControlIDToRetailPlayerDeviceMapping(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
ALTER TABLE retail_player_device_mapping
DROP COLUMN remote_control_id;
`)
	if err != nil {
		lowerErr := strings.ToLower(err.Error())
		if strings.Contains(lowerErr, "no such column") || strings.Contains(lowerErr, "does not exist") || strings.Contains(lowerErr, "syntax error") {
			return nil
		}
	}
	return err
}
