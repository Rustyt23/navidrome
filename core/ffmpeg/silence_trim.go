package ffmpeg

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const (
	SilenceTrimMethodLossless   = "lossless_sample_trim"
	SilenceTrimMethodPacketCopy = "frame_aligned_copy"
)

// EdgeSilence is the interval touching one physical edge of a track. Strict
// silence is safe to propose automatically; quiet silence is informational
// and must be reviewed because it can contain fades, reverb or room tone.
type EdgeSilence struct {
	Strict float64
	Quiet  float64
}

type EdgeSilenceReport struct {
	Probe             *FileProbe
	Leading           EdgeSilence
	Trailing          EdgeSilence
	StrictThresholdDB float64
	QuietThresholdDB  float64
	MinimumDuration   float64
}

type silenceInterval struct {
	Start float64
	End   float64
}

var silenceEventRE = regexp.MustCompile(
	`(?:channel:\s*(\d+)\s*\|\s*)?silence_(start|end):\s*(-?\d+(?:\.\d+)?(?:e[+-]?\d+)?)`,
)
var decodedTimeRE = regexp.MustCompile(`(?m)^out_time_us=(\d+)\s*$`)

func isLosslessSilenceCodec(codec string) bool {
	switch strings.ToLower(codec) {
	case "flac", "alac", "wavpack",
		"pcm_s16le", "pcm_s24le", "pcm_s32le", "pcm_f32le":
		return true
	}
	return false
}

// SilenceTrimMethod returns the quality-preserving implementation available
// for a codec. Opus/Vorbis are intentionally not accepted: packet-copy seeking
// in their containers was observed to retain the original leading blank.
func SilenceTrimMethod(probe *FileProbe) (string, error) {
	if probe == nil {
		return "", fmt.Errorf("no source probe")
	}
	switch strings.ToLower(probe.Codec) {
	case "alac", "wavpack":
		return "", fmt.Errorf(
			"codec %q is lossless, but its sample-format and channel-layout preservation are not yet verified",
			probe.Codec,
		)
	}
	if isLosslessSilenceCodec(probe.Codec) {
		if probe.Channels > 2 {
			return "", fmt.Errorf(
				"lossless audio with %d channels requires channel-layout preservation that is not yet verified",
				probe.Channels,
			)
		}
		if _, err := encoderForSource(probe); err != nil {
			return "", err
		}
		return SilenceTrimMethodLossless, nil
	}
	switch strings.ToLower(probe.Codec) {
	case "mp3", "aac":
		return SilenceTrimMethodPacketCopy, nil
	default:
		return "", fmt.Errorf("codec %q has no verified quality-preserving edge-trim method", probe.Codec)
	}
}

// DetectEdgeSilence measures only intervals that touch the beginning or EOF.
// Internal pauses are parsed but deliberately ignored.
//
// mono=1 asks silencedetect to report every channel independently. The result
// takes the shortest qualifying interval across all channels, so one channel
// containing audio always prevents a cut.
func DetectEdgeSilence(ctx context.Context, path string, minimumDuration, quietThresholdDB float64) (*EdgeSilenceReport, error) {
	probe, err := ProbeFile(ctx, path)
	if err != nil {
		return nil, err
	}
	if probe.AudioStreams != 1 {
		return nil, fmt.Errorf("expected one audio stream, found %d", probe.AudioStreams)
	}
	if probe.Channels <= 0 {
		return nil, fmt.Errorf("audio channel count is unknown")
	}

	strictThreshold := -90.0
	if isLosslessSilenceCodec(probe.Codec) {
		// FFmpeg's s16 silencedetect path reports a zero-valued sample at its
		// quantisation floor (about -90.3 dB), so a lower threshold misses even
		// literal zero. Wider integer/float paths can use the much stricter
		// floor. Both were checked against generated zero/tone/zero fixtures.
		if probe.BitDepth > 16 || probe.BitDepth == 0 {
			strictThreshold = -150
		}
	}

	strictLeading, strictTrailing, err := detectSilenceAtThreshold(
		ctx, path, probe, minimumDuration, strictThreshold,
	)
	if err != nil {
		return nil, fmt.Errorf("detecting strict edge silence: %w", err)
	}
	quietLeading, quietTrailing, err := detectSilenceAtThreshold(
		ctx, path, probe, minimumDuration, quietThresholdDB,
	)
	if err != nil {
		return nil, fmt.Errorf("detecting near-silence at track edges: %w", err)
	}

	return &EdgeSilenceReport{
		Probe:             probe,
		Leading:           EdgeSilence{Strict: strictLeading, Quiet: quietLeading},
		Trailing:          EdgeSilence{Strict: strictTrailing, Quiet: quietTrailing},
		StrictThresholdDB: strictThreshold,
		QuietThresholdDB:  quietThresholdDB,
		MinimumDuration:   minimumDuration,
	}, nil
}

