package ffmpeg

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LoudnessBackupFolderName is the folder that holds untouched copies of every
// song loudness normalization rewrites. It lives outside the music library:
// backups must not sit inside the thing they protect against, where a cleanup,
// re-organization or rescan of the library could take them with it.
const LoudnessBackupFolderName = "original_backup before normalisation"

// LoudnessBackupRoot resolves where originals are kept. A configured folder
// wins; otherwise backups go next to the library rather than inside it, so
// "<...>/music" backs up to "<...>/original_backup before normalisation".
func LoudnessBackupRoot(backupFolder, libraryPath string) string {
	if folder := strings.TrimSpace(backupFolder); folder != "" {
		return filepath.Clean(folder)
	}
	if libraryPath == "" {
		return ""
	}
	lib := filepath.Clean(libraryPath)
	return filepath.Join(filepath.Dir(lib), LoudnessBackupFolderName)
}

// LoudnessBackupPath mirrors a track's library-relative location under the
// backup root, so "Artist/Album/Song.mp3" is backed up to
// "<root>/Artist/Album/Song.mp3". Keeping the structure means restoring is a
// straight copy back, and two songs sharing a filename in different albums
// cannot overwrite each other's backup.
func LoudnessBackupPath(backupFolder, libraryPath, trackPath string) string {
	root := LoudnessBackupRoot(backupFolder, libraryPath)
	if root == "" {
		return ""
	}
	rel := filepath.Base(trackPath)
	if libraryPath != "" {
		if r, err := filepath.Rel(filepath.Clean(libraryPath), filepath.Clean(trackPath)); err == nil &&
			r != "." && !strings.HasPrefix(r, "..") {
			rel = r
		}
	}
	return filepath.Join(root, rel)
}

// FindLoudnessBackup returns the stored original for trackPath, or "" when
// there is none.
func FindLoudnessBackup(backupFolder, libraryPath, trackPath string) string {
	path := LoudnessBackupPath(backupFolder, libraryPath, trackPath)
	if path != "" && fileExists(path) == nil {
		return path
	}
	return ""
}

// BackupOriginal stores an untouched copy of trackPath before it is replaced.
//
// An existing backup is never overwritten: it is the original, and by the time
// a song is processed a second time the file on disk is already normalized.
func BackupOriginal(trackPath string, mode os.FileMode, libraryPath, backupFolder string) error {
	dest := LoudnessBackupPath(backupFolder, libraryPath, trackPath)
	if dest == "" {
		return fmt.Errorf("no backup location: neither BackupFolder nor LibraryPath is set")
	}

	if _, err := os.Stat(dest); err == nil {
		return nil // already have the original
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating backup folder: %w", err)
	}
	return copyFileWithMode(trackPath, dest, mode)
}

// copyFileWithMode copies src to dst so that dst is either absent or a
// complete copy - never a partial one.
//
// This matters more here than anywhere else in the pipeline. The backup is the
// only surviving original once the track is rewritten, and the check above
// treats "the file exists" as "the original is safe". If a crash could leave a
// half-written file at that path, the next run would trust it and overwrite the
// real audio, losing it for good. So the copy goes to a temporary file in the
// same directory, is flushed to disk, and only then is renamed into place -
// and rename within a directory is atomic.
func copyFileWithMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".incomplete-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	completed := false
	defer func() {
		if !completed {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		return err
	}
	// A backup still sitting in the page cache is not yet a backup.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	completed = true
	return nil
}
