package model

import "time"

const (
	SilenceTrimStatusAnalyzed  = "analyzed"
	SilenceTrimStatusProcessed = "processed"
	SilenceTrimStatusFailed    = "failed"
)

const (
	SilenceTrimClassNone    = "none"
	SilenceTrimClassSafe    = "safe"
	SilenceTrimClassReview  = "review"
	SilenceTrimClassBlocked = "blocked"
)

const (
	SilenceTrimDecisionPending = ""
	SilenceTrimDecisionApprove = "approve"
	SilenceTrimDecisionSkip    = "skip"
)

const (
	SilenceTrimEdgeNone  = "none"
	SilenceTrimEdgeExact = "confirmed_silence"
	SilenceTrimEdgeQuiet = "near_silence"
)

const (
	SilenceTrimMethodLossless   = "lossless_sample_trim"
	SilenceTrimMethodPacketCopy = "frame_aligned_copy"
)

const (
	SilenceTrimIntegrityPending  = "pending"
	SilenceTrimIntegrityVerified = "verified"
	SilenceTrimIntegrityFailed   = "failed"
)

// SilenceTrimAudit is the proposal and before/after proof for one song.
// It is intentionally independent of LoudnessAudit: the two workflows answer
// different questions and have different expected duration rules.
type SilenceTrimAudit struct {
	MediaFileID string `structs:"media_file_id" json:"mediaFileId"`

	Status         string `structs:"status" json:"status"`
	Classification string `structs:"classification" json:"classification"`
	Decision       string `structs:"decision" json:"decision"`
	Reason         string `structs:"reason" json:"reason"`
	LeadingKind    string `structs:"leading_kind" json:"leadingKind"`
	TrailingKind   string `structs:"trailing_kind" json:"trailingKind"`

	LeadingSilence         float64 `structs:"leading_silence" json:"leadingSilence"`
	TrailingSilence        float64 `structs:"trailing_silence" json:"trailingSilence"`
	LeadingSamples         int64   `structs:"leading_samples" json:"leadingSamples"`
	TrailingSamples        int64   `structs:"trailing_samples" json:"trailingSamples"`
	ProposedStartTrim      float64 `structs:"proposed_start_trim" json:"proposedStartTrim"`
	ProposedEndTrim        float64 `structs:"proposed_end_trim" json:"proposedEndTrim"`
	ProposedStartSamples   int64   `structs:"proposed_start_samples" json:"proposedStartSamples"`
	ProposedEndSamples     int64   `structs:"proposed_end_samples" json:"proposedEndSamples"`
	AppliedStartTrim       float64 `structs:"applied_start_trim" json:"appliedStartTrim"`
	AppliedEndTrim         float64 `structs:"applied_end_trim" json:"appliedEndTrim"`
	AppliedStartSamples    int64   `structs:"applied_start_samples" json:"appliedStartSamples"`
	AppliedEndSamples      int64   `structs:"applied_end_samples" json:"appliedEndSamples"`
	RetainedPadding        float64 `structs:"retained_padding" json:"retainedPadding"`
	RetainedPaddingSamples int64   `structs:"retained_padding_samples" json:"retainedPaddingSamples"`
	Method                 string  `structs:"method" json:"method"`
	Integrity              string  `structs:"integrity" json:"integrity"`

	CodecBefore      string  `structs:"codec_before" json:"codecBefore"`
	CodecAfter       string  `structs:"codec_after" json:"codecAfter"`
	BitrateBefore    int     `structs:"bitrate_before" json:"bitrateBefore"`
	BitrateAfter     int     `structs:"bitrate_after" json:"bitrateAfter"`
	SampleRateBefore int     `structs:"sample_rate_before" json:"sampleRateBefore"`
	SampleRateAfter  int     `structs:"sample_rate_after" json:"sampleRateAfter"`
	BitDepthBefore   int     `structs:"bit_depth_before" json:"bitDepthBefore"`
	BitDepthAfter    int     `structs:"bit_depth_after" json:"bitDepthAfter"`
	ChannelsBefore   int     `structs:"channels_before" json:"channelsBefore"`
	ChannelsAfter    int     `structs:"channels_after" json:"channelsAfter"`
	DurationBefore   float64 `structs:"duration_before" json:"durationBefore"`
	DurationAfter    float64 `structs:"duration_after" json:"durationAfter"`
	SizeBefore       int64   `structs:"size_before" json:"sizeBefore"`
	SizeAfter        int64   `structs:"size_after" json:"sizeAfter"`
	ArtBefore        bool    `structs:"art_before" json:"artBefore"`
	ArtAfter         bool    `structs:"art_after" json:"artAfter"`
	HasBackup        bool    `structs:"has_backup" json:"hasBackup"`
	BackupSHA256     string  `structs:"backup_sha256" json:"backupSha256"`
	SourceSHA256     string  `structs:"source_sha256" json:"sourceSha256"`
	ResultSHA256     string  `structs:"result_sha256" json:"resultSha256"`

	Error            string     `structs:"error" json:"error,omitempty"`
	SourceModifiedAt *time.Time `structs:"source_modified_at" json:"sourceModifiedAt,omitempty"`
	ResultModifiedAt *time.Time `structs:"result_modified_at" json:"resultModifiedAt,omitempty"`
	AnalyzedAt       time.Time  `structs:"analyzed_at" json:"analyzedAt"`
	AppliedAt        *time.Time `structs:"applied_at" json:"appliedAt,omitempty"`
}

type SilenceTrimAuditRepository interface {
	Put(audit *SilenceTrimAudit) error
	Get(mediaFileID string) (*SilenceTrimAudit, error)
	SetDecision(mediaFileID, decision string) error
	Clear() (int64, error)
}
