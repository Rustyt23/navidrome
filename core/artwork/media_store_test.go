package artwork

import (
	"context"
	"io"
	"testing"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/model"
)

func TestMediaStoreSaveAndRead(t *testing.T) {
	conf.Server.DataFolder = t.TempDir()
	store := NewMediaStore()
	ctx := context.Background()

	artID, err := store.SaveMediaArtwork(ctx, "track-1", []byte("art"))
	if err != nil {
		t.Fatalf("SaveMediaArtwork returned error: %v", err)
	}
	if artID.Kind != model.KindMediaFileArtwork {
		t.Fatalf("unexpected kind: %s", artID.Kind)
	}
	if !store.HasMediaArtwork("track-1") {
		t.Fatalf("expected artwork to exist")
	}

	reader, _, err := store.OpenMediaArtwork("track-1")
	if err != nil {
		t.Fatalf("OpenMediaArtwork returned error: %v", err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(data) != "art" {
		t.Fatalf("unexpected data: %s", data)
	}
}
