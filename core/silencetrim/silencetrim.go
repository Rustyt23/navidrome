package silencetrim

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/filelock"
)

const (
	MinimumSilenceSeconds           = 0.50
	RetainedPaddingSeconds          = 0.25
	QuietThresholdDB                = -70.0
	MaximumAutomaticTrimSeconds     = 5.0
	MinimumRemainingDurationSeconds = 1.0
)

type Settings struct {
	MinimumSilence       float64 `json:"minimumSilence"`
	RetainedPadding      float64 `json:"retainedPadding"`
	QuietThresholdDB     float64 `json:"quietThresholdDB"`
	MaximumAutomaticTrim float64 `json:"maximumAutomaticTrim"`
}

func CurrentSettings() Settings {
	return Settings{
		MinimumSilence:       MinimumSilenceSeconds,
		RetainedPadding:      RetainedPaddingSeconds,
		QuietThresholdDB:     QuietThresholdDB,
		MaximumAutomaticTrim: MaximumAutomaticTrimSeconds,
	}
}

type ApplyResult struct {
	Audit      *model.SilenceTrimAudit
	Changed    bool
	Reconciled bool
}

func Analyze(ctx context.Context, mf *model.MediaFile, trackPath string) *model.SilenceTrimAudit {
	audit := &model.SilenceTrimAudit{
		MediaFileID:     mf.ID,
		Status:          model.SilenceTrimStatusAnalyzed,
		Decision:        model.SilenceTrimDecisionPending,
		Integrity:       model.SilenceTrimIntegrityPending,
		RetainedPadding: RetainedPaddingSeconds,
		AnalyzedAt:      time.Now(),
	}

	linkStat, err := os.Lstat(trackPath)
	if err != nil {
		return failedAudit(audit, err)
	}
	if linkStat.Mode()&os.ModeSymlink != 0 {
		return blockAudit(audit, "Symbolic-link audio paths require manual trimming")
	}
	stat, err := os.Stat(trackPath)
	if err != nil {
		return failedAudit(audit, err)
	}
	modified := stat.ModTime()
	audit.SourceModifiedAt = &modified
	sourceHash, err := ffmpeg.FileSHA256(trackPath)
	if err != nil {
		return failedAudit(audit, fmt.Errorf("hashing source before analysis: %w", err))
	}
	audit.SourceSHA256 = sourceHash

	report, err := ffmpeg.DetectEdgeSilence(
		ctx,
		trackPath,
		MinimumSilenceSeconds,
		QuietThresholdDB,
	)
	if err != nil {
		return failedAudit(audit, err)
	}
	afterStat, err := os.Stat(trackPath)
	if err != nil {
		return failedAudit(audit, err)
	}
	afterHash, err := ffmpeg.FileSHA256(trackPath)
	if err != nil {
		return failedAudit(audit, fmt.Errorf("hashing source after analysis: %w", err))
	}
	if afterStat.Size() != stat.Size() ||
		!afterStat.ModTime().Equal(stat.ModTime()) ||
		!strings.EqualFold(afterHash, sourceHash) {
		return failedAudit(audit, fmt.Errorf("the song changed while it was being analyzed"))
	}
	recordBefore(audit, report.Probe)

	method, methodErr := ffmpeg.SilenceTrimMethod(report.Probe)
	audit.Method = method
	if report.Probe.SampleRate > 0 {
		audit.RetainedPaddingSamples = int64(math.Round(
			RetainedPaddingSeconds * float64(report.Probe.SampleRate),
		))
	}

	audit.LeadingKind, audit.LeadingSilence = classifyEdge(report.Leading)
	audit.TrailingKind, audit.TrailingSilence = classifyEdge(report.Trailing)
	audit.LeadingSamples = secondsToSamples(audit.LeadingSilence, report.Probe.SampleRate)
	audit.TrailingSamples = secondsToSamples(audit.TrailingSilence, report.Probe.SampleRate)
	audit.ProposedStartSamples = proposedSamples(audit.LeadingSamples, audit.RetainedPaddingSamples)
	audit.ProposedEndSamples = proposedSamples(audit.TrailingSamples, audit.RetainedPaddingSamples)
	audit.ProposedStartTrim = samplesToSeconds(audit.ProposedStartSamples, report.Probe.SampleRate)
	audit.ProposedEndTrim = samplesToSeconds(audit.ProposedEndSamples, report.Probe.SampleRate)

	if audit.ProposedStartSamples == 0 && audit.ProposedEndSamples == 0 {
		audit.Classification = model.SilenceTrimClassNone
		audit.Reason = fmt.Sprintf(
			"No start/end blank of at least %.0f ms was found; internal pauses were ignored",
			MinimumSilenceSeconds*1000,
		)
		audit.Method = ""
		return audit
	}

	if report.Probe.AudioStreams != 1 {
		return blockAudit(audit, fmt.Sprintf(
			"Found %d audio streams; trimming could drop or desynchronise one",
			report.Probe.AudioStreams,
		))
	}
	if report.Probe.NonAttachedVideoStreams > 0 {
		return blockAudit(audit, "The file contains a real video stream, not only cover art")
	}
	if report.Probe.AttachedPicStreams > 1 {
		return blockAudit(audit, "The file contains multiple embedded pictures that cannot all be preserved safely")
	}
	if report.Probe.OtherStreams > 0 {
		return blockAudit(audit, "The file contains subtitle, data, attachment, or unknown streams")
	}
	if report.Probe.ChapterCount > 0 {
		return blockAudit(audit, "The file contains chapters whose timestamps would need to be rewritten")
	}
	if audit.ProposedStartSamples > 0 && hasTimedLyrics(mf) {
		return blockAudit(audit, "The song has synced lyrics; a leading cut would move their timestamps")
	}
	if audit.ProposedStartSamples > 0 && hasTimedSidecar(trackPath) {
		return blockAudit(audit, "A timed .lrc or .cue sidecar exists; a leading cut would move its timestamps")
	}
	if audit.ProposedStartSamples > 0 && (mf.BookmarkPosition > 0 || mf.HasAnyBookmark) {
		return blockAudit(audit, "The song has a saved playback position that a leading cut would invalidate")
	}
	if methodErr != nil {
		return blockAudit(audit, methodErr.Error())
	}

	remaining := report.Probe.Duration - audit.ProposedStartTrim - audit.ProposedEndTrim
	if remaining < MinimumRemainingDurationSeconds {
		return blockAudit(audit, "The proposal would leave too little audio to be a valid song")
	}

	needsReview := audit.LeadingKind == model.SilenceTrimEdgeQuiet ||
		audit.TrailingKind == model.SilenceTrimEdgeQuiet ||
		audit.LeadingSilence > MaximumAutomaticTrimSeconds ||
		audit.TrailingSilence > MaximumAutomaticTrimSeconds
	if needsReview {
		audit.Classification = model.SilenceTrimClassReview
		audit.Reason = "Possible fade, reverb, room tone, or unusually long blank detected; listen and approve before trimming"
		return audit
	}

	audit.Classification = model.SilenceTrimClassSafe
	if method == ffmpeg.SilenceTrimMethodPacketCopy {
		audit.Reason = "Confirmed codec silence; MP3/AAC packets will be copied without re-encoding"
	} else {
		audit.Reason = "Confirmed digital silence; retained lossless samples will be verified byte-for-byte"
	}
	return audit
}

