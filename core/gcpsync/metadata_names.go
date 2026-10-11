package gcpsync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/filelock"
	"go.senan.xyz/taglib"
	"golang.org/x/text/unicode/norm"
)

// MetadataName derives the object filename from tags, never from the current
// filename (which may be arbitrary, reversed, or contain a track number).
func MetadataName(path string) (name string, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			name = ""
			err = fmt.Errorf("reading metadata from %q: %v", path, failure)
		}
	}()
	f, err := taglib.Open(path, taglib.WithReadStyle(taglib.ReadStyleFast))
	if err != nil {
		return "", fmt.Errorf("reading artist/title from %q: %w", path, err)
	}
	defer f.Close()
	tags := f.AllTags().Tags
	name = ExpectedName(strings.Join(tags["ARTIST"], ", "), strings.Join(tags["TITLE"], ", "))
	if name == "" {
		return "", fmt.Errorf("%q needs artist and title metadata before syncing", path)
	}
	return norm.NFC.String(name), nil
}

func (s *service) mp3Name(path string) (string, error) {
	if s.readName != nil {
		return s.readName(path)
	}
	return MetadataName(path)
}

var libraryRenameMu sync.Mutex

// StageSong corrects the library path and its database row before staging the
// updated song. IDs stay stable, so database playlists retain their references.
func StageSong(ctx context.Context, ds model.DataStore, mf *model.MediaFile, reason string) (resultErr error) {
	if !enabled() {
		// Upload configuration can be completed later. Preserve the updated
		// bytes now without changing the library filename or playlists.
		return GetInstance().StageMP3(mf.LibraryPath, mf.AbsolutePath(), reason, mf.ID)
	}
	if syncRoot() == "" {
		return fmt.Errorf("SyncFolder is not configured")
	}
	// Naming and playlist updates must not make a rewritten library file
	// disappear from the pending Sync work. Keep its surviving path so even a
	// failed rename rollback can preserve the updated bytes for a later Sync.
	// This defer runs after the rename locks are released: StageMP3 acquires
	// the source lock itself.
	fallbackPath := mf.AbsolutePath()
	stagingAttempted := false
	defer func() {
		if resultErr != nil && !stagingAttempted {
			if err := GetInstance().StageMP3(mf.LibraryPath, fallbackPath, reason, mf.ID); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("preserving updated song in Sync staging: %w", err))
			}
		}
	}()
	libraryRenameMu.Lock()
	defer libraryRenameMu.Unlock()
	oldPath := mf.AbsolutePath()
	name, err := MetadataName(oldPath)
	if err != nil {
		return err
	}
	newPath := filepath.Join(filepath.Dir(oldPath), name)
	var playlistEdits []playlistNameEdit
	if norm.NFC.String(filepath.Base(oldPath)) != name && !dryRun() {
		playlistEdits, err = planPlaylistNameEdits(mf.LibraryPath, filepath.Base(oldPath), name)
		if err != nil {
			return err
		}
		first, second := oldPath, newPath
		if first > second {
			first, second = second, first
		}
		unlockFirst := filelock.Lock(first)
		unlockSecond := filelock.Lock(second)
		defer unlockFirst()
		defer unlockSecond()
		if _, err := os.Stat(newPath); err == nil {
			return fmt.Errorf("cannot rename %q: %q already exists; resolve the duplicate first", oldPath, newPath)
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return err
		}
		fallbackPath = newPath
		oldRelative := mf.Path
		relative := filepath.Join(filepath.Dir(oldRelative), name)
		if err := ds.MediaFile(ctx).UpdatePath(mf.ID, relative); err != nil {
			if rollback := os.Rename(newPath, oldPath); rollback != nil {
				return fmt.Errorf("updating library path: %v; rolling back rename: %w", err, rollback)
			}
			fallbackPath = oldPath
			return err
		}
		if err := applyPlaylistNameEdits(playlistEdits); err != nil {
			rowErr := ds.MediaFile(ctx).UpdatePath(mf.ID, oldRelative)
			fileErr := os.Rename(newPath, oldPath)
			if fileErr == nil {
				fallbackPath = oldPath
			}
			return fmt.Errorf("playlist rename failed: %v (database rollback: %v; file rollback: %v)", err, rowErr, fileErr)
		}
		mf.Path = relative
		// StageMP3 acquires the library lock itself.
		unlockSecond()
		unlockFirst()
	}
	stagingAttempted = true
	if err := GetInstance().StageMP3(mf.LibraryPath, mf.AbsolutePath(), reason, mf.ID); err != nil {
		return err
	}
	if oldPath != mf.AbsolutePath() && !dryRun() {
		// A pending copy under the previous name targets the same metadata-derived
		// object. Retire it so a later sweep cannot upload stale audio last.
		oldStaged := StagingPath(mf.LibraryPath, oldPath)
		err := retireRenamedStagedCopy(oldStaged, mf.ID)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("retiring obsolete staged filename: %w", err)
		}
	}
	if len(playlistEdits) > 0 && immediateUploadsEnabled() {
		go func() { _ = GetInstance().Sweep(context.Background()) }()
	}
	return nil
}
