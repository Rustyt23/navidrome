package persistence

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	. "github.com/Masterminds/squirrel"
	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/slice"
	"golang.org/x/text/unicode/norm"
)

type playlistTrackRepository struct {
	sqlRepository
	playlistId   string
	playlist     *model.Playlist
	playlistRepo *playlistRepository
	lastRestOpts rest.QueryOptions
}

type dbPlaylistTrack struct {
	dbMediaFile
	*model.PlaylistTrack `structs:",flatten"`
}

func (t *dbPlaylistTrack) PostScan() error {
	if err := t.dbMediaFile.PostScan(); err != nil {
		return err
	}
	t.PlaylistTrack.MediaFile = *t.dbMediaFile.MediaFile
	t.PlaylistTrack.MediaFile.ID = t.MediaFileID
	return nil
}

type dbPlaylistTracks []dbPlaylistTrack

func (t dbPlaylistTracks) toModels() model.PlaylistTracks {
	return slice.Map(t, func(trk dbPlaylistTrack) model.PlaylistTrack {
		return *trk.PlaylistTrack
	})
}

func (r *playlistRepository) Tracks(playlistId string, refreshSmartPlaylist bool) model.PlaylistTrackRepository {
	p := &playlistTrackRepository{}
	p.playlistRepo = r
	p.playlistId = playlistId
	p.ctx = r.ctx
	p.db = r.db
	p.tableName = "playlist_tracks"
	p.registerModel(&model.PlaylistTrack{}, map[string]filterFunc{
		"missing":    booleanFilter,
		"library_id": libraryIdFilter,
		"q":          fullTextFilter("f"),
	})
	p.setSortMappings(
		map[string]string{
			"id":           "playlist_tracks.id",
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

	pls, err := r.Get(playlistId)
	if err != nil {
		log.Warn(r.ctx, "Error getting playlist's tracks", "playlistId", playlistId, err)
		return nil
	}
	if refreshSmartPlaylist {
		r.refreshSmartPlaylist(pls)
	}
	p.playlist = pls
	return p
}

func (r *playlistTrackRepository) Count(options ...rest.QueryOptions) (int64, error) {
	var restOpts rest.QueryOptions
	if len(options) > 0 {
		restOpts = options[0]
	}
	modelOpts := r.parseRestOptions(r.ctx, options...)
	tracks, err := r.listWithMissing(modelOpts, restOpts)
	if err != nil {
		return 0, err
	}
	return int64(len(tracks)), nil
}

func (r *playlistTrackRepository) Read(id string) (interface{}, error) {
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
			"playlist_tracks.*",
		).
		Join("media_file f on f.id = media_file_id").
		Where(And{Eq{"playlist_id": r.playlistId}, Eq{"playlist_tracks.id": id}})
	var trk dbPlaylistTrack
	err := r.queryOne(sel, &trk)
	return trk.PlaylistTrack, err
}

func (r *playlistTrackRepository) GetAll(options ...model.QueryOptions) (model.PlaylistTracks, error) {
	var opt model.QueryOptions
	if len(options) > 0 {
		opt = options[0]
	}

	tracks, err := r.listWithMissing(opt, r.lastRestOpts)
	if err != nil {
		return nil, err
	}

	if opt.Offset > 0 {
		if opt.Offset >= len(tracks) {
			return model.PlaylistTracks{}, nil
		}
		tracks = tracks[opt.Offset:]
	}
	if opt.Max > 0 && opt.Max < len(tracks) {
		tracks = tracks[:opt.Max]
	}

	return tracks, nil
}

func (r *playlistTrackRepository) GetAlbumIDs(options ...model.QueryOptions) ([]string, error) {
	query := r.newSelect(options...).Columns("distinct mf.album_id").
		Join("media_file mf on mf.id = media_file_id").
		Where(Eq{"playlist_id": r.playlistId})
	var ids []string
	err := r.queryAllSlice(query, &ids)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *playlistTrackRepository) Search(q string, offset, size int, options ...model.QueryOptions) (model.PlaylistTracks, error) {
	q = strings.TrimSpace(q)
	q = strings.TrimSuffix(q, "*")
	if len(q) < 2 {
		return nil, nil
	}

	sel := r.newSelect(options...).
		Join("media_file f on f.id = media_file_id").
		Where(Eq{"playlist_id": r.playlistId})

	if filter := fullTextExpr("f", q); filter != nil {
		sel = sel.Where(filter).OrderBy("order_title")
	} else {
		sel = sel.OrderBy("playlist_tracks.rowid")
	}
	sel = sel.Where(Eq{"f.missing": false}).Limit(uint64(size)).Offset(uint64(offset))

	tracks, err := r.playlistRepo.loadTracks(sel, r.playlistId)
	if err != nil {
		return nil, fmt.Errorf("searching playlist tracks by query %q: %w", q, err)
	}
	return tracks, nil
}

func (r *playlistTrackRepository) ReadAll(options ...rest.QueryOptions) (interface{}, error) {
	if len(options) > 0 {
		r.lastRestOpts = options[0]
	} else {
		r.lastRestOpts = rest.QueryOptions{}
	}
	return r.GetAll(r.parseRestOptions(r.ctx, options...))
}

func (r *playlistTrackRepository) listWithMissing(opt model.QueryOptions, restOpts rest.QueryOptions) (model.PlaylistTracks, error) {
	noLimit := opt
	noLimit.Max = 0
	noLimit.Offset = 0

	tracks, err := r.playlistRepo.loadTracks(r.newSelect(noLimit), r.playlistId)
	if err != nil {
		return nil, err
	}

	if r.playlist == nil || !r.playlist.Sync || r.playlist.Path == "" {
		return tracks, nil
	}

	searchTerm := ""
	if restOpts.Filters != nil {
		if v, ok := restOpts.Filters["q"].(string); ok {
			searchTerm = strings.TrimSpace(strings.ToLower(v))
		}
	}

	merged, err := mergePlaylistTracksWithMissing(r.ctx, tracks, r.playlist, searchTerm)
	if err != nil {
		log.Warn(r.ctx, "Error resolving missing playlist tracks", "playlistId", r.playlistId, err)
		return tracks, nil
	}
	return merged, nil
}

func (r *playlistTrackRepository) EntityName() string {
	return "playlist_tracks"
}

func (r *playlistTrackRepository) NewInstance() interface{} {
	return &model.PlaylistTrack{}
}

func (r *playlistTrackRepository) isTracksEditable() bool {
	return r.playlistRepo.isWritable(r.playlistId) && !r.playlist.IsSmartPlaylist()
}

func (r *playlistTrackRepository) Add(mediaFileIds []string) (int, error) {
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

	log.Debug(r.ctx, "Adding songs to playlist", "playlistId", r.playlistId, "mediaFileIds", unique)

	// Get next pos (ID) in playlist
	sq := r.newSelect().Columns("max(id) as max").Where(Eq{"playlist_id": r.playlistId})
	var res struct{ Max sql.NullInt32 }
	err = r.queryOne(sq, &res)
	if err != nil {
		return 0, err
	}

	return len(unique), r.playlistRepo.addTracks(r.playlistId, int(res.Max.Int32+1), unique)
}

func (r *playlistTrackRepository) addMediaFileIds(cond Sqlizer) (int, error) {
	sq := Select("id").From("media_file").Where(cond).OrderBy("album_artist, album, release_date, disc_number, track_number")
	var ids []string
	err := r.queryAllSlice(sq, &ids)
	if err != nil {
		log.Error(r.ctx, "Error getting tracks to add to playlist", err)
		return 0, err
	}
	return r.Add(ids)
}

func (r *playlistTrackRepository) AddAlbums(albumIds []string) (int, error) {
	return r.addMediaFileIds(Eq{"album_id": albumIds})
}

func (r *playlistTrackRepository) AddArtists(artistIds []string) (int, error) {
	return r.addMediaFileIds(Eq{"album_artist_id": artistIds})
}

func (r *playlistTrackRepository) AddDiscs(discs []model.DiscID) (int, error) {
	if len(discs) == 0 {
		return 0, nil
	}
	var clauses Or
	for _, d := range discs {
		clauses = append(clauses, And{Eq{"album_id": d.AlbumID}, Eq{"release_date": d.ReleaseDate}, Eq{"disc_number": d.DiscNumber}})
	}
	return r.addMediaFileIds(clauses)
}

func mergePlaylistTracksWithMissing(ctx context.Context, tracks model.PlaylistTracks, pls *model.Playlist, searchTerm string) (model.PlaylistTracks, error) {
	entries, err := readPlaylistEntries(pls.Path)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return tracks, nil
	}

	normalized := make(map[string][]int, len(tracks))
	for idx, t := range tracks {
		rel := filepath.ToSlash(t.Path)
		key := normalizePlaylistPath(rel)
		normalized[key] = append(normalized[key], idx)
		if t.LibraryPath != "" && t.Path != "" {
			abs := filepath.ToSlash(filepath.Join(t.LibraryPath, t.Path))
			absKey := normalizePlaylistPath(abs)
			normalized[absKey] = append(normalized[absKey], idx)
		}
	}

	used := make([]bool, len(tracks))
	result := make(model.PlaylistTracks, 0, len(entries))
	missingCount := 0

	for _, entry := range entries {
		display := filepath.ToSlash(entry)
		normalizedEntry := normalizePlaylistPath(display)
		matchIdx, ok := popTrackIndex(normalizedEntry, normalized)
		if !ok {
			for key := range normalized {
				if strings.HasSuffix(normalizedEntry, key) {
					if idx, matched := popTrackIndex(key, normalized); matched {
						matchIdx = idx
						ok = true
						break
					}
				}
			}
		}

		if ok && matchIdx >= 0 && matchIdx < len(tracks) && !used[matchIdx] {
			result = append(result, tracks[matchIdx])
			used[matchIdx] = true
			continue
		}

		if searchTerm != "" {
			lowerDisplay := strings.ToLower(display)
			base := strings.ToLower(filepath.Base(display))
			if !strings.Contains(lowerDisplay, searchTerm) && !strings.Contains(base, searchTerm) {
				continue
			}
		}

		missingCount++
		id := fmt.Sprintf("%d", missingCount)
		title := strings.TrimSuffix(filepath.Base(display), filepath.Ext(display))
		if title == "" {
			title = display
		}
		suffix := strings.TrimPrefix(strings.ToLower(filepath.Ext(display)), ".")

		placeholder := model.PlaylistTrack{
			ID:          id,
			MediaFileID: id,
			PlaylistID:  pls.ID,
			MediaFile: model.MediaFile{
				ID:      id,
				Title:   title,
				Path:    display,
				Missing: true,
				Suffix:  suffix,
			},
		}
		result = append(result, placeholder)
	}

	for idx, t := range tracks {
		if used[idx] {
			continue
		}
		result = append(result, t)
	}

	return result, nil
}

