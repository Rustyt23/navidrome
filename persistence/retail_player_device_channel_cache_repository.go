package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

type retailPlayerDeviceChannelCacheRepository struct {
	sqlRepository
}

func NewRetailPlayerDeviceChannelCacheRepository(ctx context.Context, db dbx.Builder) model.RetailPlayerDeviceChannelCacheRepository {
	r := &retailPlayerDeviceChannelCacheRepository{}
	r.ctx = ctx
	r.db = db
	r.tableName = "retail_player_device_channel_cache"
	r.registerModel(&model.RetailPlayerDeviceChannelCache{}, nil)
	return r
}

func (r retailPlayerDeviceChannelCacheRepository) PutMany(ctx context.Context, entries []model.RetailPlayerDeviceChannelCache) error {
	if len(entries) == 0 {
		return nil
	}

	now := time.Now().UTC()

	insert := Insert(r.tableName).
		Columns(
			"device_id",
			"channel_name",
			"channel_list_name",
			"org_unit",
			"updated_at",
		)
	valuesAdded := 0

	for _, entry := range entries {
		id := strings.TrimSpace(entry.DeviceID)
		if id == "" {
			continue
		}

		insert = insert.Values(
			id,
			strings.TrimSpace(entry.ChannelName),
			strings.TrimSpace(entry.ChannelListName),
			strings.TrimSpace(entry.OrgUnit),
			now,
		)
		valuesAdded++
	}

	if valuesAdded == 0 {
		return nil
	}

	insert = insert.Suffix(`ON CONFLICT(device_id) DO UPDATE SET
		channel_name = excluded.channel_name,
		channel_list_name = excluded.channel_list_name,
		org_unit = excluded.org_unit,
		updated_at = excluded.updated_at`)

	_, err := r.executeSQL(insert)
	return err
}

func (r retailPlayerDeviceChannelCacheRepository) FindByDeviceIDs(ctx context.Context, deviceIDs []string) ([]model.RetailPlayerDeviceChannelCache, error) {
	normalized := make([]string, 0, len(deviceIDs))
	for _, id := range deviceIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}

	if len(normalized) == 0 {
		return nil, model.ErrNotFound
	}

	sel := Select("device_id", "channel_name", "channel_list_name", "org_unit", "updated_at").
		From(r.tableName).
		Where(Eq{"device_id": normalized})

	var entries []model.RetailPlayerDeviceChannelCache
	if err := r.queryAll(sel, &entries); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return entries, nil
}
