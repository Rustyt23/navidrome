package loudness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"
)

func f64(v float64) *float64 { return &v }

// writeTestMP3 makes a real, decodable file. Restoring now refuses to overwrite
// a song on the strength of a backup it has not been able to read, so these
// tests cannot use placeholder text where the stored original goes.
func writeTestMP3(t *testing.T, path string, seconds float64) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("ffmpeg", "-y", "-nostdin", "-hide_banner", "-f", "lavfi",
		"-i", fmt.Sprintf("sine=frequency=440:duration=%g", seconds),
		"-c:a", "libmp3lame", "-b:a", "128k", "-ar", "44100", "-ac", "2", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generating test audio: %v: %s", err, out)
	}
}

// processedAudit is the record of a track that was measured, rewritten and
// verified - the state a restore has to undo.
func processedAudit() *model.LoudnessAudit {
	return &model.LoudnessAudit{
		MediaFileID:  "track-1",
		Status:       model.LoudnessStatusProcessed,
		Verdict:      model.LoudnessVerdictSafe,
		Action:       model.LoudnessActionGain,
		Phase:        PhaseDone,
		Decision:     DecisionLimit,
		LufsBefore:   f64(-15.20),
		TpBefore:     f64(-3.10),
		LraBefore:    f64(7.0),
		LufsAfter:    f64(-12.58),
		TpAfter:      f64(-0.48),
		LraAfter:     f64(7.0),
		GainApplied:  f64(2.62),
		NullResidual: f64(-31.4),
		CodecBefore:  "mp3",
		CodecAfter:   "mp3",
		ArtBefore:    true,
		ArtAfter:     true,
		HasBackup:    true,
	}
}

// setupRestore leaves a real original in the backup folder and a rewritten file
// in the library, which is the state a restore is asked to undo.
func setupRestore(t *testing.T) (library, track, backupFolder string) {
	t.Helper()
	library, backupFolder = t.TempDir(), t.TempDir()
	track = filepath.Join(library, "Song.mp3")

	writeTestMP3(t, track, 1)
	if err := ffmpeg.BackupOriginal(track, 0o644, library, backupFolder, "track-1"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(track, []byte("the rewritten file"), 0o644); err != nil {
		t.Fatal(err)
	}
	return library, track, backupFolder
}

func storedOriginal(t *testing.T, library, track, backupFolder string) string {
	t.Helper()
	path := ffmpeg.FindLoudnessBackup(backupFolder, library, "track-1", track)
	if path == "" {
		t.Fatal("no stored original was set up")
	}
	return path
}

func TestRestoreReturnsTheTrackToItsOriginalWithoutMeasuring(t *testing.T) {
	library, track, backupFolder := setupRestore(t)
	normalizer := &countingNormalizer{}
	want, err := os.ReadFile(storedOriginal(t, library, track, backupFolder))
	if err != nil {
		t.Fatal(err)
	}

	res, err := Restore(context.Background(), normalizer, "track-1", library, track,
		processedAudit(), auditTestTarget, testTolerance, backupFolder)
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(track)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Error("the track is not byte-for-byte the stored original")
	}
	// Checking the backup is readable is a cheap probe; measuring it would mean
	// decoding it end to end, and its measurement is already on record.
	if normalizer.calls != 0 {
		t.Errorf("measured the restored file %d time(s); its measurement was already on record", normalizer.calls)
	}
	if res.LUFS != -15.20 {
		t.Errorf("LUFS = %v, want the original -15.20 for the stored tag", res.LUFS)
	}
}

