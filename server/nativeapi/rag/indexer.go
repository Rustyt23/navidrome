package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	embedBatchSize    = 32
	// syncScanCap bounds a full-library sync so a runaway library can never loop
	// unbounded.
	syncScanCap = 200000
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

// IndexedSongRepository can reload the exact Navidrome songs referenced by
// existing Qdrant points. Refreshing must use those IDs instead of walking the
// first page of the library, otherwise unrelated songs can be indexed while
// the records the user asked to refresh remain stale.
type IndexedSongRepository interface {
	SongRepository
	Get(id string) (*model.MediaFile, error)
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
	UpsertPoints(ctx context.Context, points []PointUpsert) error
	ExistingContentHashes(ctx context.Context, logicalIDs []string) (map[string]string, error)
	DeletePoints(ctx context.Context, logicalIDs []string) error
	AllIndexedSongIDs(ctx context.Context, maxTotal int) ([]string, error)
}

// IndexResult is returned by the bounded song indexing operation.
type IndexResult struct {
	Indexed int    `json:"indexed"`
	Skipped int    `json:"skipped"`
	Failed  int    `json:"failed"`
	Error   string `json:"error,omitempty"`
}

// SyncResult extends IndexResult with the number of orphaned points removed when
// syncing the whole library.
type SyncResult struct {
	IndexResult
	Deleted int `json:"deleted"`
}

// IndexSongs indexes up to `limit` songs that are new or whose content changed
// since they were last indexed, embedding them in batches and writing only their
// derived vectors and payloads. Unchanged songs are skipped via a bulk content
// hash comparison. It never updates Navidrome records.
func IndexSongs(
	ctx context.Context,
	repository SongRepository,
	embedder Embedder,
	store VectorStore,
	limit int,
	force bool,
	embedderTag string,
) (IndexResult, error) {
	result := IndexResult{}
	if limit <= 0 {
		limit = DefaultIndexLimit
	}
	if limit > MaxIndexLimit {
		return result, fmt.Errorf("index limit must not exceed %d", MaxIndexLimit)
	}

	for offset := 0; result.Indexed+result.Failed < limit; {
		songs, err := repository.GetAll(model.QueryOptions{
			Sort: "id", Order: "ASC", Max: indexPageSize, Offset: offset,
		})
		if err != nil {
			return result, fmt.Errorf("could not read songs: %w", err)
		}
		if len(songs) == 0 {
			break
		}
		offset += len(songs)

		budget := limit - (result.Indexed + result.Failed)
		if err := indexSongPage(ctx, songs, embedder, store, force, embedderTag, budget, &result); err != nil {
			return result, err
		}
		if len(songs) < indexPageSize {
			break
		}
	}

	return result, nil
}

// RefreshIndexedSongs reloads up to limit songs that are already present in
// Qdrant and force-upserts their latest Navidrome document, vector, and payload.
// This updates metadata, lyrics, filter fields, and other indexed information
// without adding unrelated library songs.
func RefreshIndexedSongs(
	ctx context.Context,
	repository IndexedSongRepository,
	embedder Embedder,
	store VectorStore,
	limit int,
	embedderTag string,
) (IndexResult, error) {
	result := IndexResult{}
	if limit <= 0 {
		limit = DefaultIndexLimit
	}
	if limit > MaxIndexLimit {
		return result, fmt.Errorf("index limit must not exceed %d", MaxIndexLimit)
	}

	logicalIDs, err := store.AllIndexedSongIDs(ctx, limit)
	if err != nil {
		return result, fmt.Errorf("could not list indexed songs: %w", err)
	}
	if len(logicalIDs) > limit {
		logicalIDs = logicalIDs[:limit]
	}

	songs := make(model.MediaFiles, 0, len(logicalIDs))
	for _, logicalID := range logicalIDs {
		songID, ok := strings.CutPrefix(strings.TrimSpace(logicalID), "song:")
		if !ok || strings.TrimSpace(songID) == "" {
			recordIndexFailure(&result, fmt.Errorf("invalid indexed song ID %q", logicalID))
			continue
		}
		song, loadErr := repository.Get(songID)
		if loadErr != nil {
			recordIndexFailure(&result, fmt.Errorf("could not read indexed song %q: %w", songID, loadErr))
			continue
		}
		songs = append(songs, *song)
	}

	if len(songs) > 0 {
		if err := indexSongPage(ctx, songs, embedder, store, true, embedderTag, len(songs), &result); err != nil {
			return result, err
		}
	}
	return result, nil
}

