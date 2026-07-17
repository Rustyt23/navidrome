// Package filelock serializes in-process mutations of the same filesystem
// path while allowing unrelated files to be modified concurrently.
package filelock

import (
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type pathLock struct {
	mu   sync.Mutex
	refs int
}

type registry struct {
	mu    sync.Mutex
	locks map[string]*pathLock
}

var paths = registry{locks: make(map[string]*pathLock)}

// Lock acquires the mutation lock for path and returns an idempotent unlock
// function. Paths are made absolute and existing symlinks are resolved so
// equivalent spellings of the same media file share a lock.
func Lock(path string) func() {
	key := canonicalPath(path)

	paths.mu.Lock()
	entry := paths.locks[key]
	if entry == nil {
		entry = &pathLock{}
		paths.locks[key] = entry
	}
	entry.refs++
	paths.mu.Unlock()

	entry.mu.Lock()

	var once sync.Once
	return func() {
		once.Do(func() {
			entry.mu.Unlock()

			paths.mu.Lock()
			entry.refs--
			if entry.refs == 0 {
				delete(paths.locks, key)
			}
			paths.mu.Unlock()
		})
	}
}

func canonicalPath(path string) string {
	key := filepath.Clean(path)
	if absolute, err := filepath.Abs(key); err == nil {
		key = absolute
	}
	if resolved, err := filepath.EvalSymlinks(key); err == nil {
		key = resolved
	}
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	return key
}
