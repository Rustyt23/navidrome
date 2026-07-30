package loudness

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/utils/filelock"
)

const (
	// maxOptimizeAttempts: encoding shifts both the measured loudness and the
	// true peak slightly. Each attempt feeds the measured miss back in, so a
	// couple of retries is enough to converge on both.
	maxOptimizeAttempts = 3
	// maxDriftCorrectionDB guards against feeding back a wild measurement.
	maxDriftCorrectionDB = 1.5
	// truePeakToleranceDB: how far over the ceiling a finished file may sit
	// before it is rejected. Only measurement noise belongs in here.
	truePeakToleranceDB = 0.1

	// fallbackCeilingDB is the highest true peak a file may ship at when the
	// configured ceiling turns out to be physically unreachable.
	//
	// Re-encoding pushes the true peak back up, and on a badly degraded source
	// it pushes it up further than any amount of limiting can pull it down -
	// clamping harder distorts the waveform more, which the encoder then
	// reconstructs worse. For such a track the choice is not between a safe
	// peak and an unsafe one, it is between landing slightly above the ceiling
	// and not reaching the target at all.
	//
	// It keeps what actually matters: the file still cannot clip, because
	// clipping starts at 0. What is given up is part of the reserve held back
	// for whatever handles the file next, and only on the few tracks that
	// cannot do better.
	//
	// -0.1 is where the measurements put the floor rather than where judgement
	// would. Across a real library the refused peaks landed at -0.17, -0.15,
	// -0.14, -0.11, -0.11 and then +0.01, +0.10, +0.21, +0.33, +0.39: a clean
	// gap with nothing in it between -0.11 and zero. So no value below this
	// rescues one more track, and every value above it ships audio that clips.
	// A track that cannot reach even here has a peak past zero, which no
	// ceiling can fix - it needs a better source file.
	fallbackCeilingDB = -0.1
)

// OptimizeOptions carries the target, the safety limits and where backups go.
type OptimizeOptions struct {
	Target      ffmpeg.LoudnessTarget
	Tolerance   float64
	Backup      bool
	LibraryPath string
	// BackupFolder is where untouched originals are kept.
	BackupFolder string
	// MediaFileID identifies the song, so its backup is stored under its own
	// identity rather than under whatever path it currently occupies.
	MediaFileID string
}

// OptimizeResult reports what happened to one track.
type OptimizeResult struct {
	Phase        int
	Decision     string
	Changed      bool
	OldLUFS      float64
	NewLUFS      float64
	GainDB       float64
	ExpectedLUFS float64
	Attempts     int
	Rejected     string // why a produced file was discarded, if it was
	Plan         Plan
	BeforeSet    *Measurement
	AfterSet     *Measurement
	// BackupCreated reports that this run is what stored the untouched
	// original. It is false when a backup was already there, which means the
	// stored original is older than anything this run measured.
	BackupCreated bool
	// CeilingRelaxed reports that the configured ceiling could not be reached
	// and the file was accepted against fallbackCeilingDB instead. The audio is
	// unaffected; only the headroom left above the peak is smaller.
	CeilingRelaxed bool
}

