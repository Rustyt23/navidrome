package model

import "time"

type DiscoveryPlaylist struct {
	ID         string    `structs:"id" json:"id"`
	Name       string    `structs:"name" json:"name"`
	FolderPath string    `structs:"folder_path" json:"folderPath"`
	SongCount  int       `structs:"song_count" json:"songCount"`
	UpdatedAt  time.Time `structs:"updated_at" json:"updatedAt"`
	Tracks     []string  `structs:"-" json:"tracks,omitempty"`
}

type DiscoveryPlaylists []DiscoveryPlaylist

type DiscoveryPlaylistRepository interface {
	GetAll(options ...QueryOptions) (DiscoveryPlaylists, error)
	Get(id string) (*DiscoveryPlaylist, error)
	ReplaceAll(playlists DiscoveryPlaylists) error
}
