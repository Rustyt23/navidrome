package rag

import (
	"strings"
	"testing"

	"github.com/navidrome/navidrome/model"
)

func TestDocumentFromMediaFile(t *testing.T) {
	song := model.MediaFile{
		ID:             "song-1",
		Title:          "A Song",
		Artist:         "An Artist",
		ArtistID:       "artist-1",
		Album:          "An Album",
		AlbumID:        "album-1",
		AlbumArtist:    "Album Artist",
		Year:           2024,
		TrackNumber:    2,
		Genre:          "Fallback genre",
		ExplicitStatus: "c",
		Comment:        "A useful comment",
		Tags: model.Tags{
			model.TagGenre: {"Rock", "Alternative"},
			model.TagMood:  {"Energetic"},
		},
		Lyrics: `[{"lang":"en","line":[{"value":"First line"},{"value":"Second line"}],"synced":false}]`,
	}

	document := DocumentFromMediaFile(song)

	if document.ID != song.ID {
		t.Fatalf("expected ID %q, got %q", song.ID, document.ID)
	}
	for _, expected := range []string{
		"Title: A Song",
		"Artist: An Artist",
		"Album: An Album",
		"Year: 2024",
		"Track: 2",
		"Genre: Rock, Alternative",
		"Mood: Energetic",
		"Comment: A useful comment",
		"Explicit status: Clean",
		"Lyrics:\nFirst line\nSecond line",
	} {
		if !strings.Contains(document.Text, expected) {
			t.Errorf("expected document text to contain %q; got:\n%s", expected, document.Text)
		}
	}
	if document.Metadata["source"] != "song" || document.Metadata["songId"] != song.ID {
		t.Fatalf("unexpected metadata: %#v", document.Metadata)
	}
}

func TestDocumentFromMediaFileIgnoresInvalidLyrics(t *testing.T) {
	document := DocumentFromMediaFile(model.MediaFile{ID: "song-1", Title: "A Song", Lyrics: "not-json"})

	if strings.Contains(document.Text, "Lyrics:") {
		t.Fatalf("expected invalid lyrics to be ignored; got:\n%s", document.Text)
	}
}
