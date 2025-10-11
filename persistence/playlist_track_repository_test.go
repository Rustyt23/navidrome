package persistence

import (
	"context"

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
	})
})
