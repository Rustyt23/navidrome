-- +goose Up
ALTER TABLE media_file_silence ADD COLUMN leading_silence_after real;
ALTER TABLE media_file_silence ADD COLUMN trailing_silence_after real;

-- +goose Down
-- SQLite column removal is not supported safely across all versions.
