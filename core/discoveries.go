package core

import (
	"context"
	"fmt"

	"github.com/navidrome/navidrome/model"
)

type Discoveries interface {
	Update(ctx context.Context, discoveryID string, name *string, comment *string, public *bool, idsToAdd []string, idxToRemove []int) error
	SetFolder(ctx context.Context, discoveryID string, folderID *string) error
}

type discoveries struct {
	ds model.DataStore
}

func NewDiscoveries(ds model.DataStore) Discoveries {
	return &discoveries{ds: ds}
}

func (s *discoveries) Update(ctx context.Context, discoveryID string, name *string, comment *string, public *bool, idsToAdd []string, idxToRemove []int) error {
	changed := name != nil || comment != nil || public != nil || len(idsToAdd) > 0 || len(idxToRemove) > 0
	if !changed {
		return nil
	}

	return s.ds.WithTxImmediate(func(tx model.DataStore) error {
		repo := tx.Discovery(ctx)
		discovery, err := repo.GetWithSongs(discoveryID, true, false)
		if err != nil {
			return err
		}

		if len(idxToRemove) > 0 {
			discovery.RemoveTracks(idxToRemove)
		}
		if len(idsToAdd) > 0 {
			discovery.AddMediaFilesByID(idsToAdd)
		}

		if name != nil {
			discovery.Name = *name
		}
		if comment != nil {
			discovery.Comment = *comment
		}
		if public != nil {
			discovery.Public = *public
		}

		if len(idxToRemove) > 0 && len(discovery.Tracks) == 0 {
			if err := tx.DiscoverySong(ctx, discoveryID, true).DeleteAll(); err != nil {
				return err
			}
		}

		return repo.Put(discovery)
	})
}

func (s *discoveries) SetFolder(ctx context.Context, discoveryID string, folderID *string) error {
	if folderID != nil && *folderID == "" {
		return fmt.Errorf("folderID cannot be empty")
	}
	return s.ds.WithTxImmediate(func(tx model.DataStore) error {
		return tx.Discovery(ctx).UpdateDiscoveryFolder(discoveryID, folderID)
	})
}