// IndexSongsWithLyrics scans the Navidrome library for songs with fetched
// lyrics and force-upserts their latest Qdrant points. The forced write also
// backfills older points whose content hash is current but whose payload was
// created before lyricsText was stored. Songs without lyrics are ignored.
func IndexSongsWithLyrics(
	ctx context.Context,
	repository SongRepository,
	embedder Embedder,
	store VectorStore,
	limit int,
	embedderTag string,
) (IndexResult, error) {
	result := IndexResult{}
	if limit <= 0 {
		limit = MaxIndexLimit
	}
	if limit > MaxIndexLimit {
		return result, fmt.Errorf("index limit must not exceed %d", MaxIndexLimit)
	}

	for offset := 0; result.Indexed+result.Failed < limit; {
		songs, err := repository.GetAll(model.QueryOptions{
			Sort: "id", Order: "ASC", Max: indexPageSize, Offset: offset,
		})
		if err != nil {
			return result, fmt.Errorf("could not read songs: %w", err)
		}
		if len(songs) == 0 {
			break
		}
		offset += len(songs)

		withLyrics := make(model.MediaFiles, 0, len(songs))
		for index := range songs {
			if strings.TrimSpace(LyricsText(songs[index])) != "" {
				withLyrics = append(withLyrics, songs[index])
			}
		}
		if len(withLyrics) > 0 {
			budget := limit - (result.Indexed + result.Failed)
			if err := indexSongPage(ctx, withLyrics, embedder, store, true, embedderTag, budget, &result); err != nil {
				return result, err
			}
		}
		if len(songs) < indexPageSize {
			break
		}
	}

	return result, nil
}

// SyncSongs brings the vector store in line with the current library: it indexes
// new and changed songs across the whole library and removes points for songs
// that no longer exist. It is the operation to run after a library scan.
func SyncSongs(
	ctx context.Context,
	repository SongRepository,
	embedder Embedder,
	store VectorStore,
	embedderTag string,
) (SyncResult, error) {
	result := SyncResult{}
	currentIDs := make(map[string]struct{}, 1024)

	for offset := 0; offset < syncScanCap; {
		songs, err := repository.GetAll(model.QueryOptions{
			Sort: "id", Order: "ASC", Max: indexPageSize, Offset: offset,
		})
		if err != nil {
			return result, fmt.Errorf("could not read songs: %w", err)
		}
		if len(songs) == 0 {
			break
		}
		offset += len(songs)

		for i := range songs {
			currentIDs[StableSongPointID(songs[i].ID)] = struct{}{}
		}
		if err := indexSongPage(ctx, songs, embedder, store, false, embedderTag, len(songs), &result.IndexResult); err != nil {
			return result, err
		}
		if len(songs) < indexPageSize {
			break
		}
	}

	// Remove points whose songs are no longer in the library.
	indexed, err := store.AllIndexedSongIDs(ctx, syncScanCap)
	if err != nil {
		return result, fmt.Errorf("could not list indexed songs: %w", err)
	}
	orphans := make([]string, 0)
	for _, logicalID := range indexed {
		if _, ok := currentIDs[logicalID]; !ok {
			orphans = append(orphans, logicalID)
		}
	}
	if len(orphans) > 0 {
		if err := store.DeletePoints(ctx, orphans); err != nil {
			return result, fmt.Errorf("could not delete orphaned points: %w", err)
		}
		result.Deleted = len(orphans)
	}
	return result, nil
}

