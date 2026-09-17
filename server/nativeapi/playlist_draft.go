package nativeapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
)

// The draft endpoints are the only supported way for an AI suggestion to reach
// a playlist. Nothing here writes to a live playlist except publish, and that
// requires an approved, non-stale draft.
func (n *Router) addPlaylistDraftRoutes(r chi.Router) {
	r.Route("/playlist-draft", func(r chi.Router) {
		r.Get("/", n.handleListPlaylistDrafts)
		r.Post("/", n.handleCreatePlaylistDraft)
		r.Get("/recommendation-decisions", n.handleListPlaylistRecommendationDecisions)
		r.Post("/recommendation-decisions", n.handleSavePlaylistRecommendationDecision)
		r.Get("/{id}", n.handleGetPlaylistDraft)
		r.Get("/{id}/diff", n.handlePlaylistDraftDiff)
		r.Put("/{id}/tracks", n.handleApplyPlaylistDraftOperations)
		r.Post("/{id}/status", n.handlePlaylistDraftStatus)
		r.Post("/{id}/publish", n.handlePublishPlaylistDraft)
		r.Delete("/{id}", n.handleDiscardPlaylistDraft)
	})
	r.Route("/playlist-history", func(r chi.Router) {
		r.Get("/", n.handleListPlaylistHistory)
		r.Get("/audit", n.handleListPlaylistAudit)
		r.Get("/{id}", n.handleGetPlaylistHistoryVersion)
		r.Post("/{id}/rollback", n.handleCreatePlaylistRollbackDraft)
	})
}

type createPlaylistDraftRequest struct {
	PlaylistID string `json:"playlistId"`
	Name       string `json:"name"`
	// Operations may be supplied at creation time so an AI suggestion becomes a
	// reviewable draft in a single call.
	Operations []playlistDraftOperation `json:"operations"`
}

// playlistDraftOperation is the wire form of one proposed change. Positions
// default to -1 (unspecified) rather than 0, which is a real index.
type playlistDraftOperation struct {
	Kind           string `json:"kind"`
	MediaFileID    string `json:"mediaFileId"`
	ReplacedID     string `json:"replacedId"`
	FromPosition   *int   `json:"fromPosition"`
	ToPosition     *int   `json:"toPosition"`
	Reason         string `json:"reason"`
	Source         string `json:"source"`
	Confidence     int    `json:"confidence"`
	AIModelVersion string `json:"aiModelVersion"`
	AIIndexVersion string `json:"aiIndexVersion"`
	RulesetVersion string `json:"rulesetVersion"`
}

func (o playlistDraftOperation) toModel() model.PlaylistDraftChange {
	change := model.PlaylistDraftChange{
		Kind:           model.PlaylistDraftChangeKind(strings.ToLower(strings.TrimSpace(o.Kind))),
		MediaFileID:    strings.TrimSpace(o.MediaFileID),
		ReplacedID:     strings.TrimSpace(o.ReplacedID),
		FromPosition:   -1,
		ToPosition:     -1,
		Reason:         o.Reason,
		Source:         o.Source,
		Confidence:     o.Confidence,
		AIModelVersion: strings.TrimSpace(o.AIModelVersion),
		AIIndexVersion: strings.TrimSpace(o.AIIndexVersion),
		RulesetVersion: strings.TrimSpace(o.RulesetVersion),
	}
	if o.FromPosition != nil {
		change.FromPosition = *o.FromPosition
	}
	if o.ToPosition != nil {
		change.ToPosition = *o.ToPosition
	}
	return change
}

type applyPlaylistDraftOperationsRequest struct {
	Operations []playlistDraftOperation `json:"operations"`
}

type playlistDraftStatusRequest struct {
	Status string `json:"status"`
}

type playlistRecommendationDecisionRequest struct {
	PlaylistID         string `json:"playlistId"`
	DraftID            string `json:"draftId,omitempty"`
	ProfileID          string `json:"profileId,omitempty"`
	SongID             string `json:"songId"`
	OriginalSongID     string `json:"originalSongId,omitempty"`
	RecommendationType string `json:"recommendationType"`
	Decision           string `json:"decision"`
}

func (n *Router) handleListPlaylistDrafts(w http.ResponseWriter, r *http.Request) {
	drafts, err := n.ds.PlaylistDraft(r.Context()).GetAll(strings.TrimSpace(r.URL.Query().Get("playlistId")))
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"drafts": drafts})
}

func (n *Router) handleGetPlaylistDraft(w http.ResponseWriter, r *http.Request) {
	draft, err := n.ds.PlaylistDraft(r.Context()).Get(chi.URLParam(r, "id"))
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, draft)
}

