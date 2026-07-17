package scanner

import (
	"path/filepath"

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

	Describe("effectiveLoudnessParallelism", func() {
		It("uses the configured worker count", func() {
			Expect(effectiveLoudnessParallelism(8, 100)).To(Equal(8))
		})

		It("does not start more workers than files", func() {
			Expect(effectiveLoudnessParallelism(8, 3)).To(Equal(3))
		})

		It("falls back to serial processing when not set", func() {
			Expect(effectiveLoudnessParallelism(0, 100)).To(Equal(1))
		})
	})

	Describe("configuredLoudnessParallelism", func() {
		It("keeps positive configured values", func() {
			Expect(configuredLoudnessParallelism(8)).To(Equal(8))
		})

		It("falls back to one worker when disabled by config", func() {
			Expect(configuredLoudnessParallelism(0)).To(Equal(1))
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

	Describe("loudnessSyncPath", func() {
		BeforeEach(func() {
			conf.Server.SyncFolder = filepath.Join(string(filepath.Separator), "sync")
		})

		It("preserves a relative media path inside the sync folder", func() {
			path := loudnessSyncPath("/music", "artist/album/song.mp3", "/music/artist/album/song.mp3")

			Expect(path).To(Equal(filepath.Join(string(filepath.Separator), "sync", "artist", "album", "song.mp3")))
		})

		It("uses the library-relative path for absolute media paths", func() {
			path := loudnessSyncPath("/music", "/music/artist/album/song.mp3", "/music/artist/album/song.mp3")

			Expect(path).To(Equal(filepath.Join(string(filepath.Separator), "sync", "artist", "album", "song.mp3")))
		})

		It("falls back to the track filename for paths outside the library", func() {
			path := loudnessSyncPath("/music", "../../song.mp3", "/other/song.mp3")

			Expect(path).To(Equal(filepath.Join(string(filepath.Separator), "sync", "song.mp3")))
		})
	})
})
