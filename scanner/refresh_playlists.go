package scanner

import (
	"context"
	"errors"
	"os"
	"sync/atomic"

	"github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/core"
	"github.com/navidrome/navidrome/core/artwork"
	"github.com/navidrome/navidrome/model"
)

// RefreshPlaylists runs the playlist scan phase without scanning the full library.
func RefreshPlaylists(ctx context.Context, ds model.DataStore, pls core.Playlists) error {
	release, err := lockScan(ctx)
	if err != nil {
		return err
	}
	defer release()

	ctx = core.WithPlaylistRefreshPublic(ctx)
	if err := purgeMissingSyncedPlaylists(ctx, ds); err != nil {
		return err
	}
	state := scanState{
		fullScan:        true,
		changesDetected: atomic.Bool{},
	}
	phase := createPhasePlaylists(ctx, &state, ds, pls, artwork.NoopCacheWarmer())
	return runPhase[*model.Folder](ctx, 4, phase)()
}

func purgeMissingSyncedPlaylists(ctx context.Context, ds model.DataStore) error {
	playlists, err := ds.Playlist(ctx).GetAll(model.QueryOptions{
		Filters: squirrel.And{
			squirrel.Eq{"sync": true},
		},
	})
	if err != nil {
		return err
	}
	for _, pls := range playlists {
		if pls.Path == "" {
			continue
		}
		if _, err := os.Stat(pls.Path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if err := ds.Playlist(ctx).Delete(pls.ID); err != nil {
					return err
				}
				continue
			}
			return err
		}
	}
	return nil
}