// Optimize brings one track to the target loudness without altering anything
// else about it.
//
// The transform is decided before the file is touched: the gain is computed
// from the measurement, and the output format is pinned to the source. The
// result is then measured and inspected, and only replaces the original if it
// is genuinely better and genuinely intact. Anything short of that is thrown
// away and the original is left exactly as it was.
func Optimize(ctx context.Context, normalizer ffmpeg.LoudnessNormalizer, trackPath, decision string,
	opts OptimizeOptions) (OptimizeResult, error) {
	unlock := filelock.Lock(trackPath)
	defer unlock()

	res := OptimizeResult{Decision: decision}

	stat, err := os.Stat(trackPath)
	if err != nil {
		return res, err
	}

	before, err := Measure(ctx, normalizer, trackPath, opts.Target)
	if err != nil {
		return res, err
	}
	res.BeforeSet = before
	res.OldLUFS = before.LUFS
	res.NewLUFS = before.LUFS

	plan := PlanFor(before.LUFS, before.TruePeak, opts.Target.IntegratedLUFS,
		opts.Target.TruePeak, opts.Tolerance, before.Probe.BitRate)
	res.Plan = plan
	res.Phase = plan.Phase

	spec, expectedLUFS, ok := SpecFor(plan, decision, before.Probe,
		opts.Target.IntegratedLUFS, opts.Target.TruePeak)
	if !ok {
		// Already on target, or a review track with no decision yet.
		return res, nil
	}
	res.ExpectedLUFS = expectedLUFS

	dir := filepath.Dir(trackPath)
	base := filepath.Base(trackPath)
	ext := filepath.Ext(base)

	var accepted string
	defer func() {
		if accepted != "" {
			_ = os.Remove(accepted)
		}
	}()

	// A produced file that lands on target but sits above the ceiling is kept
	// aside rather than thrown away, in case the ceiling turns out to be
	// unreachable. The lowest-peaked such file wins, so relaxing the ceiling
	// never costs more headroom than it has to.
	fallbackCeiling := math.Max(opts.Target.TruePeak, fallbackCeilingDB)
	var fallbackPath string
	var fallbackAfter *Measurement
	var fallbackGain float64
	defer func() {
		if fallbackPath != "" {
			_ = os.Remove(fallbackPath)
		}
	}()
	keepAsFallback := func(path string, after *Measurement, gain float64) bool {
		if !acceptableAsFallback(after.TruePeak, fallbackCeiling) {
			return false
		}
		if fallbackAfter != nil && after.TruePeak >= fallbackAfter.TruePeak {
			return false
		}
		if fallbackPath != "" {
			_ = os.Remove(fallbackPath)
		}
		fallbackPath, fallbackAfter, fallbackGain = path, after, gain
		return true
	}

	// Without a limiter the only way to hold a ceiling is to not exceed it in
	// the first place, so the gain may never rise above the largest one that
	// keeps the peaks under it - however the retry feedback pushes.
	//
	// The bound is the fallback ceiling rather than the configured one. What
	// ships is still decided by the acceptance checks below, which are stricter;
	// stopping the gain at the configured ceiling only meant a track that could
	// have reached the target never produced a file to judge.
	maxGain := math.Inf(1)
	if !spec.LimitTruePeak {
		maxGain = fallbackCeiling - before.TruePeak
	}
	ceiling := opts.Target.TruePeak

	for attempt := 1; attempt <= maxOptimizeAttempts; attempt++ {
		res.Attempts = attempt

		out, err := reservedTempPath(dir, "."+base+".lufs-*"+ext)
		if err != nil {
			return res, err
		}
		if err := ffmpeg.Apply(ctx, trackPath, out, spec); err != nil {
			_ = os.Remove(out)
			return res, err
		}

		after, err := Measure(ctx, normalizer, out, opts.Target)
		if err != nil {
			_ = os.Remove(out)
			return res, fmt.Errorf("verifying result: %w", err)
		}

		// The file must still be the same recording in the same format.
		if issues := IntegrityIssues(before, after); len(issues) > 0 {
			_ = os.Remove(out)
			res.Rejected = fmt.Sprintf("output was not format-identical: %v", issues)
			return res, nil
		}

		// Two things have to be true of the finished file: it lands where this
		// transform intended - the target normally, deliberately short of it
		// when the client chose to stay under the ceiling - and its peaks
		// respect the ceiling. Loudness alone is not enough: a file that hits
		// -12.6 by pushing peaks past the limit is not what was asked for.
		miss := expectedLUFS - after.LUFS
		peakOver := after.TruePeak - ceiling
		loudnessOK := math.Abs(miss) <= opts.Tolerance
		peakOK := peakOver <= truePeakToleranceDB

		if loudnessOK && peakOK {
			accepted = out
			res.AfterSet = after
			res.NewLUFS = after.LUFS
			res.GainDB = spec.GainDB
			break
		}

		// Only the ceiling was missed, so this file is still a usable outcome if
		// the ceiling proves unreachable. Everything else is discarded.
		if !(loudnessOK && keepAsFallback(out, after, spec.GainDB)) {
			_ = os.Remove(out)
		}

		if attempt == maxOptimizeAttempts {
			res.Rejected = rejection(after, expectedLUFS, ceiling, loudnessOK, peakOK)
			break
		}
		if !peakOK && !spec.LimitTruePeak {
			// Peaks came out over the ceiling on a transform that does not
			// limit. Reducing the gain would break the loudness it was chosen
			// for, so there is nothing further to try.
			res.Rejected = fmt.Sprintf("true peak %.2f dBTP exceeds the %.2f dBTP ceiling",
				after.TruePeak, ceiling)
			break
		}
		if !loudnessOK && math.Abs(miss) > maxDriftCorrectionDB {
			res.Rejected = rejection(after, expectedLUFS, ceiling, loudnessOK, peakOK)
			break
		}

		// Feed the measured misses back in and try again.
		if !peakOK {
			spec.CeilingDB -= peakOver + truePeakToleranceDB
		}
		if !loudnessOK {
			spec.GainDB = math.Min(spec.GainDB+miss, maxGain)
		}
	}

	// The ceiling was not reachable. On a degraded source that is a property of
	// the codec rather than of the music: re-encoding pushes the peak back up,
	// and clamping harder only makes it worse. Shipping the best attempt - on
	// target, still unable to clip - beats leaving the track short of target,
	// and nothing about the audio differs between the two.
	if accepted == "" && fallbackPath != "" {
		accepted, fallbackPath = fallbackPath, ""
		res.AfterSet = fallbackAfter
		res.NewLUFS = fallbackAfter.LUFS
		res.GainDB = fallbackGain
		res.CeilingRelaxed = true
		res.Rejected = ""
		log.Debug(ctx, "Loudness: ceiling unreachable, accepted against the fallback",
			"path", trackPath, "truePeak", fallbackAfter.TruePeak,
			"ceiling", ceiling, "fallbackCeiling", fallbackCeiling)
	}

	if accepted == "" {
		return res, nil
	}

	if opts.Backup {
		// Whether the original was already stored decides, later, which
		// measurement may be used as the "before" snapshot: an existing backup
		// is never overwritten, so on a second pass it predates this run.
		stored := ffmpeg.FindLoudnessBackup(opts.BackupFolder, opts.LibraryPath, opts.MediaFileID, trackPath)
		if err := ffmpeg.BackupOriginal(trackPath, stat.Mode(), opts.LibraryPath, opts.BackupFolder, opts.MediaFileID); err != nil {
			return res, fmt.Errorf("creating backup: %w", err)
		}
		res.BackupCreated = stored == ""
	}
	if err := os.Chmod(accepted, stat.Mode()); err != nil {
		return res, err
	}
	if err := os.Rename(accepted, trackPath); err != nil {
		return res, err
	}
	accepted = "" // consumed
	res.Changed = true
	log.Debug(ctx, "Loudness optimised", "path", trackPath, "from", res.OldLUFS, "to", res.NewLUFS,
		"gain", res.GainDB, "phase", res.Phase, "decision", decision)
	return res, nil
}

