package playlists

// This file contains the custom playlist features from the Rustyt23 fork:
// playlist folders (mirrored to the filesystem), publishing playlists back
// to disk (with optional mirroring to the SyncFolder), and recording of
// missing playlist tracks in a side sqlite database.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sq "github.com/Masterminds/squirrel"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/utils/str"
)

func (s *playlists) ensurePlaylistFolder(ctx context.Context, playlistPath string) (*string, error) {
	if conf.Server.PlaylistsPath == "" {
		return nil, nil
	}
	dir := filepath.Dir(playlistPath)
	paths := strings.Split(conf.Server.PlaylistsPath, string(filepath.ListSeparator))
	for _, root := range paths {
		root = strings.TrimSuffix(root, "**")
		root = strings.TrimSuffix(root, string(os.PathSeparator))
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absRoot, dir)
		if err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		if rel == "." {
			return nil, nil
		}
		owner, _ := request.UserFrom(ctx)
		repo := s.ds.PlaylistFolder(ctx)
		parts := strings.Split(rel, string(os.PathSeparator))
		var parentID *string
		for _, p := range parts {
			if p == "" {
				continue
			}
			filters := sq.And{
				sq.Eq{"playlist_folder.name": p},
				sq.Eq{"playlist_folder.owner_id": owner.ID},
			}
			if parentID == nil {
				filters = append(filters, sq.Eq{"playlist_folder.parent_id": nil})
			} else {
				filters = append(filters, sq.Eq{"playlist_folder.parent_id": *parentID})
			}
			folders, err := repo.GetAll(model.QueryOptions{Filters: filters, Max: 1})
			if err != nil {
				return nil, err
			}
			var id string
			if len(folders) > 0 {
				id = folders[0].ID
			} else {
				f := &model.PlaylistFolder{Name: p, OwnerID: owner.ID, Public: conf.Server.DefaultPlaylistPublicVisibility}
				if parentID != nil {
					f.ParentID = parentID
				}
				if err := repo.Put(f); err != nil {
					return nil, err
				}
				id = f.ID
			}
			idCopy := id
			parentID = &idCopy
		}
		return parentID, nil
	}
	return nil, nil
}

func recordMissingPlaylistTrack(ctx context.Context, playlistPath, trackPath string) {
	if trackPath == "" || conf.Server.DataFolder.String() == "" {
		return
	}

	dbFile := filepath.Join(conf.Server.DataFolder.String(), "missing_tracks.db")
	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_journal_mode=WAL", filepath.ToSlash(dbFile))

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		log.Debug(ctx, "Unable to open missing tracks database", "path", dbFile, "err", err)
		return
	}
	defer db.Close()

	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		log.Debug(ctx, "Unable to enable WAL for missing tracks database", "path", dbFile, "err", err)
		return
	}

	_, err = db.Exec(`
CREATE TABLE IF NOT EXISTS missing_playlist_tracks (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        playlist_id TEXT,
        track_path TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
)`)
	if err != nil {
		log.Debug(ctx, "Unable to ensure missing tracks table", "path", dbFile, "err", err)
		return
	}

	if _, err := db.Exec(`INSERT INTO missing_playlist_tracks (playlist_id, track_path) VALUES (?, ?)`, playlistPath, trackPath); err != nil {
		log.Debug(ctx, "Unable to record missing track", "path", dbFile, "err", err)
	}
}

func inPlaylistsPath(rel string) bool {
	if conf.Server.PlaylistsPath == "" {
		return false
	}
	for _, p := range strings.Split(conf.Server.PlaylistsPath, string(filepath.ListSeparator)) {
		if match, _ := doublestar.Match(p, rel); match {
			return true
		}
	}
	return false
}

func (s *playlists) SetFolder(ctx context.Context, playlistID string, folderID *string) error {
	if folderID != nil && *folderID == "" {
		return fmt.Errorf("folderID cannot be empty")
	}
	return s.ds.WithTxImmediate(func(tx model.DataStore) error {
		repo := tx.Playlist(ctx)
		pls, err := repo.Get(playlistID)
		if err != nil {
			return err
		}
		oldPath := pls.Path
		if err := repo.UpdatePlaylistFolder(playlistID, folderID); err != nil {
			return err
		}
		if !pls.Sync {
			return nil
		}
		ext := filepath.Ext(oldPath)
		if ext == "" {
			ext = ".m3u"
		}
		newPath, err := s.buildPlaylistPath(ctx, tx, folderID, pls.Name, ext)
		if err != nil {
			return err
		}
		pls.Path = newPath
		pls.FolderID = folderID
		if err := repo.Put(pls); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
			return err
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if writeErr := s.writePlaylistFile(newPath, pls, false); writeErr != nil {
				return writeErr
			}
		}
		return nil
	})
}

