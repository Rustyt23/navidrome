package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(Up20241014120000, Down20241014120000)
}

func Up20241014120000(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`
create table playlist_tracks_tmp (
        id integer default 0 not null,
        playlist_id varchar(255) not null
                constraint playlist_tracks_playlist_id_fk
                        references playlist
                                on update cascade on delete cascade,
        media_file_id varchar(255) not null
);
`)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`
select playlist_id, media_file_id
from playlist_tracks
order by playlist_id, id
`)
	if err != nil {
		return err
	}
	defer rows.Close()

	stmt, err := tx.Prepare(`
insert into playlist_tracks_tmp (id, playlist_id, media_file_id)
values (?, ?, ?)
`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	currentPlaylist := ""
	position := 1
	seen := map[string]struct{}{}
	for rows.Next() {
		var playlistID, mediaFileID string
		if err := rows.Scan(&playlistID, &mediaFileID); err != nil {
			return err
		}
		if playlistID != currentPlaylist {
			currentPlaylist = playlistID
			position = 1
			seen = map[string]struct{}{}
		}
		if _, exists := seen[mediaFileID]; exists {
			continue
		}
		seen[mediaFileID] = struct{}{}
		if _, err := stmt.Exec(position, playlistID, mediaFileID); err != nil {
			return err
		}
		position++
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if _, err := tx.Exec(`drop table playlist_tracks`); err != nil {
		return err
	}
	if _, err := tx.Exec(`alter table playlist_tracks_tmp rename to playlist_tracks`); err != nil {
		return err
	}
	if _, err := tx.Exec(`create unique index playlist_tracks_pos on playlist_tracks (playlist_id, id)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`create unique index playlist_tracks_unique_media on playlist_tracks (playlist_id, media_file_id)`); err != nil {
		return err
	}

	_, err = tx.Exec(`
update playlist
set song_count = (
        select count(*) from playlist_tracks where playlist_id = playlist.id
),
    duration = coalesce((
        select sum(m.duration)
        from playlist_tracks pt
        join media_file m on m.id = pt.media_file_id
        where pt.playlist_id = playlist.id
), 0),
    size = coalesce((
        select sum(m.size)
        from playlist_tracks pt
        join media_file m on m.id = pt.media_file_id
        where pt.playlist_id = playlist.id
), 0)
`)
	return err
}

func Down20241014120000(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`drop index if exists playlist_tracks_unique_media`)
	return err
}
