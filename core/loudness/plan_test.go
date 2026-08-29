package loudness

import (
	"math"
	"testing"

	"github.com/navidrome/navidrome/core/ffmpeg"
)

const (
	testTarget    = -12.6
	testCeiling   = -1.5
	testTolerance = 0.5
)

func TestPlanForClassifiesByHeadroom(t *testing.T) {
	cases := []struct {
		name      string
		lufs      float64
		truePeak  float64
		wantPhase int
	}{
		{"already on target", -12.6, -3.0, PhaseDone},
		{"just inside tolerance", -12.15, -3.0, PhaseDone},
		{"just outside tolerance, plenty of headroom", -13.2, -8.0, PhaseGain},
		{"quiet with low peaks reaches target cleanly", -20.0, -12.0, PhaseGain},
		{"lands exactly on the ceiling", -18.0, -6.9, PhaseGain},
		{"loud master needing a cut is always safe", -8.0, -0.2, PhaseGain},
		{"quiet but peaky cannot be fixed by gain", -18.0, -0.5, PhaseReview},
		// Holding the gain back to the ceiling still lands inside the tolerance
		// window, so these need no client decision.
		{"a hair over the ceiling is still reachable", -18.0, -6.8, PhaseGain},
		{"too loud, and a cut leaves peaks over the ceiling", -11.89, -0.46, PhaseGain},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := PlanFor(tc.lufs, tc.truePeak, testTarget, testCeiling, testTolerance, 320)
			if p.Phase != tc.wantPhase {
				t.Fatalf("phase = %d, want %d (predicted peak %.2f)", p.Phase, tc.wantPhase, p.PredictedPeak)
			}
		})
	}
}

// A constant gain shifts loudness and true peak by the same amount. The plan's
// arithmetic must reflect that exactly, because the phase decision rests on it.
func TestPlanArithmetic(t *testing.T) {
	p := PlanFor(-18.0, -0.5, testTarget, testCeiling, testTolerance, 320)

	if got, want := p.GainToTarget, 5.4; math.Abs(got-want) > 0.001 {
		t.Errorf("GainToTarget = %.3f, want %.3f", got, want)
	}
	if got, want := p.PredictedPeak, 4.9; math.Abs(got-want) > 0.001 {
		t.Errorf("PredictedPeak = %.3f, want %.3f", got, want)
	}
	if got, want := p.PeakOverBy, 6.4; math.Abs(got-want) > 0.001 {
		t.Errorf("PeakOverBy = %.3f, want %.3f", got, want)
	}
	// Largest gain that keeps the peak at the ceiling once re-encoding has put
	// part of it back: -0.5 - (-0.5) less the 0.15 dB a 320k encode springs.
	if got, want := p.TransparentGain, -1.15; math.Abs(got-want) > 0.001 {
		t.Errorf("TransparentGain = %.3f, want %.3f", got, want)
	}
	if got, want := p.LoudnessAtCeiling, -19.15; math.Abs(got-want) > 0.001 {
		t.Errorf("LoudnessAtCeiling = %.3f, want %.3f", got, want)
	}
	if got, want := p.Shortfall, 6.55; math.Abs(got-want) > 0.001 {
		t.Errorf("Shortfall = %.3f, want %.3f", got, want)
	}
}

