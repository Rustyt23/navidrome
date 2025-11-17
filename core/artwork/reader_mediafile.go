package artwork

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/model"
)

type mediafileArtworkReader struct {
	cacheKey
	a         *artwork
	mediafile model.MediaFile
	album     model.Album
}

func newMediafileArtworkReader(ctx context.Context, artwork *artwork, artID model.ArtworkID) (*mediafileArtworkReader, error) {
	mf, err := artwork.ds.MediaFile(ctx).Get(artID.ID)
	if err != nil {
		return nil, err
	}
	al, err := artwork.ds.Album(ctx).Get(mf.AlbumID)
	if err != nil {
		return nil, err
	}
	a := &mediafileArtworkReader{
		a:         artwork,
		mediafile: *mf,
		album:     *al,
	}
	a.cacheKey.artID = artID
	if al.UpdatedAt.After(mf.UpdatedAt) {
		a.cacheKey.lastUpdate = al.UpdatedAt
	} else {
		a.cacheKey.lastUpdate = mf.UpdatedAt
	}
	return a, nil
}

func (a *mediafileArtworkReader) Key() string {
	return fmt.Sprintf(
		"%s.%t",
		a.cacheKey.Key(),
		conf.Server.EnableMediaFileCoverArt,
	)
}
func (a *mediafileArtworkReader) LastUpdated() time.Time {
	return a.lastUpdate
}

func (a *mediafileArtworkReader) Reader(ctx context.Context) (io.ReadCloser, string, error) {
	var ff []sourceFunc
	if conf.Server.EnableMediaFileCoverArt {
		ff = append(ff, fromStoredMediaArtwork(a.mediafile.ID))
	}
	if a.mediafile.CoverArtID().Kind == model.KindMediaFileArtwork {
		path := a.mediafile.AbsolutePath()
		ff = []sourceFunc{
			fromTag(ctx, path),
			fromFFmpegTag(ctx, a.a.ffmpeg, path),
		}
	}
	ff = append(ff, fromAlbum(ctx, a.a, a.mediafile.AlbumCoverArtID()))
	return selectImageReader(ctx, a.artID, ff...)
}

func fromStoredMediaArtwork(mediafileID string) sourceFunc {
	return func() (io.ReadCloser, string, error) {
		if mediafileID == "" {
			return nil, "", nil
		}
		r, err := OpenMediaArtwork(mediafileID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, "", nil
			}
			return nil, "", err
		}
		return r, mediaArtworkPath(mediafileID), nil
	}
}
