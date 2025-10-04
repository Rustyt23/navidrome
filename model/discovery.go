package model

import (
	"strconv"
	"time"
)

type Discovery struct {
	ID        string          `structs:"id" json:"id"`
	Name      string          `structs:"name" json:"name"`
	Comment   string          `structs:"comment" json:"comment"`
	Duration  float32         `structs:"duration" json:"duration"`
	Size      int64           `structs:"size" json:"size"`
	SongCount int             `structs:"song_count" json:"songCount"`
	OwnerName string          `structs:"-" json:"ownerName"`
	OwnerID   string          `structs:"owner_id" json:"ownerId"`
	Path      string          `structs:"path" json:"path"`
	CreatedAt time.Time       `structs:"created_at" json:"createdAt"`
	UpdatedAt time.Time       `structs:"updated_at" json:"updatedAt"`
	Tracks    DiscoveryTracks `structs:"-" json:"tracks,omitempty"`
	Type      string          `structs:"-" json:"type,omitempty"`
}

func (d *Discovery) refreshStats() {
	d.SongCount = len(d.Tracks)
	d.Duration = 0
	d.Size = 0
	for _, t := range d.Tracks {
		d.Duration += t.MediaFile.Duration
		d.Size += t.MediaFile.Size
	}
}

func (d *Discovery) SetTracks(tracks DiscoveryTracks) {
	d.Tracks = tracks
	d.refreshStats()
}

func (d *Discovery) AddMediaFiles(mfs MediaFiles) {
	pos := len(d.Tracks)
	for _, mf := range mfs {
		pos++
		t := DiscoveryTrack{
			ID:          strconv.Itoa(pos),
			MediaFileID: mf.ID,
			DiscoveryID: d.ID,
			MediaFile:   mf,
		}
		d.Tracks = append(d.Tracks, t)
	}
	d.refreshStats()
}

type Discoveries []Discovery

type DiscoveryRepository interface {
	ResourceRepository
	CountAll(options ...QueryOptions) (int64, error)
	Exists(id string) (bool, error)
	Put(d *Discovery) error
	Get(id string) (*Discovery, error)
	GetWithTracks(id string) (*Discovery, error)
	GetAll(options ...QueryOptions) (Discoveries, error)
	FindByPath(path string) (*Discovery, error)
	Delete(id string) error
	Tracks(discoveryID string) DiscoveryTrackRepository
	GetDiscoveries(mediaFileId string) (Discoveries, error)
	ReplaceTracks(id string, mediaFileIDs []string) error
}

type DiscoveryTrack struct {
	ID          string `json:"id"`
	MediaFileID string `json:"mediaFileId"`
	DiscoveryID string `json:"discoveryId"`
	MediaFile
}

type DiscoveryTracks []DiscoveryTrack

type DiscoveryTrackRepository interface {
	ResourceRepository
	GetAll(options ...QueryOptions) (DiscoveryTracks, error)
	Delete(id ...string) error
	DeleteAll() error
}
