package nativeapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/gcpsync"
	"github.com/navidrome/navidrome/core/loudness"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
)

type syncMediaRepo struct{ *tests.MockMediaFileRepo }

func (m *syncMediaRepo) GetCursor(...model.QueryOptions) (model.MediaFileCursor, error) {
	return func(yield func(model.MediaFile, error) bool) {
		for _, mf := range m.Data {
			if !yield(*mf, nil) {
				return
			}
		}
	}, nil
}
func (m *syncMediaRepo) UpdatePath(id, path string) error {
	m.Data[id].Path = path
	return nil
}

func (m *syncMediaRepo) ClearReplayGain(id string) error {
	m.Data[id].RGTrackGain = nil
	m.Data[id].RGTrackPeak = nil
	m.Data[id].RGAlbumGain = nil
	m.Data[id].RGAlbumPeak = nil
	return nil
}

type observedGCPSync struct {
	gcpsync.Service
	done chan error
}

func (s *observedGCPSync) Sweep(ctx context.Context) error {
	err := s.Service.Sweep(ctx)
	s.done <- err
	return err
}

func setupSyncIntegration(t *testing.T) (*Router, *model.MediaFile, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("test gcloud uses a shell script")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is required")
	}
	saved := conf.Server
	t.Cleanup(func() { conf.Server = saved; loudness.ResetCache(); gcpsync.ResetNamingCache() })
	root := t.TempDir()
	conf.Server.MusicFolder = filepath.Join(root, "music")
	conf.Server.SyncFolder = filepath.Join(root, "sync")
	conf.Server.DataFolder = conf.NewDir(filepath.Join(root, "data"))
	conf.Server.GCPSync.Notifications.Enabled = false
	conf.Server.GCPSync.Enabled = true
	conf.Server.GCPSync.AutoUpload = false
	conf.Server.GCPSync.Bucket = "test-bucket"
	conf.Server.GCPSync.DryRun = false
	conf.Server.GCPSync.EnforceNaming = true
	conf.Server.GCPSync.CredentialsFile = ""
	conf.Server.GCPSync.GcloudPath = filepath.Join(root, "gcloud")
	conf.Server.Scanner.LoudnessNormalization.TargetLUFS = -12.6
	conf.Server.Scanner.LoudnessNormalization.TruePeak = -0.5
	conf.Server.Scanner.LoudnessNormalization.LRA = 11
	conf.Server.Scanner.LoudnessNormalization.Tolerance = 0.2
	conf.Server.Scanner.LoudnessNormalization.Backup = false
	loudness.ResetCache()
	gcpsync.ResetNamingCache()
	gcpsync.GetInstance().Start(context.Background())
	bucket := filepath.Join(root, "bucket")
	if err := os.MkdirAll(bucket, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GCPSYNC_TEST_BUCKET", bucket)
	script := `#!/bin/sh
if [ "$2" = "ls" ]; then exit 0; fi
if [ "$2" != "cp" ]; then exit 1; fi
if [ -e "$GCPSYNC_TEST_BUCKET/fail" ]; then echo 'simulated upload failure' >&2; exit 1; fi
cp "$3" "$GCPSYNC_TEST_BUCKET/${4##*/}"
`
	if err := os.WriteFile(conf.Server.GCPSync.GcloudPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	mf := &model.MediaFile{ID: "sync-song", LibraryPath: conf.Server.MusicFolder, Path: "Album/Artist - Song.mp3", Album: "Pool", Year: 2026}
	if err := os.MkdirAll(filepath.Dir(mf.AbsolutePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	// The sine is already around -22 LUFS, safely within the automatic gain limit.
	runSyncFFmpeg(t, "-f", "lavfi", "-i", "sine=frequency=440:duration=6", "-c:a", "libmp3lame", "-b:a", "192k", "-ar", "44100", "-ac", "2", "-metadata", "artist=Artist", "-metadata", "title=Song", mf.AbsolutePath())
	repo := &syncMediaRepo{tests.CreateMockMediaFileRepo()}
	repo.Data[mf.ID] = mf
	ds := &tests.MockDataStore{MockedMediaFile: repo}
	return &Router{ds: ds}, mf, bucket
}

func runSyncFFmpeg(t *testing.T, args ...string) []byte {
	t.Helper()
	cmd := exec.Command("ffmpeg", append([]string{"-y", "-nostdin", "-hide_banner", "-loglevel", "error"}, args...)...)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("ffmpeg: %v (%s)", err, output)
	}
	return output
}

func syncDigest(t *testing.T, path string) [32]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(data)
}

func clickSyncAndWait(t *testing.T, api *Router, wantErr bool) {
	t.Helper()
	observed := &observedGCPSync{Service: gcpsync.GetInstance(), done: make(chan error, 1)}
	api.gcpSync = observed
	router := chi.NewRouter()
	router.Route("/api", api.addSyncRoute)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/sync", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("Sync returned %d: %s", response.Code, response.Body)
	}
	select {
	case err := <-observed.done:
		if (err != nil) != wantErr {
			t.Fatalf("Sync error = %v, expected failure = %v", err, wantErr)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Sync did not finish")
	}
}

func TestGCPSyncNormalizedMP3ReachesBucketThroughSyncButton(t *testing.T) {
	api, mf, bucket := setupSyncIntegration(t)
	original := syncDigest(t, mf.AbsolutePath())
	normalizer := ffmpeg.NewLoudnessNormalizer()
	result, err := optimizeOneTrack(context.Background(), api.ds, normalizer, mf)
	if err != nil || !result.Changed {
		t.Fatalf("normalization did not change the fixture: %+v, %v", result, err)
	}
	updated := syncDigest(t, mf.AbsolutePath())
	if updated == original {
		t.Fatal("normalization left audio unchanged")
	}
	staged := gcpsync.StagingPath(mf.LibraryPath, mf.AbsolutePath())
	if got := syncDigest(t, staged); got != updated {
		t.Fatal("staging does not contain normalized bytes")
	}
	object := filepath.Join(bucket, filepath.Base(mf.Path))
	if _, err := os.Stat(object); !os.IsNotExist(err) {
		t.Fatalf("file uploaded before Sync was clicked: %v", err)
	}
	if err := os.WriteFile(object, []byte("outdated bucket copy"), 0o644); err != nil {
		t.Fatal(err)
	}
	clickSyncAndWait(t, api, false)
	if got := syncDigest(t, object); got != updated {
		t.Fatal("bucket does not contain normalized bytes")
	}
	target := loudnessRunOptions(context.Background(), api.ds, mf).Target
	analysis, err := normalizer.AnalyzeLoudness(context.Background(), object, target)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(analysis.InputIntegrated-target.IntegratedLUFS) > 0.2 {
		t.Fatalf("uploaded LUFS = %g, want %g", analysis.InputIntegrated, target.IntegratedLUFS)
	}
	// A no-op optimize must not queue or upload unchanged audio again.
	result, err = optimizeOneTrack(context.Background(), api.ds, normalizer, mf)
	if err != nil || result.Changed {
		t.Fatalf("already-normalized result: %+v, %v", result, err)
	}
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Fatalf("unchanged song was restaged: %v", err)
	}
}