func Apply(
	ctx context.Context,
	mf *model.MediaFile,
	trackPath string,
	stored *model.SilenceTrimAudit,
	backupFolder string,
) (ApplyResult, error) {
	unlock := filelock.Lock(trackPath)
	defer unlock()

	recovered, generationState, generationErr := ReconcileGeneration(
		trackPath,
		mf.LibraryPath,
		backupFolder,
		mf.ID,
	)
	if generationState == GenerationStateResult {
		alreadyPersisted := stored != nil &&
			stored.Status == model.SilenceTrimStatusProcessed &&
			strings.EqualFold(stored.ResultSHA256, recovered.ResultSHA256)
		return ApplyResult{
			Audit:      recovered,
			Changed:    !alreadyPersisted,
			Reconciled: true,
		}, nil
	}
	if generationState == GenerationStateSource {
		source := *recovered
		source.Status = model.SilenceTrimStatusAnalyzed
		source.Integrity = model.SilenceTrimIntegrityPending
		source.ResultSHA256 = ""
		source.ResultModifiedAt = nil
		source.AppliedAt = nil
		source.Decision = model.SilenceTrimDecisionPending
		if stored != nil &&
			stored.Status == model.SilenceTrimStatusAnalyzed &&
			SameProposal(stored, recovered) {
			source.Decision = stored.Decision
		}
		stored = &source
	}
	if generationErr != nil &&
		!CanIgnoreGenerationError(generationErr, stored, trackPath) {
		stale := model.SilenceTrimAudit{
			MediaFileID: mf.ID,
			AnalyzedAt:  time.Now(),
		}
		if stored != nil {
			stale = *stored
		}
		stale.Status = model.SilenceTrimStatusFailed
		stale.Integrity = model.SilenceTrimIntegrityFailed
		stale.Error = generationErr.Error()
		stale.Reason = "Refused: the current song could not be matched to its durable trim generation"
		return ApplyResult{Audit: &stale}, nil
	}

	// A processed audit is the proof and restore pointer for the file that is
	// already on disk. Re-running Apply must not replace that history with a
	// fresh "no silence found" row. Restore deliberately changes the status
	// back to analyzed when the pre-trim file is put back.
	if stored != nil && stored.ResultSHA256 != "" && stored.HasBackup {
		if ResultFileUnchanged(stored, trackPath) {
			current := *stored
			current.Status = model.SilenceTrimStatusProcessed
			current.Integrity = model.SilenceTrimIntegrityVerified
			current.Error = ""
			return ApplyResult{Audit: &current}, nil
		}
		stale := *stored
		stale.Status = model.SilenceTrimStatusFailed
		stale.Integrity = model.SilenceTrimIntegrityFailed
		stale.Error = "the current file no longer matches the verified silence-trim result"
		stale.Reason = "Refused: restore or resolve later audio processing before trimming this song again"
		return ApplyResult{Audit: &stale}, nil
	}

	fresh := Analyze(ctx, mf, trackPath)
	if fresh.Status == model.SilenceTrimStatusFailed {
		return ApplyResult{Audit: fresh}, fmt.Errorf("%s", fresh.Error)
	}

	decision := model.SilenceTrimDecisionPending
	if stored != nil {
		decision = stored.Decision
	}
	fresh.Decision = decision

	if decision == model.SilenceTrimDecisionSkip {
		fresh.Reason = "Left unchanged by decision"
		return ApplyResult{Audit: fresh}, nil
	}
	if fresh.Classification == model.SilenceTrimClassNone ||
		fresh.Classification == model.SilenceTrimClassBlocked {
		return ApplyResult{Audit: fresh}, nil
	}
	if fresh.Classification == model.SilenceTrimClassReview {
		if decision != model.SilenceTrimDecisionApprove {
			return ApplyResult{Audit: fresh}, nil
		}
		if proposalChanged(stored, fresh) {
			fresh.Decision = model.SilenceTrimDecisionPending
			fresh.Reason = "The file or proposal changed after approval; review the new proposal"
			return ApplyResult{Audit: fresh}, nil
		}
	}

	useGenerationBackup, err := verifyExistingBackupState(
		trackPath,
		mf.LibraryPath,
		backupFolder,
		mf.ID,
		stored,
	)
	if err != nil {
		return ApplyResult{Audit: blockAudit(fresh, err.Error())}, nil
	}

	stat, err := os.Stat(trackPath)
	if err != nil {
		return ApplyResult{Audit: failedAudit(fresh, err)}, err
	}
	dir := filepath.Dir(trackPath)
	base := filepath.Base(trackPath)
	ext := filepath.Ext(base)
	candidate, err := reserveTempPath(dir, "."+base+".silence-trim-*"+ext)
	if err != nil {
		return ApplyResult{Audit: failedAudit(fresh, err)}, err
	}
	defer os.Remove(candidate)

	method, err := ffmpeg.WriteSilenceTrimCandidate(
		ctx,
		trackPath,
		candidate,
		probeFromAudit(fresh),
		fresh.ProposedStartSamples,
		fresh.ProposedEndSamples,
	)
	if err != nil {
		return ApplyResult{Audit: failedAudit(fresh, err)}, err
	}

	after, err := ffmpeg.ProbeFile(ctx, candidate)
	if err != nil {
		err = fmt.Errorf("probing proposed result: %w", err)
		return ApplyResult{Audit: failedAudit(fresh, err)}, err
	}
	if err := verifyContainer(fresh, after, method); err != nil {
		return ApplyResult{Audit: failedIntegrity(fresh, err)}, nil
	}
	sourceProbe := probeFromAudit(fresh)
	switch method {
	case ffmpeg.SilenceTrimMethodLossless:
		err = ffmpeg.VerifyLosslessRetainedPCM(
			ctx,
			trackPath,
			candidate,
			sourceProbe,
			fresh.ProposedStartSamples,
			fresh.ProposedEndSamples,
		)
	case ffmpeg.SilenceTrimMethodPacketCopy:
		err = ffmpeg.VerifyCopiedPackets(
			ctx,
			trackPath,
			candidate,
			sourceProbe,
			fresh.ProposedStartSamples,
			fresh.ProposedEndSamples,
		)
	}
	if err != nil {
		return ApplyResult{Audit: failedIntegrity(fresh, err)}, nil
	}

	candidateEdges, err := ffmpeg.DetectEdgeSilence(ctx, candidate, 0.05, QuietThresholdDB)
	if err != nil {
		err = fmt.Errorf("checking retained edge padding: %w", err)
		return ApplyResult{Audit: failedIntegrity(fresh, err)}, nil
	}
	if err := verifyRetainedPadding(fresh, candidateEdges, method); err != nil {
		return ApplyResult{Audit: failedIntegrity(fresh, err)}, nil
	}
	resultHash, err := ffmpeg.FileSHA256(candidate)
	if err != nil {
		return ApplyResult{Audit: failedIntegrity(fresh, fmt.Errorf("hashing verified candidate: %w", err))}, nil
	}

	var backupHash string
	if useGenerationBackup {
		backupHash, err = ffmpeg.RotateSilenceTrimOriginal(
			trackPath,
			stat.Mode(),
			mf.LibraryPath,
			backupFolder,
			mf.ID,
			stored.BackupSHA256,
		)
	} else {
		backupHash, err = ffmpeg.BackupSilenceTrimOriginal(
			trackPath,
			stat.Mode(),
			mf.LibraryPath,
			backupFolder,
			mf.ID,
		)
	}
	if err != nil {
		err = fmt.Errorf("creating verified pre-trim backup: %w", err)
		return ApplyResult{Audit: failedAudit(fresh, err)}, err
	}
	if !strings.EqualFold(backupHash, fresh.SourceSHA256) {
		err = fmt.Errorf("the verified backup does not match the analyzed source generation")
		return ApplyResult{Audit: failedAudit(fresh, err)}, err
	}
	currentSourceHash, err := ffmpeg.FileSHA256(trackPath)
	if err != nil || !strings.EqualFold(currentSourceHash, fresh.SourceSHA256) {
		if err == nil {
			err = fmt.Errorf("the song changed after candidate verification")
		}
		return ApplyResult{Audit: failedAudit(fresh, err)}, err
	}
	if err := os.Chmod(candidate, stat.Mode()); err != nil {
		return ApplyResult{Audit: failedAudit(fresh, err)}, err
	}

	candidateStat, err := os.Stat(candidate)
	if err != nil {
		return ApplyResult{Audit: failedAudit(fresh, err)}, err
	}
	modified := candidateStat.ModTime()
	fresh.ResultModifiedAt = &modified
	now := time.Now()
	fresh.Status = model.SilenceTrimStatusProcessed
	fresh.Integrity = model.SilenceTrimIntegrityVerified
	fresh.HasBackup = true
	fresh.BackupSHA256 = backupHash
	fresh.ResultSHA256 = resultHash
	fresh.AppliedAt = &now
	recordAfter(fresh, after)
	fresh.AppliedStartSamples = actualTrimSamples(
		fresh.LeadingSamples,
		edgeSamples(candidateEdges.Leading, fresh.LeadingKind, after.SampleRate),
	)
	fresh.AppliedEndSamples = actualTrimSamples(
		fresh.TrailingSamples,
		edgeSamples(candidateEdges.Trailing, fresh.TrailingKind, after.SampleRate),
	)
	fresh.AppliedStartTrim = samplesToSeconds(fresh.AppliedStartSamples, after.SampleRate)
	fresh.AppliedEndTrim = samplesToSeconds(fresh.AppliedEndSamples, after.SampleRate)
	fresh.Reason = "Trimmed and verified; the pre-trim file is stored for exact restore"
	if err := persistGenerationRecord(backupFolder, mf.LibraryPath, fresh); err != nil {
		err = fmt.Errorf("storing durable silence-trim generation: %w", err)
		return ApplyResult{Audit: failedBeforeInstall(fresh, err)}, err
	}
	if err := os.Rename(candidate, trackPath); err != nil {
		return ApplyResult{Audit: failedAudit(fresh, err)}, err
	}
	if err := syncDirectory(filepath.Dir(trackPath)); err != nil {
		rollbackErr := ffmpeg.RestoreSilenceTrimOriginal(
			trackPath,
			mf.LibraryPath,
			backupFolder,
			mf.ID,
			backupHash,
			resultHash,
		)
		if rollbackErr != nil {
			err = fmt.Errorf("%w; restoring after directory sync failed: %v", err, rollbackErr)
		}
		return ApplyResult{Audit: failedAudit(fresh, err)}, err
	}
	onDiskHash, err := ffmpeg.FileSHA256(trackPath)
	if err != nil || !strings.EqualFold(onDiskHash, resultHash) {
		if err == nil {
			err = fmt.Errorf("the installed result does not match the verified candidate")
		}
		// The bytes no longer match the candidate we installed. They may have
		// been replaced by another process, so they must never authorize their
		// own overwrite during rollback.
		return ApplyResult{Audit: failedAudit(fresh, err)}, err
	}
	resultStat, err := os.Stat(trackPath)
	if err != nil {
		_ = ffmpeg.RestoreSilenceTrimOriginal(
			trackPath,
			mf.LibraryPath,
			backupFolder,
			mf.ID,
			backupHash,
			resultHash,
		)
		return ApplyResult{Audit: failedAudit(fresh, err)}, err
	}
	modified = resultStat.ModTime()
	fresh.ResultModifiedAt = &modified

	return ApplyResult{Audit: fresh, Changed: true}, nil
}

