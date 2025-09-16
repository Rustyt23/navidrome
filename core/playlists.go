package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/RaveNoX/go-jsoncommentstrip"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/criteria"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/utils/slice"
	"golang.org/x/text/unicode/norm"
)

type Playlists interface {
	ImportFile(ctx context.Context, folder *model.Folder, filename string) (*model.Playlist, error)
	Update(ctx context.Context, playlistID string, name *string, comment *string, public *bool, idsToAdd []string, idxToRemove []int) error
	SetFolder(ctx context.Context, playlistID string, folderID *string) error
	ImportM3U(ctx context.Context, reader io.Reader) (*model.Playlist, error)
}

type playlists struct {
	ds model.DataStore
}

func NewPlaylists(ds model.DataStore) Playlists {
	return &playlists{ds: ds}
}

func InPlaylistsPath(folder model.Folder) bool {
	if conf.Server.PlaylistsPath == "" {
		return true
	}
	rel, _ := filepath.Rel(folder.LibraryPath, folder.AbsolutePath())
	for _, path := range strings.Split(conf.Server.PlaylistsPath, string(filepath.ListSeparator)) {
		if match, _ := doublestar.Match(path, rel); match {
			return true
		}
	}
	return false
}

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

func (s *playlists) ImportFile(ctx context.Context, folder *model.Folder, filename string) (*model.Playlist, error) {
	pls, err := s.parsePlaylist(ctx, filename, folder)
	if err != nil {
		log.Error(ctx, "Error parsing playlist", "path", filepath.Join(folder.AbsolutePath(), filename), err)
		return nil, err
	}
	if fid, err := s.ensurePlaylistFolder(ctx, pls.Path); err == nil {
		pls.FolderID = fid
	} else {
		log.Error(ctx, "Error ensuring playlist folder", "path", pls.Path, err)
	}
	log.Debug("Found playlist", "name", pls.Name, "lastUpdated", pls.UpdatedAt, "path", pls.Path, "numTracks", len(pls.Tracks))
	err = s.updatePlaylist(ctx, pls)
	if err != nil {
		log.Error(ctx, "Error updating playlist", "path", filepath.Join(folder.AbsolutePath(), filename), err)
	}
	return pls, err
}

func (s *playlists) ImportM3U(ctx context.Context, reader io.Reader) (*model.Playlist, error) {
	owner, _ := request.UserFrom(ctx)
	pls := &model.Playlist{
		OwnerID: owner.ID,
		Public:  false,
		Sync:    false,
	}
	err := s.parseM3U(ctx, pls, nil, reader)
	if err != nil {
		log.Error(ctx, "Error parsing playlist", err)
		return nil, err
	}
	err = s.ds.Playlist(ctx).Put(pls)
	if err != nil {
		log.Error(ctx, "Error saving playlist", err)
		return nil, err
	}
	return pls, nil
}

func (s *playlists) parsePlaylist(ctx context.Context, playlistFile string, folder *model.Folder) (*model.Playlist, error) {
	pls, err := s.newSyncedPlaylist(folder.AbsolutePath(), playlistFile)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(pls.Path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	extension := strings.ToLower(filepath.Ext(playlistFile))
	switch extension {
	case ".nsp":
		err = s.parseNSP(ctx, pls, file)
	default:
		err = s.parseM3U(ctx, pls, folder, file)
	}
	return pls, err
}

func (s *playlists) newSyncedPlaylist(baseDir string, playlistFile string) (*model.Playlist, error) {
	playlistPath := filepath.Join(baseDir, playlistFile)
	info, err := os.Stat(playlistPath)
	if err != nil {
		return nil, err
	}

	var extension = filepath.Ext(playlistFile)
	var name = playlistFile[0 : len(playlistFile)-len(extension)]

	pls := &model.Playlist{
		Name:      name,
		Comment:   fmt.Sprintf("Auto-imported from '%s'", playlistFile),
		Public:    false,
		Path:      playlistPath,
		Sync:      true,
		UpdatedAt: info.ModTime(),
	}
	return pls, nil
}

func getPositionFromOffset(data []byte, offset int64) (line, column int) {
	line = 1
	for _, b := range data[:offset] {
		if b == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}
	return
}

func (s *playlists) parseNSP(_ context.Context, pls *model.Playlist, reader io.Reader) error {
	nsp := &nspFile{}
	reader = io.LimitReader(reader, 100*1024) // Limit to 100KB
	reader = jsoncommentstrip.NewReader(reader)
	input, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("reading SmartPlaylist: %w", err)
	}
	err = json.Unmarshal(input, nsp)
	if err != nil {
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			line, col := getPositionFromOffset(input, syntaxErr.Offset)
			return fmt.Errorf("JSON syntax error in SmartPlaylist at line %d, column %d: %w", line, col, err)
		}
		return fmt.Errorf("JSON parsing error in SmartPlaylist: %w", err)
	}
	pls.Rules = &nsp.Criteria
	if nsp.Name != "" {
		pls.Name = nsp.Name
	}
	if nsp.Comment != "" {
		pls.Comment = nsp.Comment
	}
	return nil
}

