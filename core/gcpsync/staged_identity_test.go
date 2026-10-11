package gcpsync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/navidrome/navidrome/conf"
)

func TestStagedIdentityKeepsLatestRenamedEditAcrossRestart(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	conf.Server.GCPSync.AutoUpload = false
	oldSong := filepath.Join(musicDir, "Old Album", "incoming.mp3")
	newSong := filepath.Join(musicDir, "New Album", "Artist - Song.mp3")
	writeFile(t, oldSong, "normalized audio")
	if err := newService().StageMP3(musicDir, oldSong, "LUFS normalized", "stable-song-id"); err != nil {
		t.Fatal(err)
	}
	oldStage := StagingPath(musicDir, oldSong)
	assertContent(t, oldStage, "normalized audio")
	// No in-memory state or old source filename is available in a fresh process.
	if err := os.MkdirAll(filepath.Dir(newSong), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(oldSong, newSong); err != nil {
		t.Fatal(err)
	}
	writeFile(t, newSong, "normalized audio with new cover art")
	if err := newService().StageMP3(musicDir, newSong, "cover art embedded", "stable-song-id"); err != nil {
		t.Fatal(err)
	}
	assertContent(t, StagingPath(musicDir, newSong), "normalized audio with new cover art")
	assertContent(t, newSong, "normalized audio with new cover art")
	if exists(oldStage) || exists(libraryCopyMarker(oldStage)) {
		t.Fatal("old pending filename survived the same song's newer edit")
	}
	if !exists(libraryCopyMarker(StagingPath(musicDir, newSong))) {
		t.Fatal("latest pending edit lost its durable identity marker")
	}
}

func TestStagedIdentityRetainsDifferentSongIDsWithIdenticalBytes(t *testing.T) {
	_, musicDir, logPath := setupSweep(t, nil)
	conf.Server.GCPSync.AutoUpload = false
	svc := newService()
	first := filepath.Join(musicDir, "First Artist - Song.mp3")
	second := filepath.Join(musicDir, "Second Artist - Song.mp3")
	for _, song := range []struct{ path, id string }{{first, "first-id"}, {second, "second-id"}} {
		writeFile(t, song.path, "same audio bytes")
		if err := svc.StageMP3(musicDir, song.path, "LUFS normalized", song.id); err != nil {
			t.Fatal(err)
		}
	}
	for _, song := range []string{first, second} {
		assertContent(t, StagingPath(musicDir, song), "same audio bytes")
	}
	// A subsequent sweep must preserve explicit distinct identities as well.
	restarted := newService()
	restarted.readName = func(string) (string, error) { return "", errors.New("metadata unavailable") }
	if err := restarted.Sweep(context.Background()); err == nil {
		t.Fatal("expected blocked upload")
	}
	for _, song := range []string{first, second} {
		assertContent(t, StagingPath(musicDir, song), "same audio bytes")
		if !exists(libraryCopyMarker(StagingPath(musicDir, song))) {
			t.Fatal("distinct pending song lost its marker")
		}
	}
	if calls := gcloudCalls(t, logPath); len(calls) != 0 {
		t.Fatalf("blocked songs were uploaded: %v", calls)
	}
}

func TestStagedIdentityCoalescesByteIdenticalLegacyAliasKeepingFormattedName(t *testing.T) {
	syncDir, musicDir, _ := setupSweep(t, nil)
	conf.Server.GCPSync.AutoUpload = false
	song := filepath.Join(musicDir, "chand_sifarish_cute.mp3")
	alias := filepath.Join(syncDir, "mp3", "[Unknown Artist] - chand_sifarish_cute.mp3")
	writeFile(t, alias, "same untagged audio")
	writeFile(t, song, "same untagged audio")
	if err := newService().StageMP3(musicDir, song, "LUFS normalized", "song-id"); err != nil {
		t.Fatal(err)
	}
	assertContent(t, alias, "same untagged audio")
	assertContent(t, song, "same untagged audio")
	assertStagedIdentity(t, alias, "song-id")
	staged := StagingPath(musicDir, song)
	if exists(staged) || exists(libraryCopyMarker(staged)) {
		t.Fatal("raw filename was left as a second pending song instead of keeping the formatted alias")
	}
}

func TestStagedIdentitySweepCoalescesLegacyAliasAndKeepsFormattedCopyBlocked(t *testing.T) {
	syncDir, musicDir, logPath := setupSweep(t, nil)
	conf.Server.GCPSync.AutoUpload = false
	song := filepath.Join(musicDir, "chand_sifarish_cute.mp3")
	writeFile(t, song, "same untagged audio")
	if err := newService().StageMP3(musicDir, song, "LUFS normalized", "song-id"); err != nil {
		t.Fatal(err)
	}
	staged := StagingPath(musicDir, song)
	// Reproduce a duplicate that was already present when the app restarted.
	alias := filepath.Join(syncDir, "mp3", "[Unknown Artist] - chand_sifarish_cute.mp3")
	writeFile(t, alias, "same untagged audio")
	restarted := newService()
	restarted.readName = func(string) (string, error) { return "", errors.New("artist/title metadata missing") }
	if err := restarted.Sweep(context.Background()); err == nil {
		t.Fatal("missing metadata should still block upload")
	}
	assertContent(t, alias, "same untagged audio")
	assertContent(t, song, "same untagged audio")
	assertStagedIdentity(t, alias, "song-id")
	if exists(staged) || exists(libraryCopyMarker(staged)) {
		t.Fatal("raw pending copy survived reconciliation on a blocked sweep")
	}
	if calls := gcloudCalls(t, logPath); len(calls) != 0 {
		t.Fatalf("missing metadata was silently invented for upload: %v", calls)
	}
}

func TestStagedIdentityLaterRawEditKeepsFormattedPendingNameAndLatestBytes(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	conf.Server.GCPSync.AutoUpload = false
	formattedSong := filepath.Join(musicDir, "Artist - Song.mp3")
	writeFile(t, formattedSong, "original normalized recording")
	if err := newService().StageMP3(musicDir, formattedSong, "LUFS normalized", "stable-song-id"); err != nil {
		t.Fatal(err)
	}
	formattedStage := StagingPath(musicDir, formattedSong)
	// The next edit arrives through the song's raw library filename after a
	// restart. Its identity must preserve the formatted pending name, not the
	// older audio that happened to be staged under that name.
	rawSong := filepath.Join(musicDir, "incoming.mp3")
	writeFile(t, rawSong, "new normalized recording with updated cover art")
	if err := newService().StageMP3(musicDir, rawSong, "cover art embedded", "stable-song-id"); err != nil {
		t.Fatal(err)
	}
	assertContent(t, formattedStage, "new normalized recording with updated cover art")
	assertContent(t, rawSong, "new normalized recording with updated cover art")
	assertContent(t, formattedSong, "original normalized recording")
	assertStagedIdentity(t, formattedStage, "stable-song-id")
	rawStage := StagingPath(musicDir, rawSong)
	if exists(rawStage) || exists(libraryCopyMarker(rawStage)) {
		t.Fatal("latest raw edit left a second pending filename")
	}
}

func TestStagedIdentityRestartKeepsFormattedNameWithLatestRawBytes(t *testing.T) {
	syncDir, musicDir, logPath := setupSweep(t, nil)
	conf.Server.GCPSync.AutoUpload = false
	rawSong := filepath.Join(musicDir, "incoming.mp3")
	writeFile(t, rawSong, "latest normalized recording with updated cover art")
	rawStage := StagingPath(musicDir, rawSong)
	formattedStage := filepath.Join(syncDir, "mp3", "Artist - Song.mp3")
	writeFile(t, rawStage, "latest normalized recording with updated cover art")
	writeFile(t, formattedStage, "older normalized recording")
	for _, path := range []string{rawStage, formattedStage} {
		if err := markLibraryCopy(path, "stable-song-id"); err != nil {
			t.Fatal(err)
		}
	}
	older := time.Unix(1_700_000_000, 0)
	newer := older.Add(time.Minute)
	if err := os.Chtimes(formattedStage, older, older); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(rawStage, newer, newer); err != nil {
		t.Fatal(err)
	}
	restarted := newService()
	restarted.readName = func(string) (string, error) { return "", errors.New("artist/title metadata missing") }
	if err := restarted.Sweep(context.Background()); err == nil {
		t.Fatal("formatted filename alone must not bypass missing metadata")
	}
	assertContent(t, formattedStage, "latest normalized recording with updated cover art")
	assertContent(t, rawSong, "latest normalized recording with updated cover art")
	assertStagedIdentity(t, formattedStage, "stable-song-id")
	if exists(rawStage) || exists(libraryCopyMarker(rawStage)) {
		t.Fatal("raw copy or marker survived reconciliation after restart")
	}
	if calls := gcloudCalls(t, logPath); len(calls) != 0 {
		t.Fatalf("formatted filename supplied missing upload metadata: %v", calls)
	}
}

func TestStagedIdentityCurrentFormattedSourceNameSupersedesOldFormattedName(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	conf.Server.GCPSync.AutoUpload = false
	oldSong := filepath.Join(musicDir, "Old Artist - Old Song.mp3")
	newSong := filepath.Join(musicDir, "New Artist - New Song.mp3")
	writeFile(t, oldSong, "original normalized recording")
	if err := newService().StageMP3(musicDir, oldSong, "LUFS normalized", "stable-song-id"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, newSong, "new normalized recording with updated metadata")
	if err := newService().StageMP3(musicDir, newSong, "cover art embedded", "stable-song-id"); err != nil {
		t.Fatal(err)
	}
	newStage := StagingPath(musicDir, newSong)
	assertContent(t, newStage, "new normalized recording with updated metadata")
	assertStagedIdentity(t, newStage, "stable-song-id")
	oldStage := StagingPath(musicDir, oldSong)
	if exists(oldStage) || exists(libraryCopyMarker(oldStage)) {
		t.Fatal("old formatted name survived a current formatted source name")
	}
}

func TestStagedIdentityMalformedUnrelatedMarkerCannotPreventPersistence(t *testing.T) {
	syncDir, musicDir, _ := setupSweep(t, nil)
	conf.Server.GCPSync.AutoUpload = false
	unrelated := filepath.Join(syncDir, "mp3", "Other Artist - Other Song.mp3")
	writeFile(t, unrelated, "unrelated song awaiting upload")
	malformedMarker := "{incomplete previous marker"
	writeFile(t, libraryCopyMarker(unrelated), malformedMarker)
	song := filepath.Join(musicDir, "incoming.mp3")
	writeFile(t, song, "latest normalized recording with updated cover art")
	// Cleanup can report the malformed unrelated marker, but the newest song
	// must already be durable before that error is returned.
	_ = newService().StageMP3(musicDir, song, "LUFS normalized", "new-song-id")
	staged := StagingPath(musicDir, song)
	assertContent(t, staged, "latest normalized recording with updated cover art")
	assertStagedIdentity(t, staged, "new-song-id")
	assertContent(t, song, "latest normalized recording with updated cover art")
	assertContent(t, unrelated, "unrelated song awaiting upload")
	assertContent(t, libraryCopyMarker(unrelated), malformedMarker)
}

func TestStagedIdentityUnreadableUnrelatedFolderCannotPreventPersistence(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires directory permission enforcement for a non-root Unix user")
	}
	syncDir, musicDir, _ := setupSweep(t, nil)
	conf.Server.GCPSync.AutoUpload = false
	blockedDir := filepath.Join(syncDir, "mp3", "inaccessible-folder")
	unrelated := filepath.Join(blockedDir, "Other Artist - Other Song.mp3")
	writeFile(t, unrelated, "unrelated song awaiting upload")
	if err := markLibraryCopy(unrelated, "other-song-id"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blockedDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blockedDir, 0o755) })
	song := filepath.Join(musicDir, "incoming.mp3")
	writeFile(t, song, "latest normalized recording with updated cover art")
	if err := newService().StageMP3(musicDir, song, "LUFS normalized", "new-song-id"); err == nil {
		t.Fatal("expected the inaccessible pending folder to report a scan or cleanup error")
	}
	staged := StagingPath(musicDir, song)
	assertContent(t, staged, "latest normalized recording with updated cover art")
	assertStagedIdentity(t, staged, "new-song-id")
	assertContent(t, song, "latest normalized recording with updated cover art")
	if err := os.Chmod(blockedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	assertContent(t, unrelated, "unrelated song awaiting upload")
	assertStagedIdentity(t, unrelated, "other-song-id")
}

