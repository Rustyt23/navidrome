package nativeapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core"
	"github.com/navidrome/navidrome/core/stream"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils/str"
)

// maxDownloadSelection caps one archive.
//
// Not a guess about what is reasonable to want - it is what one HTTP request
// can be held open for. Every track is read, possibly transcoded, and streamed
// into the zip while the browser waits, so a selection of thousands is a
// request that times out somewhere in the middle and leaves a truncated archive
// that looks complete.
const maxDownloadSelection = 500

// downloadSongs archives a hand-picked set of songs.
//
// A GET rather than a POST, and that is the whole reason this exists as its own
// endpoint: the browser has to navigate to it for the download to stream
// straight to disk. A POST would have to be fetched, buffered in memory as a
// blob and handed to a synthetic link, which for a few hundred megabytes of
// music is the difference between working and locking up the tab. The JWT
// travels in the query string, which the auth chain already accepts.
func (n *Router) downloadSongs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if !conf.Server.EnableDownloads {
			http.Error(w, "downloads are disabled on this server", http.StatusForbidden)
			return
		}

		ids := r.URL.Query()["id"]
		if len(ids) == 0 {
			http.Error(w, "no songs selected", http.StatusBadRequest)
			return
		}
		if len(ids) > maxDownloadSelection {
			http.Error(w, fmt.Sprintf("too many songs selected: %d, the limit is %d",
				len(ids), maxDownloadSelection), http.StatusBadRequest)
			return
		}

		// Counted before a single header is written. Once the archive starts the
		// status line is gone and there is no way left to report a problem: an
		// error found later can only end the stream early, which arrives as a
		// zip file that opens short rather than as a failure.
		found, err := n.ds.MediaFile(ctx).CountAll(model.QueryOptions{
			Filters: squirrel.And{
				squirrel.Eq{"media_file.id": ids},
				squirrel.Eq{"media_file.missing": false},
			},
		})
		if err != nil {
			log.Error(ctx, "Could not count songs for download", err)
			http.Error(w, "could not read the selected songs", http.StatusInternalServerError)
			return
		}
		if found == 0 {
			http.Error(w, "none of the selected songs are available", http.StatusNotFound)
			return
		}

		format, _ := firstOf(r.URL.Query()["format"])
		if format == "" {
			format = "raw"
		}
		bitrate := 0
		if raw, ok := firstOf(r.URL.Query()["bitrate"]); ok {
			bitrate, _ = strconv.Atoi(raw)
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition",
			fmt.Sprintf("attachment; filename=%q", downloadFilename(found)))

		archiver := core.NewArchiver(n.streamer, n.ds, n.share)
		if err := archiver.ZipMediaFiles(ctx, ids, format, bitrate, w); err != nil {
			// Deliberately not written to the response: the headers went out with
			// the first byte of the zip, so anything added here would be appended
			// to the archive rather than shown to anyone. The log is the only
			// honest place left to say it.
			if errors.Is(err, stream.ErrTooManyTranscodes) {
				log.Warn(ctx, "Download archive ended early: transcode cap reached", "songs", found, err)
				return
			}
			log.Error(ctx, "Error building download archive", "songs", found, err)
		}
	}
}

func firstOf(values []string) (string, bool) {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v, true
		}
	}
	return "", false
}

// downloadFilename names the archive after what is in it, since a selection has
// no name of its own the way an album or a playlist does.
func downloadFilename(count int64) string {
	name := fmt.Sprintf("navidrome-%d-songs.zip", count)
	if count == 1 {
		name = "navidrome-1-song.zip"
	}
	return str.SanitizeFilename(name)
}