func (s *playlists) parseM3U(ctx context.Context, pls *model.Playlist, folder *model.Folder, reader io.Reader) error {
	mediaFileRepository := s.ds.MediaFile(ctx)
	var mfs model.MediaFiles
	for lines := range slice.CollectChunks(slice.LinesFrom(reader), 400) {
		filteredLines := make([]string, 0, len(lines))
		for _, line := range lines {
			line := strings.TrimSpace(line)
			if strings.HasPrefix(line, "#PLAYLIST:") {
				pls.Name = line[len("#PLAYLIST:"):]
				continue
			}
			// Skip empty lines and extended info
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, "file://") {
				line = strings.TrimPrefix(line, "file://")
				line, _ = url.QueryUnescape(line)
			}
			if !model.IsAudioFile(line) {
				continue
			}
			filteredLines = append(filteredLines, line)
		}
		paths, err := s.normalizePaths(ctx, pls, folder, filteredLines)
		if err != nil {
			log.Warn(ctx, "Error normalizing paths in playlist", "playlist", pls.Name, err)
			continue
		}
		found, err := mediaFileRepository.FindByPaths(paths)
		if err != nil {
			log.Warn(ctx, "Error reading files from DB", "playlist", pls.Name, err)
			continue
		}
		existing := make(map[string]int, len(found))
		for idx := range found {
			existing[normalizePathForComparison(found[idx].Path)] = idx
		}
		for _, path := range paths {
			idx, ok := existing[normalizePathForComparison(path)]
			if ok {
				mfs = append(mfs, found[idx])
			} else {
				log.Warn(ctx, "Path in playlist not found", "playlist", pls.Name, "path", path)
			}
		}
	}
	if pls.Name == "" {
		pls.Name = time.Now().Format(time.RFC3339)
	}
	pls.Tracks = nil
	pls.AddMediaFiles(mfs)

	return nil
}

