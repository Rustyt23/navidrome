package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddUseridToDiscovery, downAddUseridToDiscovery)
}

func upAddUseridToDiscovery(_ context.Context, tx *sql.Tx) error {
	_, err := tx.Exec(`
create table discovery_dg_tmp
(
id varchar(255) not null
primary key,
name varchar(255) default '' not null,
comment varchar(255) default '' not null,
duration real default 0 not null,
song_count integer default 0 not null,
public bool default FALSE not null,
created_at datetime,
updated_at datetime,
path string default '' not null,
sync bool default false not null,
size integer default 0 not null,
rules varchar,
evaluated_at datetime,
owner_id varchar(255) not null
constraint discovery_user_user_id_fk
references user
on update cascade on delete cascade
);

insert into discovery_dg_tmp(id, name, comment, duration, song_count, public, created_at, updated_at, path, sync, size, rules, evaluated_at, owner_id)
select id, name, comment, duration, song_count, public, created_at, updated_at, path, sync, size, rules, evaluated_at,
       (select id from user where user_name = owner) as user_id from discovery;

drop table discovery;
alter table discovery_dg_tmp rename to discovery;
create index discovery_created_at
on discovery (created_at);
create index discovery_evaluated_at
on discovery (evaluated_at);
create index discovery_name
on discovery (name);
create index discovery_size
on discovery (size);
create index discovery_updated_at
on discovery (updated_at);

`)
	return err
}

func downAddUseridToDiscovery(_ context.Context, tx *sql.Tx) error {
	return nil
}
