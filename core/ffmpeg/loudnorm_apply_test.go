package ffmpeg

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"testing"
)

// Test actual decoded samples: container duration alone cannot reveal a delay
// that replaces the end of the recording with silence at the beginning.
func TestLimiterPreservesBeginningTimingAndEnd(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	for _, rate := range []int{44100, 48000, 192000} {
		t.Run(fmt.Sprint(rate), func(t *testing.T) {
			input, output := filepath.Join(t.TempDir(), "input.wav"), filepath.Join(t.TempDir(), "output.wav")
			positions := []int{0, rate / 10, rate - 1}
			source := fmt.Sprintf("aevalsrc=0.5*(eq(n\\,0)+eq(n\\,%d)+eq(n\\,%d)):s=%d:d=1", positions[1], positions[2], rate)
			cmd := exec.Command("ffmpeg", "-nostdin", "-hide_banner", "-f", "lavfi", "-i", source, "-c:a", "pcm_s16le", input)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("create impulses: %v: %s", err, out)
			}
			probe, err := ProbeFile(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if err := Apply(context.Background(), input, output, ApplySpec{LimitTruePeak: true, CeilingDB: -6, Source: probe}); err != nil {
				t.Fatal(err)
			}
			decoded, err := exec.Command("ffmpeg", "-v", "error", "-i", output, "-f", "f32le", "-c:a", "pcm_f32le", "-").Output()
			if err != nil {
				t.Fatal(err)
			}
			if len(decoded) != rate*4 {
				t.Fatalf("got %d samples, want %d", len(decoded)/4, rate)
			}
			var peaks []int
			for i := 0; i < rate; i++ {
				value := math.Float32frombits(binary.LittleEndian.Uint32(decoded[i*4:]))
				if math.Abs(float64(value)) > 0.2 {
					peaks = append(peaks, i)
				}
			}
			if fmt.Sprint(peaks) != fmt.Sprint(positions) {
				t.Fatalf("impulse positions = %v, want %v; audio shifted or tail lost", peaks, positions)
			}
		})
	}
}
