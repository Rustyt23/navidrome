package persistence

import (
	"context"
	"time"

	"github.com/navidrome/navidrome/db"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Keeping the newest N copies is a reasonable rule right up to the moment
// somebody clears the analysis. After that every new copy is nearly empty, and
// a handful of small runs pushes the one holding the real library out of the
// window purely for being older - so the single file worth having is the first
// deleted.
var _ = Describe("LoudnessAudit snapshot pruning", func() {
	var repo model.LoudnessAuditRepository
	var mr model.MediaFileRepository
	var dir string

	BeforeEach(func() {
		ctx := log.NewContext(context.TODO())
		ctx = request.WithUser(ctx, model.User{ID: "userid"})
		repo = NewLoudnessAuditRepository(ctx, GetDBXBuilder(), db.Db())
		mr = NewMediaFileRepository(ctx, GetDBXBuilder())
		dir = GinkgoT().TempDir()

		for _, id := range []string{"prune-1", "prune-2", "prune-3"} {
			song := model.MediaFile{ID: id, LibraryID: 1, Title: "Song " + id,
				Path: "/music/" + id + ".mp3", LibraryPath: "/music"}
			Expect(mr.Put(&song)).To(Succeed())
		}
	})

	AfterEach(func() {
		_, err := GetDBXBuilder().NewQuery("delete from media_file_loudness").Execute()
		Expect(err).ToNot(HaveOccurred())
		_, err = GetDBXBuilder().NewQuery("delete from media_file where id like 'prune-%'").Execute()
		Expect(err).ToNot(HaveOccurred())
	})

	f := func(v float64) *float64 { return &v }
	measure := func(id string) {
		Expect(repo.Put(&model.LoudnessAudit{
			MediaFileID: id, Status: model.LoudnessStatusProcessed, LufsBefore: f(-18.5),
		})).To(Succeed())
	}

	// Snapshot filenames carry a one-second stamp, so copies have to be spaced
	// out or they overwrite each other and there is nothing to prune.
	It("never deletes the fullest copy to make room for emptier ones", func() {
		// The real library.
		measure("prune-1")
		measure("prune-2")
		measure("prune-3")
		big, err := repo.Snapshot(dir, 0)
		Expect(err).ToNot(HaveOccurred())
		Expect(big.Rows).To(BeNumerically("==", 3))

		// Someone clears the analysis, then small runs happen afterwards.
		for range 2 {
			time.Sleep(1100 * time.Millisecond)
			_, err := repo.Clear()
			Expect(err).ToNot(HaveOccurred())
			measure("prune-1")
			_, err = repo.Snapshot(dir, 1)
			Expect(err).ToNot(HaveOccurred())
		}

		stored, err := repo.Snapshots(dir)
		Expect(err).ToNot(HaveOccurred())

		var kept []string
		var fullest int64
		for _, s := range stored {
			kept = append(kept, s.File)
			if s.Rows > fullest {
				fullest = s.Rows
			}
		}
		Expect(kept).To(ContainElement(big.File),
			"the copy holding the whole library was aged out by emptier ones")
		Expect(fullest).To(BeNumerically("==", 3))
	})

	It("still honours the limit for copies of comparable size", func() {
		var files []string
		for range 3 {
			measure("prune-1")
			measure("prune-2")
			measure("prune-3")
			snap, err := repo.Snapshot(dir, 1)
			Expect(err).ToNot(HaveOccurred())
			files = append(files, snap.File)
			time.Sleep(1100 * time.Millisecond)
		}
		stored, err := repo.Snapshots(dir)
		Expect(err).ToNot(HaveOccurred())
		// One by the limit, plus at most the spared fullest one. Without the
		// limit working at all this would be three.
		Expect(len(stored)).To(BeNumerically("<=", 2))
	})

	It("keeps everything when no limit is set", func() {
		for range 2 {
			measure("prune-1")
			_, err := repo.Snapshot(dir, 0)
			Expect(err).ToNot(HaveOccurred())
			time.Sleep(1100 * time.Millisecond)
		}
		stored, err := repo.Snapshots(dir)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(stored)).To(Equal(2))
	})
})
