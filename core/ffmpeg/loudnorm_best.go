package ffmpeg

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

const (
	// closeMissPrecompensationLUFS: when the original file misses the target
	// by no more than this, the first attempt pre-compensates for the
	// encoder's typical over/undershoot.
	closeMissPrecompensationLUFS = 1.0
	// maxLoudnessCompensationLUFS limits how far the attempt target may drift
	// from the real target when compensating.
	maxLoudnessCompensationLUFS = 3.0
	// minLoudnessTargetStepLUFS: stop retrying when the next compensation
	// step would barely change the attempt target (converged).
	minLoudnessTargetStepLUFS = 0.05
)

type NormalizeOptions struct {
	Tolerance    float64
	MaxAttempts  int
	Backup       bool
	BackupSuffix string
}

type NormalizeResult struct {
	OldLUFS   float64 // measured loudness before normalization
	FinalLUFS float64 // measured loudness of the file now on disk
	Changed   bool    // whether trackPath was replaced with a better version
	InRange   bool    // whether FinalLUFS is within tolerance of the target
	Attempts  int
}

// NormalizeToBest normalizes trackPath towards target.IntegratedLUFS with a
// best-result guarantee:
//
//   - Every attempt re-encodes from a pristine copy of the ORIGINAL file, so
//     retries never accumulate lossy re-encoding artifacts.
//   - Every attempt's output is measured, and only the attempt that lands
//     closest to the target replaces the original - and only if it is
//     strictly closer to the target than the original was.
//   - Consequently the file on disk is never left further from the target
//     than it started; if no attempt improves it, the original is untouched
//     (Changed=false).
//
// Attempt targets use error feedback: each retry shifts the requested LUFS by
// the remaining miss of the previous attempt, clamped to
// ±maxLoudnessCompensationLUFS around the real target.
func NormalizeToBest(ctx context.Context, normalizer LoudnessNormalizer, trackPath string, target LoudnessTarget, opts NormalizeOptions) (NormalizeResult, error) {
	res := NormalizeResult{}
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 3
	}

	stat, err := os.Stat(trackPath)
	if err != nil {
		return res, err
	}

	analysis, err := normalizer.AnalyzeLoudness(ctx, trackPath, target)
	if err != nil {
		return res, err
	}
	res.OldLUFS = analysis.InputIntegrated
	res.FinalLUFS = analysis.InputIntegrated
	oldDist := math.Abs(analysis.InputIntegrated - target.IntegratedLUFS)
	res.InRange = oldDist <= opts.Tolerance
	if res.InRange {
		return res, nil
	}

	dir := filepath.Dir(trackPath)
	base := filepath.Base(trackPath)
	ext := filepath.Ext(base)

	// Pristine copy of the original: all attempts encode from this file
	srcCopy, err := tempFileCopy(trackPath, dir, "."+base+".loudnorm-src-*"+ext, stat.Mode())
	if err != nil {
		return res, err
	}
	defer func() { _ = os.Remove(srcCopy) }()

	var bestPath string
	bestDist := oldDist
	bestLUFS := analysis.InputIntegrated
	defer func() {
		if bestPath != "" {
			_ = os.Remove(bestPath)
		}
	}()

	attemptTarget := target
	if oldDist <= closeMissPrecompensationLUFS {
		attemptTarget = compensatedLoudnessTarget(target, target.IntegratedLUFS-analysis.InputIntegrated)
	}

	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		res.Attempts = attempt

		srcAnalysis, err := normalizer.AnalyzeLoudness(ctx, srcCopy, attemptTarget)
		if err != nil {
			return res, fmt.Errorf("analyzing source (attempt %d): %w", attempt, err)
		}

		out, err := reservedTempName(dir, "."+base+".loudnorm-out-*"+ext)
		if err != nil {
			return res, err
		}
		if err := normalizer.NormalizeLoudness(ctx, srcCopy, out, attemptTarget, *srcAnalysis); err != nil {
			_ = os.Remove(out)
			return res, fmt.Errorf("normalizing (attempt %d): %w", attempt, err)
		}

		verify, err := normalizer.AnalyzeLoudness(ctx, out, target)
		if err != nil {
			_ = os.Remove(out)
			return res, fmt.Errorf("verifying (attempt %d): %w", attempt, err)
		}
		dist := math.Abs(verify.InputIntegrated - target.IntegratedLUFS)

		if dist < bestDist {
			if bestPath != "" {
				_ = os.Remove(bestPath)
			}
			bestPath = out
			bestDist = dist
			bestLUFS = verify.InputIntegrated
		} else {
			_ = os.Remove(out)
		}

		if bestDist <= opts.Tolerance {
			break
		}

		// Error feedback: current compensation plus this attempt's remaining miss
		delta := (attemptTarget.IntegratedLUFS - target.IntegratedLUFS) +
			(target.IntegratedLUFS - verify.InputIntegrated)
		next := compensatedLoudnessTarget(target, delta)
		if math.Abs(next.IntegratedLUFS-attemptTarget.IntegratedLUFS) < minLoudnessTargetStepLUFS {
			break // converged; another attempt would produce the same encode
		}
		attemptTarget = next
	}

	if bestPath == "" {
		// No attempt improved on the original: leave the file untouched
		return res, nil
	}

	if opts.Backup {
		suffix := opts.BackupSuffix
		if suffix == "" {
			suffix = ".before_loudnorm"
		}
		backupPath := trackPath + suffix
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			if err := copyFileWithMode(trackPath, backupPath, stat.Mode()); err != nil {
				return res, fmt.Errorf("creating loudness backup: %w", err)
			}
		} else if err != nil {
			return res, err
		}
	}

	if err := os.Chmod(bestPath, stat.Mode()); err != nil {
		return res, err
	}
	if err := os.Rename(bestPath, trackPath); err != nil {
		return res, err
	}
	bestPath = "" // consumed; skip cleanup in defer
	res.FinalLUFS = bestLUFS
	res.Changed = true
	res.InRange = bestDist <= opts.Tolerance
	return res, nil
}

// compensatedLoudnessTarget returns target with its integrated LUFS shifted
// by delta, clamped to ±maxLoudnessCompensationLUFS around the real target.
func compensatedLoudnessTarget(target LoudnessTarget, delta float64) LoudnessTarget {
	t := target
	lufs := target.IntegratedLUFS + delta
	lufs = math.Max(lufs, target.IntegratedLUFS-maxLoudnessCompensationLUFS)
	lufs = math.Min(lufs, target.IntegratedLUFS+maxLoudnessCompensationLUFS)
	t.IntegratedLUFS = lufs
	return t
}

// reservedTempName reserves a unique file name in dir and removes the file so
// ffmpeg can create it itself.
func reservedTempName(dir, pattern string) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	if err := os.Remove(name); err != nil {
		return "", err
	}
	return name, nil
}

func tempFileCopy(src, dir, pattern string, mode os.FileMode) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	name := f.Name()
	in, err := os.Open(src)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(name)
		return "", err
	}
	_, copyErr := io.Copy(f, in)
	_ = in.Close()
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(name)
		if copyErr != nil {
			return "", copyErr
		}
		return "", closeErr
	}
	if err := os.Chmod(name, mode); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

func copyFileWithMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	return out.Close()
}