func detectSilenceAtThreshold(
	ctx context.Context,
	path string,
	probe *FileProbe,
	minimumDuration float64,
	thresholdDB float64,
) (float64, float64, error) {
	cmdPath, err := ffmpegCmd()
	if err != nil {
		return 0, 0, err
	}
	filter := fmt.Sprintf(
		"silencedetect=noise=%sdB:d=%s:mono=1",
		formatFloat(thresholdDB),
		formatFloat(minimumDuration),
	)
	args := []string{
		"-nostdin", "-hide_banner", "-i", path,
		"-map", "0:a:0", "-vn", "-af", filter,
		"-progress", "pipe:2", "-nostats",
		"-f", "null", "-",
	}
	output, err := runCommand(ctx, DecodeTimeout(probe.Duration), cmdPath, args...)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %s", err, string(output))
	}
	decodedDuration, err := decodedAudioDuration(output)
	if err != nil {
		return 0, 0, err
	}
	byChannel, err := parseSilenceIntervals(output, decodedDuration, probe.Channels)
	if err != nil {
		return 0, 0, err
	}
	return commonEdgeSilence(byChannel, decodedDuration, probe.Channels)
}

func decodedAudioDuration(output []byte) (float64, error) {
	matches := decodedTimeRE.FindAllSubmatch(output, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("ffmpeg did not report the decoded audio endpoint")
	}
	micros, err := strconv.ParseInt(string(matches[len(matches)-1][1]), 10, 64)
	if err != nil || micros <= 0 {
		return 0, fmt.Errorf("invalid decoded audio endpoint")
	}
	return float64(micros) / 1_000_000, nil
}

func parseSilenceIntervals(output []byte, duration float64, channels int) (map[int][]silenceInterval, error) {
	type channelState struct {
		start *float64
		items []silenceInterval
	}
	states := map[int]*channelState{}
	seenChannels := map[int]bool{}

	for _, match := range silenceEventRE.FindAllStringSubmatch(string(output), -1) {
		channel := 0
		if match[1] != "" {
			value, err := strconv.Atoi(match[1])
			if err != nil {
				return nil, fmt.Errorf("invalid silencedetect channel %q", match[1])
			}
			channel = value
		} else if channels > 1 {
			return nil, fmt.Errorf("silencedetect did not report per-channel results")
		}
		seenChannels[channel] = true
		state := states[channel]
		if state == nil {
			state = &channelState{}
			states[channel] = state
		}
		value, err := strconv.ParseFloat(match[3], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid silencedetect timestamp %q", match[3])
		}
		switch match[2] {
		case "start":
			v := math.Max(0, value)
			state.start = &v
		case "end":
			if state.start == nil {
				continue
			}
			state.items = append(state.items, silenceInterval{
				Start: *state.start,
				End:   math.Min(duration, math.Max(*state.start, value)),
			})
			state.start = nil
		}
	}

	intervals := make(map[int][]silenceInterval, channels)
	for channel := 0; channel < channels; channel++ {
		state := states[channel]
		if state == nil {
			// No silence events is a valid "this channel is not silent"
			// result. It must not be confused with a parser failure.
			intervals[channel] = nil
			continue
		}
		if state.start != nil {
			state.items = append(state.items, silenceInterval{Start: *state.start, End: duration})
		}
		intervals[channel] = state.items
	}
	_ = seenChannels
	return intervals, nil
}

