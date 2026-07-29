package ffmpeg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestoreOriginalPutsTheStoredFileBack(t *testing.T) {
	library := t.TempDir()
	backupFolder := t.TempDir()
	track := filepath.Join(library, "Artist", "Album", "Song.mp3")

	writeFile(t, track, "the original recording")
	if err := BackupOriginal(track, 0o644, library, backupFolder, "song-1"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, track, "normalized")

	if err := RestoreOriginal(track, library, backupFolder, "song-1"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, track); got != "the original recording" {
		t.Errorf("track = %q, want the original back", got)
	}
}

// The backup is the one irreplaceable copy, so undoing a change must not spend
// it: a track can be re-processed and restored again.
func TestRestoreOriginalKeepsTheStoredCopy(t *testing.T) {
	library := t.TempDir()
	backupFolder := t.TempDir()
	track := filepath.Join(library, "Song.mp3")

	writeFile(t, track, "the original recording")
	if err := BackupOriginal(track, 0o644, library, backupFolder, "song-1"); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		writeFile(t, track, "normalized again")
		if err := RestoreOriginal(track, library, backupFolder, "song-1"); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, track); got != "the original recording" {
			t.Fatalf("track = %q, want the original back", got)
		}
	}
	if FindLoudnessBackup(backupFolder, library, "song-1", track) == "" {
		t.Error("the stored original was consumed by restoring")
	}
}

func TestRestoreOriginalKeepsTheTracksPermissions(t *testing.T) {
	library := t.TempDir()
	backupFolder := t.TempDir()
	track := filepath.Join(library, "Song.mp3")

	writeFile(t, track, "the original recording")
	if err := BackupOriginal(track, 0o600, library, backupFolder, "song-1"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, track, "normalized")
	if err := os.Chmod(track, 0o640); err != nil {
		t.Fatal(err)
	}

	if err := RestoreOriginal(track, library, backupFolder, "song-1"); err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(track)
	if err != nil {
		t.Fatal(err)
	}
	// The file keeps its place in the library; only its audio comes back.
	if stat.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v, want 0640", stat.Mode().Perm())
	}
}

func TestRestoreOriginalReportsWhenThereIsNothingStored(t *testing.T) {
	library := t.TempDir()
	track := filepath.Join(library, "Song.mp3")
	writeFile(t, track, "never processed")

	err := RestoreOriginal(track, library, t.TempDir(), "song-1")
	if err == nil {
		t.Fatal("restoring without a stored original should fail")
	}
	if !strings.Contains(err.Error(), "no stored original") {
		t.Errorf("error = %q, want it to say there is nothing to restore", err)
	}
	// The song still has to be there afterwards.
	if got := readFile(t, track); got != "never processed" {
		t.Errorf("track = %q, want it left alone", got)
	}
}
