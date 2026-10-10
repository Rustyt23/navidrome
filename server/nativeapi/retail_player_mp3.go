package nativeapi

import (
	"bufio"
	"errors"
	"io"
	"time"
)

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

// retailPlayerMP3Reader strips tags and emits complete frames. Retaining only
// frame-aligned audio lets late subscribers start decoding without receiving a
// partial frame or a mid-stream ID3 tag from a new song.
type retailPlayerMP3Reader struct{ r *bufio.Reader }

func newRetailPlayerMP3Reader(r io.Reader) *retailPlayerMP3Reader {
	return &retailPlayerMP3Reader{r: bufio.NewReader(r)}
}

func (r *retailPlayerMP3Reader) Next() ([]byte, time.Duration, error) {
	for skipped := 0; skipped < 65536; skipped++ {
		header, err := r.r.Peek(4)
		if err != nil {
			if errors.Is(err, io.EOF) && len(header) != 0 {
				err = io.ErrUnexpectedEOF
			}
			return nil, 0, err
		}
		if string(header[:3]) == "ID3" {
			tag, err := r.r.Peek(10)
			if err != nil {
				return nil, 0, io.ErrUnexpectedEOF
			}
			size := 0
			for _, b := range tag[6:10] {
				if b&0x80 != 0 {
					return nil, 0, errors.New("invalid ID3 size")
				}
				size = size<<7 | int(b)
			}
			if tag[3] == 4 && tag[5]&0x10 != 0 {
				size += 10
			}
			if size > 1024*1024 {
				return nil, 0, errors.New("ID3 tag exceeds 1 MiB")
			}
			if _, err := io.CopyN(io.Discard, r.r, int64(size+10)); err != nil {
				return nil, 0, io.ErrUnexpectedEOF
			}
			continue
		}
		size, duration, ok := mp3ParseFrameHeader(header)
		if !ok {
			_, _ = r.r.Discard(1)
			continue
		}
		// A continuous broadcast must keep the same decoding profile at boundaries.
		if (header[1]>>3)&3 != 3 || (header[2]>>2)&3 != 0 || header[3]>>6 == 3 {
			return nil, 0, errors.New("broadcast requires 44100 Hz stereo MP3")
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(r.r, data); err != nil {
			return nil, 0, err
		}
		return data, duration, nil
	}
	return nil, 0, errors.New("unable to find an MP3 frame within 64 KiB")
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
