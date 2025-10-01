package persistence

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/Masterminds/squirrel"
	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/slice"
)

type playlistTrackRepository struct {
	sqlRepository
	playlistId   string
	playlist     *model.Playlist
	playlistRepo *playlistRepository
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
	query := Select().
		LeftJoin("media_file f on f.id = media_file_id").
		Where(Eq{"playlist_id": r.playlistId})
	return r.count(query, r.parseRestOptions(r.ctx, options...))
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
	tracks, err := r.playlistRepo.loadTracks(r.newSelect(options...), r.playlistId)
	if err != nil {
		return nil, err
	}
	return tracks, err
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
	qo := r.parseRestOptions(r.ctx, options...)
	tracks, err := r.GetAll(qo)
	if err != nil {
		return nil, err
	}

	if len(options) > 0 && options[0].Offset > 0 {
		return tracks, nil
	}

	missing, err := r.loadMissingPlaylistTracks(options...)
	if err != nil {
		log.Debug(r.ctx, "Error loading missing playlist tracks", "playlistId", r.playlistId, err)
		return tracks, nil
	}
	if len(missing) > 0 {
		tracks = append(tracks, missing...)
	}
	return tracks, nil
}

func (r *playlistTrackRepository) EntityName() string {
	return "playlist_tracks"
}

func (r *playlistTrackRepository) NewInstance() interface{} {
	return &model.PlaylistTrack{}
}

func (r *playlistTrackRepository) loadMissingPlaylistTracks(options ...rest.QueryOptions) (model.PlaylistTracks, error) {
	if r.playlist == nil || r.playlist.Path == "" {
		return nil, nil
	}
	if conf.Server.DataFolder == "" {
		return nil, nil
	}

	dbFile := filepath.Join(conf.Server.DataFolder, "missing_tracks.db")
	if _, err := os.Stat(dbFile); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_journal_mode=WAL", filepath.ToSlash(dbFile))

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	search := ""
	if len(options) > 0 {
		if title, ok := options[0].Filters["title"].(string); ok {
			search = strings.ToLower(strings.TrimSpace(title))
		}
	}

	columns := []string{"track_path"}
	if columnExists(db, "missing_playlist_tracks", "title") {
		columns = append(columns, "title")
	} else {
		columns = append(columns, "NULL")
	}
	if columnExists(db, "missing_playlist_tracks", "artist") {
		columns = append(columns, "artist")
	} else {
		columns = append(columns, "NULL")
	}

	query := fmt.Sprintf("SELECT %s FROM missing_playlist_tracks WHERE playlist_id = ? ORDER BY id", strings.Join(columns, ", "))
	rows, err := db.QueryContext(r.ctx, query, r.playlist.Path)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	missing := make(model.PlaylistTracks, 0)
	idx := 0
	for rows.Next() {
		var (
			path   string
			title  sql.NullString
			artist sql.NullString
		)
		if err := rows.Scan(&path, &title, &artist); err != nil {
			return nil, err
		}

		displayTitle := fallbackTitle(path, title)
		displayArtist := fallbackArtist(path, artist)

		if search != "" {
			haystack := strings.ToLower(displayTitle + " " + displayArtist + " " + filepath.Base(path))
			if !strings.Contains(haystack, search) {
				continue
			}
		}

		idx++
		id := fmt.Sprintf("missing-%d", idx)
		missing = append(missing, model.PlaylistTrack{
			ID:          id,
			PlaylistID:  r.playlistId,
			MediaFileID: "",
			MediaFile: model.MediaFile{
				ID:              id,
				Path:            path,
				Title:           displayTitle,
				Artist:          displayArtist,
				AlbumArtist:     displayArtist,
				Missing:         true,
				OrderTitle:      displayTitle,
				OrderArtistName: displayArtist,
			},
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return missing, nil
}

func columnExists(db *sql.DB, table, column string) bool {
	query := fmt.Sprintf("SELECT 1 FROM pragma_table_info('%s') WHERE name = ? LIMIT 1", table)
	var dummy int
	err := db.QueryRow(query, column).Scan(&dummy)
	return err == nil
}

func fallbackTitle(path string, title sql.NullString) string {
	if title.Valid && strings.TrimSpace(title.String) != "" {
		return title.String
	}
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	if base == "" {
		return path
	}
	return base
}

func fallbackArtist(path string, artist sql.NullString) string {
	if artist.Valid && strings.TrimSpace(artist.String) != "" {
		return artist.String
	}
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	if base == "" {
		return path
	}
	return base
}

func (r *playlistTrackRepository) isTracksEditable() bool {
	return r.playlistRepo.isWritable(r.playlistId) && !r.playlist.IsSmartPlaylist()
}

func (r *playlistTrackRepository) Add(mediaFileIds []string) (int, error) {
	if !r.isTracksEditable() {
		return 0, rest.ErrPermissionDenied
	}

	if len(mediaFileIds) > 0 {
		log.Debug(r.ctx, "Adding songs to playlist", "playlistId", r.playlistId, "mediaFileIds", mediaFileIds)
	} else {
		return 0, nil
	}

	// Get next pos (ID) in playlist
	sq := r.newSelect().Columns("max(id) as max").Where(Eq{"playlist_id": r.playlistId})
	var res struct{ Max sql.NullInt32 }
	err := r.queryOne(sq, &res)
	if err != nil {
		return 0, err
	}

	return len(mediaFileIds), r.playlistRepo.addTracks(r.playlistId, int(res.Max.Int32+1), mediaFileIds)
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
