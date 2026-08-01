package silence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/navidrome/navidrome/core/ffmpeg"
)

const (
	BackupFolderName = "silence trim backup"
	// Keep a full 100 ms beyond the detected edge. MP3 frame boundaries and
	// gapless encoder padding can otherwise consume part of a 50 ms margin.
	TrimSafetyPad       = 0.100
	MinimumRetainedEdge = 0.050
	minimumKeptAudio    = 1.000
)

var safeExtensionPattern = regexp.MustCompile(`^\.[a-z0-9]{1,10}$`)

type BackupInfo struct {
	FileName string
	SHA256   string
	Size     int64
	Mode     uint32
	ModTime  time.Time
}

type FileInfo struct {
	SHA256                  string
	Size                    int64
	Duration                float64
	Leading                 float64
	Trailing                float64
	DecodeVerified          bool
	AudioPropertiesVerified bool
	MetadataVerified        bool
	ArtworkVerified         bool
	PacketIntegrityVerified bool
}

type Trimmer struct {
	backupRoot string
}

// PreparedTrim is a fully written and validated replacement that has not yet
// touched the source path. The caller must persist Result.SHA256 before Commit
// so a crash after the atomic replacement can always be reconciled.
type PreparedTrim struct {
	Result     FileInfo
	path       string
	sourceInfo os.FileInfo
}

func (p *PreparedTrim) Discard() {
	if p == nil || p.path == "" {
		return
	}
	_ = os.Remove(p.path)
	p.path = ""
}

func NewTrimmer(dataFolder string) *Trimmer {
	return &Trimmer{backupRoot: filepath.Join(dataFolder, BackupFolderName)}
}

func (t *Trimmer) BackupRoot() string {
	return t.backupRoot
}

func ValidateTrimSource(sourcePath string) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("symbolic-link audio files are not supported for trimming")
	}
	if !info.Mode().IsRegular() {
		return errors.New("source is not a regular file")
	}
	if err := rejectHardLinks(info); err != nil {
		return err
	}
	if err := validateSourceLinks(sourcePath); err != nil {
		return err
	}
	// The first production-safe writer is deliberately narrow. MP3 can be cut
	// by copying whole encoded frames; no lossy re-encode is needed. Other
	// formats require format-specific metadata and lossless validation.
	if safeExtension(sourcePath) != ".mp3" {
		return errors.New("safe trimming currently supports MP3 files only")
	}
	return nil
}

