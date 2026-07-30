package silencetrim

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"
)

const generationRecordVersion = 1

var persistGenerationRecord = WriteGenerationRecord
var ErrGenerationDiverged = errors.New(
	"current song matches neither side of its durable silence-trim generation",
)

type GenerationState string

const (
	GenerationStateNone   GenerationState = ""
	GenerationStateSource GenerationState = "source"
	GenerationStateResult GenerationState = "result"
)

type generationRecord struct {
	Version int                    `json:"version"`
	Audit   model.SilenceTrimAudit `json:"audit"`
}

// HasExplicitInactiveGeneration is the only database state that can safely
// ignore an unreadable/stale journal: a verified backup is recorded and no
// trimmed result is active. Nil, dry-run, or active-result audits fail closed.
func HasExplicitInactiveGeneration(audit *model.SilenceTrimAudit) bool {
	return audit != nil &&
		audit.HasBackup &&
		audit.BackupSHA256 != "" &&
		audit.SourceSHA256 != "" &&
		strings.EqualFold(audit.SourceSHA256, audit.BackupSHA256) &&
		audit.ResultSHA256 == ""
}

// CanIgnoreGenerationError allows recovery only when the database proves an
// inactive generation. A corrupt journal additionally requires the current
// bytes to be that exact source. A fully verified but diverged older journal
// can be superseded after a legitimate later workflow changed the restored
// source.
func CanIgnoreGenerationError(
	err error,
	audit *model.SilenceTrimAudit,
	trackPath string,
) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrGenerationDiverged) {
		return audit != nil &&
			audit.HasBackup &&
			audit.BackupSHA256 != "" &&
			audit.ResultSHA256 == ""
	}
	if !HasExplicitInactiveGeneration(audit) {
		return false
	}
	currentHash, hashErr := ffmpeg.FileSHA256(trackPath)
	return hashErr == nil && strings.EqualFold(currentHash, audit.SourceSHA256)
}

// GenerationRecordPath is stored beside the separate silence-trim backup. It
// is written before the verified candidate replaces the song, closing the
// process-crash window between the filesystem rename and the database write.
func GenerationRecordPath(backupFolder, libraryPath, mediaFileID string) string {
	backup := ffmpeg.SilenceTrimBackupPath(backupFolder, libraryPath, mediaFileID)
	if backup == "" {
		return ""
	}
	return backup + ".generation.json"
}

func WriteGenerationRecord(
	backupFolder string,
	libraryPath string,
	audit *model.SilenceTrimAudit,
) error {
	if audit == nil ||
		audit.MediaFileID == "" ||
		!audit.HasBackup ||
		audit.SourceSHA256 == "" ||
		audit.BackupSHA256 == "" ||
		audit.ResultSHA256 == "" ||
		!strings.EqualFold(audit.SourceSHA256, audit.BackupSHA256) {
		return fmt.Errorf("silence-trim generation proof is incomplete")
	}
	path := GenerationRecordPath(backupFolder, libraryPath, audit.MediaFileID)
	if path == "" {
		return fmt.Errorf("no silence-trim generation record location")
	}
	data, err := json.MarshalIndent(generationRecord{
		Version: generationRecordVersion,
		Audit:   *audit,
	}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".generation-*")
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
	if _, err := file.Write(data); err != nil {
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
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	complete = true
	return nil
}

// ReconcileGeneration identifies which exact side of the durable before/after
// proof is currently installed. A matching result can reconstruct an audit
// after a process crash; a matching source identifies an already-restored or
// not-yet-installed generation.
func ReconcileGeneration(
	trackPath string,
	libraryPath string,
	backupFolder string,
	mediaFileID string,
) (*model.SilenceTrimAudit, GenerationState, error) {
	path := GenerationRecordPath(backupFolder, libraryPath, mediaFileID)
	if path == "" {
		return nil, GenerationStateNone, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, GenerationStateNone, nil
	}
	if err != nil {
		return nil, GenerationStateNone, err
	}
	var record generationRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, GenerationStateNone, fmt.Errorf("reading silence-trim generation record: %w", err)
	}
	audit := &record.Audit
	if record.Version != generationRecordVersion ||
		audit.MediaFileID != mediaFileID ||
		!audit.HasBackup ||
		audit.SourceSHA256 == "" ||
		audit.BackupSHA256 == "" ||
		audit.ResultSHA256 == "" ||
		!strings.EqualFold(audit.SourceSHA256, audit.BackupSHA256) {
		return nil, GenerationStateNone, fmt.Errorf("silence-trim generation record is incomplete")
	}
	_, backupHash, err := ffmpeg.VerifySilenceTrimBackupGeneration(
		backupFolder,
		libraryPath,
		mediaFileID,
		audit.BackupSHA256,
	)
	if err != nil {
		return nil, GenerationStateNone, err
	}
	if !strings.EqualFold(backupHash, audit.BackupSHA256) {
		return nil, GenerationStateNone, fmt.Errorf("generation record does not match the verified backup")
	}
	currentHash, err := ffmpeg.FileSHA256(trackPath)
	if err != nil {
		return nil, GenerationStateNone, err
	}
	switch {
	case strings.EqualFold(currentHash, audit.ResultSHA256):
		stat, err := os.Stat(trackPath)
		if err != nil {
			return nil, GenerationStateNone, err
		}
		modified := stat.ModTime()
		audit.Status = model.SilenceTrimStatusProcessed
		audit.Integrity = model.SilenceTrimIntegrityVerified
		audit.HasBackup = true
		audit.SizeAfter = stat.Size()
		audit.ResultModifiedAt = &modified
		audit.Error = ""
		return audit, GenerationStateResult, nil
	case strings.EqualFold(currentHash, audit.SourceSHA256):
		return audit, GenerationStateSource, nil
	default:
		return audit, GenerationStateNone, ErrGenerationDiverged
	}
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
