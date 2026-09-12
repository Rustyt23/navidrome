package loudness

import (
	"math"
	"testing"

	"github.com/navidrome/navidrome/model"
)

// Each signal has to be read for what it measures. The null test says how much
// of the file changed; only the loudness range and the true peak say whether
// the music itself was reshaped. Reading the first as if it were the second
// reported 28 untouched tracks as damaged.
func TestVerdict(t *testing.T) {
	// Taken from a real pair: a +0.40 dB lift, dynamics and peak intact, and a
	// leftover typical of rewriting a 320 kbps mp3.
	gainOnly := func() (*model.LoudnessAudit, *Measurement, *Measurement) {
		before := testMeasurement(-13.02, -1.35, 4.7)
		after := testMeasurement(-12.62, -0.95, 4.8)
		return &model.LoudnessAudit{
			Action:       model.LoudnessActionGain,
			NullResidual: f64(-47.2),
		}, before, after
	}

	t.Run("a pure gain is safe however much rewriting cost", func(t *testing.T) {
		audit, before, after := gainOnly()
		if got := verdict(audit, before, after); got != model.LoudnessVerdictSafe {
			t.Errorf("verdict = %q, want %q", got, model.LoudnessVerdictSafe)
		}
	})

	t.Run("an expensive rewrite is reported as one, not as reshaped audio", func(t *testing.T) {
		// The peak tracked the gain exactly and the range did not close, so
		// nothing limited this track - whatever the leftover cost. Calling it
		// "peaks trimmed" accuses the process of something the two signals that
		// can actually see the dynamics both say did not happen.
		//
		// It must not become "quality lost" either: the file came back in
		// exactly the format it went in.
		audit, before, after := gainOnly()
		audit.NullResidual = f64(-14.2)
		audit.Action = inferAction(audit, before, after, 0.40)
		if audit.Action != model.LoudnessActionGain {
			t.Errorf("action = %q, want %q - nothing touched the peak", audit.Action, model.LoudnessActionGain)
		}
		if got := verdict(audit, before, after); got != model.LoudnessVerdictRewriteCostly {
			t.Errorf("verdict = %q, want %q", got, model.LoudnessVerdictRewriteCostly)
		}
	})

	// The null test cannot convict on its own, but it can corroborate. A peak
	// movement too small to trust by itself is trustworthy once something
	// independent agrees the audio changed.
	t.Run("a small peak shave counts once the null test agrees", func(t *testing.T) {
		audit, before, _ := gainOnly()
		// Gain 0.40 on a -1.35 peak predicts -0.95; it came back at -1.25, so
		// something took 0.30 off - under the 0.5 standalone threshold.
		after := testMeasurement(-12.62, -1.25, 4.8)
		audit.NullResidual = f64(-12.0)
		audit.Action = inferAction(audit, before, after, 0.40)
		if audit.Action != model.LoudnessActionLimited {
			t.Errorf("action = %q, want %q", audit.Action, model.LoudnessActionLimited)
		}
		if got := verdict(audit, before, after); got != model.LoudnessVerdictDynamicsChanged {
			t.Errorf("verdict = %q, want %q", got, model.LoudnessVerdictDynamicsChanged)
		}

		// The same shave with a clean null test stays a gain: on its own, 0.30
		// is not enough to convict.
		quiet, before2, _ := gainOnly()
		quiet.NullResidual = f64(-41.0)
		quiet.Action = inferAction(quiet, before2, after, 0.40)
		if quiet.Action != model.LoudnessActionGain {
			t.Errorf("action = %q, want %q without corroboration", quiet.Action, model.LoudnessActionGain)
		}
	})

	t.Run("a moved loudness range is a change to the dynamics", func(t *testing.T) {
		audit, before, _ := gainOnly()
		// Compressed from 4.7 to 3.0: the gap between the loud and quiet
		// passages closed, which a level change cannot do.
		after := testMeasurement(-12.62, -0.95, 3.0)
		if got := verdict(audit, before, after); got != model.LoudnessVerdictDynamicsChanged {
			t.Errorf("verdict = %q, want %q", got, model.LoudnessVerdictDynamicsChanged)
		}
	})

	t.Run("a peak that did not follow the gain is a change to the dynamics", func(t *testing.T) {
		audit, before, after := gainOnly()
		audit.Action = model.LoudnessActionLimited
		if got := verdict(audit, before, after); got != model.LoudnessVerdictDynamicsChanged {
			t.Errorf("verdict = %q, want %q", got, model.LoudnessVerdictDynamicsChanged)
		}
	})

	t.Run("losing quality outranks everything else", func(t *testing.T) {
		audit, before, after := gainOnly()
		after.Probe.BitRate = 128 // was 320
		if got := verdict(audit, before, after); got != model.LoudnessVerdictReencoded {
			t.Errorf("verdict = %q, want %q", got, model.LoudnessVerdictReencoded)
		}
	})

	t.Run("a lossless round trip is held to a tighter floor", func(t *testing.T) {
		// -45 is an ordinary result for an mp3 and an alarming one for flac,
		// which has no codec noise to account for. The peak still tracked its
		// gain, so what is alarming is the cost of the rewrite, not the shape
		// of the audio.
		audit, before, after := gainOnly()
		audit.NullResidual = f64(-45.0)
		for _, m := range []*Measurement{before, after} {
			m.Probe.Codec = "flac"
		}
		audit.Action = inferAction(audit, before, after, 0.40)
		if got := verdict(audit, before, after); got != model.LoudnessVerdictRewriteCostly {
			t.Errorf("verdict = %q, want %q", got, model.LoudnessVerdictRewriteCostly)
		}

		// The same leftover on an mp3 is unremarkable.
		mp3, mb, ma := gainOnly()
		mp3.NullResidual = f64(-45.0)
		mp3.Action = inferAction(mp3, mb, ma, 0.40)
		if got := verdict(mp3, mb, ma); got != model.LoudnessVerdictSafe {
			t.Errorf("mp3 verdict = %q, want %q", got, model.LoudnessVerdictSafe)
		}
	})
}

