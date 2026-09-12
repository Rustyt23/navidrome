package loudness

import (
	"math"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"
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
	// PhaseCloseEnough: reaching the target would need a decision from the
	// client, but the track is already near enough that the decision is not
	// worth asking for. Left completely untouched, and kept off the review
	// page. See leaveAloneToleranceDB.
	PhaseCloseEnough = 4
)

// leaveAloneToleranceDB is how far from the target a track may sit before it is
// worth troubling the client about.
//
// Inside the ordinary tolerance a track is simply done. Between that and this,
// a track that cannot be corrected transparently is left alone rather than
// listed: the difference is inaudible - the smallest loudness change anyone can
// hear is around 1 LU, twice this - and the alternative is a review page full of
// decisions that change nothing anybody could notice.
//
// Measured on a real library, seven of ten refused tracks sat here: each was
// gained by a quarter to half a decibel, re-encoded, measured, found to have
// sprung its peak over the ceiling, and thrown away. The file was untouched
// either way. All that was produced was a row asking someone to decide about a
// difference they cannot hear, at the cost of a full encode per track per run.
//
// It deliberately does not apply to a track that only needs an inaudible peak
// trim (PhaseTrim). Those are handled automatically and never reach the client,
// so there is nothing to save - and one of them was clipping, which is worth
// fixing whatever its loudness already was.
const leaveAloneToleranceDB = 0.5

// LeaveAloneToleranceDB is leaveAloneToleranceDB for the run filter, which
// judges the same band in SQL.
const LeaveAloneToleranceDB = leaveAloneToleranceDB

// model repeats PhaseReview as model.LoudnessPhaseReview, because it cannot
// import this package. Fail the build here if the two ever drift apart.
const _ = uint(PhaseReview - model.LoudnessPhaseReview)

// audibleShaveDB is how deep a peak reduction has to be before anyone can hear
// it, and so where automatic trimming stops and a client decision begins.
//
// A true peak is not a passage of music: it is the highest instant in the
// waveform, a handful of samples at the tip of one transient. Taking a decibel
// off that is inaudible. Past roughly 3 dB the reduction is no longer confined
// to the tip and starts softening the attack of every drum hit - at which point
// it is a trade worth stopping for, not a formality.
const audibleShaveDB = 3.0

// minWorthwhileGainDB is the smallest level change worth rewriting a file for.
//
// Re-encoding moves the measured loudness by about this much on its own, so a
// gain below it cannot be shown to have improved anything - it sits inside the
// process's own noise. What it can be shown to cost is a codec generation.
//
// This is what stops a "gain to ceiling" track being rebuilt on every phase 2
// run. Such a track is gained until its peak meets the ceiling, which leaves
// TransparentGain at zero on the next pass - and it is still short of target
// with its peaks still over, so it is still phase 2 and still carries its
// decision. Nothing in the record can say the decision was already applied,
// because the phase describes what remains to be done and the answer is
// honestly "still short": that shortfall is what the client accepted when they
// chose this option. So the exit condition has to be the gain itself.
const minWorthwhileGainDB = 0.1

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

	// PLR is the gap between the loudest instant and the average level - how
	// much headroom this track's dynamics demand.
	//
	// It is not an independent input to the phase decision, though it looks like
	// one. PeakOverBy is derived from it exactly: the predicted peak is
	// truePeak + (target - lufs), which is (truePeak - lufs) + target, which is
	// PLR + target. So PeakOverBy = PLR + target - ceiling, and at -12.6 with a
	// -0.5 ceiling that is PLR - 12.1.
	//
	// Which means the fixed line above is already dynamics-aware, and scaling it
	// by PLR - as this once did - can only ever shift the same boundary sideways.
	// A compressed master cannot reach a peak problem at all: low PLR means it is
	// already loud and needs turning down, not up. Every track that needs a cut
	// deep enough to ask about is, necessarily, a dynamic one - which is exactly
	// where a cut is least audible.
	PLR float64
}

// maxAutomaticGainDB is the largest lift this will apply without being asked.
//
// There was no limit at all: the plan simply requested whatever the distance to
// the target happened to be, so a recording at -40 LUFS was handed +27 dB. That
// is not a correction, it is turning a room's noise floor into the loudest thing
// on the record, and nothing in the verification catches it - the result is
// exactly as loud as it was told to be.
//
// 20 dB because no commercial master lives below -32.6 LUFS. Broadcast sits at
// -23, the quietest classical and jazz masters reach about -27, and a track
// past this line is not a quiet mix - it is a recording the target was never
// written for.
//
// The gain is NOT clamped. Clamping was tried and is worse than the disease:
// GainToTarget feeds the predicted peak, the phase decision and the expected
// result, so capping it makes the plan understate how far the song is from
// target, mis-file it, and then fail its own verification - leaving the file
// untouched with a confusing reason. The distance stays honest and the track is
// routed to review instead, where a person sees the real number and decides.
const maxAutomaticGainDB = 20.0

