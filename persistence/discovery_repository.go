package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/criteria"
	"github.com/pocketbase/dbx"
)

type discoveryRepository struct {
	sqlRepository
}

type dbDiscovery struct {
	model.Discovery `structs:",flatten"`
	Rules           sql.NullString `structs:"-"`
}

func (p *dbDiscovery) PostScan() error {
	if p.Rules.String != "" {
		return json.Unmarshal([]byte(p.Rules.String), &p.Discovery.Rules)
	}
	return nil
}

func (p dbDiscovery) PostMapArgs(args map[string]any) error {
	var err error
	if p.Discovery.IsSmartPlaylist() {
		args["rules"], err = json.Marshal(p.Discovery.Rules)
		if err != nil {
			return fmt.Errorf("invalid criteria expression: %w", err)
		}
		return nil
	}
	delete(args, "rules")
	return nil
}

func NewDiscoveryRepository(ctx context.Context, db dbx.Builder) model.DiscoveryRepository {
	r := &discoveryRepository{}
	r.ctx = ctx
	r.db = db
	r.registerModel(&model.Discovery{}, map[string]filterFunc{
		"q":     discoveryFilter,
		"smart": smartDiscoveryFilter,
	})
	r.setSortMappings(map[string]string{
		"owner_name": "owner_name",
	})
	return r
}

func discoveryFilter(_ string, value interface{}) Sqlizer {
	return Or{
		substringFilter("discovery.name", value),
		substringFilter("discovery.comment", value),
	}
}

func smartDiscoveryFilter(string, interface{}) Sqlizer {
	return Or{
		Eq{"rules": ""},
		Eq{"rules": nil},
	}
}

func (r *discoveryRepository) userFilter() Sqlizer {
	user := loggedUser(r.ctx)
	if user.IsAdmin {
		return And{}
	}
	return Or{
		Eq{"public": true},
		Eq{"owner_id": user.ID},
	}
}

func (r *discoveryRepository) CountAll(options ...model.QueryOptions) (int64, error) {
	sq := Select().Where(r.userFilter())
	return r.count(sq, options...)
}

func (r *discoveryRepository) Exists(id string) (bool, error) {
	return r.exists(And{Eq{"id": id}, r.userFilter()})
}

func (r *discoveryRepository) Delete(id string) error {
	usr := loggedUser(r.ctx)
	if !usr.IsAdmin {
		pls, err := r.Get(id)
		if err != nil {
			return err
		}
		if pls.OwnerID != usr.ID {
			return rest.ErrPermissionDenied
		}
	}
	return r.delete(And{Eq{"id": id}, r.userFilter()})
}

func (r *discoveryRepository) Put(p *model.Discovery) error {
	pls := dbDiscovery{Discovery: *p}
	if pls.ID == "" {
		pls.CreatedAt = time.Now()
	} else {
		ok, err := r.Exists(pls.ID)
		if err != nil {
			return err
		}
		if !ok {
			return model.ErrNotAuthorized
		}
	}
	if p.Sync && !p.UpdatedAt.IsZero() {
		pls.UpdatedAt = p.UpdatedAt
	} else {
		now := time.Now()
		pls.UpdatedAt = now
		p.UpdatedAt = now
	}

	id, err := r.put(pls.ID, pls)
	if err != nil {
		return err
	}
	p.ID = id
	p.Type = "discovery"
	if p.Sync && !p.UpdatedAt.IsZero() {
		p.UpdatedAt = pls.UpdatedAt
	}

	if p.IsSmartPlaylist() {
		// Do not update tracks at this point, as it may take a long time and lock the DB, breaking the scan process
		//r.refreshSmartPlaylist(p)
		return nil
	}
	// Only update tracks if they were specified
	if len(pls.Tracks) > 0 {
		return r.updateTracks(id, p.MediaFiles())
	}
	return r.refreshCounters(&pls.Discovery)
}

func (r *discoveryRepository) Get(id string) (*model.Discovery, error) {
	return r.findBy(And{Eq{"discovery.id": id}, r.userFilter()})
}