// A track louder than target whose peaks sit above the ceiling: cutting it by
// the full amount would leave peaks over the limit, but a slightly larger cut
// lands inside the tolerance window with peaks exactly at the ceiling. This is
// transparent, so it belongs in phase 1 rather than the review queue.
func TestPlanHoldsGainBackToTheCeiling(t *testing.T) {
	p := PlanFor(-11.89, -0.46, testTarget, testCeiling, testTolerance, 320)

	if p.Phase != PhaseGain {
		t.Fatalf("phase = %d, want %d", p.Phase, PhaseGain)
	}
	if math.Abs(p.GainToTarget-(-0.71)) > 0.001 {
		t.Errorf("GainToTarget = %.3f, want -0.710", p.GainToTarget)
	}
	// The gain to the exact target would leave peaks at -1.17, over the ceiling.
	if p.PredictedPeak <= testCeiling {
		t.Errorf("fixture no longer exercises the held-back path")
	}
	// -1.04 less the 0.15 dB a 320k re-encode puts back on the peak.
	if math.Abs(p.SafeGain-(-1.19)) > 0.001 {
		t.Errorf("SafeGain = %.3f, want -1.190", p.SafeGain)
	}
	if math.Abs(p.SafeLoudness-(-13.08)) > 0.001 {
		t.Errorf("SafeLoudness = %.3f, want -13.080", p.SafeLoudness)
	}
	if math.Abs(p.SafeLoudness-testTarget) > leaveAloneToleranceDB {
		t.Errorf("SafeLoudness %.2f is outside even the wider window", p.SafeLoudness)
	}

	spec, expected, ok := SpecFor(p, DecisionPending, &ffmpeg.FileProbe{}, testTarget, testCeiling)
	if !ok {
		t.Fatal("expected a transform")
	}
	if spec.LimitTruePeak {
		t.Error("this must stay a pure gain - no limiting")
	}
	if math.Abs(spec.GainDB-p.SafeGain) > 0.001 {
		t.Errorf("applied gain = %.3f, want the held-back %.3f", spec.GainDB, p.SafeGain)
	}
	if math.Abs(expected-p.SafeLoudness) > 0.001 {
		t.Errorf("expected loudness = %.3f, want %.3f", expected, p.SafeLoudness)
	}
}

// With headroom to spare the gain is not held back at all.
func TestPlanUsesFullGainWhenHeadroomAllows(t *testing.T) {
	p := PlanFor(-18.0, -12.0, testTarget, testCeiling, testTolerance, 320)
	if p.Phase != PhaseGain {
		t.Fatalf("phase = %d, want %d", p.Phase, PhaseGain)
	}
	if math.Abs(p.SafeGain-p.GainToTarget) > 0.001 {
		t.Errorf("SafeGain = %.3f should equal GainToTarget %.3f", p.SafeGain, p.GainToTarget)
	}
	if math.Abs(p.SafeLoudness-testTarget) > 0.001 {
		t.Errorf("SafeLoudness = %.3f, want the target %.3f", p.SafeLoudness, testTarget)
	}
}

func TestSpecForPhaseGainIsPureGain(t *testing.T) {
	p := PlanFor(-18.0, -12.0, testTarget, testCeiling, testTolerance, 320)
	spec, expected, ok := SpecFor(p, DecisionPending, &ffmpeg.FileProbe{}, testTarget, testCeiling)
	if !ok {
		t.Fatal("expected a transform")
	}
	if spec.LimitTruePeak {
		t.Error("phase 1 must never engage the limiter")
	}
	if math.Abs(spec.GainDB-p.SafeGain) > 0.001 {
		t.Errorf("gain = %.3f, want %.3f", spec.GainDB, p.SafeGain)
	}
	if math.Abs(expected-testTarget) > 0.001 {
		t.Errorf("expected loudness = %.3f, want the target %.3f", expected, testTarget)
	}
}

func TestSpecForReviewTrackNeedsADecision(t *testing.T) {
	// Peaks at -2.0, so there is 1.5 dB of headroom for "gain to ceiling" to
	// actually use. A fixture whose peak already sits on the ceiling has none,
	// and would only exercise the do-nothing path below.
	p := PlanFor(-20.0, -2.0, testTarget, testCeiling, testTolerance, 320)
	if p.Phase != PhaseReview {
		t.Fatalf("fixture no longer exercises the review path (phase %d)", p.Phase)
	}

	if _, _, ok := SpecFor(p, DecisionPending, &ffmpeg.FileProbe{}, testTarget, testCeiling); ok {
		t.Fatal("a review track must not be touched before the client decides")
	}
	if _, _, ok := SpecFor(p, DecisionSkip, &ffmpeg.FileProbe{}, testTarget, testCeiling); ok {
		t.Fatal("'skip' must leave the file alone")
	}

	limit, expected, ok := SpecFor(p, DecisionLimit, &ffmpeg.FileProbe{}, testTarget, testCeiling)
	if !ok || !limit.LimitTruePeak {
		t.Fatal("'limit' must engage the limiter")
	}
	if math.Abs(expected-testTarget) > 0.001 {
		t.Errorf("limiting should reach the target, expected %.3f", expected)
	}

	ceil, expected, ok := SpecFor(p, DecisionCeiling, &ffmpeg.FileProbe{}, testTarget, testCeiling)
	if !ok {
		t.Fatal("'gain to ceiling' must produce a transform")
	}
	if ceil.LimitTruePeak {
		t.Error("'gain to ceiling' must stay transparent - no limiting")
	}
	// It deliberately stops short of the target; the result is judged against
	// that, not against -12.6.
	if math.Abs(expected-p.LoudnessAtCeiling) > 0.001 {
		t.Errorf("expected loudness = %.3f, want %.3f", expected, p.LoudnessAtCeiling)
	}
}

