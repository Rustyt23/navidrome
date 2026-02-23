package taglib

/*
#cgo !windows pkg-config: --define-prefix taglib
#cgo windows pkg-config: taglib
#cgo illumos LDFLAGS: -lstdc++ -lsendfile
#cgo linux darwin CXXFLAGS: -std=c++11
#cgo darwin LDFLAGS: -L/opt/homebrew/opt/taglib/lib
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "taglib_wrapper.h"
*/
import "C"
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/navidrome/navidrome/log"
)

const iTunesKeyPrefix = "----:com.apple.itunes:"

func Version() string {
	return C.GoString(C.taglib_version())
}

func Read(filename string) (tags map[string][]string, err error) {
	// Do not crash on failures in the C code/library
	debug.SetPanicOnFault(true)
	defer func() {
		if r := recover(); r != nil {
			log.Error("extractor: recovered from panic when reading tags", "file", filename, "error", r)
			err = fmt.Errorf("extractor: recovered from panic: %s", r)
		}
	}()

	fp := getFilename(filename)
	defer C.free(unsafe.Pointer(fp))
	id, m, release := newMap()
	defer release()

	log.Trace("extractor: reading tags", "filename", filename, "map_id", id)
	res := C.taglib_read(fp, C.ulong(id))
	switch res {
	case C.TAGLIB_ERR_PARSE:
		// Check additional case whether the file is unreadable due to permission
		file, fileErr := os.OpenFile(filename, os.O_RDONLY, 0600)
		defer file.Close()

		if os.IsPermission(fileErr) {
			return nil, fmt.Errorf("navidrome does not have permission: %w", fileErr)
		} else if fileErr != nil {
			return nil, fmt.Errorf("cannot parse file media file: %w", fileErr)
		} else {
			return nil, fmt.Errorf("cannot parse file media file")
		}
	case C.TAGLIB_ERR_AUDIO_PROPS:
		return nil, fmt.Errorf("can't get audio properties from file")
	}
	if log.IsGreaterOrEqualTo(log.LevelDebug) {
		j, _ := json.Marshal(m)
		log.Trace("extractor: read tags", "tags", string(j), "filename", filename, "id", id)
	} else {
		log.Trace("extractor: read tags", "tags", m, "filename", filename, "id", id)
	}

	return m, nil
}

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

type tagMap map[string][]string

var allMaps sync.Map
var mapsNextID atomic.Uint32

func newMap() (uint32, tagMap, func()) {
	id := mapsNextID.Add(1)

	m := tagMap{}
	allMaps.Store(id, m)

	return id, m, func() {
		allMaps.Delete(id)
	}
}

func doPutTag(id C.ulong, key string, val *C.char) {
	if key == "" {
		return
	}

	r, _ := allMaps.Load(uint32(id))
	m := r.(tagMap)
	k := strings.ToLower(key)
	v := strings.TrimSpace(C.GoString(val))
	m[k] = append(m[k], v)
}

//export goPutM4AStr
func goPutM4AStr(id C.ulong, key *C.char, val *C.char) {
	k := C.GoString(key)

	// Special for M4A, do not catch keys that have no actual name
	k = strings.TrimPrefix(k, iTunesKeyPrefix)
	doPutTag(id, k, val)
}

//export goPutStr
func goPutStr(id C.ulong, key *C.char, val *C.char) {
	doPutTag(id, C.GoString(key), val)
}

//export goPutInt
func goPutInt(id C.ulong, key *C.char, val C.int) {
	valStr := strconv.Itoa(int(val))
	vp := C.CString(valStr)
	defer C.free(unsafe.Pointer(vp))
	goPutStr(id, key, vp)
}

//export goPutLyrics
func goPutLyrics(id C.ulong, lang *C.char, val *C.char) {
	doPutTag(id, "lyrics:"+C.GoString(lang), val)
}

//export goPutLyricLine
func goPutLyricLine(id C.ulong, lang *C.char, text *C.char, time C.int) {
	language := C.GoString(lang)
	line := C.GoString(text)
	timeGo := int64(time)

	ms := timeGo % 1000
	timeGo /= 1000
	sec := timeGo % 60
	timeGo /= 60
	minimum := timeGo % 60
	formattedLine := fmt.Sprintf("[%02d:%02d.%02d]%s\n", minimum, sec, ms/10, line)

	key := "lyrics:" + language

	r, _ := allMaps.Load(uint32(id))
	m := r.(tagMap)
	k := strings.ToLower(key)
	existing, ok := m[k]
	if ok {
		existing[0] += formattedLine
	} else {
		m[k] = []string{formattedLine}
	}
}
