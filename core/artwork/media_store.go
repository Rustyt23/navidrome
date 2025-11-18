package artwork

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

var invalidMediaID = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// MediaStore provides persistent storage for media-specific artwork files.
type MediaStore interface {
	SaveMediaArtwork(ctx context.Context, mediaID string, data []byte) (model.ArtworkID, error)
	HasMediaArtwork(mediaID string) bool
	OpenMediaArtwork(mediaID string) (io.ReadCloser, string, error)
}

type mediaStore struct {
	basePath string
	once     sync.Once
}

// NewMediaStore builds a MediaStore rooted inside Navidrome's data folder.
func NewMediaStore() MediaStore {
	return &mediaStore{basePath: filepath.Join(conf.Server.DataFolder, "artwork")}
}

func (s *mediaStore) ensureBasePath() {
	s.once.Do(func() {
		if err := os.MkdirAll(s.basePath, 0o755); err != nil {
			log.Error("Unable to create artwork store", "path", s.basePath, "err", err)
		}
	})
}

func (s *mediaStore) SaveMediaArtwork(ctx context.Context, mediaID string, data []byte) (model.ArtworkID, error) {
	if err := ctx.Err(); err != nil {
		return model.ArtworkID{}, err
	}
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" {
		return model.ArtworkID{}, errors.New("media id is required")
	}
	if len(data) == 0 {
		return model.ArtworkID{}, errors.New("artwork data is empty")
	}
	s.ensureBasePath()
	path := s.pathFor(mediaID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return model.ArtworkID{}, fmt.Errorf("creating artwork folder: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "nd-art-*")
	if err != nil {
		return model.ArtworkID{}, fmt.Errorf("creating temp artwork file: %w", err)
	}
	defer func() {
		_ = os.Remove(tmp.Name())
	}()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return model.ArtworkID{}, fmt.Errorf("writing artwork data: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return model.ArtworkID{}, fmt.Errorf("closing artwork file: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return model.ArtworkID{}, fmt.Errorf("saving artwork: %w", err)
	}
	now := time.Now()
	_ = os.Chtimes(path, now, now)
	artID := model.NewArtworkID(model.KindMediaFileArtwork, mediaID, &now)
	return artID, nil
}

func (s *mediaStore) HasMediaArtwork(mediaID string) bool {
	s.ensureBasePath()
	path := s.pathFor(mediaID)
	_, err := os.Stat(path)
	return err == nil
}

func (s *mediaStore) OpenMediaArtwork(mediaID string) (io.ReadCloser, string, error) {
	s.ensureBasePath()
	path := s.pathFor(mediaID)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", ErrUnavailable
		}
		return nil, "", err
	}
	return f, path, nil
}

func (s *mediaStore) pathFor(mediaID string) string {
	sanitized := invalidMediaID.ReplaceAllString(mediaID, "")
	if sanitized == "" {
		sanitized = fmt.Sprintf("%x", time.Now().UnixNano())
	}
	sanitized = strings.ToLower(sanitized)
	switch {
	case len(sanitized) <= 2:
		return filepath.Join(s.basePath, sanitized)
	case len(sanitized) <= 4:
		return filepath.Join(s.basePath, sanitized[:2], sanitized)
	default:
		return filepath.Join(s.basePath, sanitized[:2], sanitized[2:4], sanitized)
	}
}

func fromMediaStore(_ context.Context, store MediaStore, mediaID string) sourceFunc {
	return func() (io.ReadCloser, string, error) {
		if store == nil {
			return nil, "", ErrUnavailable
		}
		return store.OpenMediaArtwork(mediaID)
	}
}