func (s *playlists) writePlaylistFile(path string, pls *model.Playlist, mirrorToSync bool) error {
	data := []byte(pls.ToM3U8())
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	if !mirrorToSync || conf.Server.SyncFolder == "" {
		return nil
	}

	rel := filepath.Base(path)
	if conf.Server.PlaylistsPath != "" {
		paths := strings.Split(conf.Server.PlaylistsPath, string(filepath.ListSeparator))
		for _, root := range paths {
			root = strings.TrimSuffix(root, "**")
			root = strings.TrimSuffix(root, string(os.PathSeparator))
			if absRoot, err := filepath.Abs(root); err == nil {
				root = absRoot
			}
			if r, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(r, "..") {
				rel = r
				break
			}
		}
	}
	syncPath := filepath.Join(conf.Server.SyncFolder, rel)
	if err := os.MkdirAll(filepath.Dir(syncPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(syncPath, data, 0o644); err != nil {
		return err
	}
	return nil
}

func (s *playlists) Publish(ctx context.Context, playlistID string) error {
	return s.ds.WithTxImmediate(func(tx model.DataStore) error {
		repo := tx.Playlist(ctx)
		pls, err := repo.GetWithTracks(playlistID, true, false)
		if err != nil {
			return err
		}
		if !pls.Sync {
			return fmt.Errorf("playlist is not synced")
		}
		if pls.Path == "" {
			return fmt.Errorf("playlist path is empty")
		}
		if err := os.MkdirAll(filepath.Dir(pls.Path), 0o755); err != nil {
			return err
		}
		return s.writePlaylistFile(pls.Path, pls, true)
	})
}

func (s *playlists) buildPlaylistPath(ctx context.Context, ds model.DataStore, folderID *string, name, ext string) (string, error) {
	if conf.Server.PlaylistsPath == "" {
		return "", fmt.Errorf("playlists path not configured")
	}
	paths := strings.Split(conf.Server.PlaylistsPath, string(filepath.ListSeparator))
	root := strings.TrimSuffix(paths[0], "**")
	root = strings.TrimSuffix(root, string(os.PathSeparator))
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	var rel string
	if folderID != nil {
		repo := ds.PlaylistFolder(ctx)
		id := *folderID
		parts := []string{}
		for {
			f, err := repo.Get(id)
			if err != nil {
				return "", err
			}
			parts = append([]string{str.SanitizeFilename(f.Name)}, parts...)
			if f.ParentID == nil {
				break
			}
			id = *f.ParentID
		}
		rel = filepath.Join(parts...)
	}
	filename := str.SanitizeFilename(name) + ext
	if rel != "" {
		return filepath.Join(root, rel, filename), nil
	}
	return filepath.Join(root, filename), nil
}

// syncPlaylistToDisk re-derives the playlist file path from its folder and name,
// renames the old file if needed, and writes the playlist contents to disk.
// It should be called after any change to a synced playlist.
func (s *playlists) syncPlaylistToDisk(ctx context.Context, tx model.DataStore, playlistID string) error {
	repo := tx.Playlist(ctx)
	pls, err := repo.GetWithTracks(playlistID, true, false)
	if err != nil {
		return err
	}
	if !pls.Sync {
		return nil
	}
	oldPath := pls.Path
	ext := filepath.Ext(oldPath)
	if ext == "" {
		ext = ".m3u"
	}
	newPath, err := s.buildPlaylistPath(ctx, tx, pls.FolderID, pls.Name, ext)
	if err != nil {
		return err
	}
	if newPath != oldPath {
		pls.Path = newPath
		if err := repo.Put(pls); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(pls.Path), 0o755); err != nil {
		return err
	}
	if oldPath != "" && oldPath != pls.Path {
		if err := os.Rename(oldPath, pls.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return s.writePlaylistFile(pls.Path, pls, false)
}
