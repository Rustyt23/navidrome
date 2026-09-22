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
		// 1001 (-0.49) and 1002 (-0.5) both ship: the bound here is
		// model.LoudnessShippingCeiling, the same one Optimize accepted them
		// against, so a file inside the measurement tolerance counts as finished
		// instead of being dropped from the total and listed for review. 2001
		// (-0.3) is beyond that tolerance and still counts as unsafe, which is
		// what stops the tolerance reading as a licence to ship anything.
		Expect(matched(LoudnessOutcomeFilter("on_target"))).To(Equal([]string{"1001", "1002"}))
		// 1004 and 1006 have only been measured: the next run handles
		// them, so they are not exceptions yet - however far out they are. 1005
		// was refused with its peaks still over, which is, near target or not.
		Expect(matched(LoudnessExceptionFilter())).To(Equal([]string{"1005", "2001"}))
		Expect(matched(LoudnessLevelTwoFilter())).To(BeEmpty())
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
})
