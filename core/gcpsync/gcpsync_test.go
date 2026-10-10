package gcpsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/navidrome/navidrome/conf"
)

// stubGcloud writes a fake gcloud that records every invocation and answers
// "storage ls" with the object names it is given, so a sweep can be run end to
// end without touching a real bucket.
func stubGcloud(t *testing.T, dir string, bucketObjects []string) (bin string, logPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub gcloud is a shell script")
	}
	logPath = filepath.Join(dir, "gcloud.log")
	var listing strings.Builder
	for _, o := range bucketObjects {
		listing.WriteString("gs://test-bucket/" + o + "\n")
	}
	bin = filepath.Join(dir, "gcloud")
	// Logs one "CP <dest>" line per upload. The destination is written from a
	// single positional arg, so track names containing spaces survive.
	script := "#!/bin/sh\n" +
		"if [ \"$2\" = \"ls\" ]; then printf '%s' '" + listing.String() + "'; exit 0; fi\n" +
		"if [ \"$2\" = \"cp\" ]; then echo \"CP $4\" >> " + logPath + "; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, logPath
}

func gcloudCalls(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var calls []string
	for _, l := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if l != "" {
			calls = append(calls, l)
		}
	}
	return calls
}

func uploadedObjects(t *testing.T, logPath string) []string {
	t.Helper()
	var out []string
	for _, c := range gcloudCalls(t, logPath) {
		dest, ok := strings.CutPrefix(c, "CP ")
		if !ok {
			continue
		}
		out = append(out, strings.TrimPrefix(dest, "gs://test-bucket/"))
	}
	sort.Strings(out)
	return out
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// setupSweep lays out a Sync folder + music folder and points the config at
// them, with a stub gcloud pre-loaded with bucketObjects.
func setupSweep(t *testing.T, bucketObjects []string) (syncDir, musicDir, logPath string) {
	t.Helper()
	base := t.TempDir()
	syncDir = filepath.Join(base, "sync")
	musicDir = filepath.Join(base, "music")
	binDir := filepath.Join(base, "bin")
	for _, d := range []string{syncDir, musicDir, binDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	bin, logPath := stubGcloud(t, binDir, bucketObjects)

	saved := conf.Server
	t.Cleanup(func() { conf.Server = saved })
	conf.Server.SyncFolder = syncDir
	conf.Server.MusicFolder = musicDir
	// Unset so the tests below exercise the "<SyncFolder>/trash" fallback;
	// TestSweepTrashUsesDeletedPlaylistsPath sets it explicitly.
	conf.Server.DeletedPlaylistsPath = ""
	conf.Server.GCPSync.Notifications.Enabled = false
	conf.Server.GCPSync.Enabled = true
	conf.Server.GCPSync.AutoUpload = true
	conf.Server.GCPSync.Schedule = "" // Exercise immediate mode unless a test selects another mode.
	conf.Server.GCPSync.Bucket = "test-bucket"
	conf.Server.GCPSync.GcloudPath = bin
	conf.Server.GCPSync.DryRun = false
	conf.Server.GCPSync.EnforceNaming = true
	ResetNamingCache()
	t.Cleanup(ResetNamingCache)
	return syncDir, musicDir, logPath
}

// A processed MP3 in the staging folder overwrites the bucket object even
// though the bucket already holds it, and lands in the music folder at its
// path relative to the staging root rather than flattened at the library root.
func TestSweepStagedMP3OverwritesAndKeepsItsPath(t *testing.T) {
	syncDir, musicDir, logPath := setupSweep(t, []string{"Artist - Song.mp3"})
	staged := filepath.Join(syncDir, "mp3", "Artist", "Album", "Artist - Song.mp3")
	writeFile(t, staged, "normalized")

	if err := newService().Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := uploadedObjects(t, logPath); len(got) != 1 || got[0] != "Artist - Song.mp3" {
		t.Errorf("staged MP3 should overwrite the bucket object, uploaded %v", got)
	}
	dest := filepath.Join(musicDir, "Artist", "Album", "Artist - Song.mp3")
	if !exists(dest) {
		t.Errorf("staged MP3 should land at %s, preserving its relative path", dest)
	}
	if exists(filepath.Join(musicDir, "Artist - Song.mp3")) {
		t.Error("staged MP3 was flattened to the library root, which the scanner indexes as a duplicate")
	}
	if exists(staged) {
		t.Error("staged copy should be gone from the staging folder after the sweep")
	}
}

// An MP3 outside the staging folder that the bucket already holds is moved
// into the library but never re-uploaded.
func TestSweepExistingMP3IsNotUploadedAgain(t *testing.T) {
	syncDir, musicDir, logPath := setupSweep(t, []string{"Artist - Old.mp3"})
	incoming := filepath.Join(syncDir, "drop", "Artist - Old.mp3")
	writeFile(t, incoming, "same old")

	if err := newService().Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := uploadedObjects(t, logPath); len(got) != 0 {
		t.Errorf("MP3 already in the bucket should not be uploaded again, uploaded %v", got)
	}
	if !exists(filepath.Join(musicDir, "drop", "Artist - Old.mp3")) {
		t.Error("MP3 should still be moved into the music folder")
	}
}

// An MP3 outside the staging folder that the bucket does not hold is new
// material and does get uploaded.
func TestSweepNewMP3IsUploaded(t *testing.T) {
	syncDir, musicDir, logPath := setupSweep(t, []string{"Artist - Other.mp3"})
	incoming := filepath.Join(syncDir, "drop", "Artist - New.mp3")
	writeFile(t, incoming, "brand new")

	if err := newService().Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := uploadedObjects(t, logPath); len(got) != 1 || got[0] != "Artist - New.mp3" {
		t.Errorf("new MP3 should be uploaded, uploaded %v", got)
	}
	if !exists(filepath.Join(musicDir, "drop", "Artist - New.mp3")) {
		t.Error("new MP3 should be moved into the music folder")
	}
}

// Files that are neither .mp3 nor .m3u are junk: they go to the trash instead
// of overwriting whatever bucket object shares their basename.
func TestSweepTrashesUnsupportedFilesInsteadOfUploading(t *testing.T) {
	syncDir, _, logPath := setupSweep(t, nil)
	junk := filepath.Join(syncDir, "art", "cover.jpg")
	writeFile(t, junk, "not audio")

	if err := newService().Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := uploadedObjects(t, logPath); len(got) != 0 {
		t.Errorf("unsupported file should never reach the bucket, uploaded %v", got)
	}
	if exists(junk) {
		t.Error("unsupported file should have been moved out of the sync folder")
	}
	if !exists(filepath.Join(syncDir, "trash", "art", "cover.jpg")) {
		t.Error("unsupported file should be in the trash folder, keeping its relative path")
	}
}

// The sweep must not descend into its own trash, or each run would nest the
// previous run's trash one level deeper.
func TestSweepDoesNotReprocessTrash(t *testing.T) {
	syncDir, _, _ := setupSweep(t, nil)
	writeFile(t, filepath.Join(syncDir, "trash", "art", "cover.jpg"), "already trashed")

	if err := newService().Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}

	if !exists(filepath.Join(syncDir, "trash", "art", "cover.jpg")) {
		t.Error("file already in the trash should be left where it is")
	}
	if exists(filepath.Join(syncDir, "trash", "trash")) {
		t.Error("sweep nested the trash folder inside itself")
	}
}

// When the bucket cannot be listed, new and existing MP3s cannot be told
// apart, so nothing unstaged is uploaded - but staged files still overwrite.
func TestSweepSkipsUnstagedUploadsWhenBucketListingFails(t *testing.T) {
	syncDir, musicDir, logPath := setupSweep(t, nil)
	// A gcloud that fails "ls" but succeeds on "cp".
	failing := filepath.Join(t.TempDir(), "gcloud")
	writeFile(t, failing, "#!/bin/sh\n"+
		"if [ \"$2\" = \"ls\" ]; then echo 'boom' >&2; exit 1; fi\n"+
		"if [ \"$2\" = \"cp\" ]; then echo \"CP $4\" >> "+logPath+"; fi\n"+
		"exit 0\n")
	if err := os.Chmod(failing, 0o755); err != nil {
		t.Fatal(err)
	}
	conf.Server.GCPSync.GcloudPath = failing

	writeFile(t, filepath.Join(syncDir, "drop", "Artist - Unknown.mp3"), "unclassifiable")
	writeFile(t, filepath.Join(syncDir, "mp3", "Artist - Staged.mp3"), "processed")

	if err := newService().Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}

	got := uploadedObjects(t, logPath)
	if len(got) != 1 || got[0] != "Artist - Staged.mp3" {
		t.Errorf("only the staged MP3 should be uploaded when the bucket is unreadable, uploaded %v", got)
	}
	if !exists(filepath.Join(musicDir, "drop", "Artist - Unknown.mp3")) {
		t.Error("unclassifiable MP3 should still be filed into the music folder")
	}
}

