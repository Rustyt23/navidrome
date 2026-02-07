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

type retailPlayerFolderRepository struct {
	sqlRepository
}

func NewRetailPlayerFolderRepository(ctx context.Context, db dbx.Builder) model.RetailPlayerFolderRepository {
	r := &retailPlayerFolderRepository{}
	r.ctx = ctx
	r.db = db
	r.tableName = "retail_player_folder"
	r.registerModel(&model.RetailPlayerFolder{}, nil)
	return r
}

func (r retailPlayerFolderRepository) List(ctx context.Context) ([]model.RetailPlayerFolder, error) {
	sel := Select("id", "name", "parent_id", "is_locked", "created_at", "updated_at").
		From(r.tableName).
		OrderBy("lower(name) asc", "created_at asc")

	var folders []model.RetailPlayerFolder
	err := r.queryAll(sel, &folders)
	if errors.Is(err, model.ErrNotFound) {
		return nil, nil
	}
	return folders, err
}

func (r retailPlayerFolderRepository) Find(ctx context.Context, id string) (model.RetailPlayerFolder, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return model.RetailPlayerFolder{}, errors.New("retail player folder id is required")
	}

	sel := Select("id", "name", "parent_id", "is_locked", "created_at", "updated_at").
		From(r.tableName).
		Where(Eq{"id": trimmed}).
		Limit(1)

	var folder model.RetailPlayerFolder
	if err := r.queryOne(sel, &folder); err != nil {
		return model.RetailPlayerFolder{}, err
	}

	return folder, nil
}

func (r retailPlayerFolderRepository) Upsert(ctx context.Context, folder model.RetailPlayerFolder) (model.RetailPlayerFolder, error) {
	id := strings.TrimSpace(folder.ID)
	if id == "" {
		return model.RetailPlayerFolder{}, errors.New("retail player folder id is required")
	}

	name := strings.TrimSpace(folder.Name)
	if name == "" {
		return model.RetailPlayerFolder{}, errors.New("retail player folder name is required")
	}

	var parentID *string
	if folder.ParentID != nil {
		trimmedParent := strings.TrimSpace(*folder.ParentID)
		if trimmedParent != "" && trimmedParent != id {
			parentID = &trimmedParent
		}
	}

	now := time.Now().UTC()

	insert := Insert(r.tableName).
		Columns("id", "name", "parent_id", "is_locked", "created_at", "updated_at").
		Values(id, name, parentID, folder.IsLocked, now, now).
		Suffix(`ON CONFLICT(id) DO UPDATE SET
            name = excluded.name,
            parent_id = excluded.parent_id,
            is_locked = excluded.is_locked,
            updated_at = excluded.updated_at`)

	if _, err := r.executeSQL(insert); err != nil {
		return model.RetailPlayerFolder{}, err
	}

	sel := Select("id", "name", "parent_id", "is_locked", "created_at", "updated_at").
		From(r.tableName).
		Where(Eq{"id": id}).
		Limit(1)

	var stored model.RetailPlayerFolder
	if err := r.queryOne(sel, &stored); err != nil {
		return model.RetailPlayerFolder{}, err
	}

	return stored, nil
}

func (r retailPlayerFolderRepository) DeleteMany(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	trimmed := make([]string, 0, len(ids))
	for _, id := range ids {
		if value := strings.TrimSpace(id); value != "" {
			trimmed = append(trimmed, value)
		}
	}

	if len(trimmed) == 0 {
		return nil
	}

	del := Delete(r.tableName).Where(Eq{"id": trimmed})
	_, err := r.executeSQL(del)
	return err
}

func (r retailPlayerFolderRepository) Assignments(ctx context.Context) ([]model.RetailPlayerDeviceFolder, error) {
	sel := Select("device_id", "folder_id", "created_at", "updated_at").
		From("retail_player_device_folder")

	var assignments []model.RetailPlayerDeviceFolder
	err := r.queryAll(sel, &assignments)
	if errors.Is(err, model.ErrNotFound) {
		return nil, nil
	}
	return assignments, err
}

func (r retailPlayerFolderRepository) ReplaceDeviceAssignments(ctx context.Context, deviceID string, folderIDs []string) error {
	trimmedID := strings.TrimSpace(deviceID)
	if trimmedID == "" {
		return errors.New("retail player device id is required")
	}

	if _, err := r.executeSQL(Delete("retail_player_device_folder").Where(Eq{"device_id": trimmedID})); err != nil {
		return err
	}

	normalized := make([]string, 0, len(folderIDs))
	for _, id := range folderIDs {
		if value := strings.TrimSpace(id); value != "" {
			normalized = append(normalized, value)
		}
	}

	if len(normalized) == 0 {
		return nil
	}

	now := time.Now().UTC()
	insert := Insert("retail_player_device_folder").
		Columns("device_id", "folder_id", "created_at", "updated_at")

	for _, folderID := range normalized {
		insert = insert.Values(trimmedID, folderID, now, now)
	}

	insert = insert.Suffix(`ON CONFLICT(device_id, folder_id) DO UPDATE SET
        updated_at = excluded.updated_at`)

	_, err := r.executeSQL(insert)
	return err
}