func (n *Router) handleListPlaylistRecommendationDecisions(w http.ResponseWriter, r *http.Request) {
	playlistID := strings.TrimSpace(r.URL.Query().Get("playlistId"))
	draftID := strings.TrimSpace(r.URL.Query().Get("draftId"))
	if playlistID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "playlistId is required"})
		return
	}
	decisions, err := n.ds.PlaylistDraft(r.Context()).GetRecommendationDecisions(playlistID, draftID)
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"decisions": decisions})
}

func (n *Router) handleSavePlaylistRecommendationDecision(w http.ResponseWriter, r *http.Request) {
	var payload playlistRecommendationDecisionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request payload"})
		return
	}
	payload.PlaylistID = strings.TrimSpace(payload.PlaylistID)
	payload.DraftID = strings.TrimSpace(payload.DraftID)
	payload.ProfileID = strings.TrimSpace(payload.ProfileID)
	payload.SongID = strings.TrimSpace(payload.SongID)
	payload.OriginalSongID = strings.TrimSpace(payload.OriginalSongID)
	payload.RecommendationType = strings.TrimSpace(payload.RecommendationType)
	kind := model.PlaylistRecommendationDecisionKind(strings.ToLower(strings.TrimSpace(payload.Decision)))
	if payload.PlaylistID == "" || payload.SongID == "" || !kind.Valid() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "playlistId, songId and a valid decision are required"})
		return
	}

	ctx := r.Context()
	if _, err := n.ds.Playlist(ctx).Get(payload.PlaylistID); err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	if kind == model.RecommendationDecisionIgnored {
		draft, err := n.ds.PlaylistDraft(ctx).Get(payload.DraftID)
		if err != nil {
			writePlaylistDraftError(w, r, err)
			return
		}
		if draft.PlaylistID != payload.PlaylistID {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "draft does not belong to the selected playlist"})
			return
		}
		if !draft.Status.IsEditable() {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": model.ErrDraftNotEditable.Error()})
			return
		}
	}

	user, _ := request.UserFrom(ctx)
	decision := &model.PlaylistRecommendationDecision{
		PlaylistID:         payload.PlaylistID,
		DraftID:            payload.DraftID,
		ProfileID:          payload.ProfileID,
		SongID:             payload.SongID,
		OriginalSongID:     payload.OriginalSongID,
		RecommendationType: payload.RecommendationType,
		Decision:           kind,
		CreatedBy:          user.UserName,
	}
	if err := n.ds.PlaylistDraft(ctx).SaveRecommendationDecision(decision); err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, decision)
}

// handleCreatePlaylistDraft snapshots the live playlist's content version and
// stores the proposed order. The live playlist is only read.
func (n *Router) handleCreatePlaylistDraft(w http.ResponseWriter, r *http.Request) {
	var payload createPlaylistDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request payload"})
		return
	}
	payload.PlaylistID = strings.TrimSpace(payload.PlaylistID)
	if payload.PlaylistID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "playlistId is required"})
		return
	}

	ctx := r.Context()
	live, err := n.ds.Playlist(ctx).GetWithTracks(payload.PlaylistID, false, true)
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	if live.IsSmartPlaylist() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": model.ErrDraftSmartPlaylist.Error()})
		return
	}

	liveIDs := mediaFileIDs(live)
	changes := toDraftChanges(payload.Operations)
	proposed, err := model.ApplyOperations(liveIDs, changes)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	user, _ := request.UserFrom(ctx)
	draft := &model.PlaylistDraft{
		PlaylistID:       live.ID,
		SourceVersion:    model.PlaylistContentVersion(liveIDs),
		Name:             strings.TrimSpace(payload.Name),
		CreatedBy:        user.UserName,
		ProposedTrackIDs: proposed,
	}
	drafts := n.ds.PlaylistDraft(ctx)
	if err := drafts.Create(draft); err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	if len(changes) > 0 {
		if err := drafts.SetTracks(draft.ID, proposed, changes); err != nil {
			writePlaylistDraftError(w, r, err)
			return
		}
	}

	stored, err := drafts.Get(draft.ID)
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, stored)
}

