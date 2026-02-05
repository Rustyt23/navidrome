package model

import (
	"context"
	"strings"
	"time"
)

type RetailPlayerDeviceMapping struct {
	DeviceID     string    `db:"device_id" json:"deviceId"`
	DeviceName   string    `db:"device_name" json:"deviceName"`
	DeviceSlug   string    `db:"device_slug" json:"deviceSlug"`
	IsLocked     bool      `db:"is_locked" json:"isLocked"`
	Channel      string    `db:"channel" json:"channel"`
	ChannelList  string    `db:"channel_list" json:"channelList"`
	Organization string    `db:"organization" json:"organization"`
	TimeZone     string    `db:"time_zone" json:"timeZone"`
	RemoteCtrlID string    `db:"remote_control_id" json:"remoteControlId"`
	UpdatedAt    time.Time `db:"updated_at" json:"updatedAt"`
}

type RetailPlayerDeviceMappingRepository interface {
	Put(ctx context.Context, mapping RetailPlayerDeviceMapping) error
	PutMany(ctx context.Context, mappings []RetailPlayerDeviceMapping) error
	FindByIdentifier(ctx context.Context, identifier string) (*RetailPlayerDeviceMapping, error)
	SetLockState(ctx context.Context, identifier string, isLocked bool) (*RetailPlayerDeviceMapping, error)
	All(ctx context.Context) ([]RetailPlayerDeviceMapping, error)
}

func RetailPlayerNormalizeValue(value string) string {
	return strings.TrimSpace(value)
}

func RetailPlayerDeviceSlug(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))

	lastWasHyphen := false
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastWasHyphen = false
			continue
		}

		if !lastWasHyphen && builder.Len() > 0 {
			builder.WriteRune('-')
			lastWasHyphen = true
		}
	}

	return strings.Trim(builder.String(), "-")
}