func (r *discoveryRepository) GetWithTracks(id string, refreshSmartPlaylist, includeMissing bool) (*model.Discovery, error) {
	pls, err := r.Get(id)
	if err != nil {
		return nil, err
	}
	if refreshSmartPlaylist {
		r.refreshSmartPlaylist(pls)
	}
	tracks, err := r.loadTracks(Select().From("discovery_tracks").
		Where(Eq{"missing": false}).
		OrderBy("discovery_tracks.id"), id)
	if err != nil {
		log.Error(r.ctx, "Error loading discovery tracks ", "discovery", pls.Name, "id", pls.ID, err)
		return nil, err
	}
	pls.SetTracks(tracks)
	return pls, nil
}

func (r *discoveryRepository) FindByPath(path string) (*model.Discovery, error) {
	return r.findBy(Eq{"path": path})
}

func (r *discoveryRepository) GetSyncedByDirectory(dir string) (model.Discoveries, error) {
	cleanedDir := filepath.Clean(dir)
	pattern := cleanedDir
	if cleanedDir == "." || cleanedDir == "" {
		pattern = "%"
	} else {
		if !strings.HasSuffix(pattern, string(os.PathSeparator)) {
			pattern += string(os.PathSeparator)
		}
		pattern += "%"
	}

	where := And{Eq{"sync": true}, r.userFilter()}
	if pattern != "%" {
		where = append(where, Like{"path": pattern})
	}

	sel := r.selectDiscovery().Where(where)
	var res []dbDiscovery
	if err := r.queryAll(sel, &res); err != nil {
		return nil, err
	}

	discoveries := make(model.Discoveries, 0, len(res))
	for _, p := range res {
		if filepath.Clean(filepath.Dir(p.Path)) == cleanedDir {
			discoveries = append(discoveries, p.Discovery)
		}
	}
	return discoveries, nil
}

func (r *discoveryRepository) findBy(sql Sqlizer) (*model.Discovery, error) {
	sel := r.selectDiscovery().Where(sql)
	var pls []dbDiscovery
	err := r.queryAll(sel, &pls)
	if err != nil {
		return nil, err
	}
	if len(pls) == 0 {
		return nil, model.ErrNotFound
	}

	p := &pls[0].Discovery
	p.Type = "discovery"
	return p, nil
}

func (r *discoveryRepository) GetAll(options ...model.QueryOptions) (model.Discoveries, error) {
	sel := r.selectDiscovery(options...).Where(r.userFilter())
	var res []dbDiscovery
	err := r.queryAll(sel, &res)
	if err != nil {
		return nil, err
	}
	discoveries := make(model.Discoveries, len(res))
	for i, p := range res {
		discoveries[i] = p.Discovery
	}
	return discoveries, err
}

func (r *discoveryRepository) GetAllByPlaylistFolder(options ...model.QueryOptions) (model.Discoveries, error) {
	hasFolderFilter := r.hasFolderIDFilter(options...)
	sel := r.selectDiscovery(options...).Where(r.userFilter())
	if !hasFolderFilter {
		sel = sel.Where(Eq{"folder_id": nil}) // root only
	}
	var res []dbDiscovery
	if err := r.queryAll(sel, &res); err != nil {
		return nil, err
	}
	out := make(model.Discoveries, 0, len(res))
	for _, pf := range res {
		out = append(out, pf.Discovery)
	}
	return out, nil
}

func (r *discoveryRepository) hasFolderIDFilter(options ...model.QueryOptions) bool {
	if len(options) == 0 || options[0].Filters == nil {
		return false
	}
	switch f := options[0].Filters.(type) {
	case Eq:
		_, exists := f["folder_id"]
		return exists
	case And:
		for _, sub := range f {
			if eq, ok := sub.(Eq); ok {
				if _, exists := eq["folder_id"]; exists {
					return true
				}
			}
		}
	}
	return false
}

func (r *discoveryRepository) GetPlaylists(mediaFileId string) (model.Discoveries, error) {
	sel := r.selectDiscovery(model.QueryOptions{Sort: "name"}).
		Join("discovery_tracks on discovery.id = discovery_tracks.discovery_id").
		Where(And{Eq{"discovery_tracks.media_file_id": mediaFileId}, r.userFilter()})
	var res []dbDiscovery
	err := r.queryAll(sel, &res)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return model.Discoveries{}, nil
		}
		return nil, err
	}
	discoveries := make(model.Discoveries, len(res))
	for i, p := range res {
		discoveries[i] = p.Discovery
	}
	return discoveries, nil
}