// EnsureBackup creates one exact, durable copy of the original. If a matching
// orphan from an interrupted database write already exists, it is reused only
// when its hash is identical to the current source.
func (t *Trimmer) EnsureBackup(sourcePath, mediaFileID string) (BackupInfo, error) {
	if strings.TrimSpace(mediaFileID) == "" {
		return BackupInfo{}, errors.New("media file id is required")
	}
	if err := ValidateTrimSource(sourcePath); err != nil {
		return BackupInfo{}, err
	}
	sourceHash, sourceInfo, err := hashStableFile(sourcePath)
	if err != nil {
		return BackupInfo{}, err
	}
	if err := os.MkdirAll(t.backupRoot, 0o700); err != nil {
		return BackupInfo{}, fmt.Errorf("create backup folder: %w", err)
	}
	rootInfo, err := os.Lstat(t.backupRoot)
	if err != nil {
		return BackupInfo{}, fmt.Errorf("stat backup folder: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return BackupInfo{}, errors.New("silence backup folder must be a real directory, not a symbolic link")
	}
	if err := os.Chmod(t.backupRoot, 0o700); err != nil {
		return BackupInfo{}, fmt.Errorf("secure backup folder: %w", err)
	}

	fileName := backupFileName(mediaFileID, sourcePath)
	backupPath, err := t.BackupPath(fileName)
	if err != nil {
		return BackupInfo{}, err
	}
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o700); err != nil {
		return BackupInfo{}, fmt.Errorf("create backup shard: %w", err)
	}
	if err := t.validateBackupDirectoryChain(backupPath); err != nil {
		return BackupInfo{}, err
	}
	if backupInfo, statErr := os.Lstat(backupPath); statErr == nil {
		if backupInfo.Mode()&os.ModeSymlink != 0 || !backupInfo.Mode().IsRegular() {
			return BackupInfo{}, errors.New("stored silence backup is not a regular file")
		}
		backupHash, _, hashErr := hashStableFile(backupPath)
		if hashErr != nil {
			return BackupInfo{}, hashErr
		}
		if backupHash != sourceHash {
			return BackupInfo{}, errors.New("an unrelated backup already exists for this song")
		}
		return BackupInfo{
			FileName: fileName, SHA256: backupHash, Size: backupInfo.Size(),
			Mode: uint32(sourceInfo.Mode().Perm()), ModTime: sourceInfo.ModTime(),
		}, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return BackupInfo{}, fmt.Errorf("stat backup: %w", statErr)
	}

	writtenHash, writtenSize, err := copyFileAtomically(sourcePath, backupPath, sourceInfo.Mode().Perm())
	if err != nil {
		return BackupInfo{}, fmt.Errorf("create backup: %w", err)
	}
	if writtenHash != sourceHash || writtenSize != sourceInfo.Size() {
		return BackupInfo{}, errors.New("backup verification failed")
	}
	if err := os.Chtimes(backupPath, sourceInfo.ModTime(), sourceInfo.ModTime()); err != nil {
		return BackupInfo{}, fmt.Errorf("preserve backup timestamp: %w", err)
	}
	if err := syncFile(backupPath); err != nil {
		return BackupInfo{}, err
	}
	if err := syncDirectory(filepath.Dir(backupPath)); err != nil {
		return BackupInfo{}, err
	}
	return BackupInfo{
		FileName: fileName, SHA256: writtenHash, Size: writtenSize,
		Mode: uint32(sourceInfo.Mode().Perm()), ModTime: sourceInfo.ModTime(),
	}, nil
}

func (t *Trimmer) BackupPath(fileName string) (string, error) {
	fileName = filepath.FromSlash(strings.TrimSpace(fileName))
	if fileName == "" || filepath.IsAbs(fileName) {
		return "", errors.New("invalid backup filename")
	}
	clean := filepath.Clean(fileName)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid backup filename")
	}
	path := filepath.Join(t.backupRoot, clean)
	relative, err := filepath.Rel(t.backupRoot, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("backup filename escapes the silence backup folder")
	}
	return path, nil
}