// A "gain to ceiling" track is gained until its peak meets the ceiling. On
// every run after that there is no headroom left, so the transform is a 0 dB
// gain - and the track is still phase 2 carrying the same decision, so every
// phase 2 run selected it again and rewrote it. Each pass was a fresh codec
// generation spent to change nothing.
func TestSpecForCeilingDecisionIsNotReappliedOnceThereIsNoHeadroomLeft(t *testing.T) {
	// The state the track is left in by a successful "gain to ceiling" pass:
	// short of target, with no transparent headroom left at all. The peak sits
	// a spring-back below the ceiling, which is where a 320k encode lands it.
	p := PlanFor(-18.5, testCeiling-ffmpeg.PeakSpringBack(320), testTarget, testCeiling, testTolerance, 320)
	if p.Phase != PhaseReview {
		t.Fatalf("a track gained to the ceiling stays a review track (phase %d)", p.Phase)
	}
	if math.Abs(p.TransparentGain) > 0.001 {
		t.Fatalf("fixture should have no headroom left, TransparentGain = %.3f", p.TransparentGain)
	}

	if _, _, ok := SpecFor(p, DecisionCeiling, &ffmpeg.FileProbe{}, testTarget, testCeiling); ok {
		t.Fatal("re-applying 'gain to ceiling' with no headroom left must not rewrite the file")
	}

	// The decision itself is still honoured wherever it has something to do.
	withHeadroom := PlanFor(-20.0, -2.0, testTarget, testCeiling, testTolerance, 320)
	if _, _, ok := SpecFor(withHeadroom, DecisionCeiling, &ffmpeg.FileProbe{}, testTarget, testCeiling); !ok {
		t.Fatal("'gain to ceiling' must still apply while there is headroom to use")
	}
}

// Limiting is not a level change: it is there to bring the peaks down, so it
// stays available however small the accompanying gain is.
//
// PlanFor cannot actually produce this state - a gain that small means the
// track is within tolerance of the target, which is PhaseDone - so the plan is
// built directly. The point is to pin the guard's scope: it belongs to the
// pure-gain transforms and must not spread to the limiting ones.
func TestSpecForSmallGainStillLimits(t *testing.T) {
	tiny := minWorthwhileGainDB / 2
	p := Plan{Phase: PhaseReview, GainToTarget: tiny}

	spec, expected, ok := SpecFor(p, DecisionLimit, &ffmpeg.FileProbe{}, testTarget, testCeiling)
	if !ok || !spec.LimitTruePeak {
		t.Fatal("'limit' must still engage the limiter when the gain is tiny - the peaks are the point")
	}
	if math.Abs(expected-testTarget) > 0.001 {
		t.Errorf("limiting should still aim at the target, expected %.3f", expected)
	}

	// The same tiny number on the pure-gain side is refused.
	p.TransparentGain = tiny
	if _, _, ok := SpecFor(p, DecisionCeiling, &ffmpeg.FileProbe{}, testTarget, testCeiling); ok {
		t.Error("a pure gain this small must not rewrite the file")
	}
}

func TestSpecForDoneTrackIsNeverTouched(t *testing.T) {
	p := PlanFor(-12.6, -3.0, testTarget, testCeiling, testTolerance, 320)
	for _, d := range []string{DecisionPending, DecisionLimit, DecisionCeiling, DecisionSkip} {
		if _, _, ok := SpecFor(p, d, &ffmpeg.FileProbe{}, testTarget, testCeiling); ok {
			t.Fatalf("a track already on target must not be rewritten (decision %q)", d)
		}
	}
}

