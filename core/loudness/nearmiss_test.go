package loudness

import (
	"math"
	"testing"
)

// The strict window is +/-0.2, and a result outside it used to be deleted -
// leaving the song at its ORIGINAL loudness, which was usually further from the
// target than the file just thrown away. Measured on a real library: 24 tracks
// refused this way, and for 21 of them the discarded version was closer to
// target than what was kept.
func TestNearMissBandMatchesTheRestOfTheSystem(t *testing.T) {
	// The band a kept near-miss is judged against has to be the same one the
	// planner and the pages already call "near enough to leave alone". Two
	// different definitions of close enough is how a song ends up accepted by
	// one half of the system and listed as a problem by the other.
	if leaveAloneToleranceDB != 0.5 {
		t.Fatalf("leaveAloneToleranceDB = %v; the near-miss band moved with it", leaveAloneToleranceDB)
	}
}

// Real refusals from the library, with what the discarded attempt had achieved.
func TestRefusedResultsWereBetterThanWhatWasKept(t *testing.T) {
	const target = -12.6
	// Inside the band: these are the ones now kept instead of deleted.
	rescued := []struct{ before, attempt float64 }{
		{-13.13, -12.96}, {-13.27, -12.99}, {-13.12, -12.89}, {-13.34, -12.81},
		{-13.14, -12.92}, {-13.21, -12.87},
	}
	for _, c := range rescued {
		keptOff := math.Abs(c.before - target)
		attemptOff := math.Abs(c.attempt - target)
		if attemptOff > leaveAloneToleranceDB {
			t.Errorf("attempt at %.2f is outside the band, so it would still be refused", c.attempt)
			continue
		}
		if attemptOff >= keptOff {
			t.Errorf("attempt %.2f off vs kept %.2f off - keeping it would not have helped",
				attemptOff, keptOff)
		}
	}

	// Outside it by a hair, and still refused. The band is a real line, not a
	// way of accepting whatever the encoder produced: -13.11 is 0.51 out, and
	// nine of the twenty-four real refusals sat here. They stay refused, which
	// is why this fix rescues most of that group and not all of it.
	if math.Abs(-13.11-target) <= leaveAloneToleranceDB {
		t.Error("-13.11 is 0.51 from target and must stay outside the keep-band")
	}
}

// A result that misses by more than the wider band still has to be refused,
// or the band means nothing.
func TestBandStillRefusesAGenuineMiss(t *testing.T) {
	const target = -12.6
	for _, landed := range []float64{-13.5, -11.5, -14.0} {
		if math.Abs(landed-target) <= leaveAloneToleranceDB {
			t.Errorf("%.2f should be outside the keep-band", landed)
		}
	}
}
