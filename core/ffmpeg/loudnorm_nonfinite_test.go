package ffmpeg

import (
	"math"
	"strings"
	"testing"
)

// ffmpeg prints "-inf" for a digitally silent file, and Go's ParseFloat accepts
// it happily. Left through it becomes an infinite distance to the target, and
// from there +Inf gain, NaN comparisons that are false against everything, and
// eventually a literal volume=+InfdB handed to ffmpeg.
func TestParseLoudnormFloatRejectsNonFinite(t *testing.T) {
	for _, value := range []string{"-inf", "inf", "-Inf", "+Inf", "nan", "NaN"} {
		got, err := parseLoudnormFloat(value, "input_i")
		if err == nil {
			t.Errorf("%q was accepted as %v; it is not a usable level", value, got)
			continue
		}
		if !strings.Contains(err.Error(), "not a usable level") {
			t.Errorf("%q: error does not explain itself: %v", value, err)
		}
	}
}

func TestParseLoudnormFloatStillAcceptsRealMeasurements(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want float64
	}{
		{"-12.60", -12.60}, {"0", 0}, {"-70.0", -70.0}, {"1.5", 1.5},
	} {
		got, err := parseLoudnormFloat(tc.in, "input_i")
		if err != nil {
			t.Errorf("%q was rejected: %v", tc.in, err)
			continue
		}
		if math.Abs(got-tc.want) > 0.001 {
			t.Errorf("%q parsed as %v, wanted %v", tc.in, got, tc.want)
		}
	}
	if _, err := parseLoudnormFloat("N/A", "input_i"); err == nil {
		t.Error("N/A should still be rejected")
	}
}
