package model

import (
	"context"
	"time"
)

type RetailPlayerFolder struct {
	ID        string    `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	ParentID  *string   `db:"parent_id" json:"parentId,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

type RetailPlayerDeviceFolder struct {
	DeviceID  string    `db:"device_id" json:"deviceId"`
	FolderID  string    `db:"folder_id" json:"folderId"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

type RetailPlayerFolderRepository interface {
	List(ctx context.Context) ([]RetailPlayerFolder, error)
	Upsert(ctx context.Context, folder RetailPlayerFolder) (RetailPlayerFolder, error)
	DeleteMany(ctx context.Context, ids []string) error
	Assignments(ctx context.Context) ([]RetailPlayerDeviceFolder, error)
	ReplaceDeviceAssignments(ctx context.Context, deviceID string, folderIDs []string) error
}
