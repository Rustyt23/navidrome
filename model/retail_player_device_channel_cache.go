package model

import (
	"context"
	"time"
)

type RetailPlayerDeviceChannelCache struct {
	DeviceID        string    `db:"device_id" json:"deviceId"`
	ChannelName     string    `db:"channel_name" json:"channelName"`
	ChannelListName string    `db:"channel_list_name" json:"channelListName"`
	OrgUnit         string    `db:"org_unit" json:"orgUnit"`
	UpdatedAt       time.Time `db:"updated_at" json:"updatedAt"`
}

type RetailPlayerDeviceChannelCacheRepository interface {
	PutMany(ctx context.Context, entries []RetailPlayerDeviceChannelCache) error
	FindByDeviceIDs(ctx context.Context, deviceIDs []string) ([]RetailPlayerDeviceChannelCache, error)
}
