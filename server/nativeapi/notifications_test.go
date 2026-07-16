package nativeapi

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/navidrome/navidrome/conf"
)

func newTestLibraryDB(t *testing.T, paths ...string) string {
	t.Helper()
	dbFile := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE media_file (id INTEGER PRIMARY KEY, path TEXT)`); err != nil {
		t.Fatalf("create media_file: %v", err)
	}
	for i, p := range paths {
		if _, err := db.Exec(`INSERT INTO media_file (id, path) VALUES (?, ?)`, i+1, p); err != nil {
			t.Fatalf("insert media_file: %v", err)
		}
	}
	return dbFile
}

func TestDeriveTrackBase(t *testing.T) {
	cases := map[string]string{
		"/music/rock/Song - Title.mp3": "Song - Title.mp3",
		`C:\Music\Song - Title.mp3`:    "Song - Title.mp3",
		"Song - Title.flac":            "Song - Title.flac",
		"  /a/b/c.mp3  ":               "c.mp3",
		"":                             "",
	}
	for input, want := range cases {
		if got := deriveTrackBase(input); got != want {
			t.Fatalf("deriveTrackBase(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestReconcileMissingTracksDropsResolvedAndDedupes(t *testing.T) {
	restore := conf.SnapshotConfig()
	defer restore()
	// The library already contains "Present Song.mp3" (stored relative to the
	// library root, under a subfolder).
	conf.Server.DbPath = newTestLibraryDB(t, "rock/Present Song.mp3")

	scanned := []string{
		"/playlist/paths/Missing One.mp3",
		"/other/spelling/Missing One.mp3", // same base name -> duplicate, must collapse
		"/anywhere/Present Song.mp3",      // now in the library -> must be dropped
		"/anywhere/Another Missing.flac",
	}

	got := reconcileMissingTracks(context.Background(), scanned)
	if len(got) != 2 {
		t.Fatalf("expected 2 reconciled tracks, got %d: %v", len(got), got)
	}
	if got[0] != "/playlist/paths/Missing One.mp3" || got[1] != "/anywhere/Another Missing.flac" {
		t.Fatalf("unexpected reconciled order/content: %v", got)
	}
}

func TestReconcileMissingTracksExactPathMatch(t *testing.T) {
	restore := conf.SnapshotConfig()
	defer restore()
	conf.Server.DbPath = newTestLibraryDB(t, "exact/relative/Song.mp3")

	// track_path stored exactly as the library-relative path should reconcile.
	got := reconcileMissingTracks(context.Background(), []string{"exact/relative/Song.mp3"})
	if len(got) != 0 {
		t.Fatalf("expected exact library path to reconcile away, got %v", got)
	}
}

func TestLibraryHasTrackEscapesLikeWildcards(t *testing.T) {
	restore := conf.SnapshotConfig()
	defer restore()
	dbFile := newTestLibraryDB(t, "music/real_song.mp3")
	db, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// "real%song.mp3" must NOT match "real_song.mp3": the underscore in the
	// library file is a literal, and the % in the query base must be escaped.
	if libraryHasTrack(context.Background(), db, "/x/real%song.mp3") {
		t.Fatal("wildcard base name should not match unrelated library file")
	}
	// The genuine file resolves.
	if !libraryHasTrack(context.Background(), db, "/x/real_song.mp3") {
		t.Fatal("expected exact base-name match to resolve")
	}
}