func assertStagedIdentity(t *testing.T, path, wantID string) {
	t.Helper()
	owner, err := readStagedOwner(path)
	if err != nil {
		t.Fatal(err)
	}
	if !owner.owned || owner.MediaFileID != wantID {
		t.Fatalf("pending copy %q lost its app ownership or song identity: %+v", path, owner)
	}
}

func TestStagedIdentityKeepsDifferentRecordingsWithSimilarNames(t *testing.T) {
	syncDir, musicDir, _ := setupSweep(t, nil)
	conf.Server.GCPSync.AutoUpload = false
	song := filepath.Join(musicDir, "Song.mp3")
	alias := filepath.Join(syncDir, "mp3", "[Unknown Artist] - Song.mp3")
	writeFile(t, alias, "live recording")
	writeFile(t, song, "studio recording")
	if err := newService().StageMP3(musicDir, song, "LUFS normalized", "studio-id"); err != nil {
		t.Fatal(err)
	}
	assertContent(t, alias, "live recording")
	assertContent(t, StagingPath(musicDir, song), "studio recording")
	restarted := newService()
	restarted.readName = func(string) (string, error) { return "", errors.New("artist/title metadata missing") }
	if err := restarted.Sweep(context.Background()); err == nil {
		t.Fatal("expected missing metadata to block upload")
	}
	assertContent(t, alias, "live recording")
	assertContent(t, StagingPath(musicDir, song), "studio recording")
}

