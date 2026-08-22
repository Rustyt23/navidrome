package nativeapi

import (
	"testing"
)

func TestDownloadFilename(t *testing.T) {
	// Singular, because "navidrome-1-songs.zip" is the kind of detail that makes
	// a deliverable look unfinished.
	if got := downloadFilename(1); got != "navidrome-1-song.zip" {
		t.Errorf("one song: got %q", got)
	}
	if got := downloadFilename(12); got != "navidrome-12-songs.zip" {
		t.Errorf("many songs: got %q", got)
	}
}

func TestFirstOf(t *testing.T) {
	// Query values repeat, and a browser can send an empty one before a real
	// one. Taking [0] blindly would read format="" and silently transcode.
	if v, ok := firstOf([]string{"", "raw"}); !ok || v != "raw" {
		t.Errorf("skipping blanks: got %q %v", v, ok)
	}
	if v, ok := firstOf([]string{"  "}); ok || v != "" {
		t.Errorf("whitespace is not a value: got %q %v", v, ok)
	}
	if _, ok := firstOf(nil); ok {
		t.Error("nil should report no value")
	}
}
