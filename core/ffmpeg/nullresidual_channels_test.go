package ffmpeg

import (
	"context"
	"math"
	"os/exec"
	"path/filepath"
	"testing"
)

// The null test is the check that proves normalization did not damage the
// audio. It used to compare only the left channel - pan=mono|c0=c0-c2 - so
// anything that changed on the right alone nulled perfectly and was reported as
// untouched.
func buildPair(t *testing.T, dir, name, gainFilter string) string {
	t.Helper()
	ff, err := ffmpegCmd()
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	out := filepath.Join(dir, name+".wav")
	args := []string{"-nostdin", "-hide_banner", "-y", "-filter_complex",
		"sine=frequency=440:duration=4[l];sine=frequency=550:duration=4[r];[l][r]amerge=inputs=2" + gainFilter,
		"-c:a", "pcm_s16le", out}
	if o, err := exec.Command(ff, args...).CombinedOutput(); err != nil {
		t.Fatalf("building %s: %v\n%s", name, err, o)
	}
	return out
}

func TestNullResidualHearsBothChannels(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	before := buildPair(t, dir, "before", "")
	// A 0.6 dB change confined to the RIGHT channel. Clearly a change; the old
	// filter reported it as digital silence.
	rightOnly := buildPair(t, dir, "right",
		",channelsplit=channel_layout=stereo[sl][sr];[sr]volume=0.6dB[sr2];[sl][sr2]amerge=inputs=2")

	got, err := NullResidual(ctx, before, rightOnly, 0)
	if err != nil {
		t.Fatalf("null test failed: %v", err)
	}
	if math.IsInf(got, -1) || got < -100 {
		t.Fatalf("a change in the right channel measured as silence (%.2f dB): the null test is deaf to it", got)
	}
	t.Logf("right-channel-only change measured at %.2f dB", got)
}

// Changing the wrong thing here would move every verdict on every page: the
// null thresholds were calibrated against the old filter's numbers. A gain
// applies to both channels equally, which is the ordinary case, and the reading
// for it has to stay where it was.
func TestNullResidualKeepsItsCalibration(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	before := buildPair(t, dir, "before", "")
	bothChannels := buildPair(t, dir, "both", ",volume=0.6dB")

	got, err := NullResidual(ctx, before, bothChannels, 0)
	if err != nil {
		t.Fatalf("null test failed: %v", err)
	}
	// Measured with the previous mono filter on the same pair: -44.19 dB.
	// Summing the two differences instead of panning them would have read about
	// 3 dB louder and quietly shifted every threshold with it.
	const oldFilterReading = -44.19
	if math.Abs(got-oldFilterReading) > 0.5 {
		t.Errorf("reading moved from %.2f to %.2f dB; the thresholds were calibrated on the old figure",
			oldFilterReading, got)
	}
	t.Logf("both-channel change measured at %.2f dB (old filter: %.2f)", got, oldFilterReading)
}

// Identical files must still null to silence, or every song looks damaged.
func TestNullResidualOnIdenticalFiles(t *testing.T) {
	dir := t.TempDir()
	before := buildPair(t, dir, "before", "")
	got, err := NullResidual(context.Background(), before, before, 0)
	if err != nil {
		t.Fatalf("null test failed: %v", err)
	}
	if got > -90 {
		t.Errorf("identical files measured at %.2f dB, expected silence", got)
	}
}
