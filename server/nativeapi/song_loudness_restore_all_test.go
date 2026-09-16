package nativeapi

import (
	"database/sql"

	"github.com/Masterminds/squirrel"
	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Restore all overwrites library audio with no selection to check against, so
// which songs it picks is exercised as SQL against real rows.
var _ = Describe("loudnessRestoreAllFilter", func() {
	It("picks every song with a stored original that is not already restored", func() {
		db, err := sql.Open("sqlite3", ":memory:")
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() { _ = db.Close() })

		_, err = db.Exec(`
			create table media_file (id text primary key, missing bool not null default false);
			create table media_file_loudness (
				media_file_id text primary key,
				has_backup bool not null default false,
				restored_at datetime
			);
			insert into media_file (id, missing) values
				('optimised', false),
				('optimised-2', false),
				('already-restored', false),
				('no-backup', false),
				('never-analysed', false),
				('missing-file', true);
			insert into media_file_loudness (media_file_id, has_backup, restored_at) values
				('optimised', true, null),
				('optimised-2', true, null),
				('already-restored', true, '2026-09-01 10:00:00'),
				('no-backup', false, null),
				('missing-file', true, null);
		`)
		Expect(err).ToNot(HaveOccurred())

		query, args, err := squirrel.Select("media_file.id").From("media_file").
			LeftJoin("media_file_loudness on media_file_loudness.media_file_id = media_file.id").
			Where(loudnessRestoreAllFilter()).OrderBy("media_file.id").ToSql()
		Expect(err).ToNot(HaveOccurred())
		rows, err := db.Query(query, args...)
		Expect(err).ToNot(HaveOccurred())
		defer rows.Close()

		var ids []string
		for rows.Next() {
			var id string
			Expect(rows.Scan(&id)).To(Succeed())
			ids = append(ids, id)
		}
		Expect(rows.Err()).ToNot(HaveOccurred())
		Expect(ids).To(Equal([]string{"optimised", "optimised-2"}))
	})
})
