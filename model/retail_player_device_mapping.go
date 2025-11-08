package model

import "time"

type RetailPlayerDeviceMapping struct {
	ID         string    `structs:"id"         json:"id"`
	SlugKey    string    `structs:"slug_key"    json:"slugKey"`
	Identifier string    `structs:"identifier" json:"identifier"`
	DeviceID   string    `structs:"device_id"   json:"deviceId"`
	CreatedAt  time.Time `structs:"created_at"  json:"createdAt"`
	UpdatedAt  time.Time `structs:"updated_at"  json:"updatedAt"`
}

type RetailPlayerDeviceMappingRepository interface {
	FindBySlugKey(slugKey string) (*RetailPlayerDeviceMapping, error)
}
