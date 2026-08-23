package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/navidrome/navidrome/model"
)

const (
	snapshotPrefix = "lufs-audit"
	snapshotExt    = ".db"
	// snapshotStamp sorts lexically in time order, so listing is a sort by name.
	snapshotStamp = "20060102-150405"
	// snapshotTmpPrefix deliberately starts with a dot so a half-written file is
	// never listed as a snapshot and never restored from.
	snapshotTmpPrefix = ".incomplete-" + snapshotPrefix
)

// The loudness audit lives in the main database, where it is joined into media
// file queries. That makes it fast to read and impossible to keep if the data
// directory is lost.
//
// These snapshots are the answer: the same rows, written to a SQLite database
// of their own, outside the data directory. Not a live second database - that
// would mean ATTACHing on every pooled connection and giving up the foreign key
// that clears these rows when a song is deleted - but a copy taken whenever a
// job finishes.
//
// Each row carries the song's path and title alongside it. Neither is needed to
// restore, since ids are deterministic hashes of the tags and a rebuilt library
// produces the same ones. They are there so the file can be read by a person,
// and so a row whose id no longer matches can still be identified by hand.

// attach opens a dedicated connection and attaches path as `snap`.
//
// The connection is dedicated because ATTACH belongs to one connection, not to
// the pool: run it on *sql.DB and the next statement may land on a different
// connection where `snap` does not exist. That is not a rare race - it is the
// pool working as designed.
func (r *loudnessAuditRepository) attach(ctx context.Context, path string) (*sql.Conn, func(), error) {
	if r.conn == nil {
		return nil, nil, fmt.Errorf("loudness snapshots need a database connection")
	}
	conn, err := r.conn.Conn(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("opening a connection: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "ATTACH DATABASE ? AS snap", path); err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("attaching %q: %w", path, err)
	}
	// Idempotent, so a caller that has to detach early - before renaming the
	// file, say - can still leave the usual defer in place.
	var once sync.Once
	return conn, func() {
		once.Do(func() {
			_, _ = conn.ExecContext(ctx, "DETACH DATABASE snap")
			_ = conn.Close()
		})
	}, nil
}

// columnsOf lists a table's columns in the given schema.
//
// Read from the database rather than written down here, so a migration that
// adds a column is picked up without this file being touched, and so a snapshot
// taken by an older build can still be restored - the restore uses whatever the
// two have in common.
func columnsOf(ctx context.Context, conn *sql.Conn, schema, table string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, fmt.Sprintf("PRAGMA %s.table_info(%s)", schema, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var (
			cid                 int
			name, colType       string
			notNull, primaryKey int
			defaultValue        sql.NullString
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, rows.Err()
}

func (r *loudnessAuditRepository) Snapshot(dir string, keep int) (*model.LoudnessSnapshot, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("no folder configured for loudness snapshots")
	}
	ctx := r.ctx

	// Nothing to protect, and a copy of nothing still counts against however
	// many are kept - so a few of these would evict every copy that held
	// something. Checked before the folder is created, so a server that has
	// never analysed anything does not leave an empty folder behind either.
	var rows int64
	if err := r.conn.QueryRowContext(ctx,
		fmt.Sprintf("SELECT count(*) FROM %s", r.tableName)).Scan(&rows); err != nil {
		return nil, fmt.Errorf("reading the audit table: %w", err)
	}
	if rows == 0 {
		return nil, model.ErrNoLoudnessAuditData
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating the snapshot folder: %w", err)
	}
	now := time.Now()

	// Written to a temporary name and renamed at the end, so a crash part way
	// through leaves no file that looks like a usable snapshot.
	tmp, err := os.CreateTemp(dir, snapshotTmpPrefix+"-*"+snapshotExt)
	if err != nil {
		return nil, fmt.Errorf("reserving a snapshot file: %w", err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	// SQLite wants to create the file itself.
	if err := os.Remove(tmpPath); err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	conn, done, err := r.attach(ctx, tmpPath)
	if err != nil {
		return nil, err
	}
	defer done()

	// The song's path and title are joined in so the file stands on its own.
	create := fmt.Sprintf(`CREATE TABLE snap.%s AS
		SELECT l.*,
		       coalesce(mf.path, '')  AS song_path,
		       coalesce(mf.title, '') AS song_title
		  FROM main.%s l
		  LEFT JOIN main.media_file mf ON mf.id = l.media_file_id`, r.tableName, r.tableName)
	if _, err := conn.ExecContext(ctx, create); err != nil {
		return nil, fmt.Errorf("copying the audit table: %w", err)
	}

	var rowCount int64
	if err := conn.QueryRowContext(ctx,
		fmt.Sprintf("SELECT count(*) FROM snap.%s", r.tableName)).Scan(&rowCount); err != nil {
		return nil, fmt.Errorf("counting the copied rows: %w", err)
	}

	if _, err := conn.ExecContext(ctx,
		"CREATE TABLE snap.meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)"); err != nil {
		return nil, fmt.Errorf("writing the snapshot header: %w", err)
	}
	for key, value := range map[string]string{
		"created_at": now.UTC().Format(time.RFC3339),
		"rows":       strconv.FormatInt(rowCount, 10),
		"table":      r.tableName,
	} {
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO snap.meta (key, value) VALUES (?, ?)", key, value); err != nil {
			return nil, fmt.Errorf("writing the snapshot header: %w", err)
		}
	}

	// Detach before renaming: the file must not be open when it moves.
	done()

	final := filepath.Join(dir, fmt.Sprintf("%s-%s%s", snapshotPrefix, now.Format(snapshotStamp), snapshotExt))
	if err := os.Rename(tmpPath, final); err != nil {
		return nil, fmt.Errorf("storing the snapshot: %w", err)
	}
	committed = true

	snapshot := &model.LoudnessSnapshot{
		File:      filepath.Base(final),
		Path:      final,
		Rows:      rowCount,
		CreatedAt: now,
	}
	if info, err := os.Stat(final); err == nil {
		snapshot.Size = info.Size()
	}

	r.pruneSnapshots(dir, keep)
	return snapshot, nil
}

// pruneSnapshots keeps the newest `keep`, and always keeps the fullest one.
//
// A keep of zero or less keeps everything: someone who has not chosen a limit
// should not silently get one that deletes their history.
//
// The fullest copy is spared regardless of age, because counting alone is not a
// safety net. Clearing the analysis and then making a handful of small runs
// pushes the copy holding the real library out of the window purely by being
// older, and the one file worth having is the first to go. Age is a reasonable
// way to choose between comparable copies; it is a terrible way to choose
// between six hundred rows and four.
func (r *loudnessAuditRepository) pruneSnapshots(dir string, keep int) {
	if keep <= 0 {
		return
	}
	stored, err := r.Snapshots(dir)
	if err != nil || len(stored) <= keep {
		return
	}

	fullest := 0
	for i, s := range stored {
		if s.Rows > stored[fullest].Rows {
			fullest = i
		}
	}
	for i, old := range stored {
		if i < keep || i == fullest {
			continue
		}
		_ = os.Remove(old.Path)
	}
}

func (r *loudnessAuditRepository) Snapshots(dir string) ([]model.LoudnessSnapshot, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var stored []model.LoudnessSnapshot
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !isSnapshotName(name) {
			continue
		}
		snapshot := model.LoudnessSnapshot{File: name, Path: filepath.Join(dir, name)}
		if info, err := entry.Info(); err == nil {
			snapshot.Size = info.Size()
			snapshot.CreatedAt = info.ModTime()
		}
		if at, err := time.ParseInLocation(snapshotStamp,
			strings.TrimSuffix(strings.TrimPrefix(name, snapshotPrefix+"-"), snapshotExt),
			time.Local); err == nil {
			snapshot.CreatedAt = at
		}
		snapshot.Rows = r.snapshotRowCount(snapshot.Path)
		stored = append(stored, snapshot)
	}
	// Newest first, which is also the order the names sort in.
	slices.SortFunc(stored, func(a, b model.LoudnessSnapshot) int {
		return strings.Compare(b.File, a.File)
	})
	return stored, nil
}

// snapshotRowCount reports how many rows a stored snapshot holds, or -1 when it
// cannot be read. Listing a file that turns out to be unreadable is more useful
// than hiding it: someone looking for a snapshot that is not there needs to
// know it is broken rather than absent.
func (r *loudnessAuditRepository) snapshotRowCount(path string) int64 {
	conn, done, err := r.attach(r.ctx, path)
	if err != nil {
		return -1
	}
	defer done()
	var count int64
	if err := conn.QueryRowContext(r.ctx,
		fmt.Sprintf("SELECT count(*) FROM snap.%s", r.tableName)).Scan(&count); err != nil {
		return -1
	}
	return count
}

func isSnapshotName(name string) bool {
	return strings.HasPrefix(name, snapshotPrefix+"-") && strings.HasSuffix(name, snapshotExt)
}

// RestoreSnapshot makes the audit table match the snapshot exactly: rows it
// holds are written back, and rows it does not are removed.
//
// Only media_file_loudness is touched. Nothing about playlists, play counts,
// ratings or users is part of a snapshot, so nothing about them can be rolled
// back by restoring one.
//
// A snapshot row whose song is not in this library is skipped rather than
// forced in: the row describes audio that is not here, and the foreign key
// would refuse it anyway. It is counted and reported.
func (r *loudnessAuditRepository) RestoreSnapshot(dir, file string) (*model.LoudnessRestoreReport, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("no folder configured for loudness snapshots")
	}
	// The name arrives over HTTP, so it may only ever be a plain file name in
	// the snapshot folder - never a path that walks out of it.
	file = strings.TrimSpace(file)
	if file == "" || file != filepath.Base(file) || !isSnapshotName(file) {
		return nil, fmt.Errorf("%q is not a loudness snapshot", file)
	}
	path := filepath.Join(dir, file)
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("reading %q: %w", file, err)
	}

	ctx := r.ctx
	conn, done, err := r.attach(ctx, path)
	if err != nil {
		return nil, err
	}
	defer done()

	// Restore only what both sides have. A snapshot from an older build is
	// missing whatever has been added since; one from a newer build carries
	// columns this schema does not know about.
	mainCols, err := columnsOf(ctx, conn, "main", r.tableName)
	if err != nil {
		return nil, fmt.Errorf("reading the audit table: %w", err)
	}
	snapCols, err := columnsOf(ctx, conn, "snap", r.tableName)
	if err != nil {
		return nil, fmt.Errorf("reading the snapshot: %w", err)
	}
	var shared []string
	for _, col := range mainCols {
		if slices.Contains(snapCols, col) {
			shared = append(shared, col)
		}
	}
	if !slices.Contains(shared, "media_file_id") {
		return nil, fmt.Errorf("%q does not look like a loudness snapshot", file)
	}

	report := &model.LoudnessRestoreReport{File: file}
	if err := conn.QueryRowContext(ctx,
		fmt.Sprintf("SELECT count(*) FROM snap.%s", r.tableName)).Scan(&report.Rows); err != nil {
		return nil, fmt.Errorf("reading the snapshot: %w", err)
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// Exact replace: anything the snapshot does not know about goes.
	removed, err := tx.ExecContext(ctx, fmt.Sprintf(
		`DELETE FROM main.%s
		  WHERE media_file_id NOT IN (SELECT media_file_id FROM snap.%s)`, r.tableName, r.tableName))
	if err != nil {
		return nil, fmt.Errorf("clearing rows the snapshot does not hold: %w", err)
	}
	report.Removed, _ = removed.RowsAffected()

	columns := strings.Join(shared, ", ")
	restored, err := tx.ExecContext(ctx, fmt.Sprintf(
		`INSERT OR REPLACE INTO main.%s (%s)
		 SELECT %s FROM snap.%s
		  WHERE media_file_id IN (SELECT id FROM main.media_file)`,
		r.tableName, columns, columns, r.tableName))
	if err != nil {
		return nil, fmt.Errorf("writing the snapshot back: %w", err)
	}
	report.Restored, _ = restored.RowsAffected()
	report.Skipped = report.Rows - report.Restored

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing the restore: %w", err)
	}
	return report, nil
}
