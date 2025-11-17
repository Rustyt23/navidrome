package artwork

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/navidrome/navidrome/conf"
)

const mediaArtworkFolder = "media-artwork"

// mediaArtworkPath returns the path inside the cache folder where a media file's
// downloaded artwork should live. The directory structure is spread to avoid
// having too many files on the same level.
func mediaArtworkPath(id string) string {
	if id == "" {
		return filepath.Join(conf.Server.CacheFolder, mediaArtworkFolder, "_invalid")
	}
	hash := fmt.Sprintf("%x", sha1.Sum([]byte(id)))
	return filepath.Join(
		conf.Server.CacheFolder,
		mediaArtworkFolder,
		hash[0:2],
		hash[2:4],
		fmt.Sprintf("%s.art", id),
	)
}

// SaveMediaArtwork persists the provided artwork bytes on disk, making it
// available for the mediafile artwork reader without requiring an additional
// download.
func SaveMediaArtwork(id string, r io.Reader) (string, error) {
	path := mediaArtworkPath(id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "nd-art-*.tmp")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = os.Remove(tmp.Name())
	}()
	if _, err = io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err = tmp.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}

// OpenMediaArtwork opens the stored artwork for the given media file id.
func OpenMediaArtwork(id string) (*os.File, error) {
	return os.Open(mediaArtworkPath(id))
}

// MediaArtworkExists reports whether there's a stored artwork file for the
// provided media file id.
func MediaArtworkExists(id string) bool {
	_, err := os.Stat(mediaArtworkPath(id))
	return err == nil
}
