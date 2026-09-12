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
	It("excludes unsafe peaks from completed totals and lists them for review", func() {
		_, err := GetDBXBuilder().NewQuery(`insert into media_file_loudness
			(media_file_id, phase, lufs_before, tp_before, lufs_after, tp_after) values
			('1001', 0, -20, -8, -12.6, -0.49),
			('1002', 0, -20, -8, -12.6, -0.5),
			('1003', 0, -12.6, null, null, null),
			('1004', 4, -12.9, 1, null, null)`).Execute()
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
		Expect(matched(LoudnessOutcomeFilter("on_target"))).To(Equal([]string{"1002"}))
		Expect(matched(LoudnessExceptionFilter())).To(Equal([]string{"1001", "1004"}))
		Expect(matched(LoudnessLevelTwoFilter())).To(BeEmpty())
	})
})
