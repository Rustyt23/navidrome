package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// orphanTrashPrefix names the folder an orphan is moved into. Orphans are never
// deleted outright: a backup is the only copy of a song the client gave us, and
// a mistake here cannot be undone. Moving them keeps the decision reversible
// until the folder is emptied by hand.
const orphanTrashPrefix = ".orphaned-"

type backupEntry struct {
	Path     string `json:"path"` // relative to the backup root
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

type backupReport struct {
	Root        string        `json:"root"`
	Total       int           `json:"total"`
	TotalBytes  int64         `json:"totalBytes"`
	Orphans     []backupEntry `json:"orphans"`
	OrphanBytes int64         `json:"orphanBytes"`
	LiveSongs   int           `json:"liveSongs"`
	// Safe is false when the library does not look healthy enough to judge what
	// is orphaned. Nothing may be removed while it is false.
	Safe    bool   `json:"safe"`
	Warning string `json:"warning,omitempty"`
}

func (n *Router) loudnessBackupReport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		report, err := buildBackupReport(ctx, n.ds)
		if err != nil {
			log.Error(ctx, "Could not build the backup report", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(report); err != nil {
			log.Error(ctx, "Error sending the backup report", err)
		}
	}
}

// cleanupLoudnessBackups moves orphaned backups aside. It refuses to act on a
// report it has not just rebuilt itself, so a list the client saw minutes ago
// can never authorise removing files that have since become live again.
func (n *Router) cleanupLoudnessBackups() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		report, err := buildBackupReport(ctx, n.ds)
		if err != nil {
			log.Error(ctx, "Could not build the backup report", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if !report.Safe {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(report)
			return
		}

		trash := filepath.Join(report.Root, orphanTrashPrefix+time.Now().Format("20060102-150405"))
		moved := 0
		for _, entry := range report.Orphans {
			src := filepath.Join(report.Root, entry.Path)
			dst := filepath.Join(trash, entry.Path)
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				log.Warn(ctx, "Could not prepare the orphan folder", "path", dst, err)
				continue
			}
			if err := os.Rename(src, dst); err != nil {
				log.Warn(ctx, "Could not move an orphaned backup", "path", src, err)
				continue
			}
			moved++
		}
		log.Info(ctx, "Orphaned backups moved aside", "moved", moved, "folder", trash)

		_ = json.NewEncoder(w).Encode(map[string]any{
			"moved":      moved,
			"folder":     trash,
			"freedBytes": report.OrphanBytes,
		})
	}
}

// buildBackupReport walks the backup folder and works out which stored
// originals no longer protect anything.
//
// A backup is matched to a song by the identity in its name, so a song that has
// been renamed or moved still claims its own backup. Backups written by an
// earlier version carry no identity, and are matched by the library path they
// mirror - the best that can be done for them.
func buildBackupReport(ctx context.Context, ds model.DataStore) (backupReport, error) {
	options := conf.Server.Scanner.LoudnessNormalization
	report := backupReport{}

	liveIDs := map[string]struct{}{}
	legacyPaths := map[string]struct{}{}
	cursor, err := ds.MediaFile(ctx).GetCursor()
	if err != nil {
		return report, err
	}
	for mf, err := range cursor {
		if err != nil {
			return report, err
		}
		if strings.TrimSpace(mf.Path) == "" || strings.TrimSpace(mf.LibraryPath) == "" {
			continue
		}
		if report.Root == "" {
			report.Root = ffmpeg.LoudnessBackupRoot(options.BackupFolder, mf.LibraryPath)
		}
		report.LiveSongs++
		liveIDs[mf.ID] = struct{}{}
		trackPath := absoluteSelectedMediaPath(mf.LibraryPath, mf.Path)
		if p := ffmpeg.LegacyLoudnessBackupPath(options.BackupFolder, mf.LibraryPath, trackPath); p != "" {
			legacyPaths[filepath.Clean(p)] = struct{}{}
		}
	}
	if report.Root == "" {
		report.Root = ffmpeg.LoudnessBackupRoot(options.BackupFolder, "")
	}
	if report.Root == "" {
		return report, fmt.Errorf("no backup folder is configured")
	}

	// Deciding what is orphaned means trusting that the library we just read is
	// the whole library. An empty one is what an unmounted drive or a database
	// that has not been scanned looks like, and treating that as "nothing is
	// live" would condemn every original at once.
	if report.LiveSongs == 0 {
		report.Warning = "no songs were found in the library, so nothing can be judged orphaned - check the library is present and scanned"
		return report, nil
	}

	err = filepath.WalkDir(report.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		name := d.Name()
		if d.IsDir() {
			// Skip the folders orphans were moved into, and anything hidden.
			if path != report.Root && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		report.Total++
		report.TotalBytes += info.Size()

		if backupIsLive(report.Root, path, name, liveIDs, legacyPaths) {
			return nil
		}
		rel, relErr := filepath.Rel(report.Root, path)
		if relErr != nil {
			rel = path
		}
		report.Orphans = append(report.Orphans, backupEntry{
			Path:     rel,
			Size:     info.Size(),
			Modified: info.ModTime().Format(time.RFC3339),
		})
		report.OrphanBytes += info.Size()
		return nil
	})
	if err != nil {
		return report, err
	}

	report.Safe = true
	if report.Total > 0 && len(report.Orphans) == report.Total {
		// Every single backup looking orphaned is the signature of a library
		// that moved wholesale, not of every song having been deleted.
		report.Safe = false
		report.Warning = "every stored original looks orphaned, which usually means the library moved rather than that the songs are gone - nothing was removed"
	}
	return report, nil
}

// backupIsLive reports whether a file in the backup folder still protects a
// song that exists.
func backupIsLive(root, path, name string, liveIDs, legacyPaths map[string]struct{}) bool {
	if id, ok := backupIDFromName(root, path, name); ok {
		_, live := liveIDs[id]
		return live
	}
	_, live := legacyPaths[filepath.Clean(path)]
	return live
}

// backupIDFromName recovers the song identity from a backup written under the
// identity-based layout: "<root>/<first two of id>/<id>__<original name>".
// Matching on the id alone rather than the whole name is what lets a song be
// renamed without its backup appearing to be orphaned.
func backupIDFromName(root, path, name string) (string, bool) {
	idx := strings.Index(name, "__")
	if idx <= 0 {
		return "", false
	}
	id := name[:idx]
	if len(id) < 2 {
		return "", false
	}
	// The sharding directory has to agree, or this is a coincidence in a
	// legacy filename rather than an identity-based backup.
	if filepath.Base(filepath.Dir(path)) != id[:2] {
		return "", false
	}
	if filepath.Dir(filepath.Dir(path)) != filepath.Clean(root) {
		return "", false
	}
	return id, true
}
