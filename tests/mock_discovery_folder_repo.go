package tests

import (
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/model"
)

type MockDiscoveryFolderRepo struct {
	model.DiscoveryFolderRepository
	Folders map[string]*model.DiscoveryFolder
}

func NewMockDiscoveryFolderRepo() *MockDiscoveryFolderRepo {
	return &MockDiscoveryFolderRepo{Folders: make(map[string]*model.DiscoveryFolder)}
}

func (r *MockDiscoveryFolderRepo) GetAll(options ...model.QueryOptions) (model.DiscoveryFolders, error) {
	var name, owner string
	var parent *string
	if len(options) > 0 {
		if filters, ok := options[0].Filters.(sq.And); ok {
			for _, f := range filters {
				if eq, ok := f.(sq.Eq); ok {
					if v, ok := eq["discovery_folder.name"]; ok {
						name, _ = v.(string)
					}
					if v, ok := eq["discovery_folder.owner_id"]; ok {
						owner, _ = v.(string)
					}
					if v, ok := eq["discovery_folder.parent_id"]; ok {
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
			return model.DiscoveryFolders{f}, nil
		}
	}
	return model.DiscoveryFolders{}, nil
}

func (r *MockDiscoveryFolderRepo) Put(f *model.DiscoveryFolder) error {
	if f.ID == "" {
		f.ID = fmt.Sprintf("df-%d", len(r.Folders)+1)
	}
	now := time.Now()
	if f.CreatedAt.IsZero() {
		f.CreatedAt = now
	}
	f.UpdatedAt = now
	r.Folders[f.ID] = f
	return nil
}