// normalizePathForComparison normalizes a file path to NFC form and converts to lowercase
// for consistent comparison. This fixes Unicode normalization issues on macOS where
// Apple Music creates playlists with NFC-encoded paths but the filesystem uses NFD.
func normalizePathForComparison(path string) string {
	return strings.ToLower(norm.NFC.String(path))
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

// TODO This won't work for multiple libraries
func (s *playlists) normalizePaths(ctx context.Context, pls *model.Playlist, folder *model.Folder, lines []string) ([]string, error) {
	libs, err := s.ds.Library(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	res := make([]string, 0, len(lines))
	for idx, line := range lines {
		cleanLine := filepath.Clean(line)
		var relPath string
		found := false

		if filepath.IsAbs(cleanLine) {
			for _, lib := range libs {
				base := filepath.Clean(lib.Path)
				if strings.HasPrefix(cleanLine, base+string(os.PathSeparator)) || cleanLine == base {
					if rel, err := filepath.Rel(base, cleanLine); err == nil {
						if inPlaylistsPath(rel) {
							break
						}
						relPath = rel
						found = true
					}
					break
				}
			}
		} else {
			for _, lib := range libs {
				candidate := filepath.Join(lib.Path, cleanLine)
				if _, err := os.Stat(candidate); err == nil {
					if rel, err := filepath.Rel(lib.Path, candidate); err == nil {
						if inPlaylistsPath(rel) {
							continue
						}
						relPath = rel
						found = true
						break
					}
				} else {
					base := filepath.Base(cleanLine)
					pattern := filepath.Join(lib.Path, "**", base)
					matches, _ := doublestar.FilepathGlob(pattern)
					for _, m := range matches {
						if rel, err := filepath.Rel(lib.Path, m); err == nil {
							if inPlaylistsPath(rel) {
								continue
							}
							relPath = rel
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}
		}

		if found {
			res = append(res, relPath)
		} else {
			log.Warn(ctx, "Path in playlist not found in any library", "path", line, "line", idx)
		}
	}
	return slice.Map(res, filepath.ToSlash), nil
}

func (s *playlists) updatePlaylist(ctx context.Context, newPls *model.Playlist) error {
	owner, _ := request.UserFrom(ctx)

	pls, err := s.ds.Playlist(ctx).FindByPath(newPls.Path)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return err
	}
	if err == nil && !pls.Sync {
		log.Debug(ctx, "Playlist already imported and not synced", "playlist", pls.Name, "path", pls.Path)
		return nil
	}

	if err == nil {
		log.Info(ctx, "Updating synced playlist", "playlist", pls.Name, "path", newPls.Path)
		newPls.ID = pls.ID
		newPls.Name = pls.Name
		newPls.Comment = pls.Comment
		newPls.OwnerID = pls.OwnerID
		newPls.Public = pls.Public
		newPls.EvaluatedAt = &time.Time{}
	} else {
		log.Info(ctx, "Adding synced playlist", "playlist", newPls.Name, "path", newPls.Path, "owner", owner.UserName)
		newPls.OwnerID = owner.ID
		newPls.Public = conf.Server.DefaultPlaylistPublicVisibility
	}
	return s.ds.Playlist(ctx).Put(newPls)
}

func (s *playlists) Update(ctx context.Context, playlistID string,
	name *string, comment *string, public *bool,
	idsToAdd []string, idxToRemove []int) error {
	changed := name != nil || comment != nil || public != nil || len(idsToAdd) > 0 || len(idxToRemove) > 0
	if !changed {
		return nil
	}

	return s.ds.WithTxImmediate(func(tx model.DataStore) error {
		repo := tx.Playlist(ctx)
		pls, err := repo.GetWithTracks(playlistID, true, false)
		if err != nil {
			return err
		}
		oldPath := pls.Path

		if len(idxToRemove) > 0 {
			pls.RemoveTracks(idxToRemove)
		}
		if len(idsToAdd) > 0 {
			pls.AddMediaFilesByID(idsToAdd)
		}

		if name != nil {
			pls.Name = *name
		}
		if comment != nil {
			pls.Comment = *comment
		}
		if public != nil {
			pls.Public = *public
		}

		if pls.Sync {
			ext := filepath.Ext(oldPath)
			if ext == "" {
				ext = ".m3u"
			}
			newPath, err := s.buildPlaylistPath(ctx, tx, pls.FolderID, pls.Name, ext)
			if err != nil {
				return err
			}
			pls.Path = newPath
		}

		if len(idxToRemove) > 0 && len(pls.Tracks) == 0 {
			if err := repo.Tracks(playlistID, true).DeleteAll(); err != nil {
				return err
			}
		}

		if err := repo.Put(pls); err != nil {
			return err
		}

		if pls.Sync {
			if err := os.MkdirAll(filepath.Dir(pls.Path), 0o755); err != nil {
				return err
			}
			if oldPath != "" && oldPath != pls.Path {
				if err := os.Rename(oldPath, pls.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}
			if err := s.writePlaylistFile(pls.Path, pls); err != nil {
				return err
			}
		}
		return nil
	})
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
			if writeErr := s.writePlaylistFile(newPath, pls); writeErr != nil {
				return writeErr
			}
		}
		return nil
	})
}

func (s *playlists) writePlaylistFile(path string, pls *model.Playlist) error {
	data := []byte(pls.ToM3U8())
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	if conf.Server.SyncFolder != "" {
		if err := os.MkdirAll(conf.Server.SyncFolder, 0o755); err != nil {
			return err
		}
		syncPath := filepath.Join(conf.Server.SyncFolder, filepath.Base(path))
		if err := os.WriteFile(syncPath, data, 0o644); err != nil {
			return err
		}
	}
	return nil
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
			parts = append([]string{sanitizeName(f.Name)}, parts...)
			if f.ParentID == nil {
				break
			}
			id = *f.ParentID
		}
		rel = filepath.Join(parts...)
	}
	filename := sanitizeName(name) + ext
	if rel != "" {
		return filepath.Join(root, rel, filename), nil
	}
	return filepath.Join(root, filename), nil
}

type nspFile struct {
	criteria.Criteria
	Name    string `json:"name"`
	Comment string `json:"comment"`
}

func (i *nspFile) UnmarshalJSON(data []byte) error {
	m := map[string]interface{}{}
	err := json.Unmarshal(data, &m)
	if err != nil {
		return err
	}
	i.Name, _ = m["name"].(string)
	i.Comment, _ = m["comment"].(string)
	return json.Unmarshal(data, &i.Criteria)
}
