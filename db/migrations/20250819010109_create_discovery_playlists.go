package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(Up20250819010109, Down20250819010109)
}

func Up20250819010109(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`
create table if not exists discovery_playlists (
    id varchar(255) not null primary key,
    name varchar(255) not null,
    folder_path text not null unique,
    song_count integer not null default 0,
    updated_at datetime not null
);

create index if not exists discovery_playlists_name on discovery_playlists(name);
`)
	return err
}

func Down20250819010109(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`drop table if exists discovery_playlists;`)
	return err
}
