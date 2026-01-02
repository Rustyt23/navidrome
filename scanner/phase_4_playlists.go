package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	sq "github.com/Masterminds/squirrel"
	ppl "github.com/google/go-pipeline/pkg/pipeline"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core"
	"github.com/navidrome/navidrome/core/artwork"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
)

type phasePlaylists struct {
	ctx       context.Context
	scanState *scanState
	ds        model.DataStore
	pls       core.Playlists
	cw        artwork.CacheWarmer
	refreshed atomic.Uint32
}

func createPhasePlaylists(ctx context.Context, scanState *scanState, ds model.DataStore, pls core.Playlists, cw artwork.CacheWarmer) *phasePlaylists {
	return &phasePlaylists{
		ctx:       ctx,
		scanState: scanState,
		ds:        ds,
		pls:       pls,
		cw:        cw,
	}
}

func (p *phasePlaylists) description() string {
	return "Import/update playlists"
}

func (p *phasePlaylists) producer() ppl.Producer[*model.Folder] {
	return ppl.NewProducer(p.produce, ppl.Name("load folders with playlists from db"))
}

func (p *phasePlaylists) produce(put func(entry *model.Folder)) error {
	if !conf.Server.AutoImportPlaylists {
		log.Info(p.ctx, "Playlists will not be imported, AutoImportPlaylists is set to false")
		return nil
	}
	u, _ := request.UserFrom(p.ctx)
	if !u.IsAdmin {
		log.Warn(p.ctx, "Playlists will not be imported, as there are no admin users yet, "+
			"Please create an admin user first, and then update the playlists for them to be imported")
		return nil
	}

	if conf.Server.PlaylistsPath != "" {
		count := 0
		knownPaths := make(map[string]struct{})
		emitFolder := func(root playlistRoot, relPath, absPath string) {
			absPath = filepath.Clean(absPath)
			if _, ok := knownPaths[absPath]; ok {
				return
			}
			lib := model.Library{ID: 0, Path: root.abs}
			f := model.NewFolder(lib, relPath)
			f.LibraryPath = root.abs
			knownPaths[absPath] = struct{}{}
			count++
			put(f)
		}
		roots := playlistsRoots()
		for _, root := range roots {
			err := filepath.WalkDir(root.abs, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if !d.IsDir() {
					return nil
				}
				if strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				rel, err := filepath.Rel(root.abs, path)
				if err != nil {
					return nil
				}
				emitFolder(root, rel, path)
				return nil
			})
			if err != nil {
				log.Error(p.ctx, "Scanner: error walking playlists path", "path", root.abs, err)
			}
		}
		p.emitMissingPlaylistFolders(roots, knownPaths, emitFolder)
		p.pruneMissingPlaylistFolderRecords(roots, knownPaths)
		if count == 0 {
			log.Debug(p.ctx, "Scanner: No playlists need refreshing")
		} else {
			log.Debug(p.ctx, "Scanner: Found folders with playlists that may need refreshing", "count", count)
		}
		return nil
	}

	count := 0
	cursor, err := p.ds.Folder(p.ctx).GetTouchedWithPlaylists()
	if err != nil {
		return fmt.Errorf("loading touched folders: %w", err)
	}
	log.Debug(p.ctx, "Scanner: Checking playlists that may need refresh")
	for folder, err := range cursor {
		if err != nil {
			return fmt.Errorf("loading touched folder: %w", err)
		}
		count++
		put(&folder)
	}
	if count == 0 {
		log.Debug(p.ctx, "Scanner: No playlists need refreshing")
	} else {
		log.Debug(p.ctx, "Scanner: Found folders with playlists that may need refreshing", "count", count)
	}

	return nil
}

func (p *phasePlaylists) stages() []ppl.Stage[*model.Folder] {
	return []ppl.Stage[*model.Folder]{
		ppl.NewStage(p.processPlaylistsInFolder, ppl.Name("process playlists in folder"), ppl.Concurrency(3)),
	}
}

