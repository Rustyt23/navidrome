package nativeapi

import (
	"context"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
)

type DiscoveryService struct {
	ds model.DataStore
}

func NewDiscoveryService(ds model.DataStore) *DiscoveryService {
	return &DiscoveryService{ds: ds}
}

func (s *DiscoveryService) Create(ctx context.Context, discovery *model.Discovery) error {
	if user, ok := request.UserFrom(ctx); ok {
		discovery.OwnerID = user.ID
	}
	return s.ds.Discovery(ctx).Put(discovery)
}

func (s *DiscoveryService) Get(ctx context.Context, id string, withTracks bool) (*model.Discovery, error) {
	repo := s.ds.Discovery(ctx)
	if withTracks {
		return repo.GetWithTracks(id, true, false)
	}
	return repo.Get(id)
}

func (s *DiscoveryService) List(ctx context.Context, options ...model.QueryOptions) (model.Discoveries, error) {
	return s.ds.Discovery(ctx).GetAll(options...)
}

func (s *DiscoveryService) Update(ctx context.Context, discovery *model.Discovery) error {
	return s.ds.Discovery(ctx).Put(discovery)
}

func (s *DiscoveryService) Delete(ctx context.Context, id string) error {
	return s.ds.Discovery(ctx).Delete(id)
}

func (s *DiscoveryService) AddTracks(ctx context.Context, discoveryID string, trackIDs []string) (int, error) {
	return s.ds.Discovery(ctx).Tracks(discoveryID, true).Add(trackIDs)
}

func (s *DiscoveryService) RemoveTracks(ctx context.Context, discoveryID string, trackIDs []string) error {
	return s.ds.Discovery(ctx).Tracks(discoveryID, true).Delete(trackIDs...)
}

func (s *DiscoveryService) ReorderTracks(ctx context.Context, discoveryID string, trackID, newPos int) error {
	return s.ds.Discovery(ctx).Tracks(discoveryID, true).Reorder(trackID, newPos)
}
