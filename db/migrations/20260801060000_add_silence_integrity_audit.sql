-- +goose Up
ALTER TABLE media_file_silence_backup ADD COLUMN integrity_status varchar(32) NOT NULL DEFAULT '';
ALTER TABLE media_file_silence_backup ADD COLUMN integrity_backup_verified boolean NOT NULL DEFAULT 0;
ALTER TABLE media_file_silence_backup ADD COLUMN integrity_decode_verified boolean NOT NULL DEFAULT 0;
ALTER TABLE media_file_silence_backup ADD COLUMN integrity_audio_verified boolean NOT NULL DEFAULT 0;
ALTER TABLE media_file_silence_backup ADD COLUMN integrity_metadata_verified boolean NOT NULL DEFAULT 0;
ALTER TABLE media_file_silence_backup ADD COLUMN integrity_artwork_verified boolean NOT NULL DEFAULT 0;
ALTER TABLE media_file_silence_backup ADD COLUMN integrity_packet_verified boolean NOT NULL DEFAULT 0;
ALTER TABLE media_file_silence_backup ADD COLUMN integrity_restore_verified boolean NOT NULL DEFAULT 0;
ALTER TABLE media_file_silence_backup ADD COLUMN integrity_error varchar(1024) NOT NULL DEFAULT '';
ALTER TABLE media_file_silence_backup ADD COLUMN integrity_verified_at datetime;

-- +goose Down
-- SQLite column removal is not supported safely across all versions.
