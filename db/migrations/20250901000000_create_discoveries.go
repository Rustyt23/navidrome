package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upCreateDiscoveries, downCreateDiscoveries)
}

func upCreateDiscoveries(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS discoveries (
                id varchar(255) not null
                        primary key,
                name varchar(255) default '' not null,
                comment varchar(255) default '' not null,
                duration real default 0 not null,
                song_count integer default 0 not null,
                public bool default FALSE not null,
                created_at datetime,
                updated_at datetime,
                path string default '' not null,
                sync bool default false not null,
                size integer default 0 not null,
                rules varchar,
                evaluated_at datetime,
                owner_id varchar(255) not null
                        constraint discoveries_user_user_id_fk
                                references user
                                        on update cascade on delete cascade,
                folder_id text null
        );

        CREATE INDEX IF NOT EXISTS discoveries_created_at
                ON discoveries (created_at);
        CREATE INDEX IF NOT EXISTS discoveries_evaluated_at
                ON discoveries (evaluated_at);
        CREATE INDEX IF NOT EXISTS discoveries_name
                ON discoveries (name);
        CREATE INDEX IF NOT EXISTS discoveries_size
                ON discoveries (size);
        CREATE INDEX IF NOT EXISTS discoveries_updated_at
                ON discoveries (updated_at);
        CREATE INDEX IF NOT EXISTS idx_discoveries_folder_id
                ON discoveries (folder_id);

        CREATE TRIGGER IF NOT EXISTS trg_playlist_folder_delete_discoveries
        AFTER DELETE ON playlist_folder
        BEGIN
                DELETE FROM discoveries WHERE folder_id = OLD.id;
        END;

        CREATE TABLE IF NOT EXISTS discovery_tracks (
                id integer default 0 not null,
                discovery_id varchar(255) not null
                        constraint discovery_tracks_discovery_id_fk
                                references discoveries
                                        on update cascade on delete cascade,
                media_file_id varchar(255) not null
        );

        CREATE UNIQUE INDEX IF NOT EXISTS discovery_tracks_pos
                ON discovery_tracks (discovery_id, id);
    `)
	return err
}

func downCreateDiscoveries(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
        DROP INDEX IF EXISTS discovery_tracks_pos;
        DROP TABLE IF EXISTS discovery_tracks;
        DROP TRIGGER IF EXISTS trg_playlist_folder_delete_discoveries;
        DROP INDEX IF EXISTS idx_discoveries_folder_id;
        DROP INDEX IF EXISTS discoveries_updated_at;
        DROP INDEX IF EXISTS discoveries_size;
        DROP INDEX IF EXISTS discoveries_name;
        DROP INDEX IF EXISTS discoveries_evaluated_at;
        DROP INDEX IF EXISTS discoveries_created_at;
        DROP TABLE IF EXISTS discoveries;
    `)
	return err
}