// acceptableAsFallback reports whether a produced file may ship against the
// fallback ceiling rather than the configured one.
//
// The bound is hard: no measurement slack is added, unlike at the configured
// ceiling. This is the last line before the reserve above the peak is gone, and
// the slack has already been spent reaching it. Adding it again here would let
// files ship 0.1 dB higher than the fallback promises.
func acceptableAsFallback(truePeak, fallbackCeiling float64) bool {
	return truePeak <= fallbackCeiling
}

// rejection explains, in the audit record, why a produced file was thrown away.
func rejection(after *Measurement, expectedLUFS, ceiling float64, loudnessOK, peakOK bool) string {
	switch {
	case !loudnessOK && !peakOK:
		return fmt.Sprintf("landed at %.2f LUFS (expected %.2f) and %.2f dBTP (ceiling %.2f)",
			after.LUFS, expectedLUFS, after.TruePeak, ceiling)
	case !peakOK:
		return fmt.Sprintf("true peak %.2f dBTP exceeds the %.2f dBTP ceiling",
			after.TruePeak, ceiling)
	default:
		return fmt.Sprintf("landed at %.2f LUFS, expected %.2f", after.LUFS, expectedLUFS)
	}
}

// reservedTempPath reserves a unique name in dir and removes the file so
// ffmpeg can create it itself.
func reservedTempPath(dir, pattern string) (string, error) {
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
