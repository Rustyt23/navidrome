-- +goose Up
CREATE TABLE IF NOT EXISTS media_file_loudness
(
    media_file_id      varchar(255) NOT NULL PRIMARY KEY,
    status             varchar(32)  NOT NULL DEFAULT '',
    verdict            varchar(32)  NOT NULL DEFAULT '',
    action             varchar(32)  NOT NULL DEFAULT '',
    lufs_before        real,
    lufs_after         real,
    gain_applied       real,
    tp_before          real,
    tp_after           real,
    lra_before         real,
    lra_after          real,
    null_residual      real,
    codec_before       varchar(32)  NOT NULL DEFAULT '',
    bitrate_before     integer      NOT NULL DEFAULT 0,
    sample_rate_before integer      NOT NULL DEFAULT 0,
    bit_depth_before   integer      NOT NULL DEFAULT 0,
    channels_before    integer      NOT NULL DEFAULT 0,
    duration_before    real         NOT NULL DEFAULT 0,
    size_before        integer      NOT NULL DEFAULT 0,
    art_before         bool         NOT NULL DEFAULT false,
    art_after          bool         NOT NULL DEFAULT false,
    has_backup         bool         NOT NULL DEFAULT false,
    error              varchar(1024) NOT NULL DEFAULT '',
    analyzed_at        datetime,
    FOREIGN KEY (media_file_id) REFERENCES media_file (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS media_file_loudness_verdict ON media_file_loudness (verdict);
CREATE INDEX IF NOT EXISTS media_file_loudness_status ON media_file_loudness (status);

-- +goose Down
DROP TABLE IF EXISTS media_file_loudness;
