package gcpsync

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/navidrome/navidrome/utils/filelock"
)

// Always acquire this before staged file locks. Snapshot preparation shares
// this gate; transfers release it, and upload cleanup takes only the file lock.
var stagingMutationMu sync.Mutex

type stagedOwner struct {
	MediaFileID string `json:"mediaFileId"`
	owned       bool
}

func readStagedOwner(path string) (stagedOwner, error) {
	data, err := os.ReadFile(libraryCopyMarker(path))
	if os.IsNotExist(err) {
		return stagedOwner{}, nil
	}
	if err != nil {
		return stagedOwner{}, err
	}
	owner := stagedOwner{owned: true}
	if bytes.Equal(bytes.TrimSpace(data), []byte("library copy")) {
		return owner, nil // Provenance written by older versions.
	}
	if err := json.Unmarshal(data, &owner); err != nil {
		return stagedOwner{}, fmt.Errorf("reading staged song identity for %q: %w", path, err)
	}
	return owner, nil
}

func writeStagedMarker(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".gcpsync-marker-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func compatibleStagedOwners(a, b stagedOwner) bool {
	return a.MediaFileID == "" || b.MediaFileID == "" || a.MediaFileID == b.MediaFileID
}

func lockStagedPair(a, b string) func() {
	if a > b {
		a, b = b, a
	}
	first := filelock.Lock(a)
	second := filelock.Lock(b)
	return func() { second(); first() }
}

func stagedMP3Paths(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() && !strings.HasPrefix(d.Name(), ".") && strings.EqualFold(filepath.Ext(path), ".mp3") {
			paths = append(paths, path)
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	return paths, err
}

func identicalStagedBytes(a, b string) (bool, error) {
	beforeA, err := os.Lstat(a)
	if err != nil {
		return false, err
	}
	beforeB, err := os.Lstat(b)
	if err != nil {
		return false, err
	}
	if !beforeA.Mode().IsRegular() || !beforeB.Mode().IsRegular() || beforeA.Size() != beforeB.Size() {
		return false, nil
	}
	digest := func(path string) ([]byte, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return nil, err
		}
		return h.Sum(nil), nil
	}
	hashA, err := digest(a)
	if err != nil {
		return false, err
	}
	hashB, err := digest(b)
	if err != nil {
		return false, err
	}
	if !bytes.Equal(hashA, hashB) {
		return false, nil
	}
	// External replacements must not turn a checked alias into different audio.
	afterA, err := os.Lstat(a)
	if err != nil {
		return false, err
	}
	afterB, err := os.Lstat(b)
	if err != nil {
		return false, err
	}
	unchanged := func(before, after os.FileInfo) bool {
		return os.SameFile(before, after) && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
	}
	return unchanged(beforeA, afterA) && unchanged(beforeB, afterB), nil
}