func (t *Trimmer) VerifyBackup(fileName, expectedHash string, expectedSize int64) error {
	if expectedHash == "" {
		return errors.New("expected backup checksum is required")
	}
	rootInfo, err := os.Lstat(t.backupRoot)
	if err != nil {
		return fmt.Errorf("stat silence backup folder: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return errors.New("silence backup folder must be a real directory")
	}
	backupPath, err := t.BackupPath(fileName)
	if err != nil {
		return err
	}
	if err := t.validateBackupDirectoryChain(backupPath); err != nil {
		return err
	}
	backupInfo, err := os.Lstat(backupPath)
	if err != nil {
		return fmt.Errorf("stat silence backup: %w", err)
	}
	if backupInfo.Mode()&os.ModeSymlink != 0 || !backupInfo.Mode().IsRegular() {
		return errors.New("stored silence backup is not a regular file")
	}
	backupHash, stableInfo, err := hashStableFile(backupPath)
	if err != nil {
		return err
	}
	if backupHash != expectedHash || (expectedSize > 0 && stableInfo.Size() != expectedSize) {
		return errors.New("stored silence backup failed checksum or size verification")
	}
	return nil
}

func (t *Trimmer) validateBackupDirectoryChain(backupPath string) error {
	relative, err := filepath.Rel(t.backupRoot, filepath.Dir(backupPath))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("backup path escapes the silence backup folder")
	}
	current := t.backupRoot
	parts := []string{}
	if relative != "." {
		parts = strings.Split(relative, string(filepath.Separator))
	}
	for _, part := range append([]string{""}, parts...) {
		if part != "" {
			current = filepath.Join(current, part)
		}
		info, statErr := os.Lstat(current)
		if statErr != nil {
			return fmt.Errorf("stat backup directory: %w", statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("silence backup path contains a symbolic link or non-directory")
		}
	}
	return nil
}

// PrepareTrim writes and validates a replacement without touching sourcePath.
// Audio packets, metadata and embedded artwork are copied; audio is never
// decoded/re-encoded. Call CommitTrim only after Result has been persisted.
func (t *Trimmer) PrepareTrim(ctx context.Context, sourcePath, expectedSourceHash string, duration, removeStart, removeEnd float64) (*PreparedTrim, error) {
	return t.PrepareTrimWithPadding(ctx, sourcePath, expectedSourceHash, duration, removeStart, removeEnd, TrimSafetyPad)
}

// PrepareTrimWithPadding is the configurable variant used by the Silence Trim
// UI. The padding is retained on both edges and is never passed to ffmpeg as a
// removable range.
func (t *Trimmer) PrepareTrimWithPadding(ctx context.Context, sourcePath, expectedSourceHash string, duration, removeStart, removeEnd, padding float64) (*PreparedTrim, error) {
	if err := ValidateTrimSource(sourcePath); err != nil {
		return nil, err
	}
	if expectedSourceHash == "" {
		return nil, errors.New("expected source checksum is required")
	}
	if duration <= 0 {
		return nil, errors.New("decoded duration is unavailable")
	}
	if math.IsNaN(padding) || math.IsInf(padding, 0) || padding < 0 || padding > 2 {
		return nil, errors.New("trim padding must be between 0 and 2 seconds")
	}
	removeStart = max(0, removeStart-padding)
	removeEnd = max(0, removeEnd-padding)
	keepDuration := duration - removeStart - removeEnd
	if removeStart+removeEnd <= 0 || keepDuration < minimumKeptAudio {
		return nil, errors.New("safe trim range is empty")
	}

	actualSourceHash, sourceInfo, err := hashStableFile(sourcePath)
	if err != nil {
		return nil, err
	}
	if actualSourceHash != expectedSourceHash {
		return nil, errors.New("song changed while preparing the trim")
	}
	cmdPath, err := ffmpeg.New().CmdPath()
	if err != nil {
		return nil, fmt.Errorf("find ffmpeg: %w", err)
	}
	probe := ffmpeg.New()
	beforeProbe, err := probe.ProbeAudioStream(ctx, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("probe source audio: %w", err)
	}
	if beforeProbe.Codec != "mp3" {
		return nil, fmt.Errorf("safe MP3 trim cannot copy codec %q", beforeProbe.Codec)
	}
	beforeSnapshot, err := probeMediaSnapshot(ctx, cmdPath, sourcePath)
	if err != nil {
		return nil, err
	}
	if err := beforeSnapshot.validateForMP3Trim(); err != nil {
		return nil, err
	}
	var beforeArtworkHash string
	if beforeSnapshot.hasArtwork() {
		beforeArtworkHash, err = artworkSHA256(ctx, cmdPath, sourcePath)
		if err != nil {
			return nil, err
		}
	}

	audioTemporaryPath, err := createClosedTemporary(sourcePath, ".navidrome-silence-audio-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(audioTemporaryPath)
	finalTemporaryPath, err := createClosedTemporary(sourcePath, ".navidrome-silence-trim-*")
	if err != nil {
		return nil, err
	}
	keepFinal := false
	defer func() {
		if !keepFinal {
			_ = os.Remove(finalTemporaryPath)
		}
	}()

	args := []string{
		"-y", "-nostdin", "-hide_banner", "-loglevel", "error",
		"-i", sourcePath,
		"-ss", formatSeconds(removeStart),
		"-t", formatSeconds(keepDuration),
		"-map", "0:a:0", "-map_metadata", "-1", "-map_chapters", "-1",
		"-c:a", "copy", "-vn", "-avoid_negative_ts", "make_zero",
		audioTemporaryPath,
	}
	output, err := exec.CommandContext(ctx, cmdPath, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("lossless audio trim failed: %w: %s", err, tail(string(output), 2048))
	}

	remuxArgs := []string{
		"-y", "-nostdin", "-hide_banner", "-loglevel", "error",
		"-i", audioTemporaryPath, "-i", sourcePath,
		"-map", "0:a:0", "-map_metadata", "1", "-map_metadata:s:a:0", "1:s:a:0",
		"-map_chapters", "-1", "-c", "copy",
	}
	if beforeSnapshot.hasArtwork() {
		remuxArgs = append(remuxArgs,
			"-map", "1:v:0", "-map_metadata:s:v:0", "1:s:v:0",
			"-c:v", "copy", "-disposition:v:0", "attached_pic",
		)
	} else {
		remuxArgs = append(remuxArgs, "-vn")
	}
	remuxArgs = append(remuxArgs, finalTemporaryPath)
	output, err = exec.CommandContext(ctx, cmdPath, remuxArgs...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("restore MP3 metadata and artwork: %w: %s", err, tail(string(output), 2048))
	}
	trimmedInfo, err := os.Stat(finalTemporaryPath)
	if err != nil {
		return nil, fmt.Errorf("stat trimmed file: %w", err)
	}
	if trimmedInfo.Size() <= 0 {
		return nil, errors.New("trimmed output is empty")
	}
	afterProbe, err := probe.ProbeAudioStream(ctx, finalTemporaryPath)
	if err != nil {
		return nil, fmt.Errorf("probe trimmed audio: %w", err)
	}
	if beforeProbe.Codec != afterProbe.Codec ||
		beforeProbe.SampleRate != afterProbe.SampleRate ||
		beforeProbe.Channels != afterProbe.Channels ||
		beforeProbe.BitDepth != afterProbe.BitDepth {
		return nil, errors.New("trimmed audio properties do not match the original")
	}
	afterSnapshot, err := probeMediaSnapshot(ctx, cmdPath, finalTemporaryPath)
	if err != nil {
		return nil, err
	}
	if err := compareMediaSnapshots(beforeSnapshot, afterSnapshot); err != nil {
		return nil, err
	}
	if beforeSnapshot.hasArtwork() {
		afterArtworkHash, artErr := artworkSHA256(ctx, cmdPath, finalTemporaryPath)
		if artErr != nil {
			return nil, artErr
		}
		if afterArtworkHash != beforeArtworkHash {
			return nil, errors.New("embedded cover artwork changed during trim")
		}
	}
	if err := validatePacketSubsequence(ctx, cmdPath, sourcePath, finalTemporaryPath); err != nil {
		return nil, err
	}
	// Validate with a shorter detector window than the UI analysis. This catches
	// residual edge silence that is shorter than the normal 50 ms display
	// threshold, while still requiring the promised 50 ms safety margin.
	postAnalysis, err := (&analyzer{thresholdDB: DefaultThresholdDB, minimumLength: 0.005}).Analyze(ctx, finalTemporaryPath, keepDuration)
	if err != nil {
		return nil, fmt.Errorf("validate trimmed audio: %w", err)
	}
	if removeStart > 0 && postAnalysis.Leading < MinimumRetainedEdge {
		return nil, errors.New("trimmed output lost the required leading silence safety margin")
	}
	if removeEnd > 0 && postAnalysis.Trailing < MinimumRetainedEdge {
		return nil, errors.New("trimmed output lost the required trailing silence safety margin")
	}
	if postAnalysis.Duration <= 0 || postAnalysis.Duration >= duration-0.005 {
		return nil, errors.New("trimmed duration was not safely reduced")
	}
	if math.Abs(postAnalysis.Duration-keepDuration) > 0.150 {
		return nil, errors.New("trimmed duration is outside packet-copy tolerance")
	}
	// If an edge was not part of the requested trim, do not expose container
	// or encoder padding introduced while remuxing as newly detected silence.
	// The after-trim columns describe the edges we actually trimmed.
	leadingAfter, trailingAfter := postAnalysis.Leading, postAnalysis.Trailing
	if removeStart <= 0 {
		leadingAfter = 0
	}
	if removeEnd <= 0 {
		trailingAfter = 0
	}
	if err := os.Chmod(finalTemporaryPath, sourceInfo.Mode().Perm()); err != nil {
		return nil, fmt.Errorf("preserve file permissions: %w", err)
	}
	if err := syncFile(finalTemporaryPath); err != nil {
		return nil, err
	}
	trimmedHash, err := HashFile(finalTemporaryPath)
	if err != nil {
		return nil, err
	}
	prepared := &PreparedTrim{
		Result: FileInfo{
			SHA256: trimmedHash, Size: trimmedInfo.Size(), Duration: postAnalysis.Duration,
			Leading: leadingAfter, Trailing: trailingAfter,
			DecodeVerified: true, AudioPropertiesVerified: true, MetadataVerified: true,
			ArtworkVerified: true, PacketIntegrityVerified: true,
		},
		path: finalTemporaryPath, sourceInfo: sourceInfo,
	}
	keepFinal = true
	return prepared, nil
}

// CommitTrim performs the small destructive step after the caller has saved a
// recoverable prepared record. It rechecks the source immediately before the
// atomic replacement and refuses concurrent edits or path replacement.
func (t *Trimmer) CommitTrim(sourcePath, expectedSourceHash string, prepared *PreparedTrim) error {
	if prepared == nil || prepared.path == "" || prepared.sourceInfo == nil {
		return errors.New("prepared trim is unavailable")
	}
	if err := ValidateTrimSource(sourcePath); err != nil {
		return err
	}
	currentHash, currentInfo, err := hashStableFile(sourcePath)
	if err != nil {
		return err
	}
	if currentHash != expectedSourceHash || !os.SameFile(prepared.sourceInfo, currentInfo) ||
		prepared.sourceInfo.Size() != currentInfo.Size() ||
		prepared.sourceInfo.Mode() != currentInfo.Mode() ||
		!prepared.sourceInfo.ModTime().Equal(currentInfo.ModTime()) {
		return errors.New("song changed while the trim was being prepared; replacement refused")
	}
	if err := replaceFile(prepared.path, sourcePath); err != nil {
		return fmt.Errorf("replace source atomically: %w", err)
	}
	prepared.path = ""
	committedHash, err := HashFile(sourcePath)
	if err != nil {
		return err
	}
	if committedHash != prepared.Result.SHA256 {
		return errors.New("committed trim checksum does not match the validated output")
	}
	return nil
}

// Restore verifies the backup hash and then copies it to a temporary file in
// the source directory before the atomic replacement.
func (t *Trimmer) Restore(sourcePath, backupFileName, expectedBackupHash, expectedCurrentHash string, originalMode uint32, originalModTime time.Time) (FileInfo, error) {
	if err := ValidateTrimSource(sourcePath); err != nil {
		return FileInfo{}, err
	}
	if expectedCurrentHash == "" {
		return FileInfo{}, errors.New("expected current checksum is required")
	}
	initialHash, initialInfo, err := hashStableFile(sourcePath)
	if err != nil {
		return FileInfo{}, err
	}
	if initialHash != expectedCurrentHash {
		return FileInfo{}, errors.New("current song checksum changed before restore")
	}
	backupPath, err := t.BackupPath(backupFileName)
	if err != nil {
		return FileInfo{}, err
	}
	if err := t.VerifyBackup(backupFileName, expectedBackupHash, 0); err != nil {
		return FileInfo{}, err
	}
	backupInfo, err := os.Lstat(backupPath)
	if err != nil {
		return FileInfo{}, fmt.Errorf("stat backup: %w", err)
	}
	if backupInfo.Mode()&os.ModeSymlink != 0 || !backupInfo.Mode().IsRegular() {
		return FileInfo{}, errors.New("stored silence backup is not a regular file")
	}
	backupHash, _, err := hashStableFile(backupPath)
	if err != nil {
		return FileInfo{}, err
	}
	if expectedBackupHash == "" || backupHash != expectedBackupHash {
		return FileInfo{}, errors.New("backup checksum verification failed")
	}
	mode := os.FileMode(originalMode).Perm()
	if mode == 0 {
		mode = backupInfo.Mode().Perm()
	}
	temporary, err := os.CreateTemp(filepath.Dir(sourcePath), ".navidrome-silence-restore-*"+safeExtension(sourcePath))
	if err != nil {
		return FileInfo{}, fmt.Errorf("create restore temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return FileInfo{}, err
	}
	_ = os.Remove(temporaryPath)
	defer os.Remove(temporaryPath)

	writtenHash, writtenSize, err := copyFileAtomically(backupPath, temporaryPath, mode)
	if err != nil {
		return FileInfo{}, fmt.Errorf("prepare restore: %w", err)
	}
	if writtenHash != expectedBackupHash || writtenSize != backupInfo.Size() {
		return FileInfo{}, errors.New("restored temporary file verification failed")
	}
	if !originalModTime.IsZero() {
		if err := os.Chtimes(temporaryPath, originalModTime, originalModTime); err != nil {
			return FileInfo{}, fmt.Errorf("restore original timestamp: %w", err)
		}
	}
	if err := syncFile(temporaryPath); err != nil {
		return FileInfo{}, err
	}
	currentHash, currentInfo, err := hashStableFile(sourcePath)
	if err != nil {
		return FileInfo{}, err
	}
	if currentHash != expectedCurrentHash || !os.SameFile(initialInfo, currentInfo) {
		return FileInfo{}, errors.New("song changed while restore was being prepared; replacement refused")
	}
	if err := replaceFile(temporaryPath, sourcePath); err != nil {
		return FileInfo{}, fmt.Errorf("restore source atomically: %w", err)
	}
	restoredHash, err := HashFile(sourcePath)
	if err != nil {
		return FileInfo{}, err
	}
	if restoredHash != expectedBackupHash {
		return FileInfo{}, errors.New("committed restore checksum does not match the backup")
	}
	return FileInfo{SHA256: restoredHash, Size: writtenSize}, nil
}

func HashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file for checksum: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("checksum file: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func backupFileName(mediaFileID, sourcePath string) string {
	idHash := sha256.Sum256([]byte(mediaFileID))
	hash := hex.EncodeToString(idHash[:])
	return filepath.ToSlash(filepath.Join(hash[:2], hash+safeExtension(sourcePath)))
}

func safeExtension(path string) string {
	extension := strings.ToLower(filepath.Ext(path))
	if safeExtensionPattern.MatchString(extension) {
		return extension
	}
	return ".audio"
}

func formatSeconds(seconds float64) string {
	return fmt.Sprintf("%.6f", seconds)
}

func syncFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open file for sync: %w", err)
	}
	defer file.Close()
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync file: %w", err)
	}
	return nil
}