func Restore(
	ctx context.Context,
	mf *model.MediaFile,
	trackPath string,
	previous *model.SilenceTrimAudit,
	backupFolder string,
) (*model.SilenceTrimAudit, error) {
	unlock := filelock.Lock(trackPath)
	defer unlock()
	return RestoreLocked(ctx, mf, trackPath, previous, backupFolder)
}

// RestoreLocked performs the verified restore while the caller holds the
// track's file lock. It lets the API keep that same lock through its database
// commit or conditional rollback, closing the snapshot/restore race.
func RestoreLocked(
	ctx context.Context,
	mf *model.MediaFile,
	trackPath string,
	previous *model.SilenceTrimAudit,
	backupFolder string,
) (*model.SilenceTrimAudit, error) {
	if previous == nil ||
		!previous.HasBackup ||
		previous.BackupSHA256 == "" ||
		previous.SourceSHA256 == "" ||
		previous.ResultSHA256 == "" {
		return nil, fmt.Errorf("no complete silence-trim generation record is available for restore")
	}
	currentHash, err := ffmpeg.FileSHA256(trackPath)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(currentHash, previous.ResultSHA256) {
		return nil, fmt.Errorf("restore refused because the song changed after silence trimming")
	}
	if !strings.EqualFold(previous.SourceSHA256, previous.BackupSHA256) {
		return nil, fmt.Errorf("the recorded source and backup generations do not match")
	}
	backupPath := ffmpeg.FindSilenceTrimBackupGeneration(
		backupFolder,
		mf.LibraryPath,
		mf.ID,
		previous.BackupSHA256,
	)
	if backupPath == "" {
		return nil, fmt.Errorf("no stored pre-trim original")
	}
	backupProbe, err := ffmpeg.ProbeFile(ctx, backupPath)
	if err != nil {
		return nil, fmt.Errorf("probing pre-trim original before restore: %w", err)
	}
	if err := ffmpeg.RestoreSilenceTrimOriginal(
		trackPath,
		mf.LibraryPath,
		backupFolder,
		mf.ID,
		previous.BackupSHA256,
		previous.ResultSHA256,
	); err != nil {
		return nil, err
	}
	actualHash, err := ffmpeg.FileSHA256(trackPath)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(actualHash, previous.BackupSHA256) {
		return nil, fmt.Errorf("restored file failed its SHA-256 check")
	}

	audit := Analyze(ctx, mf, trackPath)
	audit.Decision = model.SilenceTrimDecisionPending
	audit.HasBackup = true
	audit.BackupSHA256 = previous.BackupSHA256
	audit.SourceSHA256 = previous.BackupSHA256
	if audit.Status == model.SilenceTrimStatusFailed {
		// The exact bytes are already restored. Keep correct media metadata
		// from the successful preflight probe rather than returning zeroes.
		recordBefore(audit, backupProbe)
		audit.Reason = "Exact pre-trim file restored; refreshing its edge analysis failed"
		return audit, nil
	}
	audit.Reason = "Exact pre-trim file restored; analysis refreshed"
	return audit, nil
}

