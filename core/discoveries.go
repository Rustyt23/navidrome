package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
)

type Discoveries interface {
	Sync(ctx context.Context) error
}

type discoveries struct {
	ds model.DataStore
}

func NewDiscoveries(ds model.DataStore) Discoveries {
	return &discoveries{ds: ds}
}

func (d *discoveries) Sync(ctx context.Context) error {
	root := conf.Server.DiscoveryPath
	if root == "" {
		return nil
	}
	if err := ensureDir(root); err != nil {
		return err
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	repo := d.ds.Discovery(ctx)
	existing, err := repo.GetAll()
	if err != nil {
		return err
	}

	existingByPath := map[string]model.Discovery{}
	for _, disc := range existing {
		existingByPath[filepath.Clean(disc.Path)] = disc
	}

	seen := map[string]struct{}{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		absPath := filepath.Join(root, entry.Name())
		seen[filepath.Clean(absPath)] = struct{}{}
		if err := d.importDirectory(ctx, repo, existingByPath[filepath.Clean(absPath)], absPath, entry.Name()); err != nil {
			log.Error(ctx, "Discovery: error importing folder", "path", absPath, err)
		}
	}

	for path, disc := range existingByPath {
		if _, ok := seen[path]; ok {
			continue
		}
		if err := repo.Delete(disc.ID); err != nil {
			log.Error(ctx, "Discovery: error removing missing discovery", "path", path, err)
		}
	}
	return nil
}

func (d *discoveries) importDirectory(ctx context.Context, repo model.DiscoveryRepository, current model.Discovery, absPath, name string) error {
	mediaFiles, err := d.collectMediaFiles(ctx, absPath)
	if err != nil {
		return err
	}
	if len(mediaFiles) == 0 {
		return nil
	}

	usr, ok := request.UserFrom(ctx)
	if !ok {
		return errors.New("discovery sync requires authenticated user")
	}

	disc := &model.Discovery{}
	if current.ID != "" {
		disc.ID = current.ID
	}
	disc.Name = name
	disc.OwnerID = usr.ID
	disc.Path = absPath
	disc.Comment = current.Comment
	disc.AddMediaFiles(mediaFiles)

	if err := repo.Put(disc); err != nil {
		return err
	}

	ids := make([]string, len(mediaFiles))
	for i, mf := range mediaFiles {
		ids[i] = mf.ID
	}
	if err := repo.ReplaceTracks(disc.ID, ids); err != nil {
		return err
	}
	return nil
}

func (d *discoveries) collectMediaFiles(ctx context.Context, root string) (model.MediaFiles, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !model.IsAudioFile(entry.Name()) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return d.lookupMediaFiles(ctx, files)
}

func (d *discoveries) lookupMediaFiles(ctx context.Context, files []string) (model.MediaFiles, error) {
	if len(files) == 0 {
		return nil, nil
	}
	libs, err := d.ds.Library(ctx).GetAll()
	if err != nil {
		return nil, err
	}
	mfRepo := d.ds.MediaFile(ctx)
	ordered := make(model.MediaFiles, 0, len(files))
	for _, file := range files {
		rels := possibleRelatives(libs, file)
		if len(rels) == 0 {
			log.Warn(ctx, "Discovery: skipping file outside libraries", "path", file)
			continue
		}
		mfs, err := mfRepo.FindByPaths(rels)
		if err != nil {
			return nil, err
		}
		if len(mfs) == 0 {
			log.Warn(ctx, "Discovery: media file not found", "path", file)
			continue
		}
		ordered = append(ordered, mfs[0])
	}
	return ordered, nil
}

func possibleRelatives(libs model.Libraries, path string) []string {
	rels := []string{}
	for _, lib := range libs {
		rel, err := filepath.Rel(lib.Path, path)
		if err != nil {
			continue
		}
		if strings.HasPrefix(rel, "..") {
			continue
		}
		rels = append(rels, filepath.ToSlash(rel))
	}
	return rels
}

func ensureDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(path, 0o755)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	return nil
}