func (r *discoveryRepository) selectDiscovery(options ...model.QueryOptions) SelectBuilder {
	return r.newSelect(options...).Join("user on user.id = owner_id").
		Columns(r.tableName+".*", "user.user_name as owner_name")
}

func (r *discoveryRepository) refreshSmartPlaylist(pls *model.Discovery) bool {
	// Only refresh if it is a smart discovery and was not refreshed within the interval provided by the refresh delay config
	if !pls.IsSmartPlaylist() || (pls.EvaluatedAt != nil && time.Since(*pls.EvaluatedAt) < conf.Server.SmartPlaylistRefreshDelay) {
		return false
	}

	// Never refresh other users' discoveries
	usr := loggedUser(r.ctx)
	if pls.OwnerID != usr.ID {
		log.Trace(r.ctx, "Not refreshing smart discovery from other user", "discovery", pls.Name, "id", pls.ID)
		return false
	}

	log.Debug(r.ctx, "Refreshing smart discovery", "discovery", pls.Name, "id", pls.ID)
	start := time.Now()

	// Remove old tracks
	del := Delete("discovery_tracks").Where(Eq{"discovery_id": pls.ID})
	_, err := r.executeSQL(del)
	if err != nil {
		log.Error(r.ctx, "Error deleting old smart discovery tracks", "discovery", pls.Name, "id", pls.ID, err)
		return false
	}

	// Re-populate discovery based on Smart Discovery criteria
	rules := *pls.Rules

	// If the discovery depends on other discoveries, recursively refresh them first
	childPlaylistIds := rules.ChildPlaylistIds()
	for _, id := range childPlaylistIds {
		childPls, err := r.Get(id)
		if err != nil {
			log.Error(r.ctx, "Error loading child discovery", "id", pls.ID, "childId", id, err)
			return false
		}
		r.refreshSmartPlaylist(childPls)
	}

	sq := Select("row_number() over (order by "+rules.OrderBy()+") as id", "'"+pls.ID+"' as discovery_id", "media_file.id as media_file_id").
		From("media_file").LeftJoin("annotation on (" +
		"annotation.item_id = media_file.id" +
		" AND annotation.item_type = 'media_file'" +
		" AND annotation.user_id = '" + usr.ID + "')")
	sq = r.addCriteria(sq, rules)
	insSql := Insert("discovery_tracks").Columns("id", "discovery_id", "media_file_id").Select(sq)
	_, err = r.executeSQL(insSql)
	if err != nil {
		log.Error(r.ctx, "Error refreshing smart discovery tracks", "discovery", pls.Name, "id", pls.ID, err)
		return false
	}

	// Update discovery stats
	err = r.refreshCounters(pls)
	if err != nil {
		log.Error(r.ctx, "Error updating smart discovery stats", "discovery", pls.Name, "id", pls.ID, err)
		return false
	}

	// Update when the discovery was last refreshed (for cache purposes)
	updSql := Update(r.tableName).Set("evaluated_at", time.Now()).Where(Eq{"id": pls.ID})
	_, err = r.executeSQL(updSql)
	if err != nil {
		log.Error(r.ctx, "Error updating smart discovery", "discovery", pls.Name, "id", pls.ID, err)
		return false
	}

	log.Debug(r.ctx, "Refreshed discovery", "discovery", pls.Name, "id", pls.ID, "numTracks", pls.SongCount, "elapsed", time.Since(start))

	return true
}

func (r *discoveryRepository) addCriteria(sql SelectBuilder, c criteria.Criteria) SelectBuilder {
	sql = sql.Where(c)
	if c.Limit > 0 {
		sql = sql.Limit(uint64(c.Limit)).Offset(uint64(c.Offset))
	}
	if order := c.OrderBy(); order != "" {
		sql = sql.OrderBy(order)
	}
	return sql
}

func (r *discoveryRepository) updateTracks(id string, tracks model.MediaFiles) error {
	ids := make([]string, len(tracks))
	for i := range tracks {
		ids[i] = tracks[i].ID
	}
	return r.updateDiscovery(id, ids)
}

