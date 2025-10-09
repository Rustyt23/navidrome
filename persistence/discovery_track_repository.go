package persistence

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

type discoveryTrackRepository struct {
	sqlRepository
	discoveryID   string
	discoveryRepo *discoveryRepository
}

func (r *discoveryTrackRepository) Count(options ...rest.QueryOptions) (int64, error) {
	qo := r.parseRestOptions(r.ctx, options...)
	sel := Select().Where(Eq{"discovery_tracks.discovery_id": r.discoveryID})
	return r.count(sel, qo)
}

func (r *discoveryTrackRepository) Read(id string) (interface{}, error) {
	sel := r.newSelect().
		Columns("discovery_tracks.*").
		Where(Eq{"discovery_tracks.id": id})
	tracks, err := r.discoveryRepo.loadTracks(sel, r.discoveryID)
	if err != nil {
		return nil, err
	}
	if len(tracks) == 0 {
		return nil, rest.ErrNotFound
	}
	return tracks[0], nil
}

func (r *discoveryTrackRepository) ReadAll(options ...rest.QueryOptions) (interface{}, error) {
	qo := r.parseRestOptions(r.ctx, options...)
	tracks, err := r.GetAll(qo)
	return tracks, err
}

func (r *discoveryTrackRepository) EntityName() string { return "discovery_tracks" }

func (r *discoveryTrackRepository) NewInstance() interface{} { return &model.DiscoveryTrack{} }

func (r *discoveryTrackRepository) GetAll(options ...model.QueryOptions) (model.DiscoveryTracks, error) {
	sel := r.newSelect(options...).
		Columns("discovery_tracks.*").
		Where(Eq{"discovery_tracks.discovery_id": r.discoveryID})
	tracks, err := r.discoveryRepo.loadTracks(sel, r.discoveryID)
	if err != nil {
		return nil, err
	}
	return tracks, nil
}

func (r *discoveryTrackRepository) Delete(id ...string) error {
	if len(id) == 0 {
		return nil
	}

	disc, err := r.discoveryRepo.Get(r.discoveryID)
	if err != nil {
		return err
	}
	usr := loggedUser(r.ctx)
	if !usr.IsAdmin && disc.OwnerID != usr.ID {
		return rest.ErrPermissionDenied
	}

	sel := Select().Where(Eq{"discovery_tracks.id": id})
	tracks, err := r.discoveryRepo.loadTracks(sel, r.discoveryID)
	if err != nil {
		return err
	}
	if len(tracks) == 0 {
		return model.ErrNotFound
	}

	basePath := disc.Path
	if abs, err := filepath.Abs(basePath); err == nil {
		basePath = abs
	}

	for _, track := range tracks {
		if track.Path == "" {
			continue
		}
		target := track.Path
		if abs, err := filepath.Abs(target); err == nil {
			target = abs
		}
		if basePath != "" {
			if rel, err := filepath.Rel(basePath, target); err != nil || strings.HasPrefix(rel, "..") {
				continue
			}
		}
		if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Warn(r.ctx, "Discovery: unable to remove track file", "path", target, err)
		}
	}

	del := Delete("discovery_tracks").
		Where(Eq{"discovery_id": r.discoveryID}).
		Where(Eq{"id": id})
	if _, err := r.executeSQL(del); err != nil {
		return err
	}

	var stats struct {
		Count    int     `db:"count"`
		Duration float64 `db:"duration"`
		Size     int64   `db:"size"`
	}
	selStats := Select("COUNT(*) as count", "COALESCE(SUM(duration),0) as duration", "COALESCE(SUM(size),0) as size").
		From("discovery_tracks").
		Where(Eq{"discovery_id": r.discoveryID})
	if err := r.discoveryRepo.queryOne(selStats, &stats); err != nil {
		if !errors.Is(err, model.ErrNotFound) {
			return err
		}
		stats.Count = 0
		stats.Duration = 0
		stats.Size = 0
	}

	upd := Update("discovery").
		Set("song_count", stats.Count).
		Set("duration", stats.Duration).
		Set("size", stats.Size).
		Set("updated_at", time.Now()).
		Where(Eq{"id": r.discoveryID})
	_, err = r.discoveryRepo.executeSQL(upd)
	return err
}

func (r *discoveryTrackRepository) DeleteAll() error {
	return rest.ErrPermissionDenied
}

var _ model.DiscoveryTrackRepository = (*discoveryTrackRepository)(nil)
var _ rest.Repository = (*discoveryTrackRepository)(nil)
