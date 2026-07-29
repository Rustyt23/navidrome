package ffmpeg

import "testing"

// astats prints "RMS peak dB" and "RMS trough dB" alongside "RMS level dB", and
// a per-channel block before the overall one. Picking the wrong line, or the
// wrong occurrence, yields a plausible number that means something else.
func TestAstatsRMSMatchesTheOverallLevelOnly(t *testing.T) {
	output := `
[Parsed_astats_2 @ 0x600] Channel: 1
[Parsed_astats_2 @ 0x600] Peak level dB: -14.224582
[Parsed_astats_2 @ 0x600] RMS level dB: -47.163977
[Parsed_astats_2 @ 0x600] RMS peak dB: -21.884215
[Parsed_astats_2 @ 0x600] RMS trough dB: -91.201103
[Parsed_astats_2 @ 0x600] Overall
[Parsed_astats_2 @ 0x600] Peak level dB: -14.224582
[Parsed_astats_2 @ 0x600] RMS level dB: -47.163977
[Parsed_astats_2 @ 0x600] RMS peak dB: -21.884215
[Parsed_astats_2 @ 0x600] RMS trough dB: -91.201103
`
	matches := astatsRMSRe.FindAllStringSubmatch(output, -1)
	if len(matches) != 2 {
		t.Fatalf("matched %d lines, want the 2 'RMS level dB' lines", len(matches))
	}
	if got := matches[len(matches)-1][1]; got != "-47.163977" {
		t.Errorf("overall RMS = %q, want -47.163977", got)
	}
}

func TestAstatsRMSHandlesSilence(t *testing.T) {
	// Two identical files cancel exactly, and ffmpeg reports -inf.
	matches := astatsRMSRe.FindAllStringSubmatch("RMS level dB: -inf", -1)
	if len(matches) != 1 || matches[0][1] != "-inf" {
		t.Fatalf("matches = %v, want -inf captured", matches)
	}
}

// The limiter shapes the decoded waveform, but the true peak of the file that
// ships is set after re-encoding, and encoding pushes it back up by an amount
// that depends on the bitrate. These are the measured spring-backs.
func TestLimiterHeadroomCoversTheCodecSpringBack(t *testing.T) {
	measured := []struct {
		bitRate    int
		springBack float64
	}{
		{96, 0.73}, {128, 0.77}, {160, 0.72},
		{192, 0.39}, {256, 0.44}, {320, 0.25},
	}
	for _, m := range measured {
		if got := limiterHeadroom(m.bitRate); got < m.springBack {
			t.Errorf("%dk: aiming %.2f below the ceiling, but re-encoding pushes the peak back up %.2f - the file ships over the ceiling",
				m.bitRate, got, m.springBack)
		}
	}
}

func TestLimiterHeadroomDoesNotOvershaveGoodSources(t *testing.T) {
	// A degraded file needs the room; a clean one must not be cut harder than
	// its own spring-back requires, because every extra dB of limiting is
	// audio removed.
	if limiterHeadroom(128) <= limiterHeadroom(320) {
		t.Error("a 128k source needs more room than a 320k one")
	}
	if got := limiterHeadroom(320); got > 0.3 {
		t.Errorf("320k headroom = %.2f, want no more than its 0.25 spring-back needs", got)
	}
	// An unreadable bitrate must not be treated as the worst case.
	if limiterHeadroom(0) != limiterHeadroom(320) {
		t.Error("unknown bitrate should assume the source is not degraded")
	}
}