func (p *phasePlaylists) processPlaylistsInFolder(folder *model.Folder) (*model.Folder, error) {
	if conf.Server.PlaylistsPath != "" {
		p.ensurePlaylistFolderEntry(folder)
	}
	files, err := os.ReadDir(folder.AbsolutePath())
	if err != nil {
		if conf.Server.PlaylistsPath != "" && os.IsNotExist(err) {
			p.removeMissingPlaylistsInFolder(folder)
			return folder, nil
		}
		log.Error(p.ctx, "Scanner: Error reading files", "folder", folder, err)
		p.scanState.sendWarning(err.Error())
		return folder, nil
	}
	playlistRepo := p.ds.Playlist(p.ctx)
	existing, err := playlistRepo.GetSyncedByDirectory(folder.AbsolutePath())
	if err != nil {
		log.Error(p.ctx, "Scanner: Error loading playlists from DB", "folder", folder, err)
		p.scanState.sendWarning(err.Error())
		return folder, nil
	}

	existingByPath := make(map[string]model.Playlist, len(existing))
	for _, pls := range existing {
		existingByPath[filepath.Clean(pls.Path)] = pls
	}

	seen := make(map[string]struct{}, len(files))
	for _, f := range files {
		started := time.Now()
		if strings.HasPrefix(f.Name(), ".") {
			continue
		}
		if !model.IsValidPlaylist(f.Name()) {
			continue
		}
		absPath := filepath.Join(folder.AbsolutePath(), f.Name())
		info, err := f.Info()
		if err != nil {
			log.Warn(p.ctx, "Scanner: Error getting playlist info", "folder", folder, "file", f.Name(), err)
			continue
		}
		seen[filepath.Clean(absPath)] = struct{}{}
		if existingPls, ok := existingByPath[filepath.Clean(absPath)]; ok {
			if !info.ModTime().After(existingPls.UpdatedAt) {
				log.Trace(p.ctx, "Scanner: Playlist unchanged, skipping", "name", existingPls.Name, "path", existingPls.Path)
				continue
			}
		}
		pls, err := p.pls.ImportFile(p.ctx, folder, f.Name())
		if err != nil {
			continue
		}
		if pls.IsSmartPlaylist() {
			log.Debug("Scanner: Imported smart playlist", "name", pls.Name, "lastUpdated", pls.UpdatedAt, "path", pls.Path, "elapsed", time.Since(started))
		} else {
			log.Debug("Scanner: Imported playlist", "name", pls.Name, "lastUpdated", pls.UpdatedAt, "path", pls.Path, "numTracks", len(pls.Tracks), "elapsed", time.Since(started))
		}
		p.cw.PreCache(pls.CoverArtID())
		p.refreshed.Add(1)
	}

	for _, pls := range existingByPath {
		if _, ok := seen[filepath.Clean(pls.Path)]; ok {
			continue
		}
		if err := playlistRepo.Delete(pls.ID); err != nil {
			log.Error(p.ctx, "Scanner: Error removing missing playlist", "playlist", pls.Name, "path", pls.Path, err)
			p.scanState.sendWarning(err.Error())
			continue
		}
		p.scanState.changesDetected.Store(true)
		log.Info(p.ctx, "Scanner: Removed missing playlist", "playlist", pls.Name, "path", pls.Path)
	}
	if len(seen) == 0 {
		p.prunePlaylistFolderPath(folder)
	}
	return folder, nil
}

func (p *phasePlaylists) finalize(err error) error {
	refreshed := p.refreshed.Load()
	logF := log.Info
	if refreshed == 0 {
		logF = log.Debug
	} else {
		p.scanState.changesDetected.Store(true)
	}
	logF(p.ctx, "Scanner: Finished refreshing playlists", "refreshed", refreshed, err)
	return err
}

var _ phase[*model.Folder] = (*phasePlaylists)(nil)

type playlistRoot struct {
	abs string
}

