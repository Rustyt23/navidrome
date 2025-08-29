package tests

import (
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/model"
)

type MockPlaylistFolderRepo struct {
	model.PlaylistFolderRepository
	Folders map[string]*model.PlaylistFolder
}

func NewMockPlaylistFolderRepo() *MockPlaylistFolderRepo {
	return &MockPlaylistFolderRepo{Folders: make(map[string]*model.PlaylistFolder)}
}

func (r *MockPlaylistFolderRepo) GetAll(options ...model.QueryOptions) (model.PlaylistFolders, error) {
	var name, owner string
	var parent *string
	if len(options) > 0 {
		if filters, ok := options[0].Filters.(sq.And); ok {
			for _, f := range filters {
				if eq, ok := f.(sq.Eq); ok {
					if v, ok := eq["playlist_folder.name"]; ok {
						name, _ = v.(string)
					}
					if v, ok := eq["playlist_folder.owner_id"]; ok {
						owner, _ = v.(string)
					}
					if v, ok := eq["playlist_folder.parent_id"]; ok {
						switch t := v.(type) {
						case string:
							parent = &t
						case *string:
							parent = t
						case nil:
							parent = nil
						}
					}
				}
			}
		}
	}
	for _, f := range r.Folders {
		if name != "" && f.Name != name {
			continue
		}
		if owner != "" && f.OwnerID != owner {
			continue
		}
		if (parent == nil && f.ParentID == nil) || (parent != nil && f.ParentID != nil && *f.ParentID == *parent) {
			return model.PlaylistFolders{f}, nil
		}
	}
	return model.PlaylistFolders{}, nil
}

func (r *MockPlaylistFolderRepo) Put(f *model.PlaylistFolder) error {
	if f.ID == "" {
		f.ID = fmt.Sprintf("pf-%d", len(r.Folders)+1)
	}
	now := time.Now()
	if f.CreatedAt.IsZero() {
		f.CreatedAt = now
	}
	f.UpdatedAt = now
	r.Folders[f.ID] = f
	return nil
}