// Rewriting a degraded source costs loudness before any gain is applied, so a
// plan that ignores it asks for a gain that lands short. On a track whose
// ceiling leaves no room for the difference, that meant failing on every run
// for ever.
func TestPlanCoversWhatRewritingCosts(t *testing.T) {
	const tol = 0.2
	// Done for Me: 128k, 0.42 below target, peaks at -1.07.
	lossy := PlanFor(-13.02, -1.07, testTarget, -0.5, tol, 128)
	clean := PlanFor(-13.02, -1.07, testTarget, -0.5, tol, 320)

	if lossy.RewriteCost <= 0 {
		t.Fatal("a 128k source loses loudness on every rewrite; the plan records none")
	}
	if clean.RewriteCost != 0 {
		t.Errorf("320k rewrite cost = %.2f, want 0 - it loses nothing measurable", clean.RewriteCost)
	}
	if lossy.SafeGain <= clean.SafeGain {
		t.Errorf("lossy gain %.2f should exceed the clean one %.2f by the rewrite cost",
			lossy.SafeGain, clean.SafeGain)
	}
	// The pure-gain landing is no longer the measure of this. Once the plan also
	// allows for the 0.5 dB a 128k encode springs back onto the peak, this track
	// has too little headroom to reach the target on gain alone - which is the
	// truth about it, and is why every attempt to do so came back over the
	// ceiling and was refused. What matters is that the plan now says so up
	// front and routes it somewhere that does reach the target.
	if lossy.Phase != PhaseTrim {
		t.Errorf("phase = %d, want PhaseTrim (%d): the track needs its peaks held to reach target",
			lossy.Phase, PhaseTrim)
	}
	// And the route it picks has to be one that actually lands on target, not
	// one that leaves it short for ever.
	if lossy.Phase == PhaseGain && math.Abs(lossy.SafeLoudness-testTarget) > tol {
		t.Errorf("planned as a plain gain but lands at %.2f, outside +/-%.1f",
			lossy.SafeLoudness, tol)
	}
}

// The fallback ceiling is a last resort in the plan exactly as it is in the
// optimiser: reached for only when holding to the configured ceiling would miss
// the target, so no track is planned to spend headroom it does not need.
func TestPlanPrefersTheConfiguredCeiling(t *testing.T) {
	// Holding to -1.5 lands at -12.93, inside a 0.5 window, so the plan must
	// stop there rather than take the full cut and a higher peak.
	p := PlanFor(-11.89, -0.46, testTarget, testCeiling, testTolerance, 320)
	if math.Abs(p.SafeGain-p.TransparentGain) > 0.001 {
		t.Errorf("SafeGain = %.3f, want the configured ceiling's %.3f", p.SafeGain, p.TransparentGain)
	}

	// The same track under a tight window cannot reach the target at -1.5, so
	// here the fallback is what makes it reachable at all.
	tight := PlanFor(-11.89, -0.46, testTarget, testCeiling, 0.2, 320)
	if tight.SafeGain <= tight.TransparentGain {
		t.Errorf("SafeGain = %.3f, want more than the configured ceiling allows (%.3f)",
			tight.SafeGain, tight.TransparentGain)
	}
	if math.Abs(tight.SafeLoudness-testTarget) > 0.2 {
		t.Errorf("lands at %.2f, still outside the window", tight.SafeLoudness)
	}
}

// Cuts too small to hear are applied without asking; only a cut deep enough to
// soften the music is a trade worth stopping for. The review page is for the
// second kind, and a page of formalities is one nobody reads.
func TestPlanTrimsQuietlyAndAsksOnlyWhenItWouldBeHeard(t *testing.T) {
	const tol = 0.2
	cases := []struct {
		name      string
		lufs      float64
		truePeak  float64
		wantPhase int
	}{
		// All of these need +0.90 dB to reach the target, so the cut the
		// limiter has to make is (peak + 0.90) - the -0.50 ceiling.
		{"a cut nobody can hear is just applied", -13.5, 0.4, PhaseTrim},          // 1.80
		{"still applied just under the audible threshold", -13.5, 1.5, PhaseTrim}, // 2.90
		{"past the threshold it is the client's call", -13.5, 1.7, PhaseReview},   // 3.10
		{"a deep cut always asks", -13.5, 6.6, PhaseReview},                       // 8.00
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := PlanFor(tc.lufs, tc.truePeak, testTarget, -0.5, tol, 320)
			if p.Phase != tc.wantPhase {
				t.Fatalf("phase = %d, want %d (cut needed %.2f dB)", p.Phase, tc.wantPhase, p.PeakOverBy)
			}
		})
	}
}

