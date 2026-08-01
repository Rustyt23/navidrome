-- +goose Up
CREATE TABLE IF NOT EXISTS media_file_silence
(
    media_file_id     varchar(255) NOT NULL PRIMARY KEY,
    leading_silence   real,
    trailing_silence  real,
    threshold_db      real         NOT NULL DEFAULT -60,
    minimum_silence   real         NOT NULL DEFAULT 0.005,
    source_size       integer      NOT NULL DEFAULT 0,
    source_updated_at datetime,
    status            varchar(32)  NOT NULL DEFAULT '',
    error             varchar(1024) NOT NULL DEFAULT '',
    analyzed_at       datetime,
    FOREIGN KEY (media_file_id) REFERENCES media_file (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS media_file_silence_status ON media_file_silence (status);
CREATE INDEX IF NOT EXISTS media_file_silence_analyzed_at ON media_file_silence (analyzed_at);

-- +goose Down
DROP TABLE IF EXISTS media_file_silence;