func TestStagedIdentityDryRunLeavesPreviousEditAndLegacyAlias(t *testing.T) {
	syncDir, musicDir, logPath := setupSweep(t, nil)
	conf.Server.GCPSync.AutoUpload = false
	oldSong := filepath.Join(musicDir, "incoming.mp3")
	newSong := filepath.Join(musicDir, "Artist - Song.mp3")
	writeFile(t, oldSong, "old pending audio")
	if err := newService().StageMP3(musicDir, oldSong, "LUFS normalized", "song-id"); err != nil {
		t.Fatal(err)
	}
	oldStage := StagingPath(musicDir, oldSong)
	alias := filepath.Join(syncDir, "mp3", "[Unknown Artist] - incoming.mp3")
	writeFile(t, alias, "old pending audio")
	writeFile(t, newSong, "new pending audio")
	conf.Server.GCPSync.DryRun = true
	if err := newService().StageMP3(musicDir, newSong, "cover art embedded", "song-id"); err != nil {
		t.Fatal(err)
	}
	if err := newService().Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertContent(t, oldStage, "old pending audio")
	assertContent(t, alias, "old pending audio")
	assertContent(t, newSong, "new pending audio")
	if !exists(libraryCopyMarker(oldStage)) || exists(StagingPath(musicDir, newSong)) {
		t.Fatal("dry run changed staged copies or identity")
	}
	if calls := gcloudCalls(t, logPath); len(calls) != 0 {
		t.Fatalf("dry run uploaded a pending song: %v", calls)
	}
}

