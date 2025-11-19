package taglib

import "C"

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/storage/local"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model/metadata"
)

type extractor struct {
	baseDir string
}

func (e extractor) Parse(files ...string) (map[string]metadata.Info, error) {
	results := make(map[string]metadata.Info)
	for _, path := range files {
		props, err := e.extractMetadata(path)
		if err != nil {
			continue
		}
		results[path] = *props
	}
	return results, nil
}

func (e extractor) Version() string {
	return Version()
}

func (e extractor) extractMetadata(filePath string) (*metadata.Info, error) {
	fullPath := filepath.Join(e.baseDir, filePath)
	tags, err := Read(fullPath)
	if err != nil {
		log.Warn("extractor: Error reading metadata from file. Skipping", "filePath", fullPath, err)
		return nil, err
	}

	// Parse audio properties
	ap := metadata.AudioProperties{}
	if length, ok := tags["_lengthinmilliseconds"]; ok && len(length) > 0 {
		millis, _ := strconv.Atoi(length[0])
		if millis > 0 {
			ap.Duration = (time.Millisecond * time.Duration(millis)).Round(time.Millisecond * 10)
		}
		delete(tags, "_lengthinmilliseconds")
	}
	parseProp := func(prop string, target *int) {
		if value, ok := tags[prop]; ok && len(value) > 0 {
			*target, _ = strconv.Atoi(value[0])
			delete(tags, prop)
		}
	}
	parseProp("_bitrate", &ap.BitRate)
	parseProp("_channels", &ap.Channels)
	parseProp("_samplerate", &ap.SampleRate)
	parseProp("_bitspersample", &ap.BitDepth)

	// Parse track/disc totals
	parseTuple := func(prop string) {
		tagName := prop + "number"
		tagTotal := prop + "total"
		if value, ok := tags[tagName]; ok && len(value) > 0 {
			parts := strings.Split(value[0], "/")
			tags[tagName] = []string{parts[0]}
			if len(parts) == 2 {
				tags[tagTotal] = []string{parts[1]}
			}
		}
	}
	parseTuple("track")
	parseTuple("disc")

	// Adjust some ID3 tags
	parseLyrics(tags)
	parseTIPL(tags)
	delete(tags, "tmcl") // TMCL is already parsed by TagLib

	return &metadata.Info{
		Tags:            tags,
		AudioProperties: ap,
		HasPicture:      tags["has_picture"] != nil && len(tags["has_picture"]) > 0 && tags["has_picture"][0] == "true",
	}, nil
}

func WriteComment(filename string, comment string) (err error) {
	debug.SetPanicOnFault(true)
	defer func() {
		if r := recover(); r != nil {
			log.Error("extractor: recovered from panic when writing comment", "file", filename, "error", r)
			err = fmt.Errorf("extractor: recovered from panic: %v", r)
		}
	}()

	fp := getFilename(filename)
	defer C.free(unsafe.Pointer(fp))

	cmt := C.CString(comment)
	defer C.free(unsafe.Pointer(cmt))

	res := C.taglib_write_comment(fp, cmt)
	switch res {
	case 0:
		return nil
	case C.TAGLIB_ERR_PARSE:
		return fmt.Errorf("taglib: cannot parse file %s", filename)
	case C.TAGLIB_ERR_AUDIO_PROPS:
		return fmt.Errorf("taglib: cannot write tags to file %s", filename)
	default:
		return fmt.Errorf("taglib: unexpected error writing comment to file %s", filename)
	}
}

// parseLyrics make sure lyrics tags have language
func parseLyrics(tags map[string][]string) {
	lyrics := tags["lyrics"]
	if len(lyrics) > 0 {
		tags["lyrics:xxx"] = lyrics
		delete(tags, "lyrics")
	}
}

// These are the only roles we support, based on Picard's tag map:
// https://picard-docs.musicbrainz.org/downloads/MusicBrainz_Picard_Tag_Map.html
var tiplMapping = map[string]string{
	"arranger": "arranger",
	"engineer": "engineer",
	"producer": "producer",
	"mix":      "mixer",
	"DJ-mix":   "djmixer",
}

// parseTIPL parses the ID3v2.4 TIPL frame string, which is received from TagLib in the format:
//
//	"arranger Andrew Powell engineer Chris Blair engineer Pat Stapley producer Eric Woolfson".
//
// and breaks it down into a map of roles and names, e.g.:
//
//	{"arranger": ["Andrew Powell"], "engineer": ["Chris Blair", "Pat Stapley"], "producer": ["Eric Woolfson"]}.
func parseTIPL(tags map[string][]string) {
	tipl := tags["tipl"]
	if len(tipl) == 0 {
		return
	}

	addRole := func(currentRole string, currentValue []string) {
		if currentRole != "" && len(currentValue) > 0 {
			role := tiplMapping[currentRole]
			tags[role] = append(tags[role], strings.Join(currentValue, " "))
		}
	}

	var currentRole string
	var currentValue []string
	for _, part := range strings.Split(tipl[0], " ") {
		if _, ok := tiplMapping[part]; ok {
			addRole(currentRole, currentValue)
			currentRole = part
			currentValue = nil
			continue
		}
		currentValue = append(currentValue, part)
	}
	addRole(currentRole, currentValue)
	delete(tags, "tipl")
}

var _ local.Extractor = (*extractor)(nil)

func init() {
	local.RegisterExtractor("taglib", func(_ fs.FS, baseDir string) local.Extractor {
		// ignores fs, as taglib extractor only works with local files
		return &extractor{baseDir}
	})
	conf.AddHook(func() {
		log.Debug("TagLib version", "version", Version())
	})
}
