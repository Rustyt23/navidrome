package loudness

import (
	"math"

	"github.com/navidrome/navidrome/core/ffmpeg"
)

// Phases of the optimisation process.
const (
	// PhaseUnplanned: never measured, so nothing is known about the track yet.
	// Matches the column default, which is what a row carries before its first
	// analysis and what a LEFT JOIN yields for a track with no audit row at all.
	PhaseUnplanned = -1
	// PhaseDone: already within tolerance of the target - nothing to do, the
	// file is never opened.
	PhaseDone = 0
	// PhaseGain: the target is reachable by a constant gain without pushing
	// the true peak past the ceiling. Mathematically transparent - only the
	// level changes. These are optimised automatically.
	PhaseGain = 1
	// PhaseReview: reaching the target means removing enough of the loudest
	// moments to be heard. That is a real trade, so the client decides.
	PhaseReview = 2
	// PhaseTrim: the target needs the peaks brought down, but by so little that
	// nobody can hear it. Applied automatically, and kept apart from PhaseGain
	// so a track that had its peaks touched at all can still be told from one
	// where only the level moved.
	PhaseTrim = 3
)

// audibleShaveDB is how deep a peak reduction has to be before anyone can hear
// it, and so where automatic trimming stops and a client decision begins.
//
// A true peak is not a passage of music: it is the highest instant in the
// waveform, a handful of samples at the tip of one transient. Taking a decibel
// off that is inaudible. Past roughly 3 dB the reduction is no longer confined
// to the tip and starts softening the attack of every drum hit - at which point
// it is a trade worth stopping for, not a formality.
const audibleShaveDB = 3.0

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

	// RewriteCost is the loudness a rewrite of this source loses on its own,
	// before any gain. Zero for a bitrate high enough that re-encoding costs
	// nothing measurable.
	RewriteCost float64

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
func PlanFor(lufs, truePeak, target, ceiling, tolerance float64, sourceBitRate int) Plan {
	p := Plan{}
	p.RewriteCost = ffmpeg.RewriteLoudnessCost(sourceBitRate)
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
	//
	// Rewriting a degraded source loses loudness on its own, so the gain has to
	// cover that as well as the distance to the target, or it lands short.
	wanted := p.GainToTarget + p.RewriteCost
	landsAt := func(gain float64) float64 { return lufs + gain - p.RewriteCost }

	// The configured ceiling comes first and is used wherever it reaches the
	// target. Only when holding to it would miss does the plan reach into the
	// fallback ceiling - the same order the optimiser applies, so a track is
	// never planned to spend headroom it does not need.
	p.SafeGain = math.Min(wanted, p.TransparentGain)
	if math.Abs(landsAt(p.SafeGain)-target) > tolerance {
		p.SafeGain = math.Min(wanted, math.Max(ceiling, fallbackCeilingDB)-truePeak)
	}
	p.SafeLoudness = landsAt(p.SafeGain)

	switch {
	case math.Abs(lufs-target) <= tolerance:
		p.Phase = PhaseDone
	case math.Abs(p.SafeLoudness-target) <= tolerance:
		p.Phase = PhaseGain
	case p.PeakOverBy <= audibleShaveDB:
		// The level alone cannot get there, but the peaks only have to come
		// down by an amount nobody can hear. Nothing is gained by asking.
		p.Phase = PhaseTrim
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
	case PhaseTrim:
		// The same transform the client would be offered as "limit to target",
		// applied without asking because the cut is too small to hear.
		return ffmpeg.ApplySpec{
			GainDB:        plan.GainToTarget,
			LimitTruePeak: true,
			CeilingDB:     ceiling,
			Source:        source,
		}, target, true
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
