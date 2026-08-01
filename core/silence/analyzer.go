// Package silence measures leading and trailing silence without modifying audio.
package silence

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/navidrome/navidrome/core/ffmpeg"
)

const (
	DefaultThresholdDB = -60.0
	// Five milliseconds exposes very short edge gaps and encoder padding in the
	// analysis. Trimming remains separately protected by the 100 ms safety pad.
	DefaultMinimumSilence = 0.005
	edgeTolerance         = 0.05
	maxAnalysisTimeout    = 2 * time.Hour
)

var (
	silenceStartPattern = regexp.MustCompile(`silence_start:\s*([-+0-9.eE]+)`)
	silenceEndPattern   = regexp.MustCompile(`silence_end:\s*([-+0-9.eE]+)\s*\|\s*silence_duration:\s*([-+0-9.eE]+)`)
	outTimePattern      = regexp.MustCompile(`(?m)^out_time_us=([0-9]+)\s*$`)
)

type Result struct {
	Leading  float64
	Trailing float64
	Duration float64
}

type Analyzer interface {
	Analyze(ctx context.Context, path string, duration float64) (Result, error)
}

type analyzer struct {
	thresholdDB   float64
	minimumLength float64
}

func NewAnalyzer() Analyzer {
	return &analyzer{thresholdDB: DefaultThresholdDB, minimumLength: DefaultMinimumSilence}
}

func (a *analyzer) Analyze(ctx context.Context, path string, duration float64) (Result, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Result{}, fmt.Errorf("open audio: %w", err)
	}
	if !info.Mode().IsRegular() {
		return Result{}, fmt.Errorf("audio path is not a regular file")
	}

	cmdPath, err := ffmpeg.New().CmdPath()
	if err != nil {
		return Result{}, fmt.Errorf("find ffmpeg: %w", err)
	}
	analysisCtx, cancel := context.WithTimeout(ctx, analysisTimeout(duration))
	defer cancel()

	// Reset timestamps so formats with a non-zero stream start still report
	// leading silence from zero. FFmpeg progress supplies the decoded duration,
	// which is more reliable for trailing-edge checks than container metadata.
	filter := fmt.Sprintf("asetpts=N/SR/TB,silencedetect=noise=%.1fdB:duration=%.3f", a.thresholdDB, a.minimumLength)
	cmd := exec.CommandContext(analysisCtx, cmdPath,
		"-nostdin", "-hide_banner", "-nostats", "-xerror", "-loglevel", "info",
		"-i", path, "-map", "0:a:0", "-vn", "-af", filter,
		"-progress", "pipe:2",
		"-f", "null", "-",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if analysisCtx.Err() != nil {
			return Result{}, analysisCtx.Err()
		}
		return Result{}, fmt.Errorf("ffmpeg silence analysis failed: %w: %s", err, tail(string(output), 2048))
	}
	return parseOutput(string(output), duration)
}

func analysisTimeout(duration float64) time.Duration {
	if duration <= 0 || math.IsNaN(duration) || math.IsInf(duration, 0) {
		return 30 * time.Minute
	}
	timeout := 2*time.Minute + time.Duration(duration*float64(time.Second))
	if timeout > maxAnalysisTimeout {
		return maxAnalysisTimeout
	}
	return timeout
}

type interval struct {
	start    float64
	end      float64
	duration float64
}

func parseOutput(output string, mediaDuration float64) (Result, error) {
	decodedDuration := parseDecodedDuration(output)
	if decodedDuration <= 0 {
		decodedDuration = mediaDuration
	}
	intervals := make([]interval, 0, 4)
	var openStart *float64
	for _, line := range strings.Split(output, "\n") {
		if match := silenceStartPattern.FindStringSubmatch(line); len(match) == 2 {
			start, err := strconv.ParseFloat(match[1], 64)
			if err == nil {
				openStart = &start
			}
		}
		if match := silenceEndPattern.FindStringSubmatch(line); len(match) == 3 {
			end, endErr := strconv.ParseFloat(match[1], 64)
			length, lengthErr := strconv.ParseFloat(match[2], 64)
			if endErr != nil || lengthErr != nil {
				continue
			}
			start := end - length
			if openStart != nil {
				start = *openStart
			}
			intervals = append(intervals, interval{start: max(0, start), end: end, duration: max(0, length)})
			openStart = nil
		}
	}

	// Some FFmpeg builds report an open silence interval at EOF without a final
	// silence_end line. The known media duration safely closes that interval.
	if openStart != nil && decodedDuration > *openStart {
		intervals = append(intervals, interval{
			start: *openStart, end: decodedDuration, duration: decodedDuration - *openStart,
		})
	}

	result := Result{Duration: decodedDuration}
	if len(intervals) == 0 {
		return result, nil
	}
	first := intervals[0]
	if first.start <= edgeTolerance {
		result.Leading = clamp(first.end, 0, decodedDuration)
	}
	last := intervals[len(intervals)-1]
	if decodedDuration > 0 && last.end >= decodedDuration-edgeTolerance {
		result.Trailing = clamp(decodedDuration-last.start, 0, decodedDuration)
	}
	if math.IsNaN(result.Leading) || math.IsNaN(result.Trailing) {
		return Result{}, errors.New("ffmpeg returned an invalid silence duration")
	}
	return result, nil
}

func parseDecodedDuration(output string) float64 {
	matches := outTimePattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return 0
	}
	microseconds, err := strconv.ParseInt(matches[len(matches)-1][1], 10, 64)
	if err != nil || microseconds <= 0 {
		return 0
	}
	return float64(microseconds) / 1_000_000
}

func clamp(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if high > 0 && value > high {
		return high
	}
	return value
}

func tail(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[len(value)-limit:]
}
