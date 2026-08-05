package silence

import (
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"
)

const (
	// DefaultMarginSeconds is how much quiet is deliberately left in place at
	// each end.
	//
	// The client asked for a 0.5s margin, and it means keep: a song that opened
	// with four seconds of dead air opens with half a second afterwards, not
	// with the first note. Cutting all the way to the audio makes a track start
	// abruptly, and on a lossy source the cut lands on a frame boundary anyway,
	// so "exactly at the music" is not a thing that can be hit.
	DefaultMarginSeconds = 0.5

	// MaxOnsetGapSeconds is how far apart the two detection thresholds may sit
	// before the onset is treated as too gradual to act on.
	//
	// This used to be 10ms, and it was the wrong instrument. It was calibrated
	// on synthetic fixtures where a tone switches on and off instantly, and real
	// music does not end that way - it decays. On a real library it refused all
	// 23 tracks that had removable silence at the end, on the grounds that their
	// endings faded.
	//
	// The reason it was refusing them is that it was guarding against something
	// the margin already prevents. The cut is anchored to where the audio drops
	// below PrimaryThresholdDB and then pulled back by the margin, so the region
	// removed is by construction quieter than that - no matter how gradually the
	// audio arrives at it. Measured on the tracks it was blocking, the loudest
	// sample in the removed region sat well below the threshold either way.
	//
	// So this is now a backstop for the absurd rather than the working rule: it
	// catches a track whose two thresholds are so far apart that the measurement
	// itself is suspect. The real gate is VerifyInaudible, which measures the
	// region about to be removed instead of inferring anything about it.
	MaxOnsetGapSeconds = 5.0

	// MaxTrimSeconds is the most that will be taken off one end.
	//
	// Beyond this it is not dead air. Half a minute of silence at the head of a
	// track is a hidden track, a mis-split rip, or a file whose tags do not
	// describe its contents - all cases where a person should look rather than
	// have thirty seconds removed on the strength of a level measurement.
	MaxTrimSeconds = 30.0

	// MinRemainingSeconds is the shortest a track may be left. A cut that takes
	// a file below this has almost certainly measured something wrong, and the
	// result would not be a song.
	MinRemainingSeconds = 1.0
)

// Options are the knobs a run works under.
type Options struct {
	// Margin is how much silence to leave at each end.
	Margin float64
	// MaxOnsetGap overrides the fade guard's sensitivity.
	MaxOnsetGap float64
	// MaxTrim caps how much comes off one end.
	MaxTrim float64
	// AllowGapless trims tracks that sit on a continuous album seam. Off by
	// default: the gap between two tracks of a live set or a DJ mix is part of
	// the recording, and closing it is audible.
	AllowGapless bool
}

func (o Options) withDefaults() Options {
	if o.Margin <= 0 {
		o.Margin = DefaultMarginSeconds
	}
	if o.MaxOnsetGap <= 0 {
		o.MaxOnsetGap = MaxOnsetGapSeconds
	}
	if o.MaxTrim <= 0 {
		o.MaxTrim = MaxTrimSeconds
	}
	return o
}

// Plan is what a run decided to do to one track, before anything is written.
type Plan struct {
	// LeadTrim/TrailTrim is how much to remove from each end, after the margin
	// is kept back. Zero means that end is left alone.
	LeadTrim  float64
	TrailTrim float64

	// StartSeconds/EndSeconds are the resulting cut points in the source's
	// timeline, ready to hand to the trimmer.
	StartSeconds float64
	EndSeconds   float64

	// Verdict is what the page shows: clean, trimmable or skipped.
	Verdict string
	// SkipReason explains a skipped verdict.
	SkipReason string
	// Method is how the cut would be made, so the page can show which tracks
	// keep their bits exactly and which are re-encoded.
	Method string
}

// TotalTrim is how many seconds the plan removes in total.
func (p Plan) TotalTrim() float64 { return p.LeadTrim + p.TrailTrim }