func createClosedTemporary(sourcePath, pattern string) (string, error) {
	temporary, err := os.CreateTemp(filepath.Dir(sourcePath), pattern+safeExtension(sourcePath))
	if err != nil {
		return "", fmt.Errorf("create temporary file: %w", err)
	}
	path := temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

// hashStableFile verifies that the path still names the same unmodified file
// after the complete checksum read. It detects path swaps and normal concurrent
// writers before any destructive commit is attempted.
func hashStableFile(path string) (string, os.FileInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", nil, fmt.Errorf("open file for stable checksum: %w", err)
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil {
		return "", nil, err
	}
	if !before.Mode().IsRegular() {
		return "", nil, errors.New("file is not regular")
	}
	if err := rejectHardLinks(before); err != nil {
		return "", nil, err
	}
	if err := validateSourceLinks(path); err != nil {
		return "", nil, err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", nil, fmt.Errorf("checksum file: %w", err)
	}
	after, err := file.Stat()
	if err != nil {
		return "", nil, err
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return "", nil, err
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(before, after) || !os.SameFile(after, pathInfo) ||
		before.Size() != after.Size() || before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) {
		return "", nil, errors.New("file changed while its checksum was being read")
	}
	return hex.EncodeToString(hash.Sum(nil)), after, nil
}

func copyFileAtomically(sourcePath, destinationPath string, mode os.FileMode) (hash string, size int64, err error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", 0, err
	}
	defer source.Close()
	temporary, err := os.CreateTemp(filepath.Dir(destinationPath), ".navidrome-backup-*")
	if err != nil {
		return "", 0, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return "", 0, err
	}
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temporary, hasher), source)
	if copyErr != nil {
		_ = temporary.Close()
		return "", 0, copyErr
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return "", 0, err
	}
	if err := temporary.Close(); err != nil {
		return "", 0, err
	}
	if err := replaceFile(temporaryPath, destinationPath); err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hasher.Sum(nil)), written, nil
}
