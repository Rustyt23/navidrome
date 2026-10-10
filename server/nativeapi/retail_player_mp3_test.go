package nativeapi

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type retailFragmentReader struct{ r io.Reader }

func (r retailFragmentReader) Read(p []byte) (int, error) {
	if len(p) > 3 {
		p = p[:3]
	}
	return r.r.Read(p)
}

func TestRetailMP3ReaderTagsFragmentationAndBoundaries(t *testing.T) {
	// A fake MPEG header inside an ID3 tag must never become audio.
	tag := append([]byte{'I', 'D', '3', 4, 0, 0, 0, 0, 3, 34}, bytes.Repeat([]byte{0xff, 0xfb, 0x90, 0}, 104)...)
	tag = append(tag, 0, 0) // 418 byte tag payload, synchsafe size 3*128+34
	input := bytes.Join([][]byte{tag, testRetailFrame(1).data, testRetailFrame(2).data, tag, testRetailFrame(3).data}, nil)
	r := newRetailPlayerMP3Reader(retailFragmentReader{bytes.NewReader(input)})
	for want := uint32(1); want <= 3; want++ {
		data, duration, err := r.Next()
		if err != nil {
			t.Fatal(err)
		}
		if got := testRetailSequence(retailPlayerAudioFrame{data: data}); got != want {
			t.Fatalf("frame %d != %d", got, want)
		}
		if duration != retailPlayerMP3FrameDuration {
			t.Fatalf("incorrect pacing duration: %v", duration)
		}
	}
	if _, _, err := r.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestRetailMP3ReaderRejectsInvalidOrTruncatedAudio(t *testing.T) {
	wrongRate := bytes.Clone(retailPlayerSilence)
	wrongRate[2] |= 4
	mono := bytes.Clone(retailPlayerSilence)
	mono[3] |= 192
	for name, data := range map[string][]byte{
		"truncated-frame":  retailPlayerSilence[:100],
		"truncated-header": retailPlayerSilence[:3],
		"truncated-tag":    []byte("ID3\x04\x00"),
		"invalid-tag-size": {'I', 'D', '3', 4, 0, 0, 128, 0, 0, 0},
		"oversized-tag":    {'I', 'D', '3', 4, 0, 0, 1, 0, 0, 0},
		"non-audio":        bytes.Repeat([]byte{'x'}, 65540),
		"wrong-rate":       wrongRate,
		"mono":             mono,
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := newRetailPlayerMP3Reader(bytes.NewReader(data)).Next(); err == nil || errors.Is(err, io.EOF) {
				t.Fatalf("expected invalid audio error, got %v", err)
			}
		})
	}
}

func TestRetailMP3ReaderResyncsAndParsesVariableBitrates(t *testing.T) {
	var data bytes.Buffer
	data.WriteString("junk")
	for _, bitrate := range []byte{5, 9, 14} {
		header := []byte{0xff, 0xfb, bitrate << 4, 0}
		length, _, ok := mp3ParseFrameHeader(header)
		if !ok {
			t.Fatal("invalid test header")
		}
		data.Write(header)
		data.Write(make([]byte, length-4))
	}
	r := newRetailPlayerMP3Reader(&data)
	for i := 0; i < 3; i++ {
		_, duration, err := r.Next()
		if err != nil || duration != retailPlayerMP3FrameDuration {
			t.Fatalf("bitrate changed audio clock: %v %v", duration, err)
		}
	}
}
