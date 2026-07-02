-- +goose Up
-- Fix case-insensitive sorting for playlist names
-- Fork note: temporarily drop the playlist_folder trigger that references the
-- playlist table, and preserve the fork's folder_id column through the rebuild.
DROP TRIGGER IF EXISTS trg_playlist_folder_delete_playlists;

create table playlist_dg_tmp
(
    id           varchar(255)                              not null
        primary key,
    name         varchar(255) collate NOCASE default ''    not null,
    comment      varchar(255)                default ''    not null,
    duration     real                        default 0     not null,
    song_count   integer                     default 0     not null,
    public       bool                        default FALSE not null,
    created_at   datetime,
    updated_at   datetime,
    path         string                      default ''    not null,
    sync         bool                        default false not null,
    size         integer                     default 0     not null,
    rules        varchar,
    evaluated_at datetime,
    owner_id     varchar(255)                              not null
        constraint playlist_user_user_id_fk
            references user
            on update cascade on delete cascade,
    folder_id    TEXT
);

insert into playlist_dg_tmp(id, name, comment, duration, song_count, public, created_at, updated_at, path, sync, size,
                            rules, evaluated_at, owner_id, folder_id)
select id, name, comment, duration, song_count, public, created_at, updated_at, path, sync, size, rules, evaluated_at,
       owner_id, folder_id
from playlist;

drop table playlist;

alter table playlist_dg_tmp
    rename to playlist;

create index playlist_name
    on playlist (name);

create index playlist_created_at
    on playlist (created_at);

create index playlist_updated_at
    on playlist (updated_at);

create index playlist_evaluated_at
    on playlist (evaluated_at);

create index playlist_size
    on playlist (size);

create index if not exists idx_playlist_folder_id
    on playlist (folder_id);

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_playlist_folder_delete_playlists
AFTER DELETE ON playlist_folder
BEGIN
    DELETE FROM playlist WHERE folder_id = OLD.id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS trg_playlist_folder_delete_playlists;
-- Note: Downgrade loses the collation but preserves data
create table playlist_dg_tmp
(
    id           varchar(255)              not null
        primary key,
    name         varchar(255) default ''   not null,
    comment      varchar(255) default ''   not null,
    duration     real         default 0    not null,
    song_count   integer      default 0    not null,
    public       bool         default FALSE not null,
    created_at   datetime,
    updated_at   datetime,
    path         string       default ''   not null,
    sync         bool         default false not null,
    size         integer      default 0    not null,
    rules        varchar,
    evaluated_at datetime,
    owner_id     varchar(255)              not null
        constraint playlist_user_user_id_fk
            references user
            on update cascade on delete cascade,
    folder_id    TEXT
);

insert into playlist_dg_tmp(id, name, comment, duration, song_count, public, created_at, updated_at, path, sync, size,
                            rules, evaluated_at, owner_id, folder_id)
select id, name, comment, duration, song_count, public, created_at, updated_at, path, sync, size, rules, evaluated_at,
       owner_id, folder_id
from playlist;

drop table playlist;

alter table playlist_dg_tmp
    rename to playlist;

create index playlist_name
    on playlist (name);

create index playlist_created_at
    on playlist (created_at);

create index playlist_updated_at
    on playlist (updated_at);

create index playlist_evaluated_at
    on playlist (evaluated_at);

create index playlist_size
    on playlist (size);

create index if not exists idx_playlist_folder_id
    on playlist (folder_id);

-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_playlist_folder_delete_playlists
AFTER DELETE ON playlist_folder
BEGIN
    DELETE FROM playlist WHERE folder_id = OLD.id;
END;
-- +goose StatementEnd