func TestNullResidualThresholdSeparatesRewritingFromReshaping(t *testing.T) {
	// Measured on one track, same encoder settings throughout: a gain-only
	// rewrite against the same track put through a limiter and a compressor.
	const (
		worstGainOnlyRewrite = -39.0
		mildestReshaping     = -14.5
	)
	// Every lossy floor has to clear both, at every bitrate. A floor that fails
	// ordinary rewrites cries wolf; one that passes reshaped audio is not
	// checking anything.
	for _, bitRate := range []int{0, 128, 160, 192, 256, 320} {
		floor := NullResidualThreshold("mp3", bitRate)
		if floor <= worstGainOnlyRewrite {
			t.Errorf("%dk: floor %.1f would flag an ordinary rewrite at %.1f",
				bitRate, floor, worstGainOnlyRewrite)
		}
		if floor >= mildestReshaping {
			t.Errorf("%dk: floor %.1f would pass reshaped audio at %.1f",
				bitRate, floor, mildestReshaping)
		}
	}

	// The floor must never tighten as the format gets noisier, or the client's
	// worst material is held to the strictest standard - which is where the
	// false alarms were landing.
	noisy := NullResidualThreshold("mp3", 128)
	middle := NullResidualThreshold("mp3", 224)
	clean := NullResidualThreshold("mp3", 320)
	if !(noisy > middle && middle > clean) {
		t.Errorf("floors must loosen as the bitrate drops: 128k=%.1f 224k=%.1f 320k=%.1f",
			noisy, middle, clean)
	}
	if lossless := NullResidualThreshold("flac", 0); lossless >= clean {
		t.Errorf("lossless floor %.1f must be tighter than any lossy one", lossless)
	}
}

// A target loudness never licenses exceeding the client's peak ceiling.
func TestConfiguredPeakCeilingIsAHardBound(t *testing.T) {
	for _, ceiling := range []float64{-3, -1.5, -0.5} {
		for _, peak := range []float64{ceiling - 1, ceiling} {
			if !peakWithinCeiling(peak, ceiling) {
				t.Errorf("peak %g should satisfy ceiling %g", peak, ceiling)
			}
		}
		for _, peak := range []float64{ceiling + 0.01, ceiling + 0.1, -0.1, 0, 1, math.NaN(), math.Inf(1), math.Inf(-1)} {
			if peakWithinCeiling(peak, ceiling) {
				t.Errorf("accepted peak %g above/invalid for ceiling %g", peak, ceiling)
			}
		}
	}
}

