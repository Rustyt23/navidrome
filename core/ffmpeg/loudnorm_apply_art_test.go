package ffmpeg

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Cover art that lies about its format used to take the whole conversion down.
//
// Ten songs in a production library carried an ID3 APIC frame declaring
// image/png over JPEG bytes. ffmpeg believed the declaration, could not read a
// PNG header with it, and left the stream without dimensions - at which point
// copying it failed at the muxer and the song failed on every run, for ever,
// with no record of why.
//
// The tag is assembled by hand because ffmpeg will not produce one: asked to
// write image/png over JPEG bytes it corrects the mime and writes image/jpeg,
// so a fixture built with ffmpeg alone reproduces nothing and passes whatever
// it is asked.
func run(t *testing.T, name string, args ...string) {
	t.Helper()
	if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
		t.Fatalf("setup command failed: %v\n%s", err, out)
	}
}

func syncsafe(n int) []byte {
	return []byte{
		byte((n >> 21) & 0x7f), byte((n >> 14) & 0x7f),
		byte((n >> 7) & 0x7f), byte(n & 0x7f),
	}
}

// stripID3v2 drops a leading ID3v2 tag so our own can be the only one.
func stripID3v2(b []byte) []byte {
	if len(b) < 10 || !bytes.HasPrefix(b, []byte("ID3")) {
		return b
	}
	size := int(b[6]&0x7f)<<21 | int(b[7]&0x7f)<<14 | int(b[8]&0x7f)<<7 | int(b[9]&0x7f)
	if 10+size > len(b) {
		return b
	}
	return b[10+size:]
}

// id3WithAPIC builds an ID3v2.3 tag holding one attached picture, declaring
// whatever mime it is told regardless of what the bytes actually are.
func id3WithAPIC(mime string, pic []byte) []byte {
	var payload bytes.Buffer
	payload.WriteByte(0x00) // text encoding: ISO-8859-1
	payload.WriteString(mime)
	payload.WriteByte(0x00)
	payload.WriteByte(0x03) // picture type: front cover
	payload.WriteByte(0x00) // empty description
	payload.Write(pic)

	var frame bytes.Buffer
	frame.WriteString("APIC")
	_ = binary.Write(&frame, binary.BigEndian, uint32(payload.Len()))
	frame.Write([]byte{0x00, 0x00}) // frame flags
	frame.Write(payload.Bytes())

	var tag bytes.Buffer
	tag.WriteString("ID3")
	tag.Write([]byte{0x03, 0x00, 0x00}) // v2.3, no flags
	tag.Write(syncsafe(frame.Len()))
	tag.Write(frame.Bytes())
	return tag.Bytes()
}

// buildArtMP3 writes an mp3 whose cover art is JPEG data announced as declaredMime.
func buildArtMP3(t *testing.T, dir, name, declaredMime string) string {
	t.Helper()
	ff, err := ffmpegCmd()
	if err != nil {
		t.Skip("ffmpeg not available")
	}

	jpg := filepath.Join(dir, name+"-art.jpg")
	run(t, ff, "-nostdin", "-hide_banner", "-y", "-f", "lavfi",
		"-i", "color=c=blue:s=120x120:d=1", "-frames:v", "1", jpg)
	pic, err := os.ReadFile(jpg)
	if err != nil {
		t.Fatalf("reading art: %v", err)
	}

	audio := filepath.Join(dir, name+"-plain.mp3")
	run(t, ff, "-nostdin", "-hide_banner", "-y", "-f", "lavfi",
		"-i", "sine=frequency=440:duration=2", "-c:a", "libmp3lame", "-b:a", "128k", audio)
	raw, err := os.ReadFile(audio)
	if err != nil {
		t.Fatalf("reading audio: %v", err)
	}

	out := filepath.Join(dir, name+".mp3")
	if err := os.WriteFile(out, append(id3WithAPIC(declaredMime, pic), stripID3v2(raw)...), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return out
}

func TestProbeFlagsOnlyUndecodableArt(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	bad, err := ProbeFile(ctx, buildArtMP3(t, dir, "lying", "image/png"))
	if err != nil {
		t.Fatalf("probing: %v", err)
	}
	if !bad.HasArt {
		t.Fatal("fixture has no cover art, so it cannot exercise the bug")
	}
	if !bad.ArtUndecodable {
		t.Fatal("art with no dimensions was not flagged, so Apply will never retry it")
	}

	// The retry costs two extra encodes, so honest art must never trigger it.
	ok, err := ProbeFile(ctx, buildArtMP3(t, dir, "honest", "image/jpeg"))
	if err != nil {
		t.Fatalf("probing: %v", err)
	}
	if !ok.HasArt {
		t.Fatal("fixture lost its art")
	}
	if ok.ArtUndecodable {
		t.Error("honest art was flagged as broken; every failure would pay for two extra encodes")
	}
}

func TestApplyRecoversUndecodableCoverArt(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	src := buildArtMP3(t, dir, "lying", "image/png")

	probe, err := ProbeFile(ctx, src)
	if err != nil {
		t.Fatalf("probing the fixture: %v", err)
	}
	if !probe.ArtUndecodable {
		t.Fatal("fixture does not reproduce the bug")
	}

	out := filepath.Join(dir, "out.mp3")
	if err := Apply(ctx, src, out, ApplySpec{GainDB: 0.5, Source: probe}); err != nil {
		t.Fatalf("Apply failed on a file with mislabelled art: %v", err)
	}

	after, err := ProbeFile(ctx, out)
	if err != nil {
		t.Fatalf("probing the result: %v", err)
	}
	// Getting through by quietly dropping the art would pass a weaker test
	// while destroying the artwork the run promises to preserve.
	if !after.HasArt {
		t.Fatal("cover art was lost")
	}
	if after.ArtUndecodable {
		t.Error("art is still unreadable in the result; it was copied but not repaired")
	}
	if st, err := os.Stat(out); err != nil || st.Size() == 0 {
		t.Errorf("no output written: %v", err)
	}
}

// Honest art must still copy without any of this getting in the way.
func TestApplyKeepsHonestArtUntouched(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	src := buildArtMP3(t, dir, "honest", "image/jpeg")

	probe, err := ProbeFile(ctx, src)
	if err != nil {
		t.Fatalf("probing: %v", err)
	}
	out := filepath.Join(dir, "out.mp3")
	if err := Apply(ctx, src, out, ApplySpec{GainDB: 0.5, Source: probe}); err != nil {
		t.Fatalf("Apply failed on a file with ordinary art: %v", err)
	}
	after, err := ProbeFile(ctx, out)
	if err != nil {
		t.Fatalf("probing the result: %v", err)
	}
	if !after.HasArt {
		t.Error("cover art was lost")
	}
}
