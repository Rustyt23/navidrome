package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(Up20250819010210, Down20250819010210)
}

func Up20250819010210(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`
create table if not exists discovery_tracks (
    id varchar(255) not null primary key,
    discovery_id varchar(255) not null,
    media_file_id varchar(255),
    path text not null,
    position integer not null,
    missing boolean not null default false,
    media_file text not null,
    created_at datetime not null,
    updated_at datetime not null,
    foreign key(discovery_id) references discovery_playlists(id) on delete cascade
);

create index if not exists discovery_tracks_discovery_id on discovery_tracks(discovery_id, position);
`)
	return err
}

func Down20250819010210(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`drop table if exists discovery_tracks;`)
	return err
}
