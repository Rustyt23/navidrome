// Package gcpsync replaces the external "sync-to-GCS" bash script. It keeps a
// GCS bucket in sync with files produced by Navidrome itself.
//
// The staging folder SyncFolder/mp3 is what marks an MP3 as *processed*.
// Anything found under it is force-overwritten in the bucket. Copies made
// from the library are retired after upload; imported processed files move
// into the music folder. An MP3 elsewhere in the Sync folder is treated as new
// material arriving from outside and is only uploaded when the bucket does
// not already hold an object with that name.
//
//   - When the app improves a file (embedded cover art, fetched metadata or
//     LUFS normalization), it stages a copy under SyncFolder/mp3. The Sync
//     button uploads it, overwriting the bucket object, then removes the copy.
//     The library already holds the updated song. GCPSync.AutoUpload can opt
//     into immediate uploads or timed sweeps, selected by GCPSync.Schedule.
//
//   - A periodic sweep processes the rest of the Sync folder: MP3s under
//     SyncFolder/mp3 are uploaded (overwrite) and moved to the music folder;
//     MP3s outside it are uploaded only if absent from the bucket, then moved;
//     M3U playlists are cleaned (CRLF -> space, Unicode NFC), uploaded, and
//     placed under PlaylistsPath with timestamped versioning into the
//     last-version folder; every other file is moved to the trash folder,
//     since the library holds nothing but .mp3 and .m3u. Empty directories are
//     removed.
//
// Local moves preserve the file's path relative to the staging (or Sync) root,
// so the music folder keeps its directory layout. Bucket object names stay
// flat - playlists reference tracks by bare filename, so the devices resolve
// objects at the bucket root.
//
// Uploads shell out to the gcloud CLI (same as the original script), so no
// new credentials handling is needed on the server.
package gcpsync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/utils/filelock"
	"github.com/navidrome/navidrome/utils/singleton"
	"golang.org/x/text/unicode/norm"
)

const uploadTimeout = 5 * time.Minute

type Service interface {
	// EnqueueMP3 schedules an updated MP3 when automatic uploads are enabled,
	// overwriting any existing bucket object with the same name.
	EnqueueMP3(path, reason string)
	// StageMP3 copies an MP3 the app has just rewritten into the staging
	// folder (SyncFolder/mp3), optionally queuing an automatic upload. libraryRoot is
	// the library the file belongs to, which need not be MusicFolder; pass ""
	// only when it is genuinely unknown.
	StageMP3(libraryRoot, path, reason string) error
	// Sweep processes the Sync folder: playlists, leftover MP3s and other files.
	Sweep(ctx context.Context) error
	// Start creates the staging folder and, when enabled, starts automatic uploads.
	Start(ctx context.Context)
}

// IsEligibleLUFS reports whether a normalized track should overwrite the copy
// in the GCS bucket: only when the new loudness is strictly closer to the
// target than the old one.
func IsEligibleLUFS(oldLUFS, newLUFS, targetLUFS float64) bool {
	return math.Abs(newLUFS-targetLUFS) < math.Abs(oldLUFS-targetLUFS)
}

type uploadRequest struct {
	// path is the staged copy to upload.
	path string
	// source is the library file the staged copy was made from, empty for
	// files discovered by a sweep. A durable marker identifies library copies
	// when no queue entry exists (manual mode or after a restart).
	source string
	reason string
}

type service struct {
	readName func(string) (string, error)            // nil uses embedded artist/title tags
	notify   func(context.Context, syncReport) error // nil uses configured SMTP
	queue    chan uploadRequest
	mu       sync.Mutex
	pending  map[string]struct{}
	// restaged holds work that arrived for a path already being uploaded. The
	// staged file on disk was replaced with newer content after its queue entry
	// was taken, so that entry is stale: without this the newer version would
	// silently never reach the bucket. See enqueue.
	restaged map[string]uploadRequest
	sweepMu  sync.Mutex
}

func GetInstance() Service {
	return singleton.GetInstance(func() *service {
		return &service{
			queue:    make(chan uploadRequest, 4096),
			pending:  make(map[string]struct{}),
			restaged: make(map[string]uploadRequest),
		}
	})
}