func (r *discoveryRepository) updateDiscovery(discoveryId string, mediaFileIds []string) error {
	if !r.isWritable(discoveryId) {
		return rest.ErrPermissionDenied
	}

	// Remove old tracks
	del := Delete("discovery_tracks").Where(Eq{"discovery_id": discoveryId})
	_, err := r.executeSQL(del)
	if err != nil {
		return err
	}

	return r.addTracks(discoveryId, 1, mediaFileIds)
}

func (r *discoveryRepository) addTracks(discoveryId string, startingPos int, mediaFileIds []string) error {
	// Break the track list in chunks to avoid hitting SQLITE_MAX_VARIABLE_NUMBER limit
	// Add new tracks, chunk by chunk
	pos := startingPos
	for chunk := range slices.Chunk(mediaFileIds, 200) {
		ins := Insert("discovery_tracks").Columns("discovery_id", "media_file_id", "id")
		for _, t := range chunk {
			ins = ins.Values(discoveryId, t, pos)
			pos++
		}
		_, err := r.executeSQL(ins)
		if err != nil {
			return err
		}
	}

	return r.refreshCounters(&model.Discovery{ID: discoveryId})
}

// refreshCounters updates total discovery duration, size and count
func (r *discoveryRepository) refreshCounters(pls *model.Discovery) error {
	statsSql := Select(
		"coalesce(sum(duration), 0) as duration",
		"coalesce(sum(size), 0) as size",
		"count(*) as count",
	).
		From("media_file").
		Join("discovery_tracks f on f.media_file_id = media_file.id").
		Where(Eq{"discovery_id": pls.ID})
	var res struct{ Duration, Size, Count float32 }
	err := r.queryOne(statsSql, &res)
	if err != nil {
		return err
	}

	// Update discovery's total duration, size and count
	upd := Update("discovery").
		Set("duration", res.Duration).
		Set("size", res.Size).
		Set("song_count", res.Count).
		Set("updated_at", time.Now()).
		Where(Eq{"id": pls.ID})
	_, err = r.executeSQL(upd)
	if err != nil {
		return err
	}
	pls.SongCount = int(res.Count)
	pls.Duration = res.Duration
	pls.Size = int64(res.Size)
	return nil
}

func (r *discoveryRepository) loadTracks(sel SelectBuilder, id string) (model.DiscoveryTracks, error) {
	sel = r.applyLibraryFilter(sel, "f")
	userID := loggedUser(r.ctx).ID
	tracksQuery := sel.
		Columns(
			"coalesce(starred, 0) as starred",
			"starred_at",
			"coalesce(play_count, 0) as play_count",
			"play_date",
			"coalesce(rating, 0) as rating",
			"f.*",
			"discovery_tracks.*",
			"library.path as library_path",
			"library.name as library_name",
		).
		LeftJoin("annotation on (" +
			"annotation.item_id = media_file_id" +
			" AND annotation.item_type = 'media_file'" +
			" AND annotation.user_id = '" + userID + "')").
		Join("media_file f on f.id = media_file_id").
		Join("library on f.library_id = library.id").
		Where(Eq{"discovery_id": id})
	tracks := dbDiscoveryTracks{}
	err := r.queryAll(tracksQuery, &tracks)
	if err != nil {
		return nil, err
	}
	return tracks.toModels(), err
}

func (r *discoveryRepository) Count(options ...rest.QueryOptions) (int64, error) {
	return r.CountAll(r.parseRestOptions(r.ctx, options...))
}

func (r *discoveryRepository) Read(id string) (interface{}, error) {
	return r.Get(id)
}

func (r *discoveryRepository) ReadAll(options ...rest.QueryOptions) (interface{}, error) {
	return r.GetAll(r.parseRestOptions(r.ctx, options...))
}

func (r *discoveryRepository) EntityName() string {
	return "discovery"
}

func (r *discoveryRepository) NewInstance() interface{} {
	return &model.Discovery{}
}

func (r *discoveryRepository) Save(entity interface{}) (string, error) {
	pls := entity.(*model.Discovery)
	pls.OwnerID = loggedUser(r.ctx).ID
	pls.ID = "" // Make sure we don't override an existing discovery
	err := r.Put(pls)
	if err != nil {
		return "", err
	}
	return pls.ID, err
}

