// Package taglibwrite provides tag-writing capabilities (comments, fetched
// metadata, embedded covers) using the CGO TagLib wrapper. Reading tags is
// handled by the pure-Go adapters/gotaglib extractor; this package only keeps
// the custom write functionality that go-taglib does not provide.
package taglibwrite

/*
#cgo !windows pkg-config: taglib
#cgo windows pkg-config: taglib
#cgo illumos LDFLAGS: -lstdc++ -lsendfile
#cgo linux darwin CXXFLAGS: -std=c++11
#cgo darwin LDFLAGS: -L/opt/homebrew/opt/taglib/lib
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "taglib_write_wrapper.h"
*/
import "C"
import (
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"unsafe"

	"github.com/navidrome/navidrome/log"
)

func WriteComment(filename, comment string) (err error) {
	debug.SetPanicOnFault(true)
	defer func() {
		if r := recover(); r != nil {
			log.Error("extractor: recovered from panic when writing comment", "file", filename, "error", r)
			err = fmt.Errorf("extractor: recovered from panic: %s", r)
		}
	}()

	fp := getFilename(filename)
	defer C.free(unsafe.Pointer(fp))

	cComment := C.CString(comment)
	defer C.free(unsafe.Pointer(cComment))

	res := C.taglib_write_comment(fp, cComment)
	switch res {
	case 0:
		return nil
	case C.TAGLIB_ERR_PARSE:
		return fmt.Errorf("cannot open media file for writing comment")
	case C.TAGLIB_ERR_SAVE:
		return fmt.Errorf("cannot save media file after writing comment")
	default:
		return fmt.Errorf("unknown error writing comment: %d", int(res))
	}
}

type FetchedMetadata struct {
	Album         string
	Year          int
	Genre         string
	RecordingMBID string
	ReleaseMBID   string
	CoverPath     string
}

func WriteFetchedMetadata(filename string, md FetchedMetadata) (err error) {
	debug.SetPanicOnFault(true)
	defer func() {
		if r := recover(); r != nil {
			log.Error("extractor: recovered from panic when writing fetched metadata", "file", filename, "error", r)
			err = fmt.Errorf("extractor: recovered from panic: %s", r)
		}
	}()

	fp := getFilename(filename)
	defer C.free(unsafe.Pointer(fp))

	cAlbum := C.CString(strings.TrimSpace(md.Album))
	defer C.free(unsafe.Pointer(cAlbum))

	year := ""
	if md.Year > 0 {
		year = strconv.Itoa(md.Year)
	}
	cYear := C.CString(year)
	defer C.free(unsafe.Pointer(cYear))

	cGenre := C.CString(strings.TrimSpace(md.Genre))
	defer C.free(unsafe.Pointer(cGenre))

	cRecordingMBID := C.CString(strings.TrimSpace(md.RecordingMBID))
	defer C.free(unsafe.Pointer(cRecordingMBID))

	cReleaseMBID := C.CString(strings.TrimSpace(md.ReleaseMBID))
	defer C.free(unsafe.Pointer(cReleaseMBID))

	coverPath := strings.TrimSpace(md.CoverPath)
	if coverPath != "" {
		coverPath = filepath.Clean(coverPath)
	}
	cCoverPath := C.CString(coverPath)
	defer C.free(unsafe.Pointer(cCoverPath))

	res := C.taglib_write_fetched_metadata(fp, cAlbum, cYear, cGenre, cRecordingMBID, cReleaseMBID, cCoverPath)
	switch res {
	case 0:
		return nil
	case C.TAGLIB_ERR_PARSE:
		return fmt.Errorf("cannot open media file for writing metadata")
	case C.TAGLIB_ERR_SAVE:
		return fmt.Errorf("cannot save media file after writing metadata")
	default:
		return fmt.Errorf("unknown error writing metadata: %d", int(res))
	}
}

