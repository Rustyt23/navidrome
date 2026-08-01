package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	coresilence "github.com/navidrome/navidrome/core/silence"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

const maxSilenceMutationSongs = 500

const maxSilenceTrimTimeout = 30 * time.Minute

const (
	silenceMutationTrim    = "trim"
	silenceMutationRestore = "restore"
)

func validateSilenceMargin(margin float64) error {
	if math.IsNaN(margin) || math.IsInf(margin, 0) || margin < 0 || margin > 2 {
		return errors.New("trim margin must be between 0 and 2 seconds")
	}
	return nil
}

type silenceMutationJob struct {
	running   atomic.Bool
	total     atomic.Int64
	processed atomic.Int64
	succeeded atomic.Int64
	skipped   atomic.Int64
	failed    atomic.Int64
	inFlight  atomic.Int64
	startedAt atomic.Int64
	stateMu   sync.RWMutex
	mode      string
	message   string
	margin    float64
	errors    []string
}

type silenceMutationPayload struct {
	IDs           []string `json:"ids"`
	MarginSeconds *float64 `json:"marginSeconds,omitempty"`
}

type silenceMutationStatus struct {
	Running       bool     `json:"running"`
	Mode          string   `json:"mode,omitempty"`
	StartedAt     string   `json:"startedAt,omitempty"`
	Total         int64    `json:"total"`
	Processed     int64    `json:"processed"`
	Succeeded     int64    `json:"succeeded"`
	Skipped       int64    `json:"skipped"`
	Failed        int64    `json:"failed"`
	InFlight      int64    `json:"inFlight"`
	Message       string   `json:"message,omitempty"`
	BackupFolder  string   `json:"backupFolder"`
	MarginSeconds float64  `json:"marginSeconds"`
	Errors        []string `json:"errors,omitempty"`
}

func newSilenceMutationJob() *silenceMutationJob {
	return &silenceMutationJob{margin: coresilence.TrimSafetyPad}
}

// newSilenceTrimmer keeps Silence Trim beside the LUFS backup root. With the
// default LUFS layout, a library at <root>/music uses <root>/original_backup
// before normalisation, so Silence Trim belongs at <root>/silence trim backup.
func newSilenceTrimmer(libraryPath string) *coresilence.Trimmer {
	loudnessRoot := ffmpeg.LoudnessBackupRoot(
		conf.Server.Scanner.LoudnessNormalization.BackupFolder,
		libraryPath,
	)
	if loudnessRoot != "" {
		return coresilence.NewTrimmer(filepath.Dir(loudnessRoot))
	}
	return coresilence.NewTrimmer(conf.Server.DataFolder.String())
}

func (j *silenceMutationJob) setState(mode, message string) {
	j.stateMu.Lock()
	j.mode = mode
	j.message = message
	j.stateMu.Unlock()
}

func (j *silenceMutationJob) begin(mode string, total int, margin float64) bool {
	if !j.running.CompareAndSwap(false, true) {
		return false
	}
	j.total.Store(int64(total))
	j.processed.Store(0)
	j.succeeded.Store(0)
	j.skipped.Store(0)
	j.failed.Store(0)
	j.inFlight.Store(0)
	j.startedAt.Store(time.Now().Unix())
	j.stateMu.Lock()
	j.mode = mode
	j.message = fmt.Sprintf("Silence %s job started", mode)
	j.margin = margin
	j.errors = nil
	j.stateMu.Unlock()
	return true
}

func (j *silenceMutationJob) addError(id string, err error) {
	j.stateMu.Lock()
	defer j.stateMu.Unlock()
	if len(j.errors) >= 10 {
		return
	}
	j.errors = append(j.errors, fmt.Sprintf("%s: %v", id, err))
}

func (j *silenceMutationJob) status() silenceMutationStatus {
	j.stateMu.RLock()
	mode, message, margin := j.mode, j.message, j.margin
	errors := append([]string(nil), j.errors...)
	j.stateMu.RUnlock()
	status := silenceMutationStatus{
		Running:       j.running.Load(),
		Mode:          mode,
		Total:         j.total.Load(),
		Processed:     j.processed.Load(),
		Succeeded:     j.succeeded.Load(),
		Skipped:       j.skipped.Load(),
		Failed:        j.failed.Load(),
		InFlight:      j.inFlight.Load(),
		Message:       message,
		BackupFolder:  coresilence.BackupFolderName,
		MarginSeconds: margin,
		Errors:        errors,
	}
	if startedAt := j.startedAt.Load(); startedAt > 0 {
		status.StartedAt = time.Unix(startedAt, 0).Format(time.RFC3339)
	}
	return status
}

