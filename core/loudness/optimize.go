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
	// peakRetryHeadroomDB is extra clearance on a retry, never acceptance slack.
	peakRetryHeadroomDB = 0.1
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
	// BackupSkipped reports that no original was kept because none are being
	// kept, as opposed to none being made because one was already there. The
	// first means this run's measurements are the only record the track will
	// ever have; the second means an older original is on disk to read.
	//
	// Phrased as the exception rather than the rule so that the zero value means
	// "an original is expected", which is the conservative reading: a caller
	// that does not set it gets the path that goes and looks for the stored file.
	BackupSkipped bool
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

	res := OptimizeResult{Decision: decision, BackupSkipped: !opts.Backup}

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

	// A produced file whose peaks are correct but whose loudness landed a little
	// short of the target is kept aside.
	//
	// The strict window is +/-0.2, and a result outside it used to be deleted -
	// leaving the song at its ORIGINAL loudness, which was usually further from
	// the target than the file just thrown away. Measured on a real library: 24
	// tracks refused this way, and for 21 of them the discarded version was
	// closer to target than what was kept. One was left 1.95 dB out when the
	// deleted file was 0.25 dB out.
	//
	// The wider band is the same leaveAloneToleranceDB the rest of the system
	// already uses to mean "near enough that nobody would act on it". It was
	// simply never consulted here, because the file was in the bin before
	// anything asked.
	var nearPath string
	var nearAfter *Measurement
	var nearGain float64
	defer func() {
		if nearPath != "" {
			_ = os.Remove(nearPath)
		}
	}()
	keepAsNearMiss := func(path string, after *Measurement, gain float64) bool {
		offBy := math.Abs(after.LUFS - opts.Target.IntegratedLUFS)
		if offBy > leaveAloneToleranceDB {
			return false
		}
		// Closest to target wins, so extra attempts can only improve on it.
		if nearAfter != nil &&
			offBy >= math.Abs(nearAfter.LUFS-opts.Target.IntegratedLUFS) {
			return false
		}
		if nearPath != "" {
			_ = os.Remove(nearPath)
		}
		nearPath, nearAfter, nearGain = path, after, gain
		return true
	}

	// Without a limiter the only way to hold a ceiling is to not exceed it in
	// the first place, so the gain may never rise above the largest one that
	// keeps the peaks under it - however the retry feedback pushes.
	maxGain := math.Inf(1)
	if !spec.LimitTruePeak {
		maxGain = opts.Target.TruePeak - before.TruePeak
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
		peakOK := peakWithinCeiling(after.TruePeak, ceiling)

		if loudnessOK && peakOK {
			accepted = out
			res.AfterSet = after
			res.NewLUFS = after.LUFS
			res.GainDB = spec.GainDB
			break
		}

		// Only preserve near misses whose measured peaks satisfy the ceiling.
		if !peakOK || !keepAsNearMiss(out, after, spec.GainDB) {
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
			spec.CeilingDB -= peakOver + peakRetryHeadroomDB
		}
		if !loudnessOK {
			spec.GainDB = math.Min(spec.GainDB+miss, maxGain)
		}
	}

	// Peaks correct, loudness a little short of target. Shipping this beats
	// deleting it and leaving the song where it started, which was usually
	// further out than the file being deleted. It reports itself short on the
	// page - visible, and restorable - rather than vanishing into a refusal.
	if accepted == "" && nearPath != "" {
		accepted, nearPath = nearPath, ""
		res.AfterSet = nearAfter
		res.NewLUFS = nearAfter.LUFS
		res.GainDB = nearGain
		res.Rejected = ""
		log.Debug(ctx, "Loudness: kept a result short of target rather than discarding it",
			"path", trackPath, "landed", nearAfter.LUFS, "target", opts.Target.IntegratedLUFS)
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

// peakWithinCeiling requires a finite measurement at or below the configured
// ceiling. No fallback or measurement slack is allowed to raise this bound.
func peakWithinCeiling(truePeak, ceiling float64) bool {
	return !math.IsNaN(truePeak) && !math.IsInf(truePeak, 0) &&
		!math.IsNaN(ceiling) && !math.IsInf(ceiling, 0) && truePeak <= ceiling
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
