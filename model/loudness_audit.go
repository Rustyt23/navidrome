package model

import (
	"errors"
	"time"
)

// Loudness audit lifecycle states.
const (
	LoudnessStatusAnalyzed  = "analyzed"  // measured, file not modified
	LoudnessStatusProcessed = "processed" // file was rewritten by normalization
	LoudnessStatusFailed    = "failed"
)

// Quality verdicts. Ordered from best to worst; the UI colours them accordingly.
const (
	LoudnessVerdictUntouched       = "untouched"        // already in range, file never rewritten
	LoudnessVerdictSafe            = "safe"             // only the level changed
	LoudnessVerdictDynamicsChanged = "dynamics_changed" // format intact, but the audio itself was reshaped
	// LoudnessVerdictRewriteCostly: the format survived and nothing reshaped the
	// audio, but the null test found more difference from the original than
	// re-encoding this format should account for.
	//
	// It is deliberately separate from "dynamics changed". The null test measures
	// HOW MUCH of the file differs, never WHAT about it differs - only the
	// loudness range and the true peak can say the dynamics were touched. Routing
	// a large leftover into the dynamics verdict claimed the peaks had been
	// trimmed on tracks where nothing had trimmed them, which is a specific
	// accusation the measurement cannot support.
	LoudnessVerdictRewriteCostly = "rewrite_costly"
	LoudnessVerdictReencoded     = "reencoded" // codec/bitrate/rate/depth/channels/art degraded
	LoudnessVerdictFailed        = "failed"
	// LoudnessVerdictNoAudio: the file holds no playable audio at all - almost
	// always a download that was truncated or never finished. Kept apart from
	// "failed" because the two need different answers: a failure may be worth
	// retrying, an empty file never is. No amount of re-running fixes it; the
	// song has to be fetched again from its source.
	LoudnessVerdictNoAudio = "no_audio"
)

// How the loudness change was applied.
const (
	LoudnessActionGain    = "gain"    // constant gain only (linear)
	LoudnessActionLimited = "limited" // gain plus peak limiting / dynamic processing
	LoudnessActionSkipped = "skipped" // nothing applied
	// LoudnessActionRefused: a result was produced and thrown away because it
	// was not good enough to ship. The file is untouched. Recorded so a run does
	// not keep rebuilding the same rejected file on every pass; a fresh analysis
	// clears it and the track is tried again.
	LoudnessActionRefused = "refused"
)

// LoudnessPhaseReview mirrors loudness.PhaseReview: the stage where the target
// cannot be reached without an audible change and the client has to choose.
// The value is repeated here because core/loudness imports this package, so it
// cannot be imported back. core/loudness asserts the two stay equal.
const LoudnessPhaseReview = 2

// IsException reports whether this record describes a track that needed a human
// to look at it. It is the condition that raises WasException; the stored latch
// is what keeps the answer true afterwards.
//
// Being refused is deliberately not one of these. Refusal is a note to the next
// run - "this exact file was built and thrown away, do not build it again" -
// which a fresh analysis clears by design. Latching a permanent flag off a mark
// the system clears on purpose brands a track for ever on the strength of one
// bad attempt: a run that failed because the disk was full, or an attempt made
// before the track was restored, is enough. Refusal still puts a track on the
// exceptions list through the live filter, and takes it off again once it is no
// longer refused, which is the behaviour a temporary mark should have.
func (a *LoudnessAudit) IsException() bool {
	return a.Phase == LoudnessPhaseReview || a.Decision != ""
}

