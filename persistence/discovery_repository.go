package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

type discoveryRepository struct {
	sqlRepository
}

type dbDiscoveryTrack struct {
	dbMediaFile
	*model.DiscoveryTrack `structs:",flatten"`
}

func (t *dbDiscoveryTrack) PostScan() error {
	if err := t.dbMediaFile.PostScan(); err != nil {
		return err
	}
	t.DiscoveryTrack.MediaFile = *t.dbMediaFile.MediaFile
	t.DiscoveryTrack.MediaFile.ID = t.MediaFileID
	return nil
}

type dbDiscoveryTracks []dbDiscoveryTrack

func (t dbDiscoveryTracks) toModels() model.DiscoveryTracks {
	tracks := make(model.DiscoveryTracks, len(t))
	for i := range t {
		tracks[i] = *t[i].DiscoveryTrack
	}
	return tracks
}

func NewDiscoveryRepository(ctx context.Context, db dbx.Builder) model.DiscoveryRepository {
	r := &discoveryRepository{}
	r.ctx = ctx
	r.db = db
	r.registerModel(&model.Discovery{}, map[string]filterFunc{
		"q": discoveryFilter,
	})
	r.setSortMappings(map[string]string{
		"owner_name": "owner_name",
	})
	return r
}

func discoveryFilter(_ string, value any) Sqlizer {
	return substringFilter("discovery.name", value)
}

func (r *discoveryRepository) userFilter() Sqlizer {
	user := loggedUser(r.ctx)
	if user.IsAdmin {
		return And{}
	}
	return Eq{"owner_id": user.ID}
}

func (r *discoveryRepository) CountAll(options ...model.QueryOptions) (int64, error) {
	sel := Select().Where(r.userFilter())
	return r.count(sel, options...)
}

func (r *discoveryRepository) Count(options ...rest.QueryOptions) (int64, error) {
	return r.CountAll(r.parseRestOptions(r.ctx, options...))
}

func (r *discoveryRepository) Exists(id string) (bool, error) {
	return r.exists(And{Eq{"discovery.id": id}, r.userFilter()})
}

func (r *discoveryRepository) Put(d *model.Discovery) error {
	usr := loggedUser(r.ctx)
	if !usr.IsAdmin && usr.ID != d.OwnerID {
		return rest.ErrPermissionDenied
	}
	if d.ID == "" {
		d.CreatedAt = time.Now()
	}
	d.UpdatedAt = time.Now()
	id, err := r.put(d.ID, d)
	if err != nil {
		return err
	}
	d.ID = id
	d.Type = "discovery"
	return nil
}

func (r *discoveryRepository) Get(id string) (*model.Discovery, error) {
	return r.findBy(And{Eq{"discovery.id": id}, r.userFilter()})
}

func (r *discoveryRepository) Read(id string) (interface{}, error) {
	return r.Get(id)
}

func (r *discoveryRepository) GetWithTracks(id string) (*model.Discovery, error) {
	disc, err := r.Get(id)
	if err != nil {
		return nil, err
	}
	tracks, err := r.loadTracks(
		r.newSelect().Columns("discovery_tracks.*").Where(Eq{"discovery_tracks.discovery_id": id}),
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("loading discovery tracks: %w", err)
	}
	disc.SetTracks(tracks)
	return disc, nil
}

func (r *discoveryRepository) GetAll(options ...model.QueryOptions) (model.Discoveries, error) {
	sel := r.selectDiscovery(options...).Where(r.userFilter())
	var res []struct {
		model.Discovery
		OwnerName string `db:"owner_name"`
	}
	if err := r.queryAll(sel, &res); err != nil {
		return nil, err
	}
	out := make(model.Discoveries, len(res))
	for i := range res {
		res[i].Discovery.OwnerName = res[i].OwnerName
		res[i].Discovery.Type = "discovery"
		out[i] = res[i].Discovery
	}
	return out, nil
}

func (r *discoveryRepository) ReadAll(options ...rest.QueryOptions) (interface{}, error) {
	return r.GetAll(r.parseRestOptions(r.ctx, options...))
}

func (r *discoveryRepository) FindByPath(path string) (*model.Discovery, error) {
	return r.findBy(And{Eq{"discovery.path": path}, r.userFilter()})
}

func (r *discoveryRepository) Delete(id string) error {
	disc, err := r.Get(id)
	if err != nil {
		return err
	}
	usr := loggedUser(r.ctx)
	if !usr.IsAdmin && disc.OwnerID != usr.ID {
		return rest.ErrPermissionDenied
	}
	return r.delete(And{Eq{"discovery.id": id}, r.userFilter()})
}

