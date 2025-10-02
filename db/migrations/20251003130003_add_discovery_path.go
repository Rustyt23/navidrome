package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddDiscoveryPath, downAddDiscoveryPath)
}

func upAddDiscoveryPath(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`
alter table discovery
add path string default '' not null;

alter table discovery
add sync bool default false not null;
`)

	return err
}

func downAddDiscoveryPath(_ context.Context, tx *sql.Tx) error {
	return nil
}
