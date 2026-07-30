package silencetrim

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"
)

func TestGenerationRecordFailureLeavesOriginalRetryable(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	library := t.TempDir()
	track := filepath.Join(library, "song.flac")
	createFixture(t, track, "anullsrc=r=44100:cl=stereo:d=1")
	mf := &model.MediaFile{ID: "journal-failure", LibraryPath: library, Path: "song.flac"}
	audit := Analyze(context.Background(), mf, track)
	before, err := ffmpeg.FileSHA256(track)
	if err != nil {
		t.Fatal(err)
	}

	originalWriter := persistGenerationRecord
	persistGenerationRecord = func(string, string, *model.SilenceTrimAudit) error {
		return errors.New("injected generation record failure")
	}
	failed, applyErr := Apply(context.Background(), mf, track, audit, "")
	persistGenerationRecord = originalWriter
	if applyErr == nil {
		t.Fatal("injected generation record failure was ignored")
	}
	if failed.Audit.ResultSHA256 != "" {
		t.Fatalf("uninstalled result hash was persisted: %#v", failed.Audit)
	}
	after, err := ffmpeg.FileSHA256(track)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatal("journal failure changed the original song")
	}

	retried, err := Apply(context.Background(), mf, track, failed.Audit, "")
	if err != nil {
		t.Fatal(err)
	}
	if !retried.Changed || retried.Audit.ResultSHA256 == "" {
		t.Fatalf("journal failure left the source unretryable: %#v", retried)
	}
}