func (r *discoveryRepository) Tracks(discoveryID string) model.DiscoveryTrackRepository {
	repo := &discoveryTrackRepository{}
	repo.ctx = r.ctx
	repo.db = r.db
	repo.discoveryID = discoveryID
	repo.discoveryRepo = r
	repo.registerModel(&model.DiscoveryTrack{}, nil)
	repo.tableName = "discovery_tracks"
	return repo
}

func (r *discoveryRepository) GetDiscoveries(mediaFileId string) (model.Discoveries, error) {
	sel := r.selectDiscovery().
		Join("discovery_tracks on discovery.id = discovery_tracks.discovery_id").
		Where(And{Eq{"discovery_tracks.media_file_id": mediaFileId}, r.userFilter()})
	var res []struct {
		model.Discovery
		OwnerName string `db:"owner_name"`
	}
	if err := r.queryAll(sel, &res); err != nil {
		return nil, err
	}
	out := make(model.Discoveries, len(res))
	for i := range res {
		res[i].Discovery.OwnerName = res[i].OwnerName
		res[i].Discovery.Type = "discovery"
		out[i] = res[i].Discovery
	}
	return out, nil
}

func (r *discoveryRepository) loadTracks(sel SelectBuilder, id string) (model.DiscoveryTracks, error) {
	sel = sel.
		Columns(
			"discovery_tracks.*",
			"f.*",
		).
		From("discovery_tracks").
		Join("media_file f on f.id = discovery_tracks.media_file_id").
		Where(Eq{"discovery_tracks.discovery_id": id}).
		OrderBy("discovery_tracks.id")
	rows := dbDiscoveryTracks{}
	if err := r.queryAll(sel, &rows); err != nil {
		return nil, err
	}
	return rows.toModels(), nil
}

func (r *discoveryRepository) ReplaceTracks(id string, mediaFileIDs []string) error {
	del := Delete("discovery_tracks").Where(Eq{"discovery_id": id})
	if _, err := r.executeSQL(del); err != nil {
		return err
	}
	if len(mediaFileIDs) == 0 {
		return r.refreshCounters(id)
	}
	builder := Insert("discovery_tracks").Columns("id", "discovery_id", "media_file_id")
	for idx, mfID := range mediaFileIDs {
		builder = builder.Values(idx+1, id, mfID)
	}
	if _, err := r.executeSQL(builder); err != nil {
		return err
	}
	return r.refreshCounters(id)
}

func (r *discoveryRepository) refreshCounters(id string) error {
	upd := Update("discovery").
		Set("song_count", Select("count(*)").From("discovery_tracks").Where(Eq{"discovery_id": id})).
		Set("duration", Select("ifnull(sum(m.duration),0)").
			From("discovery_tracks dt").
			Join("media_file m on m.id = dt.media_file_id").
			Where(Eq{"dt.discovery_id": id})).
		Set("size", Select("ifnull(sum(m.size),0)").
			From("discovery_tracks dt").
			Join("media_file m on m.id = dt.media_file_id").
			Where(Eq{"dt.discovery_id": id})).
		Set("updated_at", time.Now()).
		Where(Eq{"id": id})
	_, err := r.executeSQL(upd)
	return err
}

func (r *discoveryRepository) findBy(cond Sqlizer) (*model.Discovery, error) {
	sel := r.selectDiscovery().Where(cond)
	var res struct {
		model.Discovery
		OwnerName string `db:"owner_name"`
	}
	err := r.queryOne(sel, &res)
	if errors.Is(err, rest.ErrNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	res.Discovery.OwnerName = res.OwnerName
	res.Discovery.Type = "discovery"
	return &res.Discovery, nil
}

func (r *discoveryRepository) selectDiscovery(options ...model.QueryOptions) SelectBuilder {
	sel := r.newSelect(options...).
		Columns("discovery.*", "owner.username as owner_name").
		From("discovery").
		LeftJoin("user owner on owner.id = discovery.owner_id")
	return sel
}

func (r *discoveryRepository) EntityName() string { return "discovery" }

func (r *discoveryRepository) NewInstance() interface{} { return &model.Discovery{} }

func (r *discoveryRepository) Save(entity interface{}) (string, error) {
	disc := entity.(*model.Discovery)
	if err := r.Put(disc); err != nil {
		return "", err
	}
	return disc.ID, nil
}

func (r *discoveryRepository) Update(id string, entity interface{}, cols ...string) error {
	disc := entity.(*model.Discovery)
	disc.ID = id
	return r.Put(disc)
}

var _ model.DiscoveryRepository = (*discoveryRepository)(nil)
var _ rest.Repository = (*discoveryRepository)(nil)
var _ rest.Persistable = (*discoveryRepository)(nil)
