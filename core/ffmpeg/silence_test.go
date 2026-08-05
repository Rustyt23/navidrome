package ffmpeg

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("parseSilenceIntervals", func() {
	It("separates the two detectors by instance index", func() {
		output := `
[Parsed_silencedetect_0 @ 0x14e004100] silence_start: 0
[Parsed_silencedetect_1 @ 0x14e004200] silence_start: 0
[Parsed_silencedetect_0 @ 0x14e004100] silence_end: 3.199048 | silence_duration: 3.199048
[Parsed_silencedetect_1 @ 0x14e004200] silence_end: 3.2 | silence_duration: 3.2
`
		primary, onset := parseSilenceIntervals(output)
		Expect(primary).To(HaveLen(1))
		Expect(onset).To(HaveLen(1))
		Expect(primary[0].start).To(BeNumerically("~", 0, 0.0001))
		Expect(primary[0].end).To(BeNumerically("~", 3.199048, 0.0001))
		Expect(onset[0].end).To(BeNumerically("~", 3.2, 0.0001))
	})

	It("records a stretch that never closes as silent to the end", func() {
		output := `[Parsed_silencedetect_0 @ 0x1] silence_start: 23.2`
		primary, _ := parseSilenceIntervals(output)
		Expect(primary).To(HaveLen(1))
		Expect(primary[0].end).To(BeNumerically("<", 0))
	})
})

var _ = Describe("edgeAt", func() {
	// The numbers are the measured ones: a sharp start puts the two thresholds
	// ~1ms apart, an exponential fade ~756ms.
	It("reports a sharp onset as a near-zero gap", func() {
		primary := []silenceInterval{{start: 0, end: 3.199048}}
		onset := []silenceInterval{{start: 0, end: 3.2}}
		edge := edgeAt(primary, onset, 0, 28.9, true)
		Expect(edge.silence).To(BeNumerically("~", 3.199048, 0.0001))
		Expect(edge.onsetGap).To(BeNumerically("<", 0.01))
	})

	It("reports a fade-in as a large gap", func() {
		primary := []silenceInterval{{start: 0, end: 6.282948}}
		onset := []silenceInterval{{start: 0, end: 7.03898}}
		edge := edgeAt(primary, onset, 0, 23, true)
		Expect(edge.onsetGap).To(BeNumerically("~", 0.756, 0.01))
	})

	It("ignores silence that does not touch the edge", func() {
		primary := []silenceInterval{{start: 8, end: 9}}
		Expect(edgeAt(primary, nil, 0, 28.9, true).silence).To(BeZero())
	})

	It("finds trailing silence that ffmpeg never closed", func() {
		primary := []silenceInterval{{start: 23.2, end: -1}}
		onset := []silenceInterval{{start: 23.19, end: -1}}
		edge := edgeAt(primary, onset, 0, 28.9, false)
		Expect(edge.silence).To(BeNumerically("~", 5.7, 0.01))
	})

	It("treats an unconfirmed edge as maximally gradual", func() {
		// The louder threshold saw no silence here at all, so nothing confirms
		// where the music starts. Refusing is the only safe reading.
		primary := []silenceInterval{{start: 0, end: 3.2}}
		edge := edgeAt(primary, nil, 0, 28.9, true)
		Expect(edge.onsetGap).To(Equal(edge.silence))
	})
})

var _ = Describe("canCopyCodec", func() {
	It("allows the codecs whose headers stay honest after a copy", func() {
		Expect(canCopyCodec("mp3")).To(BeTrue())
		Expect(canCopyCodec("MP3")).To(BeTrue())
		Expect(canCopyCodec("aac")).To(BeTrue())
	})

	It("refuses flac, whose STREAMINFO keeps the original sample count", func() {
		Expect(canCopyCodec("flac")).To(BeFalse())
	})

	It("refuses opus, whose page granularity misses the cut", func() {
		Expect(canCopyCodec("opus")).To(BeFalse())
		Expect(canCopyCodec("vorbis")).To(BeFalse())
	})
})
