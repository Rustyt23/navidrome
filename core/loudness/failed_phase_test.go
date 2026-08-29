package loudness

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/navidrome/navidrome/core/ffmpeg"
)

// Phase 0 is PhaseDone - "already on target, never needs opening". The failure
// paths never set a phase, so it took that zero value and every place reading
// the phase directly agreed that an unreadable file was finished.
func TestFailedAnalysisIsNotRecordedAsDone(t *testing.T) {
	if _, err := ffmpeg.ProbeFile(context.Background(), "/nonexistent"); err == nil {
		t.Skip("expected probing a missing file to fail")
	}
	dir := t.TempDir()
	// A file that exists and contains nothing playable, which is what a
	// truncated download looks like.
	broken := filepath.Join(dir, "broken.mp3")
	if err := os.WriteFile(broken, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	target := ffmpeg.LoudnessTarget{IntegratedLUFS: -12.6, TruePeak: -0.5, LRA: 11}
	normalizer := ffmpeg.NewLoudnessNormalizer()
	ctx := context.Background()

	full := Audit(ctx, normalizer, "id-1", dir, broken, target, 0.2, "")
	if full.Phase == PhaseDone {
		t.Errorf("Audit: a file that could not be measured was recorded as phase %d (PhaseDone)", full.Phase)
	}
	if full.Phase != PhaseUnplanned {
		t.Errorf("Audit: phase %d, wanted PhaseUnplanned (%d)", full.Phase, PhaseUnplanned)
	}

	orig := MeasureOriginal(ctx, normalizer, "id-1", dir, broken, target, 0.2, "")
	if orig.Phase == PhaseDone {
		t.Errorf("MeasureOriginal: a file that could not be measured was recorded as phase %d (PhaseDone)", orig.Phase)
	}
	if orig.Phase != PhaseUnplanned {
		t.Errorf("MeasureOriginal: phase %d, wanted PhaseUnplanned (%d)", orig.Phase, PhaseUnplanned)
	}
}

// A lift this large is a question about the recording, not a correction to
// apply unasked: nothing in the verification notices that a room's noise floor
// has been made the loudest thing on the record.
func TestHugeLiftGoesToReviewInsteadOfBeingApplied(t *testing.T) {
	const target, ceiling, tol = -12.6, -0.5, 0.2

	// -40 LUFS with plenty of headroom: a plain gain would reach the target and
	// pass every check, applying +27.4 dB on the way.
	huge := PlanFor(-40, -38, target, ceiling, tol, 320)
	if huge.Phase != PhaseReview {
		t.Errorf("a %.1f dB lift was planned as phase %d, expected review", huge.GainToTarget, huge.Phase)
	}
	// The distance must stay honest. Clamping it here made the plan understate
	// how far the song was, mis-file it, and fail its own verification.
	if huge.GainToTarget < 27.0 {
		t.Errorf("GainToTarget was capped to %.2f; it must report the real distance", huge.GainToTarget)
	}

	// A quiet but real master still gets corrected automatically.
	for _, lufs := range []float64{-30.0, -27.0, -23.0, -20.0} {
		p := PlanFor(lufs, lufs+1.0, target, ceiling, tol, 320)
		if p.Phase == PhaseReview {
			t.Errorf("a %.1f LUFS master (%.2f dB lift) was sent to review; that is an ordinary quiet mix",
				lufs, p.GainToTarget)
		}
	}
}
