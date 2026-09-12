package loudness

import (
	"context"
	"math"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/navidrome/navidrome/core/ffmpeg"
)

func TestOptimizeRepairsAnUnsafePeakAtTargetLoudness(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	library, backups := t.TempDir(), t.TempDir()
	track := filepath.Join(library, "peaky.wav")
	cmd := exec.Command("ffmpeg", "-nostdin", "-hide_banner", "-f", "lavfi", "-i",
		"aevalsrc=0.08*sin(2*PI*440*t)+0.95*eq(n\\,48000):s=48000:d=3", "-c:a", "pcm_s16le", track)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create peaky audio: %v: %s", err, out)
	}
	n := ffmpeg.NewLoudnessNormalizer()
	opts := OptimizeOptions{
		Target:    ffmpeg.LoudnessTarget{IntegratedLUFS: -12.6, TruePeak: -3, LRA: 11},
		Tolerance: 0.2, Backup: true, LibraryPath: library, BackupFolder: backups, MediaFileID: "track-1",
	}
	before, err := Measure(context.Background(), n, track, opts.Target)
	if err != nil {
		t.Fatal(err)
	}
	if before.TruePeak <= opts.Target.TruePeak {
		t.Fatalf("fixture needs unsafe peaks, got %g", before.TruePeak)
	}
	opts.Target.IntegratedLUFS = before.LUFS
	res, err := Optimize(context.Background(), n, track, DecisionPending, opts)
	if err != nil || !res.Changed {
		t.Fatalf("on-target audio still needs peak correction: changed=%v phase=%d rejection=%q error=%v", res.Changed, res.Phase, res.Rejected, err)
	}
	after, err := Measure(context.Background(), n, track, opts.Target)
	if err != nil {
		t.Fatal(err)
	}
	if after.TruePeak > opts.Target.TruePeak || math.Abs(after.LUFS-before.LUFS) > opts.Tolerance {
		t.Fatalf("result misses target: LUFS %g -> %g, peak %g (ceiling %g)", before.LUFS, after.LUFS, after.TruePeak, opts.Target.TruePeak)
	}
	if after.Probe.Duration != before.Probe.Duration {
		t.Fatalf("duration changed: %g -> %g", before.Probe.Duration, after.Probe.Duration)
	}
}

// Inject measured encoding outcomes while still exercising real file writes,
// retries, acceptance, and cleanup through Optimize.
type peakSequenceNormalizer struct {
	before ffmpeg.LoudnessAnalysis
	after  []ffmpeg.LoudnessAnalysis
	calls  int
}

func TestInvalidLoudnessMeasurementCannotReplaceAudio(t *testing.T) {
	for _, lufs := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		library := t.TempDir()
		track := filepath.Join(library, "song.mp3")
		writeQuietTestMP3(t, track, 2)
		original := digest(t, track)
		n := &peakSequenceNormalizer{
			before: ffmpeg.LoudnessAnalysis{InputIntegrated: -18, InputTruePeak: -8},
			after:  []ffmpeg.LoudnessAnalysis{{InputIntegrated: lufs, InputTruePeak: -1}},
		}
		res, err := Optimize(context.Background(), n, track, DecisionPending, OptimizeOptions{
			Target: ffmpeg.LoudnessTarget{IntegratedLUFS: -12.6, TruePeak: -0.5, LRA: 11}, Tolerance: 0.2,
		})
		if err == nil || res.Changed || digest(t, track) != original {
			t.Fatalf("invalid LUFS %v was accepted: %+v, %v", lufs, res, err)
		}
	}
}

func (n *peakSequenceNormalizer) AnalyzeLoudness(context.Context, string, ffmpeg.LoudnessTarget) (*ffmpeg.LoudnessAnalysis, error) {
	i := n.calls
	n.calls++
	if i == 0 {
		return &n.before, nil
	}
	if i > len(n.after) {
		i = len(n.after)
	}
	return &n.after[i-1], nil
}

func TestOptimizeEnforcesPeakCeilingBeforeReplacingAudio(t *testing.T) {
	for _, tc := range []struct {
		name         string
		beforeLUFS   float64
		beforePeak   float64
		afterLUFS    float64
		peaks        []float64
		wantChanged  bool
		wantAttempts int
	}{
		{"gain rejects even 0.01 above ceiling", -18, -8, -12.6, []float64{-0.49}, false, 1},
		{"limiter never falls back", -12.6, 1, -12.6, []float64{-0.4, -0.2, -0.1}, false, 3},
		{"unsafe near miss is not kept", -12.6, 1, -12.9, []float64{-0.1}, false, 3},
		{"limiter retries until peak is safe", -12.6, 1, -12.6, []float64{-0.4, -0.6}, true, 2},
		{"gain accepts exactly the ceiling", -18, -8, -12.6, []float64{-0.5}, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			library, backups := t.TempDir(), t.TempDir()
			track := filepath.Join(library, "song.mp3")
			writeQuietTestMP3(t, track, 2)
			original := digest(t, track)
			n := &peakSequenceNormalizer{before: ffmpeg.LoudnessAnalysis{InputIntegrated: tc.beforeLUFS, InputTruePeak: tc.beforePeak}}
			for _, peak := range tc.peaks {
				n.after = append(n.after, ffmpeg.LoudnessAnalysis{InputIntegrated: tc.afterLUFS, InputTruePeak: peak})
			}
			opts := OptimizeOptions{
				Target:    ffmpeg.LoudnessTarget{IntegratedLUFS: -12.6, TruePeak: -0.5, LRA: 11},
				Tolerance: 0.2, Backup: true, LibraryPath: library, BackupFolder: backups, MediaFileID: "track-1",
			}
			res, err := Optimize(context.Background(), n, track, DecisionPending, opts)
			if err != nil {
				t.Fatal(err)
			}
			if res.Changed != tc.wantChanged || res.Attempts != tc.wantAttempts {
				t.Fatalf("changed=%v, attempts=%d, rejection=%q; want changed=%v, attempts=%d", res.Changed, res.Attempts, res.Rejected, tc.wantChanged, tc.wantAttempts)
			}
			if tc.wantChanged {
				if res.AfterSet == nil || res.AfterSet.TruePeak > opts.Target.TruePeak {
					t.Fatal("accepted an unsafe result")
				}
				if res.Rejected != "" || digest(t, track) == original {
					t.Fatal("accepted result did not replace the audio cleanly")
				}
			} else if res.Rejected == "" || digest(t, track) != original {
				t.Fatal("unsafe result must be rejected and leave original bytes unchanged")
			}
			if files, _ := filepath.Glob(filepath.Join(library, ".*.lufs-*")); len(files) != 0 {
				t.Fatalf("temporary outputs left behind: %v", files)
			}
		})
	}
}
