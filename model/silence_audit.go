package model

import "time"

// Silence audit lifecycle states.
const (
	SilenceStatusAnalyzed = "analyzed" // measured, file not modified
	SilenceStatusTrimmed  = "trimmed"  // file was cut
	SilenceStatusFailed   = "failed"
)

// What the analysis concluded about a track.
const (
	// SilenceVerdictClean: nothing worth removing at either end.
	SilenceVerdictClean = "clean"
	// SilenceVerdictTrimmable: silence found and safe to cut.
	SilenceVerdictTrimmable = "trimmable"
	// SilenceVerdictSkipped: silence found but a guard refused it. SkipReason
	// says which one.
	SilenceVerdictSkipped = "skipped"
	SilenceVerdictFailed  = "failed"
)

// Why a track with detectable silence was left alone.
//
// These are not errors. Each one is a case where cutting would remove something
// that is not silence, and the whole point of storing them is that "skipped" on
// its own tells the client nothing about whether the track needs attention.
const (
	// SilenceSkipFade: the level ramps into the music rather than starting
	// sharply, so the detected boundary sits somewhere inside a fade-in (or
	// fade-out) rather than at its edge. Cutting to it removes part of the fade.
	SilenceSkipFade = "fade"
	// SilenceSkipTooLong: more silence than any real lead-in or run-out, which
	// usually means a hidden track or a mis-tagged file rather than dead air.
	SilenceSkipTooLong = "too_long"
	// SilenceSkipGapless: the track abuts its neighbour on a continuous album,
	// where the gap between tracks is part of the recording.
	SilenceSkipGapless = "gapless"
	// SilenceSkipUnsupported: the codec has no honest way to be cut here.
	SilenceSkipUnsupported = "unsupported"
	// SilenceSkipTooShort: the silence is under the margin, so cutting it would
	// remove less than the margin the client asked to keep - i.e. nothing.
	SilenceSkipTooShort = "too_short"
)

// How the cut was performed.
const (
	// SilenceMethodCopy rewraps the existing compressed frames without decoding.
	// The retained audio is bit-identical to the source; tags and embedded cover
	// art carry over untouched.
	SilenceMethodCopy = "copy"
	// SilenceMethodEncode decodes and re-encodes. Used only where copying cannot
	// produce an honest file - notably FLAC, whose STREAMINFO keeps the original
	// sample count when copied, and Ogg/Opus, whose page granularity puts the
	// real length far from the requested cut.
	SilenceMethodEncode = "encode"
)

// SilenceAudit is the record of what one media file's head and tail looked like
// and what was removed from them.
type SilenceAudit struct {
	MediaFileID string `structs:"media_file_id" json:"mediaFileId"`

	Status  string `structs:"status" json:"status"`
	Verdict string `structs:"verdict" json:"verdict"`

	// LeadSilence/TrailSilence is what the detector found, in seconds.
	LeadSilence  float64 `structs:"lead_silence" json:"leadSilence"`
	TrailSilence float64 `structs:"trail_silence" json:"trailSilence"`

	// LeadTrim/TrailTrim is what will actually be removed once the margin is
	// kept back. Zero when a guard refused the cut.
	LeadTrim  float64 `structs:"lead_trim" json:"leadTrim"`
	TrailTrim float64 `structs:"trail_trim" json:"trailTrim"`

	// Onset gaps are the distance between the two detection thresholds. Small
	// means a sharp start; large means a fade. See SilenceSkipFade.
	LeadOnsetGap  float64 `structs:"lead_onset_gap" json:"leadOnsetGap"`
	TrailOnsetGap float64 `structs:"trail_onset_gap" json:"trailOnsetGap"`

	SkipReason string `structs:"skip_reason" json:"skipReason,omitempty"`
	Method     string `structs:"method" json:"method,omitempty"`

	Codec          string  `structs:"codec" json:"codec"`
	DurationBefore float64 `structs:"duration_before" json:"durationBefore"`
	DurationAfter  float64 `structs:"duration_after" json:"durationAfter"`
	SizeBefore     int64   `structs:"size_before" json:"sizeBefore"`
	SizeAfter      int64   `structs:"size_after" json:"sizeAfter"`

	Gapless bool `structs:"gapless" json:"gapless"`

	Error      string     `structs:"error" json:"error,omitempty"`
	AnalyzedAt time.Time  `structs:"analyzed_at" json:"analyzedAt"`
	TrimmedAt  *time.Time `structs:"trimmed_at" json:"trimmedAt,omitempty"`
}

// TotalTrim is how much time the plan removes from the track in total. This is
// the number the page leads with, because it is the one thing the client asked
// for: how many seconds come off this song.
func (a *SilenceAudit) TotalTrim() float64 { return a.LeadTrim + a.TrailTrim }

// TotalSilence is how much silence was found, whether or not it is being cut.
func (a *SilenceAudit) TotalSilence() float64 { return a.LeadSilence + a.TrailSilence }

// IsTrimmed reports whether the file on disk has already been cut. A trimmed
// track is left alone by later runs: its remaining head and tail are the margin
// that was deliberately kept, and re-running would eat the margin, then eat it
// again on the next run, until the music itself was reached.
func (a *SilenceAudit) IsTrimmed() bool { return a.TrimmedAt != nil }

type SilenceAuditRepository interface {
	Put(audit *SilenceAudit) error
	Get(mediaFileID string) (*SilenceAudit, error)
	Clear() (int64, error)
	// CountByVerdict returns how many rows carry each verdict, for the summary
	// header. Counting in the database rather than over a page of results is
	// what lets the header describe the library instead of the current page.
	CountByVerdict() (map[string]int64, error)
	// PendingTrimSeconds totals the trim planned across every trimmable track
	// that has not been cut yet.
	PendingTrimSeconds() (float64, error)
}
