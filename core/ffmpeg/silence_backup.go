package ffmpeg

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const SilenceTrimBackupFolderName = "original_backup before silence trimming"

func SilenceTrimBackupRoot(backupFolder, libraryPath string) string {
	if folder := strings.TrimSpace(backupFolder); folder != "" {
		return filepath.Join(filepath.Clean(folder), SilenceTrimBackupFolderName)
	}
	if strings.TrimSpace(libraryPath) == "" {
		return ""
	}
	lib := filepath.Clean(libraryPath)
	return filepath.Join(filepath.Dir(lib), SilenceTrimBackupFolderName)
}

// SilenceTrimBackupPath is ID-only, so moving or renaming a song cannot orphan
// the one copy that makes the trim reversible.
func SilenceTrimBackupPath(backupFolder, libraryPath, mediaFileID string) string {
	root := SilenceTrimBackupRoot(backupFolder, libraryPath)
	id := backupIDComponent(mediaFileID)
	if root == "" || id == "" {
		return ""
	}
	return filepath.Join(root, id[:2], id+".original")
}

func SilenceTrimBackupHashPath(backupFolder, libraryPath, mediaFileID string) string {
	path := SilenceTrimBackupPath(backupFolder, libraryPath, mediaFileID)
	if path == "" {
		return ""
	}
	return path + ".sha256"
}

// Rotated generations are immutable and keyed by their content hash. The
// original ID-only path remains readable for backward compatibility and as the
// first generation, while later generations never overwrite it.
func SilenceTrimBackupGenerationPath(
	backupFolder string,
	libraryPath string,
	mediaFileID string,
	sha256Value string,
) string {
	base := SilenceTrimBackupPath(backupFolder, libraryPath, mediaFileID)
	value := strings.ToLower(strings.TrimSpace(sha256Value))
	if base == "" || len(value) != sha256.Size*2 {
		return ""
	}
	if _, err := hex.DecodeString(value); err != nil {
		return ""
	}
	return base + "." + value
}

func silenceTrimBackupGenerationHashPath(
	backupFolder string,
	libraryPath string,
	mediaFileID string,
	sha256Value string,
) string {
	path := SilenceTrimBackupGenerationPath(
		backupFolder,
		libraryPath,
		mediaFileID,
		sha256Value,
	)
	if path == "" {
		return ""
	}
	return path + ".sha256"
}

func FindSilenceTrimBackup(backupFolder, libraryPath, mediaFileID string) string {
	path := SilenceTrimBackupPath(backupFolder, libraryPath, mediaFileID)
	if path != "" && fileExists(path) == nil {
		return path
	}
	return ""
}

func FileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// BackupSilenceTrimOriginal stores and verifies the exact pre-trim file. It
// returns the immutable backup's SHA-256.
func BackupSilenceTrimOriginal(
	trackPath string,
	mode os.FileMode,
	libraryPath string,
	backupFolder string,
	mediaFileID string,
) (string, error) {
	dest := SilenceTrimBackupPath(backupFolder, libraryPath, mediaFileID)
	if dest == "" {
		return "", fmt.Errorf("no silence-trim backup location")
	}
	if existing := FindSilenceTrimBackup(backupFolder, libraryPath, mediaFileID); existing != "" {
		backupHash, err := FileSHA256(existing)
		if err != nil {
			return "", err
		}
		sourceHash, err := FileSHA256(trackPath)
		if err != nil {
			return "", err
		}
		if !strings.EqualFold(sourceHash, backupHash) {
			return "", fmt.Errorf("stored silence-trim backup belongs to a different file generation")
		}
		checksumPath := SilenceTrimBackupHashPath(backupFolder, libraryPath, mediaFileID)
		if expected, err := readSilenceBackupChecksum(checksumPath); err == nil {
			if !strings.EqualFold(expected, backupHash) {
				// The current source independently proves that the backup bytes
				// are correct, so an interrupted checksum rotation can be
				// repaired without trusting either stale checksum.
				if err := writeSilenceBackupChecksum(checksumPath, backupHash); err != nil {
					return "", err
				}
			}
			return backupHash, nil
		}
		// A checksum sidecar should always exist. It may only be rebuilt when
		// the current track proves the backup is still an exact copy.
		if err := writeSilenceBackupChecksum(checksumPath, backupHash); err != nil {
			return "", err
		}
		return backupHash, nil
	}
	sourceHash, err := FileSHA256(trackPath)
	if err != nil {
		return "", fmt.Errorf("hashing source before backup: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("creating silence-trim backup folder: %w", err)
	}
	if err := copyFileWithMode(trackPath, dest, mode); err != nil {
		return "", err
	}
	backupHash, err := FileSHA256(dest)
	if err != nil {
		return "", fmt.Errorf("hashing completed silence-trim backup: %w", err)
	}
	if sourceHash != backupHash {
		return "", fmt.Errorf("silence-trim backup hash does not match the source")
	}
	if err := writeSilenceBackupChecksum(
		SilenceTrimBackupHashPath(backupFolder, libraryPath, mediaFileID),
		backupHash,
	); err != nil {
		return "", fmt.Errorf("storing silence-trim backup checksum: %w", err)
	}
	return backupHash, nil
}

// RotateSilenceTrimOriginal starts a new reversible generation after an older
// silence trim was restored and another legitimate workflow changed the song.
// Generations are immutable: the previous backup is verified and retained,
// while the new source is written under its SHA-256 name.
func RotateSilenceTrimOriginal(
	trackPath string,
	mode os.FileMode,
	libraryPath string,
	backupFolder string,
	mediaFileID string,
	expectedExistingSHA256 string,
) (string, error) {
	_, existingHash, err := VerifySilenceTrimBackupGeneration(
		backupFolder,
		libraryPath,
		mediaFileID,
		expectedExistingSHA256,
	)
	if err != nil {
		return "", fmt.Errorf("verifying previous silence-trim backup: %w", err)
	}
	if expectedExistingSHA256 == "" ||
		!strings.EqualFold(existingHash, expectedExistingSHA256) {
		return "", fmt.Errorf("previous silence-trim backup generation does not match its audit")
	}
	sourceHash, err := FileSHA256(trackPath)
	if err != nil {
		return "", fmt.Errorf("hashing replacement backup source: %w", err)
	}
	if strings.EqualFold(sourceHash, existingHash) {
		return existingHash, nil
	}
	dest := SilenceTrimBackupGenerationPath(
		backupFolder,
		libraryPath,
		mediaFileID,
		sourceHash,
	)
	checksumPath := silenceTrimBackupGenerationHashPath(
		backupFolder,
		libraryPath,
		mediaFileID,
		sourceHash,
	)
	if dest == "" || checksumPath == "" {
		return "", fmt.Errorf("no rotated silence-trim backup location")
	}
	needsCopy := true
	if hash, err := FileSHA256(dest); err == nil && strings.EqualFold(hash, sourceHash) {
		needsCopy = false
	}
	if needsCopy {
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return "", fmt.Errorf("creating rotated silence-trim backup folder: %w", err)
		}
		if err := copyFileWithMode(trackPath, dest, mode); err != nil {
			return "", fmt.Errorf("rotating silence-trim backup: %w", err)
		}
	}
	backupHash, err := FileSHA256(dest)
	if err != nil {
		return "", fmt.Errorf("hashing rotated silence-trim backup: %w", err)
	}
	if !strings.EqualFold(sourceHash, backupHash) {
		return "", fmt.Errorf("rotated silence-trim backup hash does not match the source")
	}
	if err := writeSilenceBackupChecksum(
		checksumPath,
		backupHash,
	); err != nil {
		return "", fmt.Errorf("storing rotated silence-trim backup checksum: %w", err)
	}
	return backupHash, nil
}

func VerifySilenceTrimBackup(
	backupFolder string,
	libraryPath string,
	mediaFileID string,
) (string, error) {
	_, hash, err := VerifySilenceTrimBackupGeneration(
		backupFolder,
		libraryPath,
		mediaFileID,
		"",
	)
	return hash, err
}