// AnalyzeRecoveredSource reconciles a source file that a restore already put
// back before the database update completed. If decoding is temporarily
// unavailable, the durable generation still supplies trusted source metadata
// so the media row is never zeroed.
func AnalyzeRecoveredSource(
	ctx context.Context,
	mf *model.MediaFile,
	trackPath string,
	generation *model.SilenceTrimAudit,
) *model.SilenceTrimAudit {
	audit := Analyze(ctx, mf, trackPath)
	audit.Decision = model.SilenceTrimDecisionPending
	if generation == nil {
		return audit
	}
	audit.HasBackup = true
	audit.BackupSHA256 = generation.BackupSHA256
	audit.SourceSHA256 = generation.BackupSHA256
	if audit.Status == model.SilenceTrimStatusFailed {
		audit.CodecBefore = generation.CodecBefore
		audit.BitrateBefore = generation.BitrateBefore
		audit.SampleRateBefore = generation.SampleRateBefore
		audit.BitDepthBefore = generation.BitDepthBefore
		audit.ChannelsBefore = generation.ChannelsBefore
		audit.DurationBefore = generation.DurationBefore
		audit.SizeBefore = generation.SizeBefore
		audit.ArtBefore = generation.ArtBefore
	}
	return audit
}

func classifyEdge(edge ffmpeg.EdgeSilence) (string, float64) {
	if edge.Strict >= MinimumSilenceSeconds {
		return model.SilenceTrimEdgeExact, edge.Strict
	}
	if edge.Quiet >= MinimumSilenceSeconds {
		return model.SilenceTrimEdgeQuiet, edge.Quiet
	}
	return model.SilenceTrimEdgeNone, 0
}

