-- +goose Up
-- The "after" side of the audit was being read from media_file, which the
-- scanner populates with its own metadata extractor. That measures duration
-- differently from ffprobe, so every before/after comparison carried a ~0.03s
-- phantom delta. Recording the after state with the same probe that produced
-- the before state makes the comparison like-for-like.
ALTER TABLE media_file_loudness ADD COLUMN codec_after varchar(32) NOT NULL DEFAULT '';
ALTER TABLE media_file_loudness ADD COLUMN bitrate_after integer NOT NULL DEFAULT 0;
ALTER TABLE media_file_loudness ADD COLUMN sample_rate_after integer NOT NULL DEFAULT 0;
ALTER TABLE media_file_loudness ADD COLUMN bit_depth_after integer NOT NULL DEFAULT 0;
ALTER TABLE media_file_loudness ADD COLUMN channels_after integer NOT NULL DEFAULT 0;
ALTER TABLE media_file_loudness ADD COLUMN duration_after real NOT NULL DEFAULT 0;
ALTER TABLE media_file_loudness ADD COLUMN size_after integer NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE media_file_loudness DROP COLUMN codec_after;
ALTER TABLE media_file_loudness DROP COLUMN bitrate_after;
ALTER TABLE media_file_loudness DROP COLUMN sample_rate_after;
ALTER TABLE media_file_loudness DROP COLUMN bit_depth_after;
ALTER TABLE media_file_loudness DROP COLUMN channels_after;
ALTER TABLE media_file_loudness DROP COLUMN duration_after;
ALTER TABLE media_file_loudness DROP COLUMN size_after;
