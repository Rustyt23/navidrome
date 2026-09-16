package ffmpeg

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The shape ffmpeg really prints: a header, indented metadata, then progress
// lines overwritten with carriage returns.
const killedOutput = "Input #0, mp3, from '/music/song.mp3':\n" +
	"  Metadata:\n    title           : La Rosa\n" +
	"  Stream #0:0: Audio: mp3 (mp3float), 44100 Hz, stereo, fltp, 320 kb/s\n" +
	"Stream mapping:\n  Stream #0:0 -> #0:0 (mp3 (mp3float) -> pcm_s16le (native))\n" +
	"Output #0, null, to 'pipe:':\n  Metadata:\n    encoder         : Lavf62.12.101\n" +
	"size=N/A time=00:00:14.60 bitrate=N/A speed=  29x elapsed=0:00:00.50    \r" +
	"size=N/A time=00:00:30.30 bitrate=N/A speed=  30x elapsed=0:00:01.00    \r"

func TestErrorDetailDropsProgressAndHeader(t *testing.T) {
	err := commandError("analyzing loudness", errors.New("signal: killed"), []byte(killedOutput))
	if got, want := err.Error(), "analyzing loudness: signal: killed"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestErrorDetailKeepsTheReason(t *testing.T) {
	output := "[in#0 @ 0x1546136c0] Error opening input: Invalid data found when processing input\n" +
		"Error opening input file /music/bad.mp3.\n" +
		"Error opening input files: Invalid data found when processing input\n"
	got := commandError("analyzing loudness", errors.New("exit status 183"), []byte(output)).Error()
	if !strings.HasPrefix(got, "analyzing loudness: exit status 183: ") ||
		!strings.HasSuffix(got, "Error opening input files: Invalid data found when processing input") {
		t.Fatalf("reason lost: %q", got)
	}
}

func TestErrorDetailKeepsOnlyTheLastLinesAndCapsLength(t *testing.T) {
	var b strings.Builder
	for i := range 50 {
		b.WriteString("[mp3float @ 0x1] error ")
		b.WriteString(strings.Repeat("x", i))
		b.WriteString("\n")
	}
	b.WriteString("Conversion failed!\n")
	detail := errorDetail([]byte(b.String()))
	if strings.Count(detail, " | ") != errorDetailLines-1 {
		t.Fatalf("kept %d lines: %q", strings.Count(detail, " | ")+1, detail)
	}
	if len([]rune(detail)) > errorDetailRunes+1 || !strings.HasSuffix(detail, "Conversion failed!") {
		t.Fatalf("detail not capped at the end: %d runes, %q", len([]rune(detail)), detail)
	}
}

// A run stopped by the user kills ffmpeg part way through. Its error used to
// carry every progress line printed until then.
func TestStoppedMeasurementErrorIsShort(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	track := filepath.Join(t.TempDir(), "long.wav")
	cmd := exec.Command("ffmpeg", "-nostdin", "-hide_banner", "-f", "lavfi", "-i",
		"sine=frequency=440:sample_rate=44100:duration=600", "-c:a", "pcm_s16le", track)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create audio: %v: %s", err, out)
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(700*time.Millisecond, cancel)

	_, err := NewLoudnessNormalizer().AnalyzeLoudness(ctx, track, LoudnessTarget{IntegratedLUFS: -12.6, TruePeak: -0.5, LRA: 11})
	if err == nil {
		t.Fatal("measurement finished before it could be stopped; lengthen the fixture")
	}
	if msg := err.Error(); strings.Contains(msg, "time=") || len(msg) > 400 {
		t.Fatalf("stopped measurement error is not short (%d chars): %q", len(msg), msg)
	}
}

// Shortening the error must not touch what a successful run reads.
func TestMeasurementStillWorksOnRealAudio(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	track := filepath.Join(t.TempDir(), "tone.wav")
	cmd := exec.Command("ffmpeg", "-nostdin", "-hide_banner", "-f", "lavfi", "-i",
		"sine=frequency=440:sample_rate=44100:duration=5", "-c:a", "pcm_s16le", track)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create audio: %v: %s", err, out)
	}
	analysis, err := NewLoudnessNormalizer().AnalyzeLoudness(context.Background(), track, LoudnessTarget{IntegratedLUFS: -12.6, TruePeak: -0.5, LRA: 11})
	if err != nil || analysis.InputIntegrated >= 0 || analysis.InputIntegrated < -70 {
		t.Fatalf("measurement broken: %+v, %v", analysis, err)
	}
	bad := filepath.Join(t.TempDir(), "bad.mp3")
	if err := os.WriteFile(bad, []byte("not audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewLoudnessNormalizer().AnalyzeLoudness(context.Background(), bad, LoudnessTarget{IntegratedLUFS: -12.6, TruePeak: -0.5, LRA: 11}); err == nil ||
		!strings.Contains(err.Error(), "Invalid data") {
		t.Fatalf("broken file should say why: %v", err)
	}
}
