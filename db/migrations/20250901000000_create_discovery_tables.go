package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upCreateDiscoveryTables, downCreateDiscoveryTables)
}

func upCreateDiscoveryTables(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS discovery_folders (
            id         VARCHAR NOT NULL PRIMARY KEY,
            name       VARCHAR NOT NULL CHECK (length(trim(name)) > 0),
            parent_id  VARCHAR NULL,
            owner_id   VARCHAR NOT NULL REFERENCES user(id) ON UPDATE CASCADE ON DELETE CASCADE,
            public     BOOL NOT NULL DEFAULT FALSE,
            created_at DATETIME NOT NULL DEFAULT (datetime('now')),
            updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
            FOREIGN KEY (parent_id) REFERENCES discovery_folders(id) ON DELETE CASCADE
        );

        CREATE UNIQUE INDEX IF NOT EXISTS discovery_folders_sibling_uniq
          ON discovery_folders (owner_id, parent_id, lower(name));

        CREATE INDEX IF NOT EXISTS idx_discovery_folders_parent_id ON discovery_folders(parent_id);
        CREATE INDEX IF NOT EXISTS idx_discovery_folders_owner_id  ON discovery_folders(owner_id);

        CREATE TABLE IF NOT EXISTS discovery (
            id           VARCHAR(255) NOT NULL PRIMARY KEY,
            name         VARCHAR(255) DEFAULT '' NOT NULL,
            comment      VARCHAR(255) DEFAULT '' NOT NULL,
            duration     REAL DEFAULT 0 NOT NULL,
            song_count   INTEGER DEFAULT 0 NOT NULL,
            public       BOOL DEFAULT FALSE NOT NULL,
            created_at   DATETIME,
            updated_at   DATETIME,
            path         STRING DEFAULT '' NOT NULL,
            sync         BOOL DEFAULT FALSE NOT NULL,
            size         INTEGER DEFAULT 0 NOT NULL,
            rules        VARCHAR,
            evaluated_at DATETIME,
            owner_id     VARCHAR(255) NOT NULL
                REFERENCES user(id)
                    ON UPDATE CASCADE ON DELETE CASCADE,
            folder_id    TEXT NULL
        );

        CREATE INDEX IF NOT EXISTS discovery_created_at   ON discovery(created_at);
        CREATE INDEX IF NOT EXISTS discovery_evaluated_at ON discovery(evaluated_at);
        CREATE INDEX IF NOT EXISTS discovery_name         ON discovery(name);
        CREATE INDEX IF NOT EXISTS discovery_size         ON discovery(size);
        CREATE INDEX IF NOT EXISTS discovery_updated_at   ON discovery(updated_at);
        CREATE INDEX IF NOT EXISTS discovery_folder_id    ON discovery(folder_id);

        CREATE TRIGGER IF NOT EXISTS trg_discovery_folder_delete_discovery
        AFTER DELETE ON discovery_folders
        BEGIN
            DELETE FROM discovery WHERE folder_id = OLD.id;
        END;

        CREATE TABLE IF NOT EXISTS discovery_songs (
            id            INTEGER DEFAULT 0 NOT NULL,
            discovery_id  VARCHAR(255) NOT NULL
                REFERENCES discovery(id)
                    ON UPDATE CASCADE ON DELETE CASCADE,
            media_file_id VARCHAR(255) NOT NULL
        );

        CREATE UNIQUE INDEX IF NOT EXISTS discovery_songs_pos
          ON discovery_songs (discovery_id, id);
    `)
	return err
}

func downCreateDiscoveryTables(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
        DROP INDEX IF EXISTS discovery_songs_pos;
        DROP TABLE IF EXISTS discovery_songs;

        DROP TRIGGER IF EXISTS trg_discovery_folder_delete_discovery;

        DROP INDEX IF EXISTS discovery_folder_id;
        DROP INDEX IF EXISTS discovery_updated_at;
        DROP INDEX IF EXISTS discovery_size;
        DROP INDEX IF EXISTS discovery_name;
        DROP INDEX IF EXISTS discovery_evaluated_at;
        DROP INDEX IF EXISTS discovery_created_at;
        DROP TABLE IF EXISTS discovery;

        DROP INDEX IF EXISTS discovery_folders_sibling_uniq;
        DROP INDEX IF EXISTS idx_discovery_folders_parent_id;
        DROP INDEX IF EXISTS idx_discovery_folders_owner_id;
        DROP TABLE IF EXISTS discovery_folders;
    `)
	return err
}