func enabled() bool {
	return conf.Server.GCPSync.Enabled && conf.Server.GCPSync.Bucket != ""
}

func dryRun() bool { return conf.Server.GCPSync.DryRun }

// A scheduled sweep publishes the whole staging folder at the configured time.
// Do not let app edits or playlist renames bypass it through the immediate path.
func immediateUploadsEnabled() bool {
	return enabled() && conf.Server.GCPSync.AutoUpload && strings.TrimSpace(conf.Server.GCPSync.Schedule) == ""
}

func (s *service) EnqueueMP3(path, reason string) {
	s.enqueue(uploadRequest{path: path, reason: reason})
}

// enqueue queues one upload, coalescing repeats of the same path.
//
// A song is commonly improved twice in a row - LUFS normalizes it, then cover
// art is embedded - and the second pass replaces the staged copy while the
// first is still queued or mid-upload. Dropping the second request (which is
// what a plain de-duplicating set does) left the bucket holding the older
// version with nothing left to correct it. Instead the request is remembered
// and run again once the in-flight one finishes.
func (s *service) enqueue(req uploadRequest) {
	if !immediateUploadsEnabled() {
		return
	}
	req.path = filepath.Clean(req.path)

	s.mu.Lock()
	if _, ok := s.pending[req.path]; ok {
		if s.restaged == nil {
			s.restaged = make(map[string]uploadRequest)
		}
		s.restaged[req.path] = req
		s.mu.Unlock()
		log.Debug("GCPSync: MP3 updated again while queued, will re-upload after the current pass",
			"path", req.path, "reason", req.reason)
		return
	}
	s.pending[req.path] = struct{}{}
	s.mu.Unlock()

	select {
	case s.queue <- req:
		log.Info("GCPSync: queued MP3 for upload", "path", req.path, "reason", req.reason)
	default:
		s.mu.Lock()
		delete(s.pending, req.path)
		s.mu.Unlock()
		log.Warn("GCPSync: upload queue full, dropping file", "path", req.path, "reason", req.reason)
	}
}

// StageMP3 copies a file the app has just rewritten into SyncFolder/mp3,
// keeping its path relative to the music folder. Automatic queuing is opt-in.
// The original is left untouched: the library row, the loudness audit
// and any in-flight playback all still point at it. After upload, the staged
// copy is retired without replacing the library file again.
func (s *service) StageMP3(libraryRoot, path, reason string) error {
	if !enabled() {
		return nil
	}
	staging := stagingRoot()
	if staging == "" {
		log.Debug("GCPSync: SyncFolder not configured, not staging MP3", "path", path)
		return fmt.Errorf("SyncFolder is not configured")
	}
	dest := filepath.Join(staging, libraryRel(libraryRoot, path))
	if dryRun() {
		log.Info("GCPSync: (dry-run) would stage MP3 for upload", "src", path, "dest", dest, "reason", reason)
		return nil
	}
	// TagLib writes in place, whereas normalization replaces the file. Use
	// the same library lock as both writers, and serialize staged replacements
	// with the upload snapshot/cleanup steps.
	unlockSource := filelock.Lock(path)
	defer unlockSource()
	unlockStaged := filelock.Lock(dest)
	defer unlockStaged()
	// Persist provenance before publishing the copy so even a restart cannot
	// mistake it for an imported file and move it over a newer library edit.
	if err := markLibraryCopy(dest); err != nil {
		log.Warn("GCPSync: could not mark staged library copy", "dest", dest, err)
		return err
	}
	if err := copyFile(path, dest); err != nil {
		log.Warn("GCPSync: could not stage MP3 for upload", "src", path, "dest", dest, err)
		return err
	}
	log.Info("GCPSync: staged MP3 for upload", "src", path, "dest", dest, "reason", reason)
	s.enqueue(uploadRequest{path: dest, source: path, reason: reason})
	return nil
}

