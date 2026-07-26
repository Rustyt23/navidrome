package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddPlaylistRecommendationDecision, downAddPlaylistRecommendationDecision)
}

// Recommendation decisions make Ignore durable for the current draft and make
// Block durable for a playlist/client profile. They are intentionally separate
// from draft changes: neither decision proposes a playlist track mutation.
func upAddPlaylistRecommendationDecision(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `
create table if not exists playlist_recommendation_decision (
    id                  varchar(255) not null primary key,
    playlist_id         varchar(255) not null
        references playlist(id) on update cascade on delete cascade,
    draft_id            varchar(255) not null default '',
    profile_id          varchar(255) not null default '',
    song_id             varchar(255) not null,
    original_song_id    varchar(255) not null default '',
    recommendation_type varchar(64)  not null default '',
    decision            varchar(16)  not null,
    created_by          varchar(255) not null default '',
    created_at          datetime     not null
);`); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
create index if not exists playlist_recommendation_decision_scope
    on playlist_recommendation_decision(playlist_id, draft_id, decision, song_id);`)
	return err
}

func downAddPlaylistRecommendationDecision(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `drop table if exists playlist_recommendation_decision;`)
	return err
}
