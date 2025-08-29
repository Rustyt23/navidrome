package migrations

import (
	"context"
	"database/sql"
	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upCreatePlaylistFolder, downCreatePlaylistFolder)
}

func upCreatePlaylistFolder(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		-- Create playlist_folder table
		CREATE TABLE IF NOT EXISTS playlist_folder (
			id         VARCHAR NOT NULL PRIMARY KEY,
			name       VARCHAR NOT NULL CHECK (length(trim(name)) > 0),
			parent_id  VARCHAR NULL,
			owner_id   VARCHAR NOT NULL REFERENCES user(id) ON UPDATE CASCADE ON DELETE CASCADE,
			public     BOOL NOT NULL DEFAULT FALSE,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (parent_id) REFERENCES playlist_folder(id) ON DELETE CASCADE
		);

		-- Case-insensitive uniqueness among siblings for each owner
		CREATE UNIQUE INDEX IF NOT EXISTS playlist_folder_sibling_uniq
		  ON playlist_folder (owner_id, parent_id, lower(name));

		CREATE INDEX IF NOT EXISTS idx_playlist_folder_parent_id ON playlist_folder(parent_id);
		CREATE INDEX IF NOT EXISTS idx_playlist_folder_owner_id  ON playlist_folder(owner_id);

		-- Add folder_id to playlist table
		ALTER TABLE playlist ADD COLUMN folder_id TEXT NULL;
		CREATE INDEX IF NOT EXISTS idx_playlist_folder_id ON playlist (folder_id);

		-- Delete playlists when a folder is deleted
		CREATE TRIGGER IF NOT EXISTS trg_playlist_folder_delete_playlists
		AFTER DELETE ON playlist_folder
		BEGIN
			DELETE FROM playlist WHERE folder_id = OLD.id;
		END;

		-- ===== Normalize any legacy empty-string roots to NULL =====
		UPDATE playlist_folder SET parent_id = NULL WHERE parent_id = '';
		UPDATE playlist        SET folder_id = NULL WHERE folder_id = '';
	`)
	return err
}

func downCreatePlaylistFolder(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		-- Drop trigger
		DROP TRIGGER IF EXISTS trg_playlist_folder_delete_playlists;

		-- Drop playlist indexes / column
		DROP INDEX IF EXISTS idx_playlist_folder_id;
		ALTER TABLE playlist DROP COLUMN IF EXISTS folder_id;

		-- Drop folder indexes & table
		DROP INDEX IF EXISTS playlist_folder_sibling_uniq;
		DROP INDEX IF EXISTS idx_playlist_folder_parent_id;
		DROP INDEX IF EXISTS idx_playlist_folder_owner_id;
		DROP TABLE IF EXISTS playlist_folder;
	`)
	return err
}
