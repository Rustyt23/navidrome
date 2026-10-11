package gcpsync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/navidrome/navidrome/conf"
)

func taggedMP3(t *testing.T, path, artist, title string) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	args := []string{"-v", "error", "-y", "-f", "lavfi", "-i", "sine=duration=1", "-c:a", "libmp3lame", "-metadata", "artist=" + artist, "-metadata", "title=" + title, path}
	if out, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v %s", err, out)
	}
}

func TestMetadataNameIgnoresExistingFilename(t *testing.T) {
	for _, old := range []string{"wrong.mp3", "03 - Wrong Artist - Wrong Title.mp3", "Wrong Artist - Wrong Title.mp3"} {
		t.Run(old, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), old)
			taggedMP3(t, path, "Real Artist", "Real Title")
			name, err := MetadataName(path)
			if err != nil || name != "Real Artist - Real Title.mp3" {
				t.Fatalf("name=%q err=%v", name, err)
			}
		})
	}
}

func TestMetadataNameRequiresTagsEvenWithValidFilename(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Artist - Title.mp3")
	taggedMP3(t, path, "", "")
	if _, err := MetadataName(path); err == nil {
		t.Fatal("missing tags silently fell back to filename")
	}
}

func TestMetadataNamedUploadPreservesBytesAndSendsSummary(t *testing.T) {
	syncDir, _, _ := setupSweep(t, nil)
	bucket := captureBucket(t)
	path := filepath.Join(syncDir, "mp3", "Wrong Artist - Wrong Title.mp3")
	taggedMP3(t, path, "Real Artist", "Real Title")
	expected, _ := os.ReadFile(path)
	svc := newService()
	svc.readName = nil // Exercise the actual TagLib reader.
	var reports []syncReport
	svc.notify = func(_ context.Context, r syncReport) error { reports = append(reports, r); return nil }
	if err := svc.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filepath.Join(bucket, "Real Artist - Real Title.mp3"))
	if err != nil || !bytes.Equal(expected, actual) {
		t.Fatalf("upload changed audio or missed canonical object: %v", err)
	}
	if len(reports) != 1 || len(reports[0].Uploaded) != 1 || reports[0].Uploaded[0] != "Real Artist - Real Title.mp3" || len(reports[0].Failed) != 0 {
		t.Fatalf("reports: %+v", reports)
	}
}

func TestSyncNotificationIncludesFailures(t *testing.T) {
	syncDir, _, _ := setupSweep(t, nil)
	path := filepath.Join(syncDir, "mp3", "Artist - Title.mp3")
	taggedMP3(t, path, "", "")
	svc := newService()
	svc.readName = nil
	var report syncReport
	svc.notify = func(_ context.Context, r syncReport) error { report = r; return nil }
	if err := svc.Sweep(context.Background()); err == nil {
		t.Fatal("expected metadata failure")
	}
	if len(report.Failed) != 1 || !strings.Contains(report.body(), "artist and title") {
		t.Fatalf("missing failure details: %+v", report)
	}
	if !exists(path) {
		t.Fatal("failed file was removed")
	}
}

func TestNotificationMessageUsesConfiguredRecipient(t *testing.T) {
	sender, to, message, err := notificationMessage("sender@example.com", "New Recipient <new@example.com>", "Sync result", "Uploaded: 2\nFailed: 1")
	if err != nil || sender != "sender@example.com" || to != "new@example.com" {
		t.Fatalf("addresses: %q %q %v", sender, to, err)
	}
	if !strings.Contains(string(message), "new@example.com") || !strings.Contains(string(message), "Uploaded: 2\r\nFailed: 1") {
		t.Fatalf("message: %s", message)
	}
	for _, bad := range []string{"not-an-email", "new@example.com\r\nBcc: bad@example.com"} {
		if _, _, _, err := notificationMessage(sender, bad, "Sync", "body"); err == nil {
			t.Fatal(fmt.Sprintf("accepted bad recipient %q", bad))
		}
	}
}

