package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upCreateRetailPlayerFolders, downCreateRetailPlayerFolders)
}

func upCreateRetailPlayerFolders(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS retail_player_folder (
            id         TEXT PRIMARY KEY NOT NULL,
            name       TEXT NOT NULL,
            parent_id  TEXT,
            created_at DATETIME NOT NULL DEFAULT (datetime('now')),
            updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
            FOREIGN KEY(parent_id) REFERENCES retail_player_folder(id) ON DELETE SET NULL
        );

        CREATE INDEX IF NOT EXISTS idx_retail_player_folder_parent
            ON retail_player_folder(parent_id);
        CREATE INDEX IF NOT EXISTS idx_retail_player_folder_name
            ON retail_player_folder(name COLLATE NOCASE);

        CREATE TABLE IF NOT EXISTS retail_player_device_folder (
            device_id  TEXT NOT NULL,
            folder_id  TEXT NOT NULL,
            created_at DATETIME NOT NULL DEFAULT (datetime('now')),
            updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
            PRIMARY KEY(device_id, folder_id),
            FOREIGN KEY(folder_id) REFERENCES retail_player_folder(id) ON DELETE CASCADE
        );

        CREATE INDEX IF NOT EXISTS idx_retail_player_device_folder_device
            ON retail_player_device_folder(device_id);
        CREATE INDEX IF NOT EXISTS idx_retail_player_device_folder_folder
            ON retail_player_device_folder(folder_id);
    `)
	return err
}

func downCreateRetailPlayerFolders(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
        DROP INDEX IF EXISTS idx_retail_player_device_folder_device;
        DROP INDEX IF EXISTS idx_retail_player_device_folder_folder;
        DROP TABLE IF EXISTS retail_player_device_folder;
        DROP INDEX IF EXISTS idx_retail_player_folder_parent;
        DROP INDEX IF EXISTS idx_retail_player_folder_name;
        DROP TABLE IF EXISTS retail_player_folder;
    `)
	return err
}
