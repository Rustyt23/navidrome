package loudness

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/navidrome/navidrome/model"
)

// A hard failure has to leave a mark. optimizeOneTrack used to return bare on
// error, writing nothing at all, so the song stayed looking untouched - not
// analysed, not optimised, no verdict - and every later run failed on it the
// same way. Eight songs in a real library sat in exactly that state, visible
// only as a number in a progress bar.
func TestFailedAuditNamesAnEmptyFileForWhatItIs(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.mp3")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	a := FailedAudit("id-1", empty, errors.New("probing: exit status 1"))
	if a.Verdict != model.LoudnessVerdictNoAudio {
		t.Errorf("verdict = %q, want %q - an empty file will never work and should not read as a retryable failure",
			a.Verdict, model.LoudnessVerdictNoAudio)
	}
	if a.Status != model.LoudnessStatusFailed {
		t.Errorf("status = %q, want failed", a.Status)
	}
	// Phase 0 is PhaseDone. Left at its zero value an unreadable file reads as
	// finished everywhere the phase is consulted.
	if a.Phase != PhaseUnplanned {
		t.Errorf("phase = %d, want PhaseUnplanned (%d)", a.Phase, PhaseUnplanned)
	}
	if a.Error == "" {
		t.Error("the engine's own words were dropped; nothing is left to debug with")
	}
	if a.AnalyzedAt.IsZero() {
		t.Error("no timestamp, so the row cannot be told from one never written")
	}
}

// A file with audio in it that failed for some other reason might work next
// time, so it must not be branded unfixable.
func TestFailedAuditKeepsOrdinaryFailuresRetryable(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.mp3")
	if err := os.WriteFile(real, []byte("not empty"), 0o600); err != nil {
		t.Fatal(err)
	}

	a := FailedAudit("id-2", real, errors.New("signal: killed"))
	if a.Verdict != model.LoudnessVerdictFailed {
		t.Errorf("verdict = %q, want %q", a.Verdict, model.LoudnessVerdictFailed)
	}

	// A path that cannot be stat'd at all is not evidence of an empty file.
	missing := FailedAudit("id-3", filepath.Join(dir, "gone.mp3"), errors.New("no such file"))
	if missing.Verdict != model.LoudnessVerdictFailed {
		t.Errorf("unreadable path: verdict = %q, want %q", missing.Verdict, model.LoudnessVerdictFailed)
	}
}