func TestAutomaticUploadUsesMetadataAndReportsResult(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	bucket := captureBucket(t)
	song := filepath.Join(musicDir, "misnamed.mp3")
	taggedMP3(t, song, "Tagged Artist", "Tagged Title")
	svc := newService()
	svc.readName = nil
	var reports []syncReport
	svc.notify = func(_ context.Context, r syncReport) error {
		reports = append(reports, r)
		return fmt.Errorf("simulated SMTP outage")
	}
	if err := svc.StageMP3(musicDir, song, "LUFS normalized"); err != nil {
		t.Fatal(err)
	}
	req := <-svc.queue
	svc.processUpload(context.Background(), req)
	if !exists(filepath.Join(bucket, "Tagged Artist - Tagged Title.mp3")) {
		t.Fatal("canonical object missing")
	}
	if exists(req.path) {
		t.Fatal("SMTP failure incorrectly retained an already-uploaded copy")
	}
	if len(reports) != 1 || len(reports[0].Uploaded) != 1 || reports[0].Operation != "Automatic upload" {
		t.Fatalf("reports: %+v", reports)
	}
	svc.processUpload(context.Background(), req)
	if len(reports) != 1 {
		t.Fatal("stale queue entry sent duplicate mail")
	}
}

func TestPlaylistNameRewritePreservesCommentsAndLineEndings(t *testing.T) {
	source := []byte("#EXTM3U\r\n#EXTINF:12,Old Title\r\nOld.mp3\r\nAlbum/Old.mp3\r\nAlbum\\Old.mp3\r\nOther.mp3\r\n")
	got := rewritePlaylistName(source, "Old.mp3", "Artist - Title.mp3")
	want := "#EXTM3U\r\n#EXTINF:12,Old Title\r\nArtist - Title.mp3\r\nAlbum/Artist - Title.mp3\r\nAlbum\\Artist - Title.mp3\r\nOther.mp3\r\n"
	if string(got) != want {
		t.Fatalf("playlist: %q", got)
	}
}

func TestFailedAudioUploadDoesNotBlockPlaylists(t *testing.T) {
	for _, failure := range []string{"metadata", "audio"} {
		t.Run(failure, func(t *testing.T) {
			syncDir, _, _ := setupSweep(t, nil)
			bucket := independentPlaylistBucket(t, failure)
			playlistNames := []string{"A Playlist.m3u", "B Empty.m3u"}
			writeFile(t, filepath.Join(syncDir, playlistNames[0]), "bad.mp3\n")
			writeFile(t, filepath.Join(syncDir, playlistNames[1]), "")
			staged := filepath.Join(syncDir, "mp3", "bad.mp3")
			writeFile(t, staged, "audio")
			svc := newService()
			metadataErr := errors.New("missing artist metadata")
			wantFailure := "simulated MP3 upload failure"
			if failure == "metadata" {
				svc.readName = func(string) (string, error) { return "", metadataErr }
				wantFailure = metadataErr.Error()
			}
			var report syncReport
			svc.notify = func(_ context.Context, r syncReport) error { report = r; return nil }

			err := svc.Sweep(context.Background())
			if err == nil || !strings.Contains(err.Error(), wantFailure) {
				t.Fatalf("expected original MP3 failure, got %v", err)
			}
			if failure == "metadata" && !errors.Is(err, metadataErr) {
				t.Fatalf("metadata error was replaced: %v", err)
			}
			assertContent(t, staged, "audio")
			if exists(filepath.Join(bucket, "bad.mp3")) {
				t.Fatal("failed audio reached the bucket")
			}
			assertContent(t, filepath.Join(bucket, playlistNames[0]), "bad.mp3\n")
			assertContent(t, filepath.Join(bucket, playlistNames[1]), "")
			for _, name := range playlistNames {
				if exists(filepath.Join(syncDir, name)) {
					t.Fatalf("successfully published playlist remains staged: %s", name)
				}
			}
			if !slices.Equal(report.Uploaded, playlistNames) || len(report.Failed) != 1 || !strings.Contains(report.Failed[0], wantFailure) {
				t.Fatalf("successful playlists or original MP3 failure missing from report: %+v", report)
			}
		})
	}
}