// copyFile copies src to dest, creating parent directories and replacing any
// existing file. Streamed rather than read whole: these are audio files.
//
// Written to a temporary file beside the destination and renamed into place, so
// the replacement is atomic. Truncating the destination and streaming into it
// would expose two failures that both end in a corrupt object in the bucket:
// the upload worker reading a half-written file, and a copy that dies partway
// leaving neither the old copy nor a complete new one. A rename means a reader
// sees the previous file or the new one, never a partial.
func copyFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dest), "."+filepath.Base(dest)+".staging-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		// No-op once the rename has succeeded.
		_ = os.Remove(tmpName)
	}()

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpName, dest)
}

func (s *service) Start(ctx context.Context) {
	// Created up front so the staging folder is visible before anything is
	// staged. It is a marker as much as a directory - files under it are the
	// ones that overwrite - and an operator should be able to see it exists.
	if staging := stagingRoot(); staging != "" && !dryRun() {
		if err := os.MkdirAll(staging, 0o755); err != nil {
			log.Warn(ctx, "GCPSync: could not create staging folder", "path", staging, err)
		}
	}

	if !immediateUploadsEnabled() {
		return
	}
	go s.runWorker(ctx)
}

func (s *service) runWorker(ctx context.Context) {
	log.Info(ctx, "GCPSync: upload worker started", "bucket", conf.Server.GCPSync.Bucket,
		"dryRun", dryRun())
	for {
		select {
		case <-ctx.Done():
			log.Info("GCPSync: upload worker stopped")
			return
		case req := <-s.queue:
			s.processUpload(ctx, req)

			s.mu.Lock()
			delete(s.pending, req.path)
			again, superseded := s.restaged[req.path]
			delete(s.restaged, req.path)
			s.mu.Unlock()

			if superseded {
				// A newer staged version is retained by uploadProcessedMP3.
				// Do not recreate an already-swept copy from the library.
				if _, err := os.Stat(again.path); err == nil {
					s.enqueue(again)
				}
			}
		}
	}
}

func (s *service) processUpload(ctx context.Context, req uploadRequest) {
	s.sweepMu.Lock()
	name, err := s.uploadProcessedMP3(ctx, req)
	s.sweepMu.Unlock()
	report := syncReport{Operation: "Automatic upload"}
	if name != "" {
		report.Uploaded = append(report.Uploaded, name)
	}
	if err != nil {
		report.Failed = append(report.Failed, err.Error())
		log.Error(ctx, "GCPSync: upload failed", "path", req.path, "reason", req.reason, err)
	}
	if len(report.Uploaded)+len(report.Failed) > 0 {
		s.deliverReport(ctx, report)
	}
}

// syncRoot is the configured Sync folder, cleaned and absolute.
func syncRoot() string {
	root := strings.TrimSpace(conf.Server.SyncFolder)
	if root == "" {
		return ""
	}
	if abs, err := filepath.Abs(root); err == nil {
		return abs
	}
	return filepath.Clean(root)
}

// stagingRoot is SyncFolder/mp3, the folder that marks an MP3 as processed and
// therefore eligible to overwrite the bucket object.
func stagingRoot() string {
	root := syncRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "mp3")
}

// trashRoot is where the sweep parks files it will not sync. The library holds
// only .mp3 and .m3u, so anything else arriving in the Sync folder is junk
// rather than something to push to the bucket.
//
// Reuses DeletedPlaylistsPath, the folder the app already empties deleted
// playlists into, rather than inventing a second trash for the same purpose.
// Only when that is unset does it fall back to a folder inside the Sync root,
// so trashing always has somewhere to go.
func trashRoot() string {
	if t := strings.TrimSpace(conf.Server.DeletedPlaylistsPath); t != "" {
		if abs, err := filepath.Abs(t); err == nil {
			return abs
		}
		return filepath.Clean(t)
	}
	root := syncRoot()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "trash")
}

