package gcpsync

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type playlistNameEdit struct {
	path          string
	before, after []byte
	existed       bool
}

// Plan all text-playlist changes before renaming a song. Database playlists use
// media IDs and need no edit. Changed external playlists are also staged.
func planPlaylistNameEdits(library, oldName, newName string) ([]playlistNameEdit, error) {
	planned := map[string]playlistNameEdit{}
	visited := map[string]bool{}
	for _, root := range []string{library, playlistDestRoot(), syncRoot()} {
		if root == "" {
			continue
		}
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") || filepath.Clean(path) == trashRoot() {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.EqualFold(filepath.Ext(path), ".m3u") {
				return nil
			}
			absolute, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			if visited[absolute] {
				return nil
			}
			visited[absolute] = true
			before, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			after := rewritePlaylistName(before, oldName, newName)
			if bytes.Equal(before, after) {
				return nil
			}
			planned[absolute] = playlistNameEdit{absolute, before, after, true}
			if _, inside := relUnder(syncRoot(), absolute); !inside && syncRoot() != "" {
				relative, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				staged := filepath.Join(syncRoot(), relative)
				old, err := os.ReadFile(staged)
				if err != nil && !os.IsNotExist(err) {
					return err
				}
				// Do not replace an independent pending playlist edit.
				if err == nil && !bytes.Equal(old, before) && !bytes.Equal(old, after) {
					return fmt.Errorf("pending playlist %q has different content; sync or reconcile it first", staged)
				}
				planned[staged] = playlistNameEdit{staged, old, after, err == nil}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	edits := make([]playlistNameEdit, 0, len(planned))
	for _, edit := range planned {
		edits = append(edits, edit)
	}
	return edits, nil
}

func rewritePlaylistName(contents []byte, oldName, newName string) []byte {
	lines := strings.Split(string(contents), "\n")
	for i, line := range lines {
		value := strings.TrimSpace(line)
		if value == "" || strings.HasPrefix(value, "#") {
			continue
		}
		// Paths may come from Windows even when the server runs on Unix.
		at := strings.LastIndexAny(value, "/\\") + 1
		if value[at:] != oldName {
			continue
		}
		replacement := value[:at] + newName
		lines[i] = strings.Replace(line, value, replacement, 1)
	}
	return []byte(strings.Join(lines, "\n"))
}

func writePlaylistEdit(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".playlist-rename-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func applyPlaylistNameEdits(edits []playlistNameEdit) error {
	for i, edit := range edits {
		if err := writePlaylistEdit(edit.path, edit.after); err != nil {
			var rollbackErr error
			for j := i - 1; j >= 0; j-- {
				old := edits[j]
				var e error
				if old.existed {
					e = writePlaylistEdit(old.path, old.before)
				} else {
					e = os.Remove(old.path)
				}
				if e != nil {
					rollbackErr = e
				}
			}
			if rollbackErr != nil {
				return fmt.Errorf("updating playlist: %v; rolling back playlists: %w", err, rollbackErr)
			}
			return err
		}
	}
	return nil
}