func TestRestoreRewritesTheRecordToDescribeTheOriginal(t *testing.T) {
	library, track, backupFolder := setupRestore(t)
	// The shipped ceiling rather than the tighter one the planner tests pin, so
	// the recomputed phase is the one production would reach.
	target := ffmpeg.LoudnessTarget{IntegratedLUFS: testTarget, TruePeak: -0.5}

	res, err := Restore(context.Background(), &countingNormalizer{}, "track-1", library, track,
		processedAudit(), target, testTolerance, backupFolder)
	if err != nil {
		t.Fatal(err)
	}
	audit := res.Audit

	if audit.Status != model.LoudnessStatusAnalyzed {
		t.Errorf("status = %q, want %q: nothing on disk has been processed now",
			audit.Status, model.LoudnessStatusAnalyzed)
	}
	// Everything describing the rewrite referred to audio that no longer
	// exists. Leaving any of it would report the track as still processed.
	for name, got := range map[string]*float64{
		"lufsAfter":    audit.LufsAfter,
		"tpAfter":      audit.TpAfter,
		"lraAfter":     audit.LraAfter,
		"gainApplied":  audit.GainApplied,
		"nullResidual": audit.NullResidual,
	} {
		if got != nil {
			t.Errorf("%s = %v, want it cleared", name, *got)
		}
	}
	if audit.CodecAfter != "" {
		t.Errorf("codecAfter = %q, want it cleared", audit.CodecAfter)
	}
	if audit.Verdict == model.LoudnessVerdictSafe {
		t.Error("kept a verdict about a change that has been undone")
	}
	if audit.LufsBefore == nil || *audit.LufsBefore != -15.20 {
		t.Errorf("lufsBefore = %v, want -15.20 carried over", audit.LufsBefore)
	}
	if !audit.HasBackup {
		t.Error("hasBackup = false, but the stored original is kept")
	}
	// The record said PhaseDone, because the track had been brought to target.
	// It is not on target any more, so carrying that over would hide it from
	// every run. -15.20 with peaks at -3.10 is reachable by level alone.
	if audit.Phase != PhaseGain {
		t.Errorf("phase = %d, want %d: the work to be done is what it was before", audit.Phase, PhaseGain)
	}
}

// Restoring destroys the working song first and would discover a bad backup
// second, with nothing left to go back to. So the backup is read before
// anything is touched.
func TestRestoreRefusesAnUnreadableStoredOriginal(t *testing.T) {
	library, track, backupFolder := setupRestore(t)
	if err := os.WriteFile(storedOriginal(t, library, track, backupFolder),
		[]byte("not audio at all"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Restore(context.Background(), &countingNormalizer{}, "track-1", library, track,
		processedAudit(), auditTestTarget, testTolerance, backupFolder)
	if err == nil {
		t.Fatal("restored from a file that is not audio")
	}
	if !strings.Contains(err.Error(), "could not be read as audio") {
		t.Errorf("error = %q, want it to say the stored original is unreadable", err)
	}
	got, readErr := os.ReadFile(track)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "the rewritten file" {
		t.Error("the song was overwritten before the backup had been checked")
	}
}

// The case that made this necessary: backups used to be keyed by library path,
// so a song arriving where a deleted one used to live inherited its backup.
// Restoring then replaced the client's song with a different recording.
func TestRestoreRefusesAStoredOriginalBelongingToAnotherSong(t *testing.T) {
	library, track, backupFolder := setupRestore(t)
	// A perfectly valid file - just not this song. Its length gives it away.
	writeTestMP3(t, storedOriginal(t, library, track, backupFolder), 5)

	previous := processedAudit()
	previous.DurationBefore = 1.0

	_, err := Restore(context.Background(), &countingNormalizer{}, "track-1", library, track,
		previous, auditTestTarget, testTolerance, backupFolder)
	if err == nil {
		t.Fatal("restored a backup belonging to a different recording")
	}
	if !strings.Contains(err.Error(), "does not match this song") {
		t.Errorf("error = %q, want it to say the stored original is not this song", err)
	}
	got, readErr := os.ReadFile(track)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "the rewritten file" {
		t.Error("the song was replaced with another recording")
	}
}

func TestRestoreMeasuresWhenThereIsNoRecordToRestoreFrom(t *testing.T) {
	library, track, backupFolder := setupRestore(t)
	normalizer := &countingNormalizer{}

	// With no record there is nothing to rebuild the audit from, so the
	// restored file has to be measured. Nothing to check the backup against
	// either, beyond it being readable audio.
	res, err := Restore(context.Background(), normalizer, "track-1", library, track,
		nil, auditTestTarget, testTolerance, backupFolder)
	if err != nil {
		t.Fatal(err)
	}
	if normalizer.calls == 0 {
		t.Error("did not measure, though there was no record to restore from")
	}
	if res.Audit == nil || res.Audit.LufsBefore == nil {
		t.Error("no before snapshot was produced for the restored file")
	}
}
