package loudness

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"
)

// countingNormalizer records how often a file is measured. Measuring decodes
// the track from end to end, so it is the cost that dominates a run.
type countingNormalizer struct{ calls int }

func (c *countingNormalizer) AnalyzeLoudness(context.Context, string, ffmpeg.LoudnessTarget) (*ffmpeg.LoudnessAnalysis, error) {
	c.calls++
	return &ffmpeg.LoudnessAnalysis{}, nil
}

func testMeasurement(lufs, truePeak, lra float64) *Measurement {
	return &Measurement{
		Probe: &ffmpeg.FileProbe{
			Codec: "mp3", BitRate: 320, SampleRate: 44100,
			Channels: 2, Duration: 210, Size: 8_400_000, HasArt: true,
		},
		LUFS: lufs, TruePeak: truePeak, LRA: lra,
	}
}

var auditTestTarget = ffmpeg.LoudnessTarget{IntegratedLUFS: testTarget, TruePeak: testCeiling}

// paths returns a library holding one track, plus an empty backup root.
func auditTestPaths(t *testing.T) (libraryPath, trackPath, backupFolder string) {
	t.Helper()
	libraryPath = t.TempDir()
	trackPath = filepath.Join(libraryPath, "song.mp3")
	return libraryPath, trackPath, t.TempDir()
}

func TestAuditFromOptimizeReusesTheRunsMeasurements(t *testing.T) {
	libraryPath, trackPath, backupFolder := auditTestPaths(t)
	normalizer := &countingNormalizer{}

	res := OptimizeResult{
		Changed:       true,
		BackupCreated: true,
		BeforeSet:     testMeasurement(-15.20, -3.10, 7.0),
		AfterSet:      testMeasurement(-12.58, -0.48, 7.0),
	}
	audit := AuditFromOptimize(context.Background(), normalizer, "track-1", libraryPath, trackPath,
		res, auditTestTarget, testTolerance, backupFolder)

	if normalizer.calls != 0 {
		t.Fatalf("measured the files again %d time(s); the run had already measured both", normalizer.calls)
	}
	if audit.Status != model.LoudnessStatusProcessed {
		t.Errorf("status = %q, want %q", audit.Status, model.LoudnessStatusProcessed)
	}
	if audit.LufsBefore == nil || *audit.LufsBefore != -15.20 {
		t.Errorf("before snapshot = %v, want -15.20", audit.LufsBefore)
	}
	if audit.LufsAfter == nil || *audit.LufsAfter != -12.58 {
		t.Errorf("after snapshot = %v, want -12.58", audit.LufsAfter)
	}
	if audit.GainApplied == nil || math.Abs(*audit.GainApplied-2.62) > 0.001 {
		t.Errorf("gain = %v, want 2.62", audit.GainApplied)
	}
	// The peak moved by exactly the gain and the loudness range did not move,
	// which is what a constant gain looks like.
	if audit.Action != model.LoudnessActionGain {
		t.Errorf("action = %q, want %q", audit.Action, model.LoudnessActionGain)
	}
	if audit.Verdict != model.LoudnessVerdictSafe {
		t.Errorf("verdict = %q, want %q", audit.Verdict, model.LoudnessVerdictSafe)
	}
	if audit.Phase != PhaseDone {
		t.Errorf("phase = %d, want %d: the track landed on target", audit.Phase, PhaseDone)
	}
}

func TestAuditFromOptimizeReusesTheMeasurementWhenNothingChanged(t *testing.T) {
	libraryPath, trackPath, backupFolder := auditTestPaths(t)
	normalizer := &countingNormalizer{}

	// The engine measured the track, found it already on target and left it.
	res := OptimizeResult{BeforeSet: testMeasurement(-12.61, -1.90, 7.0)}
	audit := AuditFromOptimize(context.Background(), normalizer, "track-1", libraryPath, trackPath,
		res, auditTestTarget, testTolerance, backupFolder)

	if normalizer.calls != 0 {
		t.Fatalf("measured the file again %d time(s); the run had already measured it", normalizer.calls)
	}
	if audit.Status != model.LoudnessStatusAnalyzed {
		t.Errorf("status = %q, want %q", audit.Status, model.LoudnessStatusAnalyzed)
	}
	if audit.LufsBefore == nil || *audit.LufsBefore != -12.61 {
		t.Errorf("before snapshot = %v, want -12.61", audit.LufsBefore)
	}
	if audit.LufsAfter != nil {
		t.Errorf("after snapshot = %v, want none: nothing was rewritten", *audit.LufsAfter)
	}
}

