package persistence

import (
	"github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/conf"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("loudness outcome peak safety", func() {
	BeforeEach(func() {
		original := conf.Server.Scanner.LoudnessNormalization
		conf.Server.Scanner.LoudnessNormalization.TargetLUFS = -12.6
		conf.Server.Scanner.LoudnessNormalization.TruePeak = -0.5
		conf.Server.Scanner.LoudnessNormalization.Tolerance = 0.2
		DeferCleanup(func() { conf.Server.Scanner.LoudnessNormalization = original })
	})
	AfterEach(func() {
		_, err := GetDBXBuilder().NewQuery("delete from media_file_loudness").Execute()
		Expect(err).ToNot(HaveOccurred())
	})
	It("counts both tolerance boundaries on target without widening the range", func() {
		_, err := GetDBXBuilder().NewQuery(`insert into media_file_loudness
			(media_file_id, phase, lufs_before, tp_before) values
			('1001', 0, -12.80, -1), ('1002', 0, -12.40, -1),
			('1003', 1, -12.81, -1), ('1004', 1, -12.39, -1)`).Execute()
		Expect(err).ToNot(HaveOccurred())
		for _, tc := range []struct {
			filter squirrel.Sqlizer
			ids    string
		}{
			{LoudnessOutcomeFilter("on_target"), "1001,1002"},
			{LoudnessOutcomeFilter("short"), "1003,1004"},
		} {
			query, args, err := squirrel.Select("media_file_id").From("media_file_loudness").Where(tc.filter).OrderBy("media_file_id").ToSql()
			Expect(err).ToNot(HaveOccurred())
			var ids string
			err = GetDBXBuilder().DB().QueryRow("select group_concat(media_file_id) from ("+query+")", args...).Scan(&ids)
			Expect(err).ToNot(HaveOccurred())
			Expect(ids).To(Equal(tc.ids))
		}
	})
	It("excludes unsafe peaks from completed totals and lists them for review", func() {
		_, err := GetDBXBuilder().NewQuery(`insert into media_file_loudness
			(media_file_id, phase, action, lufs_before, tp_before, lufs_after, tp_after) values
			('1001', 0, 'gain', -20, -8, -12.6, -0.49),
			('1002', 0, 'gain', -20, -8, -12.6, -0.5),
			('1003', 0, '', -12.6, null, null, null),
			('1004', 4, '', -12.9, 1, null, null),
			('1005', 3, 'refused', -12.15, 0.63, null, null),
			('1006', 1, '', -20, 0.26, null, null),
			('2001', 0, 'gain', -20, -8, -12.6, -0.3)`).Execute()
		Expect(err).ToNot(HaveOccurred())
		matched := func(filter squirrel.Sqlizer) []string {
			sql, args, err := squirrel.Select("media_file_id").From("media_file_loudness").Where(filter).OrderBy("media_file_id").ToSql()
			Expect(err).ToNot(HaveOccurred())
			query, err := GetDBXBuilder().DB().Query(sql, args...)
			Expect(err).ToNot(HaveOccurred())
			defer query.Close()
			var ids []string
			for query.Next() {
				var id string
				Expect(query.Scan(&id)).To(Succeed())
				ids = append(ids, id)
			}
			Expect(query.Err()).ToNot(HaveOccurred())
			return ids
		}
		// On target is decided on loudness, because a song inside the band is
		// never rewritten and its peak is therefore the client's own master.
		// 1003 has no peak reading at all and 2001 peaks past the bound; both
		// are on target, because the loudness is where the client asked for it.
		Expect(matched(LoudnessOutcomeFilter("on_target"))).To(
			Equal([]string{"1001", "1002", "1003", "2001"}))

		// Only 2001 is left: it is the one file here this project WROTE and
		// shipped past the bound, which is the one peak question worth asking.
		//
		// 1005 was refused. Its peak (0.63) is the master's own - the produced
		// file was thrown away - and at 0.45 from target it sits inside the
		// wider band, so it is left alone rather than put to the client. That
		// is the rule this page exists to honour: a correction that could not
		// land, on a song already near enough, is not a decision for anyone.
		//
		// 1004 and 1006 have only been measured; the next run handles them.
		Expect(matched(LoudnessExceptionFilter())).To(Equal([]string{"2001"}))

		// And the two near-target songs land where they belong instead of
		// vanishing between the categories: 1004 left alone by the planner,
		// 1005 left alone after its correction failed.
		Expect(matched(LoudnessLevelTwoFilter())).To(Equal([]string{"1004", "1005"}))
	})

	// was_exception is a one-way latch: nothing in the application lowers it, by
	// design, so the record of what once needed attention survives. Reading that
	// history as a current state is what kept finished songs on the exceptions
	// page for ever - every song the earlier peak rule wrongly listed was latched
	// on the way past, and re-analysing cannot release it because Put declines to
	// write a non-exception over the latch.
	It("releases a latched song once it is finished, and keeps the others", func() {
		_, err := GetDBXBuilder().NewQuery(`insert into media_file_loudness
			(media_file_id, phase, action, was_exception, lufs_before, tp_before, lufs_after, tp_after) values
			('1001', 0, 'gain', 1, -20, -8, -12.6, -0.43),
			('1002', 1, 'gain', 1, -20, -8, -13.6, -2),
			('1003', 0, 'gain', 1, -20, -8, -12.6, -0.2),
			-- Never rewritten: on target as it was mastered, with the peak a
			-- commercial master ordinarily has. The planner calls this finished
			-- and the peak is not correctable without moving the loudness out of
			-- the band, so the latch must let go of it. This is the shape that
			-- filled the client's page - almost every song it had ever marked.
			('1004', 0, 'skipped', 1, -12.53, -0.26, null, null)`).Execute()
		Expect(err).ToNot(HaveOccurred())
		matched := func(filter squirrel.Sqlizer) []string {
			sql, args, err := squirrel.Select("media_file_id").From("media_file_loudness").Where(filter).OrderBy("media_file_id").ToSql()
			Expect(err).ToNot(HaveOccurred())
			query, err := GetDBXBuilder().DB().Query(sql, args...)
			Expect(err).ToNot(HaveOccurred())
			defer query.Close()
			var ids []string
			for query.Next() {
				var id string
				Expect(query.Scan(&id)).To(Succeed())
				ids = append(ids, id)
			}
			Expect(query.Err()).ToNot(HaveOccurred())
			return ids
		}
		// 1001 and 1004 are on target, so the latch lets go of both - 1004 even
		// though its peak sits above the ceiling, because that peak cannot be
		// corrected without moving the song out of the band.
		//
		// 1002 is still a decibel out, so it stays. 1003 stays too, but not
		// through the latch: it was rewritten and SHIPPED at -0.2, past the
		// bound the engine accepts, which the first arm of the filter catches.
		// That is a peak someone can act on, and the distinction is the whole
		// point - an unreachable peak on an in-band song is not.
		Expect(matched(LoudnessExceptionFilter())).To(Equal([]string{"1002", "1003"}))
	})

	// "Rejected" answers a different question from "needs a decision": how much
	// effort produced nothing, rather than what anyone has to act on. Most
	// rejections near the target are filed as level two and never reach the
	// exceptions page, so the two counts stopped overlapping and the number
	// needs its own way in.
	It("counts every discarded attempt, whether or not anyone must act", func() {
		_, err := GetDBXBuilder().NewQuery(`insert into media_file_loudness
			(media_file_id, phase, action, lufs_before, tp_before, error) values
			('1001', 4, 'refused', -12.97, 0.63, 'true peak 0.12 dBTP exceeds the -0.50 dBTP ceiling'),
			('1002', 3, 'refused', -14.76, 0.56, 'landed at -14.10 LUFS, expected -12.60'),
			('1003', 0, 'skipped', -12.60, -2, ''),
			('1004', 1, 'gain', -13.40, -3, '')`).Execute()
		Expect(err).ToNot(HaveOccurred())

		sql, args, err := squirrel.Select("media_file_id").From("media_file_loudness").
			Where(LoudnessRejectedFilter()).OrderBy("media_file_id").ToSql()
		Expect(err).ToNot(HaveOccurred())
		rows, err := GetDBXBuilder().DB().Query(sql, args...)
		Expect(err).ToNot(HaveOccurred())
		defer rows.Close()
		var ids []string
		for rows.Next() {
			var id string
			Expect(rows.Scan(&id)).To(Succeed())
			ids = append(ids, id)
		}
		Expect(rows.Err()).ToNot(HaveOccurred())

		// 1001 is level two - left alone, nobody has to act - and still counted
		// here, because a file WAS built for it and thrown away. That is the
		// distinction the card exists to show.
		Expect(ids).To(Equal([]string{"1001", "1002"}))
	})

	// The summary's cards open the songs they counted, so each one has to go
	// through the same expression the count did. A card that printed one number
	// and opened a different set would be worse than not opening anything.
	It("opens exactly the songs each summary card counted", func() {
		_, err := GetDBXBuilder().NewQuery(`insert into media_file_loudness
			(media_file_id, phase, action, lufs_before, tp_before) values
			('1001', 0, 'skipped', -12.60, -2),
			('1002', 4, 'skipped', -12.90, -2),
			('1003', 1, 'refused', -14.20, -2),
			('1004', -1, '', null, null)`).Execute()
		Expect(err).ToNot(HaveOccurred())
		matched := func(filter squirrel.Sqlizer) []string {
			sql, args, err := squirrel.Select("media_file_id").From("media_file_loudness").
				Where(filter).OrderBy("media_file_id").ToSql()
			Expect(err).ToNot(HaveOccurred())
			rows, err := GetDBXBuilder().DB().Query(sql, args...)
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
		// The list route and the counting route are the same function, so these
		// cannot drift: 1002 is the only song held to the wider tolerance.
		Expect(matched(LoudnessLevelTwoFilter())).To(Equal([]string{"1002"}))
		Expect(matched(LoudnessOutcomeFilter("not_measured"))).To(Equal([]string{"1004"}))
		Expect(matched(LoudnessRejectedFilter())).To(Equal([]string{"1003"}))
	})

	// Level two means the file was never opened. A song corrected as far as it
	// would go and left a little short sits in the same band and is a different
	// thing entirely - one was spared a re-encode, the other had one.
	//
	// Measured on the client's library: 24 of 37 "level two" songs had been
	// rewritten, one of them lifted 3.2 dB. Reporting those as untouched is the
	// opposite of what the wider tolerance exists to promise.
	It("separates songs left alone from songs corrected as far as they would go", func() {
		_, err := GetDBXBuilder().NewQuery(`insert into media_file_loudness
			(media_file_id, phase, status, action, lufs_before, tp_before, lufs_after, tp_after) values
			-- never opened, 0.30 out: the wider tolerance, as promised
			('1001', 4, 'analyzed', 'skipped', -12.90, -2, null, null),
			-- lifted 3.20 dB and still 0.50 out: corrected, not untouched
			('1002', 1, 'processed', 'gain', -16.30, -8, -13.10, -2),
			-- rewritten and landed on target
			('1003', 0, 'processed', 'gain', -15.00, -8, -12.60, -2),
			-- rewritten, then RESTORED: its file is the original again, so it
			-- counts as untouched once more
			('1004', 4, 'analyzed', 'skipped', -12.85, -2, null, null)`).Execute()
		Expect(err).ToNot(HaveOccurred())
		matched := func(filter squirrel.Sqlizer) []string {
			sql, args, err := squirrel.Select("media_file_id").From("media_file_loudness").
				Where(filter).OrderBy("media_file_id").ToSql()
			Expect(err).ToNot(HaveOccurred())
			rows, err := GetDBXBuilder().DB().Query(sql, args...)
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
		Expect(matched(LoudnessLevelTwoFilter())).To(Equal([]string{"1001", "1004"}))
		Expect(matched(LoudnessShortOfTargetFilter())).To(Equal([]string{"1002"}))
		Expect(matched(LoudnessOutcomeFilter("on_target"))).To(Equal([]string{"1003"}))
	})

	// The panel promises every song is in exactly one group. Narrowing level two
	// without somewhere for the corrected-but-short songs to go would have
	// dropped 24 of them out of the totals on the client's library.
	It("keeps every song in exactly one group", func() {
		_, err := GetDBXBuilder().NewQuery(`insert into media_file_loudness
			(media_file_id, phase, status, action, lufs_before, tp_before, lufs_after, tp_after) values
			('1001', 0, 'analyzed', 'skipped', -12.60, -2, null, null),
			('1002', 4, 'analyzed', 'skipped', -12.90, -2, null, null),
			('1003', 1, 'processed', 'gain', -16.30, -8, -13.10, -2),
			('1004', 2, 'analyzed', '', -9.10, -2, null, null),
			('1005', -1, 'failed', '', null, null, null, null)`).Execute()
		Expect(err).ToNot(HaveOccurred())
		count := func(filter squirrel.Sqlizer) int {
			sql, args, err := squirrel.Select("count(*)").From("media_file_loudness").
				Where(filter).ToSql()
			Expect(err).ToNot(HaveOccurred())
			var n int
			Expect(GetDBXBuilder().DB().QueryRow(sql, args...).Scan(&n)).To(Succeed())
			return n
		}
		total := count(LoudnessOutcomeFilter("on_target")) +
			count(LoudnessLevelTwoFilter()) +
			count(LoudnessShortOfTargetFilter()) +
			count(LoudnessExceptionFilter()) +
			count(LoudnessOutcomeFilter("not_measured"))
		Expect(total).To(Equal(5), "the five groups must add up to the library")
	})
})
