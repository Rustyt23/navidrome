package persistence

import (
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Discovery repositories", func() {
	It("do not modify playlist folders", func() {
		ctx := request.WithUser(log.NewContext(GinkgoT().Context()), adminUser)
		playlistRepo := NewPlaylistFolderRepository(ctx, GetDBXBuilder())
		discoveryRepo := NewDiscoveryFolderRepository(ctx, GetDBXBuilder())

		before, err := playlistRepo.CountAll()
		Expect(err).ToNot(HaveOccurred())

		folder := &model.DiscoveryFolder{Name: "Isolated", OwnerID: adminUser.ID, Public: true}
		Expect(discoveryRepo.Put(folder)).To(Succeed())

		after, err := playlistRepo.CountAll()
		Expect(err).ToNot(HaveOccurred())
		Expect(after).To(Equal(before))
	})
})
