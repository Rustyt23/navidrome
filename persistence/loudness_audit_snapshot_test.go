package persistence

import (
	"context"
	"os"
	"path/filepath"

	"github.com/navidrome/navidrome/db"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LoudnessAudit snapshots", func() {
	var repo model.LoudnessAuditRepository
	var mr model.MediaFileRepository
	var dir string

	// Two real songs, so the foreign key has something to point at: a snapshot
	// row whose song is gone is deliberately not restorable.
	newSong := func(id string) model.MediaFile {
		return model.MediaFile{
			ID: id, LibraryID: 1, Title: "Song " + id, Path: "/music/" + id + ".mp3",
			LibraryPath: "/music",
		}
	}

	BeforeEach(func() {
		ctx := log.NewContext(context.TODO())
		ctx = request.WithUser(ctx, model.User{ID: "userid"})
		repo = NewLoudnessAuditRepository(ctx, GetDBXBuilder(), db.Db())
		mr = NewMediaFileRepository(ctx, GetDBXBuilder())
		dir = GinkgoT().TempDir()

		for _, id := range []string{"snap-1", "snap-2"} {
			song := newSong(id)
			Expect(mr.Put(&song)).To(Succeed())
		}
	})

	AfterEach(func() {
		_, err := GetDBXBuilder().NewQuery("delete from media_file_loudness").Execute()
		Expect(err).ToNot(HaveOccurred())
		_, err = GetDBXBuilder().NewQuery("delete from media_file where id like 'snap-%'").Execute()
		Expect(err).ToNot(HaveOccurred())
	})

	decision := func(id, d string) {
		Expect(repo.SetDecision(id, d)).To(Succeed())
	}

	It("writes a snapshot and reads it back", func() {
		lufs := -18.5
		Expect(repo.Put(&model.LoudnessAudit{
			MediaFileID: "snap-1", Status: model.LoudnessStatusProcessed, LufsBefore: &lufs,
		})).To(Succeed())

		snapshot, err := repo.Snapshot(dir, 0)
		Expect(err).ToNot(HaveOccurred())
		Expect(snapshot.Rows).To(Equal(int64(1)))
		Expect(snapshot.File).To(HavePrefix("lufs-audit-"))
		Expect(snapshot.Path).To(BeAnExistingFile())

		listed, err := repo.Snapshots(dir)
		Expect(err).ToNot(HaveOccurred())
		Expect(listed).To(HaveLen(1))
		Expect(listed[0].File).To(Equal(snapshot.File))
		Expect(listed[0].Rows).To(Equal(int64(1)))
	})

	// The whole point of the feature: a decision is human input and nothing can
	// derive it again.
	It("brings back a decision that was lost", func() {
		decision("snap-1", "gain_ceiling")
		snapshot, err := repo.Snapshot(dir, 0)
		Expect(err).ToNot(HaveOccurred())

		_, err = repo.Clear()
		Expect(err).ToNot(HaveOccurred())
		_, err = repo.Get("snap-1")
		Expect(err).To(MatchError(model.ErrNotFound))

		report, err := repo.RestoreSnapshot(dir, snapshot.File)
		Expect(err).ToNot(HaveOccurred())
		Expect(report.Restored).To(Equal(int64(1)))

		restored, err := repo.Get("snap-1")
		Expect(err).ToNot(HaveOccurred())
		Expect(restored.Decision).To(Equal("gain_ceiling"))
	})

	// "Exact replace": the table ends up matching the snapshot, so work done
	// after it was taken is deliberately dropped.
	It("removes rows the snapshot does not hold", func() {
		decision("snap-1", "limit")
		snapshot, err := repo.Snapshot(dir, 0)
		Expect(err).ToNot(HaveOccurred())

		decision("snap-2", "skip")
		Expect(repo.Get("snap-2")).ToNot(BeNil())

		report, err := repo.RestoreSnapshot(dir, snapshot.File)
		Expect(err).ToNot(HaveOccurred())
		Expect(report.Restored).To(Equal(int64(1)))
		Expect(report.Removed).To(Equal(int64(1)))

		_, err = repo.Get("snap-2")
		Expect(err).To(MatchError(model.ErrNotFound))
	})

	// A row whose song is not in this library has nothing to describe, and the
	// foreign key would refuse it. It is reported rather than silently dropped.
	It("skips rows whose song is no longer in the library", func() {
		decision("snap-1", "limit")
		decision("snap-2", "skip")
		snapshot, err := repo.Snapshot(dir, 0)
		Expect(err).ToNot(HaveOccurred())
		Expect(snapshot.Rows).To(Equal(int64(2)))

		_, err = repo.Clear()
		Expect(err).ToNot(HaveOccurred())
		_, err = GetDBXBuilder().NewQuery("delete from media_file where id = 'snap-2'").Execute()
		Expect(err).ToNot(HaveOccurred())

		report, err := repo.RestoreSnapshot(dir, snapshot.File)
		Expect(err).ToNot(HaveOccurred())
		Expect(report.Rows).To(Equal(int64(2)))
		Expect(report.Restored).To(Equal(int64(1)))
		Expect(report.Skipped).To(Equal(int64(1)))
	})

	// A copy of nothing still counts against however many are kept, so a few of
	// them - after a cleared table, or a run that measured nothing - would
	// quietly evict every copy that did hold something.
	It("refuses to copy an empty audit table", func() {
		_, err := repo.Clear()
		Expect(err).ToNot(HaveOccurred())

		_, err = repo.Snapshot(dir, 3)
		Expect(err).To(MatchError(model.ErrNoLoudnessAuditData))

		listed, err := repo.Snapshots(dir)
		Expect(err).ToNot(HaveOccurred())
		Expect(listed).To(BeEmpty())
	})

	It("does not let empty copies push out the ones that hold something", func() {
		decision("snap-1", "limit")
		real1, err := repo.Snapshot(dir, 2)
		Expect(err).ToNot(HaveOccurred())

		_, err = repo.Clear()
		Expect(err).ToNot(HaveOccurred())
		for range 5 {
			_, err = repo.Snapshot(dir, 2)
			Expect(err).To(MatchError(model.ErrNoLoudnessAuditData))
		}

		listed, err := repo.Snapshots(dir)
		Expect(err).ToNot(HaveOccurred())
		Expect(listed).To(HaveLen(1))
		Expect(listed[0].File).To(Equal(real1.File))
		Expect(listed[0].Rows).To(Equal(int64(1)))
	})

	It("keeps only the newest copies", func() {
		decision("snap-1", "limit")
		var files []string
		for range 4 {
			snapshot, err := repo.Snapshot(dir, 2)
			Expect(err).ToNot(HaveOccurred())
			files = append(files, snapshot.File)
		}

		listed, err := repo.Snapshots(dir)
		Expect(err).ToNot(HaveOccurred())
		Expect(len(listed)).To(BeNumerically("<=", 2))
	})

	// The copy is built under a temporary name and renamed at the end, so a
	// crash part way through cannot leave a file that looks usable. On success
	// the temporary name must be gone.
	It("leaves no working files behind", func() {
		decision("snap-1", "limit")
		_, err := repo.Snapshot(dir, 0)
		Expect(err).ToNot(HaveOccurred())

		entries, err := os.ReadDir(dir)
		Expect(err).ToNot(HaveOccurred())
		Expect(entries).To(HaveLen(1))
		Expect(entries[0].Name()).ToNot(HavePrefix("."))
	})

	// The name arrives over HTTP, so it must never be able to walk out of the
	// snapshot folder or point at an unrelated file.
	It("refuses a name that is not a snapshot in the folder", func() {
		for _, bad := range []string{
			"", "../../etc/passwd", "/etc/passwd", "notes.txt",
			filepath.Join("sub", "lufs-audit-20260101-000000.db"),
		} {
			_, err := repo.RestoreSnapshot(dir, bad)
			Expect(err).To(HaveOccurred(), "should have refused %q", bad)
		}
	})

	It("reports a missing snapshot rather than emptying the table", func() {
		decision("snap-1", "limit")

		_, err := repo.RestoreSnapshot(dir, "lufs-audit-20200101-000000.db")
		Expect(err).To(HaveOccurred())

		kept, err := repo.Get("snap-1")
		Expect(err).ToNot(HaveOccurred())
		Expect(kept.Decision).To(Equal("limit"))
	})
})
