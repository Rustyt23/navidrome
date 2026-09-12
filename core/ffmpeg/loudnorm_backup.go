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

// LoudnessBackupPath returns where a song's original is stored.
//
// The location is derived from the song's own identifier, not from where it
// currently sits in the library. Two things follow from that, and both were
// bugs while backups were keyed by path: a song can be moved or renamed without
// losing its original, and two different songs can never claim the same slot -
// which previously meant a song arriving at a deleted song's path inherited its
// backup, and restoring handed back the wrong recording.
//
// The original filename is kept on the end so the folder is still readable, and
// the first characters of the id shard the tree so no single directory has to
// hold a whole library.
func LoudnessBackupPath(backupFolder, libraryPath, mediaFileID, trackPath string) string {
	root := LoudnessBackupRoot(backupFolder, libraryPath)
	id := backupIDComponent(mediaFileID)
	if root == "" || id == "" {
		return ""
	}
	return filepath.Join(root, id[:2], id+"__"+filepath.Base(trackPath))
}

// backupIDComponent reduces a media file id to something safe to build a path
// from, and long enough to shard on.
func backupIDComponent(mediaFileID string) string {
	var b strings.Builder
	for _, r := range mediaFileID {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	id := b.String()
	if len(id) < 2 {
		return ""
	}
	return id
}

// LegacyLoudnessBackupPath is where backups were kept before they were keyed by
// song identity: the library-relative path mirrored under the backup root. Read
// so that originals stored by an earlier version are still found; never written
// to again.
func LegacyLoudnessBackupPath(backupFolder, libraryPath, trackPath string) string {
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

// FindLoudnessBackup returns the stored original for a song, or "" when there
// is none. A backup stored by an earlier version is found at its old location;
// whether it is really this song's original is settled before anything is
// overwritten with it, not here.
func FindLoudnessBackup(backupFolder, libraryPath, mediaFileID, trackPath string) string {
	path, _ := findLoudnessBackup(backupFolder, libraryPath, mediaFileID, trackPath)
	return path
}

// The filename is descriptive; only the permanent id identifies the original.
// Mutation callers must propagate lookup failures, especially multiple originals
// left by older versions, rather than backing up or restoring the wrong file.
func findLoudnessBackup(backupFolder, libraryPath, mediaFileID, trackPath string) (string, error) {
	if path := LoudnessBackupPath(backupFolder, libraryPath, mediaFileID, trackPath); path != "" {
		entries, err := os.ReadDir(filepath.Dir(path))
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("reading original backups: %w", err)
		}
		prefix := backupIDComponent(mediaFileID) + "__"
		var found string
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), prefix) {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return "", fmt.Errorf("checking original backup: %w", err)
			}
			if !info.Mode().IsRegular() {
				return "", fmt.Errorf("original backup %q is not a regular file", entry.Name())
			}
			if found != "" {
				return "", fmt.Errorf("multiple original backups for song %s; resolve them before processing or restoring", mediaFileID)
			}
			found = filepath.Join(filepath.Dir(path), entry.Name())
		}
		if found != "" {
			return found, nil
		}
	}
	if path := LegacyLoudnessBackupPath(backupFolder, libraryPath, trackPath); path != "" {
		if fileExists(path) == nil {
			return path, nil
		}
	}
	return "", nil
}

// BackupOriginal stores an untouched copy of trackPath before it is replaced.
//
// An existing backup is never overwritten: it is the original, and by the time
// a song is processed a second time the file on disk is already normalized.
func BackupOriginal(trackPath string, mode os.FileMode, libraryPath, backupFolder, mediaFileID string) error {
	dest := LoudnessBackupPath(backupFolder, libraryPath, mediaFileID, trackPath)
	if dest == "" {
		return fmt.Errorf("no backup location: BackupFolder/LibraryPath or the song id is missing")
	}

	// Any stored original for this song counts, wherever an earlier version put
	// it. What must not happen is a second copy being taken of a file that has
	// already been rewritten.
	stored, err := findLoudnessBackup(backupFolder, libraryPath, mediaFileID, trackPath)
	if err != nil {
		return err
	}
	if stored != "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating backup folder: %w", err)
	}
	return copyFileWithMode(trackPath, dest, mode)
}

// RestoreOriginal puts the stored original back at trackPath.
//
// The backup is copied, not moved. It is the client's original: keeping it
// means a track can be restored again after being re-processed, and means the
// one irreplaceable copy is never removed by an operation whose whole purpose
// is to undo something. The copy is atomic for the reason the backup was - a
// crash part way through must not leave a truncated file where the song is.
func RestoreOriginal(trackPath, libraryPath, backupFolder, mediaFileID string) error {
	backup, err := findLoudnessBackup(backupFolder, libraryPath, mediaFileID, trackPath)
	if err != nil {
		return err
	}
	if backup == "" {
		return fmt.Errorf("no stored original for %s", trackPath)
	}

	// Restore the permissions the track has now rather than the backup's: the
	// file keeps its place in the library, it only gets its audio back.
	mode := os.FileMode(0o644)
	if stat, err := os.Stat(trackPath); err == nil {
		mode = stat.Mode()
	} else if !os.IsNotExist(err) {
		return err
	}
	return copyFileWithMode(backup, trackPath, mode)
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