func (r *discoveryRepository) Update(id string, entity interface{}, cols ...string) error {
	pls := dbDiscovery{Discovery: *entity.(*model.Discovery)}
	current, err := r.Get(id)
	if err != nil {
		return err
	}
	usr := loggedUser(r.ctx)
	if !usr.IsAdmin {
		// Only the owner can update the discovery
		if current.OwnerID != usr.ID {
			return rest.ErrPermissionDenied
		}
		// Regular users can't change the ownership of a discovery
		if pls.OwnerID != "" && pls.OwnerID != usr.ID {
			return rest.ErrPermissionDenied
		}
	}
	pls.ID = id
	pls.UpdatedAt = time.Now()
	_, err = r.put(id, pls, append(cols, "updatedAt")...)
	if errors.Is(err, model.ErrNotFound) {
		return rest.ErrNotFound
	}
	return err
}

func (r *discoveryRepository) removeOrphans() error {
	sel := Select("discovery_tracks.discovery_id as id", "p.name").From("discovery_tracks").
		Join("discovery p on discovery_tracks.discovery_id = p.id").
		LeftJoin("media_file mf on discovery_tracks.media_file_id = mf.id").
		Where(Eq{"mf.id": nil}).
		GroupBy("discovery_tracks.discovery_id")

	var pls []struct{ Id, Name string }
	err := r.queryAll(sel, &pls)
	if err != nil {
		return fmt.Errorf("fetching discoveries with orphan tracks: %w", err)
	}

	for _, pl := range pls {
		log.Debug(r.ctx, "Cleaning-up orphan tracks from discovery", "id", pl.Id, "name", pl.Name)
		del := Delete("discovery_tracks").Where(And{
			ConcatExpr("media_file_id not in (select id from media_file)"),
			Eq{"discovery_id": pl.Id},
		})
		n, err := r.executeSQL(del)
		if n == 0 || err != nil {
			return fmt.Errorf("deleting orphan tracks from discovery %s: %w", pl.Name, err)
		}
		log.Debug(r.ctx, "Deleted tracks, now reordering", "id", pl.Id, "name", pl.Name, "deleted", n)

		// Renumber the discovery if any track was removed
		if err := r.renumber(pl.Id); err != nil {
			return fmt.Errorf("renumbering discovery %s: %w", pl.Name, err)
		}
	}
	return nil
}

func (r *discoveryRepository) renumber(id string) error {
	var ids []string
	sq := Select("media_file_id").From("discovery_tracks").Where(Eq{"discovery_id": id}).OrderBy("id")
	err := r.queryAllSlice(sq, &ids)
	if err != nil {
		return err
	}
	return r.updateDiscovery(id, ids)
}

func (r *discoveryRepository) isWritable(discoveryId string) bool {
	usr := loggedUser(r.ctx)
	if usr.IsAdmin {
		return true
	}
	pls, err := r.Get(discoveryId)
	return err == nil && pls.OwnerID == usr.ID
}

func (r *discoveryRepository) UpdatePlaylistFolder(id string, folderID *string) error {
	if err := rejectEmptyOptionalID(folderID); err != nil {
		return err
	}

	discovery, err := r.Get(id)
	if err != nil {
		return err
	}

	if (discovery.FolderID == nil && folderID == nil) ||
		(discovery.FolderID != nil && folderID != nil && *discovery.FolderID == *folderID) {
		return nil
	}

	if folderID != nil {
		var dstOwner struct{ OwnerID string }
		if err := r.queryOne(Select("owner_id").From("discovery_folder").Where(Eq{"id": *folderID}), &dstOwner); err != nil {
			return err
		}
		usr := loggedUser(r.ctx)
		if !usr.IsAdmin && dstOwner.OwnerID != usr.ID {
			return rest.ErrPermissionDenied
		}
	}

	discovery.FolderID = folderID
	return r.Update(id, discovery, "folderId")
}

var _ model.DiscoveryRepository = (*discoveryRepository)(nil)
var _ rest.Repository = (*discoveryRepository)(nil)
var _ rest.Persistable = (*discoveryRepository)(nil)
