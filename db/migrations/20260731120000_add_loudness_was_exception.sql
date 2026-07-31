-- +goose Up
-- The exceptions list was selecting on action = 'limited', which used to mean
-- "this track needed unusual handling". It no longer does: a small automatic
-- peak trim (PhaseTrim) is now a routine outcome and also records 'limited', so
-- ordinary successes were being listed alongside genuine exceptions.
--
-- Which tracks were exceptional is a fact about their history, not about their
-- current state, and no combination of the live columns can express it: once a
-- refused track is successfully reprocessed, every trace that it ever gave
-- trouble is gone. This column remembers it. It is a latch - set when a track
-- first needs a decision or is refused, never cleared - so the list stays a
-- permanent record the admin can come back to.
ALTER TABLE media_file_loudness ADD COLUMN was_exception bool NOT NULL DEFAULT false;

-- Backfill from the history still visible in the existing rows.
UPDATE media_file_loudness
   SET was_exception = true
 WHERE phase = 2
    OR action = 'refused'
    OR decision <> '';

CREATE INDEX IF NOT EXISTS media_file_loudness_was_exception
    ON media_file_loudness (was_exception);

-- +goose Down
DROP INDEX IF EXISTS media_file_loudness_was_exception;
ALTER TABLE media_file_loudness DROP COLUMN was_exception;