func proposedSamples(silence, padding int64) int64 {
	if silence <= padding {
		return 0
	}
	return silence - padding
}

func secondsToSamples(seconds float64, sampleRate int) int64 {
	if seconds <= 0 || sampleRate <= 0 {
		return 0
	}
	return int64(math.Round(seconds * float64(sampleRate)))
}

func samplesToSeconds(samples int64, sampleRate int) float64 {
	if samples <= 0 || sampleRate <= 0 {
		return 0
	}
	return float64(samples) / float64(sampleRate)
}

func recordBefore(audit *model.SilenceTrimAudit, probe *ffmpeg.FileProbe) {
	audit.CodecBefore = probe.Codec
	audit.BitrateBefore = probe.BitRate
	audit.SampleRateBefore = probe.SampleRate
	audit.BitDepthBefore = probe.BitDepth
	audit.ChannelsBefore = probe.Channels
	audit.DurationBefore = probe.Duration
	audit.SizeBefore = probe.Size
	audit.ArtBefore = probe.HasArt
}

func recordAfter(audit *model.SilenceTrimAudit, probe *ffmpeg.FileProbe) {
	audit.CodecAfter = probe.Codec
	audit.BitrateAfter = probe.BitRate
	audit.SampleRateAfter = probe.SampleRate
	audit.BitDepthAfter = probe.BitDepth
	audit.ChannelsAfter = probe.Channels
	audit.DurationAfter = probe.Duration
	audit.SizeAfter = probe.Size
	audit.ArtAfter = probe.HasArt
}

