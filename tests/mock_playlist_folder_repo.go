package tests

import (
	"github.com/navidrome/navidrome/model"
)

type MockPlaylistFolderRepo struct {
	model.PlaylistFolderRepository

	Entity *model.PlaylistFolder
	Error  error
}
