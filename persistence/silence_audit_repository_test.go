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

var _ = Describe("SilenceAuditRepository", func() {
	var repo model.SilenceAuditRepository
	var mr model.MediaFileRepository

	BeforeEach(func() {
		ctx := log.NewContext(context.TODO())
		ctx = request.WithUser(ctx, model.User{ID: "userid"})
		repo = NewSilenceAuditRepository(ctx, GetDBXBuilder())
		mr = NewMediaFileRepository(ctx, GetDBXBuilder())
	})

	// The suite shares one DB across specs, so leaving rows behind would change
	// what every other media file assertion sees.
	AfterEach(func() {
		_, err := GetDBXBuilder().NewQuery("delete from media_file_silence").Execute()
		Expect(err).ToNot(HaveOccurred())
	})

	It("returns no audit for a track that was never analysed", func() {
		mf, err := mr.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(mf.SilenceAudit).To(BeNil())
	})

	// The state a fresh install is in, and what the page hits on first load:
	// nothing analysed, so the table is empty. Both summary queries run against
	// it before anything else happens, and an error from either is what the
	// client sees as a failed page.
	Context("with nothing analysed yet", func() {
		It("counts verdicts without error", func() {
			counts, err := repo.CountByVerdict()
			Expect(err).ToNot(HaveOccurred())
			Expect(counts).To(BeEmpty())
		})

		It("totals the pending trim without error", func() {
			pending, err := repo.PendingTrimSeconds()
			Expect(err).ToNot(HaveOccurred())
			Expect(pending).To(BeZero())
		})
	})

	It("round-trips a record through the media file join", func() {
		Expect(repo.Put(&model.SilenceAudit{
			MediaFileID:    "1004",
			Status:         model.SilenceStatusAnalyzed,
			Verdict:        model.SilenceVerdictTrimmable,
			LeadSilence:    3.199,
			TrailSilence:   5.698,
			LeadTrim:       2.699,
			TrailTrim:      5.198,
			LeadOnsetGap:   0.001,
			Method:         model.SilenceMethodCopy,
			Codec:          "mp3",
			DurationBefore: 28.9,
		})).To(Succeed())

		mf, err := mr.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(mf.SilenceAudit).ToNot(BeNil())
		Expect(mf.SilenceAudit.Verdict).To(Equal(model.SilenceVerdictTrimmable))
		Expect(mf.SilenceAudit.LeadTrim).To(BeNumerically("~", 2.699, 0.0001))
		Expect(mf.SilenceAudit.TrailTrim).To(BeNumerically("~", 5.198, 0.0001))
		Expect(mf.SilenceAudit.TotalTrim()).To(BeNumerically("~", 7.897, 0.0001))
		Expect(mf.SilenceAudit.Method).To(Equal(model.SilenceMethodCopy))
		Expect(mf.SilenceAudit.IsTrimmed()).To(BeFalse())
	})

	// The safety-critical one. Re-analysing an already-trimmed song must not
	// erase the fact that it was trimmed: the trim run selects on exactly that
	// column, so losing it puts the song back in the queue and the next run
	// eats the half-second margin that was deliberately left behind.
	It("keeps trimmed_at when a later analysis does not carry one", func() {
		trimmedAt := time.Now().Add(-time.Hour).Truncate(time.Second)
		Expect(repo.Put(&model.SilenceAudit{
			MediaFileID: "1004",
			Status:      model.SilenceStatusTrimmed,
			Verdict:     model.SilenceVerdictTrimmable,
			LeadTrim:    2.5,
			TrimmedAt:   &trimmedAt,
		})).To(Succeed())

		// A fresh analysis of the same song: no TrimmedAt on the new record.
		Expect(repo.Put(&model.SilenceAudit{
			MediaFileID:    "1004",
			Status:         model.SilenceStatusAnalyzed,
			Verdict:        model.SilenceVerdictClean,
			LeadSilence:    0.4,
			DurationBefore: 26.4,
		})).To(Succeed())

		stored, err := repo.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(stored.IsTrimmed()).To(BeTrue(), "the trim record was erased by a re-analysis")
		Expect(stored.Verdict).To(Equal(model.SilenceVerdictClean))
		Expect(stored.LeadSilence).To(BeNumerically("~", 0.4, 0.0001))
	})

	It("counts verdicts and totals the pending trim", func() {
		Expect(repo.Put(&model.SilenceAudit{
			MediaFileID: "1004", Verdict: model.SilenceVerdictTrimmable,
			LeadTrim: 2.0, TrailTrim: 1.5,
		})).To(Succeed())
		Expect(repo.Put(&model.SilenceAudit{
			MediaFileID: "1003", Verdict: model.SilenceVerdictClean,
		})).To(Succeed())
		Expect(repo.Put(&model.SilenceAudit{
			MediaFileID: "1002", Verdict: model.SilenceVerdictSkipped,
			SkipReason: model.SilenceSkipFade,
		})).To(Succeed())

		counts, err := repo.CountByVerdict()
		Expect(err).ToNot(HaveOccurred())
		Expect(counts[model.SilenceVerdictTrimmable]).To(BeNumerically("==", 1))
		Expect(counts[model.SilenceVerdictClean]).To(BeNumerically("==", 1))
		Expect(counts[model.SilenceVerdictSkipped]).To(BeNumerically("==", 1))

		pending, err := repo.PendingTrimSeconds()
		Expect(err).ToNot(HaveOccurred())
		Expect(pending).To(BeNumerically("~", 3.5, 0.0001))
	})

	// A song already cut is no longer waiting, so it must drop out of the
	// headline figure - otherwise the total never falls as a run progresses.
	It("excludes already-trimmed songs from the pending total", func() {
		trimmedAt := time.Now()
		Expect(repo.Put(&model.SilenceAudit{
			MediaFileID: "1004", Verdict: model.SilenceVerdictTrimmable,
			LeadTrim: 2.0, TrailTrim: 1.5, TrimmedAt: &trimmedAt,
		})).To(Succeed())

		pending, err := repo.PendingTrimSeconds()
		Expect(err).ToNot(HaveOccurred())
		Expect(pending).To(BeZero())
	})

	It("clears every record and nothing else", func() {
		Expect(repo.Put(&model.SilenceAudit{
			MediaFileID: "1004", Verdict: model.SilenceVerdictTrimmable, LeadTrim: 1,
		})).To(Succeed())

		removed, err := repo.Clear()
		Expect(err).ToNot(HaveOccurred())
		Expect(removed).To(BeNumerically("==", 1))

		// The song itself is untouched.
		mf, err := mr.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(mf.SilenceAudit).To(BeNil())
		Expect(mf.Title).ToNot(BeEmpty())
	})
})
