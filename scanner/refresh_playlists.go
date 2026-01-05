package scanner

import (
	"context"
	"sync/atomic"

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

	state := scanState{
		fullScan:        false,
		changesDetected: atomic.Bool{},
	}
	phase := createPhasePlaylists(ctx, &state, ds, pls, artwork.NoopCacheWarmer())
	return runPhase[*model.Folder](ctx, 4, phase)()
}