func (n *Router) silenceMutationStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(n.silenceMutation.status())
	}
}

func (n *Router) startSilenceMutationHandler(mode string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var payload silenceMutationPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload.IDs = uniqueSilenceIDs(payload.IDs)
		if len(payload.IDs) == 0 {
			http.Error(w, "ids are required", http.StatusBadRequest)
			return
		}
		if len(payload.IDs) > maxSilenceMutationSongs {
			http.Error(w, fmt.Sprintf("at most %d songs can be changed in one job", maxSilenceMutationSongs), http.StatusBadRequest)
			return
		}
		margin := coresilence.TrimSafetyPad
		if payload.MarginSeconds != nil {
			margin = *payload.MarginSeconds
		}
		if err := validateSilenceMargin(margin); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := n.validateAllSilenceBackupLocations(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}

		n.silenceOperationMu.Lock()
		started := !n.silenceJob.running.Load() && n.silenceMutation.begin(mode, len(payload.IDs), margin)
		n.silenceOperationMu.Unlock()
		if !started {
			w.WriteHeader(http.StatusConflict)
			status := n.silenceMutation.status()
			status.Message = "Another silence analysis, trim, or restore job is already running"
			_ = json.NewEncoder(w).Encode(status)
			return
		}

		go n.runSilenceMutation(context.WithoutCancel(r.Context()), mode, payload.IDs, margin)
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(n.silenceMutation.status())
	}
}

// validateAllSilenceBackupLocations validates the actual per-library backup
// roots before accepting a job. The root is derived from the LUFS backup root,
// so validating the data directory here would protect the wrong location.
func (n *Router) validateAllSilenceBackupLocations(ctx context.Context) error {
	libraries, err := n.ds.Library(ctx).GetAll()
	if err != nil {
		return fmt.Errorf("load music libraries before file operation: %w", err)
	}
	validated := make(map[string]struct{}, len(libraries))
	for _, library := range libraries {
		trimmer := newSilenceTrimmer(library.Path)
		root, rootErr := filepath.Abs(filepath.Clean(trimmer.BackupRoot()))
		if rootErr != nil {
			return fmt.Errorf("resolve silence backup folder for %q: %w", library.Name, rootErr)
		}
		if _, ok := validated[root]; ok {
			continue
		}
		if err := n.validateSilenceBackupLocation(ctx, root); err != nil {
			return err
		}
		validated[root] = struct{}{}
	}
	return nil
}

func (n *Router) runSilenceMutation(ctx context.Context, mode string, ids []string, margin float64) {
	defer n.silenceMutation.running.Store(false)
	trimmer := coresilence.NewTrimmer(conf.Server.DataFolder.String())
	scanTargets := map[string]model.ScanTarget{}
	for _, id := range ids {
		n.silenceMutation.inFlight.Store(1)
		changed, err := n.mutateSongSilence(ctx, trimmer, mode, id, margin)
		n.silenceMutation.inFlight.Store(0)
		n.silenceMutation.processed.Add(1)
		if err != nil {
			n.silenceMutation.failed.Add(1)
			n.silenceMutation.addError(id, err)
			log.Warn(ctx, "Silence file operation failed", "mode", mode, "id", id, "err", err)
			continue
		}
		if !changed {
			n.silenceMutation.skipped.Add(1)
			continue
		}
		n.silenceMutation.succeeded.Add(1)
		mediaFile, loadErr := n.ds.MediaFile(ctx).Get(id)
		if loadErr != nil {
			n.silenceMutation.addError(id, fmt.Errorf("queue metadata refresh: %w", loadErr))
			continue
		}
		target, targetErr := silenceScanTarget(mediaFile)
		if targetErr != nil {
			n.silenceMutation.addError(id, fmt.Errorf("queue metadata refresh: %w", targetErr))
			continue
		}
		scanTargets[target.String()] = target
	}
	refreshFailed := false
	if n.scanner != nil && len(scanTargets) > 0 {
		n.silenceMutation.setState(mode, "Refreshing changed song metadata")
		targets := make([]model.ScanTarget, 0, len(scanTargets))
		for _, target := range scanTargets {
			targets = append(targets, target)
		}
		sort.Slice(targets, func(i, j int) bool { return targets[i].String() < targets[j].String() })
		warnings, scanErr := n.scanner.ScanFolders(ctx, false, targets)
		for _, warning := range warnings {
			log.Warn(ctx, "Silence file operation scan warning", "warning", warning)
		}
		if scanErr != nil {
			refreshFailed = true
			n.silenceMutation.addError("library refresh", scanErr)
			log.Warn(ctx, "Could not refresh changed songs after silence operation", "err", scanErr)
		}
	}
	failed := n.silenceMutation.failed.Load()
	if refreshFailed {
		n.silenceMutation.setState(mode, fmt.Sprintf("Silence %s completed; library metadata refresh failed", mode))
	} else if failed > 0 {
		n.silenceMutation.setState(mode, fmt.Sprintf("Silence %s finished with %d failure(s)", mode, failed))
	} else {
		n.silenceMutation.setState(mode, fmt.Sprintf("Silence %s completed", mode))
	}
}