func readPlaylistEntries(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	entries := make([]string, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "file://") {
			line = strings.TrimPrefix(line, "file://")
			if decoded, err := url.QueryUnescape(line); err == nil {
				line = decoded
			}
		}
		if !model.IsAudioFile(line) {
			continue
		}
		entries = append(entries, filepath.ToSlash(line))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func normalizePlaylistPath(path string) string {
	return strings.ToLower(norm.NFC.String(path))
}

func popTrackIndex(key string, indexes map[string][]int) (int, bool) {
	list, ok := indexes[key]
	if !ok || len(list) == 0 {
		return 0, false
	}
	idx := list[0]
	if len(list) == 1 {
		delete(indexes, key)
	} else {
		indexes[key] = list[1:]
	}
	return idx, true
}

// Get ids from all current tracks
func (r *playlistTrackRepository) getTracks() ([]string, error) {
	all := r.newSelect().Columns("media_file_id").Where(Eq{"playlist_id": r.playlistId}).OrderBy("id")
	var ids []string
	err := r.queryAllSlice(all, &ids)
	if err != nil {
		log.Error(r.ctx, "Error querying current tracks from playlist", "playlistId", r.playlistId, err)
		return nil, err
	}
	return ids, nil
}

func (r *playlistTrackRepository) Delete(ids ...string) error {
	if !r.isTracksEditable() {
		return rest.ErrPermissionDenied
	}
	err := r.delete(And{Eq{"playlist_id": r.playlistId}, Eq{"id": ids}})
	if err != nil {
		return err
	}

	return r.playlistRepo.renumber(r.playlistId)
}

func (r *playlistTrackRepository) DeleteAll() error {
	if !r.isTracksEditable() {
		return rest.ErrPermissionDenied
	}
	err := r.delete(Eq{"playlist_id": r.playlistId})
	if err != nil {
		return err
	}

	return r.playlistRepo.renumber(r.playlistId)
}

func (r *playlistTrackRepository) Reorder(pos int, newPos int) error {
	if !r.isTracksEditable() {
		return rest.ErrPermissionDenied
	}
	ids, err := r.getTracks()
	if err != nil {
		return err
	}
	newOrder := slice.Move(ids, pos-1, newPos-1)
	return r.playlistRepo.updatePlaylist(r.playlistId, newOrder)
}

var _ model.PlaylistTrackRepository = (*playlistTrackRepository)(nil)
