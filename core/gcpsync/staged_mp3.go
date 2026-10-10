package gcpsync

import (
	"context"
	"os"
	"path/filepath"

	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/utils/filelock"
)

// uploadProcessedMP3 sends an immutable snapshot. StageMP3 may replace the
// queued path while gcloud is reading it; only the version actually uploaded
// may be retired. The caller holds sweepMu to serialize transfers to GCS.
func (s *service) uploadProcessedMP3(ctx context.Context, req uploadRequest) (string, error) {
	if dryRun() {
		return "", s.upload(ctx, req.path)
	}
	unlock := filelock.Lock(req.path)
	info, err := os.Stat(req.path)
	if os.IsNotExist(err) {
		// A manual sweep already sent it.
		unlock()
		return "", nil
	}
	if err != nil {
		unlock()
		return "", err
	}
	_, markerErr := os.Stat(libraryCopyMarker(req.path))
	if markerErr != nil && !os.IsNotExist(markerErr) {
		unlock()
		return "", markerErr
	}
	libraryCopy := req.source != "" || markerErr == nil
	dir, err := os.MkdirTemp("", "navidrome-gcpsync-")
	if err != nil {
		unlock()
		return "", err
	}
	defer os.RemoveAll(dir)
	name, err := s.mp3Name(req.path)
	if err != nil {
		unlock()
		return "", err
	}
	snapshot := filepath.Join(dir, name)
	err = copyFile(req.path, snapshot)
	unlock()
	if err != nil {
		return "", err
	}
	if err := s.upload(ctx, snapshot); err != nil {
		return "", err
	}

	// Imported processed files still move into the library. App-generated
	// copies already live there and must never roll back a later local edit.
	_, underSync := relUnder(syncRoot(), req.path)
	if !libraryCopy && underSync && musicRoot() != "" {
		unlockLibrary := filelock.Lock(filepath.Join(musicRoot(), filepath.Dir(destRel(req.path)), name))
		defer unlockLibrary()
	}
	unlock = filelock.Lock(req.path)
	defer unlock()
	current, err := os.Stat(req.path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !os.SameFile(info, current) || info.Size() != current.Size() || !info.ModTime().Equal(current.ModTime()) {
		log.Debug(ctx, "GCPSync: newer staged version retained for the next upload", "path", req.path)
		return name, nil
	}
	if libraryCopy {
		if err := os.Remove(req.path); err != nil {
			return "", err
		}
	} else if underSync {
		dest := filepath.Join(musicRoot(), filepath.Dir(destRel(req.path)), name)
		if err := s.move(ctx, req.path, dest); err != nil {
			return "", err
		}
	}
	if libraryCopy {
		if err := os.Remove(libraryCopyMarker(req.path)); err != nil && !os.IsNotExist(err) {
			return "", err
		}
	}
	return name, nil
}

// The marker survives restarts and only records ownership, never a mutable
// source path. Hidden files are excluded from sweeps and bucket uploads.
func libraryCopyMarker(file string) string {
	return filepath.Join(filepath.Dir(file), "."+filepath.Base(file)+".gcpsync")
}

func markLibraryCopy(file string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(libraryCopyMarker(file), []byte("library copy\n"), 0o600)
}

// RenameStagedCopy keeps a staged song's provenance when its name is corrected.
func RenameStagedCopy(oldPath, newPath string) error {
	if filepath.Clean(oldPath) == filepath.Clean(newPath) {
		return nil
	}
	first, second := oldPath, newPath
	if first > second {
		first, second = second, first
	}
	unlockFirst := filelock.Lock(first)
	defer unlockFirst()
	unlockSecond := filelock.Lock(second)
	defer unlockSecond()
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return err
	}
	oldMarker, newMarker := libraryCopyMarker(oldPath), libraryCopyMarker(newPath)
	_, err := os.Stat(oldMarker)
	marked := err == nil
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if marked {
		if err := os.Rename(oldMarker, newMarker); err != nil {
			return err
		}
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		if marked {
			_ = os.Rename(newMarker, oldMarker)
		}
		return err
	}
	return nil
}
