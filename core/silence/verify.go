package silence

import (
	"context"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"
)

// VerifyInaudible checks that the audio a plan would remove really is nothing.
//
// This is the safety gate, and it is a measurement rather than an inference.
// The planner decides what to cut from silencedetect's report; this then takes
// the exact stretches that would go and measures their loudest sample with a
// different tool. Only if both agree that there is nothing there does the trim
// stand.
//
// It replaces a heuristic that tried to reason about the SHAPE of the audio -
// how gradually it arrived - and drew the wrong conclusion from it on real
// music. Measuring what is actually about to be deleted cannot be wrong in that
// way: whatever shape the fade has, if the loudest sample in the removed region
// is below the threshold, removing it changes nothing anybody can hear.
//
// Only called for tracks that would actually be cut, so the extra decode is
// paid on a handful of tracks rather than the whole library - and it decodes
// only the seconds being removed, not the file.
func VerifyInaudible(ctx context.Context, measurer ffmpeg.PeakMeasurer, trackPath string,
	plan Plan) (Plan, float64, error) {

	loudest := ffmpegNegInfinity
	if !plan.ShouldTrim() {
		return plan, loudest, nil
	}

	if plan.LeadTrim > 0 {
		peak, err := measurer.MeasurePeakDB(ctx, trackPath,
			ffmpeg.PeakRegion{Seconds: plan.LeadTrim})
		if err != nil {
			return plan, loudest, err
		}
		loudest = max(loudest, peak)
	}
	if plan.TrailTrim > 0 {
		peak, err := measurer.MeasurePeakDB(ctx, trackPath,
			ffmpeg.PeakRegion{Seconds: plan.TrailTrim, FromEnd: true})
		if err != nil {
			return plan, loudest, err
		}
		loudest = max(loudest, peak)
	}

	if loudest > ffmpeg.SilentPeakDB {
		// The two tools disagree: silencedetect placed the boundary somewhere
		// that volumedetect can still hear. Refusing is the only safe reading -
		// one of the measurements is wrong and there is no way to tell which.
		plan.Verdict = model.SilenceVerdictSkipped
		plan.SkipReason = model.SilenceSkipAudible
		plan.LeadTrim = 0
		plan.TrailTrim = 0
	}
	return plan, loudest, nil
}

// ffmpegNegInfinity mirrors the "nothing there" value the measurer returns, so
// a plan with nothing to check reports silence rather than zero - which would
// read as full scale.
const ffmpegNegInfinity = -200.0
