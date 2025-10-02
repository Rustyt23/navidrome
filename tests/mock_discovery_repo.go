package tests

import (
	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/model"
)

type MockPlaylistRepo struct {
	model.DiscoveryRepository

	Entity *model.Discovery
	Error  error
}

func (m *MockPlaylistRepo) Get(_ string) (*model.Discovery, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if m.Entity == nil {
		return nil, model.ErrNotFound
	}
	return m.Entity, nil
}

func (m *MockPlaylistRepo) Count(_ ...rest.QueryOptions) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	if m.Entity == nil {
		return 0, nil
	}
	return 1, nil
}

func (m *MockPlaylistRepo) GetSyncedByDirectory(string) (model.Discoveries, error) {
	return nil, nil
}
