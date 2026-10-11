package gcpsync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/navidrome/navidrome/conf"
)

// Capture the bytes actually consumed by gcloud, including overwrite behavior.
func captureBucket(t *testing.T) string {
	t.Helper()
	bucket := t.TempDir()
	t.Setenv("GCPSYNC_TEST_BUCKET", bucket)
	script := `#!/bin/sh
if [ "$2" = "ls" ]; then exit 0; fi
if [ "$2" != "cp" ]; then exit 1; fi
if [ -e "$GCPSYNC_TEST_BUCKET/fail" ]; then exit 1; fi
cp "$3" "$GCPSYNC_TEST_BUCKET/${4##*/}" || exit 1
if [ -e "$GCPSYNC_TEST_BUCKET/pause" ]; then
 touch "$GCPSYNC_TEST_BUCKET/started"
 while [ -e "$GCPSYNC_TEST_BUCKET/pause" ]; do sleep 0.01; done
fi
`
	writeFile(t, conf.Server.GCPSync.GcloudPath, script)
	if err := os.Chmod(conf.Server.GCPSync.GcloudPath, 0o755); err != nil {
		t.Fatal(err)
	}
	return bucket
}

func assertContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("%s: got %q, %v; want %q", path, got, err, want)
	}
}

func TestManualSyncUploadsQueuedEdits(t *testing.T) {
	for _, reason := range []string{"LUFS normalized", "cover art embedded"} {
		t.Run(reason, func(t *testing.T) {
			syncDir, musicDir, _ := setupSweep(t, nil)
			bucket := captureBucket(t)
			song := filepath.Join(musicDir, "Album", "Artist - Song.mp3")
			object := filepath.Join(bucket, filepath.Base(song))
			writeFile(t, object, "old bucket audio")
			writeFile(t, song, reason)
			svc := newService()
			svc.StageMP3(musicDir, song, reason)
			assertContent(t, filepath.Join(syncDir, "mp3", "Album", filepath.Base(song)), reason)
			if err := svc.Sweep(context.Background()); err != nil {
				t.Fatal(err)
			}
			assertContent(t, object, reason)
			assertContent(t, song, reason)
		})
	}
}

func TestUploadKeepsEditRestagedDuringTransfer(t *testing.T) {
	syncDir, musicDir, _ := setupSweep(t, nil)
	bucket := captureBucket(t)
	song := filepath.Join(musicDir, "Album", "Artist - Song.mp3")
	staged := filepath.Join(syncDir, "mp3", "Album", filepath.Base(song))
	writeFile(t, song, "LUFS")
	svc := newService()
	svc.StageMP3(musicDir, song, "LUFS normalized")
	req := <-svc.queue
	pause := filepath.Join(bucket, "pause")
	writeFile(t, pause, "")
	done := make(chan struct{})
	go func() { svc.processUpload(context.Background(), req); close(done) }()
	t.Cleanup(func() { _ = os.Remove(pause); <-done })
	waitSyncFile(t, filepath.Join(bucket, "started"))
	writeFile(t, song, "LUFS and cover art")
	svc.StageMP3(musicDir, song, "cover art embedded")
	if err := os.Remove(pause); err != nil {
		t.Fatal(err)
	}
	<-done
	assertContent(t, song, "LUFS and cover art")
	assertContent(t, staged, "LUFS and cover art")
	if err := svc.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertContent(t, filepath.Join(bucket, filepath.Base(song)), "LUFS and cover art")
}

func TestUploadDoesNotRestoreOldCopyOverNewLibraryEdit(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	bucket := captureBucket(t)
	song := filepath.Join(musicDir, "Artist - Song.mp3")
	writeFile(t, song, "LUFS")
	svc := newService()
	svc.StageMP3(musicDir, song, "LUFS normalized")
	// The cover writer has completed but has not yet called StageMP3.
	writeFile(t, song, "LUFS and cover art")
	svc.processUpload(context.Background(), <-svc.queue)
	assertContent(t, song, "LUFS and cover art")
	assertContent(t, filepath.Join(bucket, filepath.Base(song)), "LUFS")
}

func waitSyncFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if exists(path) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func TestStagedLibraryCopySurvivesRestartWithoutRollingBackLibrary(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "main library", true: "other library"}[external], func(t *testing.T) {
			syncDir, musicDir, _ := setupSweep(t, nil)
			bucket := captureBucket(t)
			library := musicDir
			if external {
				library = t.TempDir()
			}
			song := filepath.Join(library, "Album", "Artist - Song.mp3")
			writeFile(t, song, "normalized")
			newService().StageMP3(library, song, "LUFS normalized")
			writeFile(t, song, "newer local cover")
			// A fresh service has no pending queue or remembered source paths.
			if err := newService().Sweep(context.Background()); err != nil {
				t.Fatal(err)
			}
			assertContent(t, song, "newer local cover")
			assertContent(t, filepath.Join(bucket, filepath.Base(song)), "normalized")
			staged := filepath.Join(syncDir, "mp3", "Album", filepath.Base(song))
			if exists(staged) || exists(libraryCopyMarker(staged)) {
				t.Fatal("uploaded copy/marker not retired")
			}
			if external && exists(filepath.Join(musicDir, "Album", filepath.Base(song))) {
				t.Fatal("external library song was duplicated into the main library")
			}
		})
	}
}

func TestFailedUploadLeavesCopyForManualRetry(t *testing.T) {
	syncDir, musicDir, _ := setupSweep(t, nil)
	bucket := captureBucket(t)
	song := filepath.Join(musicDir, "Artist - Song.mp3")
	staged := filepath.Join(syncDir, "mp3", filepath.Base(song))
	writeFile(t, song, "normalized and cover embedded")
	svc := newService()
	svc.StageMP3(musicDir, song, "cover art embedded")
	writeFile(t, filepath.Join(bucket, "fail"), "")
	if err := svc.Sweep(context.Background()); err == nil {
		t.Fatal("expected upload failure")
	}
	assertContent(t, staged, "normalized and cover embedded")
	assertContent(t, song, "normalized and cover embedded")
	if err := os.Remove(filepath.Join(bucket, "fail")); err != nil {
		t.Fatal(err)
	}
	if err := svc.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertContent(t, filepath.Join(bucket, filepath.Base(song)), "normalized and cover embedded")
	if exists(staged) {
		t.Fatal("successful retry left staged copy")
	}
}

func TestAutomaticWorkerAndSyncButtonSendLatestEdit(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	bucket := captureBucket(t)
	song := filepath.Join(musicDir, "Artist - Song.mp3")
	writeFile(t, song, "normalized")
	svc := newService()
	pause := filepath.Join(bucket, "pause")
	writeFile(t, pause, "")
	svc.StageMP3(musicDir, song, "LUFS normalized")
	ctx, cancel := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); svc.runWorker(ctx) }()
	t.Cleanup(func() { _ = os.Remove(pause); cancel(); <-workerDone })
	waitSyncFile(t, filepath.Join(bucket, "started"))
	writeFile(t, song, "normalized plus cover")
	svc.StageMP3(musicDir, song, "cover art embedded")
	sweepDone := make(chan error, 1)
	go func() { sweepDone <- svc.Sweep(context.Background()) }()
	if err := os.Remove(pause); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-sweepDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("worker and manual sweep deadlocked")
	}
	cancel()
	<-workerDone
	assertContent(t, filepath.Join(bucket, filepath.Base(song)), "normalized plus cover")
	assertContent(t, song, "normalized plus cover")
}

