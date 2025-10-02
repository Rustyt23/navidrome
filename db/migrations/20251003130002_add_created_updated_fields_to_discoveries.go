package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(Up20251003130002, Down20251003130002)
}

func Up20251003130002(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`
alter table discovery
add created_at datetime;
alter table discovery
add updated_at datetime;
update discovery
set created_at = datetime('now'), updated_at = datetime('now');
`)
	return err
}

func Down20251003130002(_ context.Context, tx *sql.Tx) error {
	return nil
}
