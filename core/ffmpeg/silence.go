package ffmpeg

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
)

// SilenceDetector finds the silence at the head and tail of a track.
//
// Deliberately its own interface rather than a method on LoudnessNormalizer:
// the two features share ffmpeg and nothing else, and a track can be measured
// for silence without loudness ever being involved.
type SilenceDetector interface {
	DetectSilence(ctx context.Context, path string, opts SilenceDetectOptions) (*SilenceReport, error)
}

func NewSilenceDetector() SilenceDetector { return &ffmpeg{} }

const (
	// PrimaryThresholdDB is what counts as silence, and therefore where the cut
	// lands: the boundary is where the audio falls below this, pulled back by
	// the margin. Everything removed is quieter than this by construction, so
	// this number alone decides how loud the removed audio can possibly be.
	//
	// -50 dB is about three thousandths of full scale. Played at any normal
	// level it sits at roughly 35 dB SPL - under the noise floor of a quiet room
	// and far under a shop's. Measured on real tracks, the audio actually
	// removed at this threshold peaked between -55 and -64 dB, so the figure
	// here is a ceiling that real material stays well below.
	//
	// It was -60 dB, which is inaudible with more room to spare but found very
	// little: 3 of 70 sampled tracks, against 17 at this threshold. Going
	// further the other way is what does damage - at -40 dB the removed audio
	// measured -43 dB on real material, which is the song's own decay rather
	// than dead air, and truncating a decay is heard as a chopped ending even
	// though it is quiet.
	PrimaryThresholdDB = -50.0

	// OnsetThresholdDB is the second, louder threshold used only to judge how
	// abruptly the audio arrives. It is never used to decide where to cut.
	//
	// Held 15 dB above the primary threshold, which is the spacing that makes
	// the distance between the two boundaries mean something: at a sharp start
	// they land ~1 ms apart, on a fade hundreds of ms apart. Moving the primary
	// threshold without moving this one would squeeze the two together until
	// every track looked sharp and the measurement said nothing.
	OnsetThresholdDB = -35.0

	// MinSilenceDuration is how long a quiet stretch must last to be reported.
	// Shorter than this is a pause in the music, not dead air at the edge.
	MinSilenceDuration = 0.3

	// edgeWindowSeconds is how much of each end is decoded when looking for
	// silence. Head and tail silence is by definition at the edges, so decoding
	// the middle finds nothing and costs the most.
	//
	// A minute is far past any real lead-in or run-out while still cutting the
	// work on a typical track to a fraction of a full decode - the reason a
	// silence sweep finishes in a fraction of the time a loudness sweep takes.
	edgeWindowSeconds = 60.0
)

// SilenceDetectOptions carries the thresholds. Defaulted rather than required,
// so a caller that has no opinion gets the measured-good values.
type SilenceDetectOptions struct {
	ThresholdDB      float64
	OnsetThresholdDB float64
	MinDuration      float64
	// Duration of the track, used to place the tail window and to convert the
	// tail-window timestamps back to absolute positions. Zero means unknown, in
	// which case the whole file is decoded.
	Duration float64
}

func (o SilenceDetectOptions) withDefaults() SilenceDetectOptions {
	if o.ThresholdDB == 0 {
		o.ThresholdDB = PrimaryThresholdDB
	}
	if o.OnsetThresholdDB == 0 {
		o.OnsetThresholdDB = OnsetThresholdDB
	}
	if o.MinDuration <= 0 {
		o.MinDuration = MinSilenceDuration
	}
	return o
}

// SilenceReport is what the detector found at each end of one track.
type SilenceReport struct {
	// LeadSilence is how many seconds of silence the track opens with, measured
	// at the primary threshold. Zero when it opens straight into audio.
	LeadSilence float64
	// TrailSilence is how many seconds of silence the track ends with.
	TrailSilence float64

	// LeadOnsetGap is how much later the louder threshold places the start of
	// the music than the primary one does. Near zero for a sharp start; large
	// for a fade-in. See OnsetThresholdDB.
	LeadOnsetGap float64
	// TrailOnsetGap is the same measure at the end, for a fade-out.
	TrailOnsetGap float64

	// Duration is the track length the detector worked against.
	Duration float64
}

