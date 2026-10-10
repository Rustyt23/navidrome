package gcpsync

import (
	"path/filepath"
	"strings"
	"unicode"
)

// Names are derived from artist/title metadata for the flat GCS bucket.
// Conforms and WellFormedName remain available for UI diagnostics; upload
// eligibility never depends on the shape of the existing filename.

const nameSeparator = " - "

// ExpectedName is the filename a track must have for its bucket object to be
// reachable from a playlist: "Artist - Title.mp3".
func ExpectedName(artist, title string) string {
	artist = sanitizeNamePart(artist)
	title = sanitizeNamePart(title)
	if artist == "" || title == "" {
		return ""
	}
	return artist + nameSeparator + title + ".mp3"
}

// Conforms reports whether the file at path is already named after its artist
// and title. An empty expected name means the row is too incomplete to judge,
// which counts as conforming: the fix for a track with no artist tag is to tag
// it, not to rename it to something meaningless.
func Conforms(path, artist, title string) bool {
	expected := ExpectedName(artist, title)
	if expected == "" {
		return true
	}
	return strings.EqualFold(filepath.Base(path), expected)
}

// WellFormedName reports whether a filename has the "Artist - Title.mp3" shape,
// without knowing what the artist or title should be. Non-MP3 files are not
// judged - the convention is about tracks.
func WellFormedName(path string) bool {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if !strings.EqualFold(ext, ".mp3") {
		return true
	}
	stem := strings.TrimSuffix(base, ext)
	artist, title, found := strings.Cut(stem, nameSeparator)
	if !found {
		return false
	}
	// Split on the *first* separator: "A - B - C.mp3" is a title that itself
	// contains a dash, which is common and perfectly resolvable.
	return strings.TrimSpace(artist) != "" && strings.TrimSpace(title) != ""
}

// sanitizeNamePart makes a tag value safe to use as one half of a filename:
// path separators would create directories, control characters break tooling,
// and runs of whitespace make two names that look identical compare unequal.
func sanitizeNamePart(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '/' || r == '\\' || r == ':':
			return '_'
		case unicode.IsControl(r):
			return ' '
		default:
			return r
		}
	}, s)
	return strings.Join(strings.Fields(s), " ")
}
