package silence

import (
	"context"
	"time"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"
)

// Analyze measures one track's head and tail and records what it found and what
// it would do. It never modifies the audio file.
//
// Analysis and trimming are deliberately separate steps. The client sees how
// many seconds every song would lose, and on which songs the guards refused,
// before anything is cut - which is the whole reason the page is worth having
// rather than a single "remove silence" button.
func Analyze(ctx context.Context, detector ffmpeg.SilenceDetector, measurer ffmpeg.PeakMeasurer,
	mf *model.MediaFile, gapless bool, opts Options) (*model.SilenceAudit, error) {

	audit := &model.SilenceAudit{
		MediaFileID: mf.ID,
		Status:      model.SilenceStatusAnalyzed,
		AnalyzedAt:  time.Now(),
		Gapless:     gapless,
	}

	// The stored path is library-relative, so it has to be resolved before
	// anything tries to open it.
	trackPath := TrackPath(mf)

	probe, err := ffmpeg.ProbeFile(ctx, trackPath)
	if err != nil {
		return failedAudit(audit, err), err
	}
	audit.Codec = probe.Codec
	audit.DurationBefore = probe.Duration
	audit.SizeBefore = probe.Size

	report, err := detector.DetectSilence(ctx, trackPath, ffmpeg.SilenceDetectOptions{Duration: probe.Duration})
	if err != nil {
		return failedAudit(audit, err), err
	}

	audit.LeadSilence = report.LeadSilence
	audit.TrailSilence = report.TrailSilence
	audit.LeadOnsetGap = report.LeadOnsetGap
	audit.TrailOnsetGap = report.TrailOnsetGap

	plan := BuildPlan(report, probe, gapless, opts)

	// Nothing is trusted to the detector alone. Where the plan would actually
	// remove audio, the stretch coming off is measured with a level meter and
	// the trim only stands if there is provably nothing in it.
	if measurer != nil && plan.ShouldTrim() {
		verified, peak, err := VerifyInaudible(ctx, measurer, trackPath, plan)
		if err != nil {
			return failedAudit(audit, err), err
		}
		plan = verified
		audit.RemovedPeakDB = &peak
	}

	audit.LeadTrim = plan.LeadTrim
	audit.TrailTrim = plan.TrailTrim
	audit.Verdict = plan.Verdict
	audit.SkipReason = plan.SkipReason
	audit.Method = plan.Method

	return audit, nil
}

func failedAudit(audit *model.SilenceAudit, err error) *model.SilenceAudit {
	audit.Status = model.SilenceStatusFailed
	audit.Verdict = model.SilenceVerdictFailed
	audit.Error = err.Error()
	return audit
}

// PlanFromAudit rebuilds the cut points from a stored analysis.
//
// The trim step reads the plan the analysis recorded rather than re-measuring.
// Re-measuring would be a second full decode of every track for an answer
// already on file, and worse, it would let the two steps disagree: the client
// would approve the seconds shown on the page and a different number would come
// off. What was shown is what gets cut.
func PlanFromAudit(audit *model.SilenceAudit) Plan {
	if audit == nil {
		return Plan{Verdict: model.SilenceVerdictClean}
	}
	plan := Plan{
		LeadTrim:   audit.LeadTrim,
		TrailTrim:  audit.TrailTrim,
		Verdict:    audit.Verdict,
		SkipReason: audit.SkipReason,
		Method:     audit.Method,
	}
	plan.StartSeconds = audit.LeadTrim
	plan.EndSeconds = audit.DurationBefore - audit.TrailTrim
	return plan
}