func TestBucketListingFailureDoesNotBlockPlaylists(t *testing.T) {
	syncDir, _, _ := setupSweep(t, nil)
	bucket := independentPlaylistBucket(t, "listing")
	writeFile(t, filepath.Join(syncDir, "drop", "Artist - Song.mp3"), "unclassified audio")
	writeFile(t, filepath.Join(syncDir, "Playlist.m3u"), "Artist - Song.mp3\n")
	var report syncReport
	svc := newService()
	svc.notify = func(_ context.Context, r syncReport) error { report = r; return nil }

	if err := svc.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(bucket, "Artist - Song.mp3")) {
		t.Fatal("unstaged MP3 uploaded despite failed bucket listing")
	}
	assertContent(t, filepath.Join(bucket, "Playlist.m3u"), "Artist - Song.mp3\n")
	if !slices.Equal(report.Uploaded, []string{"Playlist.m3u"}) {
		t.Fatalf("published playlist missing from report: %+v", report)
	}
}

func TestFailedPlaylistUploadDoesNotBlockNextPlaylist(t *testing.T) {
	syncDir, _, _ := setupSweep(t, nil)
	bucket := independentPlaylistBucket(t, "playlist")
	failed := filepath.Join(syncDir, "A Failed.m3u")
	next := filepath.Join(syncDir, "B Next.m3u")
	writeFile(t, failed, "first.mp3\n")
	writeFile(t, next, "next.mp3\n")
	var report syncReport
	svc := newService()
	svc.notify = func(_ context.Context, r syncReport) error { report = r; return nil }

	err := svc.Sweep(context.Background())
	if err == nil || !strings.Contains(err.Error(), "simulated playlist upload failure") {
		t.Fatalf("expected first playlist failure, got %v", err)
	}
	assertContent(t, failed, "first.mp3\n")
	if exists(filepath.Join(bucket, "A Failed.m3u")) {
		t.Fatal("failed playlist reached the bucket")
	}
	assertContent(t, filepath.Join(bucket, "B Next.m3u"), "next.mp3\n")
	if exists(next) {
		t.Fatal("successfully published next playlist remains staged")
	}
	if !slices.Equal(report.Uploaded, []string{"B Next.m3u"}) || len(report.Failed) != 1 || !strings.Contains(report.Failed[0], "simulated playlist upload failure") {
		t.Fatalf("next playlist or original failure missing from report: %+v", report)
	}
}

// Capture actual playlist bytes while selectively failing one other operation.
func independentPlaylistBucket(t *testing.T, failure string) string {
	t.Helper()
	bucket := t.TempDir()
	t.Setenv("GCPSYNC_TEST_BUCKET", bucket)
	t.Setenv("GCPSYNC_TEST_FAILURE", failure)
	conf.Server.PlaylistsPath = t.TempDir()
	conf.Server.GCPSync.LastVersionFolder = t.TempDir()
	script := `#!/bin/sh
if [ "$2" = "ls" ]; then
 if [ "$GCPSYNC_TEST_FAILURE" = "listing" ]; then echo 'simulated bucket listing failure' >&2; exit 1; fi
 exit 0
fi
if [ "$2" != "cp" ]; then exit 1; fi
if [ "$GCPSYNC_TEST_FAILURE" = "audio" ] && [ "${4##*.}" = "mp3" ]; then
 echo 'simulated MP3 upload failure' >&2; exit 1
fi
if [ "$GCPSYNC_TEST_FAILURE" = "playlist" ] && [ "${4##*/}" = "A Failed.m3u" ]; then
 echo 'simulated playlist upload failure' >&2; exit 1
fi
cp "$3" "$GCPSYNC_TEST_BUCKET/${4##*/}" || exit 1
`
	if err := os.WriteFile(conf.Server.GCPSync.GcloudPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bucket
}
