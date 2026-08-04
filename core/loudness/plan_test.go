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
	// Largest gain that keeps the peak at the ceiling, and where it lands.
	if got, want := p.TransparentGain, -1.0; math.Abs(got-want) > 0.001 {
		t.Errorf("TransparentGain = %.3f, want %.3f", got, want)
	}
	if got, want := p.LoudnessAtCeiling, -19.0; math.Abs(got-want) > 0.001 {
		t.Errorf("LoudnessAtCeiling = %.3f, want %.3f", got, want)
	}
	if got, want := p.Shortfall, 6.4; math.Abs(got-want) > 0.001 {
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
	if math.Abs(p.SafeGain-(-1.04)) > 0.001 {
		t.Errorf("SafeGain = %.3f, want -1.040", p.SafeGain)
	}
	if math.Abs(p.SafeLoudness-(-12.93)) > 0.001 {
		t.Errorf("SafeLoudness = %.3f, want -12.930", p.SafeLoudness)
	}
	if math.Abs(p.SafeLoudness-testTarget) > testTolerance {
		t.Errorf("SafeLoudness %.2f is outside the tolerance window", p.SafeLoudness)
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
	// short of target, peaks resting exactly on the ceiling.
	p := PlanFor(-18.5, testCeiling, testTarget, testCeiling, testTolerance, 320)
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
	if math.Abs(lossy.SafeLoudness-testTarget) > tol {
		t.Errorf("lands at %.2f, outside +/-%.1f of the target - the cost was not covered",
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