// musicRoot is the configured music folder, cleaned and absolute. The paths
// this package compares against it are absolute, and filepath.Rel refuses to
// mix the two.
func musicRoot() string {
	dir := strings.TrimSpace(conf.Server.MusicFolder)
	if dir == "" {
		return ""
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return filepath.Clean(dir)
}

// relUnder returns path relative to root when path sits inside it.
//
// Compared literally first, then again with symlinks resolved. A music folder
// reached through a symlink (a mounted volume, a /data link) is configured as
// one string and stored in the library as another, and a purely textual
// comparison says the track is not in the library at all - which used to send
// every staged copy to a flattened fallback name.
func relUnder(root, path string) (string, bool) {
	if root == "" {
		return "", false
	}
	if rel, ok := relUnderLiteral(root, path); ok {
		return rel, true
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", false
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		// The file may not exist yet (a rename destination); resolving just
		// its directory is enough to place it.
		realDir, dirErr := filepath.EvalSymlinks(filepath.Dir(path))
		if dirErr != nil {
			return "", false
		}
		realPath = filepath.Join(realDir, filepath.Base(path))
	}
	return relUnderLiteral(realRoot, realPath)
}

func relUnderLiteral(root, path string) (string, bool) {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// destRel is where a Sync-folder file belongs once it leaves the Sync folder.
// Files in the staging folder keep their path relative to it (staging mirrors
// the library layout), other Sync-folder files keep their path relative to the
// Sync root. Preserving the relative path is what stops every synced file from
// piling up flattened at the destination root, which the scanner would then
// index as a second copy of the track.
func destRel(file string) string {
	if rel, ok := relUnder(stagingRoot(), file); ok {
		return rel
	}
	if rel, ok := relUnder(syncRoot(), file); ok {
		return rel
	}
	return filepath.Base(file)
}

// libraryRel is the reverse of destRel for a file that is still in the music
// folder: the path a staged copy of it should take inside the staging folder.
//
// libraryRoot is the root the caller knows the file belongs to (a library may
// be mounted outside MusicFolder), and is tried first. Falling back to the bare
// filename is a last resort and says so loudly: it flattens the staging folder,
// so two songs sharing a filename in different albums overwrite each other
// there - one pass silently destroying the other's work.
func libraryRel(libraryRoot, file string) string {
	if rel, ok := relUnder(strings.TrimSpace(libraryRoot), file); ok {
		return rel
	}
	if rel, ok := relUnder(musicRoot(), file); ok {
		return rel
	}
	log.Warn("GCPSync: file is not under any known library root, staging it by filename alone",
		"path", file, "libraryRoot", libraryRoot, "musicFolder", musicRoot())
	return filepath.Base(file)
}

// StagingPath returns where a staged copy of the given library file lives, or
// "" when there is no Sync folder configured. Used to keep a staged copy in
// step when the library file it was made from is renamed.
func StagingPath(libraryRoot, absPath string) string {
	staging := stagingRoot()
	if staging == "" {
		return ""
	}
	return filepath.Join(staging, libraryRel(libraryRoot, absPath))
}

// objectName returns the bucket object name for a local file (basename,
// NFC-normalized, like the original script's behavior after convmv).
//
// Deliberately flat, even though local moves preserve directory structure:
// playlists list their tracks as bare "Artist - Title.mp3" filenames, so the
// devices look every object up at the bucket root. Nesting object names here
// would leave those playlist entries pointing at nothing.
func objectName(path string) string {
	return norm.NFC.String(filepath.Base(path))
}

// gcloudCmd builds a gcloud invocation carrying the configured credentials.
func gcloudCmd(ctx context.Context, args ...string) *exec.Cmd {
	gcloud := conf.Server.GCPSync.GcloudPath
	if gcloud == "" {
		gcloud = "gcloud"
	}
	cmd := exec.CommandContext(ctx, gcloud, args...)
	if credFile := strings.TrimSpace(conf.Server.GCPSync.CredentialsFile); credFile != "" {
		cmd.Env = append(os.Environ(), "GOOGLE_APPLICATION_CREDENTIALS="+credFile)
	}
	return cmd
}

// bucketObjects lists the bucket once, so the sweep can tell a new MP3 from one
// the bucket already holds without paying a gcloud round trip per file.
func (s *service) bucketObjects(ctx context.Context) (map[string]struct{}, error) {
	cctx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()
	cmd := gcloudCmd(cctx, "storage", "ls", fmt.Sprintf("gs://%s/**", conf.Server.GCPSync.Bucket))
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		// An empty bucket is not a failure - gcloud just matches nothing.
		lower := strings.ToLower(text)
		if strings.Contains(lower, "matched no objects") || strings.Contains(lower, "one or more urls matched no objects") {
			return map[string]struct{}{}, nil
		}
		return nil, fmt.Errorf("gcloud storage ls gs://%s: %w: %s", conf.Server.GCPSync.Bucket, err, text)
	}
	objects := make(map[string]struct{})
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "gs://") || strings.HasSuffix(line, "/") {
			continue
		}
		objects[norm.NFC.String(path.Base(line))] = struct{}{}
	}
	return objects, nil
}

