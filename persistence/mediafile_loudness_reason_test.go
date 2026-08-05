package persistence

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The exceptions page groups songs by why they are listed so a whole cause can
// be actioned in one go. That is only safe if the filter picks exactly the rows
// whose visible reason matches - selecting a song filed under someone else's
// cause and then applying a bulk decision to it is the failure that matters
// here, not selecting too few.
//
// The error strings below are copied verbatim out of a real library's
// media_file_loudness table. Written by hand they would only prove the LIKE
// patterns match themselves.
var _ = Describe("loudness_reason filter", func() {
	var mr model.MediaFileRepository

	const (
		errPeak     = "true peak 0.16 dBTP exceeds the -0.50 dBTP ceiling"
		errPeakNeg  = "true peak -0.08 dBTP exceeds the -0.50 dBTP ceiling"
		errFormat   = "output was not format-identical: [duration changed]"
		errLoudness = "landed at -12.92 LUFS, expected -12.60"
	)

	BeforeEach(func() {
		ctx := log.NewContext(context.TODO())
		ctx = request.WithUser(ctx, model.User{ID: "userid"})
		mr = NewMediaFileRepository(ctx, GetDBXBuilder())
	})

	AfterEach(func() {
		_, err := GetDBXBuilder().NewQuery("delete from media_file_loudness").Execute()
		Expect(err).ToNot(HaveOccurred())
	})

	seed := func(mediaFileID, status, action, errText string) {
		_, err := GetDBXBuilder().NewQuery(
			"insert into media_file_loudness (media_file_id, status, action, error, phase) " +
				"values ({:id}, {:status}, {:action}, {:err}, 1)").
			Bind(map[string]any{
				"id": mediaFileID, "status": status, "action": action, "err": errText,
			}).Execute()
		Expect(err).ToNot(HaveOccurred())
	}

	matched := func(reason string) []string {
		found, err := mr.GetAll(model.QueryOptions{
			Filters: squirrel.And{
				squirrel.Eq{"media_file.missing": false},
				LoudnessReasonFilter(reason),
			},
			Sort: "id",
		})
		Expect(err).ToNot(HaveOccurred())
		ids := make([]string, 0, len(found))
		for _, mf := range found {
			ids = append(ids, mf.ID)
		}
		return ids
	}

	Context("with one song of each kind", func() {
		BeforeEach(func() {
			seed("1001", model.LoudnessStatusAnalyzed, model.LoudnessActionRefused, errPeak)
			seed("1002", model.LoudnessStatusAnalyzed, model.LoudnessActionRefused, errFormat)
			seed("1003", model.LoudnessStatusAnalyzed, model.LoudnessActionRefused, errLoudness)
			seed("1004", model.LoudnessStatusProcessed, model.LoudnessActionLimited, "")
			seed("1005", model.LoudnessStatusAnalyzed, model.LoudnessActionSkipped, "")
		})

		It("separates the three refusals by their cause", func() {
			Expect(matched("peak_over")).To(Equal([]string{"1001"}))
			Expect(matched("format_changed")).To(Equal([]string{"1002"}))
			Expect(matched("missed_target")).To(Equal([]string{"1003"}))
		})

		It("finds the songs whose peaks were trimmed", func() {
			Expect(matched("peaks_trimmed")).To(Equal([]string{"1004"}))
		})

		It("leaves the rest as decisions for a person", func() {
			// Everything not refused and not automatically trimmed - including
			// the songs with no audit row at all, which the LEFT join gives a
			// NULL action. Those belong here rather than nowhere.
			ids := matched("needs_trade")
			Expect(ids).To(ContainElement("1005"))
			Expect(ids).ToNot(ContainElement("1001"))
			Expect(ids).ToNot(ContainElement("1004"))
		})

		It("puts every song in exactly one category", func() {
			counted := map[string]int{}
			for _, reason := range []string{
				"peak_over", "format_changed", "missed_target", "peaks_trimmed", "needs_trade",
			} {
				for _, id := range matched(reason) {
					counted[id]++
				}
			}
			for id, n := range counted {
				Expect(n).To(Equal(1), "song %s matched %d categories, expected 1", id, n)
			}
			for _, id := range []string{"1001", "1002", "1003", "1004", "1005"} {
				Expect(counted).To(HaveKey(id), "song %s matched no category", id)
			}
		})

		It("accepts several causes at once, which is how the UI sends them", func() {
			found, err := mr.GetAll(model.QueryOptions{
				Filters: squirrel.And{
					squirrel.Eq{"media_file.missing": false},
					loudnessReasonFilter("", []string{"peak_over", "format_changed"}),
				},
				Sort: "id",
			})
			Expect(err).ToNot(HaveOccurred())
			ids := make([]string, 0, len(found))
			for _, mf := range found {
				ids = append(ids, mf.ID)
			}
			Expect(ids).To(Equal([]string{"1001", "1002"}))
		})

		It("ignores a cause it does not know", func() {
			Expect(loudnessReasonFilter("", "nonsense")).To(BeNil())
		})
	})

	It("reads a peak refusal whose peak is negative", func() {
		// -0.08 is over a -0.50 ceiling. Most of the real refusals look like
		// this, and a pattern anchored on a digit would miss every one.
		seed("1001", model.LoudnessStatusAnalyzed, model.LoudnessActionRefused, errPeakNeg)
		Expect(matched("peak_over")).To(Equal([]string{"1001"}))
	})

	It("does not file a refused song under a handled cause", func() {
		// A song can be refused after having been processed earlier. The page
		// shows the refusal, so the filter has to agree - otherwise a bulk
		// action aimed at trimmed songs would reach a song that was refused.
		seed("1001", model.LoudnessStatusProcessed, model.LoudnessActionRefused, errPeak)
		Expect(matched("peaks_trimmed")).To(BeEmpty())
		Expect(matched("needs_trade")).ToNot(ContainElement("1001"))
		Expect(matched("peak_over")).To(Equal([]string{"1001"}))
	})
})
