package utils

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/navidrome/navidrome/model/id"
)

func TempFileName(prefix, suffix string) string {
	return filepath.Join(os.TempDir(), prefix+id.NewRandom()+suffix)
}

func BaseName(filePath string) string {
	p := path.Base(filePath)
	return strings.TrimSuffix(p, path.Ext(p))
}

// FileExists checks if a file or directory exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !os.IsNotExist(err)
}

// ResolvePlaylistEntryPath returns an absolute representation of the track path referenced
// inside a playlist file. If the entry is relative, it is resolved against the playlist
// location. The returned value always uses forward slashes so it can be safely stored and
// displayed regardless of the operating system in use.
func ResolvePlaylistEntryPath(playlistPath, trackPath string) string {
	if trackPath == "" {
		return ""
	}

	cleaned := filepath.Clean(trackPath)
	if filepath.IsAbs(cleaned) || playlistPath == "" {
		return filepath.ToSlash(cleaned)
	}

	baseDir := filepath.Dir(filepath.Clean(playlistPath))
	resolved := filepath.Join(baseDir, cleaned)

	return filepath.ToSlash(filepath.Clean(resolved))
}