// handleApplyPlaylistDraftOperations folds more operations onto the draft's
// current proposal. Operations are applied to what the draft already proposes,
// not to the live playlist, so successive edits compose.
func (n *Router) handleApplyPlaylistDraftOperations(w http.ResponseWriter, r *http.Request) {
	var payload applyPlaylistDraftOperationsRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request payload"})
		return
	}

	ctx := r.Context()
	drafts := n.ds.PlaylistDraft(ctx)
	draft, err := drafts.Get(chi.URLParam(r, "id"))
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}

	newChanges := toDraftChanges(payload.Operations)
	proposed, err := model.ApplyOperations(draft.ProposedTrackIDs, newChanges)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	// The stored operation log is cumulative so the reviewer sees every step
	// that produced the proposal, not only the most recent batch.
	if err := drafts.SetTracks(draft.ID, proposed, append(draft.Changes, newChanges...)); err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}

	stored, err := drafts.Get(draft.ID)
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, stored)
}

// handlePlaylistDraftDiff returns the before/after review payload, including
// whether the live playlist has moved since the draft was taken.
func (n *Router) handlePlaylistDraftDiff(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	draft, err := n.ds.PlaylistDraft(ctx).Get(chi.URLParam(r, "id"))
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	live, err := n.ds.Playlist(ctx).GetWithTracks(draft.PlaylistID, false, true)
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}

	liveIDs := mediaFileIDs(live)
	songs := map[string]model.MediaFile{}
	for _, track := range live.MediaFiles() {
		songs[track.ID] = track
	}
	// Songs the proposal introduces are not in the live playlist, so their
	// titles have to be looked up separately or the diff shows bare IDs.
	mediaFiles := n.ds.MediaFile(ctx)
	for _, songID := range draft.ProposedTrackIDs {
		if _, ok := songs[songID]; ok {
			continue
		}
		if mf, mfErr := mediaFiles.Get(songID); mfErr == nil {
			songs[songID] = *mf
		}
	}

	reasons := map[string]model.PlaylistDraftChange{}
	for _, change := range draft.Changes {
		if change.MediaFileID != "" {
			reasons[change.MediaFileID] = change
		}
	}
	writeJSON(w, http.StatusOK, model.BuildDraftDiff(draft, liveIDs, songs, reasons))
}

func (n *Router) handlePlaylistDraftStatus(w http.ResponseWriter, r *http.Request) {
	var payload playlistDraftStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request payload"})
		return
	}
	next := model.PlaylistDraftStatus(strings.TrimSpace(payload.Status))
	// Publishing has its own endpoint because it writes to the live playlist;
	// it must not be reachable by simply asking for a status change.
	if next == model.DraftStatusPublished {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "use the publish endpoint to publish a draft"})
		return
	}

	ctx := r.Context()
	user, _ := request.UserFrom(ctx)
	drafts := n.ds.PlaylistDraft(ctx)
	if err := drafts.UpdateStatus(chi.URLParam(r, "id"), next, user.UserName); err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	stored, err := drafts.Get(chi.URLParam(r, "id"))
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, stored)
}

func (n *Router) handlePublishPlaylistDraft(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := request.UserFrom(ctx)
	drafts := n.ds.PlaylistDraft(ctx)
	draftID := chi.URLParam(r, "id")

	if err := drafts.Publish(draftID, user.UserName); err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	stored, err := drafts.Get(draftID)
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, stored)
}

// handleDiscardPlaylistDraft marks the draft discarded rather than deleting the
// row, so the audit trail of what was proposed and rejected survives.
func (n *Router) handleDiscardPlaylistDraft(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := request.UserFrom(ctx)
	if err := n.ds.PlaylistDraft(ctx).UpdateStatus(chi.URLParam(r, "id"), model.DraftStatusDiscarded, user.UserName); err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"discarded": true})
}

func toDraftChanges(operations []playlistDraftOperation) []model.PlaylistDraftChange {
	changes := make([]model.PlaylistDraftChange, 0, len(operations))
	for _, operation := range operations {
		changes = append(changes, operation.toModel())
	}
	return changes
}

func mediaFileIDs(pls *model.Playlist) []string {
	tracks := pls.MediaFiles()
	ids := make([]string, 0, len(tracks))
	for _, track := range tracks {
		ids = append(ids, track.ID)
	}
	return ids
}

// writePlaylistDraftError maps domain errors onto status codes. A stale draft
// is 409 so a client can distinguish "someone else changed the playlist" from
// a malformed request.
func writePlaylistDraftError(w http.ResponseWriter, _ *http.Request, err error) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "playlist draft not found"})
	case errors.Is(err, model.ErrDraftStale):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	case errors.Is(err, model.ErrDraftNotEditable),
		errors.Is(err, model.ErrDraftInvalidStatus),
		errors.Is(err, model.ErrDraftNotApproved),
		errors.Is(err, model.ErrDraftSmartPlaylist),
		errors.Is(err, model.ErrDraftUnknownOperation):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