// StageMP3 copies a library file into the staging folder under its path
// relative to the music folder, leaving the original in place.
func TestStageMP3PreservesLibraryLayout(t *testing.T) {
	syncDir, musicDir, _ := setupSweep(t, nil)
	original := filepath.Join(musicDir, "Artist", "Album", "Artist - Song.mp3")
	writeFile(t, original, "normalized audio")

	svc := &service{queue: make(chan uploadRequest, 4), pending: map[string]struct{}{}}
	svc.StageMP3(musicDir, original, "LUFS normalized")

	staged := filepath.Join(syncDir, "mp3", "Artist", "Album", "Artist - Song.mp3")
	if !exists(staged) {
		t.Fatalf("expected a staged copy at %s", staged)
	}
	if !exists(original) {
		t.Error("the library file must stay in place; the row and audit still point at it")
	}
	data, err := os.ReadFile(staged)
	if err != nil || string(data) != "normalized audio" {
		t.Errorf("staged copy has wrong content %q (err %v)", data, err)
	}
	select {
	case req := <-svc.queue:
		if req.path != staged {
			t.Errorf("queued %s, want the staged copy %s", req.path, staged)
		}
	default:
		t.Error("staged file should have been queued for upload")
	}
}

// Bucket object names stay flat: playlists reference tracks by bare filename.
func TestObjectNameStaysFlat(t *testing.T) {
	if got := objectName(filepath.Join("a", "b", "Artist - Song.mp3")); got != "Artist - Song.mp3" {
		t.Errorf("objectName = %q, want the bare filename the playlists reference", got)
	}
}