func silenceScanTarget(mediaFile *model.MediaFile) (model.ScanTarget, error) {
	if mediaFile == nil || mediaFile.LibraryID <= 0 {
		return model.ScanTarget{}, errors.New("song library is unavailable")
	}
	sourcePath, err := secureSilenceMediaPath(mediaFile.LibraryPath, mediaFile.Path)
	if err != nil {
		return model.ScanTarget{}, err
	}
	libraryPath, err := filepath.Abs(filepath.Clean(mediaFile.LibraryPath))
	if err != nil {
		return model.ScanTarget{}, err
	}
	relativeFolder, err := filepath.Rel(libraryPath, filepath.Dir(sourcePath))
	if err != nil || relativeFolder == ".." || strings.HasPrefix(relativeFolder, ".."+string(filepath.Separator)) {
		return model.ScanTarget{}, errors.New("song folder escapes its library")
	}
	if relativeFolder == "." {
		relativeFolder = ""
	}
	return model.ScanTarget{LibraryID: mediaFile.LibraryID, FolderPath: filepath.ToSlash(relativeFolder)}, nil
}

func (n *Router) mutateSongSilence(ctx context.Context, trimmer *coresilence.Trimmer, mode, id string, margin float64) (bool, error) {
	mediaFile, err := n.ds.MediaFile(ctx).Get(id)
	if err != nil {
		return false, err
	}
	trimmer = newSilenceTrimmer(mediaFile.LibraryPath)
	if err := n.validateSilenceBackupLocation(ctx, trimmer.BackupRoot()); err != nil {
		return false, err
	}
	sourcePath, err := secureSilenceMediaPath(mediaFile.LibraryPath, mediaFile.Path)
	if err != nil {
		return false, err
	}
	if pathIsWithin(mediaFile.LibraryPath, trimmer.BackupRoot()) {
		return false, errors.New("silence backup folder must be outside the music library")
	}
	if mode == silenceMutationRestore {
		return n.restoreSongSilence(ctx, trimmer, mediaFile, sourcePath)
	}
	return n.trimSongSilence(ctx, trimmer, mediaFile, sourcePath, margin)
}