// LoudnessAudit is the before/after record for one media file. It lives in its
// own table so that a library rescan - which rebuilds media_file rows from the
// files on disk - cannot erase the "before" snapshot, which is unrecoverable
// once the original file has been overwritten and its backup deleted.
type LoudnessAudit struct {
	MediaFileID string `structs:"media_file_id" json:"mediaFileId"`

	Status  string `structs:"status" json:"status"`
	Verdict string `structs:"verdict" json:"verdict"`
	Action  string `structs:"action" json:"action"`

	// Phase is which stage of the process this track belongs to: 1 when the
	// target is reachable by a constant gain (optimised automatically), 2 when
	// it is not and the client has to choose. -1 means not yet planned.
	Phase int `structs:"phase" json:"phase"`
	// Decision is the client's choice for a phase 2 track.
	Decision string `structs:"decision" json:"decision"`
	// WasException records that this track needed a decision or was refused at
	// some point. It is a latch: once raised it is never lowered, so a track
	// that has since been dealt with stays on the exceptions list instead of
	// disappearing the moment it stops being a problem. Nothing in the live
	// columns can stand in for it - a refused track that is later reprocessed
	// successfully looks identical to one that never gave any trouble.
	WasException bool `structs:"was_exception" json:"wasException"`

	// Field names must stay in this CamelCase form: the DB layer maps them to
	// snake_case columns by lower->upper transitions, so LUFSBefore would map
	// to "lufsbefore" instead of "lufs_before".
	LufsBefore   *float64 `structs:"lufs_before" json:"lufsBefore"`
	LufsAfter    *float64 `structs:"lufs_after" json:"lufsAfter"`
	GainApplied  *float64 `structs:"gain_applied" json:"gainApplied"`
	TpBefore     *float64 `structs:"tp_before" json:"tpBefore"`
	TpAfter      *float64 `structs:"tp_after" json:"tpAfter"`
	LraBefore    *float64 `structs:"lra_before" json:"lraBefore"`
	LraAfter     *float64 `structs:"lra_after" json:"lraAfter"`
	NullResidual *float64 `structs:"null_residual" json:"nullResidual"`

	CodecBefore      string  `structs:"codec_before" json:"codecBefore"`
	BitrateBefore    int     `structs:"bitrate_before" json:"bitrateBefore"`
	SampleRateBefore int     `structs:"sample_rate_before" json:"sampleRateBefore"`
	BitDepthBefore   int     `structs:"bit_depth_before" json:"bitDepthBefore"`
	ChannelsBefore   int     `structs:"channels_before" json:"channelsBefore"`
	DurationBefore   float64 `structs:"duration_before" json:"durationBefore"`
	SizeBefore       int64   `structs:"size_before" json:"sizeBefore"`
	ArtBefore        bool    `structs:"art_before" json:"artBefore"`
	ArtAfter         bool    `structs:"art_after" json:"artAfter"`

	// The after snapshot is measured with the same probe as the before
	// snapshot. Reading it from media_file instead would compare against the
	// scanner's own metadata extractor, which reports a slightly different
	// duration and would make every rewrite look like it shifted the track.
	CodecAfter      string  `structs:"codec_after" json:"codecAfter"`
	BitrateAfter    int     `structs:"bitrate_after" json:"bitrateAfter"`
	SampleRateAfter int     `structs:"sample_rate_after" json:"sampleRateAfter"`
	BitDepthAfter   int     `structs:"bit_depth_after" json:"bitDepthAfter"`
	ChannelsAfter   int     `structs:"channels_after" json:"channelsAfter"`
	DurationAfter   float64 `structs:"duration_after" json:"durationAfter"`
	SizeAfter       int64   `structs:"size_after" json:"sizeAfter"`
	HasBackup       bool    `structs:"has_backup" json:"hasBackup"`

	Error      string    `structs:"error" json:"error,omitempty"`
	AnalyzedAt time.Time `structs:"analyzed_at" json:"analyzedAt"`

	// RestoredAt is when the client's original was last put back.
	//
	// A restored song is left alone by library sweeps. Restoring is someone
	// saying "I want the original"; a sweep that normalized it again an hour
	// later would undo that silently, and would keep doing so after every
	// restore. Nothing else in the record can tell a restored song from one that
	// was never touched - they measure the same and plan the same - so the fact
	// that a person asked has to be written down.
	//
	// Not a latch: a fresh analysis clears it, and picking the song out by hand
	// overrides it. Both are explicit instructions that outrank it.
	RestoredAt *time.Time `structs:"restored_at" json:"restoredAt,omitempty"`
}

// IsRestored reports whether the client asked for this song's original back and
// has not since asked for it to be looked at again.
func (a *LoudnessAudit) IsRestored() bool { return a.RestoredAt != nil }

// ErrNoLoudnessAuditData is returned instead of writing a copy of an empty
// audit table. There is nothing to protect, and storing one would count against
// however many copies are kept - so a handful of them, taken after a cleared
// table or a run that measured nothing, would quietly evict every copy that did
// hold something.
var ErrNoLoudnessAuditData = errors.New("there is no LUFS audit data to copy yet")

// LoudnessSnapshot describes one stored copy of the loudness audit table.
//
// The copy is a SQLite database of its own, kept outside the application's data
// directory. What was done to the client's audio is the one record here that
// cannot be rebuilt cheaply - re-deriving it means decoding every song and its
// stored original again - and the client's phase 2 decisions cannot be rebuilt
// at all, because nothing but a person can produce them.
type LoudnessSnapshot struct {
	File      string    `json:"file"`
	Path      string    `json:"path"`
	Rows      int64     `json:"rows"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

// LoudnessRestoreReport accounts for every row a restore touched, so the result
// can be checked rather than trusted.
type LoudnessRestoreReport struct {
	File string `json:"file"`
	// Rows is how many the snapshot held.
	Rows int64 `json:"rows"`
	// Restored is how many were written back.
	Restored int64 `json:"restored"`
	// Skipped is snapshot rows whose song is not in this library. Their audio is
	// gone or was never here, so there is nothing for the record to describe.
	Skipped int64 `json:"skipped"`
	// Removed is rows deleted because the snapshot did not have them - the
	// "exact replace" half of the restore.
	Removed int64 `json:"removed"`
}

type LoudnessAuditRepository interface {
	Put(audit *LoudnessAudit) error
	Get(mediaFileID string) (*LoudnessAudit, error)
	SetDecision(mediaFileID, decision string) error
	Clear() (int64, error)

	// Snapshot writes the current audit table to its own database file in dir,
	// keeping at most `keep` of them.
	Snapshot(dir string, keep int) (*LoudnessSnapshot, error)
	// Snapshots lists what is stored in dir, newest first.
	Snapshots(dir string) ([]LoudnessSnapshot, error)
	// RestoreSnapshot makes the audit table match the named snapshot exactly.
	// It touches no other table.
	RestoreSnapshot(dir, file string) (*LoudnessRestoreReport, error)
}