func TestRenamedStagedCopyKeepsLibraryProvenance(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	bucket := captureBucket(t)
	original := filepath.Join(musicDir, "bad name.mp3")
	renamed := filepath.Join(musicDir, "Artist - Song.mp3")
	writeFile(t, original, "normalized")
	svc := newService()
	svc.StageMP3(musicDir, original, "LUFS normalized")
	if err := os.Rename(original, renamed); err != nil {
		t.Fatal(err)
	}
	oldStage, newStage := StagingPath(musicDir, original), StagingPath(musicDir, renamed)
	if err := RenameStagedCopy(oldStage, newStage); err != nil {
		t.Fatal(err)
	}
	writeFile(t, renamed, "newer cover")
	if err := newService().Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertContent(t, renamed, "newer cover")
	assertContent(t, filepath.Join(bucket, filepath.Base(renamed)), "normalized")
	if exists(oldStage) || exists(libraryCopyMarker(oldStage)) || exists(libraryCopyMarker(newStage)) {
		t.Fatal("renamed copy left stale files")
	}
}

func TestSyncPreservesUnicodeLibraryLayout(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	bucket := captureBucket(t)
	song := filepath.Join(musicDir, "Cafe\u0301", "Beyonce\u0301 - Song.mp3")
	writeFile(t, song, "normalized with art")
	svc := newService()
	svc.StageMP3(musicDir, song, "cover art embedded")
	if err := svc.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertContent(t, song, "normalized with art")
	assertContent(t, filepath.Join(bucket, "Beyoncé - Song.mp3"), "normalized with art")
}

func TestCancelledUploadKeepsStagedCopy(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	captureBucket(t)
	song := filepath.Join(musicDir, "Artist - Song.mp3")
	writeFile(t, song, "normalized")
	svc := newService()
	svc.StageMP3(musicDir, song, "LUFS normalized")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.processUpload(ctx, <-svc.queue)
	assertContent(t, StagingPath(musicDir, song), "normalized")
	assertContent(t, song, "normalized")
}

func TestInterruptedUploadKeepsNormalizedCopyAcrossRestart(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	bucket := captureBucket(t)
	song := filepath.Join(musicDir, "Artist - Song.mp3")
	writeFile(t, song, "normalized")
	svc := newService()
	if err := svc.StageMP3(musicDir, song, "LUFS normalized"); err != nil {
		t.Fatal(err)
	}
	staged := StagingPath(musicDir, song)
	pause := filepath.Join(bucket, "pause")
	writeFile(t, pause, "")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { svc.processUpload(ctx, <-svc.queue); close(done) }()
	t.Cleanup(func() { cancel(); _ = os.Remove(pause); <-done })
	waitSyncFile(t, filepath.Join(bucket, "started"))
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("interrupted upload did not stop")
	}
	assertContent(t, staged, "normalized")
	if !exists(libraryCopyMarker(staged)) {
		t.Fatal("interruption lost pending-file provenance")
	}
	if err := os.Remove(pause); err != nil {
		t.Fatal(err)
	}
	if err := newService().Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertContent(t, filepath.Join(bucket, filepath.Base(song)), "normalized")
	assertContent(t, song, "normalized")
	if exists(staged) || exists(libraryCopyMarker(staged)) {
		t.Fatal("successful retry did not retire the staged copy")
	}
}

func TestSyncUploadsStagedFilesWhenAutomaticQueueIsFull(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	bucket := captureBucket(t)
	svc := newService()
	svc.queue = make(chan uploadRequest, 1)
	for _, name := range []string{"Artist - LUFS.mp3", "Artist - Cover.mp3"} {
		song := filepath.Join(musicDir, name)
		writeFile(t, song, name)
		svc.StageMP3(musicDir, song, "updated")
	}
	if err := svc.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Artist - LUFS.mp3", "Artist - Cover.mp3"} {
		assertContent(t, filepath.Join(bucket, name), name)
	}
}

