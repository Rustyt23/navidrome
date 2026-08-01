-- +goose Up
CREATE TABLE IF NOT EXISTS media_file_silence_backup
(
    media_file_id   varchar(255) NOT NULL PRIMARY KEY,
    backup_file     varchar(255) NOT NULL,
    status          varchar(32)  NOT NULL DEFAULT '',
    original_sha256 varchar(64)  NOT NULL,
    trimmed_sha256  varchar(64)  NOT NULL DEFAULT '',
    original_size   integer      NOT NULL DEFAULT 0,
    trimmed_size    integer      NOT NULL DEFAULT 0,
    original_mode   integer      NOT NULL DEFAULT 0,
    trim_start      real         NOT NULL DEFAULT 0,
    trim_end        real         NOT NULL DEFAULT 0,
    original_mod_time datetime   NOT NULL,
    created_at      datetime     NOT NULL,
    prepared_at     datetime,
    trimmed_at      datetime,
    restored_at     datetime,
    FOREIGN KEY (media_file_id) REFERENCES media_file (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS media_file_silence_backup_status ON media_file_silence_backup (status);

-- +goose Down
DROP TABLE IF EXISTS media_file_silence_backup;
