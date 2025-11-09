package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upCreateRetailPlayerDeviceMapping, downCreateRetailPlayerDeviceMapping)
}

func upCreateRetailPlayerDeviceMapping(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
                CREATE TABLE IF NOT EXISTS retail_player_device_mapping (
                        device_id    TEXT NOT NULL PRIMARY KEY,
                        device_name  TEXT NOT NULL,
                        device_slug  TEXT NOT NULL,
                        channel      TEXT,
                        channel_list TEXT,
                        organization TEXT,
                        time_zone    TEXT,
                        updated_at   DATETIME NOT NULL DEFAULT (datetime('now'))
                );

                CREATE UNIQUE INDEX IF NOT EXISTS idx_retail_player_device_mapping_slug
                        ON retail_player_device_mapping(device_slug);
                CREATE INDEX IF NOT EXISTS idx_retail_player_device_mapping_name
                        ON retail_player_device_mapping(device_name COLLATE NOCASE);
        `)
	return err
}

func downCreateRetailPlayerDeviceMapping(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
                DROP INDEX IF EXISTS idx_retail_player_device_mapping_slug;
                DROP INDEX IF EXISTS idx_retail_player_device_mapping_name;
                DROP TABLE IF EXISTS retail_player_device_mapping;
        `)
	return err
}
