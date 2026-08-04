-- +goose Up
-- Restoring a song put its original back and then left it looking exactly like
-- a song that had never been processed - which is what it is. The next library
-- sweep therefore picked it up and normalized it again, silently undoing the
-- restore, and would do so after every restore for ever.
--
-- Nothing in the live columns could express the difference. A restored song and
-- an untouched one measure the same, plan the same and sit in the same phase;
-- the only thing that separates them is that a person asked for one of them to
-- be left as it is. That is what this column records.
--
-- It is not a latch. A fresh analysis clears it, because re-analysing a song is
-- someone saying "look at this again", and picking a song out by hand overrides
-- it too - both are explicit instructions that outrank a standing preference.
ALTER TABLE media_file_loudness ADD COLUMN restored_at DATETIME;

CREATE INDEX IF NOT EXISTS media_file_loudness_restored_at
    ON media_file_loudness (restored_at);

-- +goose Down
DROP INDEX IF EXISTS media_file_loudness_restored_at;
ALTER TABLE media_file_loudness DROP COLUMN restored_at;
