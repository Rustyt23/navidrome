package persistence

import (
	"context"
	"time"

	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// What normalizing a song has to do to its row, beyond the audit record.
var _ = Describe("media_file row after normalization", func() {
	var repo model.MediaFileRepository

	// Its own song, not one of the shared fixtures. These specs mutate the row
	// they work on, the suite shares a database, and Ginkgo randomises order -
	// so borrowing a fixture other specs assert on makes them fail whenever the
	// dice land badly. That is exactly what happened first time round.
	const songID = "lufsrow-1"

	BeforeEach(func() {
		ctx := log.NewContext(context.TODO())
		ctx = request.WithUser(ctx, model.User{ID: "userid", IsAdmin: true})
		repo = NewMediaFileRepository(ctx, GetDBXBuilder())

		song := model.MediaFile{
			ID: songID, LibraryID: 1, Title: "Row Test", Path: "/music/rowtest.mp3",
			LibraryPath: "/music", BitRate: 128, SampleRate: 44100,
			Channels: 2, Duration: 100, Size: 1000,
		}
		Expect(repo.Put(&song)).To(Succeed())
	})

	AfterEach(func() {
		_, err := GetDBXBuilder().NewQuery("delete from media_file where id = 'lufsrow-1'").Execute()
		Expect(err).ToNot(HaveOccurred())
	})

	f := func(v float64) *float64 { return &v }

	Describe("ClearReplayGain", func() {
		It("drops the correction a player would otherwise apply again", func() {
			// A song carrying "play me 3.2 dB quieter". Once the audio has been
			// normalized that instruction is wrong, and it is sent to every
			// client and to the built-in web player.
			mf, err := repo.Get(songID)
			Expect(err).ToNot(HaveOccurred())
			mf.RGTrackGain = f(-3.2)
			mf.RGTrackPeak = f(0.98)
			mf.RGAlbumGain = f(-3.0)
			mf.RGAlbumPeak = f(0.99)
			Expect(repo.Put(mf)).To(Succeed())

			before, err := repo.Get(songID)
			Expect(err).ToNot(HaveOccurred())
			Expect(before.RGTrackGain).ToNot(BeNil(), "fixture did not store a gain, so this proves nothing")

			Expect(repo.ClearReplayGain(songID)).To(Succeed())

			after, err := repo.Get(songID)
			Expect(err).ToNot(HaveOccurred())
			Expect(after.RGTrackGain).To(BeNil())
			Expect(after.RGTrackPeak).To(BeNil())
			Expect(after.RGAlbumGain).To(BeNil())
			Expect(after.RGAlbumPeak).To(BeNil())
		})

		It("is harmless on an empty id", func() {
			Expect(repo.ClearReplayGain("")).To(Succeed())
		})
	})

	// The scanner decides a file needs re-reading with
	// info.ModTime().After(dbTrack.UpdatedAt). Both callers run just after the
	// file on disk was replaced, so moving updated_at forward here made every
	// normalized song look newer than its own file and skipped it in every
	// incremental scan from then on.
	Describe("UpdateLoudnessTags", func() {
		It("does not move the scanner's timestamp forward", func() {
			mf, err := repo.Get(songID)
			Expect(err).ToNot(HaveOccurred())
			was := mf.UpdatedAt

			time.Sleep(10 * time.Millisecond)
			Expect(repo.UpdateLoudnessTags(songID, -12.6)).To(Succeed())

			after, err := repo.Get(songID)
			Expect(err).ToNot(HaveOccurred())
			Expect(after.UpdatedAt).To(BeTemporally("==", was),
				"updated_at moved, so the scanner will never re-read this file")
		})

		It("still records the measured loudness", func() {
			Expect(repo.UpdateLoudnessTags(songID, -12.6)).To(Succeed())
			after, err := repo.Get(songID)
			Expect(err).ToNot(HaveOccurred())
			Expect(after.Tags[model.TagName("loudnorm_final_lufs")]).To(ContainElement("-12.60"))
		})
	})

	Describe("UpdateAudioProperties", func() {
		It("brings the row back in step with the rewritten file", func() {
			Expect(repo.UpdateAudioProperties(songID, model.AudioFileProperties{
				BitRate: 320, SampleRate: 44100, BitDepth: 16,
				Channels: 2, Duration: 193.5, Size: 7654321,
			})).To(Succeed())

			after, err := repo.Get(songID)
			Expect(err).ToNot(HaveOccurred())
			Expect(after.BitRate).To(Equal(320))
			Expect(after.Size).To(Equal(int64(7654321)))
			Expect(after.Duration).To(BeNumerically("~", 193.5, 0.01))
		})
	})
})
