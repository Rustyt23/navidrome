package rag

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/navidrome/navidrome/model"
)

const (
	DefaultIndexLimit = 50
	MaxIndexLimit     = 500
	indexPageSize     = 50
)

// Indexer defines a provider-independent document-indexing boundary.
type Indexer interface {
	Index(context.Context, []RAGDocument) error
}

// SongRepository is the read-only subset of Navidrome's media repository used
// by the RAG indexer.
type SongRepository interface {
	GetAll(options ...model.QueryOptions) (model.MediaFiles, error)
}

// PlaylistRepository is the read-only subset needed for optional playlist
// indexing. Loading tracks with refreshSmartPlaylist=false avoids mutations.
type PlaylistRepository interface {
	GetAll(options ...model.QueryOptions) (model.Playlists, error)
	GetWithTracks(id string, refreshSmartPlaylist, includeMissing bool) (*model.Playlist, error)
}

// VectorStore is the Qdrant subset used by the indexing pipeline.
type VectorStore interface {
	PointExists(ctx context.Context, logicalID string) (bool, error)
	UpsertPoint(ctx context.Context, logicalID string, vector []float32, payload map[string]any) error
}

// IndexResult is returned by the first bounded song indexing operation.
type IndexResult struct {
	Indexed int    `json:"indexed"`
	Skipped int    `json:"skipped"`
	Failed  int    `json:"failed"`
	Error   string `json:"error,omitempty"`
}

// IndexSongs indexes up to MaxIndexLimit new songs and writes only their
// derived vectors to the configured vector store. Existing points are skipped
// while the library is scanned in bounded pages. It never updates Navidrome
// records.
func IndexSongs(
	ctx context.Context,
	repository SongRepository,
	embedder Embedder,
	store VectorStore,
	limit int,
	force bool,
) (IndexResult, error) {
	result := IndexResult{}
	if limit <= 0 {
		limit = DefaultIndexLimit
	}
	if limit > MaxIndexLimit {
		return result, fmt.Errorf("index limit must not exceed %d", MaxIndexLimit)
	}

	for offset := 0; result.Indexed+result.Failed < limit; {
		pageSize := min(indexPageSize, limit-result.Indexed-result.Failed)
		songs, err := repository.GetAll(model.QueryOptions{
			Sort:   "id",
			Order:  "ASC",
			Max:    pageSize,
			Offset: offset,
		})
		if err != nil {
			return result, fmt.Errorf("could not read songs: %w", err)
		}
		if len(songs) == 0 {
			break
		}
		offset += len(songs)

		for i := range songs {
			song := songs[i]
			logicalID := StableSongPointID(song.ID)
			if !force {
				exists, err := store.PointExists(ctx, logicalID)
				if err != nil {
					recordIndexFailure(&result, fmt.Errorf("could not check song %q in Qdrant: %w", song.ID, err))
					if result.Indexed+result.Failed == limit {
						break
					}
					continue
				}
				if exists {
					result.Skipped++
					continue
				}
			}

			document := DocumentFromMediaFile(song)
			vector, err := embedder.EmbedText(ctx, document.Text)
			if err != nil {
				recordIndexFailure(&result, fmt.Errorf("could not embed song %q: %w", song.ID, err))
				if result.Indexed+result.Failed == limit {
					break
				}
				continue
			}
			if err := store.UpsertPoint(ctx, logicalID, vector, songPayload(song)); err != nil {
				recordIndexFailure(&result, fmt.Errorf("could not upsert song %q: %w", song.ID, err))
				if result.Indexed+result.Failed == limit {
					break
				}
				continue
			}
			result.Indexed++
			if result.Indexed+result.Failed == limit {
				break
			}
		}

		if len(songs) < pageSize {
			break
		}
	}

	return result, nil
}

