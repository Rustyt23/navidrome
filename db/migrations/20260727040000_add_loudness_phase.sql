-- +goose Up
ALTER TABLE media_file_loudness ADD COLUMN phase integer NOT NULL DEFAULT -1;
ALTER TABLE media_file_loudness ADD COLUMN decision varchar(32) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS media_file_loudness_phase ON media_file_loudness (phase);

-- +goose Down
DROP INDEX IF EXISTS media_file_loudness_phase;
ALTER TABLE media_file_loudness DROP COLUMN phase;
ALTER TABLE media_file_loudness DROP COLUMN decision;
