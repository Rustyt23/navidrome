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

type discoverySongRepository struct {
	sqlRepository
	discoveryId   string
	discovery     *model.Discovery
	discoveryRepo *discoveryRepository
}

type dbDiscoverySong struct {
	dbMediaFile
	*model.DiscoverySong `structs:",flatten"`
}

func (t *dbDiscoverySong) PostScan() error {
	if err := t.dbMediaFile.PostScan(); err != nil {
		return err
	}
	t.DiscoverySong.MediaFile = *t.dbMediaFile.MediaFile
	t.DiscoverySong.MediaFile.ID = t.MediaFileID
	return nil
}

type dbDiscoverySongs []dbDiscoverySong

func (t dbDiscoverySongs) toModels() model.DiscoverySongs {
	return slice.Map(t, func(trk dbDiscoverySong) model.DiscoverySong {
		return *trk.DiscoverySong
	})
}

func (r *discoveryRepository) Songs(discoveryId string, refreshSmartDiscovery bool) model.DiscoverySongRepository {
	p := &discoverySongRepository{}
	p.discoveryRepo = r
	p.discoveryId = discoveryId
	p.ctx = r.ctx
	p.db = r.db
	p.tableName = "discovery_songs"
	p.registerModel(&model.DiscoverySong{}, map[string]filterFunc{
		"missing":    booleanFilter,
		"library_id": libraryIdFilter,
		"q":          fullTextFilter("f"),
	})
	p.setSortMappings(
		map[string]string{
			"id":           "discovery_songs.id",
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
		log.Warn(r.ctx, "Error getting discovery's songs", "discoveryId", discoveryId, err)
		return nil
	}
	if refreshSmartDiscovery {
		r.refreshSmartDiscovery(pls)
	}
	p.discovery = pls
	return p
}

func (r *discoverySongRepository) Count(options ...rest.QueryOptions) (int64, error) {
	query := Select().
		LeftJoin("media_file f on f.id = media_file_id").
		Where(Eq{"discovery_id": r.discoveryId})
	return r.count(query, r.parseRestOptions(r.ctx, options...))
}

func (r *discoverySongRepository) Read(id string) (interface{}, error) {
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
			"discovery_songs.*",
		).
		Join("media_file f on f.id = media_file_id").
		Where(And{Eq{"discovery_id": r.discoveryId}, Eq{"discovery_songs.id": id}})
	var trk dbDiscoverySong
	err := r.queryOne(sel, &trk)
	return trk.DiscoverySong, err
}

func (r *discoverySongRepository) GetAll(options ...model.QueryOptions) (model.DiscoverySongs, error) {
	songs, err := r.discoveryRepo.loadSongs(r.newSelect(options...), r.discoveryId)
	if err != nil {
		return nil, err
	}
	return songs, err
}

func (r *discoverySongRepository) GetAlbumIDs(options ...model.QueryOptions) ([]string, error) {
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

func (r *discoverySongRepository) Search(q string, offset, size int, options ...model.QueryOptions) (model.DiscoverySongs, error) {
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
		sel = sel.OrderBy("discovery_songs.rowid")
	}
	sel = sel.Where(Eq{"f.missing": false}).Limit(uint64(size)).Offset(uint64(offset))

	songs, err := r.discoveryRepo.loadSongs(sel, r.discoveryId)
	if err != nil {
		return nil, fmt.Errorf("searching discovery songs by query %q: %w", q, err)
	}
	return songs, nil
}

func (r *discoverySongRepository) ReadAll(options ...rest.QueryOptions) (interface{}, error) {
	return r.GetAll(r.parseRestOptions(r.ctx, options...))
}

func (r *discoverySongRepository) EntityName() string {
	return "discovery_songs"
}

func (r *discoverySongRepository) NewInstance() interface{} {
	return &model.DiscoverySong{}
}

func (r *discoverySongRepository) isSongsEditable() bool {
	return r.discoveryRepo.isWritable(r.discoveryId) && !r.discovery.IsSmartDiscovery()
}

func (r *discoverySongRepository) Add(mediaFileIds []string) (int, error) {
	if !r.isSongsEditable() {
		return 0, rest.ErrPermissionDenied
	}

	if len(mediaFileIds) == 0 {
		return 0, nil
	}

	existing, err := r.getSongs()
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

	return len(unique), r.discoveryRepo.addSongs(r.discoveryId, int(res.Max.Int32+1), unique)
}

func (r *discoverySongRepository) addMediaFileIds(cond Sqlizer) (int, error) {
	sq := Select("id").From("media_file").Where(cond).OrderBy("album_artist, album, release_date, disc_number, track_number")
	var ids []string
	err := r.queryAllSlice(sq, &ids)
	if err != nil {
		log.Error(r.ctx, "Error getting songs to add to discovery", err)
		return 0, err
	}
	return r.Add(ids)
}

func (r *discoverySongRepository) AddAlbums(albumIds []string) (int, error) {
	return r.addMediaFileIds(Eq{"album_id": albumIds})
}

func (r *discoverySongRepository) AddArtists(artistIds []string) (int, error) {
	return r.addMediaFileIds(Eq{"album_artist_id": artistIds})
}

func (r *discoverySongRepository) AddDiscs(discs []model.DiscID) (int, error) {
	if len(discs) == 0 {
		return 0, nil
	}
	var clauses Or
	for _, d := range discs {
		clauses = append(clauses, And{Eq{"album_id": d.AlbumID}, Eq{"release_date": d.ReleaseDate}, Eq{"disc_number": d.DiscNumber}})
	}
	return r.addMediaFileIds(clauses)
}

// Get ids from all current songs
func (r *discoverySongRepository) getSongs() ([]string, error) {
	all := r.newSelect().Columns("media_file_id").Where(Eq{"discovery_id": r.discoveryId}).OrderBy("id")
	var ids []string
	err := r.queryAllSlice(all, &ids)
	if err != nil {
		log.Error(r.ctx, "Error querying current songs from discovery", "discoveryId", r.discoveryId, err)
		return nil, err
	}
	return ids, nil
}

func (r *discoverySongRepository) Delete(ids ...string) error {
	if !r.isSongsEditable() {
		return rest.ErrPermissionDenied
	}
	err := r.delete(And{Eq{"discovery_id": r.discoveryId}, Eq{"id": ids}})
	if err != nil {
		return err
	}

	return r.discoveryRepo.renumber(r.discoveryId)
}

func (r *discoverySongRepository) DeleteAll() error {
	if !r.isSongsEditable() {
		return rest.ErrPermissionDenied
	}
	err := r.delete(Eq{"discovery_id": r.discoveryId})
	if err != nil {
		return err
	}

	return r.discoveryRepo.renumber(r.discoveryId)
}

func (r *discoverySongRepository) Reorder(pos int, newPos int) error {
	if !r.isSongsEditable() {
		return rest.ErrPermissionDenied
	}
	ids, err := r.getSongs()
	if err != nil {
		return err
	}
	newOrder := slice.Move(ids, pos-1, newPos-1)
	return r.discoveryRepo.updateDiscovery(r.discoveryId, newOrder)
}

var _ model.DiscoverySongRepository = (*discoverySongRepository)(nil)