// The sweep trashes into DeletedPlaylistsPath, the folder the app already uses
// for deleted playlists, rather than a second trash of its own.
func TestSweepTrashUsesDeletedPlaylistsPath(t *testing.T) {
	syncDir, _, logPath := setupSweep(t, nil)
	deleted := filepath.Join(t.TempDir(), "deleted playlists")
	conf.Server.DeletedPlaylistsPath = deleted
	writeFile(t, filepath.Join(syncDir, "art", "cover.jpg"), "not audio")

	if err := newService().Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := uploadedObjects(t, logPath); len(got) != 0 {
		t.Errorf("unsupported file should never reach the bucket, uploaded %v", got)
	}
	if !exists(filepath.Join(deleted, "art", "cover.jpg")) {
		t.Errorf("expected the file under DeletedPlaylistsPath at %s", deleted)
	}
	if exists(filepath.Join(syncDir, "trash")) {
		t.Error("the fallback trash folder should not be created when DeletedPlaylistsPath is set")
	}
}

// Trashing a name that is already taken must not destroy the file already
// there - the trash exists to be looked at later.
func TestSweepTrashDoesNotOverwriteExistingFile(t *testing.T) {
	syncDir, _, _ := setupSweep(t, nil)
	deleted := filepath.Join(t.TempDir(), "deleted playlists")
	conf.Server.DeletedPlaylistsPath = deleted
	writeFile(t, filepath.Join(deleted, "art", "cover.jpg"), "the first one")
	writeFile(t, filepath.Join(syncDir, "art", "cover.jpg"), "the second one")

	if err := newService().Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}

	first, err := os.ReadFile(filepath.Join(deleted, "art", "cover.jpg"))
	if err != nil || string(first) != "the first one" {
		t.Errorf("the file already in the trash was overwritten: %q (err %v)", first, err)
	}
	entries, err := os.ReadDir(filepath.Join(deleted, "art"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("expected both files in the trash, found %d", len(entries))
	}
}

// Missing tags are reported rather than guessed from the old filename.
func TestSweepHoldsMP3WithMissingMetadata(t *testing.T) {
	syncDir, musicDir, logPath := setupSweep(t, nil)
	bad := filepath.Join(syncDir, "mp3", "01 Track.mp3")
	writeFile(t, bad, "processed but missing tags")
	svc := newService()
	svc.readName = func(string) (string, error) { return "", fmt.Errorf("artist/title metadata missing") }
	if err := svc.Sweep(context.Background()); err == nil {
		t.Fatal("expected missing metadata error")
	}
	if got := uploadedObjects(t, logPath); len(got) != 0 {
		t.Fatalf("unexpected uploads: %v", got)
	}
	if !exists(bad) || exists(filepath.Join(musicDir, "01 Track.mp3")) {
		t.Fatal("failed song was not retained")
	}
}

