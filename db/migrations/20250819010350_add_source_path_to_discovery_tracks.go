package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(Up20250819010350, Down20250819010350)
}

func Up20250819010350(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`
alter table discovery_tracks add column source_path text;
`)
	return err
}

func Down20250819010350(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`
create table if not exists discovery_tracks_tmp as select id, discovery_id, media_file_id, path, position, missing, media_file, created_at, updated_at from discovery_tracks;
drop table if exists discovery_tracks;
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
insert into discovery_tracks select id, discovery_id, media_file_id, path, position, missing, media_file, created_at, updated_at from discovery_tracks_tmp;
drop table if exists discovery_tracks_tmp;
create index if not exists discovery_tracks_discovery_id on discovery_tracks(discovery_id, position);
`)
	return err
}