// The shortcut is only sound when this run is what stored the original. A
// backup is never overwritten, so on a second pass the stored original predates
// everything the run measured and has to be read rather than assumed.
func TestAuditFromOptimizeWillNotTreatAnIntermediateStateAsTheOriginal(t *testing.T) {
	libraryPath, trackPath, backupFolder := auditTestPaths(t)
	stored := ffmpeg.LoudnessBackupPath(backupFolder, libraryPath, "track-1", trackPath)
	if err := os.MkdirAll(filepath.Dir(stored), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stored, []byte("an earlier run's original"), 0o600); err != nil {
		t.Fatal(err)
	}
	normalizer := &countingNormalizer{}

	res := OptimizeResult{
		Changed:       true,
		BackupCreated: false, // the original was already there
		BeforeSet:     testMeasurement(-13.90, -2.80, 7.0),
		AfterSet:      testMeasurement(-12.60, -1.50, 7.0),
	}
	audit := AuditFromOptimize(context.Background(), normalizer, "track-1", libraryPath, trackPath,
		res, auditTestTarget, testTolerance, backupFolder)

	if !audit.HasBackup {
		t.Fatal("the stored original was not found")
	}
	// The dummy backup is not decodable, so reading it fails - which is itself
	// the proof that the shortcut was refused rather than silently taken.
	if !strings.Contains(audit.Error, "reading backup") {
		t.Fatalf("error = %q, want the stored original to have been read", audit.Error)
	}
	if audit.LufsBefore != nil && *audit.LufsBefore == -13.90 {
		t.Error("recorded the run's own starting point as the untouched original")
	}
}

// A run that builds a file and then rejects it is the only thing that knows
// why. Discarding that leaves a track which refuses to process looking exactly
// like one nothing was ever attempted on - which is what made three stuck
// tracks take a full manual re-run of the pipeline to explain.
func TestAuditFromOptimizeRecordsWhyAFileWasRejected(t *testing.T) {
	libraryPath, trackPath, backupFolder := auditTestPaths(t)
	const why = "true peak -0.37 dBTP exceeds the -0.50 dBTP ceiling"

	res := OptimizeResult{
		BeforeSet: testMeasurement(-13.04, -0.07, 6.7),
		Rejected:  why,
	}
	audit := AuditFromOptimize(context.Background(), &countingNormalizer{}, "track-1",
		libraryPath, trackPath, res, auditTestTarget, testTolerance, backupFolder)

	if audit.Error != why {
		t.Errorf("error = %q, want the rejection reason %q", audit.Error, why)
	}
	// The measurements still stand: the track was read, just not rewritten.
	if audit.LufsBefore == nil || *audit.LufsBefore != -13.04 {
		t.Errorf("lufsBefore = %v, want -13.04", audit.LufsBefore)
	}
}

func TestAuditFromOptimizeLeavesTheErrorClearWhenNothingWasRejected(t *testing.T) {
	libraryPath, trackPath, backupFolder := auditTestPaths(t)
	res := OptimizeResult{BeforeSet: testMeasurement(-12.61, -1.90, 7.0)}
	audit := AuditFromOptimize(context.Background(), &countingNormalizer{}, "track-1",
		libraryPath, trackPath, res, auditTestTarget, testTolerance, backupFolder)

	if audit.Error != "" {
		t.Errorf("error = %q, want empty: the track was simply already on target", audit.Error)
	}
}