// A trim is the same transform the client would be offered as "limit to
// target"; the only difference is that nobody was asked.
func TestSpecForTrimLimitsToTheTarget(t *testing.T) {
	p := PlanFor(-13.5, 0.4, testTarget, -0.5, 0.2, 320)
	if p.Phase != PhaseTrim {
		t.Fatalf("fixture no longer exercises the trim path (phase %d)", p.Phase)
	}
	spec, expected, ok := SpecFor(p, DecisionPending, &ffmpeg.FileProbe{}, testTarget, -0.5)
	if !ok {
		t.Fatal("a trim must produce a transform without waiting for a decision")
	}
	if !spec.LimitTruePeak {
		t.Error("a trim has to engage the limiter; the level alone cannot get there")
	}
	if math.Abs(spec.GainDB-p.GainToTarget) > 0.001 {
		t.Errorf("gain = %.3f, want the full %.3f", spec.GainDB, p.GainToTarget)
	}
	if math.Abs(expected-testTarget) > 0.001 {
		t.Errorf("expected loudness = %.3f, want the target", expected)
	}
}

// The audible-cut line looks like it should scale with how dynamic a track is:
// the same trim costs more on a master that is already compressed than on a
// live recording where only one cymbal reaches the peak. It was very nearly
// changed to do exactly that.
//
// It must not be, because the two are the same variable. The predicted peak is
// truePeak + (target - lufs), which rearranges to (truePeak - lufs) + target -
// that is PLR + target. So how far a track's peaks overshoot is fixed by its
// PLR alone, and the fixed line is already dynamics-aware.
//
// The consequence worth keeping is this: a compressed master can never reach a
// peak problem at all. Low PLR means it is already loud, so it needs turning
// down or barely up, and its peaks go down with it. Every track that needs a cut
// deep enough to be worth asking about is necessarily a dynamic one, which is
// precisely where the cut is least audible.
func TestPeakOvershootIsDecidedByDynamicsAlone(t *testing.T) {
	const ceiling = -0.5
	for _, plr := range []float64{6, 9, 12, 14, 16, 19} {
		// Two tracks with the same dynamics at completely different levels.
		for _, lufs := range []float64{-20.0, -16.0, -13.0} {
			p := PlanFor(lufs, lufs+plr, testTarget, ceiling, 0.2, 320)

			want := math.Max(0, plr+testTarget-ceiling)
			if math.Abs(p.PeakOverBy-want) > 0.001 {
				t.Errorf("PLR %.0f at %.1f LUFS: overshoot %.2f, want %.2f - it is a function of PLR only",
					plr, lufs, p.PeakOverBy, want)
			}
			if math.Abs(p.PLR-plr) > 0.001 {
				t.Errorf("PLR reported as %.2f, want %.2f", p.PLR, plr)
			}
		}
	}

	// A compressed master never gets there, whatever its level.
	for _, lufs := range []float64{-20.0, -16.0, -13.0} {
		if p := PlanFor(lufs, lufs+8, testTarget, ceiling, 0.2, 320); p.PeakOverBy > 0 {
			t.Errorf("a PLR-8 master at %.1f LUFS should have no peak problem, got %.2f dB over",
				lufs, p.PeakOverBy)
		}
	}
}

// Reaching the last fraction of a decibel on a track that cannot be corrected
// transparently costs a full encode and a row on the review page, and buys a
// difference nobody can hear. Measured on a real library, seven of ten refusals
// were exactly this: gained by a quarter to half a decibel, re-encoded,
// measured, found to have sprung the peak over the ceiling, thrown away.
func TestPlanLeavesATrackAloneWhenItIsCloseEnoughToNotBeWorthAsking(t *testing.T) {
	const tol = 0.2
	// Quiet and peaky: the target is out of reach without an audible cut, which
	// is what would otherwise send it to the client.
	peaky := func(lufs float64) Plan {
		return PlanFor(lufs, lufs+16.5, testTarget, -0.5, tol, 320)
	}

	if p := peaky(-12.9); p.Phase != PhaseCloseEnough {
		t.Errorf("0.30 from target: phase %d, want %d - too close to be worth asking about",
			p.Phase, PhaseCloseEnough)
	}
	if p := peaky(-13.05); p.Phase != PhaseCloseEnough {
		t.Errorf("0.45 from target: phase %d, want %d", p.Phase, PhaseCloseEnough)
	}
	// Beyond the band the client is asked, as before.
	if p := peaky(-14.0); p.Phase != PhaseReview {
		t.Errorf("1.40 from target: phase %d, want %d - far enough to be worth a decision",
			p.Phase, PhaseReview)
	}
	// Inside the ordinary tolerance nothing changes: it was already done.
	if p := peaky(-12.7); p.Phase != PhaseDone {
		t.Errorf("0.10 from target: phase %d, want %d", p.Phase, PhaseDone)
	}
}

