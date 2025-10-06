package persistence

import (
	"context"

	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PlaylistFolderRepository", func() {
	var repo model.PlaylistFolderRepository

	BeforeEach(func() {
		ctx := log.NewContext(context.TODO())
		ctx = request.WithUser(ctx, adminUser)
		repo = NewPlaylistFolderRepository(ctx, GetDBXBuilder())
	})

	It("sets the type field when reading folders", func() {
		folder := model.PlaylistFolder{
			Name:    "Folder With Type",
			OwnerID: adminUser.ID,
		}

		Expect(repo.Put(&folder)).To(Succeed())

		By("ensuring Get returns the type")
		fetched, err := repo.Get(folder.ID)
		Expect(err).ToNot(HaveOccurred())
		Expect(fetched.Type).To(Equal("folder"))

		By("ensuring GetAllByParent returns the type")
		results, err := repo.GetAllByParent()
		Expect(err).ToNot(HaveOccurred())

		found := false
		for _, f := range results {
			if f.ID == folder.ID {
				Expect(f.Type).To(Equal("folder"))
				found = true
				break
			}
		}
		Expect(found).To(BeTrue())

		Expect(repo.Delete(folder.ID)).To(Succeed())
	})
})
