package persistence

import (
	. "github.com/Masterminds/squirrel"
	"github.com/deluan/rest"
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
		Where(And{Eq{"discovery_tracks.discovery_id": r.discoveryID}, Eq{"discovery_tracks.id": id}})
	var track model.DiscoveryTrack
	err := r.queryOne(sel, &track)
	return track, err
}

func (r *discoveryTrackRepository) ReadAll(options ...rest.QueryOptions) (interface{}, error) {
	qo := r.parseRestOptions(r.ctx, options...)
	tracks, err := r.GetAll(qo)
	return tracks, err
}

func (r *discoveryTrackRepository) EntityName() string { return "discovery_tracks" }

func (r *discoveryTrackRepository) NewInstance() interface{} { return &model.DiscoveryTrack{} }

func (r *discoveryTrackRepository) Save(entity interface{}) (string, error) {
	return "", rest.ErrNotImplemented
}

func (r *discoveryTrackRepository) Update(id string, entity interface{}, cols ...string) error {
	return rest.ErrNotImplemented
}

func (r *discoveryTrackRepository) GetAll(options ...model.QueryOptions) (model.DiscoveryTracks, error) {
	sel := r.newSelect(options...).
		Columns("discovery_tracks.*").
		Where(Eq{"discovery_tracks.discovery_id": r.discoveryID})
	tracks, err := r.discoveryRepo.loadTracks(sel)
	if err != nil {
		return nil, err
	}
	return tracks, nil
}

func (r *discoveryTrackRepository) Delete(id ...string) error {
	return rest.ErrNotImplemented
}

func (r *discoveryTrackRepository) DeleteAll() error {
	return rest.ErrNotImplemented
}

var _ model.DiscoveryTrackRepository = (*discoveryTrackRepository)(nil)
var _ rest.Repository = (*discoveryTrackRepository)(nil)
