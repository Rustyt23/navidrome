package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddSpotifyMetadataToMediaFile, downAddSpotifyMetadataToMediaFile)
}

func upAddSpotifyMetadataToMediaFile(_ context.Context, tx *sql.Tx) error {
	type columnSpec struct {
		name string
		ddl  string
	}

	columns := []columnSpec{
		{name: "spotify_confidence", ddl: `alter table media_file add spotify_confidence real not null default 0`},
		{name: "spotify_match", ddl: `alter table media_file add spotify_match text not null default ''`},
		{name: "spotify_artist", ddl: `alter table media_file add spotify_artist text not null default ''`},
		{name: "spotify_url", ddl: `alter table media_file add spotify_url text not null default ''`},
	}

	for _, col := range columns {
		hasColumn, err := migrationHasColumn(tx, "media_file", col.name)
		if err != nil {
			return err
		}
		if hasColumn {
			continue
		}
		if _, err := tx.Exec(col.ddl); err != nil {
			return err
		}
	}

	return nil
}

func migrationHasColumn(tx *sql.Tx, table, column string) (bool, error) {
	rows, err := tx.Query(`pragma table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notnull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func downAddSpotifyMetadataToMediaFile(_ context.Context, _ *sql.Tx) error {
	return nil
}