// IndexPlaylists optionally indexes playlist-level documents alongside songs.
// Existing song indexing behavior is unchanged and no playlist is refreshed or
// modified by this operation.
func IndexPlaylists(
	ctx context.Context,
	repository PlaylistRepository,
	embedder Embedder,
	store VectorStore,
	limit int,
	force bool,
) (IndexResult, error) {
	result := IndexResult{}
	if limit <= 0 {
		limit = DefaultIndexLimit
	}
	if limit > MaxIndexLimit {
		return result, fmt.Errorf("index limit must not exceed %d", MaxIndexLimit)
	}

	for offset := 0; result.Indexed+result.Failed < limit; {
		pageSize := min(indexPageSize, limit-result.Indexed-result.Failed)
		playlists, err := repository.GetAll(model.QueryOptions{
			Sort: "id", Order: "ASC", Max: pageSize, Offset: offset,
		})
		if err != nil {
			return result, fmt.Errorf("could not read playlists: %w", err)
		}
		if len(playlists) == 0 {
			break
		}
		offset += len(playlists)

		for index := range playlists {
			playlistID := playlists[index].ID
			logicalID := StablePlaylistPointID(playlistID)
			if !force {
				exists, existsErr := store.PointExists(ctx, logicalID)
				if existsErr != nil {
					recordIndexFailure(&result, fmt.Errorf("could not check playlist %q in Qdrant: %w", playlistID, existsErr))
					continue
				}
				if exists {
					result.Skipped++
					continue
				}
			}

			playlist, loadErr := repository.GetWithTracks(playlistID, false, false)
			if loadErr != nil {
				recordIndexFailure(&result, fmt.Errorf("could not read playlist %q tracks: %w", playlistID, loadErr))
				continue
			}
			document := DocumentFromPlaylist(*playlist)
			vector, embedErr := embedder.EmbedText(ctx, document.Text)
			if embedErr != nil {
				recordIndexFailure(&result, fmt.Errorf("could not embed playlist %q: %w", playlistID, embedErr))
				continue
			}
			if upsertErr := store.UpsertPoint(ctx, logicalID, vector, playlistPayload(*playlist)); upsertErr != nil {
				recordIndexFailure(&result, fmt.Errorf("could not upsert playlist %q: %w", playlistID, upsertErr))
				continue
			}
			result.Indexed++
			if result.Indexed+result.Failed == limit {
				break
			}
		}
		if len(playlists) < pageSize {
			break
		}
	}
	return result, nil
}

func recordIndexFailure(result *IndexResult, err error) {
	result.Failed++
	if result.Error == "" && err != nil {
		result.Error = err.Error()
	}
}

func songPayload(song model.MediaFile) map[string]any {
	genre := strings.TrimSpace(song.Genre)
	genres := cleanTagValues(song.Tags.Values(model.TagGenre))
	if len(genres) > 0 {
		genre = strings.Join(genres, ", ")
	} else if len(song.Genres) > 0 {
		for _, item := range song.Genres {
			if value := strings.TrimSpace(item.Name); value != "" {
				genres = append(genres, value)
			}
		}
	} else if genre != "" {
		genres = []string{genre}
	}
	lufs, hasLUFS := songLUFSValue(song)
	moods := cleanTagValues(song.Tags.Values(model.TagMood))
	groupings := cleanTagValues(song.Tags.Values(model.TagGrouping))
	composers := cleanTagValues(song.Tags.Values(model.TagComposer))
	lyricists := cleanTagValues(song.Tags.Values(model.TagLyricist))
	recordLabels := cleanTagValues(song.Tags.Values(model.TagRecordLabel))
	isrc := cleanTagValues(song.Tags.Values(model.TagISRC))
	return map[string]any{
		"songId":             song.ID,
		"type":               "song",
		"libraryId":          song.LibraryID,
		"folderId":           song.FolderID,
		"title":              song.Title,
		"artist":             song.Artist,
		"artistId":           song.ArtistID,
		"album":              song.Album,
		"albumId":            song.AlbumID,
		"albumArtist":        song.AlbumArtist,
		"albumArtistId":      song.AlbumArtistID,
		"trackNumber":        song.TrackNumber,
		"discNumber":         song.DiscNumber,
		"discSubtitle":       song.DiscSubtitle,
		"compilation":        song.Compilation,
		"year":               song.Year,
		"date":               song.Date,
		"originalYear":       song.OriginalYear,
		"originalDate":       song.OriginalDate,
		"releaseYear":        song.ReleaseYear,
		"releaseDate":        song.ReleaseDate,
		"genre":              genre,
		"genres":             genres,
		"moods":              moods,
		"groupings":          groupings,
		"composers":          composers,
		"lyricists":          lyricists,
		"recordLabels":       recordLabels,
		"isrc":               isrc,
		"explicit":           song.ExplicitStatus == "e",
		"explicitStatus":     explicitStatusPayload(song.ExplicitStatus),
		"bpm":                song.BPM,
		"lufs":               lufs,
		"duration":           song.Duration,
		"playCount":          song.PlayCount,
		"lastPlayedAt":       optionalTime(song.PlayDate),
		"rating":             song.Rating,
		"ratedAt":            optionalTime(song.RatedAt),
		"starred":            song.Starred,
		"starredAt":          optionalTime(song.StarredAt),
		"averageRating":      song.AverageRating,
		"size":               song.Size,
		"suffix":             song.Suffix,
		"codec":              song.AudioCodec(),
		"bitRate":            song.BitRate,
		"sampleRate":         song.SampleRate,
		"bitDepth":           song.BitDepth,
		"channels":           song.Channels,
		"catalogNum":         song.CatalogNum,
		"mbzRecordingId":     song.MbzRecordingID,
		"mbzReleaseId":       song.MbzReleaseID,
		"mbzReleaseTrackId":  song.MbzReleaseTrackID,
		"mbzAlbumId":         song.MbzAlbumID,
		"mbzReleaseGroupId":  song.MbzReleaseGroupID,
		"mbzArtistId":        song.MbzArtistID,
		"mbzAlbumArtistId":   song.MbzAlbumArtistID,
		"mbzAlbumType":       song.MbzAlbumType,
		"spotifyConfidence":  song.SpotifyConfidence,
		"spotifyMatch":       song.SpotifyMatch,
		"spotifyArtist":      song.SpotifyArtist,
		"spotifyUrl":         song.SpotifyURL,
		"rgAlbumGain":        optionalFloat(song.RGAlbumGain),
		"rgAlbumPeak":        optionalFloat(song.RGAlbumPeak),
		"rgTrackGain":        optionalFloat(song.RGTrackGain),
		"rgTrackPeak":        optionalFloat(song.RGTrackPeak),
		"hasCoverArt":        song.HasCoverArt,
		"missing":            song.Missing,
		"createdAt":          optionalTimeValue(song.CreatedAt),
		"updatedAt":          optionalTimeValue(song.UpdatedAt),
		"hasLyrics":          songHasLyrics(song),
		"hasGenre":           strings.TrimSpace(genre) != "",
		"hasMood":            len(moods) > 0,
		"hasYear":            song.Year > 0,
		"hasDate":            strings.TrimSpace(song.Date) != "",
		"hasBpm":             song.BPM > 0,
		"hasLufs":            hasLUFS,
		"hasReplayGain":      song.RGAlbumGain != nil || song.RGTrackGain != nil,
		"hasMusicBrainzIds":  song.MbzRecordingID != "" || song.MbzReleaseID != "",
		"hasSpotifyMetadata": song.SpotifyURL != "" || song.SpotifyMatch != "",
	}
}

