package model

import "time"

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
		d.Duration += t.Duration
		d.Size += t.Size
	}
}

func (d *Discovery) SetTracks(tracks DiscoveryTracks) {
	d.Tracks = tracks
	d.refreshStats()
}

func (d Discovery) CoverArtID() ArtworkID {
	return NewArtworkID(KindDiscoveryArtwork, d.ID, &d.UpdatedAt)
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
	ReplaceTracks(id string, tracks DiscoveryTracks) error
}

type DiscoveryTrack struct {
	ID          string  `json:"id"`
	DiscoveryID string  `json:"discoveryId"`
	Path        string  `json:"path"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Album       string  `json:"album"`
	Duration    float32 `json:"duration"`
	Size        int64   `json:"size"`
}

type DiscoveryTracks []DiscoveryTrack

type DiscoveryTrackRepository interface {
	ResourceRepository
	GetAll(options ...QueryOptions) (DiscoveryTracks, error)
	Delete(id ...string) error
	DeleteAll() error
}
