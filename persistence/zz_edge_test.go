package persistence

import (
	"context"

	"github.com/navidrome/navidrome/db"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("snapshot edge cases", func() {
	It("shows what happens with an empty audit table", func() {
		ctx := log.NewContext(context.TODO())
		ctx = request.WithUser(ctx, model.User{ID: "userid"})
		repo := NewLoudnessAuditRepository(ctx, GetDBXBuilder(), db.Db())
		dir := GinkgoT().TempDir()

		_, _ = repo.Clear()
		snap, err := repo.Snapshot(dir, 3)
		GinkgoWriter.Printf("empty snapshot: err=%v snap=%+v\n", err, snap)

		listed, _ := repo.Snapshots(dir)
		GinkgoWriter.Printf("listed after empty snapshot: %d\n", len(listed))

		// Now simulate: 3 empty snapshots would evict real ones at keep=3.
		for range 3 {
			_, _ = repo.Snapshot(dir, 3)
		}
		listed, _ = repo.Snapshots(dir)
		GinkgoWriter.Printf("after 4 empty snapshots with keep=3: %d files\n", len(listed))
	})
})
