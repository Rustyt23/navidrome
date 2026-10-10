package gcpsync

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

func TestFailedAudioUploadDoesNotPublishRenamedPlaylist(t *testing.T) {
	syncDir, _, logPath := setupSweep(t, nil)
	playlist := filepath.Join(syncDir, "A Playlist.m3u")
	writeFile(t, playlist, "Tagged Artist - Tagged Title.mp3\n")
	writeFile(t, filepath.Join(syncDir, "mp3", "bad.mp3"), "audio")
	svc := newService()
	svc.readName = func(string) (string, error) { return "", fmt.Errorf("missing artist metadata") }
	if err := svc.Sweep(context.Background()); err == nil {
		t.Fatal("expected MP3 failure")
	}
	if got := uploadedObjects(t, logPath); len(got) != 0 {
		t.Fatalf("playlist published before its audio: %v", got)
	}
	if !exists(playlist) {
		t.Fatal("deferred playlist was removed")
	}
}
