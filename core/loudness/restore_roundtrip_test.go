package loudness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"
)

// writeQuietTestMP3 makes a real file that is genuinely off target, so the
// optimiser has something to do: a tone 15 dB down measures around -18 LUFS
// with peaks nowhere near the ceiling, which is the plain constant-gain case.
func writeQuietTestMP3(t *testing.T, path string, seconds float64) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("ffmpeg", "-y", "-nostdin", "-hide_banner", "-f", "lavfi",
		"-i", fmt.Sprintf("sine=frequency=440:duration=%g", seconds),
		"-af", "volume=-15dB",
		"-c:a", "libmp3lame", "-b:a", "192k", "-ar", "44100", "-ac", "2", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generating test audio: %v: %s", err, out)
	}
}

func digest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// The promise the backups exist to keep, tested against real audio rather than
// against a placeholder: optimise a song for real, then restore it, and the
// file has to come back byte for byte. Not "close enough to the original" -
// nothing is re-encoded on the way back, so anything short of identical means
// the wrong bytes were written.
func TestOptimiseThenRestoreReturnsTheExactOriginalFile(t *testing.T) {
	library, backupFolder := t.TempDir(), t.TempDir()
	track := filepath.Join(library, "Song.mp3")
	writeQuietTestMP3(t, track, 10)

	original := digest(t, track)
	originalSize := func() int64 {
		info, err := os.Stat(track)
		if err != nil {
			t.Fatal(err)
		}
		return info.Size()
	}()

	ctx := context.Background()
	normalizer := ffmpeg.NewLoudnessNormalizer()
	target := ffmpeg.LoudnessTarget{IntegratedLUFS: -12.6, TruePeak: -0.5, LRA: 11}
	opts := OptimizeOptions{
		Target:       target,
		Tolerance:    0.2,
		Backup:       true,
		LibraryPath:  library,
		BackupFolder: backupFolder,
		MediaFileID:  "track-1",
	}

	res, err := Optimize(ctx, normalizer, track, DecisionPending, opts)
	if err != nil {
		t.Fatalf("optimising: %v", err)
	}
	if !res.Changed {
		t.Fatalf("fixture was not optimised, so there is nothing to restore (phase %d, rejected %q)",
			res.Phase, res.Rejected)
	}
	if digest(t, track) == original {
		t.Fatal("the file on disk did not change, so the test is not exercising a restore")
	}
	if !res.BackupCreated {
		t.Fatal("the run did not report storing the original")
	}

	// The audit the restore is handed, as the API would build it.
	audit := AuditFromOptimize(ctx, normalizer, "track-1", library, track, res,
		target, opts.Tolerance, backupFolder)

	restored, err := Restore(ctx, normalizer, "track-1", library, track, audit,
		target, opts.Tolerance, backupFolder)
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}

	if got := digest(t, track); got != original {
		t.Errorf("restored file is not the original\n got %s\nwant %s", got, original)
	}
	if info, err := os.Stat(track); err != nil {
		t.Fatal(err)
	} else if info.Size() != originalSize {
		t.Errorf("restored size %d, want %d", info.Size(), originalSize)
	}

	// The stored original is kept, so the song can be optimised and restored
	// again. Moving it would spend the one irreplaceable copy on an undo.
	if ffmpeg.FindLoudnessBackup(backupFolder, library, "track-1", track) == "" {
		t.Error("the stored original was consumed by the restore")
	}

	// Marked as restored, so the next library sweep leaves it alone instead of
	// normalising it straight back.
	if !restored.Audit.IsRestored() {
		t.Error("a restored song must be marked so a sweep does not undo it")
	}

	// The probe describes the file now on disk, so the song's own row can be
	// brought back in step without waiting for a scan.
	if restored.Probe == nil {
		t.Fatal("no description of the restored file was returned")
	}
	if restored.Probe.Size != originalSize {
		t.Errorf("described size %d, want the original %d", restored.Probe.Size, originalSize)
	}

	// The record has to describe the file that is now on disk, not the one that
	// was replaced - otherwise every page reports the track as still processed.
	if restored.Audit.Status != model.LoudnessStatusProcessed &&
		restored.Audit.Status != model.LoudnessStatusAnalyzed {
		t.Errorf("unexpected status after restore: %q", restored.Audit.Status)
	}
	if restored.Audit.Status != model.LoudnessStatusAnalyzed {
		t.Errorf("a restored track is no longer processed, got status %q", restored.Audit.Status)
	}
	if restored.Audit.LufsAfter != nil || restored.Audit.GainApplied != nil ||
		restored.Audit.NullResidual != nil {
		t.Error("the record still carries measurements of the audio that was replaced")
	}
	if restored.Audit.LufsBefore == nil {
		t.Fatal("the record lost the original measurement")
	}
	// The before snapshot describes the file now on disk, so it must still agree
	// with what the run measured before it touched anything.
	if got, want := *restored.Audit.LufsBefore, res.OldLUFS; got != want {
		t.Errorf("original loudness became %.2f, want %.2f", got, want)
	}
	if restored.LUFS != res.OldLUFS {
		t.Errorf("reported loudness %.2f, want the original %.2f", restored.LUFS, res.OldLUFS)
	}
}