func removeStagedCopy(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(libraryCopyMarker(path)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func hasArtistTitleName(path string) bool {
	base := filepath.Base(path)
	artist, title, found := strings.Cut(strings.TrimSuffix(base, filepath.Ext(base)), " - ")
	return found && strings.TrimSpace(artist) != "" && strings.TrimSpace(title) != ""
}

// Caller holds stagingMutationMu and the source lock. A formatted current
// filename wins; a raw filename can reuse a proven formatted sibling. The name
// is only a display preference and never supplies missing embedded metadata.
func preferredStagedPath(nominal, source, mediaFileID string, candidates []string) string {
	if hasArtistTitleName(nominal) {
		return nominal
	}
	legacyAlias := ""
	for _, candidate := range candidates {
		if candidate == nominal || filepath.Dir(candidate) != filepath.Dir(nominal) || !hasArtistTitleName(candidate) {
			continue
		}
		unlock := filelock.Lock(candidate)
		owner, err := readStagedOwner(candidate)
		info, statErr := os.Lstat(candidate)
		if err == nil && statErr == nil && info.Mode().IsRegular() {
			if mediaFileID != "" && owner.MediaFileID == mediaFileID {
				unlock()
				return candidate
			}
			if owner.MediaFileID == "" {
				var same bool
				same, err = identicalStagedBytes(source, candidate)
				if err == nil && same && legacyAlias == "" {
					legacyAlias = candidate
				}
			}
		}
		unlock()
		// An unrelated unreadable alias must not prevent the new edit from
		// reaching Sync. Unproven candidates stay intact; cleanup reports errors.
	}
	if legacyAlias != "" {
		return legacyAlias
	}
	return nominal
}

// Caller holds the source and destination locks. Publishing the marker before
// the complete audio keeps an interrupted replacement app-owned after restart.
func publishStagedCopy(source, dest, mediaFileID string) error {
	previousMarker, markerErr := os.ReadFile(libraryCopyMarker(dest))
	if markerErr != nil && !os.IsNotExist(markerErr) {
		return markerErr
	}
	if err := markLibraryCopy(dest, mediaFileID); err != nil {
		return err
	}
	if err := copyFile(source, dest); err != nil {
		var restoreErr error
		if markerErr == nil {
			restoreErr = writeStagedMarker(libraryCopyMarker(dest), previousMarker)
		} else {
			restoreErr = os.Remove(libraryCopyMarker(dest))
		}
		return errors.Join(err, restoreErr)
	}
	return nil
}

// Caller holds stagingMutationMu. Copy the latest bytes to the preferred name
// before retiring any aliases; a failed replacement leaves the latest intact.
func publishPreferredStagedCopy(source, dest string) error {
	unlock := lockStagedPair(source, dest)
	defer unlock()
	owner, err := readStagedOwner(source)
	if err != nil {
		return err
	}
	if !owner.owned {
		return fmt.Errorf("pending source %q is no longer an app-owned copy", source)
	}
	other, err := readStagedOwner(dest)
	if err != nil {
		return err
	}
	if !compatibleStagedOwners(owner, other) || (owner.MediaFileID == "" && other.MediaFileID != "") {
		return fmt.Errorf("preferred staging destination %q belongs to another song", dest)
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("pending source %q is not a regular file", source)
	}
	if destInfo, err := os.Lstat(dest); err == nil {
		if !destInfo.Mode().IsRegular() {
			return fmt.Errorf("preferred staging destination %q is not a regular file", dest)
		}
		if owner.MediaFileID == "" || other.MediaFileID != owner.MediaFileID {
			same, err := identicalStagedBytes(source, dest)
			if err != nil {
				return err
			}
			if !same {
				return fmt.Errorf("preferred staging destination %q contains different pending audio", dest)
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return publishStagedCopy(source, dest, owner.MediaFileID)
}

// The caller holds stagingMutationMu and has already published keep completely.
func coalesceStagedCopies(keep string, candidates []string) error {
	var failures []error
	for _, candidate := range candidates {
		if candidate == keep {
			continue
		}
		unlock := lockStagedPair(keep, candidate)
		err := coalesceStagedPair(keep, candidate)
		unlock()
		if err != nil && !os.IsNotExist(err) {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func coalesceStagedPair(keep, candidate string) error {
	keeper, err := readStagedOwner(keep)
	if err != nil || !keeper.owned {
		return err
	}
	owner, err := readStagedOwner(candidate)
	if err != nil || !compatibleStagedOwners(keeper, owner) {
		return err
	}
	// The current song's stable ID proves identity even if its cover, tags,
	// filename, folder, or normalized audio has changed since the earlier edit.
	if keeper.MediaFileID != "" && keeper.MediaFileID == owner.MediaFileID {
		if info, err := os.Lstat(keep); err != nil || !info.Mode().IsRegular() {
			return err
		}
		if info, err := os.Lstat(candidate); err != nil || !info.Mode().IsRegular() {
			return err
		}
		return removeStagedCopy(candidate)
	}
	// Older copies have no ID. Only exact siblings may be treated as aliases;
	// matching titles, placeholders, or similar filenames never prove identity.
	if filepath.Dir(keep) != filepath.Dir(candidate) || (keeper.MediaFileID == "" && owner.MediaFileID != "") {
		return nil
	}
	same, err := identicalStagedBytes(keep, candidate)
	if err != nil || !same {
		return err
	}
	return removeStagedCopy(candidate)
}

// ReconcileStagedMP3s repairs older duplicate pending copies without uploading
// or changing tags. Identified songs keep their latest complete bytes under a
// formatted name when available; legacy duplicates require matching bytes and
// an app-owned survivor in the same folder.
func ReconcileStagedMP3s() error {
	root := stagingRoot()
	if root == "" || dryRun() {
		return nil
	}
	stagingMutationMu.Lock()
	defer stagingMutationMu.Unlock()
	paths, err := stagedMP3Paths(root)
	if err != nil {
		return err
	}
	// Prefer identified copies over legacy markers, then the latest staged edit.
	type pending struct {
		path  string
		owner stagedOwner
		info  os.FileInfo
	}
	var marked []pending
	for _, path := range paths {
		unlock := filelock.Lock(path)
		owner, ownerErr := readStagedOwner(path)
		info, statErr := os.Lstat(path)
		unlock()
		if ownerErr != nil {
			return ownerErr
		}
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return statErr
		}
		if owner.owned && info.Mode().IsRegular() {
			marked = append(marked, pending{path, owner, info})
		}
	}
	sort.SliceStable(marked, func(i, j int) bool {
		if (marked[i].owner.MediaFileID != "") != (marked[j].owner.MediaFileID != "") {
			return marked[i].owner.MediaFileID != ""
		}
		return marked[i].info.ModTime().After(marked[j].info.ModTime())
	})
	var failures []error
	identified := make(map[string]bool)
	for _, keep := range marked {
		if id := keep.owner.MediaFileID; id != "" {
			// Even if removing an older alias fails, it must never become a
			// keeper itself and retire the successfully staged latest version.
			if identified[id] {
				continue
			}
			identified[id] = true
		}
		unlock := filelock.Lock(keep.path)
		preferred := preferredStagedPath(keep.path, keep.path, keep.owner.MediaFileID, paths)
		unlock()
		if preferred != keep.path {
			if err := publishPreferredStagedCopy(keep.path, preferred); err != nil {
				if !os.IsNotExist(err) {
					failures = append(failures, err)
				}
				continue
			}
		}
		if err := coalesceStagedCopies(preferred, paths); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// A successful StageSong rename can also retire its known legacy marked path.
// Explicit IDs are checked so a reused filename cannot remove another song.
func retireRenamedStagedCopy(path, mediaFileID string) error {
	stagingMutationMu.Lock()
	defer stagingMutationMu.Unlock()
	unlock := filelock.Lock(path)
	defer unlock()
	owner, err := readStagedOwner(path)
	if err != nil || !owner.owned || (owner.MediaFileID != "" && owner.MediaFileID != mediaFileID) {
		return err
	}
	return removeStagedCopy(path)
}

// Caller holds stagingMutationMu. An unlink failure can leave two versions
// pending; retain both until cleanup works instead of uploading them in name
// order and potentially putting the older version into the bucket last.
func checkUniqueStagedIdentity(path, mediaFileID string) error {
	if mediaFileID == "" {
		return nil
	}
	paths, err := stagedMP3Paths(stagingRoot())
	if err != nil {
		return err
	}
	for _, other := range paths {
		if other == path {
			continue
		}
		owner, err := readStagedOwner(other)
		if err != nil {
			// An unrelated malformed marker must not block this song.
			continue
		}
		if owner.MediaFileID == mediaFileID {
			return fmt.Errorf("song %q has multiple pending copies; resolve the staging cleanup error and retry Sync", mediaFileID)
		}
	}
	return nil
}
