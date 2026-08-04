// Command measure-null-floor finds the real null-test floor for each bitrate.
//
// The floor is the leftover a null test reports when a file is rewritten with
// no gain at all: nothing about the audio changed, so whatever is left is the
// cost of re-encoding. Anything above it means something else happened, which
// is the judgement NullResidualThreshold exists to make.
//
// Those thresholds are the last figures in the loudness system that were
// reasoned about rather than measured. Every other codec-sensitive number - what
// a rewrite costs in loudness, how far a peak springs back - came from running
// real files. This closes the gap, on the library the thresholds will actually
// be applied to rather than on someone else's.
//
// Usage:
//
//	go run ./cmd/measure-null-floor -dir /path/to/music -per-bucket 5
//
// It reads files, writes only to a temporary directory, and changes nothing in
// the library.
package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/navidrome/navidrome/core/ffmpeg"
)

// buckets mirror the ranges NullResidualThreshold switches on, so the output
// maps onto the constants one for one.
var buckets = []struct {
	name string
	max  int // inclusive upper bound in kbps; 0 means "no bound"
}{
	{"<=160k", 160},
	{"161-256k", 256},
	{">256k", 0},
}

func bucketFor(bitRate int) int {
	for i, b := range buckets {
		if b.max == 0 || bitRate <= b.max {
			return i
		}
	}
	return len(buckets) - 1
}

func main() {
	dir := flag.String("dir", "", "folder of audio files to sample (required)")
	perBucket := flag.Int("per-bucket", 5, "how many files to measure per bitrate bucket")
	flag.Parse()

	if strings.TrimSpace(*dir) == "" {
		fmt.Fprintln(os.Stderr, "usage: measure-null-floor -dir /path/to/music [-per-bucket 5]")
		os.Exit(2)
	}

	ctx := context.Background()
	work, err := os.MkdirTemp("", "null-floor-*")
	if err != nil {
		fatal(err)
	}
	defer os.RemoveAll(work)

	results := make([][]float64, len(buckets))
	counts := make([]int, len(buckets))

	err = filepath.WalkDir(*dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // an unreadable corner of the library is not a reason to stop
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".mp3", ".m4a", ".aac", ".ogg", ".opus":
		default:
			return nil
		}

		probe, err := ffmpeg.ProbeFile(ctx, path)
		if err != nil || probe.BitRate <= 0 {
			return nil //nolint:nilerr
		}
		b := bucketFor(probe.BitRate)
		if counts[b] >= *perBucket {
			return nil
		}

		// Rewritten at its own settings with a gain of exactly zero. Whatever
		// the null test finds is therefore the cost of the round trip and
		// nothing else - which is precisely the floor being measured.
		out := filepath.Join(work, fmt.Sprintf("b%d-%d%s", b, counts[b], filepath.Ext(path)))
		if err := ffmpeg.Apply(ctx, path, out, ffmpeg.ApplySpec{GainDB: 0, Source: probe}); err != nil {
			fmt.Fprintf(os.Stderr, "  skipped %s: %v\n", filepath.Base(path), err)
			return nil
		}
		residual, err := ffmpeg.NullResidual(ctx, path, out, 0)
		_ = os.Remove(out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  skipped %s: %v\n", filepath.Base(path), err)
			return nil
		}

		results[b] = append(results[b], residual)
		counts[b]++
		fmt.Printf("  %-9s %4dk  %7.1f dB  %s\n",
			buckets[b].name, probe.BitRate, residual, filepath.Base(path))
		return nil
	})
	if err != nil {
		fatal(err)
	}

	fmt.Println("\nWorst leftover per bucket, and the floor it implies.")
	fmt.Println("The floor sits above the worst clean rewrite with margin, and must stay")
	fmt.Println("well below the -10 to -15 dB that genuinely reshaped audio produces.")
	fmt.Println()
	for i, b := range buckets {
		if len(results[i]) == 0 {
			fmt.Printf("  %-9s no files found\n", b.name)
			continue
		}
		sort.Float64s(results[i])
		worst := results[i][len(results[i])-1]
		// Round up to the next 5 dB above the worst clean result: enough margin
		// that ordinary variation cannot cross it, still far from reshaping.
		suggested := math.Ceil((worst+5)/5) * 5
		fmt.Printf("  %-9s n=%d  worst %7.1f dB  →  suggested floor %.0f dB\n",
			b.name, len(results[i]), worst, suggested)
	}
	fmt.Println("\nPut these in nullResidual*DB in core/loudness/audit.go.")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
