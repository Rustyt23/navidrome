package gcpsync

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/navidrome/navidrome/conf"
)

func TestScheduledAndManualModesHoldLatestEditsUntilSweep(t *testing.T) {
	for _, mode := range []struct {
		name       string
		autoUpload bool
		schedule   string
	}{
		{"scheduled", true, "CRON_TZ=Asia/Kolkata 0 2 * * *"},
		{"schedule_disabled", true, "0"},
		{"schedule_disabled_with_whitespace", true, " 0 "},
		{"manual_with_schedule", false, "0 2 * * *"},
	} {
		t.Run(mode.name, func(t *testing.T) {
			syncDir, musicDir, _ := setupSweep(t, nil)
			bucket := captureBucket(t)
			conf.Server.GCPSync.AutoUpload = mode.autoUpload
			conf.Server.GCPSync.Schedule = mode.schedule
			svc := newService()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			svc.Start(ctx)
			if !exists(filepath.Join(syncDir, "mp3")) {
				t.Fatal("startup did not create the staging folder")
			}
			song := filepath.Join(musicDir, "Album", "Artist - Song.mp3")
			object := filepath.Join(bucket, filepath.Base(song))
			writeFile(t, object, "previous bucket version")
			writeFile(t, song, "normalized audio")
			if err := svc.StageMP3(musicDir, song, "LUFS normalized"); err != nil {
				t.Fatal(err)
			}
			writeFile(t, song, "normalized audio with cover art")
			if err := svc.StageMP3(musicDir, song, "cover art embedded"); err != nil {
				t.Fatal(err)
			}
			svc.EnqueueMP3(song, "direct upload request")
			if len(svc.queue) != 0 || len(svc.pending) != 0 {
				t.Fatal("immediate upload bypassed the configured schedule/manual mode")
			}
			assertContent(t, object, "previous bucket version")
			staged := StagingPath(musicDir, song)
			assertContent(t, staged, "normalized audio with cover art")

			// Both a cron callback and the manual Sync button invoke Sweep.
			if err := svc.Sweep(ctx); err != nil {
				t.Fatal(err)
			}
			assertContent(t, object, "normalized audio with cover art")
			assertContent(t, song, "normalized audio with cover art")
			if exists(staged) || exists(libraryCopyMarker(staged)) {
				t.Fatal("successfully uploaded staged copy was not retired")
			}
		})
	}
}

func TestEmptyScheduleKeepsImmediateUploads(t *testing.T) {
	for _, schedule := range []string{"", " \t "} {
		t.Run("schedule_"+schedule, func(t *testing.T) {
			_, musicDir, _ := setupSweep(t, nil)
			bucket := captureBucket(t)
			conf.Server.GCPSync.Schedule = schedule
			svc := newService()
			song := filepath.Join(musicDir, "Artist - Song.mp3")
			writeFile(t, song, "updated audio")
			if err := svc.StageMP3(musicDir, song, "updated"); err != nil {
				t.Fatal(err)
			}
			if len(svc.queue) != 1 {
				t.Fatalf("immediate queue length = %d, want 1", len(svc.queue))
			}
			svc.processUpload(context.Background(), <-svc.queue)
			assertContent(t, filepath.Join(bucket, filepath.Base(song)), "updated audio")
			assertContent(t, song, "updated audio")
		})
	}
}
