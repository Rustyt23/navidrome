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

// The "before" measurement is the only record of what a song was, and once the
// backup is deleted it cannot be taken again. A failed re-analysis must not be
// able to erase it - one decode timeout or a drive unmounted for a moment used
// to be enough.
var _ = Describe("LoudnessAudit Put preserves the before snapshot", func() {
	var repo model.LoudnessAuditRepository

	BeforeEach(func() {
		ctx := log.NewContext(context.TODO())
		ctx = request.WithUser(ctx, model.User{ID: "userid"})
		repo = NewLoudnessAuditRepository(ctx, GetDBXBuilder(), db.Db())
	})

	AfterEach(func() {
		_, err := GetDBXBuilder().NewQuery("delete from media_file_loudness").Execute()
		Expect(err).ToNot(HaveOccurred())
	})

	f := func(v float64) *float64 { return &v }

	good := func() *model.LoudnessAudit {
		return &model.LoudnessAudit{
			MediaFileID:   "1004",
			Status:        model.LoudnessStatusProcessed,
			Action:        model.LoudnessActionGain,
			LufsBefore:    f(-18.25),
			LufsAfter:     f(-12.60),
			TpBefore:      f(-1.33),
			CodecBefore:   "mp3",
			BitrateBefore: 320,
			SizeBefore:    7781377,
			AnalyzedAt:    time.Now(),
		}
	}

	It("keeps the stored measurement when a re-analysis fails", func() {
		Expect(repo.Put(good())).To(Succeed())

		// Exactly what every failure path builds: no measurement at all.
		Expect(repo.Put(&model.LoudnessAudit{
			MediaFileID: "1004",
			Status:      model.LoudnessStatusFailed,
			Verdict:     model.LoudnessVerdictFailed,
			Error:       "measuring loudness: context deadline exceeded",
			AnalyzedAt:  time.Now(),
		})).To(Succeed())

		after, err := repo.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(after.LufsBefore).ToNot(BeNil(), "the before measurement was destroyed")
		Expect(*after.LufsBefore).To(BeNumerically("~", -18.25, 0.001))
		Expect(after.CodecBefore).To(Equal("mp3"))
		Expect(after.BitrateBefore).To(Equal(320))
		Expect(after.SizeBefore).To(Equal(int64(7781377)))
		// The failure itself still has to be recorded, or it is invisible.
		Expect(after.Status).To(Equal(model.LoudnessStatusFailed))
		Expect(after.Error).To(ContainSubstring("context deadline exceeded"))
	})

	It("still stores the before columns on a first, successful analysis", func() {
		// The guard must not stop a genuine measurement from being written.
		Expect(repo.Put(good())).To(Succeed())
		after, err := repo.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(after.LufsBefore).ToNot(BeNil())
		Expect(*after.LufsBefore).To(BeNumerically("~", -18.25, 0.001))
	})

	It("lets a later successful measurement replace the stored one", func() {
		Expect(repo.Put(good())).To(Succeed())
		updated := good()
		updated.LufsBefore = f(-14.10)
		updated.BitrateBefore = 128
		Expect(repo.Put(updated)).To(Succeed())

		after, err := repo.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(*after.LufsBefore).To(BeNumerically("~", -14.10, 0.001))
		Expect(after.BitrateBefore).To(Equal(128))
	})

	It("records a failure for a track that was never measured", func() {
		// Nothing to protect here, so the row must still be created.
		Expect(repo.Put(&model.LoudnessAudit{
			MediaFileID: "1003",
			Status:      model.LoudnessStatusFailed,
			Verdict:     model.LoudnessVerdictFailed,
			Error:       "no audio stream found",
			AnalyzedAt:  time.Now(),
		})).To(Succeed())

		after, err := repo.Get("1003")
		Expect(err).ToNot(HaveOccurred())
		Expect(after.Status).To(Equal(model.LoudnessStatusFailed))
		Expect(after.LufsBefore).To(BeNil())
	})
})

// A failure must not erase the after-snapshot or the phase either. The first
// version of this guard listed the columns to skip and missed both, which is
// why the rule is now "what may a failure write" rather than "what must it not".
var _ = Describe("LoudnessAudit Put on a failed re-analysis", func() {
	var repo model.LoudnessAuditRepository

	BeforeEach(func() {
		ctx := log.NewContext(context.TODO())
		ctx = request.WithUser(ctx, model.User{ID: "userid"})
		repo = NewLoudnessAuditRepository(ctx, GetDBXBuilder(), db.Db())
	})

	AfterEach(func() {
		_, err := GetDBXBuilder().NewQuery("delete from media_file_loudness").Execute()
		Expect(err).ToNot(HaveOccurred())
	})

	f := func(v float64) *float64 { return &v }

	It("keeps everything that describes the song", func() {
		Expect(repo.Put(&model.LoudnessAudit{
			MediaFileID:  "1004",
			Status:       model.LoudnessStatusProcessed,
			Verdict:      model.LoudnessVerdictSafe,
			Action:       model.LoudnessActionGain,
			Phase:        1,
			LufsBefore:   f(-18.25),
			LufsAfter:    f(-12.60),
			GainApplied:  f(5.65),
			NullResidual: f(-52.3),
			CodecAfter:   "mp3",
			SizeAfter:    7781000,
		})).To(Succeed())

		Expect(repo.Put(&model.LoudnessAudit{
			MediaFileID: "1004",
			Status:      model.LoudnessStatusFailed,
			Verdict:     model.LoudnessVerdictFailed,
			Phase:       -1,
			Error:       "measuring loudness: signal: killed",
		})).To(Succeed())

		after, err := repo.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(after.LufsBefore).ToNot(BeNil())
		Expect(after.LufsAfter).ToNot(BeNil(), "the after snapshot was erased")
		Expect(after.GainApplied).ToNot(BeNil(), "the applied gain was erased")
		Expect(after.NullResidual).ToNot(BeNil(), "the null test result was erased")
		Expect(after.SizeAfter).To(Equal(int64(7781000)))
		Expect(after.Phase).To(Equal(1), "the phase was reset, so the song reads as never planned")
		// The failure itself still has to land, or it is invisible.
		Expect(after.Status).To(Equal(model.LoudnessStatusFailed))
		Expect(after.Error).To(ContainSubstring("signal: killed"))
	})

	It("does not freeze a track that is being re-analysed successfully", func() {
		// A record with no before-measurement is not automatically a failure -
		// a re-analysis that found nothing wrong looks like this, and blocking
		// it would freeze the phase and action of every track it touched.
		Expect(repo.Put(&model.LoudnessAudit{
			MediaFileID: "1004", Phase: 2, Action: model.LoudnessActionRefused,
		})).To(Succeed())
		Expect(repo.Put(&model.LoudnessAudit{
			MediaFileID: "1004", Phase: 0,
			Action: model.LoudnessActionGain, Verdict: model.LoudnessVerdictSafe,
		})).To(Succeed())

		after, err := repo.Get("1004")
		Expect(err).ToNot(HaveOccurred())
		Expect(after.Phase).To(Equal(0))
		Expect(after.Action).To(Equal(model.LoudnessActionGain))
	})
})
