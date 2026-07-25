package nativeapi

import "time"

// MPEG audio frame tables, indexed by the bitrate/sample-rate fields of a frame
// header. Only Layer III entries are listed since that is all we transcode to.
var (
	mp3BitratesV1L3 = [16]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
	mp3BitratesV2L3 = [16]int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0}
	mp3SampleRates  = [4][3]int{
		{11025, 12000, 8000},  // MPEG 2.5
		{0, 0, 0},             // reserved
		{22050, 24000, 16000}, // MPEG 2
		{44100, 48000, 32000}, // MPEG 1
	}
)

// mp3FramePacer measures how much audio a byte stream actually represents by
// walking its MPEG frame headers.
//
// This exists because pacing cannot assume the nominal bitrate: Navidrome
// transcodes with `ffmpeg -b:a 128k`, which is ABR, and real output averages
// noticeably below target (~114kbps on this library). Pacing off the nominal
// 128kbps shipped audio ~12% faster than real time, so a listener drifted
// further ahead of the device the longer they listened — a song change was
// heard long after it happened.
type mp3FramePacer struct {
	partial []byte
	audio   time.Duration
	// bytes/audio seen so far, used to estimate any trailing partial frame.
	bytesParsed int64
}

// Consume feeds newly produced bytes and returns the total audio duration the
// stream has carried so far. Unparseable data is skipped rather than fatal.
func (p *mp3FramePacer) Consume(data []byte) time.Duration {
	buf := data
	if len(p.partial) > 0 {
		buf = append(p.partial, data...)
		p.partial = nil
	}

	i := 0
	for i+4 <= len(buf) {
		frameLen, frameDur, ok := mp3ParseFrameHeader(buf[i:])
		if !ok {
			i++ // not a frame boundary; resync
			continue
		}
		if i+frameLen > len(buf) {
			break // frame continues into the next chunk
		}
		p.audio += frameDur
		p.bytesParsed += int64(frameLen)
		i += frameLen
	}

	// Carry the tail over. Cap it so a stream of non-MP3 bytes cannot grow this
	// buffer without bound.
	if rest := len(buf) - i; rest > 0 {
		if rest > 8192 {
			i = len(buf) - 8192
		}
		p.partial = append([]byte(nil), buf[i:]...)
	}

	return p.audio
}

// mp3ParseFrameHeader decodes a frame header, returning the frame length in
// bytes and the audio duration it represents.
func mp3ParseFrameHeader(b []byte) (frameLen int, dur time.Duration, ok bool) {
	if len(b) < 4 || b[0] != 0xFF || b[1]&0xE0 != 0xE0 {
		return 0, 0, false
	}

	versionID := (b[1] >> 3) & 0x03 // 0=MPEG2.5 1=reserved 2=MPEG2 3=MPEG1
	layer := (b[1] >> 1) & 0x03     // 1 = Layer III
	if versionID == 1 || layer != 1 {
		return 0, 0, false
	}

	bitrateIdx := (b[2] >> 4) & 0x0F
	sampleIdx := (b[2] >> 2) & 0x03
	padding := int((b[2] >> 1) & 0x01)
	if bitrateIdx == 0 || bitrateIdx == 15 || sampleIdx == 3 {
		return 0, 0, false
	}

	sampleRate := mp3SampleRates[versionID][sampleIdx]
	if sampleRate == 0 {
		return 0, 0, false
	}

	isV1 := versionID == 3
	var bitrate, samplesPerFrame, coef int
	if isV1 {
		bitrate = mp3BitratesV1L3[bitrateIdx]
		samplesPerFrame = 1152
		coef = 144
	} else {
		bitrate = mp3BitratesV2L3[bitrateIdx]
		samplesPerFrame = 576
		coef = 72
	}
	if bitrate == 0 {
		return 0, 0, false
	}

	frameLen = coef*bitrate*1000/sampleRate + padding
	if frameLen < 4 {
		return 0, 0, false
	}

	dur = time.Duration(samplesPerFrame) * time.Second / time.Duration(sampleRate)
	return frameLen, dur, true
}