func (s *service) upload(ctx context.Context, localPath string) error {
	dest := fmt.Sprintf("gs://%s/%s", conf.Server.GCPSync.Bucket, objectName(localPath))
	if dryRun() {
		log.Info(ctx, "GCPSync: (dry-run) would upload", "src", localPath, "dest", dest)
		return nil
	}
	cctx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()
	cmd := gcloudCmd(cctx, "storage", "cp", localPath, dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gcloud storage cp %q %q: %w: %s", localPath, dest, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *service) moveToMusicFolder(ctx context.Context, path string) error {
	musicDir := musicRoot()
	if musicDir == "" {
		log.Debug(ctx, "GCPSync: MusicFolder not configured, leaving file in place", "path", path)
		return nil
	}
	dest := filepath.Join(musicDir, destRel(path))
	return s.move(ctx, path, dest)
}

// moveToTrash parks a file the sweep will not sync, keeping its path relative
// to the Sync root so the trash folder shows where each file came from.
//
// A name already taken in the trash gets a timestamp suffix rather than being
// overwritten, the same way deleted playlists are handled: the trash is where
// files go to be looked at later, so silently replacing one defeats the point.
func (s *service) moveToTrash(ctx context.Context, path string) error {
	trash := trashRoot()
	if trash == "" {
		return fmt.Errorf("trash folder not configured")
	}
	dest := filepath.Join(trash, destRel(path))
	if _, err := os.Stat(dest); err == nil {
		ext := filepath.Ext(dest)
		dest = strings.TrimSuffix(dest, ext) + "-" + strconv.FormatInt(time.Now().UnixNano(), 10) + ext
	}
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
		log.Info(ctx, "GCPSync: (dry-run) would move", "src", src, "dest", dest)
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

// Sweep replaces the periodic bash script run.
func (s *service) Sweep(ctx context.Context) (resultErr error) {
	if !enabled() {
		log.Debug(ctx, "GCPSync: disabled, skipping sweep")
		return nil
	}
	s.sweepMu.Lock()
	var failures []error
	report := syncReport{Operation: "Sync sweep"}
	defer func() {
		s.sweepMu.Unlock()
		for _, failure := range failures {
			report.Failed = append(report.Failed, failure.Error())
		}
		if resultErr != nil && len(report.Failed) == 0 {
			report.Failed = append(report.Failed, resultErr.Error())
		}
		s.deliverReport(ctx, report)
	}()

	root := syncRoot()
	if root == "" {
		log.Warn(ctx, "GCPSync: SyncFolder not configured, nothing to sweep")
		return nil
	}
	start := time.Now()
	log.Info(ctx, "GCPSync: sweep started", "syncFolder", root, "dryRun", dryRun())

	trash := trashRoot()
	s.normalizeFilenamesNFC(ctx, root, trash)

	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Warn(ctx, "GCPSync: error walking sync folder", "path", path, err)
			return nil
		}
		if d.IsDir() {
			// Never descend into the trash: its contents are already dealt
			// with, and re-sweeping them would nest trash inside itself.
			if trash != "" && filepath.Clean(path) == trash {
				return filepath.SkipDir
			}
			return nil
		}
		// Dotfiles are not content. Chief among them are the temporary files
		// copyFile writes beside a staged copy before renaming them into
		// place: sweeping one would move a half-written file out from under
		// the copy that is still writing it.
		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return err
	}
	// Publish the renamed audio before playlists start referencing its new name.
	sort.Slice(files, func(i, j int) bool {
		left := strings.EqualFold(filepath.Ext(files[i]), ".mp3")
		right := strings.EqualFold(filepath.Ext(files[j]), ".mp3")
		if left != right {
			return left
		}
		return files[i] < files[j]
	})

	staging := stagingRoot()

	// One listing for the whole sweep tells new MP3s from ones the bucket
	// already holds. If it fails we cannot tell them apart, so no unstaged MP3
	// is uploaded this round - re-uploading every existing track is the worse
	// of the two mistakes, and the next sweep retries. Only staged MP3s in the
	// batch means nothing needs classifying, so the call is skipped entirely.
	var bucketFiles map[string]struct{}
	var bucketErr error
	if slices.ContainsFunc(files, func(f string) bool {
		if !strings.EqualFold(filepath.Ext(f), ".mp3") {
			return false
		}
		_, staged := relUnder(staging, f)
		return !staged
	}) {
		bucketFiles, bucketErr = s.bucketObjects(ctx)
		if bucketErr != nil {
			log.Warn(ctx, "GCPSync: could not list bucket; unstaged MP3s will not be uploaded this sweep", bucketErr)
		}
	}

	var moved, uploaded, playlists, alreadyInBucket, trashed int
	for _, file := range files {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		switch strings.ToLower(filepath.Ext(file)) {
		case ".mp3":
			if _, staged := relUnder(staging, file); !staged {
				name, err := s.mp3Name(file)
				if err != nil {
					failures = append(failures, err)
					continue
				}
				dest := filepath.Join(filepath.Dir(file), name)
				if dest != file && !dryRun() {
					if _, err := os.Stat(dest); !os.IsNotExist(err) {
						failures = append(failures, fmt.Errorf("cannot rename %q: destination %q already exists or is inaccessible", file, dest))
						continue
					}
					if err := os.Rename(file, dest); err != nil {
						failures = append(failures, err)
						continue
					}
					file = dest
				}
			}
			if _, staged := relUnder(staging, file); staged {
				// Processed by the app (LUFS, cover art, fetched metadata).
				// The staging folder is the marker: overwrite unconditionally,
				// which also covers files whose queue entry was lost to a
				// restart.
				name, err := s.uploadProcessedMP3(ctx, uploadRequest{path: file})
				if err != nil {
					log.Warn(ctx, "GCPSync: could not upload processed MP3, leaving in staging", "path", file, err)
					failures = append(failures, err)
					continue
				}
				if name != "" {
					report.Uploaded = append(report.Uploaded, name)
					uploaded++
				}
				log.Info(ctx, "GCPSync: uploaded processed MP3 (overwrite)", "path", file, "object", name)
				continue
			} else if bucketErr != nil {
				log.Warn(ctx, "GCPSync: bucket contents unknown, not uploading unstaged MP3", "path", file)
			} else if _, exists := bucketFiles[objectName(file)]; exists {
				// Already in the bucket and not marked processed: leave the
				// object alone, just file the local copy into the library.
				log.Info(ctx, "GCPSync: MP3 already in bucket, skipping upload", "path", file, "object", objectName(file))
				alreadyInBucket++
			} else {
				if err := s.upload(ctx, file); err != nil {
					log.Warn(ctx, "GCPSync: could not upload new MP3, leaving in sync folder", "path", file, err)
					failures = append(failures, err)
					continue
				}
				log.Info(ctx, "GCPSync: uploaded new MP3", "path", file, "object", objectName(file))
				report.Uploaded = append(report.Uploaded, objectName(file))
				uploaded++
			}
			if err := s.moveToMusicFolder(ctx, file); err != nil {
				log.Warn(ctx, "GCPSync: could not move MP3 to music folder", "path", file, err)
				failures = append(failures, err)
			} else {
				moved++
			}
		case ".m3u":
			if err := s.processPlaylist(ctx, root, file); err != nil {
				log.Warn(ctx, "GCPSync: could not process playlist", "path", file, err)
				failures = append(failures, err)
			} else {
				report.Uploaded = append(report.Uploaded, objectName(file))
				playlists++
			}
		default:
			// The library is .mp3 and .m3u only, so anything else that turns up
			// here is junk. It used to be uploaded unconditionally, which let a
			// stray file overwrite whatever bucket object shared its basename.
			if err := s.moveToTrash(ctx, file); err != nil {
				log.Warn(ctx, "GCPSync: could not move unsupported file to trash", "path", file, err)
				failures = append(failures, err)
			} else {
				log.Info(ctx, "GCPSync: unsupported file moved to trash", "path", file)
				trashed++
			}
		}
	}

	s.removeEmptyDirs(ctx, root)
	log.Info(ctx, "GCPSync: sweep finished", "elapsed", time.Since(start), "mp3sToMusicFolder", moved,
		"playlists", playlists, "uploaded", uploaded, "alreadyInBucket", alreadyInBucket,
		"trashed", trashed)
	return errors.Join(failures...)
}