func TestDryRunPreservesStagedCopiesAndBucket(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	bucket := captureBucket(t)
	song := filepath.Join(musicDir, "Artist - Song.mp3")
	writeFile(t, song, "normalized and cover")
	svc := newService()
	svc.StageMP3(musicDir, song, "updated")
	staged := StagingPath(musicDir, song)
	conf.Server.GCPSync.DryRun = true
	if err := svc.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertContent(t, staged, "normalized and cover")
	assertContent(t, song, "normalized and cover")
	if exists(filepath.Join(bucket, filepath.Base(song))) {
		t.Fatal("dry run uploaded the file")
	}
	if !exists(libraryCopyMarker(staged)) {
		t.Fatal("dry run removed the provenance marker")
	}
}

func TestManualModeKeepsLUFSAndCoverEditsUntilSync(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	bucket := captureBucket(t)
	conf.Server.GCPSync.AutoUpload = false
	svc := newService()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx)
	song := filepath.Join(musicDir, "Artist - Song.mp3")
	object := filepath.Join(bucket, filepath.Base(song))
	writeFile(t, object, "old bucket version")
	writeFile(t, song, "normalized")
	svc.StageMP3(musicDir, song, "LUFS normalized")
	writeFile(t, song, "normalized with cover")
	svc.StageMP3(musicDir, song, "cover art embedded")
	if len(svc.queue) != 0 || len(svc.pending) != 0 {
		t.Fatal("manual mode queued an automatic upload")
	}
	assertContent(t, object, "old bucket version")
	assertContent(t, StagingPath(musicDir, song), "normalized with cover")
	if err := svc.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertContent(t, object, "normalized with cover")
	assertContent(t, song, "normalized with cover")
}

func TestUnavailableUploadsKeepNormalizedMP3ForRestartAndRetry(t *testing.T) {
	for _, unavailable := range []string{"service disabled", "bucket missing"} {
		t.Run(unavailable, func(t *testing.T) {
			_, musicDir, _ := setupSweep(t, nil)
			bucket := captureBucket(t)
			conf.Server.GCPSync.Enabled = unavailable != "service disabled"
			if unavailable == "bucket missing" {
				conf.Server.GCPSync.Bucket = ""
			}
			song := filepath.Join(musicDir, "Album", "Artist - Song.mp3")
			object := filepath.Join(bucket, filepath.Base(song))
			writeFile(t, object, "previous bucket version")
			writeFile(t, song, "normalized audio")
			svc := newService()
			if err := svc.StageMP3(musicDir, song, "LUFS normalized"); err != nil {
				t.Fatal(err)
			}
			staged := StagingPath(musicDir, song)
			assertContent(t, staged, "normalized audio")
			if !exists(libraryCopyMarker(staged)) || len(svc.queue) != 0 {
				t.Fatal("unavailable upload lost provenance or queued an upload")
			}
			// A fresh process must keep pending disk files while uploads are
			// unavailable and recover them once configuration is corrected.
			restarted := newService()
			if err := restarted.Sweep(context.Background()); err != nil {
				t.Fatal(err)
			}
			assertContent(t, staged, "normalized audio")
			assertContent(t, object, "previous bucket version")
			conf.Server.GCPSync.Enabled = true
			conf.Server.GCPSync.Bucket = "test-bucket"
			if err := restarted.Sweep(context.Background()); err != nil {
				t.Fatal(err)
			}
			assertContent(t, object, "normalized audio")
			assertContent(t, song, "normalized audio")
			if exists(staged) || exists(libraryCopyMarker(staged)) {
				t.Fatal("successful retry left the uploaded copy pending")
			}
		})
	}
}

func TestDirectQueuedMP3KeepsLibrarySource(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	bucket := captureBucket(t)
	song := filepath.Join(musicDir, "Artist - Song.mp3")
	writeFile(t, song, "updated")
	svc := newService()
	svc.EnqueueMP3(song, "updated")
	done := make(chan struct{})
	go func() { svc.processUpload(context.Background(), <-svc.queue); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("direct library upload deadlocked")
	}
	assertContent(t, song, "updated")
	assertContent(t, filepath.Join(bucket, filepath.Base(song)), "updated")
}
