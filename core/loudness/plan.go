package loudness

import (
	"math"

	"github.com/navidrome/navidrome/core/ffmpeg"
)

// Phases of the optimisation process.
const (
	// PhaseDone: already within tolerance of the target - nothing to do, the
	// file is never opened.
	PhaseDone = 0
	// PhaseGain: the target is reachable by a constant gain without pushing
	// the true peak past the ceiling. Mathematically transparent - only the
	// level changes. These are optimised automatically.
	PhaseGain = 1
	// PhaseReview: a constant gain would push the peaks over the ceiling.
	// Reaching the target requires reshaping the audio, so the client decides.
	PhaseReview = 2
)

// Decisions available for a PhaseReview track.
const (
	DecisionPending = ""             // client has not chosen yet
	DecisionLimit   = "limit"        // gain to target + peak limiting: hits -12.6, alters transients
	DecisionCeiling = "gain_ceiling" // largest transparent gain: nothing altered, lands short of target
	DecisionSkip    = "skip"         // leave the file untouched
)

// Plan is the decided outcome for one track: what would be done, and what the
// result would be. Everything is derived from two measurements (integrated
// loudness and true peak) plus the configured target and ceiling.
type Plan struct {
	Phase int

	// GainToTarget is the constant gain that lands exactly on target.
	GainToTarget float64
	// PredictedPeak is where the true peak ends up after GainToTarget. When
	// this exceeds the ceiling the track cannot be fixed transparently.
	PredictedPeak float64
	// PeakOverBy is how far past the ceiling PredictedPeak lands (0 when fine).
	PeakOverBy float64

	// TransparentGain is the largest gain that keeps the true peak at or under
	// the ceiling, and LoudnessAtCeiling is the loudness it produces.
	TransparentGain   float64
	LoudnessAtCeiling float64
	// Shortfall is how far LoudnessAtCeiling stays below the target.
	Shortfall float64

	// SafeGain is the gain a phase 1 correction actually applies: the gain to
	// target, held back to TransparentGain when that would put the peaks over
	// the ceiling. SafeLoudness is where it lands.
	SafeGain     float64
	SafeLoudness float64
}

// PlanFor works out what can be done with a track.
//
// The arithmetic is simple and exact: a constant gain moves both the loudness
// and the true peak by the same amount. So the peak after correction is known
// before touching the file, and with it whether the target is reachable
// without altering the audio.
func PlanFor(lufs, truePeak, target, ceiling, tolerance float64) Plan {
	p := Plan{}
	p.GainToTarget = target - lufs
	p.PredictedPeak = truePeak + p.GainToTarget
	p.TransparentGain = ceiling - truePeak
	p.LoudnessAtCeiling = lufs + p.TransparentGain
	p.Shortfall = target - p.LoudnessAtCeiling
	if p.Shortfall < 0 {
		p.Shortfall = 0
	}
	if over := p.PredictedPeak - ceiling; over > 0 {
		p.PeakOverBy = over
	}

	// The target is a range, not a point. When the gain to the exact target
	// would push the peaks over the ceiling, holding it back to the largest
	// safe value often still lands inside the tolerance window - and that
	// result is fully transparent, so there is nothing for a client to decide.
	p.SafeGain = math.Min(p.GainToTarget, p.TransparentGain)
	p.SafeLoudness = lufs + p.SafeGain

	switch {
	case math.Abs(lufs-target) <= tolerance:
		p.Phase = PhaseDone
	case math.Abs(p.SafeLoudness-target) <= tolerance:
		p.Phase = PhaseGain
	default:
		p.Phase = PhaseReview
	}
	return p
}

// SpecFor turns a plan plus a decision into the exact transform to apply, along
// with the loudness that transform is expected to produce.
//
// The expected loudness is not always the target: the "gain to ceiling" option
// deliberately stops short, so the result has to be judged against what was
// intended rather than against -12.6.
//
// Returns ok=false when nothing should be done to the file.
func SpecFor(plan Plan, decision string, source *ffmpeg.FileProbe, target, ceiling float64) (spec ffmpeg.ApplySpec, expectedLUFS float64, ok bool) {
	switch plan.Phase {
	case PhaseGain:
		// SafeGain equals GainToTarget whenever there is headroom for it, and
		// stops at the ceiling when there is not.
		return ffmpeg.ApplySpec{GainDB: plan.SafeGain, Source: source}, plan.SafeLoudness, true
	case PhaseReview:
		switch decision {
		case DecisionLimit:
			return ffmpeg.ApplySpec{
				GainDB:        plan.GainToTarget,
				LimitTruePeak: true,
				CeilingDB:     ceiling,
				Source:        source,
			}, target, true
		case DecisionCeiling:
			return ffmpeg.ApplySpec{GainDB: plan.TransparentGain, Source: source}, plan.LoudnessAtCeiling, true
		}
	}
	return ffmpeg.ApplySpec{}, 0, false
}
