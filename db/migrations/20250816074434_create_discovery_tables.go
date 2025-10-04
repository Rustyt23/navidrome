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
                CREATE TABLE IF NOT EXISTS discovery (
                        id         VARCHAR NOT NULL PRIMARY KEY,
                        name       VARCHAR NOT NULL CHECK (length(trim(name)) > 0),
                        comment    VARCHAR NOT NULL DEFAULT '',
                        duration   REAL NOT NULL DEFAULT 0,
                        size       INTEGER NOT NULL DEFAULT 0,
                        song_count INTEGER NOT NULL DEFAULT 0,
                        owner_id   VARCHAR NOT NULL REFERENCES user(id) ON UPDATE CASCADE ON DELETE CASCADE,
                        path       TEXT NOT NULL,
                        created_at DATETIME NOT NULL DEFAULT (datetime('now')),
                        updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
                );

                CREATE UNIQUE INDEX IF NOT EXISTS idx_discovery_owner_name ON discovery(owner_id, lower(name));
                CREATE INDEX IF NOT EXISTS idx_discovery_owner ON discovery(owner_id);
                CREATE INDEX IF NOT EXISTS idx_discovery_path ON discovery(path);

                CREATE TABLE IF NOT EXISTS discovery_tracks (
                        id            INTEGER NOT NULL,
                        discovery_id  VARCHAR NOT NULL REFERENCES discovery(id) ON DELETE CASCADE,
                        media_file_id VARCHAR NOT NULL REFERENCES media_file(id) ON DELETE CASCADE,
                        PRIMARY KEY (discovery_id, id)
                );

                CREATE INDEX IF NOT EXISTS idx_discovery_tracks_media ON discovery_tracks(media_file_id);
        `)
	return err
}

func downCreateDiscoveryTables(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
                DROP INDEX IF EXISTS idx_discovery_tracks_media;
                DROP TABLE IF EXISTS discovery_tracks;

                DROP INDEX IF EXISTS idx_discovery_owner_name;
                DROP INDEX IF EXISTS idx_discovery_owner;
                DROP INDEX IF EXISTS idx_discovery_path;
                DROP TABLE IF EXISTS discovery;
        `)
	return err
}
