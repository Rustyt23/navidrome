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

func TestLoudnessBackupPathMirrorsLibraryStructure(t *testing.T) {
	got := LoudnessBackupPath("", "/storage/music", filepath.Join("/storage", "music", "Artist", "Album", "Song.mp3"))
	want := filepath.Join("/storage", LoudnessBackupFolderName, "Artist", "Album", "Song.mp3")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLoudnessBackupPathFallsBackToFilenameOutsideLibrary(t *testing.T) {
	got := LoudnessBackupPath("/backups", "/storage/music", "/somewhere-else/Song.mp3")
	want := filepath.Join("/backups", "Song.mp3")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBackupOriginalCreatesMirroredCopyOutsideLibrary(t *testing.T) {
	base := t.TempDir()
	lib := filepath.Join(base, "music")
	backups := filepath.Join(base, "backups")
	track := filepath.Join(lib, "Artist", "Album", "Song.mp3")
	writeFile(t, track, "original")

	if err := BackupOriginal(track, 0o644, lib, backups); err != nil {
		t.Fatal(err)
	}

	backup := filepath.Join(backups, "Artist", "Album", "Song.mp3")
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

	if err := BackupOriginal(track, 0o644, lib, backups); err != nil {
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

	if err := BackupOriginal(track, 0o644, "", ""); err == nil {
		t.Fatal("expected an error when no backup location is configured")
	}
}

func TestFindLoudnessBackup(t *testing.T) {
	base := t.TempDir()
	lib := filepath.Join(base, "music")
	backups := filepath.Join(base, "backups")
	track := filepath.Join(lib, "Song.mp3")
	writeFile(t, track, "current")

	if got := FindLoudnessBackup(backups, lib, track); got != "" {
		t.Fatalf("expected no backup, got %q", got)
	}

	backup := filepath.Join(backups, "Song.mp3")
	writeFile(t, backup, "original")
	if got := FindLoudnessBackup(backups, lib, track); got != backup {
		t.Fatalf("expected %q, got %q", backup, got)
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

	if err := BackupOriginal(track, 0o644, lib, backups); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(backups, "Song.mp3")); got != "the original audio" {
		t.Fatalf("backup content = %q", got)
	}
	entries, err := os.ReadDir(backups)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected only the backup, found %d entries (stray temp file?)", len(entries))
	}
}
