package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddCoverPathToMediaFile, downAddCoverPathToMediaFile)
}

func upAddCoverPathToMediaFile(_ context.Context, tx *sql.Tx) error {
	rows, err := tx.Query(`pragma table_info(media_file)`)
	if err != nil {
		return err
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
			return err
		}
		if name == "cover_path" {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = tx.Exec(`alter table media_file add cover_path text not null default ''`)
	return err
}

func downAddCoverPathToMediaFile(_ context.Context, _ *sql.Tx) error {
	return nil
}
