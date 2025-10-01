package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	. "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/slice"
	"github.com/pocketbase/dbx"
)

type missingSongNotificationRepository struct {
	sqlRepository
}

type dbMissingSongNotification struct {
	*model.MissingSongNotification `structs:",flatten"`
	Playlists                      string `structs:"playlist_names"`
	dbMediaFile
}

type dbMissingSongNotifications []dbMissingSongNotification

func (m dbMissingSongNotifications) toModels() model.MissingSongNotifications {
	return slice.Map(m, func(item dbMissingSongNotification) model.MissingSongNotification {
		return *item.MissingSongNotification
	})
}

func (m *dbMissingSongNotification) PostScan() error {
	if m.MissingSongNotification == nil {
		m.MissingSongNotification = &model.MissingSongNotification{}
	}
	if err := m.dbMediaFile.PostScan(); err != nil {
		return err
	}
	m.MissingSongNotification.MediaFile = *m.dbMediaFile.MediaFile
	if m.Playlists != "" {
		if err := json.Unmarshal([]byte(m.Playlists), &m.MissingSongNotification.PlaylistNames); err != nil {
			return fmt.Errorf("parsing playlist names: %w", err)
		}
	} else {
		m.MissingSongNotification.PlaylistNames = nil
	}
	return nil
}

func NewMissingSongNotificationRepository(ctx context.Context, db dbx.Builder) model.MissingSongNotificationRepository {
	r := &missingSongNotificationRepository{}
	r.ctx = ctx
	r.db = db
	r.tableName = "missing_song_notification"
	r.registerModel(&model.MissingSongNotification{}, nil)
	r.setSortMappings(map[string]string{
		"detected_at": "detected_at DESC",
	})
	return r
}

func (r *missingSongNotificationRepository) CountAll(options ...model.QueryOptions) (int64, error) {
	query := Select().
		Join("media_file ON media_file.id = " + r.tableName + ".media_file_id")
	return r.count(query, options...)
}

func (r *missingSongNotificationRepository) GetAll(options ...model.QueryOptions) (model.MissingSongNotifications, error) {
	sel := r.newSelect(options...).
		Columns(
			r.tableName+".*",
			"media_file.*",
			"library.path as library_path",
			"library.name as library_name",
		).
		Join("media_file on media_file.id = " + r.tableName + ".media_file_id").
		Join("library on library.id = media_file.library_id")

	var res dbMissingSongNotifications
	if err := r.queryAll(sel, &res, options...); err != nil {
		return nil, err
	}
	return res.toModels(), nil
}

func (r *missingSongNotificationRepository) RefreshForMediaFileIDs(ids ...string) error {
	ids = r.unique(ids...)
	if len(ids) == 0 {
		return nil
	}

	for _, chunk := range slices.Chunk(ids, 200) {
		missingIDs, err := r.loadMissingMediaFileIDs(chunk)
		if err != nil {
			return err
		}
		if len(missingIDs) == 0 {
			continue
		}
		playlists, err := r.loadPlaylistNames(missingIDs)
		if err != nil {
			return err
		}
		if err := r.upsertNotifications(missingIDs, playlists); err != nil {
			return err
		}
	}
	return nil
}

func (r *missingSongNotificationRepository) RefreshForFolders(folderIDs ...string) error {
	folderIDs = r.unique(folderIDs...)
	if len(folderIDs) == 0 {
		return nil
	}
	for _, chunk := range slices.Chunk(folderIDs, 100) {
		ids, err := r.mediaFileIDsByFolders(chunk, true)
		if err != nil {
			return err
		}
		if err := r.RefreshForMediaFileIDs(ids...); err != nil {
			return err
		}
	}
	return nil
}

func (r *missingSongNotificationRepository) Delete(ids ...string) error {
	ids = r.unique(ids...)
	if len(ids) == 0 {
		return nil
	}
	for _, chunk := range slices.Chunk(ids, 200) {
		if _, err := r.executeSQL(Delete(r.tableName).Where(Eq{"media_file_id": chunk})); err != nil {
			return err
		}
	}
	return nil
}

func (r *missingSongNotificationRepository) DeleteByFolders(folderIDs ...string) error {
	folderIDs = r.unique(folderIDs...)
	if len(folderIDs) == 0 {
		return nil
	}
	for _, chunk := range slices.Chunk(folderIDs, 100) {
		ids, err := r.mediaFileIDsByFolders(chunk, false)
		if err != nil {
			return err
		}
		if err := r.Delete(ids...); err != nil {
			return err
		}
	}
	return nil
}

func (r *missingSongNotificationRepository) DeleteAll() error {
	_, err := r.executeSQL(Delete(r.tableName))
	return err
}

func (r *missingSongNotificationRepository) loadMissingMediaFileIDs(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	sel := Select("id").
		From("media_file").
		Where(And{
			Eq{"id": ids},
			Eq{"missing": true},
		})
	var res []string
	if err := r.queryAllSlice(sel, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (r *missingSongNotificationRepository) loadPlaylistNames(ids []string) (map[string][]string, error) {
	if len(ids) == 0 {
		return map[string][]string{}, nil
	}
	type row struct {
		MediaFileID string `db:"media_file_id"`
		Name        string `db:"name"`
	}
	sel := Select(
		"playlist_tracks.media_file_id",
		"playlist.name",
	).
		From("playlist_tracks").
		Join("playlist on playlist.id = playlist_tracks.playlist_id").
		Where(Eq{"playlist_tracks.media_file_id": ids}).
		OrderBy("playlist_tracks.media_file_id", "lower(playlist.name)")
	var rows []row
	if err := r.queryAll(sel, &rows); err != nil {
		return nil, err
	}
	result := make(map[string][]string, len(ids))
	for _, id := range ids {
		result[id] = []string{}
	}
	for _, row := range rows {
		list := result[row.MediaFileID]
		if len(list) == 0 || list[len(list)-1] != row.Name {
			result[row.MediaFileID] = append(result[row.MediaFileID], row.Name)
		}
	}
	return result, nil
}

func (r *missingSongNotificationRepository) upsertNotifications(ids []string, playlists map[string][]string) error {
	for _, id := range ids {
		names := playlists[id]
		sort.Strings(names)
		data, err := json.Marshal(names)
		if err != nil {
			return fmt.Errorf("marshaling playlist names: %w", err)
		}
		stmt := Expr(`
            INSERT INTO missing_song_notification (media_file_id, playlist_names, detected_at)
            VALUES (?, ?, CURRENT_TIMESTAMP)
            ON CONFLICT(media_file_id) DO UPDATE SET playlist_names=excluded.playlist_names;
        `, id, string(data))
		if _, err := r.executeSQL(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (r *missingSongNotificationRepository) mediaFileIDsByFolders(folderIDs []string, onlyMissing bool) ([]string, error) {
	if len(folderIDs) == 0 {
		return nil, nil
	}
	conds := And{Eq{"folder_id": folderIDs}}
	if onlyMissing {
		conds = append(conds, Eq{"missing": true})
	}
	sel := Select("id").From("media_file").Where(conds)
	var res []string
	if err := r.queryAllSlice(sel, &res); err != nil {
		return nil, err
	}
	return res, nil
}

func (r *missingSongNotificationRepository) unique(values ...string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

var _ model.MissingSongNotificationRepository = (*missingSongNotificationRepository)(nil)