func TestUnreadableUncommittedGenerationFailsClosed(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	library := t.TempDir()
	track := filepath.Join(library, "song.flac")
	createFixture(t, track, "anullsrc=r=44100:cl=stereo:d=1")
	mf := &model.MediaFile{ID: "broken-generation", LibraryPath: library, Path: "song.flac"}
	dryRun := Analyze(context.Background(), mf, track)
	before, err := ffmpeg.FileSHA256(track)
	if err != nil {
		t.Fatal(err)
	}
	recordPath := GenerationRecordPath("", library, mf.ID)
	if err := os.MkdirAll(filepath.Dir(recordPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recordPath, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := Apply(context.Background(), mf, track, dryRun, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Audit == nil ||
		result.Audit.Status != model.SilenceTrimStatusFailed ||
		result.Changed {
		t.Fatalf("unreadable uncommitted generation did not fail closed: %#v", result)
	}
	after, err := ffmpeg.FileSHA256(track)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatal("fail-closed journal handling changed the song")
	}
}

func TestApplyAndRestoreAreReversible(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	library := t.TempDir()
	track := filepath.Join(library, "song.flac")
	createFixture(t, track, "anullsrc=r=44100:cl=stereo:d=1")
	originalHash, err := ffmpeg.FileSHA256(track)
	if err != nil {
		t.Fatal(err)
	}
	mf := &model.MediaFile{
		ID:          "song-1",
		LibraryPath: library,
		Path:        "song.flac",
	}

	audit := Analyze(context.Background(), mf, track)
	if audit.Classification != model.SilenceTrimClassSafe {
		t.Fatalf("classification = %q: %s / %s", audit.Classification, audit.Reason, audit.Error)
	}
	result, err := Apply(context.Background(), mf, track, audit, "")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Audit.Integrity != model.SilenceTrimIntegrityVerified {
		t.Fatalf("result = %#v", result)
	}
	if result.Audit.SourceSHA256 == "" ||
		result.Audit.ResultSHA256 == "" ||
		result.Audit.BackupSHA256 == "" {
		t.Fatalf("generation hashes were not recorded: %#v", result.Audit)
	}
	if !ProcessedFileUnchanged(result.Audit, track) {
		t.Fatal("accepted result is not recognized as the processed file")
	}
	recovered, state, err := ReconcileGeneration(track, library, "", mf.ID)
	if err != nil {
		t.Fatal(err)
	}
	if state != GenerationStateResult ||
		recovered.ResultSHA256 != result.Audit.ResultSHA256 {
		t.Fatalf("durable result was not recoverable: state=%q audit=%#v", state, recovered)
	}
	recoveredApply, err := Apply(context.Background(), mf, track, audit, "")
	if err != nil {
		t.Fatal(err)
	}
	if !recoveredApply.Changed ||
		recoveredApply.Audit.ResultSHA256 != result.Audit.ResultSHA256 {
		t.Fatalf("rename-to-database crash recovery failed: %#v", recoveredApply)
	}
	persistedApply, err := Apply(context.Background(), mf, track, result.Audit, "")
	if err != nil {
		t.Fatal(err)
	}
	if persistedApply.Changed || !persistedApply.Reconciled {
		t.Fatalf("persisted generation did not request metadata reconciliation: %#v", persistedApply)
	}
	trimmedHash, err := ffmpeg.FileSHA256(track)
	if err != nil {
		t.Fatal(err)
	}
	if trimmedHash == originalHash {
		t.Fatal("trim did not replace the file")
	}
	trimmedBytes, err := os.ReadFile(track)
	if err != nil {
		t.Fatal(err)
	}
	changedBytes := append([]byte(nil), trimmedBytes...)
	changedBytes[len(changedBytes)-1] ^= 0x01
	if err := os.WriteFile(track, changedBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(context.Background(), mf, track, result.Audit, ""); err == nil {
		t.Fatal("restore overwrote a file that no longer matched the trim result")
	}
	if err := os.WriteFile(track, trimmedBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	restored, err := Restore(context.Background(), mf, track, result.Audit, "")
	if err != nil {
		t.Fatal(err)
	}
	if !restored.HasBackup {
		t.Fatal("restored audit lost the backup marker")
	}
	restoredHash, err := ffmpeg.FileSHA256(track)
	if err != nil {
		t.Fatal(err)
	}
	if restoredHash != originalHash {
		t.Fatalf("restored hash %s, want original %s", restoredHash, originalHash)
	}
	_, state, err = ReconcileGeneration(track, library, "", mf.ID)
	if err != nil || state != GenerationStateSource {
		t.Fatalf("restored generation state = %q, %v; want source", state, err)
	}
}

func TestRestoredBackupCanRotateForANewSourceGeneration(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	library := t.TempDir()
	track := filepath.Join(library, "song.flac")
	createFixture(t, track, "anullsrc=r=44100:cl=stereo:d=1")
	mf := &model.MediaFile{ID: "rotated", LibraryPath: library, Path: "song.flac"}

	first := Analyze(context.Background(), mf, track)
	applied, err := Apply(context.Background(), mf, track, first, "")
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Restore(context.Background(), mf, track, applied.Audit, "")
	if err != nil {
		t.Fatal(err)
	}
	oldBackupHash := restored.BackupSHA256

	if err := os.Remove(track); err != nil {
		t.Fatal(err)
	}
	createFixture(t, track, "anullsrc=r=44100:cl=stereo:d=1.2")
	newSourceHash, err := ffmpeg.FileSHA256(track)
	if err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(track)
	if err != nil {
		t.Fatal(err)
	}
	rotatedHash, err := ffmpeg.RotateSilenceTrimOriginal(
		track,
		stat.Mode(),
		library,
		"",
		mf.ID,
		oldBackupHash,
	)
	if err != nil {
		t.Fatal(err)
	}
	if rotatedHash != newSourceHash {
		t.Fatalf("prepared generation backup = %s, want %s", rotatedHash, newSourceHash)
	}
	second := Analyze(context.Background(), mf, track)
	if second.Classification != model.SilenceTrimClassSafe {
		t.Fatalf("second generation is not safe: %#v", second)
	}
	reapplied, err := Apply(context.Background(), mf, track, restored, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reapplied.Changed {
		t.Fatalf("new generation was not trimmed: %#v", reapplied)
	}
	if reapplied.Audit.BackupSHA256 != newSourceHash ||
		reapplied.Audit.BackupSHA256 == oldBackupHash {
		t.Fatalf(
			"backup did not rotate: old=%s new-source=%s audit=%s",
			oldBackupHash,
			newSourceHash,
			reapplied.Audit.BackupSHA256,
		)
	}
	secondRestored, err := Restore(context.Background(), mf, track, reapplied.Audit, "")
	if err != nil {
		t.Fatal(err)
	}
	finalHash, err := ffmpeg.FileSHA256(track)
	if err != nil {
		t.Fatal(err)
	}
	if finalHash != newSourceHash {
		t.Fatalf("new generation restored to %s, want %s", finalHash, newSourceHash)
	}
	retried, err := Apply(context.Background(), mf, track, secondRestored, "")
	if err != nil {
		t.Fatal(err)
	}
	if !retried.Changed || retried.Audit.BackupSHA256 != newSourceHash {
		t.Fatalf("rotated generation could not be reused after source-state reconciliation: %#v", retried)
	}
}

func TestNearSilenceRequiresReview(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	library := t.TempDir()
	track := filepath.Join(library, "quiet-edges.flac")
	createFixture(t, track, "sine=frequency=200:sample_rate=44100:duration=1,volume=-75dB")
	mf := &model.MediaFile{
		ID:          "song-2",
		LibraryPath: library,
		Path:        "quiet-edges.flac",
	}
	audit := Analyze(context.Background(), mf, track)
	if audit.Classification != model.SilenceTrimClassReview {
		t.Fatalf("classification = %q, want review: %s / %s", audit.Classification, audit.Reason, audit.Error)
	}
	if audit.LeadingKind != model.SilenceTrimEdgeQuiet ||
		audit.TrailingKind != model.SilenceTrimEdgeQuiet {
		t.Fatalf("edge kinds = %q / %q", audit.LeadingKind, audit.TrailingKind)
	}
}

func TestReviewProposalNeedsApprovalAndStaleApprovalIsCleared(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	library := t.TempDir()
	track := filepath.Join(library, "review.flac")
	createFixture(t, track, "sine=frequency=200:sample_rate=44100:duration=1,volume=-75dB")
	mf := &model.MediaFile{ID: "review", LibraryPath: library, Path: "review.flac"}

	audit := Analyze(context.Background(), mf, track)
	before, err := ffmpeg.FileSHA256(track)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply(context.Background(), mf, track, audit, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed {
		t.Fatal("a review proposal was applied without approval")
	}

	approved := *audit
	approved.Decision = model.SilenceTrimDecisionApprove
	approved.SourceSHA256 = "stale approval"
	result, err = Apply(context.Background(), mf, track, &approved, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed || result.Audit.Decision != model.SilenceTrimDecisionPending {
		t.Fatalf("stale approval was not cleared: %#v", result)
	}
	after, err := ffmpeg.FileSHA256(track)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatal("review refusal changed the song")
	}
}

func TestLeadingTrimProtectsSidecarsAndBookmarks(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	library := t.TempDir()
	track := filepath.Join(library, "song.flac")
	createFixture(t, track, "anullsrc=r=44100:cl=stereo:d=1")
	if err := os.WriteFile(filepath.Join(library, "album.cue"), []byte("FILE song.flac WAVE"), 0o600); err != nil {
		t.Fatal(err)
	}
	mf := &model.MediaFile{ID: "protected", LibraryPath: library, Path: "song.flac"}
	audit := Analyze(context.Background(), mf, track)
	if audit.Classification != model.SilenceTrimClassBlocked {
		t.Fatalf("album CUE was not protected: %#v", audit)
	}

	if err := os.Remove(filepath.Join(library, "album.cue")); err != nil {
		t.Fatal(err)
	}
	mf.HasAnyBookmark = true
	audit = Analyze(context.Background(), mf, track)
	if audit.Classification != model.SilenceTrimClassBlocked {
		t.Fatalf("another user's bookmark was not protected: %#v", audit)
	}
}

func TestResultIdentityUsesHashNotSizeOrMtime(t *testing.T) {
	track := filepath.Join(t.TempDir(), "song.bin")
	if err := os.WriteFile(track, []byte("aaaa"), 0o600); err != nil {
		t.Fatal(err)
	}
	hash, err := ffmpeg.FileSHA256(track)
	if err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(track)
	if err != nil {
		t.Fatal(err)
	}
	modified := stat.ModTime()
	audit := &model.SilenceTrimAudit{
		Status:           model.SilenceTrimStatusProcessed,
		SizeAfter:        stat.Size(),
		ResultModifiedAt: &modified,
		ResultSHA256:     hash,
	}
	if !ProcessedFileUnchanged(audit, track) {
		t.Fatal("matching result was not recognized")
	}
	if err := os.WriteFile(track, []byte("bbbb"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(track, modified, modified); err != nil {
		t.Fatal(err)
	}
	if ProcessedFileUnchanged(audit, track) {
		t.Fatal("same-size, same-mtime replacement bypassed result hashing")
	}
}

func createFixture(t *testing.T, path, edgeSource string) {
	t.Helper()
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", edgeSource,
		"-f", "lavfi", "-i", "sine=frequency=1000:sample_rate=44100:duration=2",
		"-f", "lavfi", "-i", edgeSource,
		"-filter_complex",
		"[0:a]pan=stereo|c0=c0|c1=c0[head];" +
			"[1:a]pan=stereo|c0=c0|c1=c0[tone];" +
			"[2:a]pan=stereo|c0=c0|c1=c0[tail];" +
			"[head][tone][tail]concat=n=3:v=0:a=1[out]",
		"-map", "[out]", path,
	}
	if output, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("creating fixture: %v: %s", err, output)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