func probeFromAudit(audit *model.SilenceTrimAudit) *ffmpeg.FileProbe {
	probe := &ffmpeg.FileProbe{
		Codec:        audit.CodecBefore,
		BitRate:      audit.BitrateBefore,
		SampleRate:   audit.SampleRateBefore,
		BitDepth:     audit.BitDepthBefore,
		Channels:     audit.ChannelsBefore,
		Duration:     audit.DurationBefore,
		Size:         audit.SizeBefore,
		HasArt:       audit.ArtBefore,
		AudioStreams: 1,
	}
	if audit.ArtBefore {
		probe.AttachedPicStreams = 1
	}
	return probe
}

func verifyContainer(audit *model.SilenceTrimAudit, after *ffmpeg.FileProbe, method string) error {
	var issues []string
	if after.Codec != audit.CodecBefore {
		issues = append(issues, fmt.Sprintf("codec %s -> %s", audit.CodecBefore, after.Codec))
	}
	if after.SampleRate != audit.SampleRateBefore {
		issues = append(issues, fmt.Sprintf("sample rate %d -> %d", audit.SampleRateBefore, after.SampleRate))
	}
	if after.BitDepth != audit.BitDepthBefore {
		issues = append(issues, fmt.Sprintf("bit depth %d -> %d", audit.BitDepthBefore, after.BitDepth))
	}
	if after.Channels != audit.ChannelsBefore {
		issues = append(issues, fmt.Sprintf("channels %d -> %d", audit.ChannelsBefore, after.Channels))
	}
	if after.HasArt != audit.ArtBefore {
		issues = append(issues, "embedded cover art changed")
	}
	expectedPictures := 0
	if audit.ArtBefore {
		expectedPictures = 1
	}
	if after.AudioStreams != 1 ||
		after.AttachedPicStreams != expectedPictures ||
		after.NonAttachedVideoStreams > 0 ||
		after.OtherStreams > 0 {
		issues = append(issues, "stream topology changed")
	}

	expected := audit.DurationBefore - audit.ProposedStartTrim - audit.ProposedEndTrim
	tolerance := 0.02
	if method == ffmpeg.SilenceTrimMethodPacketCopy {
		tolerance = 0.15
	}
	if math.Abs(after.Duration-expected) > tolerance {
		issues = append(issues, fmt.Sprintf(
			"duration %.3fs, expected %.3fs (tolerance %.3fs)",
			after.Duration,
			expected,
			tolerance,
		))
	}
	if after.Duration >= audit.DurationBefore-0.01 {
		issues = append(issues, "the candidate was not shortened")
	}
	if len(issues) > 0 {
		return fmt.Errorf("candidate failed integrity checks: %s", strings.Join(issues, "; "))
	}
	return nil
}