// What the page and the tag say after a restore has to be what the file is.
//
// The way this broke: "Fetch original LUFS" measured whatever was on disk and
// called it the original. Run after a song had been normalized - which happens
// the moment someone clears the analysis data and fetches again - it recorded
// the normalized loudness as the original. Restore then trusted that snapshot
// to describe the file it had just put back, so a track returned to -37 was
// reported, and tagged, at -12.6.
func TestRestoreReportsTheLoudnessTheFileActuallyHas(t *testing.T) {
	library, backupFolder := t.TempDir(), t.TempDir()
	track := filepath.Join(library, "Song.mp3")
	writeQuietTestMP3(t, track, 10)

	ctx := context.Background()
	normalizer := ffmpeg.NewLoudnessNormalizer()
	target := ffmpeg.LoudnessTarget{IntegratedLUFS: -12.6, TruePeak: -0.5, LRA: 11}
	opts := OptimizeOptions{
		Target: target, Tolerance: 0.2, Backup: true,
		LibraryPath: library, BackupFolder: backupFolder, MediaFileID: "track-1",
	}

	res, err := Optimize(ctx, normalizer, track, DecisionPending, opts)
	if err != nil || !res.Changed {
		t.Fatalf("setup: optimise did not change the file: %v (rejected %q)", err, res.Rejected)
	}

	// The analysis data was cleared, so this song is measured from scratch -
	// while it is normalized, with its original sitting in the backup folder.
	audit := MeasureOriginal(ctx, normalizer, "track-1", library, track, target, 0.2, backupFolder)
	if !audit.HasBackup {
		t.Error("a song with a stored original must be recorded as having one")
	}
	if audit.LufsBefore == nil {
		t.Fatal("no original loudness was recorded")
	}
	if got := *audit.LufsBefore; math.Abs(got-res.OldLUFS) > 0.5 {
		t.Errorf("recorded the original as %.2f LUFS, but it was %.2f - the file on disk was measured instead of the stored original",
			got, res.OldLUFS)
	}

	restored, err := Restore(ctx, normalizer, "track-1", library, track, audit, target, 0.2, backupFolder)
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}

	actual, err := Measure(ctx, normalizer, track, target)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(actual.LUFS-restored.LUFS) > 0.5 {
		t.Errorf("restore reported %.2f LUFS but the file is %.2f - off by %.2f dB",
			restored.LUFS, actual.LUFS, math.Abs(actual.LUFS-restored.LUFS))
	}
	if restored.Audit.LufsBefore == nil || math.Abs(*restored.Audit.LufsBefore-actual.LUFS) > 0.5 {
		t.Error("the stored record does not describe the file that is now on disk")
	}
	if restored.Audit.Status != model.LoudnessStatusAnalyzed {
		t.Errorf("a restored track is not processed, got status %q", restored.Audit.Status)
	}
}

