package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upCreateRetailPlayerDeviceChannelCache, downCreateRetailPlayerDeviceChannelCache)
}

func upCreateRetailPlayerDeviceChannelCache(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS retail_player_device_channel_cache (
			device_id        TEXT NOT NULL PRIMARY KEY,
			channel_name     TEXT,
			channel_list_name TEXT,
			org_unit         TEXT,
			updated_at       DATETIME NOT NULL DEFAULT (datetime('now'))
		);
	`)
	return err
}

func downCreateRetailPlayerDeviceChannelCache(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS retail_player_device_channel_cache;
	`)
	return err
}
