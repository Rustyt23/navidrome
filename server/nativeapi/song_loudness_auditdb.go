package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

// auditSnapshotFolderName is the folder copies of the audit go in when no
// location is configured. Hidden, so the backup report's walk skips it and the
// copies are never mistaken for orphaned originals.
const auditSnapshotFolderName = ".lufs-db"

// loudnessAuditDbFolder resolves where copies of the audit are kept.
//
// A configured folder wins. Otherwise they go beside the stored originals,
// which are already outside the data directory - the same folder that already
// holds the one thing here that cannot be reproduced at all.
//
// Returns "" when there is nowhere to put them, which is the same condition
// that leaves backups nowhere to go: no library path and no configured folder.
func loudnessAuditDbFolder(ctx context.Context, ds model.DataStore) string {
	options := conf.Server.Scanner.LoudnessNormalization
	if folder := strings.TrimSpace(options.AuditDbFolder); folder != "" {
		return filepath.Clean(folder)
	}
	root := ffmpeg.LoudnessBackupRoot(options.BackupFolder, firstLibraryPath(ctx, ds))
	if root == "" {
		return ""
	}
	return filepath.Join(root, auditSnapshotFolderName)
}

// firstLibraryPath finds a library to derive the default location from. Only
// used when neither a snapshot folder nor a backup folder is configured.
func firstLibraryPath(ctx context.Context, ds model.DataStore) string {
	libraries, err := ds.Library(ctx).GetAll()
	if err != nil {
		log.Warn(ctx, "Could not read libraries for the LUFS snapshot folder", err)
		return ""
	}
	for _, lib := range libraries {
		if strings.TrimSpace(lib.Path) != "" {
			return lib.Path
		}
	}
	return ""
}

// snapshotLoudnessAudit copies the audit table after a job.
//
// Failing to copy is never allowed to fail the job that triggered it: the work
// is already done and recorded, and refusing to admit that because a copy could
// not be written would be the worse outcome. It is logged loudly instead.
func snapshotLoudnessAudit(ctx context.Context, ds model.DataStore, reason string) {
	dir := loudnessAuditDbFolder(ctx, ds)
	if dir == "" {
		log.Debug(ctx, "No folder for LUFS audit copies, skipping", "reason", reason)
		return
	}
	keep := conf.Server.Scanner.LoudnessNormalization.AuditDbKeep
	snapshot, err := ds.LoudnessAudit(ctx).Snapshot(dir, keep)
	switch {
	case errors.Is(err, model.ErrNoLoudnessAuditData):
		// Not a failure: a run that measured nothing has nothing to copy.
		log.Debug(ctx, "No LUFS audit data to copy", "reason", reason)
		return
	case err != nil:
		log.Error(ctx, "Could not copy the LUFS audit data", "reason", reason, "folder", dir, err)
		return
	}
	log.Info(ctx, "LUFS audit data copied", "reason", reason, "file", snapshot.File,
		"rows", snapshot.Rows, "folder", dir)
}

func (n *Router) loudnessAuditDbList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		w.Header().Set("Content-Type", "application/json")

		dir := loudnessAuditDbFolder(ctx, n.ds)
		stored, err := n.ds.LoudnessAudit(ctx).Snapshots(dir)
		if err != nil {
			log.Error(ctx, "Could not list the LUFS audit copies", "folder", dir, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if stored == nil {
			stored = []model.LoudnessSnapshot{}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"folder":    dir,
			"snapshots": stored,
		})
	}
}

func (n *Router) loudnessAuditDbSnapshot() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		w.Header().Set("Content-Type", "application/json")
		release, busy := claimLoudnessFileWork(&loudnessAuditWork)
		if busy != "" {
			http.Error(w, "Wait for "+busy+" before copying LUFS audit data", http.StatusConflict)
			return
		}
		defer release()

		dir := loudnessAuditDbFolder(ctx, n.ds)
		if dir == "" {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"message": "No folder to keep LUFS audit copies in - set Scanner.LoudnessNormalization.AuditDbFolder",
			})
			return
		}
		keep := conf.Server.Scanner.LoudnessNormalization.AuditDbKeep
		snapshot, err := n.ds.LoudnessAudit(ctx).Snapshot(dir, keep)
		if errors.Is(err, model.ErrNoLoudnessAuditData) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"message": "There is no LUFS data to back up yet - analyse some songs first",
			})
			return
		}
		if err != nil {
			log.Error(ctx, "Could not copy the LUFS audit data", "folder", dir, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Info(ctx, "LUFS audit data copied on request", "file", snapshot.File, "rows", snapshot.Rows)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"folder":   dir,
			"snapshot": snapshot,
		})
	}
}

type loudnessAuditRestorePayload struct {
	// File names the copy to restore. Empty means the most recent one.
	File string `json:"file"`
}

// loudnessAuditDbRestore puts a stored copy of the audit back.
//
// It replaces exactly: rows the copy holds are written back, rows it does not
// hold are removed, and nothing outside the audit table is touched. A job
// running at the same time would be writing the very rows this rewrites, so it
// claims the same right they do.
func (n *Router) loudnessAuditDbRestore() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		w.Header().Set("Content-Type", "application/json")

		var payload loudnessAuditRestorePayload
		if r.Body != nil {
			// An empty body is allowed and means "the most recent one".
			_ = json.NewDecoder(r.Body).Decode(&payload)
		}

		release, busy := claimLoudnessFileWork(&loudnessAuditWork)
		if busy != "" {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"message": "Stop " + busy + " before restoring LUFS audit data",
			})
			return
		}
		defer release()

		repo := n.ds.LoudnessAudit(ctx)
		dir := loudnessAuditDbFolder(ctx, n.ds)

		file := strings.TrimSpace(payload.File)
		if file == "" {
			stored, err := repo.Snapshots(dir)
			if err != nil {
				log.Error(ctx, "Could not list the LUFS audit copies", "folder", dir, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if len(stored) == 0 {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"message": "There are no stored copies of the LUFS audit data yet",
				})
				return
			}
			file = stored[0].File
		}

		report, err := repo.RestoreSnapshot(dir, file)
		if err != nil {
			log.Error(ctx, "Could not restore the LUFS audit data", "file", file, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Warn(ctx, "LUFS audit data restored", "file", report.File, "restored", report.Restored,
			"removed", report.Removed, "skipped", report.Skipped)
		_ = json.NewEncoder(w).Encode(report)
	}
}
