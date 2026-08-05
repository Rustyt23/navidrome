-- +goose Up
-- The loudest sample in the stretch a trim would remove, measured with a level
-- meter rather than inferred from the silence detector.
--
-- Stored because it is the evidence the trim was safe. The original design
-- guessed at safety from the SHAPE of the fade - how gradually the audio
-- arrived - and got it wrong on real music, refusing every track whose ending
-- decayed rather than stopping dead. Recording what was actually measured means
-- the answer to "how do you know that was nothing?" is a number on the row.
--
-- 0 is not a sensible default for a dB value - it means full scale - so the
-- column is nullable and left NULL for rows written before it existed, and for
-- tracks where nothing was ever going to be removed.
ALTER TABLE media_file_silence ADD COLUMN removed_peak_db real;

-- +goose Down
ALTER TABLE media_file_silence DROP COLUMN removed_peak_db;
