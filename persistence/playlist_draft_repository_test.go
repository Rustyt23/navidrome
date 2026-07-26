package persistence

import (
	"errors"

	"github.com/navidrome/navidrome/db"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/id"
	"github.com/navidrome/navidrome/model/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PlaylistDraftRepository", func() {
	var (
		drafts    model.PlaylistDraftRepository
		playlists model.PlaylistRepository
		ds        model.DataStore
		playlist  model.Playlist
		songIDs   []string
	)

	// liveTrackIDs reads the live playlist straight from the database, so
	// assertions about "did the playlist actually change" cannot be satisfied
	// by in-memory state.
	liveTrackIDs := func() []string {
		live, err := playlists.GetWithTracks(playlist.ID, false, true)
		Expect(err).ToNot(HaveOccurred())
		ids := make([]string, 0, len(live.Tracks))
		for _, track := range live.MediaFiles() {
			ids = append(ids, track.ID)
		}
		return ids
	}

	newDraft := func(proposed []string) *model.PlaylistDraft {
		draft := &model.PlaylistDraft{
			PlaylistID:       playlist.ID,
			SourceVersion:    model.PlaylistContentVersion(liveTrackIDs()),
			CreatedBy:        "userid",
			Name:             "AI cleanup",
			ProposedTrackIDs: proposed,
		}
		Expect(drafts.Create(draft)).To(Succeed())
		return draft
	}

	approve := func(draftID string) {
		Expect(drafts.UpdateStatus(draftID, model.DraftStatusReadyForReview, "userid")).To(Succeed())
		Expect(drafts.UpdateStatus(draftID, model.DraftStatusApproved, "reviewer")).To(Succeed())
	}

	BeforeEach(func() {
		ctx := log.NewContext(GinkgoT().Context())
		ctx = request.WithUser(ctx, model.User{ID: "userid", UserName: "userid", IsAdmin: true})
		ds = New(db.Db())
		playlists = ds.Playlist(ctx)
		drafts = ds.PlaylistDraft(ctx)

		mediaFiles := ds.MediaFile(ctx)
		songIDs = nil
		for _, title := range []string{"Draft Song A", "Draft Song B", "Draft Song C"} {
			mf := model.MediaFile{ID: id.NewRandom(), LibraryID: 1, Title: title, Path: title + ".mp3"}
			Expect(mediaFiles.Put(&mf)).To(Succeed())
			DeferCleanup(func() { _ = mediaFiles.Delete(mf.ID) })
			songIDs = append(songIDs, mf.ID)
		}

		playlist = model.Playlist{Name: "Draft Target", OwnerID: "userid"}
		playlist.AddMediaFilesByID(songIDs[:2])
		Expect(playlists.Put(&playlist)).To(Succeed())
		DeferCleanup(func() { _ = playlists.Delete(playlist.ID) })
	})

	Describe("Create and Get", func() {
		It("round-trips the proposed order and its change operations", func() {
			draft := newDraft([]string{songIDs[1], songIDs[0]})
			Expect(drafts.SetTracks(draft.ID, []string{songIDs[1], songIDs[0]}, []model.PlaylistDraftChange{
				{Kind: model.DraftChangeReorder, MediaFileID: songIDs[1], FromPosition: 1, ToPosition: 0,
					Reason: "opens stronger", Source: "ai", Confidence: 80},
			})).To(Succeed())

			stored, err := drafts.Get(draft.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(stored.ProposedTrackIDs).To(Equal([]string{songIDs[1], songIDs[0]}))
			Expect(stored.Status).To(Equal(model.DraftStatusDraft))
			Expect(stored.Changes).To(HaveLen(1))
			Expect(stored.Changes[0].Reason).To(Equal("opens stronger"))
			Expect(stored.Changes[0].Confidence).To(Equal(80))
		})

		It("lists drafts for a playlist", func() {
			newDraft([]string{songIDs[0]})
			newDraft([]string{songIDs[1]})
			list, err := drafts.GetAll(playlist.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(list).To(HaveLen(2))
		})
	})

	Describe("Editing", func() {
		It("refuses track edits once the draft has left the draft state", func() {
			draft := newDraft([]string{songIDs[0]})
			Expect(drafts.UpdateStatus(draft.ID, model.DraftStatusReadyForReview, "userid")).To(Succeed())

			err := drafts.SetTracks(draft.ID, []string{songIDs[2]}, nil)
			Expect(errors.Is(err, model.ErrDraftNotEditable)).To(BeTrue())
		})
	})

	Describe("Recommendation decisions", func() {
		It("scopes Ignore to a draft and Block to the whole playlist", func() {
			draft := newDraft(songIDs[:2])
			before := liveTrackIDs()
			Expect(drafts.SaveRecommendationDecision(&model.PlaylistRecommendationDecision{
				PlaylistID: playlist.ID, DraftID: draft.ID, SongID: songIDs[2],
				RecommendationType: "playlist_expansion",
				Decision:           model.RecommendationDecisionIgnored,
				CreatedBy:          "reviewer",
			})).To(Succeed())
			Expect(drafts.SaveRecommendationDecision(&model.PlaylistRecommendationDecision{
				PlaylistID: playlist.ID, SongID: songIDs[1],
				RecommendationType: "playlist_replacements",
				Decision:           model.RecommendationDecisionBlocked,
				CreatedBy:          "reviewer",
			})).To(Succeed())

			current, err := drafts.GetRecommendationDecisions(playlist.ID, draft.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(current).To(HaveLen(2))

			anotherDraft, err := drafts.GetRecommendationDecisions(playlist.ID, "another-draft")
			Expect(err).ToNot(HaveOccurred())
			Expect(anotherDraft).To(HaveLen(1))
			Expect(anotherDraft[0].Decision).To(Equal(model.RecommendationDecisionBlocked))
			Expect(anotherDraft[0].DraftID).To(BeEmpty())
			Expect(liveTrackIDs()).To(Equal(before))
		})
	})

	Describe("Status transitions", func() {
		It("rejects an illegal transition", func() {
			draft := newDraft([]string{songIDs[0]})
			err := drafts.UpdateStatus(draft.ID, model.DraftStatusPublished, "userid")
			Expect(errors.Is(err, model.ErrDraftInvalidStatus)).To(BeTrue())
		})

		It("clears a prior approval when the draft is reopened", func() {
			draft := newDraft([]string{songIDs[0]})
			approve(draft.ID)
			Expect(drafts.UpdateStatus(draft.ID, model.DraftStatusDraft, "userid")).To(Succeed())

			stored, err := drafts.Get(draft.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(stored.Status).To(Equal(model.DraftStatusDraft))
			Expect(stored.ReviewedBy).To(BeEmpty())
			Expect(stored.ReviewedAt).To(BeNil())
		})
	})

	Describe("Publish", func() {
		It("atomically creates an immutable version snapshot and publication audit", func() {
			proposed := []string{songIDs[2], songIDs[1]}
			draft := newDraft(proposed)
			Expect(drafts.SetTracks(draft.ID, proposed, []model.PlaylistDraftChange{{
				Kind: model.DraftChangeReplace, MediaFileID: songIDs[2], ReplacedID: songIDs[0],
				Reason: "cleaner fit", Source: "ai", Confidence: 92,
				AIModelVersion: "embedding-model-v2", AIIndexVersion: "3", RulesetVersion: "playlist-recommendation-v1",
			}})).To(Succeed())
			approve(draft.ID)

			Expect(drafts.Publish(draft.ID, "publisher")).To(Succeed())

			versions, err := drafts.GetVersions(playlist.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(versions).To(HaveLen(1))
			version := versions[0]
			Expect(version.Version).To(Equal(1))
			Expect(version.DraftID).To(Equal(draft.ID))
			Expect(version.TrackIDs).To(Equal(proposed))
			Expect(version.PreviousTrackIDs).To(Equal(songIDs[:2]))
			Expect(version.PublishedBy).To(Equal("publisher"))
			Expect(version.ApprovedBy).To(Equal("reviewer"))
			Expect(version.ApprovedAt).ToNot(BeNil())
			Expect(version.ChangeSummary.Replaced).To(Equal(1))
			Expect(version.ChangeSummary.AISelected).To(Equal(1))
			Expect(version.AIModelVersion).To(Equal("embedding-model-v2"))
			Expect(version.AIIndexVersion).To(Equal("3"))
			Expect(version.RulesetVersion).To(Equal("playlist-recommendation-v1"))

			stored, getErr := drafts.GetVersion(version.ID)
			Expect(getErr).ToNot(HaveOccurred())
			Expect(stored.TrackIDs).To(Equal(proposed))
			Expect(stored.Changes).To(HaveLen(1))

			events, auditErr := drafts.GetAuditEvents(playlist.ID)
			Expect(auditErr).ToNot(HaveOccurred())
			types := make([]model.PlaylistAuditEventType, 0, len(events))
			for _, event := range events {
				types = append(types, event.EventType)
			}
			Expect(types).To(ContainElements(
				model.AuditDraftCreated,
				model.AuditRecommendationAccepted,
				model.AuditReviewRequested,
				model.AuditApproved,
				model.AuditPublished,
			))
		})

		It("writes the proposed order onto the live playlist", func() {
			proposed := []string{songIDs[2], songIDs[0], songIDs[1]}
			draft := newDraft(proposed)
			approve(draft.ID)

			Expect(drafts.Publish(draft.ID, "publisher")).To(Succeed())

			Expect(liveTrackIDs()).To(Equal(proposed))
			stored, err := drafts.Get(draft.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(stored.Status).To(Equal(model.DraftStatusPublished))
			Expect(stored.PublishedBy).To(Equal("publisher"))
			Expect(stored.PublishedAt).ToNot(BeNil())
		})

		It("keeps duplicate tracks that the proposal asks for", func() {
			// The track repository's Add() de-duplicates, so publishing has to
			// go through a path that preserves a deliberate repeat.
			proposed := []string{songIDs[0], songIDs[0], songIDs[1]}
			draft := newDraft(proposed)
			approve(draft.ID)

			Expect(drafts.Publish(draft.ID, "publisher")).To(Succeed())
			Expect(liveTrackIDs()).To(Equal(proposed))
		})

		It("can empty a playlist", func() {
			draft := newDraft([]string{})
			approve(draft.ID)

			Expect(drafts.Publish(draft.ID, "publisher")).To(Succeed())
			Expect(liveTrackIDs()).To(BeEmpty())
		})

		It("refuses to publish a draft that was never approved", func() {
			draft := newDraft([]string{songIDs[2]})
			err := drafts.Publish(draft.ID, "publisher")
			Expect(errors.Is(err, model.ErrDraftNotApproved)).To(BeTrue())
			Expect(liveTrackIDs()).To(Equal(songIDs[:2]))

			events, auditErr := drafts.GetAuditEvents(playlist.ID)
			Expect(auditErr).ToNot(HaveOccurred())
			foundFailure := false
			for _, event := range events {
				if event.EventType == model.AuditPublicationFailed && event.DraftID == draft.ID {
					foundFailure = true
				}
			}
			Expect(foundFailure).To(BeTrue())
		})

		It("leaves the live playlist untouched when the draft is stale", func() {
			draft := newDraft([]string{songIDs[2]})
			approve(draft.ID)

			// Somebody edits the playlist after the draft was approved.
			changed := playlist
			changed.Tracks = nil
			changed.AddMediaFilesByID([]string{songIDs[0], songIDs[1], songIDs[2]})
			Expect(playlists.Put(&changed)).To(Succeed())
			before := liveTrackIDs()

			err := drafts.Publish(draft.ID, "publisher")
			Expect(errors.Is(err, model.ErrDraftStale)).To(BeTrue())
			Expect(liveTrackIDs()).To(Equal(before))

			stored, getErr := drafts.Get(draft.ID)
			Expect(getErr).ToNot(HaveOccurred())
			Expect(stored.Status).To(Equal(model.DraftStatusConflicted))
			Expect(stored.ConflictDetail).ToNot(BeEmpty())
			Expect(stored.PublishedAt).To(BeNil())
		})
	})
})
