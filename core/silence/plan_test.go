package silence

import (
	"testing"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSilence(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Silence Suite")
}

func mp3Probe(duration float64) *ffmpeg.FileProbe {
	return &ffmpeg.FileProbe{Codec: "mp3", Duration: duration, SampleRate: 44100, Channels: 2}
}

var _ = Describe("BuildPlan", func() {
	Describe("the margin", func() {
		// The measured fixture: 3.199s of silence at the head, 5.698s at the
		// tail, both with a sharp onset, on a 28.9s file.
		It("keeps the margin rather than cutting to the music", func() {
			report := &ffmpeg.SilenceReport{
				LeadSilence: 3.199, TrailSilence: 5.698,
				LeadOnsetGap: 0.001, TrailOnsetGap: 0.0017,
				Duration: 28.9,
			}
			plan := BuildPlan(report, mp3Probe(28.9), false, Options{})

			Expect(plan.Verdict).To(Equal(model.SilenceVerdictTrimmable))
			Expect(plan.LeadTrim).To(BeNumerically("~", 2.699, 0.001))
			Expect(plan.TrailTrim).To(BeNumerically("~", 5.198, 0.001))
			// Half a second of quiet survives at each end - the whole point.
			Expect(plan.StartSeconds).To(BeNumerically("~", report.LeadSilence-0.5, 0.001))
			Expect(28.9 - plan.EndSeconds).To(BeNumerically("~", report.TrailSilence-0.5, 0.001))
		})

		It("leaves silence shorter than the margin alone", func() {
			report := &ffmpeg.SilenceReport{LeadSilence: 0.4, TrailSilence: 0.3, Duration: 200}
			plan := BuildPlan(report, mp3Probe(200), false, Options{})
			Expect(plan.Verdict).To(Equal(model.SilenceVerdictClean))
			Expect(plan.TotalTrim()).To(BeZero())
		})

		It("reports a clean track with no silence at all", func() {
			plan := BuildPlan(&ffmpeg.SilenceReport{Duration: 200}, mp3Probe(200), false, Options{})
			Expect(plan.Verdict).To(Equal(model.SilenceVerdictClean))
			Expect(plan.SkipReason).To(BeEmpty())
		})
	})

	Describe("gradual onsets", func() {
		// These used to be refused. They are not any more, and the reason is
		// worth stating: the cut is anchored to where the audio falls below the
		// detector's threshold and then pulled back by the margin, so a gradual
		// arrival does not put the cut any closer to the music - it only means
		// the quiet part lasts longer. On a real library the old 10ms rule
		// refused all 23 tracks that had removable silence at the end, because
		// real endings decay rather than stopping dead.
		It("trims a track whose ending decays", func() {
			report := &ffmpeg.SilenceReport{
				TrailSilence: 2.718, TrailOnsetGap: 0.698, Duration: 166,
			}
			plan := BuildPlan(report, mp3Probe(166), false, Options{})
			Expect(plan.Verdict).To(Equal(model.SilenceVerdictTrimmable))
			Expect(plan.TrailTrim).To(BeNumerically("~", 2.218, 0.001))
		})

		It("trims a track that fades in", func() {
			report := &ffmpeg.SilenceReport{
				LeadSilence: 6.283, LeadOnsetGap: 0.756, Duration: 23,
			}
			plan := BuildPlan(report, mp3Probe(23), false, Options{})
			Expect(plan.Verdict).To(Equal(model.SilenceVerdictTrimmable))
			Expect(plan.LeadTrim).To(BeNumerically("~", 5.783, 0.001))
		})

		It("still allows a sharp onset", func() {
			report := &ffmpeg.SilenceReport{LeadSilence: 3.199, LeadOnsetGap: 0.001, Duration: 23}
			plan := BuildPlan(report, mp3Probe(23), false, Options{})
			Expect(plan.Verdict).To(Equal(model.SilenceVerdictTrimmable))
		})

		// The backstop still exists for measurements that make no sense.
		It("refuses an absurdly wide gap, where the measurement is suspect", func() {
			report := &ffmpeg.SilenceReport{
				LeadSilence: 20, LeadOnsetGap: 12, Duration: 200,
			}
			plan := BuildPlan(report, mp3Probe(200), false, Options{})
			Expect(plan.Verdict).To(Equal(model.SilenceVerdictSkipped))
			Expect(plan.SkipReason).To(Equal(model.SilenceSkipFade))
		})
	})

	Describe("the safety caps", func() {
		It("refuses more silence than any real lead-in", func() {
			report := &ffmpeg.SilenceReport{LeadSilence: 45, LeadOnsetGap: 0.001, Duration: 300}
			plan := BuildPlan(report, mp3Probe(300), false, Options{})
			Expect(plan.Verdict).To(Equal(model.SilenceVerdictSkipped))
			Expect(plan.SkipReason).To(Equal(model.SilenceSkipTooLong))
		})

		It("refuses a cut that would leave almost nothing", func() {
			report := &ffmpeg.SilenceReport{
				LeadSilence: 4, TrailSilence: 4.5,
				LeadOnsetGap: 0.001, TrailOnsetGap: 0.001,
				Duration: 8,
			}
			plan := BuildPlan(report, mp3Probe(8), false, Options{})
			Expect(plan.Verdict).To(Equal(model.SilenceVerdictSkipped))
			Expect(plan.SkipReason).To(Equal(model.SilenceSkipTooLong))
		})

		// A file that is silent end to end has both detectors reporting the
		// whole thing, so the two ends overlap and the arithmetic runs past
		// itself. Nothing should be cut: there is no music to keep.
		It("refuses a file that is silent throughout", func() {
			report := &ffmpeg.SilenceReport{
				LeadSilence: 30, TrailSilence: 30,
				LeadOnsetGap: 0.001, TrailOnsetGap: 0.001,
				Duration: 30,
			}
			plan := BuildPlan(report, mp3Probe(30), false, Options{})
			Expect(plan.Verdict).To(Equal(model.SilenceVerdictSkipped))
			Expect(plan.TotalTrim()).To(BeZero())
		})
	})

	Describe("gapless albums", func() {
		It("refuses a track on a continuous album", func() {
			report := &ffmpeg.SilenceReport{LeadSilence: 3.0, LeadOnsetGap: 0.001, Duration: 200}
			plan := BuildPlan(report, mp3Probe(200), true, Options{})
			Expect(plan.Verdict).To(Equal(model.SilenceVerdictSkipped))
			Expect(plan.SkipReason).To(Equal(model.SilenceSkipGapless))
		})

		It("trims it when the client overrides", func() {
			report := &ffmpeg.SilenceReport{LeadSilence: 3.0, LeadOnsetGap: 0.001, Duration: 200}
			plan := BuildPlan(report, mp3Probe(200), true, Options{AllowGapless: true})
			Expect(plan.Verdict).To(Equal(model.SilenceVerdictTrimmable))
		})
	})

	Describe("the method", func() {
		It("copies mp3 and re-encodes flac", func() {
			report := &ffmpeg.SilenceReport{LeadSilence: 3.0, LeadOnsetGap: 0.001, Duration: 200}
			mp3 := BuildPlan(report, mp3Probe(200), false, Options{})
			Expect(mp3.Method).To(Equal(model.SilenceMethodCopy))

			flac := BuildPlan(report, &ffmpeg.FileProbe{Codec: "flac", Duration: 200}, false, Options{})
			Expect(flac.Method).To(Equal(model.SilenceMethodEncode))
		})
	})
})

var _ = Describe("PlanFromAudit", func() {
	It("reproduces the cut points the analysis recorded", func() {
		audit := &model.SilenceAudit{
			Verdict: model.SilenceVerdictTrimmable, DurationBefore: 28.9,
			LeadTrim: 2.699, TrailTrim: 5.198,
		}
		plan := PlanFromAudit(audit)
		Expect(plan.ShouldTrim()).To(BeTrue())
		Expect(plan.StartSeconds).To(BeNumerically("~", 2.699, 0.001))
		Expect(plan.EndSeconds).To(BeNumerically("~", 23.702, 0.001))
	})

	It("does not trim a skipped track", func() {
		audit := &model.SilenceAudit{Verdict: model.SilenceVerdictSkipped, LeadTrim: 0}
		Expect(PlanFromAudit(audit).ShouldTrim()).To(BeFalse())
	})
})

var _ = Describe("DetectGaplessAlbum", func() {
	It("spots a continuous album", func() {
		tracks := []TrackSeam{
			{TrackNumber: 1, LeadSilence: 0, TailSilence: 0.1},
			{TrackNumber: 2, LeadSilence: 0.1, TailSilence: 0.2},
			{TrackNumber: 3, LeadSilence: 0.1, TailSilence: 0.1},
			{TrackNumber: 4, LeadSilence: 0.2, TailSilence: 3.0},
		}
		Expect(DetectGaplessAlbum(tracks)).To(BeTrue())
	})

	It("leaves an ordinary album alone", func() {
		tracks := []TrackSeam{
			{TrackNumber: 1, LeadSilence: 2.0, TailSilence: 3.0},
			{TrackNumber: 2, LeadSilence: 1.8, TailSilence: 2.5},
			{TrackNumber: 3, LeadSilence: 2.2, TailSilence: 4.0},
			{TrackNumber: 4, LeadSilence: 1.5, TailSilence: 3.0},
		}
		Expect(DetectGaplessAlbum(tracks)).To(BeFalse())
	})

	It("is not fooled by one segued pair on a normal album", func() {
		tracks := []TrackSeam{
			{TrackNumber: 1, LeadSilence: 2.0, TailSilence: 0.1},
			{TrackNumber: 2, LeadSilence: 0.1, TailSilence: 3.0},
			{TrackNumber: 3, LeadSilence: 2.2, TailSilence: 4.0},
			{TrackNumber: 4, LeadSilence: 1.5, TailSilence: 3.0},
		}
		Expect(DetectGaplessAlbum(tracks)).To(BeFalse())
	})

	It("refuses to judge an album too short to have evidence", func() {
		tracks := []TrackSeam{
			{TrackNumber: 1, LeadSilence: 0, TailSilence: 0},
			{TrackNumber: 2, LeadSilence: 0, TailSilence: 0},
		}
		Expect(DetectGaplessAlbum(tracks)).To(BeFalse())
	})

	It("does not count the gap between two discs as a seam", func() {
		tracks := []TrackSeam{
			{DiscNumber: 1, TrackNumber: 1, LeadSilence: 2.0, TailSilence: 3.0},
			{DiscNumber: 1, TrackNumber: 2, LeadSilence: 2.0, TailSilence: 0.1},
			{DiscNumber: 2, TrackNumber: 1, LeadSilence: 0.1, TailSilence: 3.0},
			{DiscNumber: 2, TrackNumber: 2, LeadSilence: 2.0, TailSilence: 3.0},
		}
		Expect(DetectGaplessAlbum(tracks)).To(BeFalse())
	})
})

var _ = Describe("ApplyGaplessVerdict", func() {
	It("withdraws planned trims and says why", func() {
		audits := []*model.SilenceAudit{
			{Verdict: model.SilenceVerdictTrimmable, LeadTrim: 2.5, TrailTrim: 1.0},
		}
		ApplyGaplessVerdict(audits, true)
		Expect(audits[0].Verdict).To(Equal(model.SilenceVerdictSkipped))
		Expect(audits[0].SkipReason).To(Equal(model.SilenceSkipGapless))
		Expect(audits[0].LeadTrim).To(BeZero())
		Expect(audits[0].Gapless).To(BeTrue())
	})

	It("leaves a normal album's plans intact", func() {
		audits := []*model.SilenceAudit{
			{Verdict: model.SilenceVerdictTrimmable, LeadTrim: 2.5},
		}
		ApplyGaplessVerdict(audits, false)
		Expect(audits[0].Verdict).To(Equal(model.SilenceVerdictTrimmable))
		Expect(audits[0].LeadTrim).To(BeNumerically("~", 2.5, 0.001))
	})
})
