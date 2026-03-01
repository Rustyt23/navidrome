package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddNewMBGenreToMediaFile, downAddNewMBGenreToMediaFile)
}

func upAddNewMBGenreToMediaFile(_ context.Context, tx *sql.Tx) error {
	rows, err := tx.Query(`pragma table_info(media_file)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	hasNewMBGenre := false
	hasLegacyMBGenre := false
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
		if name == "new_mb_genre" {
			hasNewMBGenre = true
		}
		if name == "mb_genre" {
			hasLegacyMBGenre = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if !hasNewMBGenre {
		if _, err := tx.Exec(`alter table media_file add new_mb_genre text not null default ''`); err != nil {
			return err
		}
	}

	if hasLegacyMBGenre {
		_, err = tx.Exec(`
			update media_file
			set new_mb_genre = mb_genre
			where trim(ifnull(new_mb_genre, '')) = '' and trim(ifnull(mb_genre, '')) <> ''
		`)
		if err != nil {
			return err
		}
	}

	return nil
}

func downAddNewMBGenreToMediaFile(_ context.Context, _ *sql.Tx) error {
	return nil
}
