package persistence

import (
	"context"
	"encoding/json"
	"time"

	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Silence backup repository", func() {
	const mediaFileID = "silence-backup-repository-test"
	var (
		ctx        context.Context
		mediaFiles model.MediaFileRepository
		backups    model.SilenceBackupRepository
	)

	BeforeEach(func() {
		ctx = request.WithUser(log.NewContext(context.Background()), model.User{ID: "adminid", IsAdmin: true})
		mediaFiles = NewMediaFileRepository(ctx, GetDBXBuilder())
		backups = NewSilenceBackupRepository(ctx, GetDBXBuilder())
		media := &model.MediaFile{
			ID: mediaFileID, LibraryID: 1, Path: "silence/backup-test.mp3",
			Title: "Silence backup test", Suffix: "mp3", Lyrics: "[]",
			Tags: model.Tags{}, Participants: model.Participants{},
		}
		Expect(mediaFiles.Put(media)).To(Succeed())
	})

	AfterEach(func() {
		_ = mediaFiles.Delete(mediaFileID)
	})

	It("persists the two-phase audit, joins only safe status JSON, and refreshes file properties", func() {
		originalTime := time.Unix(1_700_000_000, 0).UTC()
		preparedAt := originalTime.Add(time.Minute)
		backup := &model.SilenceBackup{
			MediaFileID: mediaFileID, BackupFile: "ab/abcdef.mp3",
			Status:         model.SilenceBackupStatusPrepared,
			OriginalSHA256: "original-sha", TrimmedSHA256: "trimmed-sha",
			OriginalSize: 1000, TrimmedSize: 800, OriginalMode: 0o640,
			TrimStart: 0.25, TrimEnd: 0.5, OriginalModTime: originalTime,
			CreatedAt: originalTime, PreparedAt: &preparedAt,
		}
		Expect(backups.Put(backup)).To(Succeed())

		stored, err := backups.Get(mediaFileID)
		Expect(err).NotTo(HaveOccurred())
		Expect(stored.Status).To(Equal(model.SilenceBackupStatusPrepared))
		Expect(stored.TrimmedSHA256).To(Equal("trimmed-sha"))
		Expect(stored.OriginalMode).To(Equal(uint32(0o640)))
		Expect(stored.OriginalModTime.Unix()).To(Equal(originalTime.Unix()))

		joined, err := mediaFiles.Get(mediaFileID)
		Expect(err).NotTo(HaveOccurred())
		Expect(joined.SilenceBackup).NotTo(BeNil())
		Expect(joined.SilenceBackup.Status).To(Equal(model.SilenceBackupStatusPrepared))
		encoded, err := json.Marshal(joined.SilenceBackup)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(encoded)).To(Equal(`{"status":"prepared"}`))

		updatedAt := originalTime.Add(2 * time.Minute)
		Expect(mediaFiles.UpdateSilenceMutationProperties(mediaFileID, 800, 2.25, updatedAt)).To(Succeed())
		updated, err := mediaFiles.Get(mediaFileID)
		Expect(err).NotTo(HaveOccurred())
		Expect(updated.Size).To(Equal(int64(800)))
		Expect(updated.Duration).To(BeNumerically("~", 2.25, 0.001))
		Expect(updated.UpdatedAt.Unix()).To(Equal(updatedAt.Unix()))
	})
})
