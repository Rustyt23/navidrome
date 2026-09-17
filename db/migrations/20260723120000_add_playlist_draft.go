package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddPlaylistDraft, downAddPlaylistDraft)
}

// Playlist drafts hold proposed changes to a live playlist so an AI suggestion
// can be reviewed and approved before anything is written to the playlist
// itself. Drafts live on the server rather than in browser storage so a review
// survives a different browser, a different reviewer, and a page reload.
func upAddPlaylistDraft(ctx context.Context, tx *sql.Tx) error {
	// source_version is a content hash of the live playlist's ordered track
	// list when the draft was taken. Publishing recomputes it and refuses to
	// proceed if it moved, which is what makes a stale draft detectable.
	if _, err := tx.ExecContext(ctx, `
create table if not exists playlist_draft (
    id                 varchar(255) not null primary key,
    playlist_id        varchar(255) not null
        references playlist(id) on update cascade on delete cascade,
    source_version     varchar(64)  not null default '',
    status             varchar(32)  not null default 'draft',
    name               varchar(255) not null default '',
    created_by         varchar(255) not null default '',
    created_at         datetime     not null,
    updated_at         datetime     not null,
    proposed_track_ids text         not null default '[]',
    reviewed_by        varchar(255) not null default '',
    reviewed_at        datetime,
    published_by       varchar(255) not null default '',
    published_at       datetime,
    conflict_detail    text         not null default ''
);`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
create index if not exists playlist_draft_playlist_id on playlist_draft(playlist_id);`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
create index if not exists playlist_draft_status on playlist_draft(status);`); err != nil {
		return err
	}

	// One row per proposed operation, kept alongside the resulting track order
	// so a reviewer can see not just what the playlist would become but which
	// AI suggestion produced each change and why.
	if _, err := tx.ExecContext(ctx, `
create table if not exists playlist_draft_change (
    id                 varchar(255) not null primary key,
    draft_id           varchar(255) not null
        references playlist_draft(id) on update cascade on delete cascade,
    seq                integer      not null default 0,
    kind               varchar(32)  not null,
    media_file_id      varchar(255) not null default '',
    replaced_id        varchar(255) not null default '',
    from_position      integer      not null default -1,
    to_position        integer      not null default -1,
    reason             text         not null default '',
    source             varchar(64)  not null default '',
    confidence         integer      not null default 0,
    created_at         datetime     not null
);`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `
create index if not exists playlist_draft_change_draft_id on playlist_draft_change(draft_id, seq);`)
	return err
}

func downAddPlaylistDraft(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `drop table if exists playlist_draft_change;`); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `drop table if exists playlist_draft;`)
	return err
}