func (n *Router) trimSongSilence(ctx context.Context, trimmer *coresilence.Trimmer, mediaFile *model.MediaFile, sourcePath string, margin float64) (bool, error) {
	if err := validateSilenceMargin(margin); err != nil {
		return false, err
	}
	if err := coresilence.ValidateTrimSource(sourcePath); err != nil {
		return false, err
	}
	stored := mediaFile.SilenceAnalysis
	if stored == nil || stored.Status != model.SilenceStatusAnalyzed ||
		stored.ThresholdDB != coresilence.DefaultThresholdDB || stored.MinimumSilence != coresilence.DefaultMinimumSilence {
		return false, nil
	}
	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		return false, err
	}
	if stored.SourceSize != sourceStat.Size() {
		return false, errors.New("song changed after analysis; analyze it again before trimming")
	}

	fresh, err := coresilence.NewAnalyzer().Analyze(ctx, sourcePath, float64(mediaFile.Duration))
	if err != nil {
		return false, err
	}
	trimStart := max(0, fresh.Leading-margin)
	trimEnd := max(0, fresh.Trailing-margin)
	if trimStart+trimEnd <= 0 || fresh.Duration-trimStart-trimEnd < 1 {
		return false, nil
	}
	currentHash, err := coresilence.HashFile(sourcePath)
	if err != nil {
		return false, err
	}

	backup := mediaFile.SilenceBackup
	if backup == nil {
		backupInfo, backupErr := trimmer.EnsureBackup(sourcePath, mediaFile.ID)
		if backupErr != nil {
			return false, backupErr
		}
		if backupInfo.SHA256 != currentHash {
			return false, errors.New("song changed while its backup was being created; analyze it again")
		}
		backup = &model.SilenceBackup{
			MediaFileID: mediaFile.ID, BackupFile: backupInfo.FileName,
			Status: model.SilenceBackupStatusReady, OriginalSHA256: backupInfo.SHA256,
			OriginalSize: backupInfo.Size, OriginalMode: backupInfo.Mode,
			OriginalModTime: backupInfo.ModTime, CreatedAt: time.Now(),
			IntegrityStatus: model.SilenceIntegrityPending, IntegrityBackupVerified: true,
		}
		if err := n.ds.SilenceBackup(ctx).Put(backup); err != nil {
			return false, fmt.Errorf("record silence backup before trim: %w", err)
		}
	} else {
		if verifyErr := trimmer.VerifyBackup(backup.BackupFile, backup.OriginalSHA256, backup.OriginalSize); verifyErr != nil {
			return false, verifyErr
		}
		backup.IntegrityBackupVerified = true
	}

	switch classifySilenceBackupFile(backup, currentHash) {
	case silenceFileIsTrimmed:
		// Recovery for a crash after atomic replacement but before the final
		// status write. The prepared record already contains both hashes.
		if backup.Status == model.SilenceBackupStatusPrepared {
			trimmedAt := time.Now()
			backup.Status = model.SilenceBackupStatusAvailable
			backup.TrimmedAt = &trimmedAt
			backup.IntegrityStatus = model.SilenceIntegrityVerified
			backup.IntegrityVerifiedAt = &trimmedAt
			if err := n.ds.SilenceBackup(ctx).Put(backup); err != nil {
				return false, fmt.Errorf("finish interrupted trim record: %w", err)
			}
		}
		return false, nil
	case silenceFileIsOriginal:
		// Continue. A stale prepared record means the crash occurred before
		// replacement, so it is safe to build a fresh candidate.
	default:
		return false, errors.New("current song differs from both the exact backup and recorded trim; replacement refused")
	}

	trimCtx, cancel := context.WithTimeout(ctx, silenceTrimTimeout(fresh.Duration))
	defer cancel()
	prepared, err := trimmer.PrepareTrimWithPadding(trimCtx, sourcePath, currentHash, fresh.Duration, fresh.Leading, fresh.Trailing, margin)
	if err != nil {
		return false, err
	}
	defer prepared.Discard()
	preparedAt := time.Now()
	backup.Status = model.SilenceBackupStatusPrepared
	backup.TrimmedSHA256 = prepared.Result.SHA256
	backup.TrimmedSize = prepared.Result.Size
	backup.TrimStart = trimStart
	backup.TrimEnd = trimEnd
	backup.PreparedAt = &preparedAt
	backup.TrimmedAt = nil
	backup.RestoredAt = nil
	backup.IntegrityStatus = model.SilenceIntegrityPrepared
	backup.IntegrityBackupVerified = true
	backup.IntegrityDecodeVerified = prepared.Result.DecodeVerified
	backup.IntegrityAudioVerified = prepared.Result.AudioPropertiesVerified
	backup.IntegrityMetadataVerified = prepared.Result.MetadataVerified
	backup.IntegrityArtworkVerified = prepared.Result.ArtworkVerified
	backup.IntegrityPacketVerified = prepared.Result.PacketIntegrityVerified
	backup.IntegrityError = ""
	backup.IntegrityVerifiedAt = nil
	if err := n.ds.SilenceBackup(ctx).Put(backup); err != nil {
		return false, fmt.Errorf("record prepared trim before replacement: %w", err)
	}
	if err := trimmer.CommitTrim(sourcePath, currentHash, prepared); err != nil {
		return false, err
	}

	trimmedAt := time.Now()
	backup.Status = model.SilenceBackupStatusAvailable
	backup.TrimmedAt = &trimmedAt
	verifiedAt := time.Now()
	backup.IntegrityStatus = model.SilenceIntegrityVerified
	backup.IntegrityVerifiedAt = &verifiedAt
	if err := n.ds.SilenceBackup(ctx).Put(backup); err != nil {
		// The prepared row intentionally remains recoverable. A later Trim or
		// Restore hashes the current source and completes the interrupted state.
		return false, fmt.Errorf("trim committed but final audit update failed; restore remains recoverable: %w", err)
	}

	updatedStat, statErr := os.Stat(sourcePath)
	if statErr == nil {
		leading, trailing := prepared.Result.Leading, prepared.Result.Trailing
		analysis := &model.SilenceAnalysis{
			MediaFileID:         mediaFile.ID,
			LeadingSilence:      mediaFile.SilenceAnalysis.LeadingSilence,
			TrailingSilence:     mediaFile.SilenceAnalysis.TrailingSilence,
			LeadingSilenceAfter: &leading, TrailingSilenceAfter: &trailing,
			ThresholdDB: coresilence.DefaultThresholdDB, MinimumSilence: coresilence.DefaultMinimumSilence,
			SourceSize: updatedStat.Size(), SourceUpdatedAt: updatedStat.ModTime(),
			Status: model.SilenceStatusTrimmed, AnalyzedAt: time.Now(),
		}
		if putErr := n.ds.SilenceAnalysis(ctx).Put(analysis); putErr != nil {
			log.Warn(ctx, "Could not save post-trim silence measurement", "id", mediaFile.ID, "err", putErr)
		}
		if updateErr := n.ds.MediaFile(ctx).UpdateSilenceMutationProperties(
			mediaFile.ID, updatedStat.Size(), prepared.Result.Duration, updatedStat.ModTime(),
		); updateErr != nil {
			log.Warn(ctx, "Could not refresh post-trim media properties", "id", mediaFile.ID, "err", updateErr)
		}
	}
	return true, nil
}

