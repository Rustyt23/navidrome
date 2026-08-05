package ffmpeg

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
)

// PeakMeasurer reports the loudest sample in a stretch of a file.
type PeakMeasurer interface {
	MeasurePeakDB(ctx context.Context, path string, region PeakRegion) (float64, error)
}

func NewPeakMeasurer() PeakMeasurer { return &ffmpeg{} }

// PeakRegion names a stretch at one end of a file.
type PeakRegion struct {
	// Seconds is how much audio to measure.
	Seconds float64
	// FromEnd measures the last Seconds of the file rather than the first.
	FromEnd bool
}

// SilentPeakDB is the level at or below which a stretch of audio is treated as
// nothing.
//
// It must stay equal to PrimaryThresholdDB. The detector places the cut using
// that threshold, and this check then confirms the result with an independent
// tool - so if the two numbers disagree, the check is either rejecting
// everything the detector found or waving through audio the detector never
// claimed was silent. Two measurements agreeing is a fact; one measurement is
// an assumption, and two measurements answering different questions is worse
// than either.
const SilentPeakDB = -50.0

// maxVolumeRe reads volumedetect's report. It prints to the log like
// silencedetect, so parsing the output is the only way to read it.
var maxVolumeRe = regexp.MustCompile(`max_volume:\s*(-?[\d.]+) dB`)

// MeasurePeakDB returns the loudest sample, in dBFS, within a region.
//
// A completely digital-silent region has no measurable peak at all, which
// ffmpeg reports as -91 dB (the floor of 16-bit) or omits entirely. Either is
// reported as -infinity rather than as an error: "nothing there" is the answer,
// not a failure.
func (e *ffmpeg) MeasurePeakDB(ctx context.Context, path string, region PeakRegion) (float64, error) {
	cmdPath, err := ffmpegCmd()
	if err != nil {
		return 0, err
	}
	if err := fileExists(path); err != nil {
		return 0, err
	}
	if region.Seconds <= 0 {
		return negativeInfinityDB, nil
	}

	seconds := strconv.FormatFloat(region.Seconds, 'f', 6, 64)
	args := []string{"-nostdin", "-hide_banner", "-nostats"}
	if region.FromEnd {
		args = append(args, "-sseof", "-"+seconds)
	} else {
		args = append(args, "-t", seconds)
	}
	args = append(args, "-i", path, "-map", "0:a:0", "-vn", "-af", "volumedetect", "-f", "null", "-")

	output, err := runCommand(ctx, DecodeTimeout(region.Seconds), cmdPath, args...)
	if err != nil {
		return 0, fmt.Errorf("measuring the level of %q: %w: %s", path, err, string(output))
	}
	match := maxVolumeRe.FindSubmatch(output)
	if match == nil {
		// No peak reported means nothing was above the floor.
		return negativeInfinityDB, nil
	}
	peak, err := strconv.ParseFloat(string(match[1]), 64)
	if err != nil {
		return 0, fmt.Errorf("measuring the level of %q: unreadable peak %q", path, match[1])
	}
	return peak, nil
}

// negativeInfinityDB stands in for digital silence. Low enough that every
// comparison against a real threshold treats it as nothing, without needing
// callers to handle an actual infinity.
const negativeInfinityDB = -200.0
