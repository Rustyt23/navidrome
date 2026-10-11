package loudness

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
)

type processingMediaRepo struct {
	*tests.MockMediaFileRepo
	clearedReplayGain bool
}

func (r *processingMediaRepo) ClearReplayGain(string) error {
	r.clearedReplayGain = true
	return nil
}

func processingConfig(t *testing.T) {
	t.Helper()
	saved := conf.Server
	ResetCache()
	t.Cleanup(func() { conf.Server = saved; ResetCache() })
	conf.Server.Scanner.LoudnessNormalization.TargetLUFS = -12.6
	conf.Server.Scanner.LoudnessNormalization.TruePeak = -0.5
	conf.Server.Scanner.LoudnessNormalization.LRA = 11
	conf.Server.Scanner.LoudnessNormalization.Tolerance = 0.2
	conf.Server.Scanner.LoudnessNormalization.Backup = false
}

func TestRunOptionsUsesStoredBackupSettingAndToleranceFallback(t *testing.T) {
	processingConfig(t)
	conf.Server.Scanner.LoudnessNormalization.Backup = true
	conf.Server.Scanner.LoudnessNormalization.Tolerance = 0
	ds := &tests.MockDataStore{}
	if err := SetBackupEnabled(t.Context(), ds, false); err != nil {
		t.Fatal(err)
	}
	mf := &model.MediaFile{ID: "song", LibraryPath: "/library"}
	opts := RunOptions(t.Context(), ds, mf)
	if opts.Backup || opts.Tolerance != conf.DefaultLoudnessNormalizationTolerance || opts.LibraryPath != mf.LibraryPath || opts.MediaFileID != mf.ID {
		t.Fatalf("incorrect shared options: %+v", opts)
	}
}

func TestOptimizeTrackRecordsFailureButNotCancellation(t *testing.T) {
	processingConfig(t)
	for _, cancelled := range []bool{false, true} {
		mf := &model.MediaFile{ID: "missing", LibraryPath: t.TempDir(), Path: "missing.mp3"}
		ds := &tests.MockDataStore{}
		ctx, cancel := context.WithCancel(t.Context())
		if cancelled {
			cancel()
		}
		_, err := OptimizeTrack(ctx, ds, nil, mf)
		cancel()
		if err == nil {
			t.Fatal("missing file was not rejected")
		}
		audit, err := ds.LoudnessAudit(context.Background()).Get(mf.ID)
		if cancelled {
			if err == nil {
				t.Fatal("cancelled work wrote a failure audit")
			}
		} else if err != nil || audit.Status != model.LoudnessStatusFailed {
			t.Fatalf("failed work was not recorded: %+v, %v", audit, err)
		}
	}
}

func TestOptimizeTrackPreservesNonMP3FormatWithoutSyncStaging(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is required")
	}
	processingConfig(t)
	root := t.TempDir()
	conf.Server.GCPSync.Enabled = true
	conf.Server.GCPSync.Bucket = "test-bucket"
	conf.Server.GCPSync.AutoUpload = false
	conf.Server.SyncFolder = filepath.Join(root, "sync")
	mf := &model.MediaFile{ID: "wave-song", LibraryPath: root, Path: "arbitrary.wav"}
	cmd := exec.Command("ffmpeg", "-y", "-nostdin", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=6", "-c:a", "pcm_s16le", "-ar", "44100", "-ac", "2", mf.AbsolutePath())
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("creating audio: %v (%s)", err, output)
	}
	repo := &processingMediaRepo{MockMediaFileRepo: tests.CreateMockMediaFileRepo()}
	repo.Data[mf.ID] = mf
	ds := &tests.MockDataStore{MockedMediaFile: repo}
	release, busy := ClaimFileWork("test optimisation")
	if busy != "" {
		t.Fatal(busy)
	}
	defer release()
	res, err := OptimizeTrack(t.Context(), ds, ffmpeg.NewLoudnessNormalizer(), mf)
	if err != nil || !res.Changed {
		t.Fatalf("optimisation did not change WAV fixture: %+v, %v", res, err)
	}
	if mf.Path != "arbitrary.wav" || res.AfterSet.Probe.Codec != "pcm_s16le" {
		t.Fatalf("non-MP3 was renamed or converted: %s, %s", mf.Path, res.AfterSet.Probe.Codec)
	}
	if !repo.clearedReplayGain || mf.Size == 0 || repo.LastUpdatedLoudnessID != mf.ID {
		t.Fatal("audio rewrite did not update library properties and ReplayGain")
	}
	if _, err := os.Stat(conf.Server.SyncFolder); !os.IsNotExist(err) {
		t.Fatalf("non-MP3 created Sync staging: %v", err)
	}
	audit, err := ds.LoudnessAudit(t.Context()).Get(mf.ID)
	if err != nil || audit.Status != model.LoudnessStatusProcessed {
		t.Fatalf("successful rewrite missing processed audit: %+v, %v", audit, err)
	}
}
