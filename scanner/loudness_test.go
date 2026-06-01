package scanner

import (
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("loudness normalization", func() {
	Describe("effectiveLoudnessTolerance", func() {
		It("uses the configured 0.1 LUFS tolerance", func() {
			Expect(effectiveLoudnessTolerance(0.1)).To(Equal(0.1))
		})

		It("falls back to the configured default when tolerance is not set", func() {
			Expect(effectiveLoudnessTolerance(0)).To(Equal(conf.DefaultLoudnessNormalizationTolerance))
		})
	})

	Describe("shouldNormalizeLoudness", func() {
		It("treats 0.1 LUFS as the allowed target range", func() {
			const targetLUFS = -12.6
			const tolerance = 0.1

			Expect(shouldNormalizeLoudness(-12.5, targetLUFS, tolerance)).To(BeFalse())
			Expect(shouldNormalizeLoudness(-12.49, targetLUFS, tolerance)).To(BeTrue())
			Expect(shouldNormalizeLoudness(-12.7, targetLUFS, tolerance)).To(BeFalse())
			Expect(shouldNormalizeLoudness(-12.71, targetLUFS, tolerance)).To(BeTrue())
		})
	})

	Describe("adjustedLoudnessTarget", func() {
		It("clamps a louder track adjustment to the minimum allowed LUFS", func() {
			target := ffmpeg.LoudnessTarget{IntegratedLUFS: -12.6, TruePeak: -1.5, LRA: 11}

			adjusted := adjustedLoudnessTarget(target, -11.73, -12.7, -12.5)

			Expect(adjusted.IntegratedLUFS).To(BeNumerically("~", -12.7, 0.001))
			Expect(adjusted.TruePeak).To(Equal(target.TruePeak))
			Expect(adjusted.LRA).To(Equal(target.LRA))
		})

		It("clamps a quieter track adjustment to the maximum allowed LUFS", func() {
			target := ffmpeg.LoudnessTarget{IntegratedLUFS: -12.6, TruePeak: -1.5, LRA: 11}

			adjusted := adjustedLoudnessTarget(target, -13.12, -12.7, -12.5)

			Expect(adjusted.IntegratedLUFS).To(BeNumerically("~", -12.5, 0.001))
			Expect(adjusted.TruePeak).To(Equal(target.TruePeak))
			Expect(adjusted.LRA).To(Equal(target.LRA))
		})
	})

	Describe("isAdjustedLoudnessTarget", func() {
		It("reports whether the attempt target differs from the configured target", func() {
			target := ffmpeg.LoudnessTarget{IntegratedLUFS: -12.6, TruePeak: -1.5, LRA: 11}

			Expect(isAdjustedLoudnessTarget(target, target)).To(BeFalse())
			Expect(isAdjustedLoudnessTarget(target, adjustedLoudnessTarget(target, -12.86, -12.7, -12.5))).To(BeTrue())
		})
	})
})
