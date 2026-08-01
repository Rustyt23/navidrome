package nativeapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	coresilence "github.com/navidrome/navidrome/core/silence"
	"github.com/navidrome/navidrome/model"
)

type silenceIntegrityChecks struct {
	BackupSHA256         bool `json:"backupSha256"`
	CurrentTrimmedSHA256 bool `json:"currentTrimmedSha256"`
	Decode               bool `json:"decode"`
	AudioProperties      bool `json:"audioProperties"`
	Metadata             bool `json:"metadata"`
	Artwork              bool `json:"artwork"`
	AudioPackets         bool `json:"audioPackets"`
	RestoreSHA256        bool `json:"restoreSha256"`
}

type silenceIntegrityReport struct {
	MediaFileID    string                 `json:"mediaFileId"`
	Title          string                 `json:"title"`
	Artist         string                 `json:"artist"`
	Path           string                 `json:"path"`
	Status         string                 `json:"status"`
	BackupStatus   string                 `json:"backupStatus"`
	CurrentState   string                 `json:"currentState"`
	BackupFile     string                 `json:"backupFile"`
	OriginalSHA256 string                 `json:"originalSha256"`
	TrimmedSHA256  string                 `json:"trimmedSha256"`
	CurrentSHA256  string                 `json:"currentSha256"`
	OriginalSize   int64                  `json:"originalSize"`
	TrimmedSize    int64                  `json:"trimmedSize"`
	Checks         silenceIntegrityChecks `json:"checks"`
	VerifiedAt     string                 `json:"verifiedAt,omitempty"`
	Error          string                 `json:"error,omitempty"`
}

func (n *Router) silenceIntegrityHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}
		mediaFile, err := n.ds.MediaFile(r.Context()).Get(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		backup := mediaFile.SilenceBackup
		if backup == nil {
			http.Error(w, "no silence trim backup exists for this song", http.StatusNotFound)
			return
		}
		sourcePath, err := secureSilenceMediaPath(mediaFile.LibraryPath, mediaFile.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		trimmer := newSilenceTrimmer(mediaFile.LibraryPath)
		report := silenceIntegrityReport{
			MediaFileID: id, Title: mediaFile.Title, Artist: mediaFile.Artist, Path: mediaFile.Path,
			BackupStatus: backup.Status, BackupFile: backup.BackupFile,
			OriginalSHA256: backup.OriginalSHA256, TrimmedSHA256: backup.TrimmedSHA256,
			OriginalSize: backup.OriginalSize, TrimmedSize: backup.TrimmedSize,
			Checks: silenceIntegrityChecks{
				Decode:          backup.IntegrityDecodeVerified,
				AudioProperties: backup.IntegrityAudioVerified,
				Metadata:        backup.IntegrityMetadataVerified,
				Artwork:         backup.IntegrityArtworkVerified,
				AudioPackets:    backup.IntegrityPacketVerified,
				RestoreSHA256:   backup.IntegrityRestoreVerified,
			},
		}
		if backup.IntegrityVerifiedAt != nil {
			report.VerifiedAt = backup.IntegrityVerifiedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		}
		if err := trimmer.VerifyBackup(backup.BackupFile, backup.OriginalSHA256, backup.OriginalSize); err == nil {
			report.Checks.BackupSHA256 = true
		} else {
			report.Error = err.Error()
		}
		if currentHash, hashErr := coresilence.HashFile(sourcePath); hashErr == nil {
			report.CurrentSHA256 = currentHash
			switch classifySilenceBackupFile(backup, currentHash) {
			case silenceFileIsTrimmed:
				report.CurrentState = "trimmed"
				report.Checks.CurrentTrimmedSHA256 = true
				report.Status = model.SilenceIntegrityVerified
			case silenceFileIsOriginal:
				report.CurrentState = "original"
				report.Checks.RestoreSHA256 = currentHash == backup.OriginalSHA256
				report.Status = model.SilenceIntegrityRestored
			default:
				report.CurrentState = "unknown"
				report.Status = model.SilenceIntegrityFailed
				report.Error = "current song matches neither the original nor the validated trim"
			}
		} else if report.Error == "" {
			report.Error = hashErr.Error()
		}
		if report.Status == "" {
			report.Status = model.SilenceIntegrityFailed
		}
		if report.Status == model.SilenceIntegrityVerified &&
			(!report.Checks.BackupSHA256 || !report.Checks.CurrentTrimmedSHA256 ||
				!report.Checks.Decode || !report.Checks.AudioProperties || !report.Checks.Metadata ||
				!report.Checks.Artwork || !report.Checks.AudioPackets) {
			report.Status = model.SilenceIntegrityFailed
			if report.Error == "" {
				report.Error = "one or more recorded integrity checks did not pass"
			}
		}
		if report.Status == model.SilenceIntegrityVerified || report.Status == model.SilenceIntegrityRestored {
			verifiedAt := time.Now()
			backup.IntegrityStatus = report.Status
			backup.IntegrityBackupVerified = report.Checks.BackupSHA256
			backup.IntegrityRestoreVerified = report.Checks.RestoreSHA256
			backup.IntegrityVerifiedAt = &verifiedAt
			backup.IntegrityError = ""
			_ = n.ds.SilenceBackup(r.Context()).Put(backup)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(report)
	}
}
