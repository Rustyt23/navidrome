package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/utils/cache"
)

type MediaStreamer interface {
	NewStream(ctx context.Context, id string, reqFormat string, reqBitRate int, offset int) (*Stream, error)
	DoStream(ctx context.Context, mf *model.MediaFile, reqFormat string, reqBitRate int, reqOffset int) (*Stream, error)
}

type TranscodingCache cache.FileCache

func NewMediaStreamer(ds model.DataStore, t ffmpeg.FFmpeg, cache TranscodingCache) MediaStreamer {
	return &mediaStreamer{ds: ds, transcoder: t, cache: cache}
}

type mediaStreamer struct {
	ds         model.DataStore
	transcoder ffmpeg.FFmpeg
	cache      cache.FileCache
}

type streamJob struct {
	ms       *mediaStreamer
	mf       *model.MediaFile
	filePath string
	format   string
	bitRate  int
	offset   int
}

func (j *streamJob) Key() string {
	return fmt.Sprintf("%s.%s.%d.%s.%d", j.mf.ID, j.mf.UpdatedAt.Format(time.RFC3339Nano), j.bitRate, j.format, j.offset)
}

func (ms *mediaStreamer) NewStream(ctx context.Context, id string, reqFormat string, reqBitRate int, reqOffset int) (*Stream, error) {
	mf, err := ms.ds.MediaFile(ctx).Get(id)
	if err == nil {
		return ms.DoStream(ctx, mf, reqFormat, reqBitRate, reqOffset)
	}
	if !errors.Is(err, model.ErrNotFound) && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	track, trackErr := ms.ds.DiscoveryTrack(ctx).Get(id)
	if trackErr != nil {
		return nil, trackErr
	}
	discoveryMF, buildErr := mediaFileFromDiscoveryTrack(track)
	if buildErr != nil {
		return nil, buildErr
	}
	return ms.DoStream(ctx, discoveryMF, reqFormat, reqBitRate, reqOffset)
}

func (ms *mediaStreamer) DoStream(ctx context.Context, mf *model.MediaFile, reqFormat string, reqBitRate int, reqOffset int) (*Stream, error) {
	var format string
	var bitRate int
	var cached bool
	defer func() {
		log.Info(ctx, "Streaming file", "title", mf.Title, "artist", mf.Artist, "format", format, "cached", cached,
			"bitRate", bitRate, "user", userName(ctx), "transcoding", format != "raw",
			"originalFormat", mf.Suffix, "originalBitRate", mf.BitRate)
	}()

	format, bitRate = selectTranscodingOptions(ctx, ms.ds, mf, reqFormat, reqBitRate)
	s := &Stream{ctx: ctx, mf: mf, format: format, bitRate: bitRate}
	filePath := mf.AbsolutePath()

	if format == "raw" {
		log.Debug(ctx, "Streaming RAW file", "id", mf.ID, "path", filePath,
			"requestBitrate", reqBitRate, "requestFormat", reqFormat, "requestOffset", reqOffset,
			"originalBitrate", mf.BitRate, "originalFormat", mf.Suffix,
			"selectedBitrate", bitRate, "selectedFormat", format)
		f, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		s.ReadCloser = f
		s.Seeker = f
		s.format = mf.Suffix
		return s, nil
	}

	job := &streamJob{
		ms:       ms,
		mf:       mf,
		filePath: filePath,
		format:   format,
		bitRate:  bitRate,
		offset:   reqOffset,
	}
	r, err := ms.cache.Get(ctx, job)
	if err != nil {
		log.Error(ctx, "Error accessing transcoding cache", "id", mf.ID, err)
		return nil, err
	}
	cached = r.Cached

	s.ReadCloser = r
	s.Seeker = r.Seeker

	log.Debug(ctx, "Streaming TRANSCODED file", "id", mf.ID, "path", filePath,
		"requestBitrate", reqBitRate, "requestFormat", reqFormat, "requestOffset", reqOffset,
		"originalBitrate", mf.BitRate, "originalFormat", mf.Suffix,
		"selectedBitrate", bitRate, "selectedFormat", format, "cached", cached, "seekable", s.Seekable())

	return s, nil
}

type Stream struct {
	ctx     context.Context
	mf      *model.MediaFile
	bitRate int
	format  string
	io.ReadCloser
	io.Seeker
}

func (s *Stream) Seekable() bool      { return s.Seeker != nil }
func (s *Stream) Duration() float32   { return s.mf.Duration }
func (s *Stream) ContentType() string { return mime.TypeByExtension("." + s.format) }
func (s *Stream) Name() string        { return s.mf.Title + "." + s.format }
func (s *Stream) ModTime() time.Time  { return s.mf.UpdatedAt }
func (s *Stream) EstimatedContentLength() int {
	return int(s.mf.Duration * float32(s.bitRate) / 8 * 1024)
}

