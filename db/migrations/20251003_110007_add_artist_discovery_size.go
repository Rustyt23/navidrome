package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(Up20201012210022, Down20201012210022)
}

func Up20201012210022(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`
alter table artist
	add size integer default 0 not null;
create index if not exists artist_size
	on artist(size);

update artist set size = ifnull((
   select sum(f.size)
   from album f
   where f.album_artist_id = artist.id
), 0)
where id not null;

alter table discovery
	add size integer default 0 not null;
create index if not exists playlist_size
	on discovery(size);

update discovery set size = ifnull((
    select sum(size)
    from media_file f
             left join discovery_tracks pt on f.id = pt.media_file_id
    where pt.discovery_id = playlist.id
), 0);`)

	return err
}

func Down20201012210022(_ context.Context, tx *sql.Tx) error {
	return nil
}
