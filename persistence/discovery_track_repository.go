package persistence

import (
	"context"
	"encoding/json"
	"time"

	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

type discoveryTrackRepository struct {
	ctx context.Context
	db  dbx.Builder
}

type discoveryTrackRow struct {
	ID          string    `db:"id"`
	DiscoveryID string    `db:"discovery_id"`
	MediaFileID *string   `db:"media_file_id"`
	Path        string    `db:"path"`
	Position    int       `db:"position"`
	Missing     bool      `db:"missing"`
	MediaFile   string    `db:"media_file"`
	SourcePath  *string   `db:"source_path"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func NewDiscoveryTrackRepository(ctx context.Context, db dbx.Builder) model.DiscoveryTrackRepository {
	return &discoveryTrackRepository{ctx: ctx, db: db}
}

func (r *discoveryTrackRepository) rowToModel(row discoveryTrackRow) model.DiscoveryTrack {
	var mediaFile model.MediaFile
	if err := json.Unmarshal([]byte(row.MediaFile), &mediaFile); err != nil {
		log.Warn(r.ctx, "Failed to unmarshal discovery track mediafile", "trackID", row.ID, err)
		mediaFile = model.MediaFile{ID: row.ID, Path: row.Path, Missing: row.Missing}
	}
	track := model.DiscoveryTrack{
		ID:          row.ID,
		DiscoveryID: row.DiscoveryID,
		MediaFileID: "",
		Position:    row.Position,
		MediaFile:   mediaFile,
	}
	if row.MediaFileID != nil {
		track.MediaFileID = *row.MediaFileID
	}
	if row.SourcePath != nil {
		track.SourcePath = *row.SourcePath
	}
	return track
}

func (r *discoveryTrackRepository) Get(id string) (*model.DiscoveryTrack, error) {
	var row discoveryTrackRow
	err := r.db.Select("*").From("discovery_tracks").Where(dbx.HashExp{"id": id}).One(&row)
	if err != nil {
		return nil, err
	}
	track := r.rowToModel(row)
	return &track, nil
}

func (r *discoveryTrackRepository) GetByDiscovery(discoveryID string) (model.DiscoveryTracks, error) {
	var rows []discoveryTrackRow
	err := r.db.Select("*").From("discovery_tracks").Where(dbx.HashExp{"discovery_id": discoveryID}).OrderBy("position asc").All(&rows)
	if err != nil {
		return nil, err
	}
	tracks := make(model.DiscoveryTracks, 0, len(rows))
	for _, row := range rows {
		track := r.rowToModel(row)
		tracks = append(tracks, track)
	}
	return tracks, nil
}

func (r *discoveryTrackRepository) GetByIDs(discoveryID string, ids []string) (model.DiscoveryTracks, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []discoveryTrackRow
	err := r.db.Select("*").From("discovery_tracks").Where(dbx.And(
		dbx.HashExp{"discovery_id": discoveryID},
		dbx.NewExp("id in {:ids*}", dbx.Params{"ids": ids}),
	)).OrderBy("position asc").All(&rows)
	if err != nil {
		return nil, err
	}
	tracks := make(model.DiscoveryTracks, 0, len(rows))
	for _, row := range rows {
		track := r.rowToModel(row)
		tracks = append(tracks, track)
	}
	return tracks, nil
}

func (r *discoveryTrackRepository) ReplaceForDiscovery(discoveryID string, tracks model.DiscoveryTracks) error {
	if _, err := r.db.NewQuery("delete from discovery_tracks where discovery_id = {:discovery_id}").Bind(dbx.Params{"discovery_id": discoveryID}).Execute(); err != nil {
		return err
	}
	if len(tracks) == 0 {
		return nil
	}
	now := time.Now().UTC()
	for _, track := range tracks {
		mediaPayload, err := json.Marshal(track.MediaFile)
		if err != nil {
			log.Error(r.ctx, "Error marshaling discovery track mediafile", "trackID", track.ID, err)
			return err
		}
		params := dbx.Params{
			"id":           track.ID,
			"discovery_id": discoveryID,
			"media_file_id": func() interface{} {
				if track.MediaFileID == "" {
					return nil
				}
				return track.MediaFileID
			}(),
			"path":       track.MediaFile.Path,
			"position":   track.Position,
			"missing":    track.MediaFile.Missing,
			"media_file": string(mediaPayload),
			"source_path": func() interface{} {
				if track.SourcePath == "" {
					return nil
				}
				return track.SourcePath
			}(),
			"created_at": now,
			"updated_at": now,
		}
		if _, err := r.db.Insert("discovery_tracks", params).Execute(); err != nil {
			log.Error(r.ctx, "Error inserting discovery track", "trackID", track.ID, err)
			return err
		}
	}
	return nil
}