func TestSweepNamesFromMetadataRegardlessOfLegacyEnforcement(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			syncDir, musicDir, logPath := setupSweep(t, nil)
			conf.Server.GCPSync.EnforceNaming = enabled
			writeFile(t, filepath.Join(syncDir, "mp3", "Wrong Artist - Wrong Title.mp3"), "processed")
			svc := newService()
			svc.readName = func(string) (string, error) { return "Tagged Artist - Tagged Title.mp3", nil }
			if err := svc.Sweep(context.Background()); err != nil {
				t.Fatal(err)
			}
			if got := uploadedObjects(t, logPath); len(got) != 1 || got[0] != "Tagged Artist - Tagged Title.mp3" {
				t.Fatalf("uploads: %v", got)
			}
			if !exists(filepath.Join(musicDir, "Tagged Artist - Tagged Title.mp3")) {
				t.Fatal("local filename was not corrected")
			}
		})
	}
}

// The gate is about tracks; playlists have their own naming.
func TestSweepDoesNotHoldPlaylists(t *testing.T) {
	syncDir, _, logPath := setupSweep(t, nil)
	conf.Server.PlaylistsPath = filepath.Join(t.TempDir(), "playlists")
	conf.Server.GCPSync.LastVersionFolder = filepath.Join(t.TempDir(), "last")
	writeFile(t, filepath.Join(syncDir, "MorningMix.m3u"), "Artist - Song.mp3\n")

	if err := newService().Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := uploadedObjects(t, logPath); len(got) != 1 || got[0] != "MorningMix.m3u" {
		t.Errorf("playlists are not subject to the track naming rule, uploaded %v", got)
	}
}

func newService() *service {
	return &service{
		readName: func(path string) (string, error) { return filepath.Base(path), nil },
		queue:    make(chan uploadRequest, 16),
		pending:  map[string]struct{}{},
		restaged: map[string]uploadRequest{},
	}
}

// A LUFS-processed song and a cover-art-processed song are different files and
// must both survive in the staging folder; staging one must not disturb the
// other.
func TestStagedSongsCoexist(t *testing.T) {
	syncDir, musicDir, _ := setupSweep(t, nil)
	lufs := filepath.Join(musicDir, "A", "Artist - Normalized.mp3")
	art := filepath.Join(musicDir, "B", "Artist - Arted.mp3")
	writeFile(t, lufs, "normalized audio")
	writeFile(t, art, "audio with cover art")

	svc := newService()
	svc.StageMP3(musicDir, lufs, "LUFS normalized")
	svc.StageMP3(musicDir, art, "cover art embedded")

	stagedLufs := filepath.Join(syncDir, "mp3", "A", "Artist - Normalized.mp3")
	stagedArt := filepath.Join(syncDir, "mp3", "B", "Artist - Arted.mp3")
	if !exists(stagedLufs) || !exists(stagedArt) {
		t.Fatalf("both staged copies should exist: lufs=%v art=%v", exists(stagedLufs), exists(stagedArt))
	}
	for path, want := range map[string]string{
		stagedLufs: "normalized audio",
		stagedArt:  "audio with cover art",
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Errorf("%s = %q (err %v), want %q", path, got, err, want)
		}
	}
	if len(svc.queue) != 2 {
		t.Errorf("both staged copies should be queued, queue has %d", len(svc.queue))
	}
}

// Staging the same song twice replaces the staged copy with the newer content
// rather than keeping the stale one.
func TestRestagingSameSongOverwrites(t *testing.T) {
	syncDir, musicDir, _ := setupSweep(t, nil)
	song := filepath.Join(musicDir, "A", "Artist - Song.mp3")
	writeFile(t, song, "after LUFS")

	svc := newService()
	svc.StageMP3(musicDir, song, "LUFS normalized")

	// The second pass rewrites the library file, then stages it again.
	writeFile(t, song, "after LUFS and cover art")
	svc.StageMP3(musicDir, song, "cover art embedded")

	staged := filepath.Join(syncDir, "mp3", "A", "Artist - Song.mp3")
	got, err := os.ReadFile(staged)
	if err != nil || string(got) != "after LUFS and cover art" {
		t.Errorf("staged copy = %q (err %v), want the newer content", got, err)
	}
	// Only the copy and its durable library-provenance marker remain.
	entries, err := os.ReadDir(filepath.Dir(staged))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || !exists(libraryCopyMarker(staged)) {
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected the staged copy and its provenance marker, found %v", names)
	}
}