func mediaFileFromDiscoveryTrack(track *model.DiscoveryTrack) (*model.MediaFile, error) {
	mf := track.MediaFile
	if mf.ID == "" {
		mf.ID = track.ID
	}
	abs := track.SourcePath
	if abs == "" {
		if filepath.IsAbs(mf.Path) {
			abs = mf.Path
		} else if mf.LibraryPath != "" && mf.Path != "" {
			abs = filepath.Join(mf.LibraryPath, mf.Path)
		}
	}
	if abs == "" && mf.Path != "" && mf.LibraryPath != "" {
		abs = filepath.Join(mf.LibraryPath, mf.Path)
	}
	if abs == "" {
		return nil, model.ErrNotFound
	}
	abs = filepath.Clean(abs)

	libraryPath := mf.LibraryPath
	relPath := mf.Path

	if libraryPath == "" || !filepath.IsAbs(libraryPath) {
		libraryPath = filepath.Dir(abs)
	} else {
		rel, err := filepath.Rel(libraryPath, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			libraryPath = filepath.Dir(abs)
			relPath = filepath.Base(abs)
		} else if relPath == "" || filepath.IsAbs(relPath) {
			relPath = filepath.ToSlash(rel)
		}
	}

	if relPath == "" {
		relPath = filepath.Base(abs)
	}
	if filepath.IsAbs(relPath) {
		relPath = filepath.Base(abs)
	}

	mf.LibraryPath = filepath.Clean(libraryPath)
	mf.Path = filepath.ToSlash(relPath)
	mf.Missing = false
	if mf.UpdatedAt.IsZero() {
		mf.UpdatedAt = time.Now().UTC()
	}
	if mf.Title == "" {
		mf.Title = filepath.Base(abs)
	}
	return &mf, nil
}

// TODO This function deserves some love (refactoring)
func selectTranscodingOptions(ctx context.Context, ds model.DataStore, mf *model.MediaFile, reqFormat string, reqBitRate int) (format string, bitRate int) {
	format = "raw"
	if reqFormat == "raw" {
		return format, 0
	}
	if reqFormat == mf.Suffix && reqBitRate == 0 {
		bitRate = mf.BitRate
		return format, bitRate
	}
	trc, hasDefault := request.TranscodingFrom(ctx)
	var cFormat string
	var cBitRate int
	if reqFormat != "" {
		cFormat = reqFormat
	} else {
		if hasDefault {
			cFormat = trc.TargetFormat
			cBitRate = trc.DefaultBitRate
			if p, ok := request.PlayerFrom(ctx); ok {
				cBitRate = p.MaxBitRate
			}
		} else if reqBitRate > 0 && reqBitRate < mf.BitRate && conf.Server.DefaultDownsamplingFormat != "" {
			// If no format is specified and no transcoding associated to the player, but a bitrate is specified,
			// and there is no transcoding set for the player, we use the default downsampling format.
			// But only if the requested bitRate is lower than the original bitRate.
			log.Debug("Default Downsampling", "Using default downsampling format", conf.Server.DefaultDownsamplingFormat)
			cFormat = conf.Server.DefaultDownsamplingFormat
		}
	}
	if reqBitRate > 0 {
		cBitRate = reqBitRate
	}
	if cBitRate == 0 && cFormat == "" {
		return format, bitRate
	}
	t, err := ds.Transcoding(ctx).FindByFormat(cFormat)
	if err == nil {
		format = t.TargetFormat
		if cBitRate != 0 {
			bitRate = cBitRate
		} else {
			bitRate = t.DefaultBitRate
		}
	}
	if format == mf.Suffix && bitRate >= mf.BitRate {
		format = "raw"
		bitRate = 0
	}
	return format, bitRate
}

var (
	onceTranscodingCache     sync.Once
	instanceTranscodingCache TranscodingCache
)

func GetTranscodingCache() TranscodingCache {
	onceTranscodingCache.Do(func() {
		instanceTranscodingCache = NewTranscodingCache()
	})
	return instanceTranscodingCache
}

func NewTranscodingCache() TranscodingCache {
	return cache.NewFileCache("Transcoding", conf.Server.TranscodingCacheSize,
		consts.TranscodingCacheDir, consts.DefaultTranscodingCacheMaxItems,
		func(ctx context.Context, arg cache.Item) (io.Reader, error) {
			job := arg.(*streamJob)
			t, err := job.ms.ds.Transcoding(ctx).FindByFormat(job.format)
			if err != nil {
				log.Error(ctx, "Error loading transcoding command", "format", job.format, err)
				return nil, os.ErrInvalid
			}
			out, err := job.ms.transcoder.Transcode(ctx, t.Command, job.filePath, job.bitRate, job.offset)
			if err != nil {
				log.Error(ctx, "Error starting transcoder", "id", job.mf.ID, err)
				return nil, os.ErrInvalid
			}
			return out, nil
		})
}
