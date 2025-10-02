package model

import "time"

type DiscoveryFolder struct {
	ID                 string     `structs:"id" json:"id"`
	Name               string     `structs:"name" json:"name"`
	ParentID           *string    `structs:"parent_id" json:"parentId,omitempty"`
	Public             bool       `structs:"public" json:"public"`
	OwnerID            string     `structs:"owner_id" json:"ownerId"`
	OwnerName          string     `structs:"-" json:"ownerName"`
	CreatedAt          time.Time  `structs:"created_at" json:"createdAt"`
	UpdatedAt          time.Time  `structs:"updated_at" json:"updatedAt"`

    Type               string     `structs:"-" json:"type,omitempty"`

	// for tree payload (for API responses)
	Children  []*DiscoveryFolder `structs:"-" json:"children,omitempty"`
	Discoveries []*Discovery       `structs:"-" json:"discoveries,omitempty"`
}

type DiscoveryFolders []*DiscoveryFolder

type DiscoveryFolderRepository interface {
    ResourceRepository

    Get(id string) (*DiscoveryFolder, error)
    GetAll(options ...QueryOptions) (DiscoveryFolders, error)
    Put(*DiscoveryFolder) error
    Delete(id string) error
    Exists(id string) (bool, error)

    CountAll(options ...QueryOptions) (int64, error)

    UpdateParent(id string, parentId *string) error

    GetAllByParent(options ...QueryOptions) (DiscoveryFolders, error)
}

// For combined lists (folders + playlists)
type DiscoveryAndFolderCommon struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"ownerId"`
	OwnerName string    `json:"ownerName"`
	UpdatedAt time.Time `json:"updatedAt"`
	Public    bool      `json:"public"`
	Type      string    `json:"type"` // "playlist" | "folder"
}
type DiscoveryAndFolderCommons []*DiscoveryAndFolderCommon
