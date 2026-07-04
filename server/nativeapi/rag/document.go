// Package rag contains the data structures used to prepare Navidrome data for
// retrieval-augmented generation. Phase 1 only builds readable documents; it
// does not index them or send them to an external service.
package rag

import (
	"fmt"
	"strings"

	"github.com/navidrome/navidrome/model"
)

// RAGDocument is a provider-independent representation of content that may be
// indexed by a future RAG implementation.
type RAGDocument struct {
	ID       string         `json:"id"`
	Text     string         `json:"text"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// DocumentFromMediaFile converts a song into deterministic, human-readable
// text without modifying the media file or contacting an external service.
func DocumentFromMediaFile(song model.MediaFile) RAGDocument {
	lines := make([]string, 0, 12)
	appendTextLine(&lines, "Title", song.Title)
	appendTextLine(&lines, "Artist", song.Artist)
	appendTextLine(&lines, "Album", song.Album)
	appendTextLine(&lines, "Album artist", song.AlbumArtist)

	if song.Year > 0 {
		lines = append(lines, fmt.Sprintf("Year: %d", song.Year))
	}
	if song.TrackNumber > 0 {
		lines = append(lines, fmt.Sprintf("Track: %d", song.TrackNumber))
	}
	if song.DiscNumber > 0 {
		lines = append(lines, fmt.Sprintf("Disc: %d", song.DiscNumber))
	}

	genres := song.Tags.Values(model.TagGenre)
	if len(genres) == 0 && strings.TrimSpace(song.Genre) != "" {
		genres = []string{song.Genre}
	}
	appendValuesLine(&lines, "Genre", genres)
	appendValuesLine(&lines, "Mood", song.Tags.Values(model.TagMood))
	appendValuesLine(&lines, "Grouping", song.Tags.Values(model.TagGrouping))
	appendTextLine(&lines, "Comment", song.Comment)

	switch song.ExplicitStatus {
	case "c":
		lines = append(lines, "Explicit status: Clean")
	case "e":
		lines = append(lines, "Explicit status: Explicit")
	}

	appendLyrics(&lines, song)

	return RAGDocument{
		ID:   song.ID,
		Text: strings.Join(lines, "\n"),
		Metadata: map[string]any{
			"source":   "song",
			"songId":   song.ID,
			"albumId":  song.AlbumID,
			"artistId": song.ArtistID,
		},
	}
}

func appendTextLine(lines *[]string, label, value string) {
	if value = strings.TrimSpace(value); value != "" {
		*lines = append(*lines, label+": "+value)
	}
}

func appendValuesLine(lines *[]string, label string, values []string) {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			clean = append(clean, value)
		}
	}
	if len(clean) > 0 {
		*lines = append(*lines, label+": "+strings.Join(clean, ", "))
	}
}

func appendLyrics(lines *[]string, song model.MediaFile) {
	lyrics, err := song.StructuredLyrics()
	if err != nil {
		return
	}

	values := make([]string, 0)
	for _, lyric := range lyrics {
		for _, line := range lyric.Line {
			if value := strings.TrimSpace(line.Value); value != "" {
				values = append(values, value)
			}
		}
	}
	if len(values) > 0 {
		*lines = append(*lines, "Lyrics:\n"+strings.Join(values, "\n"))
	}
}
