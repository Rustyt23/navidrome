package model

import "time"

type DiscoveryPlaylist struct {
	ID         string          `structs:"id" json:"id"`
	Name       string          `structs:"name" json:"name"`
	FolderPath string          `structs:"folder_path" json:"folderPath"`
	SongCount  int             `structs:"song_count" json:"songCount"`
	UpdatedAt  time.Time       `structs:"updated_at" json:"updatedAt"`
	Duration   float32         `structs:"-" json:"duration"`
	Size       int64           `structs:"-" json:"size"`
	Sync       bool            `structs:"-" json:"sync"`
	Tracks     DiscoveryTracks `structs:"-" json:"tracks,omitempty"`
}

type DiscoveryPlaylists []DiscoveryPlaylist

type DiscoveryTrack struct {
	ID          string `json:"id"`
	DiscoveryID string `json:"discoveryId"`
	MediaFileID string `json:"mediaFileId"`
	Position    int    `json:"position"`
	MediaFile
}

type DiscoveryTracks []DiscoveryTrack

type DiscoveryPlaylistRepository interface {
	GetAll(options ...QueryOptions) (DiscoveryPlaylists, error)
	Get(id string) (*DiscoveryPlaylist, error)
	ReplaceAll(playlists DiscoveryPlaylists) error
}