func TestStagedIdentityFailedReplacementRetainsPreviousPendingCopy(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	conf.Server.GCPSync.AutoUpload = false
	oldSong := filepath.Join(musicDir, "incoming.mp3")
	newSong := filepath.Join(musicDir, "Artist - Song.mp3")
	writeFile(t, oldSong, "last complete pending version")
	if err := newService().StageMP3(musicDir, oldSong, "LUFS normalized", "song-id"); err != nil {
		t.Fatal(err)
	}
	// A missing replacement source must fail before the known-good copy is retired.
	if err := newService().StageMP3(musicDir, newSong, "cover art embedded", "song-id"); err == nil {
		t.Fatal("expected missing replacement source to fail staging")
	}
	assertContent(t, StagingPath(musicDir, oldSong), "last complete pending version")
	assertContent(t, oldSong, "last complete pending version")
	if !exists(libraryCopyMarker(StagingPath(musicDir, oldSong))) {
		t.Fatal("failed replacement lost the previous copy's marker")
	}
	if exists(StagingPath(musicDir, newSong)) {
		t.Fatal("failed staging published an incomplete replacement")
	}
}

func TestStagedIdentityConcurrentRenamedEditsKeepOneCompleteCopy(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	conf.Server.GCPSync.AutoUpload = false
	first := filepath.Join(musicDir, "First Name.mp3")
	second := filepath.Join(musicDir, "Second Name.mp3")
	writeFile(t, first, "first complete edit")
	writeFile(t, second, "second complete edit")
	start := make(chan struct{})
	done := make(chan error, 2)
	for _, song := range []string{first, second} {
		go func(song string) {
			<-start
			done <- newService().StageMP3(musicDir, song, "updated", "same-song-id")
		}(song)
	}
	close(start)
	for range 2 {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent renamed staging deadlocked")
		}
	}
	var queued []string
	entries, err := os.ReadDir(stagingRoot())
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".mp3") && !strings.HasPrefix(entry.Name(), ".") {
			queued = append(queued, filepath.Join(stagingRoot(), entry.Name()))
		}
	}
	if len(queued) != 1 {
		t.Fatalf("concurrent edits left %d pending copies, want one: %v", len(queued), queued)
	}
	want := "first complete edit"
	if queued[0] == StagingPath(musicDir, second) {
		want = "second complete edit"
	}
	assertContent(t, queued[0], want)
	if !exists(libraryCopyMarker(queued[0])) {
		t.Fatal("surviving concurrent edit lost its durable marker")
	}
}

