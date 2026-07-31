package nativeapi

import (
	"database/sql"

	"github.com/Masterminds/squirrel"
	_ "github.com/mattn/go-sqlite3"
	"github.com/navidrome/navidrome/core/loudness"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The run filter decides which tracks a run opens at all, and it reads columns
// that are NULL for every track without an audit row - where no comparison is
// true and a careless condition silently drops exactly the tracks that most
// need processing. So it is exercised as SQL against real rows rather than
// checked as a string.
var _ = Describe("loudnessRunFilter", func() {
	var db *sql.DB

	selectedWith := func(phase int, only []string) []string {
		query := squirrel.Select("media_file.id").From("media_file").
			LeftJoin("media_file_loudness on media_file_loudness.media_file_id = media_file.id").
			Where(loudnessRunFilter(phase, only)).OrderBy("media_file.id")
		sqlStr, args, err := query.ToSql()
		Expect(err).ToNot(HaveOccurred())

		rows, err := db.Query(sqlStr, args...)
		Expect(err).ToNot(HaveOccurred())
		defer rows.Close()

		var ids []string
		for rows.Next() {
			var id string
			Expect(rows.Scan(&id)).To(Succeed())
			ids = append(ids, id)
		}
		Expect(rows.Err()).ToNot(HaveOccurred())
		return ids
	}

	selectedBy := func(phase int) []string { return selectedWith(phase, nil) }

	BeforeEach(func() {
		var err error
		db, err = sql.Open("sqlite3", ":memory:")
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() { _ = db.Close() })

		_, err = db.Exec(`
			create table media_file (id text primary key, missing bool not null default false);
			create table media_file_loudness (
				media_file_id text primary key,
				phase integer not null default -1,
				lufs_before real,
				decision text not null default '',
				action text not null default ''
			);

			insert into media_file (id, missing) values
				('never-analysed', false),
				('unplanned', false),
				('done', false),
				('done-unmeasured', false),
				('gain', false),
				('review-pending', false),
				('review-limit', false),
				('review-ceiling', false),
				('review-skip', false),
				('missing-file', true),
				('missing-review', true),
				('refused', false),
				('trim', false);

			insert into media_file_loudness (media_file_id, phase, lufs_before, decision, action) values
				('unplanned', -1, null, '', ''),
				('done', 0, -12.6, '', 'skipped'),
				('done-unmeasured', 0, null, '', ''),
				('gain', 1, -15.2, '', ''),
				('trim', 3, -13.5, '', ''),
				('refused', 1, -13.0, '', 'refused'),
				('review-pending', 2, -9.1, '', ''),
				('review-limit', 2, -9.1, 'limit', ''),
				('review-ceiling', 2, -9.1, 'gain_ceiling', ''),
				('review-skip', 2, -9.1, 'skip', ''),
				('missing-review', 2, -9.1, 'limit', '');
		`)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("phase 1", func() {
		It("includes tracks nothing is known about yet", func() {
			// A track with no audit row joins as NULL throughout; one that has
			// been written but never planned carries the column default.
			Expect(selectedBy(loudness.PhaseGain)).To(ContainElements("never-analysed", "unplanned"))
		})

		It("includes tracks known to need a gain", func() {
			Expect(selectedBy(loudness.PhaseGain)).To(ContainElement("gain"))
		})

		It("skips tracks already measured as on target", func() {
			Expect(selectedBy(loudness.PhaseGain)).ToNot(ContainElement("done"))
		})

		It("still opens an on-target track that was never measured", func() {
			// Nothing was measured, so "on target" is not established and the
			// engine has to look for itself.
			Expect(selectedBy(loudness.PhaseGain)).To(ContainElement("done-unmeasured"))
		})

		It("leaves every review track to phase 2", func() {
			Expect(selectedBy(loudness.PhaseGain)).ToNot(ContainElements(
				"review-pending", "review-limit", "review-ceiling", "review-skip"))
		})

		It("skips files that are not on disk", func() {
			Expect(selectedBy(loudness.PhaseGain)).ToNot(ContainElement("missing-file"))
		})

		It("picks up a track whose peaks need an inaudible trim", func() {
			// Applied automatically, so it belongs to the phase 1 run and must
			// never wait on the review page for a decision nobody would weigh.
			Expect(selectedBy(loudness.PhaseGain)).To(ContainElement("trim"))
			Expect(selectedBy(loudness.PhaseReview)).ToNot(ContainElement("trim"))
		})

		It("skips a track whose last attempt was built and rejected", func() {
			// Nothing about it or the settings has changed, so rebuilding the
			// same file would reach the same refusal - on every run, for ever.
			// A fresh analysis clears the mark and it is tried again.
			Expect(selectedBy(loudness.PhaseGain)).ToNot(ContainElement("refused"))
		})
	})

	// Optimising a hand-picked selection runs the same job, scoped by id. The
	// point of the scope is that it overrides the planner: someone who selects a
	// song has decided it should be tried, and the clauses that keep a sweep
	// from redoing work must not quietly drop it.
	Describe("a selected set of songs", func() {
		It("covers exactly the songs asked for", func() {
			Expect(selectedWith(loudness.PhaseGain, []string{"gain", "done"})).
				To(Equal([]string{"done", "gain"}))
		})

		It("retries a track a run refused, which a sweep would skip", func() {
			Expect(selectedBy(loudness.PhaseGain)).ToNot(ContainElement("refused"))
			Expect(selectedWith(loudness.PhaseGain, []string{"refused"})).
				To(ContainElement("refused"))
		})

		It("opens a review track without waiting for a decision", func() {
			Expect(selectedWith(loudness.PhaseGain, []string{"review-pending"})).
				To(ContainElement("review-pending"))
		})

		It("still refuses to touch a file that is not on disk", func() {
			Expect(selectedWith(loudness.PhaseGain, []string{"missing-file"})).To(BeEmpty())
		})
	})

	Describe("phase 2", func() {
		It("takes only review tracks the client has decided on", func() {
			Expect(selectedBy(loudness.PhaseReview)).To(Equal([]string{"review-ceiling", "review-limit"}))
		})

		It("waits for a decision rather than guessing", func() {
			Expect(selectedBy(loudness.PhaseReview)).ToNot(ContainElement("review-pending"))
		})

		It("honours a decision to leave the track alone", func() {
			Expect(selectedBy(loudness.PhaseReview)).ToNot(ContainElement("review-skip"))
		})

		It("skips files that are not on disk", func() {
			Expect(selectedBy(loudness.PhaseReview)).ToNot(ContainElement("missing-review"))
		})
	})
})
