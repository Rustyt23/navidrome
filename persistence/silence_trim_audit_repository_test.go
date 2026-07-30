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

var _ = Describe("SilenceTrimAuditRepository", func() {
	var repo model.SilenceTrimAuditRepository
	var mediaFiles model.MediaFileRepository

	BeforeEach(func() {
		ctx := request.WithUser(
			log.NewContext(context.TODO()),
			model.User{ID: "userid"},
		)
		repo = NewSilenceTrimAuditRepository(ctx, GetDBXBuilder())
		mediaFiles = NewMediaFileRepository(ctx, GetDBXBuilder())
	})

	AfterEach(func() {
		_, err := GetDBXBuilder().
			NewQuery("delete from media_file_silence_trim").
			Execute()
		Expect(err).ToNot(HaveOccurred())
	})

	It("round-trips the proposal and proof through the song join", func() {
		modified := time.Now().Add(-time.Minute).UTC().Truncate(time.Second)
		applied := time.Now().UTC().Truncate(time.Second)
		audit := &model.SilenceTrimAudit{
			MediaFileID:            "1004",
			Status:                 model.SilenceTrimStatusProcessed,
			Classification:         model.SilenceTrimClassSafe,
			Decision:               model.SilenceTrimDecisionApprove,
			Reason:                 "verified",
			LeadingKind:            model.SilenceTrimEdgeExact,
			TrailingKind:           model.SilenceTrimEdgeExact,
			LeadingSilence:         1.2,
			TrailingSilence:        0.8,
			LeadingSamples:         52920,
			TrailingSamples:        35280,
			ProposedStartTrim:      0.95,
			ProposedEndTrim:        0.55,
			ProposedStartSamples:   41895,
			ProposedEndSamples:     24255,
			AppliedStartTrim:       0.95,
			AppliedEndTrim:         0.55,
			AppliedStartSamples:    41895,
			AppliedEndSamples:      24255,
			RetainedPadding:        0.25,
			RetainedPaddingSamples: 11025,
			Method:                 model.SilenceTrimMethodLossless,
			Integrity:              model.SilenceTrimIntegrityVerified,
			CodecBefore:            "flac",
			CodecAfter:             "flac",
			SampleRateBefore:       44100,
			SampleRateAfter:        44100,
			BitDepthBefore:         24,
			BitDepthAfter:          24,
			ChannelsBefore:         2,
			ChannelsAfter:          2,
			DurationBefore:         180,
			DurationAfter:          178.5,
			SizeBefore:             10_000,
			SizeAfter:              9_000,
			ArtBefore:              true,
			ArtAfter:               true,
			HasBackup:              true,
			BackupSHA256:           "abc123",
			SourceSHA256:           "source123",
			ResultSHA256:           "result123",
			SourceModifiedAt:       &modified,
			ResultModifiedAt:       &applied,
			AppliedAt:              &applied,
		}
		Expect(repo.Put(audit)).To(Succeed())

		saved, err := repo.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(saved.ProposedStartSamples).To(Equal(int64(41895)))
		Expect(saved.Integrity).To(Equal(model.SilenceTrimIntegrityVerified))
		Expect(saved.AnalyzedAt).ToNot(BeZero())

		mf, err := mediaFiles.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(mf.SilenceTrimAudit).ToNot(BeNil())
		Expect(mf.SilenceTrimAudit.Method).To(Equal(model.SilenceTrimMethodLossless))
		Expect(mf.SilenceTrimAudit.DurationAfter).To(BeNumerically("~", 178.5, 0.001))
		Expect(mf.SilenceTrimAudit.SourceModifiedAt).ToNot(BeNil())
		Expect(mf.SilenceTrimAudit.ResultModifiedAt).ToNot(BeNil())
		Expect(mf.SilenceTrimAudit.AppliedAt).ToNot(BeNil())
		Expect(mf.SilenceTrimAudit.ResultSHA256).To(Equal("result123"))

		By("atomically storing the decision associated with the proposal")
		audit.Decision = model.SilenceTrimDecisionSkip
		audit.Reason = "fresh measurement"
		Expect(repo.Put(audit)).To(Succeed())
		saved, err = repo.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(saved.Decision).To(Equal(model.SilenceTrimDecisionSkip))
		Expect(saved.Reason).To(Equal("fresh measurement"))

		By("clearing only disposable dry-run rows")
		Expect(repo.Put(&model.SilenceTrimAudit{
			MediaFileID:    "1002",
			Status:         model.SilenceTrimStatusAnalyzed,
			Classification: model.SilenceTrimClassNone,
		})).To(Succeed())
		cleared, err := repo.Clear()
		Expect(err).ToNot(HaveOccurred())
		Expect(cleared).To(Equal(int64(1)))
		_, err = repo.Get("1002")
		Expect(err).To(MatchError(model.ErrNotFound))
		saved, err = repo.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(saved.HasBackup).To(BeTrue())
		Expect(saved.ResultSHA256).To(Equal("result123"))
	})
})
