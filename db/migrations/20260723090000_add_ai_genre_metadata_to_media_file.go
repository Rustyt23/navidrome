package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddAIGenreMetadataToMediaFile, downAddAIGenreMetadataToMediaFile)
}

// The AI tool page used to keep fetched genres in browser storage only, so
// clearing site data lost every genre the user had paid an API call for while
// the album, year and explicit status it fetched alongside them survived. These
// columns give the fetched genres the same durability as the rest.
func upAddAIGenreMetadataToMediaFile(_ context.Context, tx *sql.Tx) error {
	type columnSpec struct {
		name string
		ddl  string
	}

	columns := []columnSpec{
		{name: "ai_genre", ddl: `alter table media_file add ai_genre text not null default ''`},
		{name: "ai_subgenre", ddl: `alter table media_file add ai_subgenre text not null default ''`},
		{name: "spotify_genre", ddl: `alter table media_file add spotify_genre text not null default ''`},
		{name: "itunes_genre", ddl: `alter table media_file add itunes_genre text not null default ''`},
		{name: "genre_confidence", ddl: `alter table media_file add genre_confidence integer not null default 0`},
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

func downAddAIGenreMetadataToMediaFile(_ context.Context, _ *sql.Tx) error {
	return nil
}
