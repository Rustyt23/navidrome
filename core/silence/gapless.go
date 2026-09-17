package silence

import (
	"sort"

	"github.com/navidrome/navidrome/model"
)

const (
	// TightSeamSeconds is how little combined silence between two consecutive
	// tracks counts as them running into each other.
	//
	// A seam is the tail of one track plus the head of the next - the whole gap
	// a listener hears between them. Under a second means the split was made
	// inside a continuous performance (a live set, a DJ mix, one movement into
	// the next) rather than between two separately recorded songs.
	TightSeamSeconds = 1.0

	// TightSeamFraction is how much of an album must be tight before the album
	// as a whole is treated as continuous. Well over half, because one or two
	// tight seams happen on ordinary albums - two songs deliberately segued -
	// and that is not a reason to hold back the rest of the record.
	TightSeamFraction = 0.6

	// MinSeamsForGapless is the fewest seams an album must have before this
	// judgement means anything. On a two-track album a single tight seam is
	// 100% of the evidence, which is not evidence.
	MinSeamsForGapless = 2
)

// TrackSeam is one track's position in its album and what its ends measure.
type TrackSeam struct {
	MediaFileID string
	DiscNumber  int
	TrackNumber int
	LeadSilence float64
	TailSilence float64
}

// DetectGaplessAlbum reports whether an album's tracks run into one another.
//
// Judged across the album rather than per track because that is where the
// answer lives: a single track with no silence at its ends is simply a track
// that was topped and tailed properly, while a whole record of them is a
// continuous recording that was split up. Trimming the latter closes gaps the
// artist put there, and the damage is only audible on playback of the album -
// which is to say, after it is too late.
//
// Tracks must be for one album; the caller groups them.
func DetectGaplessAlbum(tracks []TrackSeam) bool {
	if len(tracks) < MinSeamsForGapless+1 {
		return false
	}
	ordered := make([]TrackSeam, len(tracks))
	copy(ordered, tracks)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].DiscNumber != ordered[j].DiscNumber {
			return ordered[i].DiscNumber < ordered[j].DiscNumber
		}
		return ordered[i].TrackNumber < ordered[j].TrackNumber
	})

	seams, tight := 0, 0
	for i := 0; i < len(ordered)-1; i++ {
		// Only seams within a disc count. The gap between the last track of
		// disc one and the first of disc two is not a seam anybody hears.
		if ordered[i].DiscNumber != ordered[i+1].DiscNumber {
			continue
		}
		seams++
		if ordered[i].TailSilence+ordered[i+1].LeadSilence < TightSeamSeconds {
			tight++
		}
	}
	if seams < MinSeamsForGapless {
		return false
	}
	return float64(tight)/float64(seams) >= TightSeamFraction
}

// ApplyGaplessVerdict re-decides an album's audits once continuity is known.
//
// Analysis runs one track at a time and cannot see the album, so it plans every
// track as though it stood alone. This is the second look: on a continuous
// album every planned trim is withdrawn and recorded as skipped-for-gapless,
// which leaves the reason visible on the page instead of the tracks simply
// never appearing in the trimmable list.
func ApplyGaplessVerdict(audits []*model.SilenceAudit, gapless bool) {
	for _, audit := range audits {
		if audit == nil {
			continue
		}
		audit.Gapless = gapless
		if !gapless {
			continue
		}
		if audit.Verdict == model.SilenceVerdictTrimmable {
			audit.Verdict = model.SilenceVerdictSkipped
			audit.SkipReason = model.SilenceSkipGapless
			audit.LeadTrim = 0
			audit.TrailTrim = 0
		}
	}
}
