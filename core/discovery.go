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
	"github.com/navidrome/navidrome/core/storage"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/id"
	"github.com/navidrome/navidrome/model/metadata"
	"golang.org/x/text/unicode/norm"
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
		if !filepath.IsAbs(absolute) {
			if absPath, err := filepath.Abs(absolute); err == nil {
				absolute = absPath
			}
		}
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
	type playlistBundle struct {
		playlist model.DiscoveryPlaylist
		tracks   model.DiscoveryTracks
	}
	bundles := make([]playlistBundle, 0, len(entries))
	now := time.Now().UTC()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		folderPath := filepath.Join(root, entry.Name())
		playlist := model.DiscoveryPlaylist{
			ID:         id.NewHash(folderPath),
			Name:       entry.Name(),
			FolderPath: folderPath,
			UpdatedAt:  now,
		}
		tracks, scanErr := d.scanTracks(ctx, d.ds, &playlist)
		if scanErr != nil {
			log.Error(ctx, "Error collecting discovery playlist tracks", "folder", folderPath, scanErr)
			continue
		}
		if len(tracks) == 0 {
			continue
		}
		playlist.SongCount = len(tracks)
		playlist.Duration = 0
		playlist.Size = 0
		for _, track := range tracks {
			playlist.Duration += track.MediaFile.Duration
			playlist.Size += track.MediaFile.Size
		}
		bundles = append(bundles, playlistBundle{playlist: playlist, tracks: tracks})
	}
	sort.SliceStable(bundles, func(i, j int) bool {
		return strings.ToLower(bundles[i].playlist.Name) < strings.ToLower(bundles[j].playlist.Name)
	})
	storeErr := d.ds.WithTx(func(tx model.DataStore) error {
		plain := make(model.DiscoveryPlaylists, 0, len(bundles))
		for _, bundle := range bundles {
			plain = append(plain, model.DiscoveryPlaylist{
				ID:         bundle.playlist.ID,
				Name:       bundle.playlist.Name,
				FolderPath: bundle.playlist.FolderPath,
				SongCount:  bundle.playlist.SongCount,
				UpdatedAt:  bundle.playlist.UpdatedAt,
			})
		}
		if err := tx.DiscoveryPlaylist(ctx).ReplaceAll(plain); err != nil {
			return err
		}
		for _, bundle := range bundles {
			if err := tx.DiscoveryTrack(ctx).ReplaceForDiscovery(bundle.playlist.ID, bundle.tracks); err != nil {
				return err
			}
		}
		return nil
	})
	if storeErr != nil {
		return nil, storeErr
	}
	playlists := make(model.DiscoveryPlaylists, 0, len(bundles))
	for _, bundle := range bundles {
		playlists = append(playlists, bundle.playlist)
	}
	return playlists, nil
}

