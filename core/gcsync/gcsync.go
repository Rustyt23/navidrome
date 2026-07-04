// Package gcsync replaces the external "sync-to-GCS" bash script. It keeps a
// GCS bucket in sync with files produced by Navidrome itself:
//
//   - MP3 uploads are event-driven: when the app improves a file (cover art /
//     fetched metadata written to tags, or LUFS normalization that brings the
//     track closer to the configured target), the code path that made the
//     change enqueues the file, and a background worker uploads it to the
//     bucket root, overwriting the existing object. Uploaded files that live
//     in the Sync folder are then moved to the music folder.
//
//   - A periodic sweep processes everything else in the Sync folder:
//     leftover MP3s (not eligible for upload) are moved to the music
//     folder (conf.Server.MusicFolder) without touching the bucket; M3U playlists are cleaned
//     (CRLF -> space, Unicode NFC), uploaded, and placed under the PlaylistsPath
//     folder with timestamped versioning into the last-version folder; any
//     other file is uploaded and left in place. Empty directories are removed.
//
// Uploads shell out to the gcloud CLI (same as the original script), so no
// new credentials handling is needed on the server.
package gcsync

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/utils/singleton"
	"golang.org/x/text/unicode/norm"
)

const uploadTimeout = 5 * time.Minute

type Service interface {
	// EnqueueMP3 schedules an updated MP3 for upload to the bucket
	// (overwriting any existing object with the same name).
	EnqueueMP3(path, reason string)
	// Sweep processes the Sync folder: playlists, leftover MP3s and other files.
	Sweep(ctx context.Context) error
	// Start launches the background upload worker.
	Start(ctx context.Context)
}

// IsEligibleLUFS reports whether a normalized track should overwrite the copy
// in the GCS bucket: only when the new loudness is strictly closer to the
// target than the old one.
func IsEligibleLUFS(oldLUFS, newLUFS, targetLUFS float64) bool {
	return math.Abs(newLUFS-targetLUFS) < math.Abs(oldLUFS-targetLUFS)
}

type uploadRequest struct {
	path   string
	reason string
}

type service struct {
	queue   chan uploadRequest
	mu      sync.Mutex
	pending map[string]struct{}
	sweepMu sync.Mutex
}

func GetInstance() Service {
	return singleton.GetInstance(func() *service {
		return &service{
			queue:   make(chan uploadRequest, 4096),
			pending: make(map[string]struct{}),
		}
	})
}

func enabled() bool {
	return conf.Server.GCSync.Enabled && conf.Server.GCSync.Bucket != ""
}

func dryRun() bool { return conf.Server.GCSync.DryRun }

func (s *service) EnqueueMP3(path, reason string) {
	if !enabled() {
		return
	}
	path = filepath.Clean(path)
	s.mu.Lock()
	if _, ok := s.pending[path]; ok {
		s.mu.Unlock()
		return
	}
	s.pending[path] = struct{}{}
	s.mu.Unlock()

	select {
	case s.queue <- uploadRequest{path: path, reason: reason}:
		log.Info("GCSync: queued MP3 for upload", "path", path, "reason", reason)
	default:
		s.mu.Lock()
		delete(s.pending, path)
		s.mu.Unlock()
		log.Warn("GCSync: upload queue full, dropping file", "path", path, "reason", reason)
	}
}

func (s *service) isPending(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.pending[filepath.Clean(path)]
	return ok
}

func (s *service) Start(ctx context.Context) {
	go func() {
		log.Info(ctx, "GCSync: upload worker started", "bucket", conf.Server.GCSync.Bucket,
			"dryRun", dryRun())
		for {
			select {
			case <-ctx.Done():
				log.Info("GCSync: upload worker stopped")
				return
			case req := <-s.queue:
				s.processUpload(ctx, req)
				s.mu.Lock()
				delete(s.pending, req.path)
				s.mu.Unlock()
			}
		}
	}()
}

func (s *service) processUpload(ctx context.Context, req uploadRequest) {
	if _, err := os.Stat(req.path); err != nil {
		log.Warn(ctx, "GCSync: queued file no longer exists, skipping", "path", req.path, err)
		return
	}
	if err := s.upload(ctx, req.path); err != nil {
		log.Error(ctx, "GCSync: upload failed", "path", req.path, "reason", req.reason, err)
		return
	}
	log.Info(ctx, "GCSync: uploaded MP3 (overwrite)", "path", req.path, "reason", req.reason,
		"object", objectName(req.path))

	// Files living in the Sync folder are archived to originals/ after upload
	syncRoot := strings.TrimSpace(conf.Server.SyncFolder)
	if syncRoot != "" && isUnder(syncRoot, req.path) {
		if err := s.moveToMusicFolder(ctx, req.path); err != nil {
			log.Warn(ctx, "GCSync: could not move uploaded file to originals", "path", req.path, err)
		}
	}
}

