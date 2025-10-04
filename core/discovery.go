package core

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/id"
)

type Discovery interface {
	List(ctx context.Context, refresh bool) (model.DiscoveryPlaylists, error)
	Get(ctx context.Context, id string) (*model.DiscoveryPlaylist, error)
	Export(ctx context.Context, id string, w io.Writer) error
	Refresh(ctx context.Context) (model.DiscoveryPlaylists, error)
}

type discovery struct {
	ds model.DataStore
}

func NewDiscovery(ds model.DataStore) Discovery {
	return &discovery{ds: ds}
}

func (d *discovery) List(ctx context.Context, refresh bool) (model.DiscoveryPlaylists, error) {
	if refresh {
		return d.Refresh(ctx)
	}
	repo := d.ds.DiscoveryPlaylist(ctx)
	playlists, err := repo.GetAll()
	if err != nil {
		return nil, err
	}
	if len(playlists) == 0 && conf.Server.DiscoveryPath != "" {
		return d.Refresh(ctx)
	}
	return playlists, nil
}

func (d *discovery) Refresh(ctx context.Context) (model.DiscoveryPlaylists, error) {
	root := conf.Server.DiscoveryPath
	if root == "" {
		return nil, errors.New("discovery directory not configured")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var playlists model.DiscoveryPlaylists
	now := time.Now().UTC()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		folderPath := filepath.Join(root, entry.Name())
		tracks, err := d.collectTracks(folderPath)
		if err != nil {
			log.Error(ctx, "Error collecting discovery playlist tracks", "folder", folderPath, err)
			continue
		}
		if len(tracks) == 0 {
			continue
		}
		playlists = append(playlists, model.DiscoveryPlaylist{
			ID:         id.NewHash(folderPath),
			Name:       entry.Name(),
			FolderPath: folderPath,
			SongCount:  len(tracks),
			UpdatedAt:  now,
		})
	}
	sort.SliceStable(playlists, func(i, j int) bool {
		return strings.ToLower(playlists[i].Name) < strings.ToLower(playlists[j].Name)
	})
	if err := d.ds.DiscoveryPlaylist(ctx).ReplaceAll(playlists); err != nil {
		return nil, err
	}
	return playlists, nil
}

func (d *discovery) Get(ctx context.Context, id string) (*model.DiscoveryPlaylist, error) {
	entry, err := d.ds.DiscoveryPlaylist(ctx).Get(id)
	if err != nil {
		return nil, err
	}
	tracks, err := d.collectTracks(entry.FolderPath)
	if err != nil {
		return nil, err
	}
	entry.Tracks = tracks
	entry.SongCount = len(tracks)
	return entry, nil
}

func (d *discovery) Export(ctx context.Context, id string, w io.Writer) error {
	entry, err := d.Get(ctx, id)
	if err != nil {
		return err
	}
	builder := &strings.Builder{}
	builder.WriteString("#EXTM3U\n")
	builder.WriteString("#PLAYLIST:" + entry.Name + "\n")
	for _, track := range entry.Tracks {
		builder.WriteString(track)
		builder.WriteString("\n")
	}
	_, err = io.Copy(w, strings.NewReader(builder.String()))
	return err
}

func (d *discovery) collectTracks(folder string) ([]string, error) {
	var tracks []string
	err := filepath.WalkDir(folder, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !model.IsAudioFile(path) {
			return nil
		}
		rel := path
		if conf.Server.MusicFolder != "" {
			if r, relErr := filepath.Rel(conf.Server.MusicFolder, path); relErr == nil && !strings.HasPrefix(r, "..") {
				rel = r
			}
		}
		rel = filepath.ToSlash(rel)
		tracks = append(tracks, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(tracks)
	return tracks, nil
}
