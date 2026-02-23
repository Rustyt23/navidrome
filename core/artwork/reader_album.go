package artwork

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core"
	"github.com/navidrome/navidrome/core/external"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"
)

type albumArtworkReader struct {
	cacheKey
	a             *artwork
	provider      external.Provider
	album         model.Album
	updatedAt     *time.Time
	imgFiles      []string
	rootFolder    string
	mbCoverPath   string
	mbCoverUpdate time.Time
}

func newAlbumArtworkReader(ctx context.Context, artwork *artwork, artID model.ArtworkID, provider external.Provider) (*albumArtworkReader, error) {
	al, err := artwork.ds.Album(ctx).Get(artID.ID)
	if err != nil {
		return nil, err
	}
	_, imgFiles, imagesUpdateAt, err := loadAlbumFoldersPaths(ctx, artwork.ds, *al)
	if err != nil {
		return nil, err
	}
	mbCoverPath, mbCoverUpdate, err := loadAlbumMetadataCover(ctx, artwork.ds, al.ID)
	if err != nil {
		return nil, err
	}
	a := &albumArtworkReader{
		a:             artwork,
		provider:      provider,
		album:         *al,
		updatedAt:     imagesUpdateAt,
		imgFiles:      imgFiles,
		rootFolder:    core.AbsolutePath(ctx, artwork.ds, al.LibraryID, ""),
		mbCoverPath:   mbCoverPath,
		mbCoverUpdate: mbCoverUpdate,
	}
	a.cacheKey.artID = artID
	if a.updatedAt != nil && a.updatedAt.After(al.UpdatedAt) {
		a.cacheKey.lastUpdate = *a.updatedAt
	} else {
		a.cacheKey.lastUpdate = al.UpdatedAt
	}
	if a.mbCoverUpdate.After(a.cacheKey.lastUpdate) {
		a.cacheKey.lastUpdate = a.mbCoverUpdate
	}
	return a, nil
}

func (a *albumArtworkReader) Key() string {
	var hash [16]byte
	if conf.Server.EnableExternalServices {
		hash = md5.Sum([]byte(conf.Server.Agents + conf.Server.CoverArtPriority))
	}
	return fmt.Sprintf(
		"%s.%x.%t",
		a.cacheKey.Key(),
		hash,
		conf.Server.EnableExternalServices,
	)
}
func (a *albumArtworkReader) LastUpdated() time.Time {
	return a.album.UpdatedAt
}

func (a *albumArtworkReader) Reader(ctx context.Context) (io.ReadCloser, string, error) {
	var ff []sourceFunc
	if strings.TrimSpace(a.mbCoverPath) != "" {
		ff = append(ff, fromLocalFile(a.mbCoverPath))
	}
	ff = append(ff, a.fromCoverArtPriority(ctx, a.a.ffmpeg, conf.Server.CoverArtPriority)...)
	return selectImageReader(ctx, a.artID, ff...)
}

func (a *albumArtworkReader) fromCoverArtPriority(ctx context.Context, ffmpeg ffmpeg.FFmpeg, priority string) []sourceFunc {
	var ff []sourceFunc
	for _, pattern := range strings.Split(strings.ToLower(priority), ",") {
		pattern = strings.TrimSpace(pattern)
		switch {
		case pattern == "embedded":
			embedArtPath := filepath.Join(a.rootFolder, a.album.EmbedArtPath)
			ff = append(ff, fromTag(ctx, embedArtPath), fromFFmpegTag(ctx, ffmpeg, embedArtPath))
		case pattern == "external":
			ff = append(ff, fromAlbumExternalSource(ctx, a.album, a.provider))
		case len(a.imgFiles) > 0:
			ff = append(ff, fromExternalFile(ctx, a.imgFiles, pattern))
		}
	}
	return ff
}

func loadAlbumMetadataCover(ctx context.Context, ds model.DataStore, albumID string) (string, time.Time, error) {
	mediaFiles, err := ds.MediaFile(ctx).GetAll(model.QueryOptions{Filters: squirrel.Eq{"album_id": albumID}})
	if err != nil {
		return "", time.Time{}, err
	}

	var coverPath string
	var updatedAt time.Time
	for _, mf := range mediaFiles {
		candidate := strings.TrimSpace(mf.CoverPath)
		if candidate == "" {
			continue
		}
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(conf.Server.DataFolder, candidate)
		}
		if coverPath == "" || candidate < coverPath {
			coverPath = candidate
			updatedAt = mf.UpdatedAt
		}
	}
	return coverPath, updatedAt, nil
}

func loadAlbumFoldersPaths(ctx context.Context, ds model.DataStore, albums ...model.Album) ([]string, []string, *time.Time, error) {
	var folderIDs []string
	for _, album := range albums {
		folderIDs = append(folderIDs, album.FolderIDs...)
	}
	folders, err := ds.Folder(ctx).GetAll(model.QueryOptions{Filters: squirrel.Eq{"folder.id": folderIDs, "missing": false}})
	if err != nil {
		return nil, nil, nil, err
	}
	var paths []string
	var imgFiles []string
	var updatedAt time.Time
	for _, f := range folders {
		path := f.AbsolutePath()
		paths = append(paths, path)
		if f.ImagesUpdatedAt.After(updatedAt) {
			updatedAt = f.ImagesUpdatedAt
		}
		for _, img := range f.ImageFiles {
			imgFiles = append(imgFiles, filepath.Join(path, img))
		}
	}

	// Sort image files to ensure consistent selection of cover art
	// This prioritizes files from lower-numbered disc folders by sorting the paths
	slices.Sort(imgFiles)

	return paths, imgFiles, &updatedAt, nil
}
