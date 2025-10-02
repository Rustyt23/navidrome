package tests

import (
	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/model"
)

type MockDiscoveryRepo struct {
	model.DiscoveryRepository

	Entity *model.Discovery
	Error  error
}

func (m *MockDiscoveryRepo) Get(_ string) (*model.Discovery, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if m.Entity == nil {
		return nil, model.ErrNotFound
	}
	return m.Entity, nil
}

func (m *MockDiscoveryRepo) Count(_ ...rest.QueryOptions) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	if m.Entity == nil {
		return 0, nil
	}
	return 1, nil
}

func (m *MockDiscoveryRepo) GetSyncedByDirectory(string) (model.Discoveries, error) {
	return nil, nil
}