func TestStagedIdentityRefusesDifferentSongAtSameDestination(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	conf.Server.GCPSync.AutoUpload = false
	otherLibrary := t.TempDir()
	first := filepath.Join(musicDir, "Artist - Song.mp3")
	second := filepath.Join(otherLibrary, "Artist - Song.mp3")
	writeFile(t, first, "first song's pending recording")
	writeFile(t, second, "different song's recording")
	if err := newService().StageMP3(musicDir, first, "LUFS normalized", "first-song-id"); err != nil {
		t.Fatal(err)
	}
	staged := StagingPath(musicDir, first)
	if alternate := StagingPath(otherLibrary, second); alternate != staged {
		t.Fatalf("test did not create a staging destination collision: %s != %s", alternate, staged)
	}
	originalMarker, err := os.ReadFile(libraryCopyMarker(staged))
	if err != nil {
		t.Fatal(err)
	}
	if err := newService().StageMP3(otherLibrary, second, "LUFS normalized", "different-song-id"); err == nil {
		t.Fatal("different song identity was permitted to overwrite the same pending destination")
	}
	assertContent(t, staged, "first song's pending recording")
	assertContent(t, libraryCopyMarker(staged), string(originalMarker))
	assertContent(t, first, "first song's pending recording")
	assertContent(t, second, "different song's recording")
}

func TestStagedIdentityRenameKeepsNewerDestinationForSameSong(t *testing.T) {
	syncDir, _, _ := setupSweep(t, nil)
	oldPath := filepath.Join(syncDir, "mp3", "incoming.mp3")
	newPath := filepath.Join(syncDir, "mp3", "Artist - Song.mp3")
	writeFile(t, oldPath, "older normalized audio")
	writeFile(t, newPath, "newer normalized audio with cover art")
	for _, path := range []string{oldPath, newPath} {
		if err := markLibraryCopy(path, "same-song-id"); err != nil {
			t.Fatal(err)
		}
	}
	older := time.Unix(1_700_000_000, 0)
	newer := older.Add(time.Minute)
	if err := os.Chtimes(oldPath, older, older); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newPath, newer, newer); err != nil {
		t.Fatal(err)
	}
	newMarker, err := os.ReadFile(libraryCopyMarker(newPath))
	if err != nil {
		t.Fatal(err)
	}
	if err := RenameStagedCopy(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	assertContent(t, newPath, "newer normalized audio with cover art")
	assertContent(t, libraryCopyMarker(newPath), string(newMarker))
	if exists(oldPath) || exists(libraryCopyMarker(oldPath)) {
		t.Fatal("older same-song pending copy survived reconciliation during rename")
	}
}

func TestStagedIdentityRenameWithMissingSourceKeepsDestinationAndMarker(t *testing.T) {
	syncDir, _, _ := setupSweep(t, nil)
	oldPath := filepath.Join(syncDir, "mp3", "missing.mp3")
	newPath := filepath.Join(syncDir, "mp3", "Artist - Song.mp3")
	writeFile(t, newPath, "only complete pending version")
	// The old marker survived an interruption but its payload is missing.
	for _, path := range []string{oldPath, newPath} {
		if err := markLibraryCopy(path, "same-song-id"); err != nil {
			t.Fatal(err)
		}
	}
	newMarker, err := os.ReadFile(libraryCopyMarker(newPath))
	if err != nil {
		t.Fatal(err)
	}
	if err := RenameStagedCopy(oldPath, newPath); err == nil {
		t.Fatal("expected rename with missing source payload to fail")
	}
	assertContent(t, newPath, "only complete pending version")
	assertContent(t, libraryCopyMarker(newPath), string(newMarker))
}

