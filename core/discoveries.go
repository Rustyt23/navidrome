package core

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/storage"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	modelmetadata "github.com/navidrome/navidrome/model/metadata"
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

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	root = absRoot

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
		path := filepath.Clean(disc.Path)
		if !filepath.IsAbs(path) {
			if absPath, err := filepath.Abs(path); err == nil {
				path = absPath
			}
		}
		existingByPath[path] = disc
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
	tracks, err := d.collectTracks(ctx, absPath)
	if err != nil {
		return err
	}
	if len(tracks) == 0 {
		return nil
	}

	usr, ok := request.UserFrom(ctx)
	if !ok {
		return fmt.Errorf("discovery sync requires authenticated user")
	}

	disc := &model.Discovery{}
	if current.ID != "" {
		disc.ID = current.ID
	}
	disc.Name = name
	disc.OwnerID = usr.ID
	disc.Path = absPath
	disc.Comment = current.Comment
	var totalDuration float32
	var totalSize int64
	for _, track := range tracks {
		totalDuration += track.Duration
		totalSize += track.Size
	}
	disc.Duration = totalDuration
	disc.Size = totalSize
	disc.SongCount = len(tracks)

	if err := repo.Put(disc); err != nil {
		return err
	}

	if err := repo.ReplaceTracks(disc.ID, tracks); err != nil {
		return err
	}
	return nil
}

type fileInfoWithBirth struct {
	fs.FileInfo
}

func (f fileInfoWithBirth) BirthTime() time.Time {
	return f.ModTime()
}

type trackCandidate struct {
	abs string
	rel string
}

func (d *discoveries) collectTracks(ctx context.Context, root string) (model.DiscoveryTracks, error) {
	store, err := storage.For(root)
	if err != nil {
		return nil, err
	}
	musicFS, err := store.FS()
	if err != nil {
		return nil, err
	}

	var candidates []trackCandidate
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
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
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		candidates = append(candidates, trackCandidate{
			abs: filepath.Clean(path),
			rel: filepath.ToSlash(rel),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].abs < candidates[j].abs
	})

	relPaths := make([]string, len(candidates))
	absPaths := make([]string, len(candidates))
	for i, cand := range candidates {
		relPaths[i] = cand.rel
		absPaths[i] = cand.abs
	}

	tagResults, err := musicFS.ReadTags(relPaths...)
	if err != nil {
		log.Warn(ctx, "Discovery: unable to read tags", "path", root, err)
	}

	tracks := make(model.DiscoveryTracks, 0, len(absPaths))
	for idx, absPath := range absPaths {
		relPath := relPaths[idx]
		info, ok := tagResults[relPath]
		if !ok {
			info = modelmetadata.Info{}
		}
		if info.FileInfo == nil {
			if stat, statErr := os.Stat(absPath); statErr == nil {
				info.FileInfo = fileInfoWithBirth{FileInfo: stat}
			}
		}
		md := modelmetadata.New(absPath, info)
		title := md.String(model.TagTitle)
		if title == "" {
			base := filepath.Base(absPath)
			title = strings.TrimSuffix(base, filepath.Ext(base))
		}
		artist := md.String(model.TagArtist)
		album := md.String(model.TagAlbum)
		size := int64(0)
		if info.FileInfo != nil {
			size = info.FileInfo.Size()
		}

		tracks = append(tracks, model.DiscoveryTrack{
			Path:     absPath,
			Title:    title,
			Artist:   artist,
			Album:    album,
			Duration: md.Length(),
			Size:     size,
		})
	}
	return tracks, nil
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
