package model

import "time"

const (
	SilenceStatusAnalyzed = "analyzed"
	SilenceStatusTrimmed  = "trimmed"
	SilenceStatusFailed   = "failed"
)

// SilenceAnalysis stores derived, read-only measurements for one media file.
// A nil silence value means that edge was not measured; a pointer to zero means
// the file was measured and no qualifying silence was found at that edge.
type SilenceAnalysis struct {
	MediaFileID string `structs:"media_file_id" json:"mediaFileId"`

	LeadingSilence       *float64 `structs:"leading_silence" json:"leadingSilence"`
	TrailingSilence      *float64 `structs:"trailing_silence" json:"trailingSilence"`
	LeadingSilenceAfter  *float64 `structs:"leading_silence_after" json:"leadingSilenceAfter"`
	TrailingSilenceAfter *float64 `structs:"trailing_silence_after" json:"trailingSilenceAfter"`
	ThresholdDB          float64  `structs:"threshold_db" json:"thresholdDb"`
	MinimumSilence       float64  `structs:"minimum_silence" json:"minimumSilence"`

	SourceSize      int64     `structs:"source_size" json:"sourceSize"`
	SourceUpdatedAt time.Time `structs:"source_updated_at" json:"sourceUpdatedAt"`
	Status          string    `structs:"status" json:"status"`
	Error           string    `structs:"error" json:"error,omitempty"`
	AnalyzedAt      time.Time `structs:"analyzed_at" json:"analyzedAt"`
}

type SilenceAnalysisRepository interface {
	Put(analysis *SilenceAnalysis) error
	Get(mediaFileID string) (*SilenceAnalysis, error)
}