func playlistsRoots() []playlistRoot {
	paths := strings.Split(conf.Server.PlaylistsPath, string(filepath.ListSeparator))
	roots := make([]playlistRoot, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, root := range paths {
		root = strings.TrimSuffix(root, "**")
		root = strings.TrimSuffix(root, string(os.PathSeparator))
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		absRoot = filepath.Clean(absRoot)
		if _, ok := seen[absRoot]; ok {
			continue
		}
		seen[absRoot] = struct{}{}
		roots = append(roots, playlistRoot{abs: absRoot})
	}
	return roots
}

func (p *phasePlaylists) emitMissingPlaylistFolders(roots []playlistRoot, known map[string]struct{}, emit func(root playlistRoot, relPath, absPath string)) {
	playlistRepo := p.ds.Playlist(p.ctx)
	playlists, err := playlistRepo.GetAll()
	if err != nil {
		log.Error(p.ctx, "Scanner: Error loading playlists for folder cleanup", err)
		return
	}
	for _, pls := range playlists {
		if !pls.Sync || pls.Path == "" {
			continue
		}
		absPath, err := filepath.Abs(pls.Path)
		if err != nil {
			continue
		}
		dir := filepath.Clean(filepath.Dir(absPath))
		if _, ok := known[dir]; ok {
			continue
		}
		root, rel, ok := matchPlaylistRoot(roots, dir)
		if !ok {
			continue
		}
		emit(root, rel, dir)
	}
}

func matchPlaylistRoot(roots []playlistRoot, dir string) (playlistRoot, string, bool) {
	for _, root := range roots {
		rel, err := filepath.Rel(root.abs, dir)
		if err != nil {
			continue
		}
		if rel == "." || rel == "" {
			return root, rel, true
		}
		if !strings.HasPrefix(rel, "..") {
			return root, rel, true
		}
	}
	return playlistRoot{}, "", false
}

func (p *phasePlaylists) ensurePlaylistFolderEntry(folder *model.Folder) {
	roots := playlistsRoots()
	_, rel, ok := matchPlaylistRoot(roots, folder.AbsolutePath())
	if !ok || rel == "." || rel == "" {
		return
	}
	owner, _ := request.UserFrom(p.ctx)
	folderRepo := p.ds.PlaylistFolder(p.ctx)
	parts := strings.Split(rel, string(os.PathSeparator))
	var parentID *string
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		filters := sq.And{
			sq.Eq{"playlist_folder.name": part},
			sq.Eq{"playlist_folder.owner_id": owner.ID},
		}
		if parentID == nil {
			filters = append(filters, sq.Eq{"playlist_folder.parent_id": nil})
		} else {
			filters = append(filters, sq.Eq{"playlist_folder.parent_id": *parentID})
		}
		folders, err := folderRepo.GetAll(model.QueryOptions{Filters: filters, Max: 1})
		if err != nil {
			log.Warn(p.ctx, "Scanner: Error resolving playlist folder", "name", part, err)
			return
		}
		if len(folders) > 0 {
			id := folders[0].ID
			parentID = &id
			continue
		}
		newFolder := &model.PlaylistFolder{
			Name:    part,
			OwnerID: owner.ID,
			Public:  conf.Server.DefaultPlaylistPublicVisibility,
		}
		if parentID != nil {
			newFolder.ParentID = parentID
		}
		if err := folderRepo.Put(newFolder); err != nil {
			log.Warn(p.ctx, "Scanner: Error creating playlist folder", "name", part, err)
			return
		}
		p.scanState.changesDetected.Store(true)
		parentID = &newFolder.ID
	}
}

func (p *phasePlaylists) pruneMissingPlaylistFolderRecords(roots []playlistRoot, known map[string]struct{}) {
	if len(known) == 0 {
		return
	}
	folderRepo := p.ds.PlaylistFolder(p.ctx)
	folders, err := folderRepo.GetAll()
	if err != nil {
		log.Warn(p.ctx, "Scanner: Error loading playlist folders for cleanup", err)
		return
	}
	if len(folders) == 0 {
		return
	}
	folderByID := make(map[string]*model.PlaylistFolder, len(folders))
	for _, f := range folders {
		folderByID[f.ID] = f
	}
	cache := make(map[string]string, len(folders))
	var buildPath func(id string) (string, bool)
	buildPath = func(id string) (string, bool) {
		if path, ok := cache[id]; ok {
			return path, true
		}
		folder, ok := folderByID[id]
		if !ok {
			return "", false
		}
		parts := []string{folder.Name}
		for folder.ParentID != nil {
			parent, ok := folderByID[*folder.ParentID]
			if !ok {
				return "", false
			}
			parts = append([]string{parent.Name}, parts...)
			folder = parent
		}
		rel := filepath.Join(parts...)
		cache[id] = rel
		return rel, true
	}
	for _, folder := range folders {
		rel, ok := buildPath(folder.ID)
		if !ok {
			continue
		}
		found := false
		for _, root := range roots {
			absPath := filepath.Join(root.abs, rel)
			if _, ok := known[filepath.Clean(absPath)]; ok {
				found = true
				break
			}
		}
		if found {
			continue
		}
		if err := folderRepo.Delete(folder.ID); err != nil {
			log.Warn(p.ctx, "Scanner: Error removing missing playlist folder", "folder", folder.Name, err)
			continue
		}
		p.scanState.changesDetected.Store(true)
		log.Info(p.ctx, "Scanner: Removed missing playlist folder", "folder", folder.Name)
	}
}

