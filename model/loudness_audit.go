package model

import "time"

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
	LoudnessVerdictReencoded       = "reencoded"        // codec/bitrate/rate/depth/channels/art degraded
	LoudnessVerdictFailed          = "failed"
)

// How the loudness change was applied.
const (
	LoudnessActionGain    = "gain"    // constant gain only (linear)
	LoudnessActionLimited = "limited" // gain plus peak limiting / dynamic processing
	LoudnessActionSkipped = "skipped" // nothing applied
)

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
	HasBackup        bool    `structs:"has_backup" json:"hasBackup"`

	Error      string    `structs:"error" json:"error,omitempty"`
	AnalyzedAt time.Time `structs:"analyzed_at" json:"analyzedAt"`
}

type LoudnessAuditRepository interface {
	Put(audit *LoudnessAudit) error
	Get(mediaFileID string) (*LoudnessAudit, error)
	SetDecision(mediaFileID, decision string) error
	Clear() (int64, error)
}
