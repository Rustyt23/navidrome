package rag

import (
	"context"
	"errors"
	"strings"
	"testing"

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

type fakeVectorStore struct {
	existing map[string]bool
	upserts  []string
	payloads []map[string]any
}

func (f *fakeVectorStore) PointExists(_ context.Context, logicalID string) (bool, error) {
	return f.existing[logicalID], nil
}

func (f *fakeVectorStore) UpsertPoint(_ context.Context, logicalID string, _ []float32, payload map[string]any) error {
	f.upserts = append(f.upserts, logicalID)
	f.payloads = append(f.payloads, payload)
	return nil
}

func TestIndexSongsCountsAndPayload(t *testing.T) {
	repository := &fakeSongRepository{songs: model.MediaFiles{
		{ID: "new", Title: "New song", Artist: "Artist", Album: "Album", Year: 2024, Genre: "Rock", ExplicitStatus: "e", BPM: 120, Tags: model.Tags{"lufs": {"-12.5"}}},
		{ID: "existing", Title: "Already indexed"},
		{ID: "failed", Title: "Fails embedding"},
	}}
	store := &fakeVectorStore{existing: map[string]bool{"song:existing": true}}

	result, err := IndexSongs(context.Background(), repository, fakeEmbedder{}, store, 3, false)
	if err != nil {
		t.Fatalf("index songs: %v", err)
	}
	if result.Indexed != 1 || result.Skipped != 1 || result.Failed != 1 || !strings.Contains(result.Error, "embedding failed") {
		t.Fatalf("unexpected index counts: %+v", result)
	}
	if len(repository.options) == 0 || repository.options[0].Max != 3 || repository.options[0].Sort != "id" {
		t.Fatalf("unexpected repository options: %+v", repository.options)
	}
	if len(store.upserts) != 1 || store.upserts[0] != "song:new" {
		t.Fatalf("unexpected upserts: %#v", store.upserts)
	}
	payload := store.payloads[0]
	for key, expected := range map[string]any{
		"songId": "new",
		"type":   "song",
		"title":  "New song",
		"artist": "Artist",
		"album":  "Album",
		"year":   2024,
		"genre":  "Rock",
		"bpm":    120,
		"lufs":   -12.5,
	} {
		if payload[key] != expected {
			t.Errorf("expected payload %s=%v, got %v", key, expected, payload[key])
		}
	}
	if payload["explicit"] != true {
		t.Errorf("expected explicit payload, got %v", payload["explicit"])
	}
}

func TestIndexSongsSkipsExistingAndContinuesToNewSongs(t *testing.T) {
	repository := &fakeSongRepository{songs: model.MediaFiles{
		{ID: "existing-1", Title: "Existing one"},
		{ID: "existing-2", Title: "Existing two"},
		{ID: "new-1", Title: "New one"},
		{ID: "new-2", Title: "New two"},
	}}
	store := &fakeVectorStore{existing: map[string]bool{
		"song:existing-1": true,
		"song:existing-2": true,
	}}

	result, err := IndexSongs(context.Background(), repository, fakeEmbedder{}, store, 2, false)
	if err != nil {
		t.Fatalf("index songs: %v", err)
	}
	if result != (IndexResult{Indexed: 2, Skipped: 2}) {
		t.Fatalf("unexpected index counts: %+v", result)
	}
	if len(repository.options) != 2 || repository.options[1].Offset != 2 {
		t.Fatalf("expected bounded pagination past existing songs: %+v", repository.options)
	}
	if len(store.upserts) != 2 || store.upserts[0] != "song:new-1" || store.upserts[1] != "song:new-2" {
		t.Fatalf("unexpected upserts: %#v", store.upserts)
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
	)
	if err == nil {
		t.Fatal("expected large limit to be rejected")
	}
}