func (p *phasePlaylists) removeMissingPlaylistsInFolder(folder *model.Folder) {
	playlistRepo := p.ds.Playlist(p.ctx)
	missing, err := playlistRepo.GetSyncedByDirectory(folder.AbsolutePath())
	if err != nil {
		log.Error(p.ctx, "Scanner: Error loading playlists from DB", "folder", folder, err)
		p.scanState.sendWarning(err.Error())
		return
	}
	for _, pls := range missing {
		if err := playlistRepo.Delete(pls.ID); err != nil {
			log.Error(p.ctx, "Scanner: Error removing missing playlist", "playlist", pls.Name, "path", pls.Path, err)
			p.scanState.sendWarning(err.Error())
			continue
		}
		p.scanState.changesDetected.Store(true)
		log.Info(p.ctx, "Scanner: Removed missing playlist", "playlist", pls.Name, "path", pls.Path)
	}
	if len(missing) > 0 {
		p.prunePlaylistFolderPath(folder)
	}
}

func (p *phasePlaylists) prunePlaylistFolderPath(folder *model.Folder) {
	if conf.Server.PlaylistsPath == "" {
		return
	}
	if info, err := os.Stat(folder.AbsolutePath()); err == nil && info.IsDir() {
		return
	}
	rel, err := filepath.Rel(folder.LibraryPath, folder.AbsolutePath())
	if err != nil || rel == "." || rel == "" {
		return
	}
	if strings.HasPrefix(rel, "..") {
		return
	}
	owner, _ := request.UserFrom(p.ctx)
	folderRepo := p.ds.PlaylistFolder(p.ctx)
	parts := strings.Split(rel, string(os.PathSeparator))
	var pathFolders []*model.PlaylistFolder
	var parentID *string
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		filters := sq.And{
			sq.Eq{"playlist_folder.name": part},
			sq.Eq{"playlist_folder.owner_id": owner.ID},
		}
		if parentID == nil {
			filters = append(filters, sq.Eq{"playlist_folder.parent_id": nil})
		} else {
			filters = append(filters, sq.Eq{"playlist_folder.parent_id": *parentID})
		}
		folders, err := folderRepo.GetAll(model.QueryOptions{Filters: filters, Max: 1})
		if err != nil {
			log.Warn(p.ctx, "Scanner: Error resolving playlist folder", "name", part, err)
			return
		}
		if len(folders) == 0 {
			return
		}
		current := folders[0]
		pathFolders = append(pathFolders, current)
		parentID = &current.ID
	}
	if len(pathFolders) == 0 {
		return
	}
	p.pruneEmptyPlaylistFolders(pathFolders)
}

func (p *phasePlaylists) pruneEmptyPlaylistFolders(pathFolders []*model.PlaylistFolder) {
	playlistRepo := p.ds.Playlist(p.ctx)
	folderRepo := p.ds.PlaylistFolder(p.ctx)
	for i := len(pathFolders) - 1; i >= 0; i-- {
		folder := pathFolders[i]
		playlists, err := playlistRepo.GetAllByPlaylistFolder(model.QueryOptions{Filters: sq.Eq{"folder_id": folder.ID}, Max: 1})
		if err != nil {
			log.Warn(p.ctx, "Scanner: Error checking playlists in folder", "folder", folder.Name, err)
			return
		}
		if len(playlists) > 0 {
			return
		}
		children, err := folderRepo.GetAllByParent(model.QueryOptions{Filters: sq.Eq{"parent_id": folder.ID}, Max: 1})
		if err != nil {
			log.Warn(p.ctx, "Scanner: Error checking playlist subfolders", "folder", folder.Name, err)
			return
		}
		if len(children) > 0 {
			return
		}
		if err := folderRepo.Delete(folder.ID); err != nil {
			log.Warn(p.ctx, "Scanner: Error removing playlist folder", "folder", folder.Name, err)
			return
		}
		p.scanState.changesDetected.Store(true)
		log.Info(p.ctx, "Scanner: Removed playlist folder", "folder", folder.Name)
	}
}