// indexSongPage embeds and upserts up to `budget` new/changed songs from a page,
// skipping unchanged ones via a bulk content-hash comparison.
func indexSongPage(
	ctx context.Context,
	songs model.MediaFiles,
	embedder Embedder,
	store VectorStore,
	force bool,
	embedderTag string,
	budget int,
	result *IndexResult,
) error {
	logicalIDs := make([]string, len(songs))
	hashes := make([]string, len(songs))
	for i := range songs {
		logicalIDs[i] = StableSongPointID(songs[i].ID)
		hashes[i] = songContentHash(songs[i], embedderTag)
	}

	existing := map[string]string{}
	if !force {
		found, err := store.ExistingContentHashes(ctx, logicalIDs)
		if err != nil {
			return fmt.Errorf("could not check indexed songs in Qdrant: %w", err)
		}
		existing = found
	}

	pending := make([]int, 0, len(songs))
	for i := range songs {
		if len(pending) >= budget {
			break
		}
		if !force {
			if hash, ok := existing[logicalIDs[i]]; ok && hash == hashes[i] {
				result.Skipped++
				continue
			}
		}
		pending = append(pending, i)
	}

	buildPoint := func(idx int, vector []float32) PointUpsert {
		payload := songPayload(songs[idx])
		payload["contentHash"] = hashes[idx]
		payload["embeddingModel"] = embedderTag
		return PointUpsert{LogicalID: logicalIDs[idx], Vector: vector, Payload: payload}
	}

	for start := 0; start < len(pending); start += embedBatchSize {
		end := min(start+embedBatchSize, len(pending))
		batch := pending[start:end]

		texts := make([]string, len(batch))
		for j, idx := range batch {
			texts[j] = DocumentFromMediaFile(songs[idx]).Text
		}
		vectors, err := embedder.EmbedTexts(ctx, texts)
		if err != nil || len(vectors) != len(batch) {
			// A batch failure is usually one bad item; isolate it so the rest of
			// the batch still gets indexed.
			indexSongsIndividually(ctx, songs, embedder, store, batch, texts, buildPoint, result)
			continue
		}
		points := make([]PointUpsert, 0, len(batch))
		for j, idx := range batch {
			points = append(points, buildPoint(idx, vectors[j]))
		}
		if err := store.UpsertPoints(ctx, points); err != nil {
			indexSongsIndividually(ctx, songs, embedder, store, batch, texts, buildPoint, result)
			continue
		}
		result.Indexed += len(points)
	}
	return nil
}

func indexSongsIndividually(
	ctx context.Context,
	songs model.MediaFiles,
	embedder Embedder,
	store VectorStore,
	batch []int,
	texts []string,
	buildPoint func(int, []float32) PointUpsert,
	result *IndexResult,
) {
	for j, idx := range batch {
		vector, err := embedder.EmbedText(ctx, texts[j])
		if err != nil {
			recordIndexFailure(result, fmt.Errorf("could not embed song %q: %w", songs[idx].ID, err))
			continue
		}
		if err := store.UpsertPoints(ctx, []PointUpsert{buildPoint(idx, vector)}); err != nil {
			recordIndexFailure(result, fmt.Errorf("could not upsert song %q: %w", songs[idx].ID, err))
			continue
		}
		result.Indexed++
	}
}

// songContentHash produces a stable fingerprint of the fields that affect a
// song's embedding and its filterable payload. It intentionally excludes
// volatile fields (play count, last played) so a song is only re-embedded when
// its actual metadata — or the embedding model — changes.
func songContentHash(song model.MediaFile, embedderTag string) string {
	var b strings.Builder
	b.WriteString(embedderTag)
	b.WriteByte('\n')
	b.WriteString(DocumentFromMediaFile(song).Text)
	fmt.Fprintf(&b, "\nbpm=%d;explicit=%s;", song.BPM, song.ExplicitStatus)
	if lufs, ok := songLUFSValue(song); ok {
		fmt.Fprintf(&b, "lufs=%.2f;", lufs)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
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
		"lyricsText":         LyricsText(song),
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
