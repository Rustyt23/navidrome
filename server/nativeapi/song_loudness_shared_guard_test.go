package nativeapi

import (
	"testing"

	"github.com/navidrome/navidrome/core/loudness"
)

func TestLoudnessNativeJobsShareScannerGuardAndReleaseOnFinish(t *testing.T) {
	finishLibraryLoudness()
	finishLoudnessAnalyze()
	finishRestoreLoudness()
	t.Cleanup(finishLoudnessAnalyze)
	scanner, busy := loudness.ClaimFileWork("automatic new-song LUFS")
	if busy != "" {
		t.Fatal(busy)
	}
	defer scanner()
	if _, ok := beginLoudnessAnalyze(t.Context()); ok {
		t.Fatal("native analysis overlapped automatic processing")
	}
	if got := loudnessFileWorkBusy(); got != "automatic new-song LUFS" {
		t.Fatalf("native busy report = %q", got)
	}
	scanner()
	if _, ok := beginLoudnessAnalyze(t.Context()); !ok {
		t.Fatal("analysis did not start after scanner finished")
	}
	if done, busy := loudness.ClaimFileWork("scanner"); done != nil || busy != "a LUFS analysis" {
		t.Fatalf("scanner overlapped analysis: %v, %q", done != nil, busy)
	}
	finishLoudnessAnalyze()
	next, busy := loudness.ClaimFileWork("scanner")
	if busy != "" {
		t.Fatalf("native finish leaked shared claim: %q", busy)
	}
	next()
}
