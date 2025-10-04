package core

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	Tracks(ctx context.Context, id string) (model.DiscoveryTracks, error)
}

type discovery struct {
	ds model.DataStore
}

func NewDiscovery(ds model.DataStore) Discovery {
	return &discovery{ds: ds}
}

type discoveryTrackEntry struct {
	display  string
	absolute string
}

func (d *discovery) collectTrackEntries(folder string) ([]discoveryTrackEntry, error) {
	entries := make([]discoveryTrackEntry, 0)
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
		absolute := filepath.Clean(path)
		display := absolute
		musicRoot := conf.Server.MusicFolder
		if musicRoot != "" {
			if absRoot, absErr := filepath.Abs(musicRoot); absErr == nil {
				musicRoot = absRoot
			}
			if rel, relErr := filepath.Rel(musicRoot, absolute); relErr == nil && !strings.HasPrefix(rel, "..") {
				display = rel
			}
		}
		display = filepath.ToSlash(display)
		entries = append(entries, discoveryTrackEntry{
			display:  display,
			absolute: absolute,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].display < entries[j].display
	})
	return entries, nil
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
		trackPaths, err := d.collectTrackPaths(folderPath)
		if err != nil {
			log.Error(ctx, "Error collecting discovery playlist tracks", "folder", folderPath, err)
			continue
		}
		if len(trackPaths) == 0 {
			continue
		}
		playlists = append(playlists, model.DiscoveryPlaylist{
			ID:         id.NewHash(folderPath),
			Name:       entry.Name(),
			FolderPath: folderPath,
			SongCount:  len(trackPaths),
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
	tracks, err := d.buildTracks(ctx, entry)
	if err != nil {
		return nil, err
	}
	entry.Tracks = tracks
	entry.SongCount = len(tracks)
	entry.Sync = false
	entry.Duration = 0
	entry.Size = 0
	for _, track := range tracks {
		entry.Duration += track.MediaFile.Duration
		entry.Size += track.MediaFile.Size
	}
	return entry, nil
}

func (d *discovery) Export(ctx context.Context, id string, w io.Writer) error {
	entry, err := d.ds.DiscoveryPlaylist(ctx).Get(id)
	if err != nil {
		return err
	}
	builder := &strings.Builder{}
	builder.WriteString("#EXTM3U\n")
	builder.WriteString("#PLAYLIST:" + entry.Name + "\n")
	trackPaths, err := d.collectTrackPaths(entry.FolderPath)
	if err != nil {
		return err
	}
	for _, track := range trackPaths {
		builder.WriteString(track)
		builder.WriteString("\n")
	}
	_, err = io.Copy(w, strings.NewReader(builder.String()))
	return err
}

func (d *discovery) collectTrackPaths(folder string) ([]string, error) {
	entries, err := d.collectTrackEntries(folder)
	if err != nil {
		return nil, err
	}
	tracks := make([]string, len(entries))
	for i, entry := range entries {
		tracks[i] = entry.display
	}
	return tracks, nil
}

func (d *discovery) libraryRoots(ctx context.Context) []string {
	libraries, err := d.ds.Library(ctx).GetAll()
	if err != nil {
		log.Warn(ctx, "Failed to list libraries for discovery matching", err)
		return nil
	}
	roots := make([]string, 0, len(libraries))
	for _, lib := range libraries {
		if lib.Path == "" {
			continue
		}
		root := lib.Path
		if abs, absErr := filepath.Abs(root); absErr == nil {
			root = abs
		}
		roots = append(roots, filepath.Clean(root))
	}
	return roots
}

func (d *discovery) discoveryPathVariants(displayPath, absolutePath string, libraryRoots []string) []string {
	seen := make(map[string]struct{}, len(libraryRoots)+2)
	variants := make([]string, 0, len(libraryRoots)+2)
	add := func(candidate string) {
		if candidate == "" {
			return
		}
		cleaned := filepath.Clean(candidate)
		cleaned = filepath.ToSlash(cleaned)
		if cleaned == "." {
			return
		}
		if _, ok := seen[cleaned]; ok {
			return
		}
		seen[cleaned] = struct{}{}
		variants = append(variants, cleaned)
	}

	add(displayPath)
	if absolutePath != "" {
		add(absolutePath)
	}
	for _, root := range libraryRoots {
		absRoot := root
		if !filepath.IsAbs(absRoot) {
			if resolved, err := filepath.Abs(absRoot); err == nil {
				absRoot = resolved
			}
		}
		if absolutePath == "" {
			continue
		}
		rel, relErr := filepath.Rel(absRoot, absolutePath)
		if relErr != nil {
			continue
		}
		if strings.HasPrefix(rel, "..") {
			continue
		}
		add(rel)
	}
	return variants
}

func (d *discovery) buildTracks(ctx context.Context, entry *model.DiscoveryPlaylist) (model.DiscoveryTracks, error) {
	entries, err := d.collectTrackEntries(entry.FolderPath)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}
	repo := d.ds.MediaFile(ctx)
	libraryRoots := d.libraryRoots(ctx)
	variantByPath := make(map[string][]string, len(entries))
	lookupSet := make(map[string]struct{}, len(entries)*2)
	lookup := make([]string, 0, len(entries)*2)
	for _, entryPath := range entries {
		normalized := entryPath.display
		variants := d.discoveryPathVariants(entryPath.display, entryPath.absolute, libraryRoots)
		variantByPath[normalized] = variants
		for _, candidate := range variants {
			if _, ok := lookupSet[candidate]; ok {
				continue
			}
			lookupSet[candidate] = struct{}{}
			lookup = append(lookup, candidate)
		}
	}
	mediaFiles, err := repo.FindByPaths(lookup)
	if err != nil {
		return nil, err
	}
	mfByPath := make(map[string]model.MediaFile, len(mediaFiles)*2)
	for _, mf := range mediaFiles {
		key := strings.ToLower(filepath.ToSlash(filepath.Clean(mf.Path)))
		mfByPath[key] = mf
		if absPath := mf.AbsolutePath(); absPath != "" {
			absKey := strings.ToLower(filepath.ToSlash(filepath.Clean(absPath)))
			mfByPath[absKey] = mf
		}
	}
	tracks := make(model.DiscoveryTracks, 0, len(entries))
	for idx, entryPath := range entries {
		normalized := entryPath.display
		variants := variantByPath[normalized]
		var (
			mediaFile model.MediaFile
			found     bool
		)
		for _, candidate := range variants {
			if mf, ok := mfByPath[strings.ToLower(candidate)]; ok {
				mediaFile = mf
				found = true
				break
			}
		}
		if !found {
			fallback := normalized
			if len(variants) > 0 {
				fallback = variants[0]
			}
			mediaFile = model.MediaFile{
				ID:      id.NewHash(entry.ID + fallback),
				Path:    fallback,
				Title:   filepath.Base(fallback),
				Missing: true,
			}
		}
		trackID := mediaFile.ID
		if trackID == "" {
			trackID = id.NewHash(entry.ID + normalized + "#" + strconv.Itoa(idx))
		}
		tracks = append(tracks, model.DiscoveryTrack{
			ID:          trackID,
			DiscoveryID: entry.ID,
			MediaFileID: mediaFile.ID,
			Position:    idx + 1,
			MediaFile:   mediaFile,
		})
	}
	return tracks, nil
}

func (d *discovery) Tracks(ctx context.Context, id string) (model.DiscoveryTracks, error) {
	entry, err := d.ds.DiscoveryPlaylist(ctx).Get(id)
	if err != nil {
		return nil, err
	}
	return d.buildTracks(ctx, entry)
}
