package silence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/filelock"
)

// ErrNothingToTrim is returned when a track's stored plan says there is nothing
// to remove. Not a failure: it is the answer for most of a tidy library.
var ErrNothingToTrim = errors.New("nothing to trim")

// ErrAlreadyTrimmed is returned for a track that has already been cut.
//
// Trimming twice is the one mistake this feature can make that quietly destroys
// audio. After the first cut, what remains at each end IS the margin - exactly
// the half-second that was deliberately kept - so a second pass measures it,
// finds it under the margin and leaves it alone. That is the safe path working.
// This check is the belt to that braces: it refuses on the record rather than
// relying on the measurement coming out right every time.
var ErrAlreadyTrimmed = errors.New("this song has already been trimmed")

// TrimResult reports what happened to one track.
type TrimResult struct {
	Trimmed        bool
	LeadTrim       float64
	TrailTrim      float64
	Method         string
	DurationBefore float64
	DurationAfter  float64
	SizeBefore     int64
	SizeAfter      int64
}

// Trim cuts one track according to the plan its analysis recorded.
//
// There is no backup. The client keeps their own copies, and a trim is
// therefore final - which is why the analysis step, the guards and the
// post-trim verification all sit in front of this, and why an already-trimmed
// track is refused outright.
func Trim(ctx context.Context, trimmer ffmpeg.SilenceTrimmer, mf *model.MediaFile,
	audit *model.SilenceAudit) (*model.SilenceAudit, TrimResult, error) {

	var res TrimResult
	if audit == nil {
		return nil, res, fmt.Errorf("this song has not been analysed yet")
	}
	if audit.IsTrimmed() {
		return audit, res, ErrAlreadyTrimmed
	}

	plan := PlanFromAudit(audit)
	if !plan.ShouldTrim() {
		return audit, res, ErrNothingToTrim
	}

	// The stored path is library-relative; resolve it before locking, so the
	// lock is held on the file that is actually about to be rewritten.
	trackPath := TrackPath(mf)

	unlock := filelock.Lock(trackPath)
	defer unlock()

	probe, err := ffmpeg.ProbeFile(ctx, trackPath)
	if err != nil {
		return recordFailure(audit, err), res, err
	}

	// The file on disk must still be the one that was measured. A rescan, a
	// re-rip or another tool touching it between analysis and trim would make
	// the stored cut points describe audio that is no longer there, and cutting
	// to them would take music off the front. Re-analysing is a cheap fix for
	// the client; cutting the wrong seconds is not.
	if !durationsAgree(probe.Duration, audit.DurationBefore) {
		err := fmt.Errorf(
			"this song has changed since it was analysed (%.2fs now, %.2fs when measured) - analyse it again",
			probe.Duration, audit.DurationBefore)
		return recordFailure(audit, err), res, err
	}

	spec := ffmpeg.TrimSpec{
		StartSeconds: plan.StartSeconds,
		EndSeconds:   plan.EndSeconds,
		Source:       probe,
	}
	trimmed, err := trimmer.TrimSilence(ctx, trackPath, spec)
	if err != nil {
		return recordFailure(audit, err), res, err
	}

	now := time.Now()
	audit.Status = model.SilenceStatusTrimmed
	audit.Method = trimmed.Method
	audit.DurationBefore = probe.Duration
	audit.DurationAfter = trimmed.DurationAfter
	audit.SizeBefore = probe.Size
	audit.SizeAfter = trimmed.SizeAfter
	audit.TrimmedAt = &now
	audit.Error = ""

	res = TrimResult{
		Trimmed:        true,
		LeadTrim:       audit.LeadTrim,
		TrailTrim:      audit.TrailTrim,
		Method:         trimmed.Method,
		DurationBefore: probe.Duration,
		DurationAfter:  trimmed.DurationAfter,
		SizeBefore:     probe.Size,
		SizeAfter:      trimmed.SizeAfter,
	}
	return audit, res, nil
}

// durationsAgree allows for the difference between the scanner's metadata and
// ffprobe's measurement, which disagree by a few tens of milliseconds on the
// same untouched file.
func durationsAgree(a, b float64) bool {
	const tolerance = 0.5
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff <= tolerance
}

func recordFailure(audit *model.SilenceAudit, err error) *model.SilenceAudit {
	audit.Status = model.SilenceStatusFailed
	audit.Verdict = model.SilenceVerdictFailed
	audit.Error = err.Error()
	return audit
}