func playlistPayload(playlist model.Playlist) map[string]any {
	tracks := playlist.MediaFiles()
	stats := summarizePlaylist(tracks)
	genres := make([]string, 0, len(stats.Genres))
	for _, item := range stats.Genres {
		genres = append(genres, item.Name)
	}
	artists := make([]string, 0, len(stats.Artists))
	for _, item := range stats.Artists {
		artists = append(artists, item.Name)
	}
	moods := make([]string, 0, len(stats.Moods))
	for _, item := range stats.Moods {
		moods = append(moods, item.Name)
	}
	return map[string]any{
		"type":          "playlist",
		"playlistId":    playlist.ID,
		"name":          playlist.Name,
		"owner":         playlist.OwnerName,
		"ownerId":       playlist.OwnerID,
		"comment":       playlist.Comment,
		"public":        playlist.Public,
		"songCount":     len(tracks),
		"duration":      playlistDuration(&playlist, tracks),
		"genres":        genres,
		"moods":         moods,
		"artists":       artists,
		"explicitCount": len(stats.ExplicitSongs),
		"bpmAverage":    stats.BPM.Average,
		"bpmMin":        stats.BPM.Min,
		"bpmMax":        stats.BPM.Max,
		"lufsAverage":   stats.LUFS.Average,
		"lufsMin":       stats.LUFS.Min,
		"lufsMax":       stats.LUFS.Max,
		"createdAt":     optionalTimeValue(playlist.CreatedAt),
		"updatedAt":     optionalTimeValue(playlist.UpdatedAt),
	}
}

func cleanTagValues(values []string) []string {
	clean := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		clean = append(clean, value)
	}
	return clean
}

func explicitStatusPayload(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "e", "explicit":
		return "explicit"
	case "c", "clean":
		return "clean"
	default:
		return "unknown"
	}
}

func optionalTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
}

func optionalTimeValue(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}

func optionalFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func songLUFS(song model.MediaFile) float64 {
	value, _ := songLUFSValue(song)
	return value
}

func songLUFSValue(song model.MediaFile) (float64, bool) {
	for _, name := range []model.TagName{
		"loudnorm_final_lufs",
		"final_lufs",
		"finallufs",
		"lufs",
	} {
		for _, value := range song.Tags.Values(name) {
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func songHasLyrics(song model.MediaFile) bool {
	lyrics, err := song.StructuredLyrics()
	if err != nil {
		return false
	}
	for _, lyric := range lyrics {
		if !lyric.IsEmpty() {
			return true
		}
	}
	return false
}

var _ VectorStore = (*QdrantClient)(nil)
