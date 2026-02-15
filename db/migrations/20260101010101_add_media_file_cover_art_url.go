package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(Up20260101010101, Down20260101010101)
}

func Up20260101010101(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`alter table media_file add column cover_art_url varchar(1024) default '' not null;`)
	return err
}

func Down20260101010101(_ context.Context, tx *sql.Tx) error {
	return nil
}
