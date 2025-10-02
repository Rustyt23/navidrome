package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upTouchDiscoveries, downTouchDiscoveries)
}

func upTouchDiscoveries(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`update discovery set updated_at = datetime('now');`)
	return err
}

func downTouchDiscoveries(_ context.Context, tx *sql.Tx) error {
	return nil
}
