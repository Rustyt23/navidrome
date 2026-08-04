package ffmpeg

import (
	"fmt"
	"strconv"
	"strings"
)

// encoderSpec is the exact set of output arguments needed to re-encode a file
// into the same format it came from. Loudness normalization must change the
// level and nothing else, so every property is pinned to the source rather
// than left to ffmpeg's defaults - which is how 320 kbps sources ended up at
// 128 kbps and 44.1 kHz sources at 48 kHz.
type encoderSpec struct {
	Codec     string // ffmpeg encoder name
	Bitrate   int    // kbps, 0 for lossless
	SampleFmt string // for lossless formats with a bit depth
	Lossless  bool
}

// standardBitrates are the rungs a lossy bitrate is rounded up to. Rounding up
// (never down) guarantees the re-encode is never given less data to work with
// than the source had.
var standardBitrates = []int{96, 128, 160, 192, 224, 256, 320}

// MaxBitrateFor is the highest bitrate a codec can actually encode at, or 0
// where there is no fixed ceiling. Asking for more is not an error - libmp3lame
// silently clamps to 320 - which is exactly the problem: the output then
// measures below the source and looks like lost quality.
func MaxBitrateFor(codec string) int {
	switch strings.ToLower(codec) {
	case "mp3":
		return 320
	default:
		return 0
	}
}

func roundUpBitrate(kbps int, codec string) int {
	wanted := kbps
	for _, b := range standardBitrates {
		if kbps <= b {
			wanted = b
			break
		}
	}
	// A source can probe above the format's ceiling when the stream reports no
	// bitrate and the container's figure - which counts cover art and tags -
	// stands in for it. Asking for the impossible just gets clamped anyway.
	if max := MaxBitrateFor(codec); max > 0 && wanted > max {
		return max
	}
	return wanted
}

// encoderForSource picks the encoder settings that reproduce the source's
// format. An unknown codec is an error rather than a guess: silently writing a
// different format would be exactly the damage this is meant to prevent.
func encoderForSource(probe *FileProbe) (*encoderSpec, error) {
	if probe == nil {
		return nil, fmt.Errorf("no source probe")
	}
	switch strings.ToLower(probe.Codec) {
	case "mp3":
		return &encoderSpec{Codec: "libmp3lame", Bitrate: roundUpBitrate(probe.BitRate, "mp3")}, nil
	case "aac":
		return &encoderSpec{Codec: "aac", Bitrate: roundUpBitrate(probe.BitRate, "aac")}, nil
	case "opus":
		return &encoderSpec{Codec: "libopus", Bitrate: roundUpBitrate(probe.BitRate, "opus")}, nil
	case "vorbis":
		return &encoderSpec{Codec: "libvorbis", Bitrate: roundUpBitrate(probe.BitRate, "vorbis")}, nil
	case "flac":
		return &encoderSpec{Codec: "flac", SampleFmt: losslessSampleFmt(probe.BitDepth), Lossless: true}, nil
	case "alac":
		return &encoderSpec{Codec: "alac", SampleFmt: losslessSampleFmt(probe.BitDepth), Lossless: true}, nil
	case "wavpack":
		return &encoderSpec{Codec: "wavpack", SampleFmt: losslessSampleFmt(probe.BitDepth), Lossless: true}, nil
	case "pcm_s16le", "pcm_s24le", "pcm_s32le", "pcm_f32le":
		return &encoderSpec{Codec: probe.Codec, Lossless: true}, nil
	default:
		return nil, fmt.Errorf("unsupported codec %q: refusing to re-encode into a different format", probe.Codec)
	}
}

func losslessSampleFmt(bitDepth int) string {
	switch bitDepth {
	case 0:
		return "" // let the encoder keep its default
	case 8, 16:
		return "s16"
	case 24, 32:
		return "s32"
	default:
		return "s32"
	}
}

// outputArgs renders the encoder settings, plus the sample rate and channel
// layout taken from the source.
func (e *encoderSpec) outputArgs(probe *FileProbe) []string {
	args := []string{"-c:a", e.Codec}
	if !e.Lossless && e.Bitrate > 0 {
		args = append(args, "-b:a", strconv.Itoa(e.Bitrate)+"k")
	}
	if e.SampleFmt != "" {
		args = append(args, "-sample_fmt", e.SampleFmt)
	}
	if probe.SampleRate > 0 {
		args = append(args, "-ar", strconv.Itoa(probe.SampleRate))
	}
	if probe.Channels > 0 {
		args = append(args, "-ac", strconv.Itoa(probe.Channels))
	}
	return args
}