func TestGCPSyncCoverArtAndNormalizedAudioSurviveFailedSyncAndRetry(t *testing.T) {
	api, mf, bucket := setupSyncIntegration(t)
	result, err := optimizeOneTrack(context.Background(), api.ds, ffmpeg.NewLoudnessNormalizer(), mf)
	if err != nil || !result.Changed {
		t.Fatalf("normalization: %+v, %v", result, err)
	}
	audioBeforeCover := runSyncFFmpeg(t, "-i", mf.AbsolutePath(), "-map", "0:a:0", "-c:a", "copy", "-f", "data", "pipe:1")
	mf.CoverPath = "covercache/sync-test.png"
	coverPath := filepath.Join(conf.Server.DataFolder.String(), mf.CoverPath)
	if err := os.MkdirAll(filepath.Dir(coverPath), 0o755); err != nil {
		t.Fatal(err)
	}
	var cover bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 16), uint8(y * 16), 140, 255})
		}
	}
	if err := png.Encode(&cover, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(coverPath, cover.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	summary, err := api.saveFetchedMetadataToSongs()
	if err != nil || summary.Saved != 1 || summary.Remaining != 0 {
		t.Fatalf("save cover: %+v, %v", summary, err)
	}
	staged := gcpsync.StagingPath(mf.LibraryPath, mf.AbsolutePath())
	updated := syncDigest(t, mf.AbsolutePath())
	if syncDigest(t, staged) != updated {
		t.Fatal("staging missed the embedded cover")
	}
	object := filepath.Join(bucket, filepath.Base(mf.Path))
	if _, err := os.Stat(object); !os.IsNotExist(err) {
		t.Fatalf("file uploaded before Sync was clicked: %v", err)
	}
	if err := os.WriteFile(object, []byte("previous version"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := syncDigest(t, object)
	fail := filepath.Join(bucket, "fail")
	if err := os.WriteFile(fail, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	clickSyncAndWait(t, api, true)
	if syncDigest(t, staged) != updated || syncDigest(t, mf.AbsolutePath()) != updated {
		t.Fatal("failed upload lost the updated song")
	}
	if syncDigest(t, object) != previous {
		t.Fatal("failed upload replaced the bucket copy")
	}
	if err := os.Remove(fail); err != nil {
		t.Fatal(err)
	}
	clickSyncAndWait(t, api, false)
	if syncDigest(t, object) != updated {
		t.Fatal("retry did not upload the exact updated MP3")
	}
	embedded := runSyncFFmpeg(t, "-i", object, "-map", "0:v:0", "-c:v", "copy", "-f", "image2pipe", "pipe:1")
	if !bytes.Equal(embedded, cover.Bytes()) {
		t.Fatal("uploaded MP3 does not contain the selected cover image")
	}
	audioAfterCover := runSyncFFmpeg(t, "-i", object, "-map", "0:a:0", "-c:a", "copy", "-f", "data", "pipe:1")
	if !bytes.Equal(audioBeforeCover, audioAfterCover) {
		t.Fatal("saving/uploading cover art changed the normalized audio packets")
	}
}

func TestGCPSyncRoutesRejectIncompleteConfig(t *testing.T) {
	saved := conf.Server
	t.Cleanup(func() { conf.Server = saved })
	for _, missing := range []string{"disabled", "bucket", "sync folder"} {
		t.Run(missing, func(t *testing.T) {
			conf.Server.GCPSync.Enabled = missing != "disabled"
			conf.Server.GCPSync.Bucket = "test-bucket"
			conf.Server.SyncFolder = t.TempDir()
			if missing == "bucket" {
				conf.Server.GCPSync.Bucket = " "
			}
			if missing == "sync folder" {
				conf.Server.SyncFolder = " "
			}
			router := chi.NewRouter()
			api := &Router{}
			api.addSyncRoute(router)
			api.addGCPSyncRoute(router)
			for _, request := range []struct{ method, path string }{{http.MethodGet, "/sync"}, {http.MethodPost, "/gcpsync/sweep"}} {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(request.method, request.path, nil))
				if response.Code != http.StatusServiceUnavailable {
					t.Errorf("%s %s: status %d, want 503", request.method, request.path, response.Code)
				}
			}
		})
	}
}

func TestGCPSyncNormalizationCorrectsLocalAndBucketNamesFromTags(t *testing.T) {
	api, mf, bucket := setupSyncIntegration(t)
	canonical := mf.AbsolutePath()
	wrong := filepath.Join(filepath.Dir(canonical), "Wrong Artist - Wrong Title.mp3")
	if err := os.Rename(canonical, wrong); err != nil {
		t.Fatal(err)
	}
	mf.Path = "Album/Wrong Artist - Wrong Title.mp3"
	oldStaged := gcpsync.StagingPath(mf.LibraryPath, wrong)
	if err := gcpsync.GetInstance().StageMP3(mf.LibraryPath, wrong, "earlier edit"); err != nil {
		t.Fatal(err)
	}
	playlist := filepath.Join(conf.Server.MusicFolder, "Mix.m3u")
	if err := os.WriteFile(playlist, []byte("#EXTM3U\nWrong Artist - Wrong Title.mp3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := optimizeOneTrack(context.Background(), api.ds, ffmpeg.NewLoudnessNormalizer(), mf)
	if err != nil || !result.Changed {
		t.Fatalf("normalization: %+v %v", result, err)
	}
	contents, err := os.ReadFile(playlist)
	if err != nil || !bytes.Contains(contents, []byte("Artist - Song.mp3")) || bytes.Contains(contents, []byte("Wrong Artist")) {
		t.Fatalf("playlist references were not updated: %s %v", contents, err)
	}
	if _, err := os.Stat(oldStaged); !os.IsNotExist(err) {
		t.Fatal("obsolete staged version could overwrite the newer normalized upload")
	}
	if mf.Path != "Album/Artist - Song.mp3" {
		t.Fatalf("library path: %s", mf.Path)
	}
	if _, err := os.Stat(wrong); !os.IsNotExist(err) {
		t.Fatal("old local filename remains")
	}
	clickSyncAndWait(t, api, false)
	if syncDigest(t, filepath.Join(bucket, "Artist - Song.mp3")) != syncDigest(t, canonical) {
		t.Fatal("canonical bucket file differs from renamed library file")
	}
}

func TestGCPSyncLocalRenameRefusesExistingDifferentFile(t *testing.T) {
	api, mf, _ := setupSyncIntegration(t)
	canonical := mf.AbsolutePath()
	wrong := filepath.Join(filepath.Dir(canonical), "misnamed.mp3")
	if err := os.Rename(canonical, wrong); err != nil {
		t.Fatal(err)
	}
	mf.Path = "Album/misnamed.mp3"
	before := syncDigest(t, wrong)
	if err := os.WriteFile(canonical, []byte("different song"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gcpsync.StageSong(context.Background(), api.ds, mf, "updated"); err == nil {
		t.Fatal("rename silently overwrote an existing song")
	}
	if syncDigest(t, wrong) != before {
		t.Fatal("source changed on collision")
	}
	got, _ := os.ReadFile(canonical)
	if string(got) != "different song" {
		t.Fatal("destination changed on collision")
	}
	if syncDigest(t, gcpsync.StagingPath(mf.LibraryPath, wrong)) != before {
		t.Fatal("rename conflict lost the updated song from Sync staging")
	}
}

func TestGCPSyncNormalizedMP3RemainsStagedWhenNamePreparationFails(t *testing.T) {
	for _, failure := range []string{"canonical destination conflict", "pending playlist conflict"} {
		t.Run(failure, func(t *testing.T) {
			api, mf, bucket := setupSyncIntegration(t)
			canonical := mf.AbsolutePath()
			wrong := filepath.Join(filepath.Dir(canonical), "misnamed.mp3")
			if err := os.Rename(canonical, wrong); err != nil {
				t.Fatal(err)
			}
			mf.Path = "Album/misnamed.mp3"
			original := syncDigest(t, wrong)
			conf.Server.Scanner.LoudnessNormalization.BackupFolder = filepath.Join(t.TempDir(), "originals")
			if err := loudness.SetBackupEnabled(t.Context(), api.ds, true); err != nil {
				t.Fatal(err)
			}

			wantError := "already exists; resolve the duplicate first"
			preserved := map[string][32]byte{}
			if failure == "canonical destination conflict" {
				if err := os.WriteFile(canonical, []byte("different song already at the canonical name"), 0o644); err != nil {
					t.Fatal(err)
				}
				preserved[canonical] = syncDigest(t, canonical)
			} else {
				wantError = "has different content; sync or reconcile it first"
				playlist := filepath.Join(conf.Server.MusicFolder, "Mix.m3u")
				pendingPlaylist := filepath.Join(conf.Server.SyncFolder, "Mix.m3u")
				if err := os.WriteFile(playlist, []byte("#EXTM3U\nmisnamed.mp3\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(pendingPlaylist, []byte("#EXTM3U\nindependent pending edit.mp3\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				preserved[playlist] = syncDigest(t, playlist)
				preserved[pendingPlaylist] = syncDigest(t, pendingPlaylist)
			}

			result, err := optimizeOneTrack(t.Context(), api.ds, ffmpeg.NewLoudnessNormalizer(), mf)
			if !result.Changed || err == nil || !strings.Contains(err.Error(), wantError) || !strings.Contains(err.Error(), "audio normalized, but sync preparation failed") {
				t.Fatalf("normalization did not retain its preparation failure: %+v, %v", result, err)
			}
			updated := syncDigest(t, wrong)
			if updated == original || mf.Path != "Album/misnamed.mp3" {
				t.Fatal("normalization did not update the original library path")
			}
			staged := gcpsync.StagingPath(mf.LibraryPath, wrong)
			if syncDigest(t, staged) != updated {
				t.Fatal("preparation failure lost the exact normalized bytes from Sync/mp3")
			}
			backup := ffmpeg.FindLoudnessBackup(conf.Server.Scanner.LoudnessNormalization.BackupFolder, mf.LibraryPath, mf.ID, wrong)
			if backup == "" || syncDigest(t, backup) != original {
				t.Fatal("preparation failure changed or lost the original backup")
			}
			for path, digest := range preserved {
				if syncDigest(t, path) != digest {
					t.Fatalf("preparation failure changed unrelated pending/library data: %s", path)
				}
			}
			audit, err := api.ds.LoudnessAudit(t.Context()).Get(mf.ID)
			if err != nil || audit.Status != model.LoudnessStatusProcessed {
				t.Fatalf("normalized song lost its LUFS audit: %+v, %v", audit, err)
			}
			if _, err := os.Stat(filepath.Join(bucket, "Artist - Song.mp3")); !os.IsNotExist(err) {
				t.Fatalf("preparation failure unexpectedly uploaded the song: %v", err)
			}
		})
	}
}

func TestGCPSyncNormalizedMP3RemainsStagedWhenUploadsAreUnconfigured(t *testing.T) {
	for _, unavailable := range []string{"disabled", "missing bucket"} {
		t.Run(unavailable, func(t *testing.T) {
			api, mf, bucket := setupSyncIntegration(t)
			if unavailable == "disabled" {
				conf.Server.GCPSync.Enabled = false
			} else {
				conf.Server.GCPSync.Bucket = ""
			}
			conf.Server.GCPSync.AutoUpload = true
			// Retention must not depend on canonical rename preparation while
			// uploads are disabled or waiting for their bucket configuration.
			old := mf.AbsolutePath()
			wrong := filepath.Join(filepath.Dir(old), "arbitrary.mp3")
			if err := os.Rename(old, wrong); err != nil {
				t.Fatal(err)
			}
			mf.Path = "Album/arbitrary.mp3"
			before := syncDigest(t, wrong)
			result, err := optimizeOneTrack(t.Context(), api.ds, ffmpeg.NewLoudnessNormalizer(), mf)
			if err != nil || !result.Changed {
				t.Fatalf("normalization: %+v, %v", result, err)
			}
			updated := syncDigest(t, wrong)
			if updated == before || mf.Path != "Album/arbitrary.mp3" {
				t.Fatal("disabled uploads did not preserve the normalized library song")
			}
			if syncDigest(t, gcpsync.StagingPath(mf.LibraryPath, wrong)) != updated {
				t.Fatal("disabled or missing-bucket uploads lost the normalized bytes from Sync/mp3")
			}
			if _, err := os.Stat(filepath.Join(bucket, "Artist - Song.mp3")); !os.IsNotExist(err) {
				t.Fatalf("unconfigured uploads unexpectedly reached the bucket: %v", err)
			}
		})
	}
}

func TestGCPSyncUnicodeEquivalentFilenameIsNotADuplicate(t *testing.T) {
	api, mf, _ := setupSyncIntegration(t)
	old := mf.AbsolutePath()
	target := filepath.Join(filepath.Dir(old), "Artist - Cafe\u0301.mp3")
	runSyncFFmpeg(t, "-i", old, "-map", "0", "-c", "copy", "-metadata", "title=Café", target)
	if err := os.Remove(old); err != nil {
		t.Fatal(err)
	}
	mf.Path = "Album/Artist - Cafe\u0301.mp3"
	if err := gcpsync.StageSong(context.Background(), api.ds, mf, "metadata updated"); err != nil {
		t.Fatal(err)
	}
	if syncDigest(t, gcpsync.StagingPath(mf.LibraryPath, mf.AbsolutePath())) != syncDigest(t, mf.AbsolutePath()) {
		t.Fatal("Unicode-equivalent name did not stage correctly")
	}
}
