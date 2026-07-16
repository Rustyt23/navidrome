package playlists

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/navidrome/navidrome/conf"
)

func countMissingRows(t *testing.T, dbFile string) int {
	t.Helper()
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		t.Fatalf("open missing db: %v", err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM missing_playlist_tracks`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}

func TestRecordMissingPlaylistTrackIsIdempotent(t *testing.T) {
	restore := conf.SnapshotConfig()
	defer restore()
	dir := t.TempDir()
	conf.Server.DataFolder = conf.NewDir(dir)
	ctx := context.Background()

	// Recording the same (playlist, track) repeatedly must not create duplicates.
	for i := 0; i < 3; i++ {
		recordMissingPlaylistTrack(ctx, "list.m3u", "Missing Song.mp3")
	}
	recordMissingPlaylistTrack(ctx, "list.m3u", "Another.mp3")

	dbFile := filepath.Join(dir, "missing_tracks.db")
	if got := countMissingRows(t, dbFile); got != 2 {
		t.Fatalf("expected 2 unique rows, got %d", got)
	}
}

func TestEnsureMissingTracksSchemaCollapsesHistoricalDuplicates(t *testing.T) {
	restore := conf.SnapshotConfig()
	defer restore()
	dir := t.TempDir()
	conf.Server.DataFolder = conf.NewDir(dir)
	dbFile := filepath.Join(dir, "missing_tracks.db")

	// Simulate a legacy table (no unique index) that already accumulated dupes.
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE missing_playlist_tracks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		playlist_id TEXT, track_path TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	for i := 0; i < 4; i++ {
		if _, err := db.Exec(`INSERT INTO missing_playlist_tracks (playlist_id, track_path) VALUES (?, ?)`, "list.m3u", "Dup.mp3"); err != nil {
			t.Fatalf("seed dupes: %v", err)
		}
	}
	if err := ensureMissingTracksSchema(db); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	db.Close()

	if got := countMissingRows(t, dbFile); got != 1 {
		t.Fatalf("expected historical duplicates collapsed to 1 row, got %d", got)
	}
}

func TestClearMissingPlaylistTracks(t *testing.T) {
	restore := conf.SnapshotConfig()
	defer restore()
	dir := t.TempDir()
	conf.Server.DataFolder = conf.NewDir(dir)
	ctx := context.Background()
	dbFile := filepath.Join(dir, "missing_tracks.db")

	recordMissingPlaylistTrack(ctx, "keep.m3u", "A.mp3")
	recordMissingPlaylistTrack(ctx, "drop.m3u", "B.mp3")
	recordMissingPlaylistTrack(ctx, "drop.m3u", "C.mp3")

	clearMissingPlaylistTracks(ctx, "drop.m3u")
	if got := countMissingRows(t, dbFile); got != 1 {
		t.Fatalf("expected only the other playlist's row to remain, got %d", got)
	}

	// An empty path must be a no-op and never wipe the whole table.
	clearMissingPlaylistTracks(ctx, "  ")
	if got := countMissingRows(t, dbFile); got != 1 {
		t.Fatalf("empty-path clear should be a no-op, got %d rows", got)
	}
}
