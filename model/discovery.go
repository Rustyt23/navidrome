package model

import (
	"slices"
	"strconv"
	"time"

	"github.com/navidrome/navidrome/model/criteria"
)

type Discovery struct {
	ID        string         `structs:"id" json:"id"`
	Name      string         `structs:"name" json:"name"`
	Comment   string         `structs:"comment" json:"comment"`
	Duration  float32        `structs:"duration" json:"duration"`
	Size      int64          `structs:"size" json:"size"`
	SongCount int            `structs:"song_count" json:"songCount"`
	OwnerName string         `structs:"-" json:"ownerName"`
	OwnerID   string         `structs:"owner_id" json:"ownerId"`
	FolderID  *string        `structs:"folder_id" json:"folderId,omitempty"`
	Public    bool           `structs:"public" json:"public"`
	Tracks    DiscoverySongs `structs:"-" json:"tracks,omitempty"`
	Path      string         `structs:"path" json:"path"`
	Sync      bool           `structs:"sync" json:"sync"`
	CreatedAt time.Time      `structs:"created_at" json:"createdAt"`
	UpdatedAt time.Time      `structs:"updated_at" json:"updatedAt"`

	Type string `structs:"-" json:"type,omitempty"`

	Rules       *criteria.Criteria `structs:"rules" json:"rules"`
	EvaluatedAt *time.Time         `structs:"evaluated_at" json:"evaluatedAt"`
}

type Discoveries []Discovery

type DiscoverySong struct {
	ID          string `json:"id"`
	MediaFileID string `json:"mediaFileId"`
	DiscoveryID string `json:"discoveryId"`
	MediaFile
}

type DiscoverySongs []DiscoverySong

func (d Discovery) IsSmartDiscovery() bool {
	return d.Rules != nil && d.Rules.Expression != nil
}

func (d Discovery) MediaFiles() MediaFiles {
	if len(d.Tracks) == 0 {
		return nil
	}
	return d.Tracks.MediaFiles()
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

func (d *Discovery) SetTracks(tracks DiscoverySongs) {
	d.Tracks = tracks
	d.refreshStats()
}

func (d *Discovery) RemoveTracks(idxToRemove []int) {
	var newTracks DiscoverySongs
	for i, t := range d.Tracks {
		if slices.Contains(idxToRemove, i) {
			continue
		}
		newTracks = append(newTracks, t)
	}
	d.Tracks = newTracks
	d.refreshStats()
}

func (d *Discovery) AddMediaFilesByID(mediaFileIds []string) {
	pos := len(d.Tracks)
	for _, mfId := range mediaFileIds {
		pos++
		t := DiscoverySong{
			ID:          strconv.Itoa(pos),
			MediaFileID: mfId,
			MediaFile:   MediaFile{ID: mfId},
			DiscoveryID: d.ID,
		}
		d.Tracks = append(d.Tracks, t)
	}
	d.refreshStats()
}

func (d *Discovery) AddMediaFiles(mfs MediaFiles) {
	pos := len(d.Tracks)
	for _, mf := range mfs {
		pos++
		t := DiscoverySong{
			ID:          strconv.Itoa(pos),
			MediaFileID: mf.ID,
			MediaFile:   mf,
			DiscoveryID: d.ID,
		}
		d.Tracks = append(d.Tracks, t)
	}
	d.refreshStats()
}

func (ds DiscoverySongs) MediaFiles() MediaFiles {
	mfs := make(MediaFiles, len(ds))
	for i, t := range ds {
		mfs[i] = t.MediaFile
	}
	return mfs
}