// silenceLineRe matches the silencedetect log lines. The filter reports through
// the log rather than through any structured output, so this is the only way to
// read it. Instance index is captured so the two thresholds can be told apart:
// they run in one pass and interleave their output.
var silenceLineRe = regexp.MustCompile(
	`\[Parsed_silencedetect_(\d+) @ [^\]]+\] silence_(start|end): (-?[\d.]+)`)

// DetectSilence measures the silence at both ends of a track.
//
// Two silencedetect instances run in a single decode: one at the cutting
// threshold, one louder, purely to judge how sharply the music begins. Running
// them together rather than in two passes costs nothing - the decode dominates -
// and guarantees both describe exactly the same audio.
func (e *ffmpeg) DetectSilence(ctx context.Context, path string, opts SilenceDetectOptions) (*SilenceReport, error) {
	cmdPath, err := ffmpegCmd()
	if err != nil {
		return nil, err
	}
	if err := fileExists(path); err != nil {
		return nil, err
	}
	opts = opts.withDefaults()

	report := &SilenceReport{Duration: opts.Duration}

	// The head and tail are decoded separately so that a long track costs two
	// short decodes instead of one full-length one. A track shorter than two
	// windows is done in a single pass, which is both cheaper and avoids the
	// two windows overlapping and reporting the same silence twice.
	if opts.Duration <= 0 || opts.Duration <= 2*edgeWindowSeconds {
		lead, trail, err := e.scanSilenceWindow(ctx, cmdPath, path, opts, 0, 0)
		if err != nil {
			return nil, err
		}
		report.LeadSilence, report.LeadOnsetGap = lead.silence, lead.onsetGap
		report.TrailSilence, report.TrailOnsetGap = trail.silence, trail.onsetGap
		return report, nil
	}

	head, _, err := e.scanSilenceWindow(ctx, cmdPath, path, opts, 0, edgeWindowSeconds)
	if err != nil {
		return nil, err
	}
	report.LeadSilence, report.LeadOnsetGap = head.silence, head.onsetGap

	_, tail, err := e.scanSilenceWindow(ctx, cmdPath, path, opts, -edgeWindowSeconds, 0)
	if err != nil {
		return nil, err
	}
	report.TrailSilence, report.TrailOnsetGap = tail.silence, tail.onsetGap

	return report, nil
}

// edgeSilence is one end's findings: how much silence, and how gradually the
// music arrives at (or departs from) it.
type edgeSilence struct {
	silence  float64
	onsetGap float64
}

// scanSilenceWindow decodes one window and reads both thresholds out of it.
//
// startOffset is where to begin: 0 for the head, negative to seek that many
// seconds before the end. limit bounds how much is decoded, 0 for no bound.
func (e *ffmpeg) scanSilenceWindow(ctx context.Context, cmdPath, path string,
	opts SilenceDetectOptions, startOffset, limit float64) (lead, trail edgeSilence, err error) {

	args := []string{"-nostdin", "-hide_banner", "-nostats"}
	if startOffset < 0 {
		args = append(args, "-sseof", strconv.FormatFloat(startOffset, 'f', -1, 64))
	}
	args = append(args, "-i", path)
	if limit > 0 {
		args = append(args, "-t", strconv.FormatFloat(limit, 'f', -1, 64))
	}
	filter := fmt.Sprintf("silencedetect=n=%sdB:d=%s,silencedetect=n=%sdB:d=%s",
		formatFloat(opts.ThresholdDB), formatFloat(opts.MinDuration),
		formatFloat(opts.OnsetThresholdDB), formatFloat(opts.MinDuration))
	args = append(args, "-map", "0:a:0", "-vn", "-af", filter, "-f", "null", "-")

	timeout := DecodeTimeout(opts.Duration)
	if limit > 0 {
		timeout = DecodeTimeout(limit)
	}
	output, err := runCommand(ctx, timeout, cmdPath, args...)
	if err != nil {
		return lead, trail, fmt.Errorf("detecting silence in %q: %w: %s", path, err, string(output))
	}

	primary, onset := parseSilenceIntervals(string(output))
	// The window's own length, which for a tail window is not the track's.
	windowLen := windowLength(output, opts, limit)

	lead = edgeAt(primary, onset, 0, windowLen, true)
	trail = edgeAt(primary, onset, 0, windowLen, false)
	return lead, trail, nil
}

