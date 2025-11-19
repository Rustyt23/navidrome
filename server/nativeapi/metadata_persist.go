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
	"github.com/navidrome/navidrome/utils/str"
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

	artWritten, err := writeTagsWithExiftool(ctx, mf.AbsolutePath(), update)
	if err != nil {
		return err
	}
	return updateMediaFileRecord(repo, mf, update, artWritten)
}

func updateMediaFileRecord(repo model.MediaFileRepository, mf *model.MediaFile, update fileMetadataUpdate, artWritten bool) error {
	if repo == nil || mf == nil {
		return errors.New("media file repository unavailable")
	}
	if !applyUpdateToMediaFile(mf, update, artWritten) {
		return nil
	}
	return repo.Put(mf)
}

func applyUpdateToMediaFile(mf *model.MediaFile, update fileMetadataUpdate, artWritten bool) bool {
	changed := false
	if update.Title != "" && update.Title != mf.Title {
		mf.Title = update.Title
		mf.OrderTitle = str.SanitizeFieldForSorting(mf.Title)
		changed = true
	}
	if update.Artist != "" && update.Artist != mf.Artist {
		mf.Artist = update.Artist
		mf.OrderArtistName = str.SanitizeFieldForSortingNoArticle(mf.Artist)
		changed = true
	}
	if update.Album != "" && update.Album != mf.Album {
		mf.Album = update.Album
		mf.OrderAlbumName = str.SanitizeFieldForSortingNoArticle(mf.Album)
		changed = true
	}
	if update.AlbumArtist != "" && update.AlbumArtist != mf.AlbumArtist {
		mf.AlbumArtist = update.AlbumArtist
		mf.OrderAlbumArtistName = str.SanitizeFieldForSortingNoArticle(mf.AlbumArtist)
		changed = true
	}
	if update.Genre != "" && update.Genre != mf.Genre {
		mf.Genre = update.Genre
		changed = true
	}
	if update.Year > 0 && update.Year != mf.Year {
		mf.Year = update.Year
		changed = true
	}
	if update.Track > 0 && update.Track != mf.TrackNumber {
		mf.TrackNumber = update.Track
		changed = true
	}
	if update.Disc > 0 && update.Disc != mf.DiscNumber {
		mf.DiscNumber = update.Disc
		changed = true
	}
	if artWritten && !mf.HasCoverArt {
		mf.HasCoverArt = true
		changed = true
	}
	return changed
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

func writeTagsWithExiftool(ctx context.Context, filePath string, update fileMetadataUpdate) (bool, error) {
	if filePath == "" {
		return false, errors.New("empty media file path")
	}

	args := []string{"-overwrite_original"}
	artWritten := false
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
			artWritten = true
		} else {
			log.Warn(ctx, "Unable to download artwork for metadata persistence", "url", update.ArtworkURL, "err", err)
		}
	}
	defer cleanup()

	if len(args) == 1 {
		// No updates to write
		return artWritten, nil
	}

	args = append(args, filePath)
	cmd := exec.CommandContext(ctx, "exiftool", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("exiftool failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return artWritten, nil
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
