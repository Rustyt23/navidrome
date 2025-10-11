package persistence

import (
	"context"
	"os"
	"path/filepath"

	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PlaylistTrackRepository", func() {
	var (
		playlistRepo model.PlaylistRepository
		ctx          context.Context
	)

	BeforeEach(func() {
		ctx = log.NewContext(context.TODO())
		ctx = request.WithUser(ctx, model.User{ID: "userid", UserName: "userid", IsAdmin: true})
		playlistRepo = NewPlaylistRepository(ctx, GetDBXBuilder())
	})

	Describe("duplicatesOnly filter", func() {
		var playlist model.Playlist

		BeforeEach(func() {
			playlist = model.Playlist{Name: "Duplicates Filter", OwnerID: "userid", OwnerName: "userid"}
			playlist.AddMediaFilesByID([]string{"1001", "1001", "1003", "1003", "1003"})
			Expect(playlistRepo.Put(&playlist)).To(Succeed())
		})

		AfterEach(func() {
			if playlist.ID != "" {
				Expect(playlistRepo.Delete(playlist.ID)).To(Succeed())
			}
		})

		It("returns only duplicate playlist tracks", func() {
			repo := playlistRepo.Tracks(playlist.ID, true)

			result, err := repo.ReadAll(rest.QueryOptions{
				Filters: map[string]interface{}{"duplicatesOnly": true},
			})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())

			Expect(tracks).To(HaveLen(3))
			Expect(tracks[0].ID).To(Equal("2"))
			Expect(tracks[1].ID).To(Equal("4"))
			Expect(tracks[2].ID).To(Equal("5"))
		})

		It("includes only duplicate missing playlist entries when filtering duplicates", func() {
			playlist.Sync = true
			playlist.Path = filepath.Join(GinkgoT().TempDir(), "duplicates_missing.m3u")
			Expect(os.WriteFile(playlist.Path, []byte("ghost-track.mp3\n"+
				"ghost-track.mp3\n"+
				"phantom.mp3\n"), 0o600)).To(Succeed())
			Expect(playlistRepo.Put(&playlist)).To(Succeed())

			repo := playlistRepo.Tracks(playlist.ID, true)

			result, err := repo.ReadAll(rest.QueryOptions{
				Filters: map[string]interface{}{"duplicatesOnly": true},
			})
			Expect(err).ToNot(HaveOccurred())

			tracks, ok := result.(model.PlaylistTracks)
			Expect(ok).To(BeTrue())

			Expect(tracks).To(HaveLen(4))
			Expect(tracks[0].ID).To(Equal("2"))
			Expect(tracks[1].ID).To(Equal("4"))
			Expect(tracks[2].ID).To(Equal("5"))
			Expect(tracks[3].Missing).To(BeTrue())
			Expect(tracks[3].Path).To(Equal("ghost-track.mp3"))
			for _, track := range tracks {
				Expect(track.Path).ToNot(Equal("phantom.mp3"))
			}
		})
	})
})
