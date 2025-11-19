package tests

import (
	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/model"
)

type MockPlaylistRepo struct {
	model.PlaylistRepository

	Entity *model.Playlist
	Error  error

	LastUpdatedCommentIDs []string
	LastComment           string
	UpdateCommentError    error
}

func (m *MockPlaylistRepo) Get(_ string) (*model.Playlist, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	if m.Entity == nil {
		return nil, model.ErrNotFound
	}
	return m.Entity, nil
}

func (m *MockPlaylistRepo) Count(_ ...rest.QueryOptions) (int64, error) {
	if m.Error != nil {
		return 0, m.Error
	}
	if m.Entity == nil {
		return 0, nil
	}
	return 1, nil
}

func (m *MockPlaylistRepo) GetSyncedByDirectory(string) (model.Playlists, error) {
	return nil, nil
}

func (m *MockPlaylistRepo) UpdateComment(ids []string, comment string) error {
	if m.Error != nil {
		return m.Error
	}
	if m.UpdateCommentError != nil {
		return m.UpdateCommentError
	}
	m.LastUpdatedCommentIDs = append([]string{}, ids...)
	m.LastComment = comment
	return nil
}

func CreateMockPlaylistRepo() *MockPlaylistRepo {
	return &MockPlaylistRepo{}
}
