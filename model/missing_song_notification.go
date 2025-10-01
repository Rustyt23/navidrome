package model

import "time"

type MissingSongNotification struct {
	MediaFileID   string    `structs:"media_file_id" json:"mediaFileId"`
	SongTitle     string    `structs:"song_title" json:"songTitle"`
	PlaylistNames []string  `structs:"-" json:"playlistNames"`
	DetectedAt    time.Time `structs:"detected_at" json:"detectedAt"`
	MediaFile     MediaFile `structs:"-" json:"mediaFile"`
}

type MissingSongNotifications []MissingSongNotification

type MissingSongNotificationRepository interface {
	CountAll(options ...QueryOptions) (int64, error)
	GetAll(options ...QueryOptions) (MissingSongNotifications, error)

	RefreshForMediaFileIDs(ids ...string) error
	RefreshForFolders(folderIDs ...string) error
	Delete(ids ...string) error
	DeleteByFolders(folderIDs ...string) error
	DeleteAll() error
}
