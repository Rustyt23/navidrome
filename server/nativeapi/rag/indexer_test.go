package rag

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/navidrome/navidrome/model"
)

type fakeSongRepository struct {
	songs   model.MediaFiles
	options []model.QueryOptions
}

func (f *fakeSongRepository) GetAll(options ...model.QueryOptions) (model.MediaFiles, error) {
	option := model.QueryOptions{}
	if len(options) > 0 {
		option = options[0]
		f.options = append(f.options, option)
	}
	if option.Offset >= len(f.songs) {
		return nil, nil
	}
	end := min(option.Offset+option.Max, len(f.songs))
	return f.songs[option.Offset:end], nil
}

type fakeEmbedder struct{}

func (fakeEmbedder) EmbedText(_ context.Context, text string) ([]float32, error) {
	if text == "Title: Fails embedding" {
		return nil, errors.New("embedding failed")
	}
	return []float32{0.1, 0.2}, nil
}

func (e fakeEmbedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, 0, len(texts))
	for _, text := range texts {
		vector, err := e.EmbedText(ctx, text)
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, vector)
	}
	return vectors, nil
}

type fakeVectorStore struct {
	existing map[string]bool
	hashes   map[string]string
	upserts  []string
	payloads []map[string]any
	deleted  []string
}

type fakePlaylistRepository struct {
	playlists model.Playlists
	loaded    map[string]*model.Playlist
}

func (f *fakePlaylistRepository) GetAll(options ...model.QueryOptions) (model.Playlists, error) {
	option := model.QueryOptions{}
	if len(options) > 0 {
		option = options[0]
	}
	if option.Offset >= len(f.playlists) {
		return nil, nil
	}
	end := min(option.Offset+option.Max, len(f.playlists))
	return f.playlists[option.Offset:end], nil
}

func (f *fakePlaylistRepository) GetWithTracks(id string, refresh, includeMissing bool) (*model.Playlist, error) {
	if refresh || includeMissing {
		return nil, errors.New("playlist indexing must use read-only load flags")
	}
	return f.loaded[id], nil
}

func (f *fakeVectorStore) PointExists(_ context.Context, logicalID string) (bool, error) {
	return f.existing[logicalID], nil
}

func (f *fakeVectorStore) UpsertPoint(_ context.Context, logicalID string, _ []float32, payload map[string]any) error {
	f.upserts = append(f.upserts, logicalID)
	f.payloads = append(f.payloads, payload)
	return nil
}

func (f *fakeVectorStore) UpsertPoints(_ context.Context, points []PointUpsert) error {
	if f.hashes == nil {
		f.hashes = map[string]string{}
	}
	for _, point := range points {
		f.upserts = append(f.upserts, point.LogicalID)
		f.payloads = append(f.payloads, point.Payload)
		if hash, ok := point.Payload["contentHash"].(string); ok {
			f.hashes[point.LogicalID] = hash
		}
	}
	return nil
}

func (f *fakeVectorStore) ExistingContentHashes(_ context.Context, logicalIDs []string) (map[string]string, error) {
	out := map[string]string{}
	for _, id := range logicalIDs {
		if hash, ok := f.hashes[id]; ok {
			out[id] = hash
		}
	}
	return out, nil
}

func (f *fakeVectorStore) DeletePoints(_ context.Context, logicalIDs []string) error {
	f.deleted = append(f.deleted, logicalIDs...)
	return nil
}

func (f *fakeVectorStore) AllIndexedSongIDs(_ context.Context, _ int) ([]string, error) {
	ids := make([]string, 0, len(f.hashes))
	for id := range f.hashes {
		ids = append(ids, id)
	}
	return ids, nil
}

