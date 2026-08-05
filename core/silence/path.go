package silence

import (
	"path/filepath"

	"github.com/navidrome/navidrome/model"
)

// TrackPath resolves where a song actually lives on disk.
//
// media_file.path is stored relative to its library, so it is usually just
// "Artist/Album/01 - Song.mp3" - and for a flat library, only the filename.
// Handing that straight to ffmpeg asks it to open a file in the process's
// working directory, which fails with "no such file or directory" on every
// track no matter how healthy the library is.
//
// An already-absolute path is left alone rather than joined, so a library whose
// entries are stored absolute is not turned into "/music/Users/.../song.mp3".
// FromSlash is applied first because the stored separator is not necessarily
// the platform's.
//
// Deliberately its own function rather than model.MediaFile.AbsolutePath(),
// which joins unconditionally and would mangle an absolute entry.
func TrackPath(mf *model.MediaFile) string {
	if mf == nil {
		return ""
	}
	path := filepath.FromSlash(mf.Path)
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(mf.LibraryPath, path)
	}
	return filepath.Clean(path)
}
