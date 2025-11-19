package nativeapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

type fileMetadataUpdate struct {
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Genre       string
	Year        int
	Track       int
	Disc        int
	ArtworkURL  string
}

var artworkHTTPClient = &http.Client{Timeout: 30 * time.Second}

func (n *Router) persistMetadata(ctx context.Context, song metadataSongPayload) error {
	if n.ds == nil {
		return errors.New("datastore not configured")
	}
	if song.ID == "" {
		return errors.New("missing song id")
	}

	repo := n.ds.MediaFile(ctx)
	if repo == nil {
		return errors.New("media file repository unavailable")
	}

	mf, err := repo.Get(song.ID)
	if err != nil {
		return err
	}

	update := fileMetadataUpdate{
		Title:       coalesceString(song.Title, mf.Title),
		Artist:      coalesceString(song.Artist, mf.Artist),
		Album:       coalesceString(song.Album, mf.Album),
		AlbumArtist: selectAlbumArtist(song, mf),
		Genre:       coalesceString(song.Genre, mf.Genre),
		Year:        selectYear(song.Year, mf.Year),
		Track:       mf.TrackNumber,
		Disc:        mf.DiscNumber,
		ArtworkURL:  song.ArtworkURL,
	}

	if err := writeTagsWithExiftool(ctx, mf.AbsolutePath(), update); err != nil {
		return err
	}
	return nil
}

func coalesceString(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func selectAlbumArtist(song metadataSongPayload, mf *model.MediaFile) string {
	albumArtist := coalesceString(mf.AlbumArtist, song.Artist, mf.Artist)
	return albumArtist
}

func selectYear(songYear *int, current int) int {
	if songYear != nil && *songYear > 0 {
		return *songYear
	}
	if current > 0 {
		return current
	}
	return 0
}

func writeTagsWithExiftool(ctx context.Context, filePath string, update fileMetadataUpdate) error {
	if filePath == "" {
		return errors.New("empty media file path")
	}

	args := []string{"-overwrite_original"}
	addString := func(key, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		args = append(args, fmt.Sprintf("-%s=%s", key, value))
	}
	addInt := func(key string, value int) {
		if value <= 0 {
			return
		}
		args = append(args, fmt.Sprintf("-%s=%d", key, value))
	}

	addString("Title", update.Title)
	addString("Artist", update.Artist)
	addString("Album", update.Album)
	addString("AlbumArtist", update.AlbumArtist)
	addString("Genre", update.Genre)
	addInt("Year", update.Year)
	addInt("Track", update.Track)
	addInt("DiscNumber", update.Disc)

	cleanup := func() {}
	if update.ArtworkURL != "" {
		if artPath, closer, err := downloadArtworkFile(ctx, update.ArtworkURL); err == nil {
			cleanup = closer
			args = append(args, "-Picture=")
			args = append(args, fmt.Sprintf("-Picture<=%s", artPath))
		} else {
			log.Warn(ctx, "Unable to download artwork for metadata persistence", "url", update.ArtworkURL, "err", err)
		}
	}
	defer cleanup()

	if len(args) == 1 {
		// No updates to write
		return nil
	}

	args = append(args, filePath)
	cmd := exec.CommandContext(ctx, "exiftool", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exiftool failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func downloadArtworkFile(ctx context.Context, url string) (string, func(), error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", nil, err
	}

	resp, err := artworkHTTPClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	tmp, err := os.CreateTemp("", "navidrome-artwork-*.img")
	if err != nil {
		return "", nil, err
	}

	cleanup := func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		cleanup()
		return "", nil, err
	}

	if err := tmp.Close(); err != nil {
		cleanup()
		return "", nil, err
	}

	return tmp.Name(), cleanup, nil
}
