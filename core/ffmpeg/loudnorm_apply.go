package ffmpeg

import (
	"context"
	"fmt"
	"math"
	"strings"
)

// staleReplayGainTags are the tags that describe the OLD level. Carried over
// unchanged they make a ReplayGain-enabled player undo the normalization, so
// they are cleared on the way out.
var staleReplayGainTags = []string{
	"replaygain_track_gain", "replaygain_track_peak",
	"replaygain_album_gain", "replaygain_album_peak",
	"REPLAYGAIN_TRACK_GAIN", "REPLAYGAIN_TRACK_PEAK",
	"REPLAYGAIN_ALBUM_GAIN", "REPLAYGAIN_ALBUM_PEAK",
	"r128_track_gain", "r128_album_gain",
}

// ApplySpec is a fully decided transform. Nothing here is left to ffmpeg's
// judgement: the gain is computed by the caller, and limiting only happens
// when the caller explicitly asks for it.
type ApplySpec struct {
	// GainDB is the constant level change. On its own this is mathematically
	// transparent: dynamics, timing and timbre are untouched.
	GainDB float64

	// LimitTruePeak, when set, adds a true-peak limiter after the gain. This
	// DOES reshape the loudest moments and is only used for tracks the client
	// has explicitly approved for limiting.
	LimitTruePeak bool
	// CeilingDB is the true-peak level the output must stay under.
	CeilingDB float64

	// Source is the probe of the input, used to pin the output format.
	Source *FileProbe
}

const (
	// maxOversampleRate caps the internal rate used for true-peak limiting.
	maxOversampleRate = 192000
)

// RewriteLoudnessCost returns how much integrated loudness a file loses purely
// from being rewritten at the given bitrate, in dB.
//
// Re-encoding a lossy file discards content, so what comes back measures
// quieter than the level change alone predicts. Measured by rewriting sources
// at their own bitrate with no gain applied at all, three tracks each:
//
//	128k -0.46   192k -0.26   256k 0.00   320k 0.00
//
// A plan that ignores this asks for a gain that lands short, and on a track
// whose ceiling leaves no room for the difference it then fails on every run,
// for ever. Bitrates between the measured points take the more expensive
// neighbour's figure, so the estimate is never optimistic.
func RewriteLoudnessCost(bitRate int) float64 {
	switch {
	case bitRate <= 0:
		// Unknown: assume the source is not degraded, rather than inflate every
		// gain on a guess.
		return 0
	case bitRate <= 160:
		return 0.46
	case bitRate <= 224:
		return 0.26
	default:
		return 0
	}
}

// limiterHeadroom returns how far below the ceiling the limiter should aim, for
// a source of the given bitrate in kbps.
//
// The limiter shapes the decoded waveform, but what ships is the re-encoded
// file, and encoding reconstructs the waveform imperfectly - so the true peak
// springs back up afterwards. How far depends almost entirely on the bitrate.
// Holding one track at -1.5 dBTP and re-encoding it at a range of bitrates
// gives, as the finished true peak:
//
//	96k -0.77   128k -0.73   160k -0.78   192k -1.11   256k -1.06   320k -1.25
//
// a spring-back of roughly 0.75 dB at and below 160k, 0.4 through the middle,
// and 0.25 at 320k. A single 0.3 dB allowance therefore holds for a
// high-bitrate file and cannot hold for a low-bitrate one, which comes out over
// the ceiling no matter how hard the limiter is asked to clamp.
func limiterHeadroom(bitRate int) float64 {
	switch {
	case bitRate <= 0:
		// Unknown bitrate: assume the source is not degraded rather than shave
		// a good file harder than it needs.
		return 0.3
	case bitRate <= 160:
		return 1.0
	case bitRate <= 256:
		return 0.6
	default:
		return 0.3
	}
}

// oversampleRate returns the rate the limiter should run at.
//
// A true peak is the level of the waveform *between* samples, which is what a
// DAC actually reconstructs, and it can sit well above the highest sample
// value. alimiter only sees sample values, so limiting at the source rate
// leaves those inter-sample peaks untouched. Running it on an upsampled signal
// makes them visible as ordinary samples, which is how true-peak limiters work.
func oversampleRate(sampleRate int) int {
	if sampleRate <= 0 {
		return 0
	}
	rate := sampleRate * 4
	if rate > maxOversampleRate {
		rate = maxOversampleRate
	}
	if rate < sampleRate {
		return sampleRate
	}
	return rate
}

// filter builds the audio filter chain.
func (s ApplySpec) filter() string {
	chain := []string{fmt.Sprintf("volume=%sdB", formatFloat(s.GainDB))}
	if s.LimitTruePeak {
		limit := dbToLinear(s.CeilingDB - limiterHeadroom(s.Source.BitRate))
		limiter := fmt.Sprintf("alimiter=limit=%s:level=disabled:attack=5:release=50",
			formatFloat(limit))

		if up := oversampleRate(s.Source.SampleRate); up > s.Source.SampleRate {
			// Upsample, catch the inter-sample peaks, then return to the
			// source rate - which is pinned on the output either way.
			chain = append(chain,
				fmt.Sprintf("aresample=%d", up),
				limiter,
				fmt.Sprintf("aresample=%d", s.Source.SampleRate))
		} else {
			chain = append(chain, limiter)
		}
	}
	return strings.Join(chain, ",")
}

// Apply writes inputPath to outputPath with the level change applied and every
// other property preserved: same codec, bitrate, sample rate, bit depth,
// channel count, metadata, chapters and embedded cover art.
func Apply(ctx context.Context, inputPath, outputPath string, spec ApplySpec) error {
	cmdPath, err := ffmpegCmd()
	if err != nil {
		return err
	}
	if err := fileExists(inputPath); err != nil {
		return err
	}
	enc, err := encoderForSource(spec.Source)
	if err != nil {
		return err
	}

	args := []string{"-nostdin", "-hide_banner", "-y", "-i", inputPath,
		"-map", "0:a:0",
		"-map_metadata", "0",
		"-map_chapters", "0",
	}
	// Carry the embedded cover art across. It must be mapped AND marked as an
	// attached picture, otherwise the muxer drops it - which is how artwork
	// was being destroyed.
	if spec.Source.HasArt {
		args = append(args, "-map", "0:v:0", "-c:v", "copy", "-disposition:v:0", "attached_pic")
	} else {
		args = append(args, "-vn")
	}
	args = append(args, "-af", spec.filter())
	args = append(args, enc.outputArgs(spec.Source)...)
	for _, tag := range staleReplayGainTags {
		args = append(args, "-metadata", tag+"=")
	}
	args = append(args, outputPath)

	output, err := runCommand(ctx, DecodeTimeout(spec.Source.Duration), cmdPath, args...)
	if err != nil {
		return fmt.Errorf("applying gain: %w: %s", err, string(output))
	}
	return nil
}

func dbToLinear(db float64) float64 {
	return math.Pow(10, db/20)
}
