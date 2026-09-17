package nativeapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
)

func TestCancelledWholeSongRequestDoesNotSaveLyrics(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()

	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	whisper := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		close(requestStarted)
		<-releaseRequest
	}))
	defer whisper.Close()
	defer close(releaseRequest)

	audioDir := t.TempDir()
	audioPath := filepath.Join(audioDir, "song.mp3")
	if err := os.WriteFile(audioPath, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	lyricsDir := t.TempDir()
	conf.Server.WhisperAPIURL = whisper.URL
	conf.Server.WhisperLyricsFolder = lyricsDir

	repo := tests.CreateMockMediaFileRepo()
	repo.SetData(model.MediaFiles{{
		ID: "song-1", LibraryPath: audioDir, Path: filepath.Base(audioPath), Duration: 180,
	}})
	mf, _ := repo.Get("song-1")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := fetchAndSaveWhisperLyrics(ctx, repo, mf)
		done <- err
	}()
	<-requestStarted
	cancel()
	if err := <-done; err == nil {
		t.Fatal("expected cancellation error")
	}

	stored, _ := repo.Get("song-1")
	if stored.Lyrics != "" {
		t.Fatalf("cancelled request saved lyrics: %q", stored.Lyrics)
	}
	if _, err := os.Stat(filepath.Join(lyricsDir, "song-1.txt")); !os.IsNotExist(err) {
		t.Fatalf("cancelled request created a lyrics file: %v", err)
	}
}

func TestIncompleteWholeSongResponseDoesNotSaveLyrics(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()

	whisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"language":"eng","text":"Only the beginning","duration":30,"segments":[{"start":0,"end":30}]}`))
	}))
	defer whisper.Close()

	audioDir := t.TempDir()
	audioPath := filepath.Join(audioDir, "song.mp3")
	if err := os.WriteFile(audioPath, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	conf.Server.WhisperAPIURL = whisper.URL
	conf.Server.WhisperLyricsFolder = t.TempDir()

	repo := tests.CreateMockMediaFileRepo()
	repo.SetData(model.MediaFiles{{
		ID: "song-1", LibraryPath: audioDir, Path: filepath.Base(audioPath), Duration: 180,
	}})
	mf, _ := repo.Get("song-1")
	if _, err := fetchAndSaveWhisperLyrics(context.Background(), repo, mf); err == nil {
		t.Fatal("expected incomplete whole-song response to fail")
	}
	stored, _ := repo.Get("song-1")
	if stored.Lyrics != "" {
		t.Fatalf("incomplete response saved lyrics: %q", stored.Lyrics)
	}
}
