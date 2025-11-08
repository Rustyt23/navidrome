package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upCreateRetailPlayerDeviceMapping, downCreateRetailPlayerDeviceMapping)
}

func upCreateRetailPlayerDeviceMapping(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`
create table if not exists retail_player_device_mapping
(
    id          varchar(255) primary key,
    slug_key    varchar       not null unique,
    identifier  varchar       not null,
    device_id   varchar       not null,
    created_at  datetime,
    updated_at  datetime
);
create unique index if not exists idx_retail_player_device_mapping_device_id
    on retail_player_device_mapping (device_id);
`)
	return err
}

func downCreateRetailPlayerDeviceMapping(_ context.Context, tx *sql.Tx) error {
	return nil
}
