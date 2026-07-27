package ffmpeg

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
)

const (
	probeFileCmd = "ffprobe -v quiet -print_format json -show_streams -show_format %s"
)

// FileProbe is the full container/stream snapshot of one audio file: every
// property that loudness normalization must leave untouched.
type FileProbe struct {
	Codec      string
	BitRate    int // kbps
	SampleRate int
	BitDepth   int
	Channels   int
	Duration   float64
	Size       int64
	HasArt     bool
}

type fullProbeOutput struct {
	Streams []fullProbeStream `json:"streams"`
	Format  fullProbeFormat   `json:"format"`
}

type fullProbeFormat struct {
	BitRate  string `json:"bit_rate"`
	Duration string `json:"duration"`
	Size     string `json:"size"`
}

type fullProbeStream struct {
	CodecName        string `json:"codec_name"`
	CodecType        string `json:"codec_type"`
	SampleRate       string `json:"sample_rate"`
	BitRate          string `json:"bit_rate"`
	Duration         string `json:"duration"`
	Channels         int    `json:"channels"`
	BitsPerSample    int    `json:"bits_per_sample"`
	BitsPerRawSample string `json:"bits_per_raw_sample"`
}

// ProbeFile reads the container and stream properties of path, including
// whether an embedded cover art (image) stream is present.
func ProbeFile(ctx context.Context, path string) (*FileProbe, error) {
	if _, err := ffmpegCmd(); err != nil {
		return nil, err
	}
	if err := fileExists(path); err != nil {
		return nil, err
	}
	args := createFFmpegCommand(probeFileCmd, path, 0, 0)
	output, err := runCommand(ctx, probeTimeout, args[0], args[1:]...)
	if err != nil {
		return nil, fmt.Errorf("probing %q: %w", path, err)
	}

	var parsed fullProbeOutput
	if err := json.Unmarshal(output, &parsed); err != nil {
		return nil, fmt.Errorf("parsing probe output for %q: %w", path, err)
	}

	res := &FileProbe{}
	for _, s := range parsed.Streams {
		switch s.CodecType {
		case "audio":
			if res.Codec != "" {
				continue // only the first audio stream matters
			}
			res.Codec = s.CodecName
			res.Channels = s.Channels
			res.SampleRate, _ = strconv.Atoi(s.SampleRate)
			res.BitDepth = s.BitsPerSample
			if res.BitDepth == 0 && s.BitsPerRawSample != "" {
				res.BitDepth, _ = strconv.Atoi(s.BitsPerRawSample)
			}
			if bps, err := strconv.Atoi(s.BitRate); err == nil {
				res.BitRate = bps / 1000
			}
			res.Duration, _ = strconv.ParseFloat(s.Duration, 64)
		case "video":
			// In audio containers a video stream is the embedded cover art
			res.HasArt = true
		}
	}
	if res.Codec == "" {
		return nil, fmt.Errorf("no audio stream found in %q", path)
	}
	if res.BitRate == 0 {
		if bps, err := strconv.Atoi(parsed.Format.BitRate); err == nil {
			res.BitRate = bps / 1000
		}
	}
	if res.Duration == 0 {
		res.Duration, _ = strconv.ParseFloat(parsed.Format.Duration, 64)
	}
	res.Size, _ = strconv.ParseInt(parsed.Format.Size, 10, 64)
	if res.Size == 0 {
		if st, err := os.Stat(path); err == nil {
			res.Size = st.Size()
		}
	}
	return res, nil
}

// probeDuration reads just the length of a file, for sizing a decode timeout.
// A failure here is not fatal: DecodeTimeout falls back to a fixed bound.
func probeDuration(ctx context.Context, path string) float64 {
	probe, err := ProbeFile(ctx, path)
	if err != nil {
		return 0
	}
	return probe.Duration
}

var astatsPeakRe = regexp.MustCompile(`Peak level dB:\s*(-?\d+(?:\.\d+)?|-?inf)`)

// NullResidual measures how much of the audio changed beyond a level shift.
//
// It undoes gainDB on the "after" file, subtracts it from the "before" file and
// returns the peak level of what remains, in dBFS. A pure gain change leaves
// only codec noise (around -70 dB); anything that reshaped the audio - dynamic
// processing, limiting, resampling, a lossy re-encode - leaves far more.
// Returns -inf as -120.
func NullResidual(ctx context.Context, beforePath, afterPath string, gainDB float64) (float64, error) {
	cmdPath, err := ffmpegCmd()
	if err != nil {
		return 0, err
	}
	if err := fileExists(beforePath); err != nil {
		return 0, err
	}
	if err := fileExists(afterPath); err != nil {
		return 0, err
	}

	filter := fmt.Sprintf(
		"[0:a]aresample=44100,aformat=sample_fmts=fltp:channel_layouts=stereo[a];"+
			"[1:a]volume=%sdB,aresample=44100,aformat=sample_fmts=fltp:channel_layouts=stereo[b];"+
			"[a][b]amerge=inputs=2,pan=mono|c0=c0-c2,astats=metadata=1:reset=0",
		formatFloat(-gainDB))

	args := []string{"-nostdin", "-hide_banner", "-i", beforePath, "-i", afterPath,
		"-filter_complex", filter, "-f", "null", "-"}
	// Both files are decoded in full, so allow for the longer of the two.
	timeout := DecodeTimeout(math.Max(probeDuration(ctx, beforePath), probeDuration(ctx, afterPath)))
	output, err := runCommand(ctx, timeout, cmdPath, args...)
	if err != nil {
		return 0, fmt.Errorf("measuring null residual: %w: %s", err, string(output))
	}

	matches := astatsPeakRe.FindAllStringSubmatch(string(output), -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("null residual: peak level not found in ffmpeg output")
	}
	value := matches[len(matches)-1][1]
	if value == "-inf" {
		return -120, nil
	}
	peak, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("null residual: invalid peak level %q: %w", value, err)
	}
	return peak, nil
}