func commonEdgeSilence(
	byChannel map[int][]silenceInterval,
	duration float64,
	channels int,
) (leading float64, trailing float64, err error) {
	leading = math.Inf(1)
	trailing = math.Inf(1)
	for channel := 0; channel < channels; channel++ {
		var channelLeading, channelTrailing float64
		for _, interval := range byChannel[channel] {
			// A silence interval must physically touch the edge. Extending a
			// nearby interval to the edge could discard a short opening
			// transient or an audible final tail.
			if interval.Start == 0 {
				channelLeading = math.Max(channelLeading, interval.End)
			}
			if interval.End >= duration {
				channelTrailing = math.Max(channelTrailing, duration-interval.Start)
			}
		}
		leading = math.Min(leading, channelLeading)
		trailing = math.Min(trailing, channelTrailing)
	}
	if math.IsInf(leading, 1) {
		leading = 0
	}
	if math.IsInf(trailing, 1) {
		trailing = 0
	}
	return math.Max(0, leading), math.Max(0, trailing), nil
}

// WriteSilenceTrimCandidate writes a proposed result without touching input.
// Lossless files are sample-trimmed and encoded losslessly. MP3/AAC audio
// packets are copied, never re-encoded.
func WriteSilenceTrimCandidate(
	ctx context.Context,
	inputPath string,
	outputPath string,
	probe *FileProbe,
	startSamples int64,
	endSamples int64,
) (string, error) {
	method, err := SilenceTrimMethod(probe)
	if err != nil {
		return "", err
	}
	if probe.SampleRate <= 0 {
		return "", fmt.Errorf("source sample rate is unknown")
	}
	totalSamples := int64(math.Round(probe.Duration * float64(probe.SampleRate)))
	endAtSample := totalSamples - endSamples
	if startSamples < 0 || endSamples < 0 || endAtSample <= startSamples {
		return "", fmt.Errorf("invalid trim boundaries")
	}

	cmdPath, err := ffmpegCmd()
	if err != nil {
		return "", err
	}
	if err := fileExists(inputPath); err != nil {
		return "", err
	}

	args := []string{"-nostdin", "-hide_banner", "-y"}
	metadataInput := "0"
	if method == SilenceTrimMethodPacketCopy {
		start := float64(startSamples) / float64(probe.SampleRate)
		keep := float64(endAtSample-startSamples) / float64(probe.SampleRate)
		// Seek/limit only the audio input. A second, unseeked input supplies
		// attached cover art, whose single packet is normally at timestamp 0
		// and would otherwise be dropped by an output-wide seek.
		args = append(args,
			"-ss", formatFloat(start),
			"-t", formatFloat(keep),
			"-i", inputPath,
		)
		if probe.HasArt {
			args = append(args, "-i", inputPath)
			metadataInput = "1"
		}
	} else {
		args = append(args, "-i", inputPath)
	}
	args = append(args,
		"-map", "0:a:0",
		"-map_metadata", metadataInput,
		"-map_chapters", metadataInput,
	)
	if probe.HasArt {
		args = append(
			args,
			"-map", metadataInput+":v:0",
			"-c:v", "copy",
			"-disposition:v:0", "attached_pic",
		)
	} else {
		args = append(args, "-vn")
	}

	if method == SilenceTrimMethodLossless {
		enc, err := encoderForSource(probe)
		if err != nil {
			return "", err
		}
		filter := fmt.Sprintf(
			"atrim=start_sample=%d:end_sample=%d,asetpts=PTS-STARTPTS",
			startSamples,
			endAtSample,
		)
		args = append(args, "-af", filter)
		args = append(args, enc.outputArgs(probe)...)
	} else {
		args = append(args, "-c:a", "copy", "-avoid_negative_ts", "make_zero")
	}
	args = append(args, outputPath)

	output, err := runCommand(ctx, DecodeTimeout(probe.Duration), cmdPath, args...)
	if err != nil {
		return "", fmt.Errorf("writing silence-trim candidate: %w: %s", err, string(output))
	}
	return method, nil
}

