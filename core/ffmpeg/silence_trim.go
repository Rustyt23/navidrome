package ffmpeg

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SilenceTrimmer cuts a decided amount off the head and tail of a track.
type SilenceTrimmer interface {
	TrimSilence(ctx context.Context, path string, spec TrimSpec) (*TrimResult, error)
}

func NewSilenceTrimmer() SilenceTrimmer { return &ffmpeg{} }

// TrimSpec is a fully decided cut. Where to cut is settled by the caller; this
// layer only carries it out and reports what it produced.
type TrimSpec struct {
	// StartSeconds is the new beginning, in seconds from the original start.
	StartSeconds float64
	// EndSeconds is the new end, in seconds from the original start. Zero means
	// "to the end of the file".
	EndSeconds float64
	// Source is the probe of the input, used to choose the strategy and to pin
	// the output format when re-encoding.
	Source *FileProbe
}

// TrimResult reports what was produced.
type TrimResult struct {
	// Method is how the cut was made - see model.SilenceMethodCopy/Encode.
	Method string
	// DurationAfter is the finished file's real decoded length.
	DurationAfter float64
	SizeAfter     int64
}

// durationToleranceSeconds is how far the finished file's real length may sit
// from what was asked for.
//
// A copy cuts on frame boundaries rather than on samples, so it always lands a
// little wide: an MP3 frame is ~26 ms at 44.1 kHz, and a measured copy came out
// 28.6 ms long. That is irrelevant against a 500 ms margin. What this is really
// for is catching a container whose header and contents disagree - see
// verifyTrimmed.
const durationToleranceSeconds = 0.25

// canCopyCodec reports whether a codec can be cut by rewrapping its existing
// compressed frames, producing a file whose header tells the truth.
//
// This is not about whether ffmpeg will accept `-c copy` - it accepts it for
// all of these - but about whether the result is honest. Measured on a 28.9 s
// source cut to 21.0 s:
//
//	mp3   header 21.029  decodes 21.01   agree
//	m4a   header 21.008  decodes 21.00   agree
//	flac  header 28.900  decodes 21.01   header keeps the ORIGINAL sample count
//	opus  header 21.020  decodes 21.71   Ogg page granularity misses by 0.7 s
//
// FLAC and ALAC re-encode losslessly, so routing them to a re-encode costs no
// quality at all - only time. Opus and Vorbis are lossy and do lose a
// generation, which is why they are re-encoded at the source's own bitrate and
// not silently copied into a file that lies about its length.
func canCopyCodec(codec string) bool {
	switch strings.ToLower(codec) {
	case "mp3", "aac", "alac":
		return true
	default:
		return false
	}
}

// CanCopyCodec exposes the strategy table so a planner can report which method
// a track will get before the cut is made. The trimmer still decides for itself
// at cut time; this only has to agree with it.
func CanCopyCodec(codec string) bool { return canCopyCodec(codec) }