func verifyRetainedPadding(
	audit *model.SilenceTrimAudit,
	report *ffmpeg.EdgeSilenceReport,
	method string,
) error {
	tolerance := 0.02
	if method == ffmpeg.SilenceTrimMethodPacketCopy {
		tolerance = 0.08
	}
	minimum := math.Max(0.05, RetainedPaddingSeconds-tolerance)
	maximum := MinimumSilenceSeconds + tolerance

	check := func(label, kind string, proposed int64, edge ffmpeg.EdgeSilence) error {
		if proposed == 0 {
			return nil
		}
		retained := edge.Strict
		if kind == model.SilenceTrimEdgeQuiet {
			retained = edge.Quiet
		}
		if retained < minimum {
			return fmt.Errorf("%s padding is %.3fs; at least %.3fs must remain", label, retained, minimum)
		}
		if retained > maximum {
			return fmt.Errorf("%s still contains %.3fs of blank; the cut was not applied accurately", label, retained)
		}
		return nil
	}
	if err := check("start", audit.LeadingKind, audit.ProposedStartSamples, report.Leading); err != nil {
		return err
	}
	return check("end", audit.TrailingKind, audit.ProposedEndSamples, report.Trailing)
}

func edgeSamples(edge ffmpeg.EdgeSilence, kind string, sampleRate int) int64 {
	seconds := edge.Strict
	if kind == model.SilenceTrimEdgeQuiet {
		seconds = edge.Quiet
	}
	return secondsToSamples(seconds, sampleRate)
}

func actualTrimSamples(before, retained int64) int64 {
	if before <= retained {
		return 0
	}
	return before - retained
}

func failedAudit(audit *model.SilenceTrimAudit, err error) *model.SilenceTrimAudit {
	audit.Status = model.SilenceTrimStatusFailed
	audit.Integrity = model.SilenceTrimIntegrityFailed
	audit.Error = err.Error()
	return audit
}

func failedIntegrity(audit *model.SilenceTrimAudit, err error) *model.SilenceTrimAudit {
	audit.Status = model.SilenceTrimStatusFailed
	audit.Integrity = model.SilenceTrimIntegrityFailed
	audit.Error = err.Error()
	audit.Reason = "Candidate rejected; the original file was not touched"
	return audit
}

func failedBeforeInstall(audit *model.SilenceTrimAudit, err error) *model.SilenceTrimAudit {
	failedAudit(audit, err)
	audit.ResultSHA256 = ""
	audit.ResultModifiedAt = nil
	audit.AppliedAt = nil
	audit.AppliedStartTrim = 0
	audit.AppliedEndTrim = 0
	audit.AppliedStartSamples = 0
	audit.AppliedEndSamples = 0
	audit.Reason = "The verified candidate was not installed; the original file was not touched"
	return audit
}

func blockAudit(audit *model.SilenceTrimAudit, reason string) *model.SilenceTrimAudit {
	audit.Classification = model.SilenceTrimClassBlocked
	audit.Reason = reason
	return audit
}

func proposalChanged(old, fresh *model.SilenceTrimAudit) bool {
	if old == nil {
		return true
	}
	if old.SizeBefore != fresh.SizeBefore ||
		old.SourceSHA256 == "" ||
		fresh.SourceSHA256 == "" ||
		!strings.EqualFold(old.SourceSHA256, fresh.SourceSHA256) ||
		old.SampleRateBefore != fresh.SampleRateBefore ||
		old.ProposedStartSamples != fresh.ProposedStartSamples ||
		old.ProposedEndSamples != fresh.ProposedEndSamples {
		return true
	}
	if old.SourceModifiedAt == nil || fresh.SourceModifiedAt == nil {
		return true
	}
	delta := old.SourceModifiedAt.Sub(*fresh.SourceModifiedAt)
	if delta < 0 {
		delta = -delta
	}
	return delta > time.Second
}

