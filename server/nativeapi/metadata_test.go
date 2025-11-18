package nativeapi

import (
	"testing"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
)

func TestPersistMetadataSongUpdatesFields(t *testing.T) {
	repo := tests.CreateMockMediaFileRepo()
	original := model.MediaFile{
		ID:              "song-1",
		Title:           "Unknown",
		Artist:          "",
		Album:           "[Unknown Album]",
		Genre:           "",
		Year:            0,
		OrderTitle:      "unknown",
		OrderArtistName: "",
		OrderAlbumName:  "",
	}
	repo.SetData(model.MediaFiles{original})

	year := 1999
	update := metadataSongPayload{
		ID:     "song-1",
		Title:  "New Title",
		Artist: "New Artist",
		Album:  "New Album",
		Genre:  "Rock",
		Year:   &year,
	}

	if err := persistMetadataSong(repo, update); err != nil {
		t.Fatalf("persistMetadataSong returned error: %v", err)
	}

	stored, err := repo.Get("song-1")
	if err != nil {
		t.Fatalf("expected song to exist: %v", err)
	}

	if stored.Title != "New Title" {
		t.Fatalf("expected title to be updated, got %q", stored.Title)
	}
	if stored.Artist != "New Artist" {
		t.Fatalf("expected artist to be updated, got %q", stored.Artist)
	}
	if stored.Album != "New Album" {
		t.Fatalf("expected album to be updated, got %q", stored.Album)
	}
	if stored.Genre != "Rock" {
		t.Fatalf("expected genre to be updated, got %q", stored.Genre)
	}
	if stored.Year != 1999 {
		t.Fatalf("expected year to be updated, got %d", stored.Year)
	}
	if stored.OrderTitle == "unknown" {
		t.Fatalf("expected order title to change")
	}
	if stored.OrderAlbumName == "" {
		t.Fatalf("expected order album name to be set")
	}
	if stored.OrderArtistName == "" {
		t.Fatalf("expected order artist name to be set")
	}
	if len(stored.Genres) == 0 || stored.Genres[0].Name != "Rock" {
		t.Fatalf("expected genres metadata to be updated")
	}
}

func TestApplyMetadataUpdateNoChanges(t *testing.T) {
	mf := &model.MediaFile{ID: "song-2", Title: "Existing"}
	if applyMetadataUpdate(mf, metadataSongPayload{ID: "song-2"}) {
		t.Fatalf("expected no changes when payload empty")
	}
}

// Ensure helper can be used without repository context; this mirrors handler usage.
func TestPersistMetadataSongHandlesMissingSong(t *testing.T) {
	repo := tests.CreateMockMediaFileRepo()
	err := persistMetadataSong(repo, metadataSongPayload{ID: "missing"})
	if err == nil {
		t.Fatalf("expected error when song does not exist")
	}
}