// ShouldTrim reports whether there is anything to do.
func (p Plan) ShouldTrim() bool {
	return p.Verdict == model.SilenceVerdictTrimmable && p.TotalTrim() > 0
}

// BuildPlan turns a measurement into a decision.
//
// Every refusal is recorded as a reason rather than as a silent zero, because
// "nothing was trimmed" and "something was found and deliberately left" are
// different answers and the client needs to tell them apart. Nothing here
// touches a file.
func BuildPlan(report *ffmpeg.SilenceReport, probe *ffmpeg.FileProbe, gapless bool, opts Options) Plan {
	opts = opts.withDefaults()
	plan := Plan{Verdict: model.SilenceVerdictClean}
	if report == nil || probe == nil {
		return plan
	}

	duration := probe.Duration
	if duration <= 0 {
		duration = report.Duration
	}
	plan.EndSeconds = duration

	plan.Method = model.SilenceMethodEncode
	if canCopy(probe.Codec) {
		plan.Method = model.SilenceMethodCopy
	}

	// Nothing at either end worth acting on.
	if report.LeadSilence <= opts.Margin && report.TrailSilence <= opts.Margin {
		if report.LeadSilence > 0 || report.TrailSilence > 0 {
			plan.SkipReason = model.SilenceSkipTooShort
		}
		return plan
	}

	// A seam on a continuous album. Checked before the per-end guards because it
	// is a fact about the album, not about either end's shape, and it disquali-
	// fies the whole track.
	if gapless && !opts.AllowGapless {
		return skipped(plan, model.SilenceSkipGapless)
	}

	lead, leadSkip := trimForEnd(report.LeadSilence, report.LeadOnsetGap, opts)
	trail, trailSkip := trimForEnd(report.TrailSilence, report.TrailOnsetGap, opts)

	if lead <= 0 && trail <= 0 {
		// Both ends refused. Report the lead's reason where there is one - it is
		// the end anyone notices - and the tail's otherwise.
		reason := leadSkip
		if reason == "" {
			reason = trailSkip
		}
		if reason == "" {
			reason = model.SilenceSkipTooShort
		}
		return skipped(plan, reason)
	}

	start := lead
	end := duration - trail
	if end-start < MinRemainingSeconds {
		return skipped(plan, model.SilenceSkipTooLong)
	}

	plan.LeadTrim = lead
	plan.TrailTrim = trail
	plan.StartSeconds = start
	plan.EndSeconds = end
	plan.Verdict = model.SilenceVerdictTrimmable
	// One end may have been refused while the other was cut. The reason is kept
	// so the page can explain why only half the silence went.
	if leadSkip != "" && lead <= 0 {
		plan.SkipReason = leadSkip
	} else if trailSkip != "" && trail <= 0 {
		plan.SkipReason = trailSkip
	}
	return plan
}

// trimForEnd decides how much comes off one end, or why none does.
func trimForEnd(silence, onsetGap float64, opts Options) (float64, string) {
	if silence <= opts.Margin {
		return 0, ""
	}
	// The music arrives gradually, so the detected boundary is somewhere inside
	// a fade rather than at its edge. See MaxOnsetGapSeconds.
	if onsetGap > opts.MaxOnsetGap {
		return 0, model.SilenceSkipFade
	}
	if silence > opts.MaxTrim {
		return 0, model.SilenceSkipTooLong
	}
	return silence - opts.Margin, ""
}

func skipped(plan Plan, reason string) Plan {
	plan.Verdict = model.SilenceVerdictSkipped
	plan.SkipReason = reason
	plan.LeadTrim = 0
	plan.TrailTrim = 0
	return plan
}

// canCopy mirrors the trimmer's strategy table so the plan can report the
// method before anything runs. The trimmer decides for itself at cut time; this
// is for display, and the two are checked against each other by test.
func canCopy(codec string) bool { return ffmpeg.CanCopyCodec(codec) }