// PlanFor works out what can be done with a track.
//
// The arithmetic is simple and exact: a constant gain moves both the loudness
// and the true peak by the same amount. So the peak after correction is known
// before touching the file, and with it whether the target is reachable
// without altering the audio.
func PlanFor(lufs, truePeak, target, ceiling, tolerance float64, sourceBitRate int) Plan {
	p := Plan{}
	p.RewriteCost = ffmpeg.RewriteLoudnessCost(sourceBitRate)
	p.PLR = truePeak - lufs
	p.GainToTarget = target - lufs
	p.PredictedPeak = truePeak + p.GainToTarget
	// Less the allowance for what re-encoding puts back on the peak. Without it
	// this promised headroom the finished file did not have, and the track was
	// planned as transparently fixable, encoded, measured over the ceiling and
	// then rebuilt or thrown away.
	// Not floored at zero. A negative value is meaningful here: it says the peak
	// is already so close to the ceiling that staying safe means turning the
	// track DOWN, and it is used as an upper bound below, where clamping it to
	// zero would quietly permit a gain the peaks cannot take.
	p.TransparentGain = ceiling - truePeak - ffmpeg.PeakSpringBack(sourceBitRate)
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

	// Never spend headroom above the configured ceiling to reach the target.
	p.SafeGain = math.Min(wanted, p.TransparentGain)
	p.SafeLoudness = landsAt(p.SafeGain)

	switch {
	// Checked before everything else: a lift this large is a question about the
	// recording, not a peak problem, and the answer does not depend on where its
	// peaks happen to sit.
	case p.GainToTarget > maxAutomaticGainDB:
		p.Phase = PhaseReview
	case math.Abs(lufs-target) <= tolerance && peakWithinCeiling(truePeak, ceiling):
		p.Phase = PhaseDone
	case math.Abs(p.SafeLoudness-target) <= tolerance:
		p.Phase = PhaseGain
	case p.PeakOverBy <= audibleShaveDB:
		// The level alone cannot get there, but the peaks only have to come
		// down by an amount nobody can hear. Nothing is gained by asking.
		p.Phase = PhaseTrim
	case math.Abs(lufs-target) <= leaveAloneToleranceDB && peakWithinCeiling(truePeak, ceiling):
		// Reaching the target from here needs a decision, and the track is
		// already close enough that the decision is not worth asking for.
		p.Phase = PhaseCloseEnough
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
	// Already where it should be, so there is nothing for anyone to decide.
	// Checked before the decision, because a decision overrules the planner's
	// judgement about HOW to correct a song, not the fact that it needs no
	// correcting - and "limit to target" on a song already at the target is a
	// re-encode that buys nothing and costs a generation of quality.
	if plan.Phase == PhaseDone {
		return ffmpeg.ApplySpec{}, 0, false
	}

	// A decision outranks the plan, whatever phase the planner arrived at.
	//
	// The phase is this package's opinion about how a song should be handled.
	// The decision is a person overruling it, and there is no reading of "the
	// client chose Limit to target" under which the answer is to apply a plain
	// gain instead. This used to be nested inside case PhaseReview, so a
	// decision recorded on a phase 1 song was silently discarded and the run
	// repeated the pure gain that had already been refused - the same refusal,
	// every time, with the client's choice on screen throughout.
	//
	// Skip is answered here too. Left to fall through it reached the phase
	// switch below and a song marked "leave alone" was gained anyway.
	switch decision {
	case DecisionLimit:
		return ffmpeg.ApplySpec{
			GainDB:        plan.GainToTarget,
			LimitTruePeak: true,
			CeilingDB:     ceiling,
			Source:        source,
		}, target, true
	case DecisionCeiling:
		return gainOnly(plan.TransparentGain, source, plan.LoudnessAtCeiling)
	case DecisionSkip:
		return ffmpeg.ApplySpec{}, 0, false
	}

	switch plan.Phase {
	case PhaseGain:
		// SafeGain equals GainToTarget whenever there is headroom for it, and
		// stops at the ceiling when there is not.
		return gainOnly(plan.SafeGain, source, plan.SafeLoudness)
	case PhaseTrim:
		// The same transform the client would be offered as "limit to target",
		// applied without asking because the cut is too small to hear.
		return ffmpeg.ApplySpec{
			GainDB:        plan.GainToTarget,
			LimitTruePeak: true,
			CeilingDB:     ceiling,
			Source:        source,
		}, target, true
	}
	return ffmpeg.ApplySpec{}, 0, false
}

// gainOnly builds a level-only transform, unless the level is not actually
// moving. A gain under minWorthwhileGainDB buys nothing measurable and costs a
// rewrite, so there is nothing to do to the file.
//
// It guards only the transforms that are pure gain. A limiting pass with a
// small gain still does real work - it is there to bring the peaks down, not
// the level - so it is left alone.
func gainOnly(gainDB float64, source *ffmpeg.FileProbe, expectedLUFS float64) (ffmpeg.ApplySpec, float64, bool) {
	if math.Abs(gainDB) < minWorthwhileGainDB {
		return ffmpeg.ApplySpec{}, 0, false
	}
	return ffmpeg.ApplySpec{GainDB: gainDB, Source: source}, expectedLUFS, true
}
