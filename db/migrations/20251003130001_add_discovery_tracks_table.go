package migrations

import (
	"context"
	"database/sql"
	"strings"

	"github.com/navidrome/navidrome/log"
	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(Up20200516140647, Down20200516140647)
}

func Up20200516140647(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`
create table if not exists discovery_tracks
(
	id integer default 0 not null, 
    discovery_id varchar(255) not null, 
	media_file_id varchar(255) not null
);

create unique index if not exists playlist_tracks_pos
	on discovery_tracks (discovery_id, id);
`)
	if err != nil {
		return err
	}
	rows, err := tx.Query("select id, tracks from discovery")
	if err != nil {
		return err
	}
	defer rows.Close()
	var id, tracks string
	for rows.Next() {
		err := rows.Scan(&id, &tracks)
		if err != nil {
			return err
		}
		err = Up20200516140647UpdatePlaylistTracks(tx, id, tracks)
		if err != nil {
			return err
		}
	}
	err = rows.Err()
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
create table discovery_dg_tmp
(
	id varchar(255) not null
		primary key,
	name varchar(255) default '' not null,
	comment varchar(255) default '' not null,
	duration real default 0 not null,
	song_count integer default 0 not null,
	owner varchar(255) default '' not null,
	public bool default FALSE not null,
	created_at datetime,
	updated_at datetime
);

insert into discovery_dg_tmp(id, name, comment, duration, owner, public, created_at, updated_at) 
	select id, name, comment, duration, owner, public, created_at, updated_at from discovery;

drop table playlist;

alter table discovery_dg_tmp rename to discovery;

create index playlist_name
	on discovery (name);

update discovery set song_count = (select count(*) from discovery_tracks where discovery_id = playlist.id)
where id <> ''

`)
	return err
}

func Up20200516140647UpdatePlaylistTracks(tx *sql.Tx, id string, tracks string) error {
	trackList := strings.Split(tracks, ",")
	stmt, err := tx.Prepare("insert into discovery_tracks (discovery_id, media_file_id, id) values (?, ?, ?)")
	if err != nil {
		return err
	}
	for i, trackId := range trackList {
		_, err := stmt.Exec(id, trackId, i+1)
		if err != nil {
			log.Error("Error adding track to playlist", "playlistId", id, "trackId", trackId, err)
		}
	}
	return nil
}

func Down20200516140647(_ context.Context, tx *sql.Tx) error {
	return nil
}