func FindSilenceTrimBackupGeneration(
	backupFolder string,
	libraryPath string,
	mediaFileID string,
	expectedSHA256 string,
) string {
	if expectedSHA256 != "" {
		path := SilenceTrimBackupGenerationPath(
			backupFolder,
			libraryPath,
			mediaFileID,
			expectedSHA256,
		)
		if path != "" && fileExists(path) == nil {
			return path
		}
	}
	return FindSilenceTrimBackup(backupFolder, libraryPath, mediaFileID)
}

func VerifySilenceTrimBackupGeneration(
	backupFolder string,
	libraryPath string,
	mediaFileID string,
	expectedSHA256 string,
) (string, string, error) {
	backup := FindSilenceTrimBackupGeneration(
		backupFolder,
		libraryPath,
		mediaFileID,
		expectedSHA256,
	)
	if backup == "" {
		return "", "", fmt.Errorf("no stored pre-trim original")
	}
	hash, err := FileSHA256(backup)
	if err != nil {
		return "", "", err
	}
	if expectedSHA256 != "" && !strings.EqualFold(hash, expectedSHA256) {
		return "", "", fmt.Errorf("stored pre-trim original failed its SHA-256 check")
	}
	checksumPath := SilenceTrimBackupHashPath(backupFolder, libraryPath, mediaFileID)
	if generationPath := SilenceTrimBackupGenerationPath(
		backupFolder,
		libraryPath,
		mediaFileID,
		hash,
	); filepath.Clean(backup) == filepath.Clean(generationPath) {
		checksumPath = silenceTrimBackupGenerationHashPath(
			backupFolder,
			libraryPath,
			mediaFileID,
			hash,
		)
	}
	sidecarHash, err := readSilenceBackupChecksum(
		checksumPath,
	)
	if err != nil {
		return "", "", fmt.Errorf("stored pre-trim original has no readable SHA-256 record: %w", err)
	}
	if !strings.EqualFold(hash, sidecarHash) {
		return "", "", fmt.Errorf("stored pre-trim original failed its saved SHA-256 check")
	}
	return backup, hash, nil
}

func RestoreSilenceTrimOriginal(
	trackPath string,
	libraryPath string,
	backupFolder string,
	mediaFileID string,
	expectedSHA256 string,
	expectedCurrentSHA256 string,
) error {
	backup, hash, err := VerifySilenceTrimBackupGeneration(
		backupFolder,
		libraryPath,
		mediaFileID,
		expectedSHA256,
	)
	if err != nil {
		return err
	}
	if expectedSHA256 != "" && !strings.EqualFold(hash, expectedSHA256) {
		return fmt.Errorf("stored pre-trim original failed its SHA-256 check")
	}
	if expectedCurrentSHA256 == "" {
		return fmt.Errorf("no verified silence-trim result generation was supplied")
	}
	currentHash, err := FileSHA256(trackPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(currentHash, expectedCurrentSHA256) {
		return fmt.Errorf("current song is not the verified silence-trim result")
	}
	mode := os.FileMode(0o644)
	if stat, err := os.Stat(trackPath); err == nil {
		mode = stat.Mode()
	} else if !os.IsNotExist(err) {
		return err
	}
	return copyFileWithMode(backup, trackPath, mode)
}

func readSilenceBackupChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(data))
	if len(value) != sha256.Size*2 {
		return "", fmt.Errorf("invalid SHA-256 record")
	}
	return value, nil
}

func writeSilenceBackupChecksum(path, hash string) error {
	if path == "" {
		return fmt.Errorf("no checksum path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".checksum-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	complete := false
	defer func() {
		if !complete {
			_ = os.Remove(temp)
		}
	}()
	if _, err := file.WriteString(strings.ToLower(hash) + "\n"); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temp, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temp, path); err != nil {
		return err
	}
	complete = true
	return nil
}
