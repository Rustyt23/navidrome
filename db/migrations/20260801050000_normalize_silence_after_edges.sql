-- +goose Up
-- Older builds stored remux/encoder padding in the after-trim columns even
-- when that edge was never trimmed. The backup audit is authoritative about
-- which sides changed, so clear those misleading measurements.
UPDATE media_file_silence
   SET leading_silence_after = 0
 WHERE status = 'trimmed'
   AND EXISTS (
       SELECT 1
         FROM media_file_silence_backup
        WHERE media_file_silence_backup.media_file_id = media_file_silence.media_file_id
          AND media_file_silence_backup.trim_start <= 0
   );

UPDATE media_file_silence
   SET trailing_silence_after = 0
 WHERE status = 'trimmed'
   AND EXISTS (
       SELECT 1
         FROM media_file_silence_backup
        WHERE media_file_silence_backup.media_file_id = media_file_silence.media_file_id
          AND media_file_silence_backup.trim_end <= 0
   );

-- +goose Down
-- The previous padding measurements cannot be reconstructed safely.
