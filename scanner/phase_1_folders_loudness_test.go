package scanner

import "testing"

func TestEffectiveLoudnessToleranceAllowsTenthLUFS(t *testing.T) {
	if got := effectiveLoudnessTolerance(0.1); got != 0.1 {
		t.Fatalf("effectiveLoudnessTolerance(0.1) = %v, want 0.1", got)
	}
}

func TestShouldNormalizeLoudnessUsesConfiguredTolerance(t *testing.T) {
	const target = -12.6
	const tolerance = 0.1

	if shouldNormalizeLoudness(-12.7, target, tolerance) {
		t.Fatal("expected exact tolerance boundary to be considered normalized")
	}
	if !shouldNormalizeLoudness(-12.71, target, tolerance) {
		t.Fatal("expected values outside 0.1 LUFS tolerance to be normalized")
	}
}
