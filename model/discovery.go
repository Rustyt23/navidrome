package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	MediaFileID string  `json:"mediaFileId"`
}

type DiscoveryTracks []DiscoveryTrack

func (tracks DiscoveryTracks) MediaFiles() MediaFiles {
	files := make(MediaFiles, 0, len(tracks))
	for _, track := range tracks {
		mf := MediaFile{
			ID:       track.MediaFileID,
			Path:     track.Path,
			Title:    track.Title,
			Artist:   track.Artist,
			Album:    track.Album,
			Duration: track.Duration,
			Size:     track.Size,
		}
		if mf.ID == "" {
			mf.ID = track.StreamID()
		}
		if track.Path != "" {
			mf.Suffix = strings.TrimPrefix(strings.ToLower(filepath.Ext(track.Path)), ".")
		}
		files = append(files, mf)
	}
	return files
}

func (d *Discovery) ToM3U8() string {
	if d == nil {
		return ""
	}
	return d.Tracks.MediaFiles().ToM3U8(d.Name, true)
}

type DiscoveryTrackRepository interface {
	ResourceRepository
	GetAll(options ...QueryOptions) (DiscoveryTracks, error)
	Delete(id ...string) error
	DeleteAll() error
}

const discoveryStreamPrefix = "disc"

func DiscoveryStreamID(discoveryID, trackID string) string {
	if discoveryID == "" || trackID == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s:%s", discoveryStreamPrefix, discoveryID, trackID)
}

func ParseDiscoveryStreamID(value string) (string, string, bool) {
	if !strings.HasPrefix(value, discoveryStreamPrefix+":") {
		return "", "", false
	}
	parts := strings.SplitN(value, ":", 3)
	if len(parts) != 3 || parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func (t DiscoveryTrack) StreamID() string {
	if t.MediaFileID != "" {
		return t.MediaFileID
	}
	return DiscoveryStreamID(t.DiscoveryID, t.ID)
}

func (t DiscoveryTrack) ToMediaFile() (*MediaFile, error) {
	if t.Path == "" {
		return nil, fmt.Errorf("discovery track missing path")
	}

	info, err := os.Stat(t.Path)
	if err != nil {
		return nil, err
	}

	suffix := strings.TrimPrefix(strings.ToLower(filepath.Ext(t.Path)), ".")
	mf := &MediaFile{
		ID:          t.StreamID(),
		Path:        t.Path,
		Title:       t.Title,
		Artist:      t.Artist,
		Album:       t.Album,
		Duration:    t.Duration,
		Size:        info.Size(),
		Suffix:      suffix,
		UpdatedAt:   info.ModTime(),
		CreatedAt:   info.ModTime(),
		BirthTime:   info.ModTime(),
		Missing:     false,
		LibraryPath: "",
	}
	if mf.ID == "" {
		mf.ID = DiscoveryStreamID(t.DiscoveryID, t.ID)
	}
	return mf, nil
}