// The rule must not reach a track that never troubles the client. A small
// automatic trim is applied without asking, so there is nothing to save by
// skipping it - and one such track in the real library was clipping, which is
// worth fixing whatever its loudness already was.
func TestPlanStillTrimsQuietlyInsideTheLeaveAloneBand(t *testing.T) {
	// 0.30 from target - inside the band - and clipping at +0.10 dBTP. A pure
	// gain cannot reach the target from here, but the peaks only need 0.90 dB
	// off, which nobody can hear. Exactly the shape of the track in the real
	// library that this rule must not skip.
	p := PlanFor(-12.9, 0.1, testTarget, -0.5, 0.2, 320)
	if p.PeakOverBy > audibleShaveDB {
		t.Fatalf("fixture no longer exercises the trim path (over by %.2f)", p.PeakOverBy)
	}
	if math.Abs(-12.9-testTarget) > leaveAloneToleranceDB {
		t.Fatal("fixture is outside the leave-alone band, so it proves nothing")
	}
	if p.Phase != PhaseTrim {
		t.Errorf("phase = %d, want %d - an inaudible trim is applied, not skipped",
			p.Phase, PhaseTrim)
	}
}

// A decision is a person overruling the planner, and it has to reach the
// transform whatever phase the planner arrived at.
//
// The five songs that exposed this were all phase 1: planned as a plain gain,
// built, refused for peaks, listed on the exceptions page, and given a
// decision. SpecFor read the decision only under case PhaseReview, so it was
// discarded and the run repeated the same pure gain - and earned the same
// refusal - with "Limit to target" showing on screen the whole time.
func TestSpecForHonoursADecisionOnAnyPhase(t *testing.T) {
	// Window to the Soul: 1.25 dB below target, peaks with almost no room.
	p := PlanFor(-13.85, -1.28, testTarget, testCeiling, testTolerance, 320)
	if p.Phase != PhaseGain {
		t.Fatalf("fixture no longer exercises the bug: phase is %d, wanted %d", p.Phase, PhaseGain)
	}

	spec, expected, ok := SpecFor(p, DecisionLimit, &ffmpeg.FileProbe{}, testTarget, testCeiling)
	if !ok {
		t.Fatal("a decision to limit produced no transform at all")
	}
	if !spec.LimitTruePeak {
		t.Error("limit to target must engage the limiter; without it this is the gain that was already refused")
	}
	if math.Abs(spec.GainDB-p.GainToTarget) > 0.001 {
		t.Errorf("gain %.2f, wanted the full %.2f to the target", spec.GainDB, p.GainToTarget)
	}
	if math.Abs(expected-testTarget) > 0.001 {
		t.Errorf("expected loudness %.2f, wanted the target %.2f", expected, testTarget)
	}

	// The other choice on the same song: lift it as far as the peaks allow and
	// reshape nothing.
	ceilingSpec, _, ok := SpecFor(p, DecisionCeiling, &ffmpeg.FileProbe{}, testTarget, testCeiling)
	if !ok {
		t.Fatal("a decision to gain to the ceiling produced no transform")
	}
	if ceilingSpec.LimitTruePeak {
		t.Error("gain to ceiling must not reshape the waveform")
	}
}

// "Leave alone" has to mean it on every phase too. Answered only under
// PhaseReview, a skip on a phase 1 song fell through to the plan and the song
// was gained anyway - the one outcome the client explicitly ruled out.
func TestSpecForSkipLeavesTheFileAlone(t *testing.T) {
	for _, p := range []Plan{
		PlanFor(-13.85, -1.28, testTarget, testCeiling, testTolerance, 320), // phase 1
		PlanFor(-16.56, -1.25, testTarget, testCeiling, testTolerance, 320), // phase 2
	} {
		if _, _, ok := SpecFor(p, DecisionSkip, &ffmpeg.FileProbe{}, testTarget, testCeiling); ok {
			t.Errorf("phase %d: a song marked skip was given a transform", p.Phase)
		}
	}
}