func TestIndexSongsCountsAndPayload(t *testing.T) {
	playedAt := time.Date(2026, time.July, 4, 12, 30, 0, 0, time.FixedZone("IST", 5*60*60+30*60))
	createdAt := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	rgTrackGain := -7.2
	repository := &fakeSongRepository{songs: model.MediaFiles{
		{
			ID: "new", LibraryID: 2, FolderID: "folder-1", Title: "New song",
			Artist: "Artist", ArtistID: "artist-1", Album: "Album", AlbumID: "album-1",
			AlbumArtist: "Album Artist", AlbumArtistID: "album-artist-1",
			TrackNumber: 3, DiscNumber: 1, Year: 2024, Date: "2024-01-02",
			OriginalYear: 2023, ReleaseYear: 2024, Genre: "Rock", ExplicitStatus: "e",
			BPM: 120, Duration: 215.5, Size: 123456, Suffix: "flac", Codec: "flac",
			BitRate: 900, SampleRate: 48000, BitDepth: 24, Channels: 2,
			Lyrics:      `[{"lang":"eng","line":[{"value":"hello"}]}]`,
			Annotations: model.Annotations{PlayCount: 7, PlayDate: &playedAt, Rating: 4, Starred: true, AverageRating: 4.5},
			CatalogNum:  "CAT-1", MbzRecordingID: "recording-1", MbzReleaseID: "release-1",
			SpotifyConfidence: 0.98, SpotifyMatch: "New song", SpotifyArtist: "Artist",
			SpotifyURL: "https://open.spotify.com/track/1", RGTrackGain: &rgTrackGain,
			HasCoverArt: true, CreatedAt: createdAt,
			Tags: model.Tags{
				"lufs": {"-12.5"}, model.TagGenre: {"Rock", "Alternative"},
				model.TagMood: {"Energetic"}, model.TagComposer: {"Composer"},
				model.TagISRC: {"US-ABC-24-00001"}, model.TagRecordLabel: {"Label"},
			},
		},
		{ID: "existing", Title: "Already indexed"},
		{ID: "failed", Title: "Fails embedding"},
	}}
	const tag = "test-model"
	existingSong := model.MediaFile{ID: "existing", Title: "Already indexed"}
	store := &fakeVectorStore{hashes: map[string]string{
		StableSongPointID("existing"): songContentHash(existingSong, tag),
	}}

	result, err := IndexSongs(context.Background(), repository, fakeEmbedder{}, store, 3, false, tag)
	if err != nil {
		t.Fatalf("index songs: %v", err)
	}
	if result.Indexed != 1 || result.Skipped != 1 || result.Failed != 1 || !strings.Contains(result.Error, "embedding failed") {
		t.Fatalf("unexpected index counts: %+v", result)
	}
	if len(repository.options) == 0 || repository.options[0].Max != indexPageSize || repository.options[0].Sort != "id" {
		t.Fatalf("unexpected repository options: %+v", repository.options)
	}
	if len(store.upserts) != 1 || store.upserts[0] != "song:new" {
		t.Fatalf("unexpected upserts: %#v", store.upserts)
	}
	payload := store.payloads[0]
	for key, expected := range map[string]any{
		"songId":         "new",
		"type":           "song",
		"title":          "New song",
		"artist":         "Artist",
		"album":          "Album",
		"albumArtist":    "Album Artist",
		"artistId":       "artist-1",
		"albumId":        "album-1",
		"trackNumber":    3,
		"year":           2024,
		"genre":          "Rock, Alternative",
		"genres":         []string{"Rock", "Alternative"},
		"moods":          []string{"Energetic"},
		"bpm":            120,
		"lufs":           -12.5,
		"duration":       float32(215.5),
		"playCount":      int64(7),
		"lastPlayedAt":   playedAt.UTC(),
		"hasLyrics":      true,
		"hasGenre":       true,
		"hasYear":        true,
		"hasBpm":         true,
		"hasLufs":        true,
		"explicitStatus": "explicit",
		"codec":          "flac",
		"bitDepth":       24,
		"rating":         4,
		"starred":        true,
		"catalogNum":     "CAT-1",
		"mbzRecordingId": "recording-1",
		"spotifyUrl":     "https://open.spotify.com/track/1",
		"hasMood":        true,
		"hasReplayGain":  true,
	} {
		if !reflect.DeepEqual(payload[key], expected) {
			t.Errorf("expected payload %s=%v, got %v", key, expected, payload[key])
		}
	}
	if payload["explicit"] != true {
		t.Errorf("expected explicit payload, got %v", payload["explicit"])
	}
	for _, key := range []string{
		"libraryId", "folderId", "albumArtistId", "discNumber", "date", "originalYear",
		"releaseYear", "groupings", "composers", "lyricists", "recordLabels", "isrc",
		"ratedAt", "starredAt", "averageRating", "size", "suffix", "bitRate", "sampleRate",
		"channels", "mbzReleaseId", "mbzReleaseTrackId", "mbzAlbumId", "mbzReleaseGroupId",
		"mbzArtistId", "mbzAlbumArtistId", "mbzAlbumType", "spotifyConfidence", "spotifyMatch",
		"spotifyArtist", "rgAlbumGain", "rgAlbumPeak", "rgTrackGain", "rgTrackPeak", "hasCoverArt",
		"missing", "createdAt", "updatedAt", "hasDate", "hasMusicBrainzIds", "hasSpotifyMetadata",
	} {
		if _, ok := payload[key]; !ok {
			t.Errorf("expected expanded payload field %q", key)
		}
	}
}