var sha256HashRE = regexp.MustCompile(`(?i)SHA256=([0-9a-f]{64})`)

func commandSHA256(ctx context.Context, duration float64, args ...string) (string, error) {
	cmdPath, err := ffmpegCmd()
	if err != nil {
		return "", err
	}
	output, err := runCommand(ctx, DecodeTimeout(duration), cmdPath, args...)
	if err != nil {
		return "", fmt.Errorf("hashing retained audio: %w: %s", err, string(output))
	}
	match := sha256HashRE.FindSubmatch(output)
	if len(match) != 2 {
		return "", fmt.Errorf("SHA-256 was not present in ffmpeg hash output")
	}
	return strings.ToLower(string(match[1])), nil
}

// VerifyLosslessRetainedPCM proves that every retained decoded sample is
// identical. Both sides are converted to the same canonical PCM representation
// before hashing.
func VerifyLosslessRetainedPCM(
	ctx context.Context,
	sourcePath string,
	candidatePath string,
	source *FileProbe,
	startSamples int64,
	endSamples int64,
) error {
	totalSamples := int64(math.Round(source.Duration * float64(source.SampleRate)))
	endAtSample := totalSamples - endSamples
	filter := fmt.Sprintf(
		"atrim=start_sample=%d:end_sample=%d,asetpts=PTS-STARTPTS",
		startSamples,
		endAtSample,
	)
	expected, err := commandSHA256(
		ctx,
		source.Duration,
		"-nostdin", "-hide_banner", "-i", sourcePath,
		"-map", "0:a:0", "-vn", "-af", filter,
		"-c:a", "pcm_s32le", "-f", "hash", "-hash", "sha256", "-",
	)
	if err != nil {
		return err
	}
	actual, err := commandSHA256(
		ctx,
		source.Duration,
		"-nostdin", "-hide_banner", "-i", candidatePath,
		"-map", "0:a:0", "-vn",
		"-c:a", "pcm_s32le", "-f", "hash", "-hash", "sha256", "-",
	)
	if err != nil {
		return err
	}
	if expected != actual {
		return fmt.Errorf("retained decoded audio does not match the source")
	}
	return nil
}

// VerifyCopiedPackets proves that the lossy audio payload was copied rather
// than re-encoded.
func VerifyCopiedPackets(
	ctx context.Context,
	sourcePath string,
	candidatePath string,
	source *FileProbe,
	startSamples int64,
	endSamples int64,
) error {
	totalSamples := int64(math.Round(source.Duration * float64(source.SampleRate)))
	start := float64(startSamples) / float64(source.SampleRate)
	keep := float64(totalSamples-startSamples-endSamples) / float64(source.SampleRate)

	expected, err := commandSHA256(
		ctx,
		source.Duration,
		"-nostdin", "-hide_banner",
		"-ss", formatFloat(start), "-t", formatFloat(keep),
		"-i", sourcePath,
		"-map", "0:a:0", "-vn", "-c:a", "copy",
		"-f", "hash", "-hash", "sha256", "-",
	)
	if err != nil {
		return err
	}
	actual, err := commandSHA256(
		ctx,
		source.Duration,
		"-nostdin", "-hide_banner", "-i", candidatePath,
		"-map", "0:a:0", "-vn", "-c:a", "copy",
		"-f", "hash", "-hash", "sha256", "-",
	)
	if err != nil {
		return err
	}
	if expected != actual {
		return fmt.Errorf("lossy audio packets do not match the source segment")
	}
	return nil
}