func silenceTrimTimeout(duration float64) time.Duration {
	if duration <= 0 || duration > float64(maxSilenceTrimTimeout/time.Second-2*time.Minute) {
		return maxSilenceTrimTimeout
	}
	return 2*time.Minute + time.Duration(duration*float64(time.Second))
}

func (n *Router) restoreSongSilence(ctx context.Context, trimmer *coresilence.Trimmer, mediaFile *model.MediaFile, sourcePath string) (bool, error) {
	backup := mediaFile.SilenceBackup
	if backup == nil {
		return false, nil
	}
	currentHash, err := coresilence.HashFile(sourcePath)
	if err != nil {
		return false, err
	}
	switch classifySilenceBackupFile(backup, currentHash) {
	case silenceFileIsOriginal:
		if backup.Status != model.SilenceBackupStatusRestored {
			restoredAt := time.Now()
			backup.Status = model.SilenceBackupStatusRestored
			backup.RestoredAt = &restoredAt
			_ = n.ds.SilenceBackup(ctx).Put(backup)
		}
		return false, nil
	case silenceFileIsTrimmed:
		// Both available and prepared are restorable. Prepared is the durable
		// crash-recovery state saved before source replacement.
	default:
		return false, errors.New("current song changed after trimming; restore refused")
	}
	restored, err := trimmer.Restore(
		sourcePath, backup.BackupFile, backup.OriginalSHA256, currentHash,
		backup.OriginalMode, backup.OriginalModTime,
	)
	if err != nil {
		return false, err
	}
	if restored.SHA256 != backup.OriginalSHA256 || restored.Size != backup.OriginalSize {
		return false, errors.New("restored song does not match the original checksum and size")
	}
	restoredAt := time.Now()
	backup.Status = model.SilenceBackupStatusRestored
	backup.RestoredAt = &restoredAt
	backup.IntegrityStatus = model.SilenceIntegrityRestored
	backup.IntegrityRestoreVerified = true
	backup.IntegrityVerifiedAt = &restoredAt
	backup.IntegrityError = ""
	if err := n.ds.SilenceBackup(ctx).Put(backup); err != nil {
		return false, fmt.Errorf("record restore: %w", err)
	}

	analysisResult, analysisErr := coresilence.NewAnalyzer().Analyze(ctx, sourcePath, float64(mediaFile.Duration))
	if analysisErr == nil {
		stat, statErr := os.Stat(sourcePath)
		if statErr == nil {
			leading, trailing := analysisResult.Leading, analysisResult.Trailing
			analysis := &model.SilenceAnalysis{
				MediaFileID: mediaFile.ID, LeadingSilence: &leading, TrailingSilence: &trailing,
				ThresholdDB: coresilence.DefaultThresholdDB, MinimumSilence: coresilence.DefaultMinimumSilence,
				SourceSize: stat.Size(), SourceUpdatedAt: stat.ModTime(),
				Status: model.SilenceStatusAnalyzed, AnalyzedAt: time.Now(),
			}
			if putErr := n.ds.SilenceAnalysis(ctx).Put(analysis); putErr != nil {
				log.Warn(ctx, "Could not save restored silence measurement", "id", mediaFile.ID, "err", putErr)
			}
			if updateErr := n.ds.MediaFile(ctx).UpdateSilenceMutationProperties(
				mediaFile.ID, stat.Size(), analysisResult.Duration, stat.ModTime(),
			); updateErr != nil {
				log.Warn(ctx, "Could not refresh restored media properties", "id", mediaFile.ID, "err", updateErr)
			}
		}
	}
	return true, nil
}

