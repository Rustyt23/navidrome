package nativeapi

import (
	"context"

	"github.com/navidrome/navidrome/model"
)

type retailPlayerDeviceMappingRepoStub struct{}

func (retailPlayerDeviceMappingRepoStub) Put(ctx context.Context, mapping model.RetailPlayerDeviceMapping) error {
	return nil
}

func (retailPlayerDeviceMappingRepoStub) PutMany(ctx context.Context, mappings []model.RetailPlayerDeviceMapping) error {
	return nil
}

func (retailPlayerDeviceMappingRepoStub) FindByIdentifier(ctx context.Context, identifier string) (*model.RetailPlayerDeviceMapping, error) {
	return nil, model.ErrNotFound
}

func (retailPlayerDeviceMappingRepoStub) All(ctx context.Context) ([]model.RetailPlayerDeviceMapping, error) {
	return nil, nil
}

func newRetailPlayerDeviceMappingRepoStub() model.RetailPlayerDeviceMappingRepository {
	return retailPlayerDeviceMappingRepoStub{}
}
