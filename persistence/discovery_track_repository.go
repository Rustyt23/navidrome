package persistence

import (
	"database/sql"
	"fmt"
	"strings"

	. "github.com/Masterminds/squirrel"
	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/slice"
)

type discoveryTrackRepository struct {
	sqlRepository
	discoveryId   string
	discovery     *model.Discovery
	discoveryRepo *discoveryRepository
}

type dbPlaylistTrack struct {
	dbMediaFile
	*model.DiscoveryTrack `structs:",flatten"`
}

func (t *dbPlaylistTrack) PostScan() error {
	if err := t.dbMediaFile.PostScan(); err != nil {
		return err
	}
	t.DiscoveryTrack.MediaFile = *t.dbMediaFile.MediaFile
	t.DiscoveryTrack.MediaFile.ID = t.MediaFileID
	return nil
}

type dbPlaylistTracks []dbPlaylistTrack

func (t dbPlaylistTracks) toModels() model.DiscoveryTracks {
	return slice.Map(t, func(trk dbPlaylistTrack) model.DiscoveryTrack {
		return *trk.DiscoveryTrack
	})
}

func (r *discoveryRepository) Tracks(discoveryId string, refreshSmartPlaylist bool) model.DiscoveryTrackRepository {
	p := &discoveryTrackRepository{}
	p.discoveryRepo = r
	p.discoveryId = discoveryId
	p.ctx = r.ctx
	p.db = r.db
	p.tableName = "discovery_tracks"
	p.registerModel(&model.DiscoveryTrack{}, map[string]filterFunc{
		"missing":    booleanFilter,
		"library_id": libraryIdFilter,
		"q":          fullTextFilter("f"),
	})
	p.setSortMappings(
		map[string]string{
			"id":           "discovery_tracks.id",
			"artist":       "order_artist_name",
			"album_artist": "order_album_artist_name",
			"album":        "order_album_name, order_album_artist_name",
			"title":        "order_title",
			// To make sure these fields will be whitelisted
			"duration": "duration",
			"year":     "year",
			"bpm":      "bpm",
			"channels": "channels",
		},
		"f") // TODO I don't like this solution, but I won't change it now as it's not the focus of BFR.

	pls, err := r.Get(discoveryId)
	if err != nil {
		log.Warn(r.ctx, "Error getting discovery's tracks", "discoveryId", discoveryId, err)
		return nil
	}
	if refreshSmartPlaylist {
		r.refreshSmartPlaylist(pls)
	}
	p.discovery = pls
	return p
}

func (r *discoveryTrackRepository) Count(options ...rest.QueryOptions) (int64, error) {
	query := Select().
		LeftJoin("media_file f on f.id = media_file_id").
		Where(Eq{"discovery_id": r.discoveryId})
	return r.count(query, r.parseRestOptions(r.ctx, options...))
}

func (r *discoveryTrackRepository) Read(id string) (interface{}, error) {
	userID := loggedUser(r.ctx).ID
	sel := r.newSelect().
		LeftJoin("annotation on ("+
			"annotation.item_id = media_file_id"+
			" AND annotation.item_type = 'media_file'"+
			" AND annotation.user_id = '"+userID+"')").
		Columns(
			"coalesce(starred, 0) as starred",
			"coalesce(play_count, 0) as play_count",
			"coalesce(rating, 0) as rating",
			"starred_at",
			"play_date",
			"f.*",
			"discovery_tracks.*",
		).
		Join("media_file f on f.id = media_file_id").
		Where(And{Eq{"discovery_id": r.discoveryId}, Eq{"discovery_tracks.id": id}})
	var trk dbPlaylistTrack
	err := r.queryOne(sel, &trk)
	return trk.DiscoveryTrack, err
}

func (r *discoveryTrackRepository) GetAll(options ...model.QueryOptions) (model.DiscoveryTracks, error) {
	tracks, err := r.discoveryRepo.loadTracks(r.newSelect(options...), r.discoveryId)
	if err != nil {
		return nil, err
	}
	return tracks, err
}

func (r *discoveryTrackRepository) GetAlbumIDs(options ...model.QueryOptions) ([]string, error) {
	query := r.newSelect(options...).Columns("distinct mf.album_id").
		Join("media_file mf on mf.id = media_file_id").
		Where(Eq{"discovery_id": r.discoveryId})
	var ids []string
	err := r.queryAllSlice(query, &ids)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *discoveryTrackRepository) Search(q string, offset, size int, options ...model.QueryOptions) (model.DiscoveryTracks, error) {
	q = strings.TrimSpace(q)
	q = strings.TrimSuffix(q, "*")
	if len(q) < 2 {
		return nil, nil
	}

	sel := r.newSelect(options...).
		Join("media_file f on f.id = media_file_id").
		Where(Eq{"discovery_id": r.discoveryId})

	if filter := fullTextExpr("f", q); filter != nil {
		sel = sel.Where(filter).OrderBy("order_title")
	} else {
		sel = sel.OrderBy("discovery_tracks.rowid")
	}
	sel = sel.Where(Eq{"f.missing": false}).Limit(uint64(size)).Offset(uint64(offset))

	tracks, err := r.discoveryRepo.loadTracks(sel, r.discoveryId)
	if err != nil {
		return nil, fmt.Errorf("searching discovery tracks by query %q: %w", q, err)
	}
	return tracks, nil
}

