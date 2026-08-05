package silence

import (
	"context"
	"errors"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeMeasurer returns a fixed peak per region, and records what it was asked.
type fakeMeasurer struct {
	head, tail float64
	err        error
	calls      []ffmpeg.PeakRegion
}

func (f *fakeMeasurer) MeasurePeakDB(_ context.Context, _ string, region ffmpeg.PeakRegion) (float64, error) {
	f.calls = append(f.calls, region)
	if f.err != nil {
		return 0, f.err
	}
	if region.FromEnd {
		return f.tail, nil
	}
	return f.head, nil
}

var _ = Describe("VerifyInaudible", func() {
	trimPlan := func() Plan {
		return Plan{
			Verdict:   model.SilenceVerdictTrimmable,
			LeadTrim:  2.2,
			TrailTrim: 1.7,
		}
	}

	// The real measurements from the library: the audio actually removed peaked
	// around -69 dB. Well under the threshold, so the trim stands.
	It("allows a trim whose removed audio is inaudible", func() {
		m := &fakeMeasurer{head: -69.5, tail: -70.3}
		plan, peak, err := VerifyInaudible(context.Background(), m, "/song.mp3", trimPlan())
		Expect(err).ToNot(HaveOccurred())
		Expect(plan.Verdict).To(Equal(model.SilenceVerdictTrimmable))
		Expect(plan.TotalTrim()).To(BeNumerically("~", 3.9, 0.001))
		Expect(peak).To(BeNumerically("~", -69.5, 0.001))
	})

	// The case the old heuristic was trying to catch, caught properly: the
	// detector said silence, the level meter can still hear something.
	It("refuses when the removed audio is audible", func() {
		m := &fakeMeasurer{head: -80, tail: -12}
		plan, peak, err := VerifyInaudible(context.Background(), m, "/song.mp3", trimPlan())
		Expect(err).ToNot(HaveOccurred())
		Expect(plan.Verdict).To(Equal(model.SilenceVerdictSkipped))
		Expect(plan.SkipReason).To(Equal(model.SilenceSkipAudible))
		Expect(plan.TotalTrim()).To(BeZero())
		Expect(peak).To(BeNumerically("~", -12, 0.001))
	})

	It("reports the louder of the two ends", func() {
		m := &fakeMeasurer{head: -65, tail: -80}
		_, peak, err := VerifyInaudible(context.Background(), m, "/song.mp3", trimPlan())
		Expect(err).ToNot(HaveOccurred())
		Expect(peak).To(BeNumerically("~", -65, 0.001))
	})

	// Measuring the whole file instead of the removed stretch would find the
	// music and refuse everything, so which region is asked for matters.
	It("measures exactly the stretches that would be removed", func() {
		m := &fakeMeasurer{head: -70, tail: -70}
		_, _, err := VerifyInaudible(context.Background(), m, "/song.mp3", trimPlan())
		Expect(err).ToNot(HaveOccurred())
		Expect(m.calls).To(HaveLen(2))
		Expect(m.calls[0]).To(Equal(ffmpeg.PeakRegion{Seconds: 2.2}))
		Expect(m.calls[1]).To(Equal(ffmpeg.PeakRegion{Seconds: 1.7, FromEnd: true}))
	})

	It("does not measure an end that is not being cut", func() {
		m := &fakeMeasurer{head: -70, tail: -70}
		plan := trimPlan()
		plan.TrailTrim = 0
		_, _, err := VerifyInaudible(context.Background(), m, "/song.mp3", plan)
		Expect(err).ToNot(HaveOccurred())
		Expect(m.calls).To(HaveLen(1))
		Expect(m.calls[0].FromEnd).To(BeFalse())
	})

	It("skips the measurement entirely when nothing would be trimmed", func() {
		m := &fakeMeasurer{}
		plan := Plan{Verdict: model.SilenceVerdictClean}
		out, _, err := VerifyInaudible(context.Background(), m, "/song.mp3", plan)
		Expect(err).ToNot(HaveOccurred())
		Expect(m.calls).To(BeEmpty())
		Expect(out.Verdict).To(Equal(model.SilenceVerdictClean))
	})

	// A measurement that could not be taken is not permission to cut.
	It("propagates a measurement failure instead of trimming blind", func() {
		m := &fakeMeasurer{err: errors.New("ffmpeg exploded")}
		_, _, err := VerifyInaudible(context.Background(), m, "/song.mp3", trimPlan())
		Expect(err).To(HaveOccurred())
	})
})
