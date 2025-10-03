package persistence

import (
	"context"
	"fmt"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

type discoveryFolderRepository struct {
	sqlRepository
}

type dbDiscoveryFolder struct {
	model.DiscoveryFolder `structs:",flatten"`
}

func NewDiscoveryFolderRepository(ctx context.Context, db dbx.Builder) model.DiscoveryFolderRepository {
	r := &discoveryFolderRepository{}
	r.ctx = ctx
	r.db = db

	r.registerModel(&model.DiscoveryFolder{}, map[string]filterFunc{
		"q":         r.withTableName(discoveryFolderFilter),
		"parent_id": r.withTableName(discoveryParentIdFilter),
		"owner_id":  r.withTableName(discoveryOwnerIdFilter),
	})
	r.setSortMappings(map[string]string{
		"name":       "lower(discovery_folder.name) asc",
		"owner_name": "owner_name",
		"updated_at": "discovery_folder.updated_at desc",
	})
	return r
}

func discoveryFolderFilter(_ string, value interface{}) Sqlizer {
	return substringFilter("discovery_folder.name", value)
}

func discoveryOwnerIdFilter(_ string, value interface{}) Sqlizer {
	return Eq{"discovery_folder.owner_id": value}
}

func discoveryParentIdFilter(_ string, value interface{}) Sqlizer {
	if value == nil {
		return Eq{"discovery_folder.parent_id": nil}
	}
	if s, ok := value.(string); ok && s == "" {
		return Eq{"discovery_folder.parent_id": nil}
	}
	return Eq{"discovery_folder.parent_id": value}
}

func (r *discoveryFolderRepository) userFilter() Sqlizer {
	user := loggedUser(r.ctx)
	if user.IsAdmin {
		return And{}
	}
	return Or{
		Eq{"discovery_folder.public": true},
		Eq{"discovery_folder.owner_id": user.ID},
	}
}

func (r *discoveryFolderRepository) CountAll(options ...model.QueryOptions) (int64, error) {
	sq := Select().Where(r.userFilter())
	return r.count(sq, options...)
}

func (r *discoveryFolderRepository) Exists(id string) (bool, error) {
	return r.exists(And{Eq{"discovery_folder.id": id}, r.userFilter()})
}

func (r *discoveryFolderRepository) Get(id string) (*model.DiscoveryFolder, error) {
	return r.findBy(And{Eq{"discovery_folder.id": id}, r.userFilter()})
}

func (r *discoveryFolderRepository) GetAll(options ...model.QueryOptions) (model.DiscoveryFolders, error) {
	sel := r.selectFolder(options...).Where(r.userFilter())
	var rows []dbDiscoveryFolder
	if err := r.queryAll(sel, &rows); err != nil {
		return nil, err
	}
	out := make(model.DiscoveryFolders, len(rows))
	for i := range rows {
		out[i] = &rows[i].DiscoveryFolder
	}
	return out, nil
}

func (r *discoveryFolderRepository) GetAllByParent(options ...model.QueryOptions) (model.DiscoveryFolders, error) {
	hasParent := r.hasParentIDFilter(options...)
	sel := r.selectFolder(options...).Where(r.userFilter())
	if !hasParent {
		sel = sel.Where(Eq{"discovery_folder.parent_id": nil})
	}
	var rows []dbDiscoveryFolder
	if err := r.queryAll(sel, &rows); err != nil {
		return nil, err
	}
	out := make(model.DiscoveryFolders, 0, len(rows))
	for i := range rows {
		out = append(out, &rows[i].DiscoveryFolder)
	}
	return out, nil
}

func (r *discoveryFolderRepository) ensureUniqueName(f *model.DiscoveryFolder) error {
	cond := And{
		Eq{"discovery_folder.owner_id": f.OwnerID},
		Eq{"discovery_folder.name": f.Name},
	}
	if f.ParentID == nil {
		cond = append(cond, Eq{"discovery_folder.parent_id": nil})
	} else {
		cond = append(cond, Eq{"discovery_folder.parent_id": *f.ParentID})
	}
	if f.ID != "" {
		cond = append(cond, NotEq{"discovery_folder.id": f.ID})
	}
	exists, err := r.exists(cond)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("unique constraint failed: discovery_folder.name")
	}
	return nil
}