func TestIndexSongsSkipsExistingAndContinuesToNewSongs(t *testing.T) {
	existing1 := model.MediaFile{ID: "existing-1", Title: "Existing one"}
	existing2 := model.MediaFile{ID: "existing-2", Title: "Existing two"}
	repository := &fakeSongRepository{songs: model.MediaFiles{
		existing1,
		existing2,
		{ID: "new-1", Title: "New one"},
		{ID: "new-2", Title: "New two"},
	}}
	const tag = "test-model"
	store := &fakeVectorStore{hashes: map[string]string{
		StableSongPointID("existing-1"): songContentHash(existing1, tag),
		StableSongPointID("existing-2"): songContentHash(existing2, tag),
	}}

	result, err := IndexSongs(context.Background(), repository, fakeEmbedder{}, store, 2, false, tag)
	if err != nil {
		t.Fatalf("index songs: %v", err)
	}
	if result.Indexed != 2 || result.Skipped != 2 || result.Failed != 0 {
		t.Fatalf("unexpected index counts: %+v", result)
	}
	if len(store.upserts) != 2 || store.upserts[0] != "song:new-1" || store.upserts[1] != "song:new-2" {
		t.Fatalf("unexpected upserts: %#v", store.upserts)
	}
}

func TestSyncSongsIndexesChangedAndDeletesOrphans(t *testing.T) {
	const tag = "test-model"
	keep := model.MediaFile{ID: "keep", Title: "Keep me"}
	added := model.MediaFile{ID: "added", Title: "New song"}
	repository := &fakeSongRepository{songs: model.MediaFiles{keep, added}}
	// "keep" is already indexed and unchanged; "orphan" is indexed but no longer
	// in the library and must be deleted.
	store := &fakeVectorStore{hashes: map[string]string{
		StableSongPointID("keep"):   songContentHash(keep, tag),
		StableSongPointID("orphan"): "stale-hash",
	}}

	result, err := SyncSongs(context.Background(), repository, fakeEmbedder{}, store, tag)
	if err != nil {
		t.Fatalf("sync songs: %v", err)
	}
	if result.Indexed != 1 || result.Skipped != 1 {
		t.Fatalf("expected 1 indexed (added) and 1 skipped (keep), got %+v", result)
	}
	if result.Deleted != 1 || len(store.deleted) != 1 || store.deleted[0] != StableSongPointID("orphan") {
		t.Fatalf("expected orphan deletion, got deleted=%d %#v", result.Deleted, store.deleted)
	}
}

func TestIndexSongsRejectsLargeLimit(t *testing.T) {
	_, err := IndexSongs(
		context.Background(),
		&fakeSongRepository{},
		fakeEmbedder{},
		&fakeVectorStore{},
		MaxIndexLimit+1,
		false,
		"test-model",
	)
	if err == nil {
		t.Fatal("expected large limit to be rejected")
	}
}

func TestIndexPlaylistsUsesStableIDAndPlaylistPayload(t *testing.T) {
	playlist := &model.Playlist{
		ID: "playlist-1", Name: "Retail Mix", OwnerName: "Owner", Public: true,
		Tracks: model.PlaylistTracks{{MediaFile: model.MediaFile{
			ID: "song-1", Title: "Song", Artist: "Artist", Genre: "Pop", BPM: 120,
			Duration: 180, ExplicitStatus: "c", Tags: model.Tags{"lufs": {"-12.5"}},
		}}},
	}
	repository := &fakePlaylistRepository{
		playlists: model.Playlists{{ID: playlist.ID}},
		loaded:    map[string]*model.Playlist{playlist.ID: playlist},
	}
	store := &fakeVectorStore{existing: map[string]bool{}}

	result, err := IndexPlaylists(context.Background(), repository, fakeEmbedder{}, store, 10, false)
	if err != nil {
		t.Fatalf("index playlists: %v", err)
	}
	if result != (IndexResult{Indexed: 1}) {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(store.upserts) != 1 || store.upserts[0] != "playlist:playlist-1" {
		t.Fatalf("unexpected playlist upserts: %#v", store.upserts)
	}
	payload := store.payloads[0]
	if payload["type"] != "playlist" || payload["playlistId"] != "playlist-1" || payload["songCount"] != 1 || payload["explicitCount"] != 0 {
		t.Fatalf("unexpected playlist payload: %#v", payload)
	}
}