func TestStagedIdentityRenameRefusesDifferentSongDestination(t *testing.T) {
	syncDir, _, _ := setupSweep(t, nil)
	oldPath := filepath.Join(syncDir, "mp3", "incoming.mp3")
	newPath := filepath.Join(syncDir, "mp3", "Artist - Song.mp3")
	writeFile(t, oldPath, "first song's recording")
	writeFile(t, newPath, "different song's pending recording")
	if err := markLibraryCopy(oldPath, "first-song-id"); err != nil {
		t.Fatal(err)
	}
	if err := markLibraryCopy(newPath, "second-song-id"); err != nil {
		t.Fatal(err)
	}
	oldMarker, err := os.ReadFile(libraryCopyMarker(oldPath))
	if err != nil {
		t.Fatal(err)
	}
	newMarker, err := os.ReadFile(libraryCopyMarker(newPath))
	if err != nil {
		t.Fatal(err)
	}
	if err := RenameStagedCopy(oldPath, newPath); err == nil {
		t.Fatal("different song identity was permitted to overwrite a rename destination")
	}
	assertContent(t, oldPath, "first song's recording")
	assertContent(t, newPath, "different song's pending recording")
	assertContent(t, libraryCopyMarker(oldPath), string(oldMarker))
	assertContent(t, libraryCopyMarker(newPath), string(newMarker))
}

func TestStagedIdentityRefusesUnmarkedImportedDestinationWithDifferentBytes(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	conf.Server.GCPSync.AutoUpload = false
	song := filepath.Join(musicDir, "Artist - Song.mp3")
	staged := StagingPath(musicDir, song)
	writeFile(t, staged, "unmarked imported recording awaiting upload")
	writeFile(t, song, "different library song's normalized recording")
	if err := newService().StageMP3(musicDir, song, "LUFS normalized", "library-song-id"); err == nil {
		t.Fatal("staging overwrote unrelated unmarked imported pending audio")
	}
	assertContent(t, staged, "unmarked imported recording awaiting upload")
	assertContent(t, song, "different library song's normalized recording")
	if exists(libraryCopyMarker(staged)) {
		t.Fatal("refused staging changed the imported copy's ownership")
	}
}

func TestStagedIdentityCleanupFailureHoldsSongWithoutBlockingPlaylist(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires directory permission enforcement for a non-root Unix user")
	}
	syncDir, musicDir, _ := setupSweep(t, nil)
	bucket := captureBucket(t)
	conf.Server.GCPSync.AutoUpload = false
	conf.Server.PlaylistsPath = t.TempDir()
	conf.Server.GCPSync.LastVersionFolder = t.TempDir()
	oldSong := filepath.Join(musicDir, "Old Album", "incoming.mp3")
	newSong := filepath.Join(musicDir, "New Album", "Artist - Song.mp3")
	writeFile(t, oldSong, "older normalized recording")
	if err := newService().StageMP3(musicDir, oldSong, "LUFS normalized", "same-song-id"); err != nil {
		t.Fatal(err)
	}
	oldStage := StagingPath(musicDir, oldSong)
	oldDir := filepath.Dir(oldStage)
	older := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(oldStage, older, older); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(oldDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(oldDir, 0o755) })
	writeFile(t, newSong, "latest normalized recording with cover art")
	svc := newService()
	if err := svc.StageMP3(musicDir, newSong, "cover art embedded", "same-song-id"); err == nil {
		t.Fatal("expected obsolete-copy cleanup to fail in a read-only directory")
	}
	newStage := StagingPath(musicDir, newSong)
	assertContent(t, oldStage, "older normalized recording")
	assertContent(t, newStage, "latest normalized recording with cover art")
	svc.readName = func(string) (string, error) { return "Artist - Song.mp3", nil }
	// Neither a stale automatic queue entry nor the latest one may upload while
	// unresolved same-song aliases could subsequently overwrite one another.
	for _, path := range []string{oldStage, newStage} {
		svc.processUpload(context.Background(), uploadRequest{path: path})
	}
	object := filepath.Join(bucket, "Artist - Song.mp3")
	if exists(object) {
		t.Fatal("automatic upload sent a song with unresolved duplicate identity")
	}
	playlist := "Unrelated Playlist.m3u"
	playlistContent := "#EXTM3U\nUnrelated Artist - Song.mp3\n"
	writeFile(t, filepath.Join(syncDir, playlist), playlistContent)
	if err := svc.Sweep(context.Background()); err == nil {
		t.Fatal("expected the sweep to report unresolved song cleanup failure")
	}
	if exists(object) {
		t.Fatal("sweep sent an obsolete or unresolved latest song copy")
	}
	assertContent(t, filepath.Join(bucket, playlist), playlistContent)
	assertContent(t, filepath.Join(conf.Server.PlaylistsPath, playlist), playlistContent)
	assertContent(t, oldStage, "older normalized recording")
	assertContent(t, newStage, "latest normalized recording with cover art")
	for _, path := range []string{oldStage, newStage} {
		if !exists(libraryCopyMarker(path)) {
			t.Fatal("blocked duplicate identity lost a pending marker")
		}
	}
	if err := os.Chmod(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := svc.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertContent(t, object, "latest normalized recording with cover art")
	assertContent(t, newSong, "latest normalized recording with cover art")
	for _, path := range []string{oldStage, newStage} {
		if exists(path) || exists(libraryCopyMarker(path)) {
			t.Fatal("successful cleanup/retry left pending aliases or markers")
		}
	}
}