func (r *discoveryFolderRepository) Put(f *model.DiscoveryFolder) error {
	usr := loggedUser(r.ctx)
	if !usr.IsAdmin && f.OwnerID != usr.ID {
		return rest.ErrPermissionDenied
	}
	if f.ID == "" {
		f.CreatedAt = time.Now()
	} else {
		ok, err := r.Exists(f.ID)
		if err != nil {
			return err
		}
		if !ok {
			return model.ErrNotAuthorized
		}
	}
	if err := r.ensureUniqueName(f); err != nil {
		return err
	}
	f.UpdatedAt = time.Now()

	dbf := dbDiscoveryFolder{DiscoveryFolder: *f}
	id, err := r.put(dbf.ID, dbf)
	if err != nil {
		return err
	}
	f.ID = id
	f.Type = "folder"
	return nil
}

func (r *discoveryFolderRepository) Delete(id string) error {
	usr := loggedUser(r.ctx)
	if !usr.IsAdmin {
		existing, err := r.Get(id)
		if err != nil {
			return err
		}
		if existing.OwnerID != usr.ID {
			return rest.ErrPermissionDenied
		}
	}
	return r.delete(And{Eq{"discovery_folder.id": id}, r.userFilter()})
}

func (r *discoveryFolderRepository) UpdateParent(id string, parentId *string) error {
	if err := rejectEmptyOptionalID(parentId); err != nil {
		return err
	}

	usr := loggedUser(r.ctx)

	var src struct {
		OwnerID string
		Name    string
	}
	if err := r.queryOne(Select("owner_id", "name").From("discovery_folder").Where(Eq{"id": id}), &src); err != nil {
		return err
	}
	if !usr.IsAdmin && src.OwnerID != usr.ID {
		return rest.ErrPermissionDenied
	}

	if parentId == nil {
		return r.Update(id, map[string]any{"parentId": nil}, "parentId")
	}

	if *parentId == id {
		return fmt.Errorf("cannot move folder into itself")
	}

	var dst struct{ OwnerID string }
	if err := r.queryOne(Select("owner_id").From("discovery_folder").Where(Eq{"id": *parentId}), &dst); err != nil {
		return err
	}
	if !usr.IsAdmin && dst.OwnerID != usr.ID {
		return rest.ErrPermissionDenied
	}

	return r.Update(id, map[string]any{"parentId": parentId}, "parentId")
}

func (r *discoveryFolderRepository) selectFolder(options ...model.QueryOptions) SelectBuilder {
	return r.newSelect(options...).Join("user on user.id = owner_id").
		Columns(r.tableName+".*", "user.user_name as owner_name")
}

func (r *discoveryFolderRepository) hasParentIDFilter(options ...model.QueryOptions) bool {
	if len(options) == 0 || options[0].Filters == nil {
		return false
	}
	switch f := options[0].Filters.(type) {
	case Eq:
		_, exists := f["parent_id"]
		return exists
	case And:
		for _, sub := range f {
			if eq, ok := sub.(Eq); ok {
				if _, exists := eq["parent_id"]; exists {
					return true
				}
			}
		}
	}
	return false
}

func (r *discoveryFolderRepository) findBy(sql Sqlizer) (*model.DiscoveryFolder, error) {
	sel := r.selectFolder().Where(sql)
	var res []dbDiscoveryFolder
	err := r.queryAll(sel, &res)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, model.ErrNotFound
	}
	f := &res[0].DiscoveryFolder
	f.Type = "folder"
	return f, nil
}

var _ model.DiscoveryFolderRepository = (*discoveryFolderRepository)(nil)
var _ rest.Repository = (*discoveryFolderRepository)(nil)
var _ rest.Persistable = (*discoveryFolderRepository)(nil)
