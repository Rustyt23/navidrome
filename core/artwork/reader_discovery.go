package artwork

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/navidrome/navidrome/model"
)

type discoveryArtworkReader struct {
	cacheKey
	a         *artwork
	discovery model.Discovery
	leadTrack *model.DiscoveryTrack
}

func newDiscoveryArtworkReader(ctx context.Context, artwork *artwork, artID model.ArtworkID) (*discoveryArtworkReader, error) {
	disc, err := artwork.ds.Discovery(ctx).Get(artID.ID)
	if err != nil {
		return nil, err
	}

	tracksRepo := artwork.ds.Discovery(ctx).Tracks(disc.ID)
	tracks, err := tracksRepo.GetAll(model.QueryOptions{Max: 1})
	if err != nil {
		return nil, err
	}
	if len(tracks) == 0 {
		return nil, ErrUnavailable
	}

	reader := &discoveryArtworkReader{
		a:         artwork,
		discovery: *disc,
		leadTrack: &tracks[0],
	}
	reader.cacheKey.artID = artID
	reader.cacheKey.lastUpdate = disc.UpdatedAt
	return reader, nil
}

func (d *discoveryArtworkReader) LastUpdated() time.Time {
	return d.lastUpdate
}

func (d *discoveryArtworkReader) Reader(ctx context.Context) (io.ReadCloser, string, error) {
	if d.leadTrack == nil {
		return nil, "", ErrUnavailable
	}

	trackPath := d.leadTrack.Path
	ff := []sourceFunc{
		fromTag(ctx, os.DirFS(filepath.Dir(trackPath)), filepath.Base(trackPath)),
		fromFFmpegTag(ctx, d.a.ffmpeg, trackPath),
		fromAlbumPlaceholder(),
	}

	return selectImageReader(ctx, d.artID, ff...)
}