func (d *discovery) Get(ctx context.Context, id string) (*model.DiscoveryPlaylist, error) {
	entry, err := d.ds.DiscoveryPlaylist(ctx).Get(id)
	if err != nil {
		return nil, err
	}
	tracks, err := d.loadTracks(ctx, entry)
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

func (d *discovery) libraryRoots(ctx context.Context, ds model.DataStore) []string {
	libraries, err := ds.Library(ctx).GetAll()
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
		cleaned := filepath.ToSlash(filepath.Clean(candidate))
		if cleaned == "." {
			return
		}
		for _, normalized := range []string{cleaned, norm.NFC.String(cleaned), norm.NFD.String(cleaned)} {
			if normalized == "." {
				continue
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			variants = append(variants, normalized)
		}
	}

	add(displayPath)
	if absolutePath != "" {
		add(absolutePath)
	}
	basePath := absolutePath
	for _, root := range libraryRoots {
		absRoot := root
		if !filepath.IsAbs(absRoot) {
			if resolved, err := filepath.Abs(absRoot); err == nil {
				absRoot = resolved
			}
		}
		if basePath == "" {
			continue
		}
		rel, relErr := filepath.Rel(absRoot, basePath)
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

func (d *discovery) loadMetadataForMissing(ctx context.Context, entry *model.DiscoveryPlaylist, missing map[int]discoveryTrackEntry) (map[int]model.MediaFile, error) {
	if len(missing) == 0 {
		return nil, nil
	}
	store, err := storage.For(entry.FolderPath)
	if err != nil {
		return nil, err
	}
	fs, err := store.FS()
	if err != nil {
		return nil, err
	}
	pathToIndices := make(map[string][]int, len(missing))
	paths := make([]string, 0, len(missing))
	for idx, entryPath := range missing {
		raw := entryPath.absolute
		if raw == "" {
			raw = entryPath.display
		}
		if raw == "" {
			continue
		}
		cleaned := filepath.Clean(raw)
		rel := cleaned
		if filepath.IsAbs(cleaned) {
			if relPath, relErr := filepath.Rel(entry.FolderPath, cleaned); relErr == nil {
				rel = filepath.Clean(relPath)
			}
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "..\\") {
			continue
		}
		if _, ok := pathToIndices[rel]; !ok {
			paths = append(paths, rel)
		}
		pathToIndices[rel] = append(pathToIndices[rel], idx)
	}
	if len(paths) == 0 {
		return nil, nil
	}
	infoByPath, err := fs.ReadTags(paths...)
	if err != nil {
		return nil, err
	}
	resolved := make(map[int]model.MediaFile, len(missing))
	for relPath, indices := range pathToIndices {
		info, ok := infoByPath[relPath]
		if !ok {
			if alt, altOk := infoByPath[filepath.ToSlash(relPath)]; altOk {
				info = alt
				ok = true
			}
		}
		absolutePath := filepath.Clean(filepath.Join(entry.FolderPath, relPath))
		if !ok {
			// Some extractors may echo back OS-specific separators. Try the joined absolute path.
			joined := filepath.ToSlash(absolutePath)
			if altInfo, altOk := infoByPath[joined]; altOk {
				info = altInfo
				ok = true
			}
		}
		if !ok {
			continue
		}
		md := metadata.New(absolutePath, info)
		mf := md.ToMediaFile(0, entry.ID)
		if mf.ID == "" {
			mf.ID = id.NewHash(entry.ID + absolutePath)
		}
		mf.Missing = false
		mf.LibraryPath = absolutePath
		for _, idx := range indices {
			entryPath := missing[idx]
			clone := mf
			if entryPath.display != "" {
				clone.Path = entryPath.display
			}
			if clone.Path == "" {
				clone.Path = entryPath.absolute
			}
			if clone.Path == "" {
				clone.Path = absolutePath
			}
			if entryPath.absolute != "" {
				clone.LibraryPath = entryPath.absolute
			}
			if clone.ID == "" {
				clone.ID = id.NewHash(entry.ID + clone.Path)
			}
			resolved[idx] = clone
		}
	}
	return resolved, nil
}

func (d *discovery) scanTracks(ctx context.Context, ds model.DataStore, entry *model.DiscoveryPlaylist) (model.DiscoveryTracks, error) {
	entries, err := d.collectTrackEntries(entry.FolderPath)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}
	repo := ds.MediaFile(ctx)
	libraryRoots := d.libraryRoots(ctx, ds)
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
	missing := make(map[int]discoveryTrackEntry)
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
				ID:          id.NewHash(entry.ID + fallback),
				Path:        fallback,
				LibraryPath: entryPath.absolute,
				Title:       filepath.Base(fallback),
				Missing:     true,
			}
			missing[idx] = entryPath
		}
		if mediaFile.Path == "" {
			mediaFile.Path = normalized
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
	if len(missing) > 0 {
		metadataByIndex, metaErr := d.loadMetadataForMissing(ctx, entry, missing)
		if metaErr != nil {
			log.Warn(ctx, "Discovery: Failed to extract metadata for tracks", metaErr)
		}
		for idx, mf := range metadataByIndex {
			track := &tracks[idx]
			track.MediaFile = mf
			track.MediaFileID = mf.ID
			if track.MediaFileID == "" {
				track.MediaFileID = track.ID
			}
		}
	}
	return tracks, nil
}

func (d *discovery) loadTracks(ctx context.Context, entry *model.DiscoveryPlaylist) (model.DiscoveryTracks, error) {
	repo := d.ds.DiscoveryTrack(ctx)
	tracks, err := repo.GetByDiscovery(entry.ID)
	if err != nil {
		return nil, err
	}
	if len(tracks) > 0 {
		return tracks, nil
	}
	return d.rescanAndPersist(ctx, entry)
}

func (d *discovery) rescanAndPersist(ctx context.Context, entry *model.DiscoveryPlaylist) (model.DiscoveryTracks, error) {
	var tracks model.DiscoveryTracks
	err := d.ds.WithTx(func(tx model.DataStore) error {
		scanned, scanErr := d.scanTracks(ctx, tx, entry)
		if scanErr != nil {
			return scanErr
		}
		if err := tx.DiscoveryTrack(ctx).ReplaceForDiscovery(entry.ID, scanned); err != nil {
			return err
		}
		tracks = scanned
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tracks, nil
}

func (d *discovery) Tracks(ctx context.Context, id string) (model.DiscoveryTracks, error) {
	entry, err := d.ds.DiscoveryPlaylist(ctx).Get(id)
	if err != nil {
		return nil, err
	}
	return d.loadTracks(ctx, entry)
}
