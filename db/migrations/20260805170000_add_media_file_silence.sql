-- +goose Up
-- Silence trim keeps its own table rather than extending media_file_loudness.
-- The two features rewrite the same files but answer different questions, and a
-- shared table would mean a silence run and a LUFS run writing the same row.
--
-- Like the loudness audit, this lives outside media_file so a library rescan -
-- which rebuilds media_file rows from what is on disk - cannot erase the record
-- of what the head and tail looked like before they were cut off. Once a file
-- is trimmed the original silence is unrecoverable, so the measurement is the
-- only evidence left that the trim was correct.
CREATE TABLE IF NOT EXISTS media_file_silence
(
    media_file_id   varchar(255) NOT NULL PRIMARY KEY,
    status          varchar(32)  NOT NULL DEFAULT '',
    verdict         varchar(32)  NOT NULL DEFAULT '',

    -- What the detector found, in seconds from the start/end of the file.
    lead_silence    real         NOT NULL DEFAULT 0,
    trail_silence   real         NOT NULL DEFAULT 0,

    -- What the plan decided to actually remove, after the margin is kept back
    -- and the guards have had their say. Always <= the detected silence.
    lead_trim       real         NOT NULL DEFAULT 0,
    trail_trim      real         NOT NULL DEFAULT 0,

    -- Onset sharpness: the gap in seconds between the -60dB and -45dB detection
    -- boundaries. A real track start puts these ~1ms apart; a fade-in puts them
    -- hundreds of ms apart. This is what stops a fade being mistaken for silence
    -- and decapitated, so it is stored rather than recomputed - it is the reason
    -- a track was skipped, and without it "skipped" has no explanation.
    lead_onset_gap  real         NOT NULL DEFAULT 0,
    trail_onset_gap real         NOT NULL DEFAULT 0,

    -- Why a track with silence was not trimmed, when it was not.
    skip_reason     varchar(64)  NOT NULL DEFAULT '',

    -- How the cut was made: 'copy' rewraps the existing compressed frames and is
    -- bit-exact; 'encode' decodes and re-encodes, used only for formats whose
    -- container cannot be cut honestly by copying.
    method          varchar(16)  NOT NULL DEFAULT '',

    codec           varchar(32)  NOT NULL DEFAULT '',
    duration_before real         NOT NULL DEFAULT 0,
    duration_after  real         NOT NULL DEFAULT 0,
    size_before     integer      NOT NULL DEFAULT 0,
    size_after      integer      NOT NULL DEFAULT 0,

    -- Album-level continuity. A track whose tail runs straight into the next
    -- track's head belongs to a continuous album (a live set, a DJ mix, a
    -- classical movement), where trimming the seam is audible damage.
    gapless         bool         NOT NULL DEFAULT false,

    error           varchar(1024) NOT NULL DEFAULT '',
    analyzed_at     datetime,
    trimmed_at      datetime,
    FOREIGN KEY (media_file_id) REFERENCES media_file (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS media_file_silence_status ON media_file_silence (status);
CREATE INDEX IF NOT EXISTS media_file_silence_verdict ON media_file_silence (verdict);

-- +goose Down
DROP INDEX IF EXISTS media_file_silence_verdict;
DROP INDEX IF EXISTS media_file_silence_status;
DROP TABLE IF EXISTS media_file_silence;
