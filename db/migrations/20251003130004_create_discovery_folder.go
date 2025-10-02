package migrations

import (
	"context"
	"database/sql"
	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upCreateDiscoveryFolder, downCreateDiscoveryFolder)
}

func upCreateDiscoveryFolder(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
-- Create discovery_folder table
CREATE TABLE IF NOT EXISTS discovery_folder (
id         VARCHAR NOT NULL PRIMARY KEY,
name       VARCHAR NOT NULL CHECK (length(trim(name)) > 0),
parent_id  VARCHAR NULL,
owner_id   VARCHAR NOT NULL REFERENCES user(id) ON UPDATE CASCADE ON DELETE CASCADE,
public     BOOL NOT NULL DEFAULT FALSE,
created_at DATETIME NOT NULL DEFAULT (datetime('now')),
updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
FOREIGN KEY (parent_id) REFERENCES discovery_folder(id) ON DELETE CASCADE
);

-- Case-insensitive uniqueness among siblings for each owner
CREATE UNIQUE INDEX IF NOT EXISTS discovery_folder_sibling_uniq
  ON discovery_folder (owner_id, parent_id, lower(name));

CREATE INDEX IF NOT EXISTS idx_discovery_folder_parent_id ON discovery_folder(parent_id);
CREATE INDEX IF NOT EXISTS idx_discovery_folder_owner_id  ON discovery_folder(owner_id);

-- Add folder_id to discovery table
ALTER TABLE discovery ADD COLUMN folder_id TEXT NULL;
CREATE INDEX IF NOT EXISTS idx_discovery_folder_id ON discovery (folder_id);

-- Delete discoveries when a folder is deleted
CREATE TRIGGER IF NOT EXISTS trg_discovery_folder_delete_discoveries
AFTER DELETE ON discovery_folder
BEGIN
DELETE FROM discovery WHERE folder_id = OLD.id;
END;

-- ===== Normalize any legacy empty-string roots to NULL =====
UPDATE discovery_folder SET parent_id = NULL WHERE parent_id = '';
UPDATE discovery        SET folder_id = NULL WHERE folder_id = '';
`)
	return err
}

func downCreateDiscoveryFolder(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
-- Drop trigger
DROP TRIGGER IF EXISTS trg_discovery_folder_delete_discoveries;

-- Drop discovery indexes / column
DROP INDEX IF EXISTS idx_discovery_folder_id;
ALTER TABLE discovery DROP COLUMN IF EXISTS folder_id;

-- Drop folder indexes & table
DROP INDEX IF EXISTS discovery_folder_sibling_uniq;
DROP INDEX IF EXISTS idx_discovery_folder_parent_id;
DROP INDEX IF EXISTS idx_discovery_folder_owner_id;
DROP TABLE IF EXISTS discovery_folder;
`)
	return err
}