// SameProposal ties a human approval to the exact file generation and sample
// boundaries they reviewed.
func SameProposal(old, fresh *model.SilenceTrimAudit) bool {
	return !proposalChanged(old, fresh)
}

// ProcessedFileUnchanged protects the accepted before/after proof from being
// erased by a routine "Analyze library" pass.
func ProcessedFileUnchanged(audit *model.SilenceTrimAudit, trackPath string) bool {
	if audit == nil ||
		audit.Status != model.SilenceTrimStatusProcessed {
		return false
	}
	return ResultFileUnchanged(audit, trackPath)
}

// ResultFileUnchanged proves the current bytes are the accepted output tied to
// this audit. Size is only a fast rejection; SHA-256 is authoritative.
func ResultFileUnchanged(audit *model.SilenceTrimAudit, trackPath string) bool {
	if audit == nil ||
		audit.ResultSHA256 == "" {
		return false
	}
	stat, err := os.Stat(trackPath)
	if err != nil {
		return false
	}
	if stat.Size() != audit.SizeAfter {
		return false
	}
	hash, err := ffmpeg.FileSHA256(trackPath)
	return err == nil && strings.EqualFold(hash, audit.ResultSHA256)
}

func verifyExistingBackupState(
	trackPath string,
	libraryPath string,
	backupFolder string,
	mediaFileID string,
	stored *model.SilenceTrimAudit,
) (bool, error) {
	var backupHash string
	var err error
	if stored != nil && stored.BackupSHA256 != "" {
		if ffmpeg.FindSilenceTrimBackupGeneration(
			backupFolder,
			libraryPath,
			mediaFileID,
			stored.BackupSHA256,
		) == "" {
			return false, nil
		}
		_, backupHash, err = ffmpeg.VerifySilenceTrimBackupGeneration(
			backupFolder,
			libraryPath,
			mediaFileID,
			stored.BackupSHA256,
		)
	} else {
		if ffmpeg.FindSilenceTrimBackup(backupFolder, libraryPath, mediaFileID) == "" {
			return false, nil
		}
		backupHash, err = ffmpeg.VerifySilenceTrimBackup(
			backupFolder,
			libraryPath,
			mediaFileID,
		)
	}
	if err != nil {
		return false, fmt.Errorf("the existing pre-trim backup cannot be verified: %w", err)
	}
	if stored != nil &&
		stored.BackupSHA256 != "" &&
		!strings.EqualFold(backupHash, stored.BackupSHA256) {
		return false, fmt.Errorf("the existing pre-trim backup no longer matches its recorded SHA-256")
	}
	currentHash, err := ffmpeg.FileSHA256(trackPath)
	if err != nil {
		return false, fmt.Errorf("the current song cannot be matched to its pre-trim backup: %w", err)
	}
	if !strings.EqualFold(currentHash, backupHash) {
		if stored != nil &&
			stored.HasBackup &&
			stored.BackupSHA256 != "" &&
			stored.ResultSHA256 == "" {
			return true, nil
		}
		return false, fmt.Errorf("a pre-trim backup already exists for a different file state; restore it before trimming again")
	}
	return stored != nil &&
		stored.HasBackup &&
		stored.BackupSHA256 != "", nil
}

func hasTimedLyrics(mf *model.MediaFile) bool {
	if mf == nil || strings.TrimSpace(mf.Lyrics) == "" {
		return false
	}
	var lyrics model.LyricList
	if err := json.Unmarshal([]byte(mf.Lyrics), &lyrics); err != nil {
		return true // malformed timing data is not safe to rewrite around
	}
	for _, lyric := range lyrics {
		if lyric.Synced {
			return true
		}
		for _, line := range lyric.Line {
			if line.Start != nil {
				return true
			}
		}
	}
	return false
}

func hasTimedSidecar(trackPath string) bool {
	dir := filepath.Dir(trackPath)
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(trackPath), filepath.Ext(trackPath)))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return true
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".cue" || (ext == ".lrc" && strings.HasPrefix(name, base)) {
			return true
		}
	}
	return false
}

func reserveTempPath(dir, pattern string) (string, error) {
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	if err := os.Remove(name); err != nil {
		return "", err
	}
	return name, nil
}
