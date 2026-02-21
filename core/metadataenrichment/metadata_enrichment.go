package metadataenrichment

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/db"
	"github.com/navidrome/navidrome/log"
)

const defaultStatus = "pending"

var (
	extraSpacesRegex = regexp.MustCompile(`\s+`)
	featRegex        = regexp.MustCompile(`\bfeat\.?\b`)
	punctRegex       = regexp.MustCompile(`[^\p{L}\p{N}\s]`)
)

type Service struct {
	dbPath string
}

func NewService() *Service {
	return &Service{dbPath: enrichmentDBPath()}
}

func enrichmentDBPath() string {
	if conf.Server.DbPath == ":memory:" || strings.HasPrefix(conf.Server.DbPath, "file::memory") {
		return "enrichment.db"
	}
	return filepath.Join(filepath.Dir(conf.Server.DbPath), "enrichment.db")
}

func (s *Service) InitAndImport(ctx context.Context) error {
	enrichmentDB, err := sql.Open("sqlite3", s.dbPath)
	if err != nil {
		return fmt.Errorf("open enrichment db: %w", err)
	}
	defer enrichmentDB.Close()

	if err := s.ensureSchema(ctx, enrichmentDB); err != nil {
		return err
	}
	if err := s.importSongs(ctx, db.Db(), enrichmentDB); err != nil {
		return err
	}
	return nil
}

func (s *Service) ensureSchema(ctx context.Context, enrichmentDB *sql.DB) error {
	const stmt = `
create table if not exists tracks (
	id integer primary key autoincrement,
	navidrome_track_id text not null unique,
	file_path text not null,
	title text not null,
	artist text not null,
	album text not null,
	duration real not null,
	status text not null default 'pending',
	normalized_title text not null,
	normalized_artist text not null
);`
	_, err := enrichmentDB.ExecContext(ctx, stmt)
	if err != nil {
		return fmt.Errorf("create tracks table: %w", err)
	}
	return nil
}

func (s *Service) importSongs(ctx context.Context, sourceDB, enrichmentDB *sql.DB) error {
	rows, err := sourceDB.QueryContext(ctx, `
select id, path, title, artist, album, duration
from media_file
`)
	if err != nil {
		return fmt.Errorf("query navidrome songs: %w", err)
	}
	defer rows.Close()

	tx, err := enrichmentDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin enrichment transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
insert into tracks (
	navidrome_track_id,
	file_path,
	title,
	artist,
	album,
	duration,
	status,
	normalized_title,
	normalized_artist
)
values (?, ?, ?, ?, ?, ?, ?, ?, ?)
on conflict(navidrome_track_id) do update set
	file_path=excluded.file_path,
	title=excluded.title,
	artist=excluded.artist,
	album=excluded.album,
	duration=excluded.duration,
	normalized_title=excluded.normalized_title,
	normalized_artist=excluded.normalized_artist
`)
	if err != nil {
		return fmt.Errorf("prepare upsert tracks: %w", err)
	}
	defer stmt.Close()

	for rows.Next() {
		var (
			trackID  string
			path     string
			title    string
			artist   string
			album    string
			duration float64
		)
		if err = rows.Scan(&trackID, &path, &title, &artist, &album, &duration); err != nil {
			return fmt.Errorf("scan song row: %w", err)
		}

		if _, err = stmt.ExecContext(ctx,
			trackID,
			path,
			title,
			artist,
			album,
			duration,
			defaultStatus,
			normalizeMetadata(title),
			normalizeMetadata(artist),
		); err != nil {
			return fmt.Errorf("upsert track %s: %w", trackID, err)
		}
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterate songs rows: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit enrichment import: %w", err)
	}

	log.Info(ctx, "Metadata enrichment import completed", "dbPath", s.dbPath)
	return nil
}

func normalizeMetadata(value string) string {
	normalized := strings.ToLower(value)
	normalized = featRegex.ReplaceAllString(normalized, " ")
	normalized = punctRegex.ReplaceAllString(normalized, " ")
	normalized = extraSpacesRegex.ReplaceAllString(normalized, " ")
	return strings.TrimSpace(normalized)
}