// The same protection, reached the other way: a record that never had a
// snapshot at all must not be believed either.
func TestRestoreMeasuresWhenTheRecordDidNotComeFromTheStoredOriginal(t *testing.T) {
	library, backupFolder := t.TempDir(), t.TempDir()
	track := filepath.Join(library, "Song.mp3")
	writeQuietTestMP3(t, track, 10)

	ctx := context.Background()
	normalizer := ffmpeg.NewLoudnessNormalizer()
	target := ffmpeg.LoudnessTarget{IntegratedLUFS: -12.6, TruePeak: -0.5, LRA: 11}
	opts := OptimizeOptions{
		Target: target, Tolerance: 0.2, Backup: true,
		LibraryPath: library, BackupFolder: backupFolder, MediaFileID: "track-1",
	}
	res, err := Optimize(ctx, normalizer, track, DecisionPending, opts)
	if err != nil || !res.Changed {
		t.Fatalf("setup: %v", err)
	}

	// A record carrying the normalized loudness and no stored original: the
	// exact shape that used to be trusted.
	wrong := &model.LoudnessAudit{
		MediaFileID: "track-1",
		Status:      model.LoudnessStatusAnalyzed,
		LufsBefore:  f64(res.NewLUFS),
		HasBackup:   false,
	}

	restored, err := Restore(ctx, normalizer, "track-1", library, track, wrong, target, 0.2, backupFolder)
	if err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if math.Abs(restored.LUFS-res.OldLUFS) > 0.5 {
		t.Errorf("reported %.2f LUFS, want the real original %.2f", restored.LUFS, res.OldLUFS)
	}
	if !restored.Audit.HasBackup {
		t.Error("the rebuilt record forgot that an original is still stored")
	}
}

// A second pass has to be undoable too. The stored original is never
// overwritten, so restoring after re-processing must still return the file the
// client first handed over - not the state it was in after the first pass.
func TestRestoreAfterASecondOptimiseStillReturnsTheFirstOriginal(t *testing.T) {
	library, backupFolder := t.TempDir(), t.TempDir()
	track := filepath.Join(library, "Song.mp3")
	writeQuietTestMP3(t, track, 10)

	original := digest(t, track)

	ctx := context.Background()
	normalizer := ffmpeg.NewLoudnessNormalizer()
	target := ffmpeg.LoudnessTarget{IntegratedLUFS: -12.6, TruePeak: -0.5, LRA: 11}
	opts := OptimizeOptions{
		Target: target, Tolerance: 0.2, Backup: true,
		LibraryPath: library, BackupFolder: backupFolder, MediaFileID: "track-1",
	}

	first, err := Optimize(ctx, normalizer, track, DecisionPending, opts)
	if err != nil || !first.Changed {
		t.Fatalf("first optimise did not change the file: %v (rejected %q)", err, first.Rejected)
	}
	// Push it off target again so there is something for a second pass to do.
	off := filepath.Join(library, "off.mp3")
	cmd := exec.Command("ffmpeg", "-y", "-nostdin", "-hide_banner", "-i", track,
		"-af", "volume=-6dB", "-c:a", "libmp3lame", "-b:a", "192k", "-ar", "44100", "-ac", "2", off)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("re-quietening: %v: %s", err, out)
	}
	if err := os.Rename(off, track); err != nil {
		t.Fatal(err)
	}

	second, err := Optimize(ctx, normalizer, track, DecisionPending, opts)
	if err != nil {
		t.Fatalf("second optimise: %v", err)
	}
	if second.BackupCreated {
		t.Error("the second pass overwrote the stored original")
	}

	if _, err := Restore(ctx, normalizer, "track-1", library, track, nil,
		target, opts.Tolerance, backupFolder); err != nil {
		t.Fatalf("restoring: %v", err)
	}
	if got := digest(t, track); got != original {
		t.Errorf("restore returned the wrong generation of the file\n got %s\nwant %s", got, original)
	}
}
