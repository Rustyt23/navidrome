package model

import "time"

type DiscoveryFolder struct {
	ID        string    `structs:"id" json:"id"`
	Name      string    `structs:"name" json:"name"`
	ParentID  *string   `structs:"parent_id" json:"parentId,omitempty"`
	Public    bool      `structs:"public" json:"public"`
	OwnerID   string    `structs:"owner_id" json:"ownerId"`
	OwnerName string    `structs:"-" json:"ownerName"`
	CreatedAt time.Time `structs:"created_at" json:"createdAt"`
	UpdatedAt time.Time `structs:"updated_at" json:"updatedAt"`

	Type string `structs:"-" json:"type,omitempty"`

	Children    []*DiscoveryFolder `structs:"-" json:"children,omitempty"`
	Discoveries []*Discovery       `structs:"-" json:"discoveries,omitempty"`
}

type DiscoveryFolders []*DiscoveryFolder
