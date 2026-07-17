package filelock

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLockSerializesEquivalentPaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "album", "..", "song.mp3")
	unlockFirst := Lock(path)

	attempting := make(chan struct{})
	acquired := make(chan struct{})
	go func() {
		close(attempting)
		unlockSecond := Lock(filepath.Clean(path))
		close(acquired)
		unlockSecond()
	}()

	<-attempting
	select {
	case <-acquired:
		t.Fatal("second writer acquired the same path while it was locked")
	case <-time.After(50 * time.Millisecond):
	}

	unlockFirst()
	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("second writer did not acquire the path after it was unlocked")
	}
}

func TestLockAllowsDifferentPaths(t *testing.T) {
	dir := t.TempDir()
	unlockFirst := Lock(filepath.Join(dir, "first.mp3"))
	defer unlockFirst()

	acquired := make(chan struct{})
	go func() {
		unlockSecond := Lock(filepath.Join(dir, "second.mp3"))
		close(acquired)
		unlockSecond()
	}()

	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Fatal("writer for a different path was blocked")
	}
}

func TestUnlockIsIdempotentAndReleasesRegistryEntry(t *testing.T) {
	unlock := Lock(filepath.Join(t.TempDir(), "song.mp3"))
	unlock()
	unlock()

	paths.mu.Lock()
	defer paths.mu.Unlock()
	if len(paths.locks) != 0 {
		t.Fatalf("lock registry contains %d entries after unlock", len(paths.locks))
	}
}