func (r *discoveryTrackRepository) ReadAll(options ...rest.QueryOptions) (interface{}, error) {
	return r.GetAll(r.parseRestOptions(r.ctx, options...))
}

func (r *discoveryTrackRepository) EntityName() string {
	return "discovery_tracks"
}

func (r *discoveryTrackRepository) NewInstance() interface{} {
	return &model.DiscoveryTrack{}
}

func (r *discoveryTrackRepository) isTracksEditable() bool {
	return r.discoveryRepo.isWritable(r.discoveryId) && !r.discovery.IsSmartPlaylist()
}

func (r *discoveryTrackRepository) Add(mediaFileIds []string) (int, error) {
	if !r.isTracksEditable() {
		return 0, rest.ErrPermissionDenied
	}

	if len(mediaFileIds) == 0 {
		return 0, nil
	}

	existing, err := r.getTracks()
	if err != nil {
		return 0, err
	}

	seen := make(map[string]struct{}, len(existing))
	for _, id := range existing {
		seen[id] = struct{}{}
	}

	unique := make([]string, 0, len(mediaFileIds))
	for _, id := range mediaFileIds {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}

	if len(unique) == 0 {
		return 0, nil
	}

	log.Debug(r.ctx, "Adding songs to discovery", "discoveryId", r.discoveryId, "mediaFileIds", unique)

	// Get next pos (ID) in discovery
	sq := r.newSelect().Columns("max(id) as max").Where(Eq{"discovery_id": r.discoveryId})
	var res struct{ Max sql.NullInt32 }
	err = r.queryOne(sq, &res)
	if err != nil {
		return 0, err
	}

	return len(unique), r.discoveryRepo.addTracks(r.discoveryId, int(res.Max.Int32+1), unique)
}

func (r *discoveryTrackRepository) addMediaFileIds(cond Sqlizer) (int, error) {
	sq := Select("id").From("media_file").Where(cond).OrderBy("album_artist, album, release_date, disc_number, track_number")
	var ids []string
	err := r.queryAllSlice(sq, &ids)
	if err != nil {
		log.Error(r.ctx, "Error getting tracks to add to discovery", err)
		return 0, err
	}
	return r.Add(ids)
}

func (r *discoveryTrackRepository) AddAlbums(albumIds []string) (int, error) {
	return r.addMediaFileIds(Eq{"album_id": albumIds})
}

func (r *discoveryTrackRepository) AddArtists(artistIds []string) (int, error) {
	return r.addMediaFileIds(Eq{"album_artist_id": artistIds})
}

func (r *discoveryTrackRepository) AddDiscs(discs []model.DiscID) (int, error) {
	if len(discs) == 0 {
		return 0, nil
	}
	var clauses Or
	for _, d := range discs {
		clauses = append(clauses, And{Eq{"album_id": d.AlbumID}, Eq{"release_date": d.ReleaseDate}, Eq{"disc_number": d.DiscNumber}})
	}
	return r.addMediaFileIds(clauses)
}

// Get ids from all current tracks
func (r *discoveryTrackRepository) getTracks() ([]string, error) {
	all := r.newSelect().Columns("media_file_id").Where(Eq{"discovery_id": r.discoveryId}).OrderBy("id")
	var ids []string
	err := r.queryAllSlice(all, &ids)
	if err != nil {
		log.Error(r.ctx, "Error querying current tracks from discovery", "discoveryId", r.discoveryId, err)
		return nil, err
	}
	return ids, nil
}

func (r *discoveryTrackRepository) Delete(ids ...string) error {
	if !r.isTracksEditable() {
		return rest.ErrPermissionDenied
	}
	err := r.delete(And{Eq{"discovery_id": r.discoveryId}, Eq{"id": ids}})
	if err != nil {
		return err
	}

	return r.discoveryRepo.renumber(r.discoveryId)
}

func (r *discoveryTrackRepository) DeleteAll() error {
	if !r.isTracksEditable() {
		return rest.ErrPermissionDenied
	}
	err := r.delete(Eq{"discovery_id": r.discoveryId})
	if err != nil {
		return err
	}

	return r.discoveryRepo.renumber(r.discoveryId)
}

func (r *discoveryTrackRepository) Reorder(pos int, newPos int) error {
	if !r.isTracksEditable() {
		return rest.ErrPermissionDenied
	}
	ids, err := r.getTracks()
	if err != nil {
		return err
	}
	newOrder := slice.Move(ids, pos-1, newPos-1)
	return r.discoveryRepo.updatePlaylist(r.discoveryId, newOrder)
}

var _ model.DiscoveryTrackRepository = (*discoveryTrackRepository)(nil)