// objectName returns the bucket object name for a local file (basename,
// NFC-normalized, like the original script's behavior after convmv).
func objectName(path string) string {
	return norm.NFC.String(filepath.Base(path))
}

func isUnder(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (s *service) upload(ctx context.Context, localPath string) error {
	dest := fmt.Sprintf("gs://%s/%s", conf.Server.GCSync.Bucket, objectName(localPath))
	if dryRun() {
		log.Info(ctx, "GCSync: (dry-run) would upload", "src", localPath, "dest", dest)
		return nil
	}
	cctx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()
	gcloud := conf.Server.GCSync.GcloudPath
	if gcloud == "" {
		gcloud = "gcloud"
	}
	cmd := exec.CommandContext(cctx, gcloud, "storage", "cp", localPath, dest)
	if credFile := strings.TrimSpace(conf.Server.GCSync.CredentialsFile); credFile != "" {
		cmd.Env = append(os.Environ(), "GOOGLE_APPLICATION_CREDENTIALS="+credFile)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gcloud storage cp %q %q: %w: %s", localPath, dest, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *service) moveToMusicFolder(ctx context.Context, path string) error {
	musicDir := strings.TrimSpace(conf.Server.MusicFolder)
	if musicDir == "" {
		log.Debug(ctx, "GCSync: MusicFolder not configured, leaving file in place", "path", path)
		return nil
	}
	dest := filepath.Join(musicDir, filepath.Base(path))
	return s.move(ctx, path, dest)
}

// playlistDestRoot derives the playlist destination directory from the
// PlaylistsPath config option (same derivation as the playlists service:
// first entry, glob suffix stripped).
func playlistDestRoot() string {
	pp := strings.TrimSpace(conf.Server.PlaylistsPath)
	if pp == "" {
		return ""
	}
	first := strings.Split(pp, string(filepath.ListSeparator))[0]
	root := strings.TrimSuffix(first, "**")
	root = strings.TrimSuffix(root, string(os.PathSeparator))
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return root
}

// move renames src to dest (creating parent dirs), falling back to
// copy+remove across filesystems. Respects dry-run.
func (s *service) move(ctx context.Context, src, dest string) error {
	if dryRun() {
		log.Info(ctx, "GCSync: (dry-run) would move", "src", src, "dest", dest)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	// Cross-device fallback
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return err
	}
	return os.Remove(src)
}

// Sweep replaces the periodic bash script run. It never uploads MP3s -
// eligible ones are handled by the event-driven queue.
func (s *service) Sweep(ctx context.Context) error {
	if !enabled() {
		log.Debug(ctx, "GCSync: disabled, skipping sweep")
		return nil
	}
	s.sweepMu.Lock()
	defer s.sweepMu.Unlock()

	root := strings.TrimSpace(conf.Server.SyncFolder)
	if root == "" {
		log.Warn(ctx, "GCSync: SyncFolder not configured, nothing to sweep")
		return nil
	}
	start := time.Now()
	log.Info(ctx, "GCSync: sweep started", "syncFolder", root, "dryRun", dryRun())

	s.normalizeFilenamesNFC(ctx, root)

	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Warn(ctx, "GCSync: error walking sync folder", "path", path, err)
			return nil
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(files)

	var moved, uploaded, playlists, skipped int
	for _, file := range files {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if s.isPending(file) {
			log.Debug(ctx, "GCSync: file queued for upload, skipping in sweep", "path", file)
			skipped++
			continue
		}
		switch strings.ToLower(filepath.Ext(file)) {
		case ".mp3":
			// Not enqueued by any update path => not eligible for bucket
			// overwrite. Move to the music folder without uploading.
			if err := s.moveToMusicFolder(ctx, file); err != nil {
				log.Warn(ctx, "GCSync: could not move MP3 to music folder", "path", file, err)
			} else {
				log.Info(ctx, "GCSync: MP3 not eligible for upload, moved to music folder", "path", file)
				moved++
			}
		case ".m3u":
			if err := s.processPlaylist(ctx, root, file); err != nil {
				log.Warn(ctx, "GCSync: could not process playlist", "path", file, err)
			} else {
				playlists++
			}
		default:
			if err := s.upload(ctx, file); err != nil {
				log.Warn(ctx, "GCSync: could not upload file", "path", file, err)
			} else {
				uploaded++
			}
		}
	}

	s.removeEmptyDirs(ctx, root)
	log.Info(ctx, "GCSync: sweep finished", "elapsed", time.Since(start), "mp3sToMusicFolder", moved,
		"playlists", playlists, "otherUploads", uploaded, "skippedPending", skipped)
	return nil
}

// normalizeFilenamesNFC renames files/dirs whose names are not in Unicode NFC
// form (replaces the script's convmv step).
func (s *service) normalizeFilenamesNFC(ctx context.Context, root string) {
	// Walk bottom-up so children are renamed before their parents
	var paths []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	for _, path := range paths {
		base := filepath.Base(path)
		nfc := norm.NFC.String(base)
		if nfc == base {
			continue
		}
		dest := filepath.Join(filepath.Dir(path), nfc)
		if dryRun() {
			log.Info(ctx, "GCSync: (dry-run) would NFC-rename", "src", path, "dest", dest)
			continue
		}
		if err := os.Rename(path, dest); err != nil {
			log.Warn(ctx, "GCSync: could not NFC-rename", "src", path, err)
		}
	}
}

// processPlaylist cleans, uploads and versions an M3U file from the Sync folder.
func (s *service) processPlaylist(ctx context.Context, root, file string) error {
	cleaned, err := s.cleanPlaylistFile(ctx, file)
	if err != nil {
		return fmt.Errorf("cleaning playlist: %w", err)
	}

	if err := s.upload(ctx, file); err != nil {
		return fmt.Errorf("uploading playlist: %w", err)
	}

	plsDir := playlistDestRoot()
	lastDir := strings.TrimSpace(conf.Server.GCSync.LastVersionFolder)
	if plsDir == "" || lastDir == "" {
		log.Warn(ctx, "GCSync: PlaylistsPath/LastVersionFolder not configured, leaving playlist in Sync", "path", file)
		return nil
	}

	rel, err := filepath.Rel(root, file)
	if err != nil {
		return err
	}
	destPath := filepath.Join(plsDir, rel)
	relDir := filepath.Dir(rel)
	name := filepath.Base(file)
	ts := time.Now().Format("20060102_150405")
	archiveName := strings.TrimSuffix(name, filepath.Ext(name)) + "_" + ts + filepath.Ext(name)

	existing, err := os.ReadFile(destPath)
	switch {
	case err == nil && bytes.Equal(existing, cleaned):
		// Identical: archive the incoming copy
		archivePath := filepath.Join(lastDir, relDir, archiveName)
		if err := s.move(ctx, file, archivePath); err != nil {
			return err
		}
		log.Info(ctx, "GCSync: playlist identical, archived incoming", "playlist", rel, "archive", archivePath)
	case err == nil:
		// Different: back up the old destination, then replace it
		backupPath := filepath.Join(lastDir, relDir, archiveName)
		if err := s.move(ctx, destPath, backupPath); err != nil {
			return err
		}
		if err := s.move(ctx, file, destPath); err != nil {
			return err
		}
		log.Info(ctx, "GCSync: playlist updated", "playlist", rel, "backup", backupPath)
	case os.IsNotExist(err):
		if err := s.move(ctx, file, destPath); err != nil {
			return err
		}
		log.Info(ctx, "GCSync: new playlist placed", "playlist", rel, "dest", destPath)
	default:
		return err
	}
	return nil
}

// cleanPlaylistFile replaces CR with spaces and normalizes the content to NFC
// (replaces the script's sed + uconv steps). Returns the cleaned content.
func (s *service) cleanPlaylistFile(ctx context.Context, file string) ([]byte, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	cleaned := norm.NFC.Bytes(bytes.ReplaceAll(data, []byte("\r"), []byte(" ")))
	if !bytes.Equal(data, cleaned) {
		if dryRun() {
			log.Info(ctx, "GCSync: (dry-run) would clean playlist (CRLF/NFC)", "path", file)
		} else if err := os.WriteFile(file, cleaned, 0o644); err != nil {
			return nil, err
		}
	}
	return cleaned, nil
}

func (s *service) removeEmptyDirs(ctx context.Context, root string) {
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && path != root {
			dirs = append(dirs, path)
		}
		return nil
	})
	// Deepest first
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			continue
		}
		if dryRun() {
			log.Info(ctx, "GCSync: (dry-run) would remove empty dir", "dir", dir)
			continue
		}
		if err := os.Remove(dir); err != nil {
			log.Debug(ctx, "GCSync: could not remove empty dir", "dir", dir, err)
		}
	}
}