// silenceInterval is one reported stretch of silence, in window-relative
// seconds. End is negative when the file ended while still silent and ffmpeg
// never printed a closing line.
type silenceInterval struct {
	start float64
	end   float64
}

// parseSilenceIntervals splits the log into the two detectors' findings.
// Instance 0 is the primary threshold, instance 1 the louder onset one, in the
// order they appear in the filter string.
func parseSilenceIntervals(output string) (primary, onset []silenceInterval) {
	open := map[string]float64{}
	seen := map[string]bool{}
	for _, match := range silenceLineRe.FindAllStringSubmatch(output, -1) {
		instance, kind := match[1], match[2]
		value, err := strconv.ParseFloat(match[3], 64)
		if err != nil {
			continue
		}
		target := &primary
		if instance != "0" {
			target = &onset
		}
		if kind == "start" {
			open[instance] = value
			seen[instance] = true
			continue
		}
		start, ok := open[instance]
		if !ok {
			continue
		}
		*target = append(*target, silenceInterval{start: start, end: value})
		delete(open, instance)
	}
	// A stretch still open at the end of the file never gets its closing line.
	// Recorded with a negative end so the caller can tell "silent to the end"
	// from "not silent at the end", which are opposite answers.
	for instance, start := range open {
		target := &primary
		if instance != "0" {
			target = &onset
		}
		*target = append(*target, silenceInterval{start: start, end: -1})
	}
	return primary, onset
}

// durationLineRe reads the length ffmpeg actually decoded, which for a tail
// window is the window and not the track.
var durationLineRe = regexp.MustCompile(`time=(\d+):(\d+):([\d.]+)`)

func windowLength(output []byte, opts SilenceDetectOptions, limit float64) float64 {
	matches := durationLineRe.FindAllStringSubmatch(string(output), -1)
	if len(matches) > 0 {
		last := matches[len(matches)-1]
		h, _ := strconv.ParseFloat(last[1], 64)
		m, _ := strconv.ParseFloat(last[2], 64)
		s, _ := strconv.ParseFloat(last[3], 64)
		if total := h*3600 + m*60 + s; total > 0 {
			return total
		}
	}
	if limit > 0 {
		return limit
	}
	return opts.Duration
}

// edgeAt reduces the intervals to what one end of the window looks like.
//
// Only an interval touching the edge counts. Silence in the middle of a track
// is a gap between movements or a pause the artist put there, and removing it
// would be editing the music rather than trimming the file.
func edgeAt(primary, onset []silenceInterval, windowStart, windowEnd float64, leading bool) edgeSilence {
	const edgeTolerance = 0.05

	find := func(intervals []silenceInterval) (float64, bool) {
		for _, iv := range intervals {
			if leading {
				if iv.start <= windowStart+edgeTolerance && iv.end > 0 {
					return iv.end - windowStart, true
				}
				continue
			}
			// Trailing: either ffmpeg closed the interval at the very end, or it
			// never closed it at all because the file ran out while silent.
			if iv.end < 0 {
				return windowEnd - iv.start, true
			}
			if windowEnd > 0 && iv.end >= windowEnd-edgeTolerance {
				return iv.end - iv.start, true
			}
		}
		return 0, false
	}

	silence, ok := find(primary)
	if !ok {
		return edgeSilence{}
	}
	onsetSilence, onsetOK := find(onset)
	if !onsetOK {
		// The louder threshold found no silence at this edge at all, which means
		// it never rose above it - the whole window is below -45 dB. Treated as
		// a maximal gap so the fade guard refuses rather than trusting a
		// boundary the second measurement could not confirm.
		return edgeSilence{silence: silence, onsetGap: silence}
	}
	gap := onsetSilence - silence
	if gap < 0 {
		gap = 0
	}
	return edgeSilence{silence: silence, onsetGap: gap}
}
