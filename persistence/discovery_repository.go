package persistence

import (
	"context"

	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

type discoveryPlaylistRepository struct {
	ctx context.Context
	db  dbx.Builder
}

func NewDiscoveryPlaylistRepository(ctx context.Context, db dbx.Builder) model.DiscoveryPlaylistRepository {
	return &discoveryPlaylistRepository{ctx: ctx, db: db}
}

func (r *discoveryPlaylistRepository) GetAll(_ ...model.QueryOptions) (model.DiscoveryPlaylists, error) {
	var entries model.DiscoveryPlaylists
	err := r.db.Select("*").From("discovery_playlists").OrderBy("name asc").All(&entries)
	return entries, err
}

func (r *discoveryPlaylistRepository) Get(id string) (*model.DiscoveryPlaylist, error) {
	var entry model.DiscoveryPlaylist
	err := r.db.Select("*").From("discovery_playlists").Where(dbx.HashExp{"id": id}).One(&entry)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *discoveryPlaylistRepository) ReplaceAll(playlists model.DiscoveryPlaylists) error {
	if _, err := r.db.NewQuery("delete from discovery_playlists").Execute(); err != nil {
		return err
	}
	for _, pls := range playlists {
		params := dbx.Params{
			"id":          pls.ID,
			"name":        pls.Name,
			"folder_path": pls.FolderPath,
			"song_count":  pls.SongCount,
			"updated_at":  pls.UpdatedAt,
		}
		if _, err := r.db.Insert("discovery_playlists", params).Execute(); err != nil {
			log.Error(r.ctx, "Error persisting discovery playlist", "name", pls.Name, err)
			return err
		}
	}
	return nil
}