// The bug this guards: a song improved a second time while its first upload is
// still in flight used to have the second request dropped, so the bucket kept
// the older version with nothing left to correct it.
func TestSupersededUploadIsRetriedNotDropped(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	song := filepath.Join(musicDir, "A", "Artist - Song.mp3")
	writeFile(t, song, "after LUFS")

	svc := newService()
	svc.StageMP3(musicDir, song, "LUFS normalized")

	// Take the request off the queue the way the worker does, leaving the path
	// marked pending and mid-upload.
	req := <-svc.queue
	if req.source != song {
		t.Fatalf("request should remember the library file it came from, got %q", req.source)
	}

	// Second pass lands while that upload is in flight.
	writeFile(t, song, "after LUFS and cover art")
	svc.StageMP3(musicDir, song, "cover art embedded")

	if len(svc.queue) != 0 {
		t.Fatal("the superseded request should be held back, not queued twice at once")
	}
	svc.mu.Lock()
	again, ok := svc.restaged[req.path]
	svc.mu.Unlock()
	if !ok {
		t.Fatal("the second pass was dropped - its content would never reach the bucket")
	}
	if again.source != song {
		t.Errorf("re-stage should be redone from the library file, got %q", again.source)
	}
}

// The temporary file copyFile writes must not be picked up by a sweep running
// at the same time, or a half-written copy gets moved out from under it.
func TestSweepIgnoresStagingTempFiles(t *testing.T) {
	syncDir, _, logPath := setupSweep(t, nil)
	tmp := filepath.Join(syncDir, "mp3", ".Artist - Song.mp3.staging-123456")
	writeFile(t, tmp, "half written")

	if err := newService().Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}

	if !exists(tmp) {
		t.Error("an in-flight staging temp file must be left alone by the sweep")
	}
	if got := uploadedObjects(t, logPath); len(got) != 0 {
		t.Errorf("temp file should not be uploaded, uploaded %v", got)
	}
}

// The staging folder is part of the layout, so a sweep that empties it must not
// make it disappear.
func TestSweepKeepsStagingFolder(t *testing.T) {
	syncDir, _, _ := setupSweep(t, []string{"Artist - Song.mp3"})
	staging := filepath.Join(syncDir, "mp3")
	writeFile(t, filepath.Join(staging, "Artist - Song.mp3"), "processed")

	if err := newService().Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}

	if !exists(staging) {
		t.Error("the staging folder should survive a sweep that drains it")
	}
}

// Two different songs that share a filename in different albums must not
// overwrite each other in the staging folder.
//
// The regression: when a track was not textually under MusicFolder - a library
// mounted elsewhere, or a symlinked music folder - the staging path fell back
// to the bare filename, so every staged copy flattened into Sync/mp3. Two songs
// named alike then landed on the same path and the second pass silently
// destroyed the first pass's work before it was ever uploaded.
func TestStagedSongsWithSameFilenameCoexist(t *testing.T) {
	syncDir, musicDir, _ := setupSweep(t, nil)

	// A library that MusicFolder does not textually cover.
	lib := filepath.Join(filepath.Dir(musicDir), "library")
	live := filepath.Join(lib, "Live Album", "Coldplay - Yellow.mp3")
	studio := filepath.Join(lib, "Studio Album", "Coldplay - Yellow.mp3")
	writeFile(t, live, "LIVE version")
	writeFile(t, studio, "STUDIO version")

	svc := newService()
	svc.StageMP3(lib, live, "LUFS normalized")
	svc.StageMP3(lib, studio, "cover art embedded")

	for path, want := range map[string]string{
		filepath.Join(syncDir, "mp3", "Live Album", "Coldplay - Yellow.mp3"):   "LIVE version",
		filepath.Join(syncDir, "mp3", "Studio Album", "Coldplay - Yellow.mp3"): "STUDIO version",
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("expected a staged copy at %s: %v", path, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q - one pass overwrote the other", path, got, want)
		}
	}
	if exists(filepath.Join(syncDir, "mp3", "Coldplay - Yellow.mp3")) {
		t.Error("staging flattened to the bare filename, which is what makes two songs collide")
	}
}

// A music folder reached through a symlink is configured as one string and
// stored in the library as another. Comparing them textually says the track is
// outside the library, which used to trigger the flattening fallback.
func TestStagingResolvesSymlinkedMusicFolder(t *testing.T) {
	syncDir, musicDir, _ := setupSweep(t, nil)

	real := filepath.Join(t.TempDir(), "real-music")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(filepath.Dir(musicDir), "linked-music")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	// Configured via the symlink, stored via the real path.
	conf.Server.MusicFolder = link

	song := filepath.Join(real, "ArtistA", "Album1", "ArtistA - SongA.mp3")
	writeFile(t, song, "audio")

	svc := newService()
	svc.StageMP3("", song, "LUFS normalized")

	staged := filepath.Join(syncDir, "mp3", "ArtistA", "Album1", "ArtistA - SongA.mp3")
	if !exists(staged) {
		t.Errorf("expected the staged copy at %s, keeping its album layout", staged)
	}
	if exists(filepath.Join(syncDir, "mp3", "ArtistA - SongA.mp3")) {
		t.Error("a symlinked music folder flattened the staged copy")
	}
}
