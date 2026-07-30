package ffmpeg

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSilenceIntervalsUsesOnlyCommonChannelEdges(t *testing.T) {
	output := []byte(strings.Join([]string{
		"channel: 0 | silence_start: 0",
		"channel: 1 | silence_start: 0",
		"channel: 0 | silence_end: 1.0 | silence_duration: 1.0",
		"channel: 1 | silence_end: 0.8 | silence_duration: 0.8",
		"channel: 0 | silence_start: 2.0",
		"channel: 0 | silence_end: 2.7 | silence_duration: 0.7",
		"channel: 0 | silence_start: 3.0",
		"channel: 1 | silence_start: 3.2",
		"channel: 0 | silence_end: 4.0 | silence_duration: 1.0",
		"channel: 1 | silence_end: 4.0 | silence_duration: 0.8",
	}, "\n"))

	intervals, err := parseSilenceIntervals(output, 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	leading, trailing, err := commonEdgeSilence(intervals, 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	if leading != 0.8 {
		t.Fatalf("leading silence = %v, want shortest channel edge 0.8", leading)
	}
	if math.Abs(trailing-0.8) > 0.000001 {
		t.Fatalf("trailing silence = %v, want shortest channel edge 0.8", trailing)
	}
}

func TestCommonEdgeSilenceNeverBridgesAudibleEdgeMaterial(t *testing.T) {
	intervals := map[int][]silenceInterval{
		0: {
			{Start: 0.005, End: 1.0},
			{Start: 2.0, End: 3.8},
		},
	}
	leading, trailing, err := commonEdgeSilence(intervals, 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	if leading != 0 {
		t.Fatalf("leading silence = %v; a 5ms opening transient must be protected", leading)
	}
	if trailing != 0 {
		t.Fatalf("trailing silence = %v; a 200ms audible outro must be protected", trailing)
	}
}

func TestSilenceTrimMethodBlocksUnverifiedLossyContainers(t *testing.T) {
	if _, err := SilenceTrimMethod(&FileProbe{Codec: "opus"}); err == nil {
		t.Fatal("Opus must be blocked until its container seek semantics are verified")
	}
	if got, err := SilenceTrimMethod(&FileProbe{Codec: "mp3"}); err != nil ||
		got != SilenceTrimMethodPacketCopy {
		t.Fatalf("MP3 method = %q, %v", got, err)
	}
	if got, err := SilenceTrimMethod(&FileProbe{Codec: "flac"}); err != nil ||
		got != SilenceTrimMethodLossless {
		t.Fatalf("FLAC method = %q, %v", got, err)
	}
	if _, err := SilenceTrimMethod(&FileProbe{Codec: "flac", Channels: 6}); err == nil {
		t.Fatal("multichannel lossless audio must be blocked until channel layouts are preserved and verified")
	}
	for _, codec := range []string{"alac", "wavpack"} {
		if _, err := SilenceTrimMethod(&FileProbe{Codec: codec, Channels: 2}); err == nil {
			t.Fatalf("%s must be blocked until its encoder sample format is preserved and verified", codec)
		}
	}
}

func TestSilenceTrimBackupIsSeparateAndRenameStable(t *testing.T) {
	dir := t.TempDir()
	library := filepath.Join(dir, "music")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	track := filepath.Join(library, "before.mp3")
	original := []byte("pre-trim bytes")
	if err := os.WriteFile(track, original, 0o640); err != nil {
		t.Fatal(err)
	}
	hash, err := BackupSilenceTrimOriginal(track, 0o640, library, "", "song-1")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "" {
		t.Fatal("backup hash is empty")
	}
	if _, err := os.Stat(SilenceTrimBackupHashPath("", library, "song-1")); err != nil {
		t.Fatalf("checksum sidecar was not stored: %v", err)
	}
	if SilenceTrimBackupPath("", library, "song-1") ==
		LoudnessBackupPath("", library, "song-1", track) {
		t.Fatal("silence and LUFS backups must never share a path")
	}

	renamed := filepath.Join(library, "renamed.mp3")
	if err := os.Rename(track, renamed); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(renamed, []byte("trimmed bytes"), 0o640); err != nil {
		t.Fatal(err)
	}
	trimmedHash, err := FileSHA256(renamed)
	if err != nil {
		t.Fatal(err)
	}
	if err := RestoreSilenceTrimOriginal(
		renamed,
		library,
		"",
		"song-1",
		hash,
		trimmedHash,
	); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(renamed)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatalf("restored %q, want %q", got, original)
	}
}

func TestSilenceTrimBackupRejectsAnotherFileGeneration(t *testing.T) {
	dir := t.TempDir()
	library := filepath.Join(dir, "music")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	track := filepath.Join(library, "song.mp3")
	if err := os.WriteFile(track, []byte("first generation"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := BackupSilenceTrimOriginal(track, 0o640, library, "", "song-1"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(track, []byte("another song"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := BackupSilenceTrimOriginal(track, 0o640, library, "", "song-1"); err == nil {
		t.Fatal("an ID-keyed backup must not be reused for different source bytes")
	}
}

func TestLosslessSilenceTrimKeepsRetainedPCM(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ffmpegPath = "ffmpeg"
	ffmpegErr = nil

	dir := t.TempDir()
	source := filepath.Join(dir, "source.flac")
	candidate := filepath.Join(dir, "candidate.flac")
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo:d=1",
		"-f", "lavfi", "-i", "sine=frequency=1000:sample_rate=44100:duration=2",
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo:d=1",
		"-filter_complex",
		"[1:a]pan=stereo|c0=c0|c1=c0[tone];[0:a][tone][2:a]concat=n=3:v=0:a=1[out]",
		"-map", "[out]", source,
	}
	if output, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("creating fixture: %v: %s", err, output)
	}

	report, err := DetectEdgeSilence(context.Background(), source, 0.5, -70)
	if err != nil {
		t.Fatal(err)
	}
	if report.Leading.Strict < 0.99 || report.Trailing.Strict < 0.99 {
		t.Fatalf("edges = %#v / %#v, want about one second each", report.Leading, report.Trailing)
	}
	padding := int64(0.25 * float64(report.Probe.SampleRate))
	start := int64(report.Leading.Strict*float64(report.Probe.SampleRate)) - padding
	end := int64(report.Trailing.Strict*float64(report.Probe.SampleRate)) - padding
	method, err := WriteSilenceTrimCandidate(
		context.Background(),
		source,
		candidate,
		report.Probe,
		start,
		end,
	)
	if err != nil {
		t.Fatal(err)
	}
	if method != SilenceTrimMethodLossless {
		t.Fatalf("method = %q", method)
	}
	if err := VerifyLosslessRetainedPCM(
		context.Background(),
		source,
		candidate,
		report.Probe,
		start,
		end,
	); err != nil {
		t.Fatal(err)
	}
	after, err := DetectEdgeSilence(context.Background(), candidate, 0.05, -70)
	if err != nil {
		t.Fatal(err)
	}
	if after.Leading.Strict < 0.23 || after.Trailing.Strict < 0.23 {
		t.Fatalf("retained edges = %#v / %#v, want the 250ms guards", after.Leading, after.Trailing)
	}
}

func TestPacketCopySilenceTrimKeepsLossyPackets(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ffmpegPath = "ffmpeg"
	ffmpegErr = nil

	for _, fixture := range []struct {
		name      string
		extension string
		codecArgs []string
	}{
		{name: "MP3", extension: ".mp3", codecArgs: []string{"-c:a", "libmp3lame", "-b:a", "192k"}},
		{name: "AAC", extension: ".m4a", codecArgs: []string{"-c:a", "aac", "-b:a", "192k"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source"+fixture.extension)
			candidate := filepath.Join(dir, "candidate"+fixture.extension)
			args := []string{
				"-hide_banner", "-loglevel", "error",
				"-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo:d=1",
				"-f", "lavfi", "-i", "sine=frequency=1000:sample_rate=44100:duration=2",
				"-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo:d=1",
				"-filter_complex",
				"[1:a]pan=stereo|c0=c0|c1=c0[tone];[0:a][tone][2:a]concat=n=3:v=0:a=1[out]",
				"-map", "[out]",
			}
			args = append(args, fixture.codecArgs...)
			args = append(args, source)
			if output, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
				t.Skipf("creating %s fixture: %v: %s", fixture.name, err, output)
			}

			report, err := DetectEdgeSilence(context.Background(), source, 0.5, -70)
			if err != nil {
				t.Fatal(err)
			}
			if report.Leading.Strict < 0.8 || report.Trailing.Strict < 0.8 {
				t.Fatalf("edges = %#v / %#v, want codec silence at both ends", report.Leading, report.Trailing)
			}
			padding := int64(0.25 * float64(report.Probe.SampleRate))
			start := int64(report.Leading.Strict*float64(report.Probe.SampleRate)) - padding
			end := int64(report.Trailing.Strict*float64(report.Probe.SampleRate)) - padding
			method, err := WriteSilenceTrimCandidate(
				context.Background(),
				source,
				candidate,
				report.Probe,
				start,
				end,
			)
			if err != nil {
				t.Fatal(err)
			}
			if method != SilenceTrimMethodPacketCopy {
				t.Fatalf("method = %q", method)
			}
			if err := VerifyCopiedPackets(
				context.Background(),
				source,
				candidate,
				report.Probe,
				start,
				end,
			); err != nil {
				t.Fatal(err)
			}
			after, err := DetectEdgeSilence(context.Background(), candidate, 0.05, -70)
			if err != nil {
				t.Fatal(err)
			}
			if after.Leading.Strict < 0.15 || after.Trailing.Strict < 0.15 {
				t.Fatalf("retained edges = %#v / %#v, packet copy cut too close", after.Leading, after.Trailing)
			}
		})
	}
}

func TestPacketCopySilenceTrimPreservesAttachedCoverArt(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	ffmpegPath = "ffmpeg"
	ffmpegErr = nil

	dir := t.TempDir()
	source := filepath.Join(dir, "source.mp3")
	candidate := filepath.Join(dir, "candidate.mp3")
	cover := filepath.Join(dir, "cover.jpg")
	if output, err := exec.Command(
		"ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=red:s=32x32",
		"-frames:v", "1",
		cover,
	).CombinedOutput(); err != nil {
		t.Skipf("creating cover-art fixture: %v: %s", err, output)
	}
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo:d=1",
		"-f", "lavfi", "-i", "sine=frequency=1000:sample_rate=44100:duration=2",
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo:d=1",
		"-i", cover,
		"-filter_complex",
		"[1:a]pan=stereo|c0=c0|c1=c0[tone];[0:a][tone][2:a]concat=n=3:v=0:a=1[out]",
		"-map", "[out]", "-map", "3:v:0",
		"-c:a", "libmp3lame", "-b:a", "192k",
		"-c:v", "copy", "-disposition:v:0", "attached_pic",
		"-metadata", "title=Silence integrity fixture",
		"-metadata", "artist=Navidrome test artist",
		"-metadata", "album=Navidrome test album",
		"-metadata", "trim_proof=keep-me",
		source,
	}
	if output, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Skipf("creating cover-art MP3 fixture: %v: %s", err, output)
	}

	report, err := DetectEdgeSilence(context.Background(), source, 0.5, -70)
	if err != nil {
		t.Fatal(err)
	}
	if report.Probe.AttachedPicStreams != 1 {
		t.Fatalf("source attached pictures = %d", report.Probe.AttachedPicStreams)
	}
	sourceArtHash, err := commandSHA256(
		context.Background(),
		report.Probe.Duration,
		"-nostdin", "-hide_banner", "-i", source,
		"-map", "0:v:0", "-c:v", "copy",
		"-f", "hash", "-hash", "sha256", "-",
	)
	if err != nil {
		t.Fatal(err)
	}
	sourceTags := probeFormatTags(t, source)
	padding := int64(0.25 * float64(report.Probe.SampleRate))
	start := int64(report.Leading.Strict*float64(report.Probe.SampleRate)) - padding
	end := int64(report.Trailing.Strict*float64(report.Probe.SampleRate)) - padding
	if _, err := WriteSilenceTrimCandidate(
		context.Background(),
		source,
		candidate,
		report.Probe,
		start,
		end,
	); err != nil {
		t.Fatal(err)
	}
	after, err := ProbeFile(context.Background(), candidate)
	if err != nil {
		t.Fatal(err)
	}
	if !after.HasArt || after.AttachedPicStreams != 1 {
		t.Fatalf("candidate lost cover art: %#v", after)
	}
	candidateArtHash, err := commandSHA256(
		context.Background(),
		after.Duration,
		"-nostdin", "-hide_banner", "-i", candidate,
		"-map", "0:v:0", "-c:v", "copy",
		"-f", "hash", "-hash", "sha256", "-",
	)
	if err != nil {
		t.Fatal(err)
	}
	if candidateArtHash != sourceArtHash {
		t.Fatalf("attached cover bytes changed: %s -> %s", sourceArtHash, candidateArtHash)
	}
	if candidateTags := probeFormatTags(t, candidate); candidateTags != sourceTags {
		t.Fatalf("format metadata changed:\nsource:\n%s\ncandidate:\n%s", sourceTags, candidateTags)
	}
	if err := VerifyCopiedPackets(
		context.Background(),
		source,
		candidate,
		report.Probe,
		start,
		end,
	); err != nil {
		t.Fatal(err)
	}
}

func probeFormatTags(t *testing.T, path string) string {
	t.Helper()
	output, err := exec.Command(
		"ffprobe",
		"-v", "error",
		"-show_entries", "format_tags=title,artist,album,trim_proof",
		"-of", "default=noprint_wrappers=1",
		path,
	).CombinedOutput()
	if err != nil {
		t.Fatalf("probing format tags: %v: %s", err, output)
	}
	return strings.TrimSpace(string(output))
}