// TrimSilence writes a trimmed copy of path and replaces the original with it.
//
// The new file is built alongside the old one and only swapped in once it has
// been probed and found to be what was asked for. Anything short of that leaves
// the original exactly as it was.
func (e *ffmpeg) TrimSilence(ctx context.Context, path string, spec TrimSpec) (*TrimResult, error) {
	cmdPath, err := ffmpegCmd()
	if err != nil {
		return nil, err
	}
	if err := fileExists(path); err != nil {
		return nil, err
	}
	if spec.Source == nil {
		return nil, fmt.Errorf("trimming %q: no source probe", path)
	}
	if spec.StartSeconds < 0 || (spec.EndSeconds > 0 && spec.EndSeconds <= spec.StartSeconds) {
		return nil, fmt.Errorf("trimming %q: nonsensical cut %.3f..%.3f",
			path, spec.StartSeconds, spec.EndSeconds)
	}

	method := SilenceMethodEncodeName
	if canCopyCodec(spec.Source.Codec) {
		method = SilenceMethodCopyName
	}

	// Written into the library folder rather than a temp dir so the final move
	// is a rename within one filesystem: an atomic swap, not a copy that can be
	// interrupted half-written over the client's only copy of the song.
	tmp := filepath.Join(filepath.Dir(path),
		fmt.Sprintf(".%s.silencetrim%s", filepath.Base(path), filepath.Ext(path)))
	defer func() { _ = os.Remove(tmp) }()

	args := trimArgs(path, tmp, spec, method)
	timeout := DecodeTimeout(spec.Source.Duration)
	if output, err := runCommand(ctx, timeout, cmdPath, args...); err != nil {
		return nil, fmt.Errorf("trimming %q: %w: %s", path, err, string(output))
	}

	wanted := spec.EndSeconds - spec.StartSeconds
	if spec.EndSeconds <= 0 {
		wanted = spec.Source.Duration - spec.StartSeconds
	}
	result, err := verifyTrimmed(ctx, tmp, wanted, method)
	if err != nil {
		return nil, fmt.Errorf("trimming %q: %w", path, err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return nil, fmt.Errorf("replacing %q with the trimmed file: %w", path, err)
	}
	return result, nil
}

// Names duplicated from the model package, which this one cannot import.
const (
	SilenceMethodCopyName   = "copy"
	SilenceMethodEncodeName = "encode"
)

func trimArgs(input, output string, spec TrimSpec, method string) []string {
	args := []string{"-nostdin", "-hide_banner", "-nostats", "-y"}

	// Both -ss and -to go BEFORE -i, which makes them input options: they name
	// positions in the source's own timeline, which is what the planner computed
	// them in. Put after -i, -to becomes an output option meaning "this many
	// seconds of output", and since -ss has already restarted output timestamps
	// at zero, a cut of 2.7..23.7 then yields 23.7 seconds instead of 21.0 -
	// silently keeping most of the trailing silence it was asked to remove.
	//
	// Seeking on the input rather than decoding up to the cut is also what makes
	// the copy path nearly free. Attached cover art survives it.
	if spec.StartSeconds > 0 {
		args = append(args, "-ss", strconv.FormatFloat(spec.StartSeconds, 'f', 6, 64))
	}
	if spec.EndSeconds > 0 {
		args = append(args, "-to", strconv.FormatFloat(spec.EndSeconds, 'f', 6, 64))
	}
	args = append(args, "-i", input)

	if method == SilenceMethodCopyName {
		// Every stream is mapped so the embedded art travels with the audio;
		// -map_metadata keeps the tags. Both ride along untouched, which is the
		// other half of what makes this path lossless.
		args = append(args, "-map", "0", "-c", "copy", "-map_metadata", "0")
		return append(args, output)
	}

	args = append(args, "-map", "0:a:0", "-map_metadata", "0")
	// Cover art is copied through as a still image rather than re-encoded.
	// Re-encoding it is how the loudness path used to lose it.
	if spec.Source.HasArt {
		args = append(args, "-map", "0:v:0?", "-c:v", "copy", "-disposition:v", "attached_pic")
	}
	args = append(args, encoderArgsForTrim(spec.Source)...)
	return append(args, output)
}

// encoderArgsForTrim pins the output format to the source's.
//
// Deliberately a separate function from the loudness path's encoder selection,
// which this feature does not share. It is also more forgiving: an unknown
// codec here falls back to the source's own encoder name rather than refusing,
// because a trim that cannot be copied and cannot be re-encoded would otherwise
// simply fail, and the planner has already decided this file is worth cutting.
func encoderArgsForTrim(probe *FileProbe) []string {
	var args []string
	switch strings.ToLower(probe.Codec) {
	case "flac":
		args = []string{"-c:a", "flac"}
		if fmtName := losslessSampleFmt(probe.BitDepth); fmtName != "" {
			args = append(args, "-sample_fmt", fmtName)
		}
	case "alac":
		args = []string{"-c:a", "alac"}
	case "opus":
		args = []string{"-c:a", "libopus", "-b:a", bitrateArg(probe.BitRate, 192)}
	case "vorbis":
		args = []string{"-c:a", "libvorbis", "-b:a", bitrateArg(probe.BitRate, 192)}
	case "mp3":
		args = []string{"-c:a", "libmp3lame", "-b:a", bitrateArg(probe.BitRate, 320)}
	case "aac":
		args = []string{"-c:a", "aac", "-b:a", bitrateArg(probe.BitRate, 256)}
	case "wavpack":
		args = []string{"-c:a", "wavpack"}
	default:
		args = []string{"-c:a", probe.Codec}
	}
	if probe.SampleRate > 0 {
		args = append(args, "-ar", strconv.Itoa(probe.SampleRate))
	}
	if probe.Channels > 0 {
		args = append(args, "-ac", strconv.Itoa(probe.Channels))
	}
	return args
}

func bitrateArg(kbps, fallback int) string {
	if kbps <= 0 {
		kbps = fallback
	}
	return strconv.Itoa(kbps) + "k"
}

// verifyTrimmed checks the finished file is really the length it was asked to
// be, by decoding it rather than by reading its header.
//
// Reading the header would miss the exact failure this is here to catch. A FLAC
// cut with `-c copy` reports its ORIGINAL duration in STREAMINFO while holding
// the trimmed audio: the header says 28.9 s, the file plays for 21 s. Trusting
// it would hand the scanner a wrong duration and leave the player hanging past
// the end of every trimmed track. Codecs that cannot be copied honestly are
// already routed away from the copy path, so this is the backstop rather than
// the defence - but it is the check that would have caught the FLAC case had
// the strategy table been wrong.
func verifyTrimmed(ctx context.Context, path string, wanted float64, method string) (*TrimResult, error) {
	probe, err := ProbeFile(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("probing the trimmed file: %w", err)
	}
	actual, err := decodedDuration(ctx, path)
	if err != nil {
		return nil, err
	}
	if math.Abs(actual-wanted) > durationToleranceSeconds {
		return nil, fmt.Errorf("the trimmed file is %.3fs but %.3fs was expected", actual, wanted)
	}
	// The header is what every other reader of this file will believe. If it
	// disagrees with what actually decodes, the file is not fit to ship however
	// correct its audio is.
	if math.Abs(probe.Duration-actual) > durationToleranceSeconds {
		return nil, fmt.Errorf(
			"the trimmed file's header claims %.3fs but it decodes to %.3fs", probe.Duration, actual)
	}
	return &TrimResult{Method: method, DurationAfter: actual, SizeAfter: probe.Size}, nil
}

// decodedDuration decodes the file and reports how much audio actually came
// out, which is the only number a broken header cannot fake.
func decodedDuration(ctx context.Context, path string) (float64, error) {
	cmdPath, err := ffmpegCmd()
	if err != nil {
		return 0, err
	}
	args := []string{"-nostdin", "-hide_banner", "-nostats", "-i", path, "-map", "0:a:0", "-f", "null", "-"}
	output, err := runCommand(ctx, unknownDurationTimeout, cmdPath, args...)
	if err != nil {
		return 0, fmt.Errorf("verifying the trimmed file: %w: %s", err, string(output))
	}
	matches := durationLineRe.FindAllStringSubmatch(string(output), -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("verifying the trimmed file: ffmpeg reported no duration")
	}
	last := matches[len(matches)-1]
	h, _ := strconv.ParseFloat(last[1], 64)
	m, _ := strconv.ParseFloat(last[2], 64)
	s, _ := strconv.ParseFloat(last[3], 64)
	return h*3600 + m*60 + s, nil
}
