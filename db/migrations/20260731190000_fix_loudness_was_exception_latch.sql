-- +goose Up
-- The exception latch was being raised by action = 'refused', which is not a
-- verdict about a track but a note to the next run: "this exact file was built
-- and thrown away, do not build it again". A fresh analysis clears it by design.
--
-- Latching a permanent flag off a deliberately temporary mark branded tracks on
-- the strength of one bad attempt - a run that failed while the disk was full,
-- or an attempt made before the track was restored - and nothing could clear it
-- afterwards. Tracks that are now perfectly on target were stuck on the
-- exceptions list with no way off.
--
-- Recomputed from what the rows still show. History that only existed in the
-- flag itself cannot be recovered, and guessing would keep the wrong answers, so
-- the latch is rebuilt from the two conditions that genuinely mean a human was
-- needed: the track is waiting on a decision, or a decision was made about it.
UPDATE media_file_loudness
   SET was_exception = (phase = 2 OR decision <> '');

-- +goose Down
-- The previous rule, restored as closely as the surviving columns allow.
UPDATE media_file_loudness
   SET was_exception = (phase = 2 OR decision <> '' OR action = 'refused');
