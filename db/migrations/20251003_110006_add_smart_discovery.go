package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddSmartDiscovery, downAddSmartDiscovery)
}

func upAddSmartDiscovery(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`
alter table discovery
add column rules varchar null;
alter table discovery
add column evaluated_at datetime null;
create index if not exists discovery_evaluated_at
on discovery(evaluated_at);

create table discovery_fields (
field varchar(255) not null,
discovery_id varchar(255) not null
constraint discovery_fields_discovery_id_fk
references discovery
on update cascade on delete cascade
);
create unique index discovery_fields_idx
on discovery_fields (field, discovery_id);
`)
	return err
}

func downAddSmartDiscovery(_ context.Context, tx *sql.Tx) error {
	return nil
}
