package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upCreateMissingPlaylistTracks, downCreateMissingPlaylistTracks)
}

func upCreateMissingPlaylistTracks(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
create table if not exists missing_playlist_tracks (
        id integer primary key autoincrement,
        playlist_id text not null,
        track_path text not null,
        created_at datetime not null default current_timestamp
);
create index if not exists missing_playlist_tracks_playlist_id_idx on missing_playlist_tracks(playlist_id);
`)
	return err
}

func downCreateMissingPlaylistTracks(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
drop index if exists missing_playlist_tracks_playlist_id_idx;
drop table if exists missing_playlist_tracks;
`)
	return err
}