func TestStagedIdentityRenameDuringUploadKeepsLatestPendingCopy(t *testing.T) {
	_, musicDir, _ := setupSweep(t, nil)
	bucket := captureBucket(t)
	oldSong := filepath.Join(musicDir, "incoming.mp3")
	newSong := filepath.Join(musicDir, "Artist - Song.mp3")
	writeFile(t, oldSong, "older normalized recording")
	oldService := newService()
	oldService.readName = func(string) (string, error) { return "Artist - Song.mp3", nil }
	if err := oldService.StageMP3(musicDir, oldSong, "LUFS normalized", "same-song-id"); err != nil {
		t.Fatal(err)
	}
	oldStage := StagingPath(musicDir, oldSong)
	pause := filepath.Join(bucket, "pause")
	writeFile(t, pause, "")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	request := <-oldService.queue
	go func() { defer close(done); oldService.processUpload(ctx, request) }()
	t.Cleanup(func() {
		cancel()
		_ = os.Remove(pause)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("paused upload did not stop during cleanup")
		}
	})
	waitSyncFile(t, filepath.Join(bucket, "started"))
	if err := os.Rename(oldSong, newSong); err != nil {
		t.Fatal(err)
	}
	writeFile(t, newSong, "latest normalized recording with cover art")
	newService := newService()
	newService.readName = func(string) (string, error) { return "Artist - Song.mp3", nil }
	if err := newService.StageMP3(musicDir, newSong, "cover art embedded", "same-song-id"); err != nil {
		t.Fatal(err)
	}
	newStage := StagingPath(musicDir, newSong)
	latestMarker, err := os.ReadFile(libraryCopyMarker(newStage))
	if err != nil {
		t.Fatal(err)
	}
	assertContent(t, newStage, "latest normalized recording with cover art")
	if exists(oldStage) || exists(libraryCopyMarker(oldStage)) {
		t.Fatal("old staged alias survived a newer renamed edit during transfer")
	}
	if err := os.Remove(pause); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("old transfer did not finish after its pause was released")
	}
	assertContent(t, newStage, "latest normalized recording with cover art")
	assertContent(t, libraryCopyMarker(newStage), string(latestMarker))
	assertContent(t, newSong, "latest normalized recording with cover art")
	object := filepath.Join(bucket, "Artist - Song.mp3")
	assertContent(t, object, "older normalized recording")
	if err := newService.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertContent(t, object, "latest normalized recording with cover art")
	assertContent(t, newSong, "latest normalized recording with cover art")
	for _, path := range []string{oldStage, newStage} {
		if exists(path) || exists(libraryCopyMarker(path)) {
			t.Fatal("successful final upload left pending staged copies or markers")
		}
	}
	entries, err := os.ReadDir(bucket)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.EqualFold(filepath.Ext(entry.Name()), ".mp3") && entry.Name() != "Artist - Song.mp3" {
			t.Fatalf("transfer created a second object using an obsolete filename: %s", entry.Name())
		}
	}
}
