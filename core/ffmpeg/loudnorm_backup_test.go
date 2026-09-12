package ffmpeg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLoudnessBackupRootDefaultsBesideTheLibrary(t *testing.T) {
	got := LoudnessBackupRoot("", filepath.Join("/storage", "music"))
	want := filepath.Join("/storage", LoudnessBackupFolderName)
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	// The whole point: never inside the library it protects
	if strings.HasPrefix(got, filepath.Join("/storage", "music")+string(filepath.Separator)) {
		t.Fatalf("backup root %q must not live inside the library", got)
	}
}

func TestLoudnessBackupRootPrefersConfiguredFolder(t *testing.T) {
	got := LoudnessBackupRoot("/elsewhere/backups", "/storage/music")
	if got != filepath.Clean("/elsewhere/backups") {
		t.Fatalf("got %q", got)
	}
}

func TestLoudnessBackupPathIsKeyedBySongIdentity(t *testing.T) {
	got := LoudnessBackupPath("", "/storage/music", "ab12cd34",
		filepath.Join("/storage", "music", "Artist", "Album", "Song.mp3"))
	want := filepath.Join("/storage", LoudnessBackupFolderName, "ab", "ab12cd34__Song.mp3")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Where a song sits in the library must not decide where its original is kept.
// While it did, moving or renaming a song orphaned its backup, and a song
// arriving at a deleted song's path inherited one that was not its own.
func TestLoudnessBackupPathDoesNotMoveWhenTheSongDoes(t *testing.T) {
	before := LoudnessBackupPath("/backups", "/music", "ab12cd34", "/music/Old Folder/Song.mp3")
	after := LoudnessBackupPath("/backups", "/music", "ab12cd34", "/music/New Folder/Song.mp3")
	if before != after {
		t.Errorf("moving the song moved its backup:\n  %q\n  %q", before, after)
	}
}

func TestLoudnessBackupPathSeparatesTwoSongsSharingAName(t *testing.T) {
	a := LoudnessBackupPath("/backups", "/music", "ab12cd34", "/music/Song.mp3")
	b := LoudnessBackupPath("/backups", "/music", "ef56gh78", "/music/Song.mp3")
	if a == b {
		t.Errorf("two songs share the backup slot %q", a)
	}
}

func TestLoudnessBackupPathNeedsAnIdentity(t *testing.T) {
	if got := LoudnessBackupPath("/backups", "/music", "", "/music/Song.mp3"); got != "" {
		t.Errorf("got %q, want no path without a song id", got)
	}
}

func TestBackupOriginalCreatesCopyOutsideLibrary(t *testing.T) {
	base := t.TempDir()
	lib := filepath.Join(base, "music")
	backups := filepath.Join(base, "backups")
	track := filepath.Join(lib, "Artist", "Album", "Song.mp3")
	writeFile(t, track, "original")

	if err := BackupOriginal(track, 0o644, lib, backups, "song-1"); err != nil {
		t.Fatal(err)
	}

	backup := LoudnessBackupPath(backups, lib, "song-1", track)
	if content := readFile(t, backup); content != "original" {
		t.Fatalf("backup content = %q", content)
	}
	// Nothing should have been written into the music library itself
	if entries, err := os.ReadDir(lib); err != nil {
		t.Fatal(err)
	} else if len(entries) != 1 || entries[0].Name() != "Artist" {
		t.Fatalf("library was polluted: %v", entries)
	}
}

func TestBackupOriginalNeverOverwritesAnExistingOriginal(t *testing.T) {
	base := t.TempDir()
	lib := filepath.Join(base, "music")
	backups := filepath.Join(base, "backups")
	track := filepath.Join(lib, "Song.mp3")
	writeFile(t, track, "already normalized")
	writeFile(t, filepath.Join(backups, "Song.mp3"), "the true original")

	if err := BackupOriginal(track, 0o644, lib, backups, "song-1"); err != nil {
		t.Fatal(err)
	}

	if content := readFile(t, filepath.Join(backups, "Song.mp3")); content != "the true original" {
		t.Fatalf("existing backup was overwritten: got %q", content)
	}
}

func TestBackupOriginalFailsWithNoLocation(t *testing.T) {
	dir := t.TempDir()
	track := filepath.Join(dir, "Song.mp3")
	writeFile(t, track, "original")

	if err := BackupOriginal(track, 0o644, "", "", "song-1"); err == nil {
		t.Fatal("expected an error when no backup location is configured")
	}
}

func TestBackupSurvivesRenamingAndNeverStoresAProcessedReplacement(t *testing.T) {
	library, backups := t.TempDir(), t.TempDir()
	track := filepath.Join(library, "Old name.mp3")
	renamed := filepath.Join(library, "New name.mp3")
	writeFile(t, track, "first untouched original")
	if err := BackupOriginal(track, 0o644, library, backups, "song-1"); err != nil {
		t.Fatal(err)
	}
	stored := FindLoudnessBackup(backups, library, "song-1", track)
	writeFile(t, track, "processed once")
	if err := os.Rename(track, renamed); err != nil {
		t.Fatal(err)
	}
	if got := FindLoudnessBackup(backups, library, "song-1", renamed); got != stored {
		t.Fatalf("renaming lost the original: got %q, want %q", got, stored)
	}
	if err := BackupOriginal(renamed, 0o644, library, backups, "song-1"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, renamed, "processed twice")
	if err := RestoreOriginal(renamed, library, backups, "song-1"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, renamed); got != "first untouched original" {
		t.Fatalf("restored the wrong generation: %q", got)
	}
	entries, err := os.ReadDir(filepath.Dir(stored))
	if err != nil || len(entries) != 1 {
		t.Fatalf("a second original was created: %v, %v", entries, err)
	}
}

func TestAmbiguousOriginalsCannotBeOverwrittenOrRestored(t *testing.T) {
	library, backups := t.TempDir(), t.TempDir()
	track := filepath.Join(library, "Current.mp3")
	writeFile(t, track, "current audio")
	for _, name := range []string{"First.mp3", "Second.mp3"} {
		writeFile(t, LoudnessBackupPath(backups, library, "song-1", name), name)
	}
	if err := BackupOriginal(track, 0o644, library, backups, "song-1"); err == nil {
		t.Fatal("must not create another original when existing backups are ambiguous")
	}
	if err := RestoreOriginal(track, library, backups, "song-1"); err == nil {
		t.Fatal("must not choose an arbitrary original")
	}
	if got := readFile(t, track); got != "current audio" {
		t.Fatalf("changed audio despite ambiguous backups: %q", got)
	}
}

func TestFindLoudnessBackup(t *testing.T) {
	base := t.TempDir()
	lib := filepath.Join(base, "music")
	backups := filepath.Join(base, "backups")
	track := filepath.Join(lib, "Song.mp3")
	writeFile(t, track, "current")

	if got := FindLoudnessBackup(backups, lib, "song-1", track); got != "" {
		t.Fatalf("expected no backup, got %q", got)
	}

	backup := LoudnessBackupPath(backups, lib, "song-1", track)
	writeFile(t, backup, "original")
	if got := FindLoudnessBackup(backups, lib, "song-1", track); got != backup {
		t.Fatalf("expected %q, got %q", backup, got)
	}
}

// Originals stored before backups were keyed by identity still have to be
// found, or an upgrade would silently strand every one of them.
func TestFindLoudnessBackupStillFindsOnesStoredByAnEarlierVersion(t *testing.T) {
	base := t.TempDir()
	lib := filepath.Join(base, "music")
	backups := filepath.Join(base, "backups")
	track := filepath.Join(lib, "Artist", "Song.mp3")
	writeFile(t, track, "current")

	legacy := LegacyLoudnessBackupPath(backups, lib, track)
	writeFile(t, legacy, "original")

	if got := FindLoudnessBackup(backups, lib, "song-1", track); got != legacy {
		t.Fatalf("expected the legacy backup %q, got %q", legacy, got)
	}
}

// The backup is the only surviving original once a track is rewritten, and the
// "does it exist" check treats any file at that path as the original. So a
// failed copy must leave nothing behind rather than a partial file.
func TestBackupCopyLeavesNoPartialFile(t *testing.T) {
	base := t.TempDir()
	lib := filepath.Join(base, "music")
	backups := filepath.Join(base, "backups")
	track := filepath.Join(lib, "Song.mp3")
	writeFile(t, track, "the original audio")

	// A directory as source makes the read fail part-way through the copy.
	badSrc := filepath.Join(lib, "notafile")
	if err := os.MkdirAll(badSrc, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backups, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(backups, "Song.mp3")
	if err := copyFileWithMode(badSrc, dst, 0o644); err == nil {
		t.Fatal("expected the copy to fail")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatal("a failed copy left a file at the backup path: the next run would trust it as the original")
	}
	entries, err := os.ReadDir(backups)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary files left behind: %v", entries)
	}
}

func TestBackupCopyIsCompleteAndClean(t *testing.T) {
	base := t.TempDir()
	lib := filepath.Join(base, "music")
	backups := filepath.Join(base, "backups")
	track := filepath.Join(lib, "Song.mp3")
	writeFile(t, track, "the original audio")

	if err := BackupOriginal(track, 0o644, lib, backups, "song-1"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, LoudnessBackupPath(backups, lib, "song-1", track)); got != "the original audio" {
		t.Fatalf("backup content = %q", got)
	}
	entries, err := os.ReadDir(filepath.Dir(LoudnessBackupPath(backups, lib, "song-1", track)))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected only the backup, found %d entries (stray temp file?)", len(entries))
	}
}
