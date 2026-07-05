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

func TestDocumentFromPlaylist(t *testing.T) {
	playlist := model.Playlist{
		ID: "playlist-1", Name: "Store Energy", OwnerName: "DJ", Comment: "Morning rotation",
		Tracks: model.PlaylistTracks{
			{MediaFile: model.MediaFile{ID: "song-1", Title: "Bright", Artist: "Artist A", Genre: "Pop", BPM: 120, Duration: 200, ExplicitStatus: "c", Tags: model.Tags{"lufs": {"-12"}}}},
			{MediaFile: model.MediaFile{ID: "song-2", Title: "Drive", Artist: "Artist B", Genre: "Dance", BPM: 124, Duration: 220, ExplicitStatus: "e", Tags: model.Tags{"lufs": {"-11"}}}},
		},
	}

	document := DocumentFromPlaylist(playlist)
	if document.ID != "playlist:playlist-1" {
		t.Fatalf("unexpected stable document ID: %q", document.ID)
	}
	for _, expected := range []string{
		"Playlist: Store Energy", "Owner: DJ", "Comment: Morning rotation",
		"Song count: 2", "Duration: 420 seconds", "Genres: Dance (1), Pop (1)",
		"Artists: Artist A (1), Artist B (1)", "Explicit summary: 1 explicit",
		"BPM summary:", "LUFS summary:", "1. Bright — Artist A", "2. Drive — Artist B",
	} {
		if !strings.Contains(document.Text, expected) {
			t.Errorf("expected playlist document to contain %q; got:\n%s", expected, document.Text)
		}
	}
	if document.Metadata["type"] != "playlist" || document.Metadata["playlistId"] != playlist.ID {
		t.Fatalf("unexpected playlist metadata: %#v", document.Metadata)
	}
}
