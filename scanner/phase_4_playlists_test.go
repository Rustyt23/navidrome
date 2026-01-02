package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/conf/configtest"
	"github.com/navidrome/navidrome/core"
	"github.com/navidrome/navidrome/core/artwork"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/tests"
	"github.com/navidrome/navidrome/utils/slice"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

var _ = Describe("phasePlaylists", func() {
	var (
		phase      *phasePlaylists
		ctx        context.Context
		state      *scanState
		folderRepo *mockFolderRepository
		ds         *tests.MockDataStore
		pls        *mockPlaylists
		cw         artwork.CacheWarmer
	)

	BeforeEach(func() {
		DeferCleanup(configtest.SetupConfig())
		conf.Server.AutoImportPlaylists = true
		ctx = context.Background()
		ctx = request.WithUser(ctx, model.User{ID: "123", IsAdmin: true})
		folderRepo = &mockFolderRepository{}
		ds = &tests.MockDataStore{
			MockedFolder:         folderRepo,
			MockedPlaylist:       &playlistRepoMock{},
			MockedPlaylistFolder: &playlistFolderRepoMock{},
		}
		pls = &mockPlaylists{}
		cw = artwork.NoopCacheWarmer()
		state = &scanState{}
		phase = createPhasePlaylists(ctx, state, ds, pls, cw)
	})

	Describe("description", func() {
		It("returns the correct description", func() {
			Expect(phase.description()).To(Equal("Import/update playlists"))
		})
	})

	Describe("producer", func() {
		It("produces folders with playlists", func() {
			folderRepo.SetData(map[*model.Folder]error{
				{Path: "/path/to/folder1"}: nil,
				{Path: "/path/to/folder2"}: nil,
			})

			var produced []*model.Folder
			err := phase.produce(func(folder *model.Folder) {
				produced = append(produced, folder)
			})

			sort.Slice(produced, func(i, j int) bool {
				return produced[i].Path < produced[j].Path
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(produced).To(HaveLen(2))
			Expect(produced[0].Path).To(Equal("/path/to/folder1"))
			Expect(produced[1].Path).To(Equal("/path/to/folder2"))
		})

		It("returns an error if there is an error loading folders", func() {
			folderRepo.SetData(map[*model.Folder]error{
				nil: errors.New("error loading folders"),
			})

			called := false
			err := phase.produce(func(folder *model.Folder) { called = true })

			Expect(err).To(HaveOccurred())
			Expect(called).To(BeFalse())
			Expect(err).To(MatchError(ContainSubstring("error loading folders")))
		})

		It("includes missing playlist folders when playlists path is set", func() {
			root := GinkgoT().TempDir()
			conf.Server.PlaylistsPath = root
			repo := ds.MockedPlaylist.(*playlistRepoMock)
			repo.playlists = map[string]model.Playlist{
				filepath.Join(root, "rock", "favorites.m3u"): {
					ID:   "1",
					Path: filepath.Join(root, "rock", "favorites.m3u"),
					Sync: true,
				},
			}

			var produced []*model.Folder
			err := phase.produce(func(folder *model.Folder) {
				produced = append(produced, folder)
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(produced).ToNot(BeEmpty())
			paths := slice.Map(produced, func(f *model.Folder) string { return f.AbsolutePath() })
			Expect(paths).To(ContainElement(filepath.Join(root, "rock")))
		})
	})

	Describe("processPlaylistsInFolder", func() {
		It("imports new or updated playlists", func() {
			libPath := GinkgoT().TempDir()
			folder := &model.Folder{LibraryPath: libPath, Path: "path/to", Name: "folder"}
			_ = os.MkdirAll(folder.AbsolutePath(), 0755)

			file1 := filepath.Join(folder.AbsolutePath(), "playlist1.m3u")
			file2 := filepath.Join(folder.AbsolutePath(), "playlist2.m3u")
			_ = os.WriteFile(file1, []byte{}, 0600)
			_ = os.WriteFile(file2, []byte{}, 0600)

			repo := ds.MockedPlaylist.(*playlistRepoMock)
			repo.playlists = map[string]model.Playlist{
				filepath.Clean(file1): {
					ID:        "1",
					Path:      file1,
					Name:      "playlist1",
					Sync:      true,
					UpdatedAt: time.Now().Add(-time.Hour),
				},
			}
			// Ensure file1 mod time is newer than stored UpdatedAt
			future := time.Now()
			Expect(os.Chtimes(file1, future, future)).To(Succeed())

			pls.On("ImportFile", mock.Anything, folder, "playlist1.m3u").
				Return(&model.Playlist{}, nil)
			pls.On("ImportFile", mock.Anything, folder, "playlist2.m3u").
				Return(&model.Playlist{}, nil)

			_, err := phase.processPlaylistsInFolder(folder)
			Expect(err).ToNot(HaveOccurred())
			Expect(pls.Calls).To(HaveLen(2))
			Expect(pls.Calls[0].Arguments[2]).To(Equal("playlist1.m3u"))
			Expect(pls.Calls[1].Arguments[2]).To(Equal("playlist2.m3u"))
			Expect(phase.refreshed.Load()).To(Equal(uint32(2)))
		})

		It("skips unchanged playlists", func() {
			libPath := GinkgoT().TempDir()
			folder := &model.Folder{LibraryPath: libPath, Path: "path/to", Name: "folder"}
			_ = os.MkdirAll(folder.AbsolutePath(), 0755)

			file1 := filepath.Join(folder.AbsolutePath(), "playlist1.m3u")
			_ = os.WriteFile(file1, []byte{}, 0600)
			info, err := os.Stat(file1)
			Expect(err).ToNot(HaveOccurred())

			repo := ds.MockedPlaylist.(*playlistRepoMock)
			repo.playlists = map[string]model.Playlist{
				filepath.Clean(file1): {
					ID:        "1",
					Path:      file1,
					Name:      "playlist1",
					Sync:      true,
					UpdatedAt: info.ModTime(),
				},
			}

			_, err = phase.processPlaylistsInFolder(folder)
			Expect(err).ToNot(HaveOccurred())
			Expect(pls.Calls).To(BeEmpty())
			Expect(repo.deleted).To(BeEmpty())
		})

		It("removes playlists missing from the folder", func() {
			libPath := GinkgoT().TempDir()
			folder := &model.Folder{LibraryPath: libPath, Path: "path/to", Name: "folder"}
			_ = os.MkdirAll(folder.AbsolutePath(), 0755)

			repo := ds.MockedPlaylist.(*playlistRepoMock)
			repo.playlists = map[string]model.Playlist{
				filepath.Clean(filepath.Join(folder.AbsolutePath(), "playlist1.m3u")): {
					ID:        "1",
					Path:      filepath.Join(folder.AbsolutePath(), "playlist1.m3u"),
					Name:      "playlist1",
					Sync:      true,
					UpdatedAt: time.Now(),
				},
			}

			_, err := phase.processPlaylistsInFolder(folder)
			Expect(err).ToNot(HaveOccurred())
			Expect(repo.deleted).To(ContainElement("1"))
			Expect(phase.scanState.changesDetected.Load()).To(BeTrue())
		})

		It("reports an error if there is an error reading files", func() {
			progress := make(chan *ProgressInfo)
			state.progress = progress
			folder := &model.Folder{Path: "/invalid/path"}
			go func() {
				_, err := phase.processPlaylistsInFolder(folder)
				// I/O errors are ignored
				Expect(err).ToNot(HaveOccurred())
			}()

			// But are reported
			info := &ProgressInfo{}
			Eventually(progress).Should(Receive(&info))
			Expect(info.Warning).To(ContainSubstring("no such file or directory"))
		})

		It("removes playlists when the playlist folder is missing", func() {
			root := GinkgoT().TempDir()
			conf.Server.PlaylistsPath = root
			missingDir := filepath.Join(root, "missing")
			folder := &model.Folder{LibraryPath: root, Path: "", Name: "missing"}

			repo := ds.MockedPlaylist.(*playlistRepoMock)
			repo.playlists = map[string]model.Playlist{
				filepath.Join(missingDir, "playlist1.m3u"): {
					ID:        "1",
					Path:      filepath.Join(missingDir, "playlist1.m3u"),
					Name:      "playlist1",
					Sync:      true,
					UpdatedAt: time.Now(),
				},
			}

			_, err := phase.processPlaylistsInFolder(folder)
			Expect(err).ToNot(HaveOccurred())
			Expect(repo.deleted).To(ContainElement("1"))
			Expect(phase.scanState.changesDetected.Load()).To(BeTrue())
		})
	})
})

type mockPlaylists struct {
	mock.Mock
	core.Playlists
}

func (p *mockPlaylists) ImportFile(ctx context.Context, folder *model.Folder, filename string) (*model.Playlist, error) {
	args := p.Called(ctx, folder, filename)
	return args.Get(0).(*model.Playlist), args.Error(1)
}

func (p *mockPlaylists) Publish(ctx context.Context, playlistID string) error {
	args := p.Called(ctx, playlistID)
	return args.Error(0)
}

type mockFolderRepository struct {
	model.FolderRepository
	data map[*model.Folder]error
}

func (f *mockFolderRepository) GetTouchedWithPlaylists() (model.FolderCursor, error) {
	return func(yield func(model.Folder, error) bool) {
		for folder, err := range f.data {
			if err != nil {
				if !yield(model.Folder{}, err) {
					return
				}
				continue
			}
			if !yield(*folder, err) {
				return
			}
		}
	}, nil
}

func (f *mockFolderRepository) SetData(m map[*model.Folder]error) {
	f.data = m
}

type playlistFolderRepoMock struct {
	model.PlaylistFolderRepository
}

func (p *playlistFolderRepoMock) GetAll(...model.QueryOptions) (model.PlaylistFolders, error) {
	return model.PlaylistFolders{}, nil
}

func (p *playlistFolderRepoMock) GetAllByParent(...model.QueryOptions) (model.PlaylistFolders, error) {
	return model.PlaylistFolders{}, nil
}

func (p *playlistFolderRepoMock) Delete(string) error {
	return nil
}

type playlistRepoMock struct {
	model.PlaylistRepository
	playlists map[string]model.Playlist
	deleted   []string
}

func (r *playlistRepoMock) GetSyncedByDirectory(dir string) (model.Playlists, error) {
	cleaned := filepath.Clean(dir)
	var res model.Playlists
	for _, pls := range r.playlists {
		if !pls.Sync || pls.Path == "" {
			continue
		}
		if filepath.Clean(filepath.Dir(pls.Path)) == cleaned {
			res = append(res, pls)
		}
	}
	return res, nil
}

func (r *playlistRepoMock) Delete(id string) error {
	r.deleted = append(r.deleted, id)
	for path, pls := range r.playlists {
		if pls.ID == id {
			delete(r.playlists, path)
		}
	}
	return nil
}

func (r *playlistRepoMock) GetAll(options ...model.QueryOptions) (model.Playlists, error) {
	out := make(model.Playlists, 0, len(r.playlists))
	for _, pls := range r.playlists {
		out = append(out, pls)
	}
	return out, nil
}

func (r *playlistRepoMock) GetAllByPlaylistFolder(options ...model.QueryOptions) (model.Playlists, error) {
	if len(options) == 0 || options[0].Filters == nil {
		return model.Playlists{}, nil
	}
	var folderID string
	switch filters := options[0].Filters.(type) {
	case sq.Eq:
		if value, ok := filters["folder_id"]; ok {
			folderID, _ = value.(string)
		}
	}
	if folderID == "" {
		return model.Playlists{}, nil
	}
	var res model.Playlists
	for _, pls := range r.playlists {
		if pls.FolderID != nil && *pls.FolderID == folderID {
			res = append(res, pls)
		}
	}
	return res, nil
}
