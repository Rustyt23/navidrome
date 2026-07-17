package ffmpeg

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type blockingLoudnessNormalizer struct {
	normalizeStarted chan struct{}
	releaseNormalize chan struct{}
}

func (n *blockingLoudnessNormalizer) AnalyzeLoudness(_ context.Context, path string, _ LoudnessTarget) (*LoudnessAnalysis, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	integrated := -20.0
	if bytes.Equal(data, []byte("normalized")) {
		integrated = -12.0
	}
	return &LoudnessAnalysis{InputIntegrated: integrated}, nil
}

func (n *blockingLoudnessNormalizer) NormalizeLoudness(ctx context.Context, _, outputPath string, _ LoudnessTarget, _ LoudnessAnalysis) error {
	select {
	case n.normalizeStarted <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-n.releaseNormalize:
	case <-ctx.Done():
		return ctx.Err()
	}
	return os.WriteFile(outputPath, []byte("normalized"), 0o600)
}

func TestNormalizeToBestSerializesWritersForSameTrack(t *testing.T) {
	trackPath := filepath.Join(t.TempDir(), "song.mp3")
	if err := os.WriteFile(trackPath, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}

	normalizer := &blockingLoudnessNormalizer{
		normalizeStarted: make(chan struct{}, 2),
		releaseNormalize: make(chan struct{}),
	}
	target := LoudnessTarget{IntegratedLUFS: -12}
	opts := NormalizeOptions{Tolerance: 0.1, MaxAttempts: 1}

	type result struct {
		value NormalizeResult
		err   error
	}
	firstDone := make(chan result, 1)
	go func() {
		value, err := NormalizeToBest(context.Background(), normalizer, trackPath, target, opts)
		firstDone <- result{value: value, err: err}
	}()

	select {
	case <-normalizer.normalizeStarted:
	case <-time.After(time.Second):
		t.Fatal("first normalization did not start")
	}

	released := false
	defer func() {
		if !released {
			close(normalizer.releaseNormalize)
		}
	}()

	secondCalling := make(chan struct{})
	secondDone := make(chan result, 1)
	go func() {
		close(secondCalling)
		value, err := NormalizeToBest(context.Background(), normalizer, trackPath, target, opts)
		secondDone <- result{value: value, err: err}
	}()
	<-secondCalling

	select {
	case <-normalizer.normalizeStarted:
		t.Fatal("second normalization wrote the track while the first writer held it")
	case <-time.After(100 * time.Millisecond):
	}

	close(normalizer.releaseNormalize)
	released = true

	first := <-firstDone
	if first.err != nil {
		t.Fatal(first.err)
	}
	if !first.value.Changed {
		t.Fatal("first normalization did not replace the track")
	}

	second := <-secondDone
	if second.err != nil {
		t.Fatal(second.err)
	}
	if second.value.Changed {
		t.Fatal("second normalization rewrote an already-normalized track")
	}

	select {
	case <-normalizer.normalizeStarted:
		t.Fatal("more than one normalization write occurred")
	default:
	}
}
