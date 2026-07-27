package persistence

import (
	"context"

	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LoudnessAuditRepository", func() {
	var repo model.LoudnessAuditRepository
	var mr model.MediaFileRepository

	BeforeEach(func() {
		ctx := log.NewContext(context.TODO())
		ctx = request.WithUser(ctx, model.User{ID: "userid"})
		repo = NewLoudnessAuditRepository(ctx, GetDBXBuilder())
		mr = NewMediaFileRepository(ctx, GetDBXBuilder())
	})

	// The suite shares one DB across specs, so leaving audit rows behind would
	// change what every other media file assertion sees.
	AfterEach(func() {
		_, err := GetDBXBuilder().NewQuery("delete from media_file_loudness").Execute()
		Expect(err).ToNot(HaveOccurred())
	})

	f := func(v float64) *float64 { return &v }

	It("returns no audit for a track that was never analyzed", func() {
		mf, err := mr.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(mf.LoudnessAudit).To(BeNil())
	})

	It("round-trips an audit record through the media file join", func() {
		audit := &model.LoudnessAudit{
			MediaFileID:      "1004",
			Status:           model.LoudnessStatusProcessed,
			Verdict:          model.LoudnessVerdictReencoded,
			Action:           model.LoudnessActionLimited,
			LufsBefore:       f(-18.25),
			LufsAfter:        f(-12.55),
			GainApplied:      f(5.7),
			TpBefore:         f(-1.33),
			TpAfter:          f(-0.75),
			LraBefore:        f(6.2),
			LraAfter:         f(5.6),
			NullResidual:     f(-11.5),
			CodecBefore:      "mp3",
			BitrateBefore:    320,
			SampleRateBefore: 44100,
			BitDepthBefore:   16,
			ChannelsBefore:   2,
			DurationBefore:   193.345,
			SizeBefore:       7781377,
			ArtBefore:        true,
			ArtAfter:         false,
		}
		Expect(repo.Put(audit)).To(Succeed())

		By("reading it back directly")
		saved, err := repo.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(saved.Verdict).To(Equal(model.LoudnessVerdictReencoded))
		Expect(*saved.LufsBefore).To(BeNumerically("~", -18.25, 0.001))
		Expect(saved.BitrateBefore).To(Equal(320))
		Expect(saved.ArtBefore).To(BeTrue())
		Expect(saved.ArtAfter).To(BeFalse())
		Expect(saved.AnalyzedAt).ToNot(BeZero())

		By("exposing it on the joined media file")
		mf, err := mr.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(mf.LoudnessAudit).ToNot(BeNil())
		Expect(mf.LoudnessAudit.Status).To(Equal(model.LoudnessStatusProcessed))
		Expect(mf.LoudnessAudit.Action).To(Equal(model.LoudnessActionLimited))
		Expect(*mf.LoudnessAudit.NullResidual).To(BeNumerically("~", -11.5, 0.001))
		Expect(mf.LoudnessAudit.SampleRateBefore).To(Equal(44100))

		By("replacing the record on re-analysis")
		audit.Verdict = model.LoudnessVerdictSafe
		audit.NullResidual = f(-68.2)
		Expect(repo.Put(audit)).To(Succeed())
		mf, err = mr.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(mf.LoudnessAudit.Verdict).To(Equal(model.LoudnessVerdictSafe))
		Expect(*mf.LoudnessAudit.NullResidual).To(BeNumerically("~", -68.2, 0.001))

		By("leaving other tracks unaffected")
		other, err := mr.Get("1003")
		Expect(err).ToNot(HaveOccurred())
		Expect(other.LoudnessAudit).To(BeNil())
	})

	It("clears every derived audit record without deleting media files", func() {
		Expect(repo.Put(&model.LoudnessAudit{MediaFileID: "1003"})).To(Succeed())
		Expect(repo.Put(&model.LoudnessAudit{MediaFileID: "1004"})).To(Succeed())

		count, err := repo.Clear()
		Expect(err).ToNot(HaveOccurred())
		Expect(count).To(Equal(int64(2)))

		_, err = repo.Get("1003")
		Expect(err).To(MatchError(model.ErrNotFound))
		_, err = repo.Get("1004")
		Expect(err).To(MatchError(model.ErrNotFound))

		mf, err := mr.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(mf).ToNot(BeNil())
		Expect(mf.LoudnessAudit).To(BeNil())
	})
})