type silenceBackupFileState uint8

const (
	silenceFileIsUnknown silenceBackupFileState = iota
	silenceFileIsOriginal
	silenceFileIsTrimmed
)

func classifySilenceBackupFile(backup *model.SilenceBackup, currentHash string) silenceBackupFileState {
	if backup == nil || currentHash == "" {
		return silenceFileIsUnknown
	}
	if currentHash == backup.OriginalSHA256 {
		return silenceFileIsOriginal
	}
	if backup.TrimmedSHA256 != "" && currentHash == backup.TrimmedSHA256 {
		return silenceFileIsTrimmed
	}
	return silenceFileIsUnknown
}

// validateSilenceBackupLocation checks the canonical path against every music
// library, not just the selected song's library. Otherwise a multi-library
// installation could accidentally scan its exact backups as new songs.
func (n *Router) validateSilenceBackupLocation(ctx context.Context, backupRoot string) error {
	canonicalBackup, err := canonicalPathAllowMissing(backupRoot)
	if err != nil {
		return fmt.Errorf("resolve silence backup folder: %w", err)
	}
	if info, statErr := os.Lstat(backupRoot); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return errors.New("silence backup folder cannot be a symbolic link")
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("stat silence backup folder: %w", statErr)
	}
	libraries, err := n.ds.Library(ctx).GetAll()
	if err != nil {
		return fmt.Errorf("load music libraries before file operation: %w", err)
	}
	for _, library := range libraries {
		canonicalLibrary, resolveErr := filepath.EvalSymlinks(library.Path)
		if resolveErr != nil {
			return fmt.Errorf("resolve music library %q: %w", library.Name, resolveErr)
		}
		if pathIsWithin(canonicalLibrary, canonicalBackup) {
			return fmt.Errorf("silence backup folder must be outside every music library (inside %q)", library.Name)
		}
	}
	return nil
}

func canonicalPathAllowMissing(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	existing := absolute
	missing := make([]string, 0, 2)
	for {
		_, statErr := os.Lstat(existing)
		if statErr == nil {
			break
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return "", statErr
		}
		missing = append(missing, filepath.Base(existing))
		existing = parent
	}
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", err
	}
	for index := len(missing) - 1; index >= 0; index-- {
		resolved = filepath.Join(resolved, missing[index])
	}
	return filepath.Clean(resolved), nil
}

func uniqueSilenceIDs(ids []string) []string {
	unique := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

func secureSilenceMediaPath(libraryPath, mediaPath string) (string, error) {
	libraryAbs, err := filepath.Abs(filepath.Clean(filepath.FromSlash(libraryPath)))
	if err != nil {
		return "", err
	}
	mediaPath = filepath.FromSlash(strings.TrimSpace(mediaPath))
	sourceAbs := mediaPath
	if !filepath.IsAbs(sourceAbs) {
		sourceAbs = filepath.Join(libraryAbs, sourceAbs)
	}
	sourceAbs, err = filepath.Abs(filepath.Clean(sourceAbs))
	if err != nil || !pathIsWithin(libraryAbs, sourceAbs) {
		return "", errors.New("song path escapes its music library")
	}
	lstat, err := os.Lstat(sourceAbs)
	if err != nil {
		return "", err
	}
	if lstat.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("symbolic-link audio files are not supported for trimming")
	}
	canonicalLibrary, libraryErr := filepath.EvalSymlinks(libraryAbs)
	canonicalSource, sourceErr := filepath.EvalSymlinks(sourceAbs)
	if libraryErr == nil && sourceErr == nil && !pathIsWithin(canonicalLibrary, canonicalSource) {
		return "", errors.New("resolved song path escapes its music library")
	}
	return sourceAbs, nil
}

func pathIsWithin(root, target string) bool {
	rootAbs, rootErr := filepath.Abs(filepath.Clean(root))
	targetAbs, targetErr := filepath.Abs(filepath.Clean(target))
	if rootErr != nil || targetErr != nil {
		return false
	}
	relative, err := filepath.Rel(rootAbs, targetAbs)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
