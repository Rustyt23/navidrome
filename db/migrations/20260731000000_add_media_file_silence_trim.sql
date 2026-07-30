-- +goose Up
-- Silence trimming is deliberately audited separately from LUFS processing.
-- A silence restore returns the exact file that existed immediately before the
-- trim, while a LUFS restore returns the pre-normalisation file.
CREATE TABLE IF NOT EXISTS media_file_silence_trim
(
    media_file_id          varchar(255) NOT NULL PRIMARY KEY,
    status                 varchar(32)  NOT NULL DEFAULT '',
    classification         varchar(32)  NOT NULL DEFAULT '',
    decision               varchar(32)  NOT NULL DEFAULT '',
    reason                 varchar(1024) NOT NULL DEFAULT '',
    leading_kind           varchar(32)  NOT NULL DEFAULT '',
    trailing_kind          varchar(32)  NOT NULL DEFAULT '',
    leading_silence        real         NOT NULL DEFAULT 0,
    trailing_silence       real         NOT NULL DEFAULT 0,
    leading_samples        integer      NOT NULL DEFAULT 0,
    trailing_samples       integer      NOT NULL DEFAULT 0,
    proposed_start_trim    real         NOT NULL DEFAULT 0,
    proposed_end_trim      real         NOT NULL DEFAULT 0,
    proposed_start_samples integer      NOT NULL DEFAULT 0,
    proposed_end_samples   integer      NOT NULL DEFAULT 0,
    applied_start_trim     real         NOT NULL DEFAULT 0,
    applied_end_trim       real         NOT NULL DEFAULT 0,
    applied_start_samples  integer      NOT NULL DEFAULT 0,
    applied_end_samples    integer      NOT NULL DEFAULT 0,
    retained_padding       real         NOT NULL DEFAULT 0,
    retained_padding_samples integer    NOT NULL DEFAULT 0,
    method                 varchar(32)  NOT NULL DEFAULT '',
    integrity              varchar(32)  NOT NULL DEFAULT '',
    codec_before           varchar(32)  NOT NULL DEFAULT '',
    codec_after            varchar(32)  NOT NULL DEFAULT '',
    bitrate_before         integer      NOT NULL DEFAULT 0,
    bitrate_after          integer      NOT NULL DEFAULT 0,
    sample_rate_before     integer      NOT NULL DEFAULT 0,
    sample_rate_after      integer      NOT NULL DEFAULT 0,
    bit_depth_before       integer      NOT NULL DEFAULT 0,
    bit_depth_after        integer      NOT NULL DEFAULT 0,
    channels_before        integer      NOT NULL DEFAULT 0,
    channels_after         integer      NOT NULL DEFAULT 0,
    duration_before        real         NOT NULL DEFAULT 0,
    duration_after         real         NOT NULL DEFAULT 0,
    size_before            integer      NOT NULL DEFAULT 0,
    size_after             integer      NOT NULL DEFAULT 0,
    art_before             bool         NOT NULL DEFAULT false,
    art_after              bool         NOT NULL DEFAULT false,
    has_backup             bool         NOT NULL DEFAULT false,
    backup_sha256          varchar(64)  NOT NULL DEFAULT '',
    source_sha256          varchar(64)  NOT NULL DEFAULT '',
    result_sha256          varchar(64)  NOT NULL DEFAULT '',
    source_modified_at     datetime,
    result_modified_at     datetime,
    error                  varchar(2048) NOT NULL DEFAULT '',
    analyzed_at            datetime,
    applied_at             datetime,
    FOREIGN KEY (media_file_id) REFERENCES media_file (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS media_file_silence_trim_status
    ON media_file_silence_trim (status);
CREATE INDEX IF NOT EXISTS media_file_silence_trim_classification
    ON media_file_silence_trim (classification);
CREATE INDEX IF NOT EXISTS media_file_silence_trim_decision
    ON media_file_silence_trim (decision);

-- +goose Down
DROP TABLE IF EXISTS media_file_silence_trim;
