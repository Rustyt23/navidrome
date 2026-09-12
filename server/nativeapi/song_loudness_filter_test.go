package nativeapi

import (
	"database/sql"

	"github.com/Masterminds/squirrel"
	_ "github.com/mattn/go-sqlite3"
	"github.com/navidrome/navidrome/conf"
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
		oldCeiling := conf.Server.Scanner.LoudnessNormalization.TruePeak
		conf.Server.Scanner.LoudnessNormalization.TruePeak = -0.5
		DeferCleanup(func() { conf.Server.Scanner.LoudnessNormalization.TruePeak = oldCeiling })
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
				tp_before real,
				tp_after real,
				decision text not null default '',
				action text not null default '',
				restored_at datetime
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
				('restored', false),
				('restored-review', false),
				('trim', false),
				('refused-decided', false);

			insert into media_file_loudness (media_file_id, phase, lufs_before, decision, action, restored_at) values
				('unplanned', -1, null, '', '', null),
				('done', 0, -12.6, '', 'skipped', null),
				('done-unmeasured', 0, null, '', '', null),
				('gain', 1, -15.2, '', '', null),
				('trim', 3, -13.5, '', '', null),
				('refused', 1, -13.0, '', 'refused', null),
				('restored', 1, -15.2, '', '', '2026-08-04 09:00:00'),
				('restored-review', 2, -9.1, 'limit', '', '2026-08-04 09:00:00'),
				('review-pending', 2, -9.1, '', '', null),
				('review-limit', 2, -9.1, 'limit', '', null),
				('review-ceiling', 2, -9.1, 'gain_ceiling', '', null),
				('review-skip', 2, -9.1, 'skip', '', null),
				('missing-review', 2, -9.1, 'limit', '', null),
				-- A song the planner called phase 1, which a run built a file for
				-- and refused, which then appeared on the exceptions page and had
				-- a decision made about it. Exactly the five songs sitting in the
				-- real library, and for a while the filter matched none of them.
				('refused-decided', 1, -13.85, 'limit', 'refused', null);
			update media_file_loudness set tp_before = -2 where media_file_id = 'done';
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

		It("revisits completed tracks with unsafe or unknown current peaks", func() {
			for _, phase := range []int{loudness.PhaseDone, loudness.PhaseCloseEnough} {
				for _, peak := range []any{nil, -0.49, 1.0} {
					_, err := db.Exec("update media_file_loudness set phase = ?, tp_before = ?, tp_after = null where media_file_id = 'done'", phase, peak)
					Expect(err).ToNot(HaveOccurred())
					Expect(selectedBy(loudness.PhaseGain)).To(ContainElement("done"))
				}
			}
		})

		It("uses the processed peak instead of the original peak", func() {
			_, err := db.Exec("update media_file_loudness set tp_before = -2, tp_after = -0.1 where media_file_id = 'done'")
			Expect(err).ToNot(HaveOccurred())
			Expect(selectedBy(loudness.PhaseGain)).To(ContainElement("done"))
			_, err = db.Exec("update media_file_loudness set tp_before = 1, tp_after = -0.5 where media_file_id = 'done'")
			Expect(err).ToNot(HaveOccurred())
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

	// A restored song looks exactly like one that was never processed - same
	// phase, same measurements - so a sweep would pick it up and normalize it
	// again, undoing the restore without a word, after every restore.
	Describe("a song whose original was put back", func() {
		It("is left alone by a sweep", func() {
			Expect(selectedBy(loudness.PhaseGain)).ToNot(ContainElement("restored"))
		})

		It("is left alone by a phase 2 run even if a decision survived", func() {
			Expect(selectedBy(loudness.PhaseReview)).ToNot(ContainElement("restored-review"))
		})

		// Picking it out by hand is someone saying "do this one", which outranks
		// a standing preference to leave it be.
		It("is still processed when picked out by hand", func() {
			Expect(selectedWith(loudness.PhaseGain, []string{"restored"})).
				To(Equal([]string{"restored"}))
		})
	})

	Describe("phase 2", func() {
		It("takes only review tracks the client has decided on", func() {
			Expect(selectedBy(loudness.PhaseReview)).
				To(Equal([]string{"refused-decided", "review-ceiling", "review-limit"}))
		})

		// The bug this guards: the filter also required phase = 2, on the
		// assumption that only a review track can carry a decision. Refused
		// tracks are listed on the exceptions page and are phase 1, so a
		// decision on one matched nothing - "Apply decisions" ran over zero
		// songs and reported success.
		It("applies a decision made on a refused phase 1 track", func() {
			Expect(selectedBy(loudness.PhaseReview)).To(ContainElement("refused-decided"))
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
