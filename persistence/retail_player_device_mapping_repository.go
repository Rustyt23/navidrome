package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

type retailPlayerDeviceMappingRepository struct {
	sqlRepository
}

func NewRetailPlayerDeviceMappingRepository(ctx context.Context, db dbx.Builder) model.RetailPlayerDeviceMappingRepository {
	r := &retailPlayerDeviceMappingRepository{}
	r.ctx = ctx
	r.db = db
	r.tableName = "retail_player_device_mapping"
	r.registerModel(&model.RetailPlayerDeviceMapping{}, nil)
	r.ensureRemoteControlColumn()
	return r
}

func (r retailPlayerDeviceMappingRepository) ensureRemoteControlColumn() {
	_, err := r.db.NewQuery(`
ALTER TABLE retail_player_device_mapping
ADD COLUMN IF NOT EXISTS remote_control_id TEXT DEFAULT '';
`).Execute()
	if err != nil {
		log.Error(r.ctx, "Unable to ensure remote control column for retail player device mappings", "err", err)
	}
}

func (r retailPlayerDeviceMappingRepository) Put(ctx context.Context, mapping model.RetailPlayerDeviceMapping) error {
	if mapping.DeviceID == "" {
		return errors.New("retail player device id is required")
	}
	return r.PutMany(ctx, []model.RetailPlayerDeviceMapping{mapping})
}

func (r retailPlayerDeviceMappingRepository) PutMany(ctx context.Context, mappings []model.RetailPlayerDeviceMapping) error {
	if len(mappings) == 0 {
		return nil
	}

	now := time.Now().UTC()

	insert := Insert(r.tableName).
		Columns(
			"device_id",
			"device_name",
			"device_slug",
			"channel",
			"channel_list",
			"organization",
			"time_zone",
			"remote_control_id",
			"updated_at",
		)
	valuesAdded := 0

	for _, mapping := range mappings {
		id := strings.TrimSpace(mapping.DeviceID)
		if id == "" {
			continue
		}

		name := strings.TrimSpace(mapping.DeviceName)
		slug := strings.TrimSpace(mapping.DeviceSlug)
		if slug == "" {
			slug = model.RetailPlayerDeviceSlug(name)
			if slug == "" {
				slug = model.RetailPlayerDeviceSlug(id)
			}
		}

		insert = insert.Values(
			id,
			name,
			slug,
			strings.TrimSpace(mapping.Channel),
			strings.TrimSpace(mapping.ChannelList),
			strings.TrimSpace(mapping.Organization),
			strings.TrimSpace(mapping.TimeZone),
			strings.TrimSpace(mapping.RemoteCtrlID),
			now,
		)
		valuesAdded++
	}

	if valuesAdded == 0 {
		return nil
	}

	insert = insert.Suffix(`ON CONFLICT(device_id) DO UPDATE SET
                device_name = excluded.device_name,
                device_slug = excluded.device_slug,
                channel = excluded.channel,
                channel_list = excluded.channel_list,
                organization = excluded.organization,
                time_zone = excluded.time_zone,
                remote_control_id = COALESCE(NULLIF(excluded.remote_control_id, ''), retail_player_device_mapping.remote_control_id),
                updated_at = excluded.updated_at`)

	_, err := r.executeSQL(insert)
	return err
}

func (r retailPlayerDeviceMappingRepository) FindByIdentifier(ctx context.Context, identifier string) (*model.RetailPlayerDeviceMapping, error) {
	trimmed := model.RetailPlayerNormalizeValue(identifier)
	if trimmed == "" {
		return nil, model.ErrNotFound
	}

	slug := model.RetailPlayerDeviceSlug(trimmed)

	conditions := []Sqlizer{Eq{"device_id": trimmed}}
	conditions = append(conditions, Expr("lower(device_name) = lower(?)", trimmed))
	if slug != "" {
		conditions = append(conditions, Eq{"device_slug": slug})
	}

	orClause := Or{}
	orClause = append(orClause, conditions...)

	sel := Select("device_id", "device_name", "device_slug", "channel", "channel_list", "organization", "time_zone", "remote_control_id", "updated_at").
		From(r.tableName).
		Where(orClause).
		OrderBy("updated_at DESC").
		Limit(1)

	var mapping model.RetailPlayerDeviceMapping
	if err := r.queryOne(sel, &mapping); err != nil {
		return nil, err
	}
	return &mapping, nil
}

func (r retailPlayerDeviceMappingRepository) All(ctx context.Context) ([]model.RetailPlayerDeviceMapping, error) {
	sel := Select("device_id", "device_name", "device_slug", "channel", "channel_list", "organization", "time_zone", "remote_control_id", "updated_at").
		From(r.tableName).
		OrderBy("updated_at DESC")

	var mappings []model.RetailPlayerDeviceMapping
	err := r.queryAll(sel, &mappings)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return mappings, nil
}
