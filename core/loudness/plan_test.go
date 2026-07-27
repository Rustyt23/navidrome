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
			p := PlanFor(tc.lufs, tc.truePeak, testTarget, testCeiling, testTolerance)
			if p.Phase != tc.wantPhase {
				t.Fatalf("phase = %d, want %d (predicted peak %.2f)", p.Phase, tc.wantPhase, p.PredictedPeak)
			}
		})
	}
}

// A constant gain shifts loudness and true peak by the same amount. The plan's
// arithmetic must reflect that exactly, because the phase decision rests on it.
func TestPlanArithmetic(t *testing.T) {
	p := PlanFor(-18.0, -0.5, testTarget, testCeiling, testTolerance)

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
	p := PlanFor(-11.89, -0.46, testTarget, testCeiling, testTolerance)

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
	p := PlanFor(-18.0, -12.0, testTarget, testCeiling, testTolerance)
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
	p := PlanFor(-18.0, -12.0, testTarget, testCeiling, testTolerance)
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
	p := PlanFor(-18.0, -0.5, testTarget, testCeiling, testTolerance)

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

func TestSpecForDoneTrackIsNeverTouched(t *testing.T) {
	p := PlanFor(-12.6, -3.0, testTarget, testCeiling, testTolerance)
	for _, d := range []string{DecisionPending, DecisionLimit, DecisionCeiling, DecisionSkip} {
		if _, _, ok := SpecFor(p, d, &ffmpeg.FileProbe{}, testTarget, testCeiling); ok {
			t.Fatalf("a track already on target must not be rewritten (decision %q)", d)
		}
	}
}
