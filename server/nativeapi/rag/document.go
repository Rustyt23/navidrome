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

// DocumentFromPlaylist converts a playlist snapshot into deterministic text
// suitable for semantic retrieval. It only reads the supplied playlist.
func DocumentFromPlaylist(playlist model.Playlist) RAGDocument {
	tracks := playlist.MediaFiles()
	stats := summarizePlaylist(tracks)
	lines := make([]string, 0, 12+len(tracks))
	appendTextLine(&lines, "Playlist", playlist.Name)
	appendTextLine(&lines, "Owner", playlist.OwnerName)
	appendTextLine(&lines, "Comment", playlist.Comment)
	lines = append(lines, fmt.Sprintf("Song count: %d", len(tracks)))
	lines = append(lines, fmt.Sprintf("Duration: %.0f seconds", playlistDuration(&playlist, tracks)))
	appendSummaryLine(&lines, "Genres", stats.Genres)
	appendSummaryLine(&lines, "Artists", stats.Artists)
	lines = append(lines, fmt.Sprintf(
		"Explicit summary: %d explicit, %d non-explicit or unknown",
		len(stats.ExplicitSongs),
		max(len(tracks)-len(stats.ExplicitSongs), 0),
	))
	lines = append(lines, numericSummaryLine("BPM", stats.BPM))
	lines = append(lines, numericSummaryLine("LUFS", stats.LUFS))
	lines = append(lines, "Song list summary:")
	for index, song := range tracks {
		if index == 100 {
			lines = append(lines, fmt.Sprintf("... and %d more songs", len(tracks)-index))
			break
		}
		genre := strings.TrimSpace(song.Genre)
		lufs, hasLUFS := songLUFSValue(song)
		profile := make([]string, 0, 3)
		if genre != "" {
			profile = append(profile, "genre "+genre)
		}
		if song.BPM > 0 {
			profile = append(profile, fmt.Sprintf("%d BPM", song.BPM))
		}
		if hasLUFS {
			profile = append(profile, fmt.Sprintf("%.1f LUFS", lufs))
		}
		line := fmt.Sprintf("%d. %s — %s", index+1, song.Title, song.Artist)
		if len(profile) > 0 {
			line += " (" + strings.Join(profile, ", ") + ")"
		}
		lines = append(lines, line)
	}

	return RAGDocument{
		ID:   StablePlaylistPointID(playlist.ID),
		Text: strings.Join(lines, "\n"),
		Metadata: map[string]any{
			"source":     "playlist",
			"type":       "playlist",
			"playlistId": playlist.ID,
			"ownerId":    playlist.OwnerID,
		},
	}
}

func appendSummaryLine(lines *[]string, label string, items []PlaylistSummaryItem) {
	if len(items) == 0 {
		*lines = append(*lines, label+": unknown")
		return
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, fmt.Sprintf("%s (%d)", item.Name, item.Count))
	}
	*lines = append(*lines, label+": "+strings.Join(values, ", "))
}

func numericSummaryLine(label string, summary playlistNumericSummary) string {
	if summary.Count == 0 {
		return fmt.Sprintf("%s summary: unavailable (%d missing)", label, summary.Missing)
	}
	return fmt.Sprintf(
		"%s summary: average %.1f, median %.1f, range %.1f to %.1f (%d known, %d missing)",
		label, summary.Average, summary.Median, summary.Min, summary.Max, summary.Count, summary.Missing,
	)
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
