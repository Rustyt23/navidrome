// persistence/playlist_folder_repository.go
package persistence

import (
	"context"
	"errors"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

type playlistFolderRepository struct {
	sqlRepository
}

type dbPlaylistFolder struct {
	model.PlaylistFolder `structs:",flatten"`
}

func NewPlaylistFolderRepository(ctx context.Context, db dbx.Builder) model.PlaylistFolderRepository {
	r := &playlistFolderRepository{}
	r.ctx = ctx
	r.db = db

	r.registerModel(&model.PlaylistFolder{}, map[string]filterFunc{
		"q":         r.withTableName(playlistFolderFilter),
		"parent_id": r.withTableName(parentIdFilter),
		"owner_id":  r.withTableName(ownerIdFilter),
	})
	r.setSortMappings(map[string]string{
		"name":       "lower(playlist_folder.name) asc",
		"owner_name": "owner_name",
		"updated_at": "playlist_folder.updated_at desc",
	})
	return r
}

func playlistFolderFilter(_ string, value interface{}) Sqlizer {
	return substringFilter("playlist_folder.name", value)
}


func ownerIdFilter(_ string, value interface{}) Sqlizer {
    return Eq{"playlist_folder.owner_id": value}
}

func parentIdFilter(_ string, value interface{}) Sqlizer {
	if value == nil {
		return Eq{"playlist_folder.parent_id": nil}
	}
	if s, ok := value.(string); ok && s == "" {
		return Eq{"playlist_folder.parent_id": nil}
	}
	return Eq{"playlist_folder.parent_id": value}
}

func (r *playlistFolderRepository) userFilter() Sqlizer {
	user := loggedUser(r.ctx)
	if user.IsAdmin {
		return And{}
	}
	return Or{
		Eq{"playlist_folder.public": true},
		Eq{"playlist_folder.owner_id": user.ID},
	}
}

func (r *playlistFolderRepository) CountAll(options ...model.QueryOptions) (int64, error) {
	sq := Select().Where(r.userFilter())
	return r.count(sq, options...)
}

func (r *playlistFolderRepository) Exists(id string) (bool, error) {
	return r.exists(And{Eq{"playlist_folder.id": id}, r.userFilter()})
}

func (r *playlistFolderRepository) Get(id string) (*model.PlaylistFolder, error) {
	return r.findBy(And{Eq{"playlist_folder.id": id}, r.userFilter()})
}

func (r *playlistFolderRepository) GetAll(options ...model.QueryOptions) (model.PlaylistFolders, error) {
	sel := r.selectFolder(options...).Where(r.userFilter())
	var rows []dbPlaylistFolder
	if err := r.queryAll(sel, &rows); err != nil {
		return nil, err
	}
	out := make(model.PlaylistFolders, len(rows))
	for i := range rows {
		out[i] = &rows[i].PlaylistFolder
	}
	return out, nil
}

func (r *playlistFolderRepository) GetAllByParent(options ...model.QueryOptions) (model.PlaylistFolders, error) {
	hasParent := r.hasParentIDFilter(options...)
	sel := r.selectFolder(options...).Where(r.userFilter())
	if !hasParent {
		sel = sel.Where(Eq{"playlist_folder.parent_id": nil})
	}
	var rows []dbPlaylistFolder
	if err := r.queryAll(sel, &rows); err != nil {
		return nil, err
	}
	out := make(model.PlaylistFolders, 0, len(rows))
	for i := range rows {
		out = append(out, &rows[i].PlaylistFolder)
	}
	return out, nil
}

func (r *playlistFolderRepository) Put(f *model.PlaylistFolder) error {
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
	f.UpdatedAt = time.Now()

	dbf := dbPlaylistFolder{PlaylistFolder: *f}
	id, err := r.put(dbf.ID, dbf)
	if err != nil {
		return err
	}
	f.ID = id
	f.Type = "folder"
	return nil
}

func (r *playlistFolderRepository) Delete(id string) error {
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
	// Children cascade via FK; playlists detach via trigger
	return r.delete(And{Eq{"playlist_folder.id": id}, r.userFilter()})
}

// UpdateParent moves a folder under new parent (nil => root). Empty string is invalid.
func (r *playlistFolderRepository) UpdateParent(id string, parentId *string) error {
    if err := rejectEmptyOptionalID(parentId); err != nil { return err }

    usr := loggedUser(r.ctx)

    var src struct{ OwnerID string }
    if err := r.queryOne(Select("owner_id").From("playlist_folder").Where(Eq{"id": id}), &src); err != nil {
        return err
    }
    if !usr.IsAdmin && src.OwnerID != usr.ID {
        return rest.ErrPermissionDenied
    }

    if parentId != nil {
        if *parentId == id {
            return ErrInvalidRequest // self-cycle
        }
        // prevent moving under descendant
        isDesc, err := r.isDescendant(*parentId, id)
        if err != nil { return err }
        if isDesc {
            return ErrInvalidRequest
        }
    }

    upd := Update("playlist_folder").
        Set("parent_id", parentId). // nil => NULL
        Set("updated_at", time.Now()).
        Where(Eq{"id": id})
    _, err := r.executeSQL(upd)
    return err
}

func (r *playlistFolderRepository) selectFolder(options ...model.QueryOptions) SelectBuilder {
	return r.newSelect(options...).
        Join("user on user.id = owner_id").
        Columns(
            r.tableName+".*",
            "user.user_name as owner_name",
            "'folder' as type",
        )
}

func (r *playlistFolderRepository) findBy(where Sqlizer) (*model.PlaylistFolder, error) {
	sel := r.selectFolder().Where(where)
	var rows []dbPlaylistFolder
	if err := r.queryAll(sel, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, model.ErrNotFound
	}
	return &rows[0].PlaylistFolder, nil
}

func (r *playlistFolderRepository) hasParentIDFilter(options ...model.QueryOptions) bool {
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

// climb child->...->root and see if we hit ancestorID
func (r *playlistFolderRepository) isDescendant(childID, ancestorID string) (bool, error) {
	for {
		var row struct{ ParentID *string `db:"parent_id"` }
		err := r.queryOne(
			Select("parent_id").From("playlist_folder").Where(Eq{"id": childID}),
			&row,
		)
		if errors.Is(err, model.ErrNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if row.ParentID == nil {
			return false, nil
		}
		if *row.ParentID == ancestorID {
			return true, nil
		}
		childID = *row.ParentID
	}
}

func (r *playlistFolderRepository) Count(options ...rest.QueryOptions) (int64, error) {
	return r.CountAll(r.parseRestOptions(r.ctx, options...))
}

func (r *playlistFolderRepository) Read(id string) (interface{}, error) {
	return r.Get(id)
}

func (r *playlistFolderRepository) ReadAll(options ...rest.QueryOptions) (interface{}, error) {
	return r.GetAll(r.parseRestOptions(r.ctx, options...))
}

func (r *playlistFolderRepository) EntityName() string { return "playlist_folder" }

func (r *playlistFolderRepository) NewInstance() interface{} { return &model.PlaylistFolder{} }

func (r *playlistFolderRepository) Save(entity interface{}) (string, error) {
	f := entity.(*model.PlaylistFolder)
	f.OwnerID = loggedUser(r.ctx).ID
	f.ID = "" // avoid overriding existing
	if err := r.Put(f); err != nil {
		return "", err
	}
	return f.ID, nil
}

func (r *playlistFolderRepository) Update(id string, entity interface{}, cols ...string) error {
	usr := loggedUser(r.ctx)
	current, err := r.Get(id)
	if err != nil {
		return err
	}
	if !usr.IsAdmin && current.OwnerID != usr.ID {
		return rest.ErrPermissionDenied
	}

	f := entity.(*model.PlaylistFolder)
	if !usr.IsAdmin && f.OwnerID != "" && f.OwnerID != usr.ID {
		return rest.ErrPermissionDenied
	}
	f.ID = id
	f.UpdatedAt = time.Now()
	f.Type = "folder"
	_, err = r.put(id, dbPlaylistFolder{PlaylistFolder: *f}, append(cols, "updatedAt")...)
	if errors.Is(err, model.ErrNotFound) {
		return rest.ErrNotFound
	}
	return err
}

var _ model.PlaylistFolderRepository = (*playlistFolderRepository)(nil)
var _ rest.Repository = (*playlistFolderRepository)(nil)
var _ rest.Persistable = (*playlistFolderRepository)(nil)
