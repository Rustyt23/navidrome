package model_test

import (
	"github.com/navidrome/navidrome/model"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PlaylistDraft", func() {
	Describe("PlaylistContentVersion", func() {
		It("is stable for the same order and differs for a different one", func() {
			Expect(model.PlaylistContentVersion([]string{"a", "b"})).
				To(Equal(model.PlaylistContentVersion([]string{"a", "b"})))
			Expect(model.PlaylistContentVersion([]string{"a", "b"})).
				ToNot(Equal(model.PlaylistContentVersion([]string{"b", "a"})))
		})

		It("does not confuse differently split ids", func() {
			// Without a separator both would hash the byte string "abc".
			Expect(model.PlaylistContentVersion([]string{"ab", "c"})).
				ToNot(Equal(model.PlaylistContentVersion([]string{"a", "bc"})))
		})

		It("distinguishes an empty playlist from one holding an empty id", func() {
			Expect(model.PlaylistContentVersion(nil)).
				ToNot(Equal(model.PlaylistContentVersion([]string{""})))
		})
	})

	Describe("status transitions", func() {
		It("allows the review path and refuses to skip approval", func() {
			Expect(model.DraftStatusDraft.CanTransitionTo(model.DraftStatusReadyForReview)).To(BeTrue())
			Expect(model.DraftStatusReadyForReview.CanTransitionTo(model.DraftStatusApproved)).To(BeTrue())
			Expect(model.DraftStatusApproved.CanTransitionTo(model.DraftStatusPublished)).To(BeTrue())
			Expect(model.DraftStatusDraft.CanTransitionTo(model.DraftStatusPublished)).To(BeFalse())
			Expect(model.DraftStatusReadyForReview.CanTransitionTo(model.DraftStatusPublished)).To(BeFalse())
		})

		It("treats published and discarded as terminal", func() {
			Expect(model.DraftStatusPublished.IsTerminal()).To(BeTrue())
			Expect(model.DraftStatusDiscarded.IsTerminal()).To(BeTrue())
			Expect(model.DraftStatusPublished.CanTransitionTo(model.DraftStatusDraft)).To(BeFalse())
		})

		It("lets a conflicted draft be rebased back to draft", func() {
			Expect(model.DraftStatusConflicted.CanTransitionTo(model.DraftStatusDraft)).To(BeTrue())
			Expect(model.DraftStatusConflicted.CanTransitionTo(model.DraftStatusPublished)).To(BeFalse())
		})

		It("freezes contents once the draft is out for review", func() {
			Expect(model.DraftStatusDraft.IsEditable()).To(BeTrue())
			Expect(model.DraftStatusReadyForReview.IsEditable()).To(BeFalse())
			Expect(model.DraftStatusApproved.IsEditable()).To(BeFalse())
		})
	})

	Describe("ApplyOperations", func() {
		It("adds at a position and appends when the position is out of range", func() {
			out, err := model.ApplyOperations([]string{"a", "c"}, []model.PlaylistDraftChange{
				{Kind: model.DraftChangeAdd, MediaFileID: "b", ToPosition: 1},
				{Kind: model.DraftChangeAdd, MediaFileID: "z", ToPosition: 99},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(Equal([]string{"a", "b", "c", "z"}))
		})

		It("removes a specific copy when a position is given", func() {
			out, err := model.ApplyOperations([]string{"a", "b", "a"}, []model.PlaylistDraftChange{
				{Kind: model.DraftChangeRemove, MediaFileID: "a", FromPosition: 2},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(Equal([]string{"a", "b"}))
		})

		It("replaces in place, preserving position", func() {
			out, err := model.ApplyOperations([]string{"a", "b", "c"}, []model.PlaylistDraftChange{
				{Kind: model.DraftChangeReplace, ReplacedID: "b", MediaFileID: "x"},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(Equal([]string{"a", "x", "c"}))
		})

		It("reorders by moving rather than swapping", func() {
			out, err := model.ApplyOperations([]string{"a", "b", "c", "d"}, []model.PlaylistDraftChange{
				{Kind: model.DraftChangeReorder, FromPosition: 0, ToPosition: 2},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(Equal([]string{"b", "c", "a", "d"}))
		})

		It("composes operations in order", func() {
			out, err := model.ApplyOperations([]string{"a", "b"}, []model.PlaylistDraftChange{
				{Kind: model.DraftChangeAdd, MediaFileID: "c", ToPosition: 2},
				{Kind: model.DraftChangeRemove, MediaFileID: "a"},
				{Kind: model.DraftChangeReorder, FromPosition: 0, ToPosition: 1},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(Equal([]string{"c", "b"}))
		})

		It("never mutates the input order", func() {
			original := []string{"a", "b"}
			_, err := model.ApplyOperations(original, []model.PlaylistDraftChange{
				{Kind: model.DraftChangeRemove, MediaFileID: "a"},
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(original).To(Equal([]string{"a", "b"}))
		})

		It("rejects operations that cannot apply", func() {
			_, err := model.ApplyOperations([]string{"a"}, []model.PlaylistDraftChange{
				{Kind: model.DraftChangeRemove, MediaFileID: "missing"},
			})
			Expect(err).To(HaveOccurred())

			_, err = model.ApplyOperations([]string{"a"}, []model.PlaylistDraftChange{
				{Kind: model.DraftChangeReorder, FromPosition: 5, ToPosition: 0},
			})
			Expect(err).To(HaveOccurred())

			_, err = model.ApplyOperations([]string{"a"}, []model.PlaylistDraftChange{
				{Kind: "explode", MediaFileID: "a"},
			})
			Expect(err).To(MatchError(ContainSubstring("unknown playlist draft operation")))
		})
	})

	Describe("BuildDraftDiff", func() {
		songs := map[string]model.MediaFile{
			"a": {ID: "a", Title: "Song A", Artist: "Artist"},
			"b": {ID: "b", Title: "Song B", Artist: "Artist"},
			"c": {ID: "c", Title: "Song C", Artist: "Artist"},
		}

		It("classifies additions, removals and moves", func() {
			draft := &model.PlaylistDraft{
				ID: "d1", PlaylistID: "p1",
				SourceVersion:    model.PlaylistContentVersion([]string{"a", "b"}),
				ProposedTrackIDs: []string{"b", "c"},
			}
			diff := model.BuildDraftDiff(draft, []string{"a", "b"}, songs, nil)

			Expect(diff.Stale).To(BeFalse())
			Expect(diff.Added).To(HaveLen(1))
			Expect(diff.Added[0].MediaFileID).To(Equal("c"))
			Expect(diff.Added[0].Title).To(Equal("Song C"))
			Expect(diff.Removed).To(HaveLen(1))
			Expect(diff.Removed[0].MediaFileID).To(Equal("a"))
			Expect(diff.Moved).To(HaveLen(1))
			Expect(diff.Moved[0].MediaFileID).To(Equal("b"))
			Expect(diff.Moved[0].FromPosition).To(Equal(1))
			Expect(diff.Moved[0].Position).To(Equal(0))
		})

		It("counts a second copy of an existing song as an addition", func() {
			// Set-based comparison would call this unchanged and hide a real
			// change from the reviewer.
			draft := &model.PlaylistDraft{
				ID: "d1", PlaylistID: "p1",
				SourceVersion:    model.PlaylistContentVersion([]string{"a"}),
				ProposedTrackIDs: []string{"a", "a"},
			}
			diff := model.BuildDraftDiff(draft, []string{"a"}, songs, nil)
			Expect(diff.Added).To(HaveLen(1))
			Expect(diff.Removed).To(BeEmpty())
			Expect(diff.Unchanged).To(Equal(1))
		})

		It("reports an unchanged playlist as entirely unchanged", func() {
			draft := &model.PlaylistDraft{
				ID: "d1", PlaylistID: "p1",
				SourceVersion:    model.PlaylistContentVersion([]string{"a", "b"}),
				ProposedTrackIDs: []string{"a", "b"},
			}
			diff := model.BuildDraftDiff(draft, []string{"a", "b"}, songs, nil)
			Expect(diff.Added).To(BeEmpty())
			Expect(diff.Removed).To(BeEmpty())
			Expect(diff.Moved).To(BeEmpty())
			Expect(diff.Unchanged).To(Equal(2))
		})

		It("flags staleness when the live playlist moved", func() {
			draft := &model.PlaylistDraft{
				ID: "d1", PlaylistID: "p1",
				SourceVersion:    model.PlaylistContentVersion([]string{"a", "b"}),
				ProposedTrackIDs: []string{"a"},
			}
			diff := model.BuildDraftDiff(draft, []string{"a", "b", "c"}, songs, nil)
			Expect(diff.Stale).To(BeTrue())
			Expect(diff.LiveVersion).ToNot(Equal(diff.SourceVersion))
		})

		It("carries the AI reason onto the entry it explains", func() {
			draft := &model.PlaylistDraft{
				ID: "d1", PlaylistID: "p1",
				SourceVersion:    model.PlaylistContentVersion([]string{"a"}),
				ProposedTrackIDs: []string{"a", "c"},
			}
			reasons := map[string]model.PlaylistDraftChange{
				"c": {Kind: model.DraftChangeAdd, MediaFileID: "c", Reason: "clean alternative", Source: "ai"},
			}
			diff := model.BuildDraftDiff(draft, []string{"a"}, songs, reasons)
			Expect(diff.Added).To(HaveLen(1))
			Expect(diff.Added[0].Reason).To(Equal("clean alternative"))
			Expect(diff.Added[0].Source).To(Equal("ai"))
		})
	})

	Describe("BuildRollbackChanges", func() {
		It("describes removals, additions and ordering without changing either snapshot", func() {
			current := []string{"a", "b", "a"}
			target := []string{"b", "c", "a"}

			changes := model.BuildRollbackChanges(current, target, "Restore version 4")

			Expect(current).To(Equal([]string{"a", "b", "a"}))
			Expect(target).To(Equal([]string{"b", "c", "a"}))
			var removed, added, reordered bool
			for _, change := range changes {
				Expect(change.Reason).To(Equal("Restore version 4"))
				Expect(change.Source).To(Equal("rollback"))
				removed = removed || change.Kind == model.DraftChangeRemove && change.MediaFileID == "a"
				added = added || change.Kind == model.DraftChangeAdd && change.MediaFileID == "c"
				reordered = reordered || change.Kind == model.DraftChangeReorder && change.MediaFileID == "b"
			}
			Expect(removed).To(BeTrue())
			Expect(added).To(BeTrue())
			Expect(reordered).To(BeTrue())
		})
	})
})