// The three tracks this was written for, with their real numbers off the LUFS
// page. All three were labelled wrongly by a rule that read the peak drift
// without its sign, and each sat on a different side of the 0.5 dB line.
func TestInferActionOnTheTracksThatWereLabelledWrongly(t *testing.T) {
	cases := []struct {
		name                    string
		tpBefore, tpAfter, gain float64
		lraBefore, lraAfter     float64
		nullResidual            float64
		want                    string
		why                     string
	}{
		{
			name: "Kryptonite", tpBefore: -3.05, tpAfter: -1.36, gain: 0.39,
			lraBefore: 3.7, lraAfter: 3.6, nullResidual: -39.0,
			want: model.LoudnessActionGain,
			why:  "the peak rose 1.30 above the gain, which limiting cannot do - the codec put it there",
		},
		{
			name: "Sleigh Ride", tpBefore: -1.92, tpAfter: -0.76, gain: 0.56,
			lraBefore: 7.9, lraAfter: 7.9, nullResidual: -41.6,
			want: model.LoudnessActionGain,
			why:  "same again: the peak came back higher than the level change, and the null test is at the gain-only floor",
		},
		{
			name: "Window to the Soul", tpBefore: -1.28, tpAfter: -0.63, gain: 1.09,
			lraBefore: 5.6, lraAfter: 5.5, nullResidual: -10.9,
			want: model.LoudnessActionLimited,
			why:  "shaved 0.44 dB - under the old 0.5 threshold, so it was called untouched while the null test said otherwise",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := testMeasurement(-13.0, c.tpBefore, c.lraBefore)
			after := testMeasurement(-12.6, c.tpAfter, c.lraAfter)
			audit := &model.LoudnessAudit{NullResidual: f64(c.nullResidual)}

			audit.Action = inferAction(audit, before, after, c.gain)
			if audit.Action != c.want {
				t.Errorf("action = %q, want %q: %s", audit.Action, c.want, c.why)
			}

			wantVerdict := model.LoudnessVerdictSafe
			if c.want == model.LoudnessActionLimited {
				wantVerdict = model.LoudnessVerdictDynamicsChanged
			}
			if got := verdict(audit, before, after); got != wantVerdict {
				t.Errorf("verdict = %q, want %q", got, wantVerdict)
			}
		})
	}
}

// Without a null test the peak is all there is, and it only means reshaping in
// one direction.
func TestInferActionReadsThePeakInOneDirectionOnly(t *testing.T) {
	before := testMeasurement(-13.0, -3.05, 3.7)

	pushedDown := testMeasurement(-12.6, -3.40, 3.7) // 0.74 below a +0.39 gain
	audit := &model.LoudnessAudit{}
	if got := inferAction(audit, before, pushedDown, 0.39); got != model.LoudnessActionLimited {
		t.Errorf("peak pushed down: action = %q, want %q", got, model.LoudnessActionLimited)
	}

	sprangUp := testMeasurement(-12.6, -1.36, 3.7) // 1.30 above the same gain
	if got := inferAction(audit, before, sprangUp, 0.39); got != model.LoudnessActionGain {
		t.Errorf("peak sprang up: action = %q, want %q - nothing in the chain raises peaks", got, model.LoudnessActionGain)
	}
}

// Reaching -12.6 means turning most masters down, and a quieter lossless file
// compresses smaller. Read as damage that rejected the output and marked the
// track refused - which, on a lossless library, is every file in it.
func TestLosslessCompressingSmallerIsNotDamage(t *testing.T) {
	before, after := codecPair("flac", 620, 592)

	if issues := IntegrityIssues(before, after); len(issues) > 0 {
		t.Errorf("a FLAC that compressed better is intact, got %v", issues)
	}
}

func TestLossyLosingBitrateIsStillDamage(t *testing.T) {
	before, after := codecPair("mp3", 320, 128)

	if issues := IntegrityIssues(before, after); len(issues) == 0 {
		t.Error("an mp3 re-encoded at a lower bitrate has lost quality")
	}
}

// A source can probe above what the format can hold when the stream reports no
// bitrate and the container's figure - cover art and tags included - stands in
// for it. The encoder is clamped to the ceiling, so the output measures lower
// through no fault of its own.
func TestLossyAtTheFormatCeilingIsNotDamage(t *testing.T) {
	before, after := codecPair("mp3", 340, 320)

	if issues := IntegrityIssues(before, after); len(issues) > 0 {
		t.Errorf("an mp3 shipped at the 320k ceiling is intact, got %v", issues)
	}
}

// codecPair builds two otherwise-identical measurements that differ only in
// codec and bitrate, so an integrity check sees nothing else to complain about.
func codecPair(codec string, beforeKbps, afterKbps int) (before, after *Measurement) {
	before = testMeasurement(-18.0, -3.0, 7.0)
	after = testMeasurement(-12.6, -0.5, 7.0)
	before.Probe.Codec, after.Probe.Codec = codec, codec
	before.Probe.BitRate, after.Probe.BitRate = beforeKbps, afterKbps
	return before, after
}
