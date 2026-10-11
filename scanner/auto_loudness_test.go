package scanner_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	_ "github.com/navidrome/navidrome/adapters/gotaglib"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/conf/configtest"
	"github.com/navidrome/navidrome/core"
	"github.com/navidrome/navidrome/core/artwork"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/gcpsync"
	"github.com/navidrome/navidrome/core/loudness"
	"github.com/navidrome/navidrome/core/metrics"
	"github.com/navidrome/navidrome/core/playlists"
	"github.com/navidrome/navidrome/db"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/persistence"
	"github.com/navidrome/navidrome/scanner"
	"github.com/navidrome/navidrome/server/events"
	"github.com/navidrome/navidrome/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Automatic loudness normalization of scanner imports", func() {
	var ctx context.Context
	var ds model.DataStore
	var s model.Scanner
	var incoming, canonical, uploadLog string

	newScanner := func(rootCtx context.Context) model.Scanner {
		return scanner.New(rootCtx, ds, artwork.NoopCacheWarmer(), events.NoopBroker(),
			playlists.NewPlaylists(ds, core.NewImageUploadService()), core.NewDiscoveries(ds), metrics.NewNoopInstance())
	}
	tracks := func() model.MediaFiles {
		files, err := ds.MediaFile(ctx).GetAll()
		Expect(err).ToNot(HaveOccurred())
		return files
	}
	pending := func() []string {
		value, err := ds.Property(ctx).DefaultGet("ScannerAutoLoudnessPending", "[]")
		Expect(err).ToNot(HaveOccurred())
		var ids []string
		Expect(json.Unmarshal([]byte(value), &ids)).To(Succeed())
		return ids
	}
	runScan := func(full bool) {
		warnings, err := s.ScanAll(ctx, full)
		Expect(err).ToNot(HaveOccurred())
		Expect(warnings).To(BeEmpty())
	}
	processedTrack := func(original [32]byte) (*model.MediaFile, *model.LoudnessAudit, [32]byte) {
		files := tracks()
		Expect(files).To(HaveLen(1))
		mf := files[0]
		Expect(mf.ID).ToNot(BeEmpty())
		Expect(mf.AbsolutePath()).To(Equal(canonical))
		Expect(incoming).ToNot(BeAnExistingFile())
		updated := autoLoudnessDigest(canonical)
		Expect(updated).ToNot(Equal(original))
		audit, err := ds.LoudnessAudit(ctx).Get(mf.ID)
		Expect(err).ToNot(HaveOccurred())
		Expect(audit.Status).To(Equal(model.LoudnessStatusProcessed))
		Expect(audit.HasBackup).To(BeTrue())
		Expect(audit.LufsAfter).ToNot(BeNil())
		Expect(*audit.LufsAfter).To(BeNumerically("~", conf.Server.Scanner.LoudnessNormalization.TargetLUFS, 0.2))
		backup := ffmpeg.FindLoudnessBackup(conf.Server.Scanner.LoudnessNormalization.BackupFolder,
			mf.LibraryPath, mf.ID, mf.AbsolutePath())
		Expect(backup).ToNot(BeEmpty())
		Expect(autoLoudnessDigest(backup)).To(Equal(original))
		Expect(autoLoudnessDigest(gcpsync.StagingPath(mf.LibraryPath, mf.AbsolutePath()))).To(Equal(updated))
		Expect(uploadLog).ToNot(BeAnExistingFile())
		Expect(pending()).To(BeEmpty())
		return &mf, audit, updated
	}

	BeforeEach(func() {
		tests.SkipOnWindows("the fake gcloud command uses a shell script")
		for _, command := range []string{"ffmpeg", "ffprobe"} {
			if _, err := exec.LookPath(command); err != nil {
				Skip(command + " is required for real audio integration tests")
			}
		}
		DeferCleanup(configtest.SetupConfig())
		loudness.ResetCache()
		gcpsync.ResetNamingCache()
		DeferCleanup(loudness.ResetCache)
		DeferCleanup(gcpsync.ResetNamingCache)
		root := GinkgoT().TempDir()
		conf.Server.MusicFolder = filepath.Join(root, "music")
		conf.Server.SyncFolder = filepath.Join(root, "sync")
		conf.Server.DataFolder = conf.NewDir(filepath.Join(root, "data"))
		conf.Server.PlaylistsPath = filepath.Join(root, "playlists", "**")
		conf.Server.DeletedPlaylistsPath = filepath.Join(root, "deleted-playlists")
		conf.Server.DiscoveryPath = filepath.Join(root, "discovery")
		conf.Server.DevExternalScanner = false
		conf.Server.Scanner.Extractor = "taglib"
		options := &conf.Server.Scanner.LoudnessNormalization
		options.Enabled = true
		options.TargetLUFS = -12.6
		options.TruePeak = -0.5
		options.LRA = 11
		options.Tolerance = 0.2
		options.Parallelism = 2
		options.Backup = true
		options.BackupFolder = filepath.Join(root, "backups")
		options.AuditDbFolder = filepath.Join(root, "audit")
		conf.Server.GCPSync.Enabled = true
		conf.Server.GCPSync.AutoUpload = false
		conf.Server.GCPSync.Schedule = "0"
		conf.Server.GCPSync.Bucket = "scanner-test-bucket"
		conf.Server.GCPSync.DryRun = false
		conf.Server.GCPSync.EnforceNaming = true
		conf.Server.GCPSync.CredentialsFile = ""
		conf.Server.GCPSync.Notifications.Enabled = false
		conf.Server.GCPSync.GcloudPath = filepath.Join(root, "gcloud")
		uploadLog = filepath.Join(root, "gcloud-called")
		GinkgoT().Setenv("SCANNER_AUTO_LUFS_UPLOAD_LOG", uploadLog)
		Expect(os.WriteFile(conf.Server.GCPSync.GcloudPath, []byte("#!/bin/sh\ntouch \"$SCANNER_AUTO_LUFS_UPLOAD_LOG\"\nexit 1\n"), 0o755)).To(Succeed())
		incoming = filepath.Join(conf.Server.MusicFolder, "Album", "incoming.mp3")
		canonical = filepath.Join(filepath.Dir(incoming), "Artist - Song.mp3")
		Expect(os.MkdirAll(filepath.Dir(incoming), 0o755)).To(Succeed())
		autoLoudnessWriteMP3(incoming, "Song")

		ctx = request.WithUser(GinkgoT().Context(), model.User{ID: "auto-lufs-admin", IsAdmin: true})
		db.Db().SetMaxOpenConns(1)
		db.Init(ctx)
		DeferCleanup(func() { Expect(tests.ClearDB()).To(Succeed()) })
		ds = persistence.New(db.Db())
		Expect(ds.User(ctx).Put(&model.User{ID: "auto-lufs-admin", UserName: "auto-lufs-admin",
			Name: "Scanner Test Admin", IsAdmin: true, NewPassword: "test-password"})).To(Succeed())
		Expect(ds.Library(ctx).Put(&model.Library{ID: 1, Name: "Auto LUFS Library", Path: conf.Server.MusicFolder})).To(Succeed())
		s = newScanner(ctx)
	})

	It("normalizes a new import, preserves its original, and stages the result despite a disabled one-shot UI switch", func() {
		original := autoLoudnessDigest(incoming)
		Expect(loudness.SetEnabled(ctx, ds, false)).To(Succeed())
		Expect(loudness.Enabled(ctx, ds)).To(BeFalse())
		runScan(false)
		mf, audit, updated := processedTrack(original)
		info, err := os.Stat(canonical)
		Expect(err).ToNot(HaveOccurred())
		modified := info.ModTime()

		// A normal scan and a forced metadata scan must not treat an existing
		// track as a new import or replace its audit with another measurement.
		for _, full := range []bool{false, true} {
			runScan(full)
			files := tracks()
			Expect(files).To(HaveLen(1))
			Expect(files[0].ID).To(Equal(mf.ID))
			Expect(autoLoudnessDigest(canonical)).To(Equal(updated))
			info, err = os.Stat(canonical)
			Expect(err).ToNot(HaveOccurred())
			Expect(info.ModTime()).To(Equal(modified))
			current, err := ds.LoudnessAudit(ctx).Get(mf.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(current.AnalyzedAt).To(Equal(audit.AnalyzedAt))
			Expect(pending()).To(BeEmpty())
		}
		Expect(uploadLog).ToNot(BeAnExistingFile())
	})

	It("leaves new imports untouched when the configuration switch is disabled", func() {
		conf.Server.Scanner.LoudnessNormalization.Enabled = false
		original := autoLoudnessDigest(incoming)
		runScan(false)
		files := tracks()
		Expect(files).To(HaveLen(1))
		Expect(files[0].AbsolutePath()).To(Equal(incoming))
		Expect(autoLoudnessDigest(incoming)).To(Equal(original))
		_, err := ds.LoudnessAudit(ctx).Get(files[0].ID)
		Expect(err).To(MatchError(model.ErrNotFound))
		Expect(pending()).To(BeEmpty())
		Expect(filepath.Join(conf.Server.SyncFolder, "mp3")).ToNot(BeADirectory())
		Expect(uploadLog).ToNot(BeAnExistingFile())
	})

	It("processes imports from multiple folders without losing queued songs or mixing their originals", func() {
		second := filepath.Join(conf.Server.MusicFolder, "Other Album", "another.mp3")
		Expect(os.MkdirAll(filepath.Dir(second), 0o755)).To(Succeed())
		autoLoudnessWriteMP3(second, "Other Song")
		originals := map[string][32]byte{"Song": autoLoudnessDigest(incoming), "Other Song": autoLoudnessDigest(second)}
		runScan(false)
		files := tracks()
		Expect(files).To(HaveLen(2))
		Expect(files[0].ID).ToNot(Equal(files[1].ID))
		for _, mf := range files {
			original, exists := originals[mf.Title]
			Expect(exists).To(BeTrue())
			Expect(filepath.Base(mf.Path)).To(Equal("Artist - " + mf.Title + ".mp3"))
			updated := autoLoudnessDigest(mf.AbsolutePath())
			Expect(updated).ToNot(Equal(original))
			audit, err := ds.LoudnessAudit(ctx).Get(mf.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(audit.Status).To(Equal(model.LoudnessStatusProcessed))
			Expect(audit.HasBackup).To(BeTrue())
			backup := ffmpeg.FindLoudnessBackup(conf.Server.Scanner.LoudnessNormalization.BackupFolder,
				mf.LibraryPath, mf.ID, mf.AbsolutePath())
			Expect(backup).ToNot(BeEmpty())
			Expect(autoLoudnessDigest(backup)).To(Equal(original))
			Expect(autoLoudnessDigest(gcpsync.StagingPath(mf.LibraryPath, mf.AbsolutePath()))).To(Equal(updated))
		}
		Expect(pending()).To(BeEmpty())
		Expect(uploadLog).ToNot(BeAnExistingFile())
	})

	It("does not normalize a previously imported song when its tags change or a full scan is requested", func() {
		conf.Server.Scanner.LoudnessNormalization.Enabled = false
		runScan(false)
		id := tracks()[0].ID
		autoLoudnessWriteMP3(incoming, "Updated Song")
		updated := autoLoudnessDigest(incoming)
		conf.Server.Scanner.LoudnessNormalization.Enabled = true
		runScan(true)
		files := tracks()
		Expect(files).To(HaveLen(1))
		Expect(files[0].ID).To(Equal(id))
		Expect(files[0].Title).To(Equal("Updated Song"))
		Expect(autoLoudnessDigest(incoming)).To(Equal(updated))
		_, err := ds.LoudnessAudit(ctx).Get(id)
		Expect(err).To(MatchError(model.ErrNotFound))
		Expect(pending()).To(BeEmpty())
		Expect(filepath.Join(conf.Server.SyncFolder, "mp3")).ToNot(BeADirectory())
	})

	It("reconciles a moved song before processing imports and preserves its identity and normalization audit", func() {
		runScan(false)
		mf := tracks()[0]
		audit, err := ds.LoudnessAudit(ctx).Get(mf.ID)
		Expect(err).ToNot(HaveOccurred())
		updated := autoLoudnessDigest(canonical)
		moved := filepath.Join(conf.Server.MusicFolder, "Moved Album", filepath.Base(canonical))
		Expect(os.MkdirAll(filepath.Dir(moved), 0o755)).To(Succeed())
		Expect(os.Rename(canonical, moved)).To(Succeed())
		runScan(true)
		files := tracks()
		Expect(files).To(HaveLen(1))
		Expect(files[0].ID).To(Equal(mf.ID))
		Expect(files[0].AbsolutePath()).To(Equal(moved))
		Expect(autoLoudnessDigest(moved)).To(Equal(updated))
		current, err := ds.LoudnessAudit(ctx).Get(mf.ID)
		Expect(err).ToNot(HaveOccurred())
		Expect(current.AnalyzedAt).To(Equal(audit.AnalyzedAt))
		Expect(current.Status).To(Equal(model.LoudnessStatusProcessed))
		Expect(pending()).To(BeEmpty())
		Expect(uploadLog).ToNot(BeAnExistingFile())
	})

	It("retains a canceled import while another LUFS job owns the files and retries it with a fresh controller", func() {
		original := autoLoudnessDigest(incoming)
		release, busy := loudness.ClaimFileWork("manual LUFS fixture")
		Expect(busy).To(BeEmpty())
		DeferCleanup(release)
		rootCtx, cancel := context.WithCancel(ctx)
		DeferCleanup(cancel)
		s = newScanner(rootCtx)
		done := make(chan error, 1)
		go func() {
			_, err := s.ScanAll(ctx, false)
			done <- err
		}()
		// LastScanAt is set after all scan phases. The queued import is now
		// waiting on the shared file-work claim, rather than inside a DB write.
		Eventually(func() bool {
			lib, err := ds.Library(ctx).Get(1)
			return err == nil && !lib.LastScanAt.IsZero() && lib.TotalSongs == 1
		}, 20*time.Second, 20*time.Millisecond).Should(BeTrue())
		files := tracks()
		Expect(files).To(HaveLen(1))
		id := files[0].ID
		Expect(pending()).To(Equal([]string{id}))
		cancel()
		Eventually(done, 10*time.Second).Should(Receive())
		Expect(scanner.IsScanning()).To(BeFalse())
		Expect(loudness.FileWorkBusy()).To(Equal("manual LUFS fixture"))
		Expect(autoLoudnessDigest(incoming)).To(Equal(original))
		Expect(pending()).To(Equal([]string{id}))
		_, err := ds.LoudnessAudit(ctx).Get(id)
		Expect(err).To(MatchError(model.ErrNotFound))

		release()
		s = newScanner(ctx)
		runScan(false)
		mf, _, _ := processedTrack(original)
		Expect(mf.ID).To(Equal(id))
		Expect(loudness.FileWorkBusy()).To(BeEmpty())
	})
})

func autoLoudnessWriteMP3(path, title string) {
	GinkgoHelper()
	// This signal is around -22 LUFS, within the engine's automatic gain limit.
	cmd := exec.Command("ffmpeg", "-y", "-nostdin", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=6", "-c:a", "libmp3lame",
		"-b:a", "192k", "-ar", "44100", "-ac", "2", "-metadata", "artist=Artist",
		"-metadata", "title="+title, "-metadata", "album=Pool", path)
	output, err := cmd.CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), "ffmpeg output: %s", output)
}

func autoLoudnessDigest(path string) [32]byte {
	GinkgoHelper()
	data, err := os.ReadFile(path)
	Expect(err).ToNot(HaveOccurred())
	return sha256.Sum256(data)
}
