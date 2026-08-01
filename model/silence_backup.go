package model

import "time"

const (
	SilenceBackupStatusReady     = "ready"
	SilenceBackupStatusPrepared  = "prepared"
	SilenceBackupStatusAvailable = "available"
	SilenceBackupStatusRestored  = "restored"
	SilenceIntegrityPending      = "pending"
	SilenceIntegrityPrepared     = "prepared"
	SilenceIntegrityVerified     = "verified"
	SilenceIntegrityRestored     = "restored"
	SilenceIntegrityFailed       = "failed"
)

// SilenceBackup records the independently stored original for a silence trim.
// BackupFile is a basename relative to the dedicated silence backup folder.
type SilenceBackup struct {
	MediaFileID               string     `structs:"media_file_id" json:"-"`
	BackupFile                string     `structs:"backup_file" json:"-"`
	Status                    string     `structs:"status" json:"status"`
	IntegrityStatus           string     `structs:"integrity_status" json:"integrityStatus,omitempty"`
	IntegrityBackupVerified   bool       `structs:"integrity_backup_verified" json:"backupChecksumVerified,omitempty"`
	IntegrityDecodeVerified   bool       `structs:"integrity_decode_verified" json:"decodeVerified,omitempty"`
	IntegrityAudioVerified    bool       `structs:"integrity_audio_verified" json:"audioPropertiesVerified,omitempty"`
	IntegrityMetadataVerified bool       `structs:"integrity_metadata_verified" json:"metadataVerified,omitempty"`
	IntegrityArtworkVerified  bool       `structs:"integrity_artwork_verified" json:"artworkVerified,omitempty"`
	IntegrityPacketVerified   bool       `structs:"integrity_packet_verified" json:"packetIntegrityVerified,omitempty"`
	IntegrityRestoreVerified  bool       `structs:"integrity_restore_verified" json:"restoreVerified,omitempty"`
	IntegrityError            string     `structs:"integrity_error" json:"integrityError,omitempty"`
	IntegrityVerifiedAt       *time.Time `structs:"integrity_verified_at" json:"integrityVerifiedAt,omitempty"`

	OriginalSHA256 string  `structs:"original_sha256" json:"-"`
	TrimmedSHA256  string  `structs:"trimmed_sha256" json:"-"`
	OriginalSize   int64   `structs:"original_size" json:"-"`
	TrimmedSize    int64   `structs:"trimmed_size" json:"-"`
	OriginalMode   uint32  `structs:"original_mode" json:"-"`
	TrimStart      float64 `structs:"trim_start" json:"-"`
	TrimEnd        float64 `structs:"trim_end" json:"-"`

	OriginalModTime time.Time  `structs:"original_mod_time" json:"-"`
	CreatedAt       time.Time  `structs:"created_at" json:"-"`
	PreparedAt      *time.Time `structs:"prepared_at" json:"-"`
	TrimmedAt       *time.Time `structs:"trimmed_at" json:"-"`
	RestoredAt      *time.Time `structs:"restored_at" json:"-"`
}

type SilenceBackupRepository interface {
	Put(backup *SilenceBackup) error
	Get(mediaFileID string) (*SilenceBackup, error)
}
