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
		for _, root := range strings.Split(conf.Server.PlaylistsPath, string(filepath.ListSeparator)) {
			root = strings.TrimSuffix(root, "**")
			root = strings.TrimSuffix(root, string(os.PathSeparator))
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if !d.IsDir() {
					return nil
				}
				if strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				rel, _ := filepath.Rel(root, path)
				lib := model.Library{ID: 0, Path: root}
				f := model.NewFolder(lib, rel)
				f.LibraryPath = root
				count++
				put(f)
				return nil
			})
			if err != nil {
				log.Error(p.ctx, "Scanner: error walking playlists path", "path", root, err)
			}
		}
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
	files, err := os.ReadDir(folder.AbsolutePath())
	if err != nil {
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
