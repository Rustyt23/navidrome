package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddPlaylistHistoryAudit, downAddPlaylistHistoryAudit)
}

func upAddPlaylistHistoryAudit(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`alter table playlist_draft add column rollback_version_id varchar(255) not null default '';`,
		`alter table playlist_draft add column rollback_version_number integer not null default 0;`,
		`alter table playlist_draft_change add column ai_model_version varchar(255) not null default '';`,
		`alter table playlist_draft_change add column ai_index_version varchar(255) not null default '';`,
		`alter table playlist_draft_change add column ruleset_version varchar(255) not null default '';`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `
create table playlist_version (
    id                    varchar(255) not null primary key,
    playlist_id           varchar(255) not null
        references playlist(id) on update cascade on delete cascade,
    version               integer      not null,
    draft_id              varchar(255) not null,
    track_ids             text         not null,
    previous_track_ids    text         not null,
    changes               text         not null,
    change_summary        text         not null,
    published_by          varchar(255) not null default '',
    published_at          datetime     not null,
    approved_by           varchar(255) not null default '',
    approved_at           datetime,
    ai_model_version      varchar(255) not null default '',
    ai_index_version      varchar(255) not null default '',
    ruleset_version       varchar(255) not null default '',
    rollback_from_version integer      not null default 0,
    unique(playlist_id, version),
    unique(draft_id)
);`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
create index playlist_version_playlist_date
    on playlist_version(playlist_id, version desc);`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
create table playlist_audit_event (
    id             varchar(255) not null primary key,
    playlist_id    varchar(255) not null
        references playlist(id) on update cascade on delete cascade,
    draft_id       varchar(255) not null default '',
    version_id     varchar(255) not null default '',
    version_number integer      not null default 0,
    event_type     varchar(64)  not null,
    actor          varchar(255) not null default '',
    summary        text         not null default '',
    details        text         not null default '',
    created_at     datetime     not null
);`); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
create index playlist_audit_event_playlist_date
    on playlist_audit_event(playlist_id, created_at desc);`)
	return err
}

func downAddPlaylistHistoryAudit(ctx context.Context, tx *sql.Tx) error {
	for _, statement := range []string{
		`drop table if exists playlist_audit_event;`,
		`drop table if exists playlist_version;`,
		`alter table playlist_draft_change drop column ruleset_version;`,
		`alter table playlist_draft_change drop column ai_index_version;`,
		`alter table playlist_draft_change drop column ai_model_version;`,
		`alter table playlist_draft drop column rollback_version_number;`,
		`alter table playlist_draft drop column rollback_version_id;`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}