// normalizeFilenamesNFC renames files/dirs whose names are not in Unicode NFC
// form (replaces the script's convmv step).
func (s *service) normalizeFilenamesNFC(ctx context.Context, root, trash string) {
	// Walk bottom-up so children are renamed before their parents
	var paths []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		// Staged paths mirror the library exactly. Only the GCS object name
		// needs NFC normalization; renaming this tree loses its source mapping.
		if d.IsDir() && filepath.Clean(path) == stagingRoot() {
			return filepath.SkipDir
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() && trash != "" && filepath.Clean(path) == trash {
			return filepath.SkipDir
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
			log.Info(ctx, "GCPSync: (dry-run) would NFC-rename", "src", path, "dest", dest)
			continue
		}
		if err := os.Rename(path, dest); err != nil {
			log.Warn(ctx, "GCPSync: could not NFC-rename", "src", path, err)
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
	lastDir := strings.TrimSpace(conf.Server.GCPSync.LastVersionFolder)
	if plsDir == "" || lastDir == "" {
		log.Warn(ctx, "GCPSync: PlaylistsPath/LastVersionFolder not configured, leaving playlist in Sync", "path", file)
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
		log.Info(ctx, "GCPSync: playlist identical, archived incoming", "playlist", rel, "archive", archivePath)
	case err == nil:
		// Different: back up the old destination, then replace it
		backupPath := filepath.Join(lastDir, relDir, archiveName)
		if err := s.move(ctx, destPath, backupPath); err != nil {
			return err
		}
		if err := s.move(ctx, file, destPath); err != nil {
			return err
		}
		log.Info(ctx, "GCPSync: playlist updated", "playlist", rel, "backup", backupPath)
	case os.IsNotExist(err):
		if err := s.move(ctx, file, destPath); err != nil {
			return err
		}
		log.Info(ctx, "GCPSync: new playlist placed", "playlist", rel, "dest", destPath)
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
			log.Info(ctx, "GCPSync: (dry-run) would clean playlist (CRLF/NFC)", "path", file)
		} else if err := os.WriteFile(file, cleaned, 0o644); err != nil {
			return nil, err
		}
	}
	return cleaned, nil
}

func (s *service) removeEmptyDirs(ctx context.Context, root string) {
	trash := trashRoot()
	staging := stagingRoot()
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == root {
			return nil
		}
		if trash != "" && filepath.Clean(path) == trash {
			return filepath.SkipDir
		}
		// The staging folder itself is part of the layout, not leftover
		// scaffolding: removing it whenever it drains makes the sync folder
		// look wiped, and the marker it represents disappears from view.
		// Empty directories *inside* it are still cleaned up.
		if staging != "" && filepath.Clean(path) == staging {
			return nil
		}
		dirs = append(dirs, path)
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
			log.Info(ctx, "GCPSync: (dry-run) would remove empty dir", "dir", dir)
			continue
		}
		if err := os.Remove(dir); err != nil {
			log.Debug(ctx, "GCPSync: could not remove empty dir", "dir", dir, err)
		}
	}
}
