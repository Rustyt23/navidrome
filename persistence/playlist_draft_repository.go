package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/id"
	"github.com/pocketbase/dbx"
)

type playlistDraftRepository struct {
	sqlRepository
	ds model.DataStore
}

// NewPlaylistDraftRepository stores proposed playlist changes. Only Publish
// touches a live playlist; everything else is confined to the draft tables.
func NewPlaylistDraftRepository(ctx context.Context, db dbx.Builder, ds model.DataStore) model.PlaylistDraftRepository {
	r := &playlistDraftRepository{ds: ds}
	r.ctx = ctx
	r.db = db
	r.tableName = "playlist_draft"
	return r
}

// dbPlaylistDraft carries the proposed order as the JSON text actually stored.
// The order is the draft's payload and is always read and written whole, so a
// column beats a child table here.
type dbPlaylistDraft struct {
	model.PlaylistDraft `structs:",flatten"`
	ProposedTrackIds    string `structs:"proposed_track_ids" json:"-"`
}

func (d *dbPlaylistDraft) PostScan() error {
	if d.ProposedTrackIds == "" {
		d.PlaylistDraft.ProposedTrackIDs = []string{}
		return nil
	}
	if err := json.Unmarshal([]byte(d.ProposedTrackIds), &d.PlaylistDraft.ProposedTrackIDs); err != nil {
		return fmt.Errorf("parsing playlist draft tracks: %w", err)
	}
	return nil
}

func (d *dbPlaylistDraft) PostMapArgs(args map[string]any) error {
	encoded, err := json.Marshal(d.PlaylistDraft.ProposedTrackIDs)
	if err != nil {
		return fmt.Errorf("encoding playlist draft tracks: %w", err)
	}
	args["proposed_track_ids"] = string(encoded)
	return nil
}

type dbPlaylistDrafts []dbPlaylistDraft

func (d dbPlaylistDrafts) toModels() model.PlaylistDrafts {
	drafts := make(model.PlaylistDrafts, len(d))
	for i := range d {
		drafts[i] = d[i].PlaylistDraft
	}
	return drafts
}

type dbPlaylistVersion struct {
	model.PlaylistVersion `structs:",flatten"`
	TrackIds              string `structs:"track_ids" json:"-"`
	PreviousTrackIds      string `structs:"previous_track_ids" json:"-"`
	Changes               string `structs:"changes" json:"-"`
	ChangeSummary         string `structs:"change_summary" json:"-"`
	AiModelVersion        string `structs:"ai_model_version" json:"-"`
	AiIndexVersion        string `structs:"ai_index_version" json:"-"`
}

func (v *dbPlaylistVersion) PostScan() error {
	v.PlaylistVersion.AIModelVersion = v.AiModelVersion
	v.PlaylistVersion.AIIndexVersion = v.AiIndexVersion
	if err := decodeJSONColumn(v.TrackIds, &v.TrackIDs, "playlist version tracks"); err != nil {
		return err
	}
	if err := decodeJSONColumn(v.PreviousTrackIds, &v.PreviousTrackIDs, "playlist version previous tracks"); err != nil {
		return err
	}
	if err := decodeJSONColumn(v.Changes, &v.PlaylistVersion.Changes, "playlist version changes"); err != nil {
		return err
	}
	return decodeJSONColumn(v.ChangeSummary, &v.PlaylistVersion.ChangeSummary, "playlist version summary")
}

type dbPlaylistVersions []dbPlaylistVersion

func (v dbPlaylistVersions) toModels() model.PlaylistVersions {
	versions := make(model.PlaylistVersions, len(v))
	for i := range v {
		versions[i] = v[i].PlaylistVersion
	}
	return versions
}

type dbPlaylistDraftChange struct {
	model.PlaylistDraftChange `structs:",flatten"`
	AiModelVersion            string `structs:"ai_model_version"`
	AiIndexVersion            string `structs:"ai_index_version"`
}

func (c *dbPlaylistDraftChange) PostScan() error {
	c.PlaylistDraftChange.AIModelVersion = c.AiModelVersion
	c.PlaylistDraftChange.AIIndexVersion = c.AiIndexVersion
	return nil
}

type dbPlaylistDraftChanges []dbPlaylistDraftChange

func decodeJSONColumn(value string, target any, label string) error {
	if strings.TrimSpace(value) == "" {
		value = "[]"
	}
	if err := json.Unmarshal([]byte(value), target); err != nil {
		return fmt.Errorf("parsing %s: %w", label, err)
	}
	return nil
}

func (r *playlistDraftRepository) Get(draftID string) (*model.PlaylistDraft, error) {
	var res dbPlaylistDraft
	err := r.queryOne(r.newSelect().Columns("*").Where(Eq{"id": draftID}), &res)
	if err != nil {
		return nil, err
	}
	changes, err := r.changesFor(draftID)
	if err != nil {
		return nil, err
	}
	res.PlaylistDraft.Changes = changes
	return &res.PlaylistDraft, nil
}

func (r *playlistDraftRepository) GetAll(playlistID string) (model.PlaylistDrafts, error) {
	sel := r.newSelect().Columns("*").OrderBy("created_at desc")
	if playlistID != "" {
		sel = sel.Where(Eq{"playlist_id": playlistID})
	}
	var res dbPlaylistDrafts
	if err := r.queryAll(sel, &res); err != nil {
		return nil, err
	}
	return res.toModels(), nil
}

func (r *playlistDraftRepository) changesFor(draftID string) ([]model.PlaylistDraftChange, error) {
	sel := Select("*").From("playlist_draft_change").Where(Eq{"draft_id": draftID}).OrderBy("seq")
	var rows dbPlaylistDraftChanges
	// queryAll (not queryAllSlice, which reads a single column into scalars).
	if err := r.queryAll(sel, &rows); err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}
	changes := make([]model.PlaylistDraftChange, len(rows))
	for i := range rows {
		changes[i] = rows[i].PlaylistDraftChange
	}
	return changes, nil
}

func (r *playlistDraftRepository) Create(draft *model.PlaylistDraft) error {
	if draft.ID == "" {
		draft.ID = id.NewRandom()
	}
	now := time.Now()
	draft.CreatedAt = now
	draft.UpdatedAt = now
	if draft.Status == "" {
		draft.Status = model.DraftStatusDraft
	}
	if draft.ProposedTrackIDs == nil {
		draft.ProposedTrackIDs = []string{}
	}
	if _, err := r.put(draft.ID, &dbPlaylistDraft{PlaylistDraft: *draft}); err != nil {
		return err
	}
	if err := r.appendAudit(&model.PlaylistAuditEvent{
		PlaylistID: draft.PlaylistID,
		DraftID:    draft.ID,
		EventType:  model.AuditDraftCreated,
		Actor:      draft.CreatedBy,
		Summary:    "Playlist draft created",
	}); err != nil {
		return err
	}
	if draft.RollbackVersionID != "" {
		details, _ := json.Marshal(map[string]any{
			"sourceVersionId": draft.RollbackVersionID,
			"sourceVersion":   draft.RollbackVersionNumber,
		})
		return r.appendAudit(&model.PlaylistAuditEvent{
			PlaylistID: draft.PlaylistID,
			DraftID:    draft.ID,
			EventType:  model.AuditRollbackCreated,
			Actor:      draft.CreatedBy,
			Summary:    fmt.Sprintf("Rollback draft created from version %d", draft.RollbackVersionNumber),
			Details:    string(details),
		})
	}
	return nil
}

// SetTracks replaces both the proposed order and the operations that produced
// it, so the stored order and its explanation can never drift apart.
func (r *playlistDraftRepository) SetTracks(draftID string, trackIDs []string, changes []model.PlaylistDraftChange) error {
	draft, err := r.Get(draftID)
	if err != nil {
		return err
	}
	if !draft.Status.IsEditable() {
		return fmt.Errorf("%w: status is %q", model.ErrDraftNotEditable, draft.Status)
	}
	if trackIDs == nil {
		trackIDs = []string{}
	}

	encoded, err := json.Marshal(trackIDs)
	if err != nil {
		return fmt.Errorf("encoding playlist draft tracks: %w", err)
	}
	upd := Update(r.tableName).
		Set("proposed_track_ids", string(encoded)).
		Set("updated_at", time.Now()).
		Where(Eq{"id": draftID})
	if _, err := r.executeSQL(upd); err != nil {
		return err
	}

	if _, err := r.executeSQL(Delete("playlist_draft_change").Where(Eq{"draft_id": draftID})); err != nil {
		return err
	}
	for i := range changes {
		change := changes[i]
		change.ID = id.NewRandom()
		change.DraftID = draftID
		change.Seq = i
		change.CreatedAt = time.Now()
		ins := Insert("playlist_draft_change").
			Columns("id", "draft_id", "seq", "kind", "media_file_id", "replaced_id",
				"from_position", "to_position", "reason", "source", "confidence",
				"ai_model_version", "ai_index_version", "ruleset_version", "created_at").
			Values(change.ID, change.DraftID, change.Seq, string(change.Kind), change.MediaFileID,
				change.ReplacedID, change.FromPosition, change.ToPosition, change.Reason,
				change.Source, change.Confidence, change.AIModelVersion, change.AIIndexVersion,
				change.RulesetVersion, change.CreatedAt)
		if _, err := r.executeSQL(ins); err != nil {
			return err
		}
	}

	actor := loggedUser(r.ctx).UserName
	if actor == "" {
		actor = draft.CreatedBy
	}
	newChanges := changes
	if len(draft.Changes) <= len(changes) {
		newChanges = changes[len(draft.Changes):]
	}
	for _, change := range newChanges {
		details, _ := json.Marshal(change)
		eventType := model.AuditManualDraftEdit
		summary := fmt.Sprintf("Manual draft %s operation", change.Kind)
		if strings.EqualFold(change.Source, "ai") {
			eventType = model.AuditRecommendationAccepted
			summary = fmt.Sprintf("AI recommendation accepted as %s", change.Kind)
		} else if strings.EqualFold(change.Source, "rollback") {
			continue
		}
		if err := r.appendAudit(&model.PlaylistAuditEvent{
			PlaylistID: draft.PlaylistID,
			DraftID:    draft.ID,
			EventType:  eventType,
			Actor:      actor,
			Summary:    summary,
			Details:    string(details),
		}); err != nil {
			return err
		}
	}
	return nil
}

// UpdateStatus moves a draft through review, rejecting illegal transitions and
// stamping whoever performed the move.
func (r *playlistDraftRepository) UpdateStatus(draftID string, next model.PlaylistDraftStatus, actor string) error {
	draft, err := r.Get(draftID)
	if err != nil {
		return err
	}
	if !next.Valid() {
		return fmt.Errorf("%w: %q is not a status", model.ErrDraftInvalidStatus, next)
	}
	if !draft.Status.CanTransitionTo(next) {
		return fmt.Errorf("%w: %s cannot become %s", model.ErrDraftInvalidStatus, draft.Status, next)
	}

	now := time.Now()
	upd := Update(r.tableName).Set("status", string(next)).Set("updated_at", now).Where(Eq{"id": draftID})
	switch next {
	case model.DraftStatusApproved:
		upd = upd.Set("reviewed_by", actor).Set("reviewed_at", now)
	case model.DraftStatusDraft:
		// Reopening clears a prior review and any conflict, so an approval can
		// never outlive the contents it was granted for.
		upd = upd.Set("reviewed_by", "").Set("reviewed_at", nil).Set("conflict_detail", "")
	}
	if _, err = r.executeSQL(upd); err != nil {
		return err
	}

	var eventType model.PlaylistAuditEventType
	var summary string
	switch next {
	case model.DraftStatusReadyForReview:
		eventType, summary = model.AuditReviewRequested, "Draft submitted for review"
	case model.DraftStatusApproved:
		eventType, summary = model.AuditApproved, "Draft approved"
	case model.DraftStatusDiscarded:
		eventType, summary = model.AuditDraftDiscarded, "Draft discarded"
	case model.DraftStatusDraft:
		if draft.Status == model.DraftStatusReadyForReview || draft.Status == model.DraftStatusApproved {
			eventType, summary = model.AuditRejected, "Draft rejected and reopened for editing"
		}
	}
	if eventType == "" {
		return nil
	}
	details, _ := json.Marshal(map[string]string{
		"fromStatus": string(draft.Status),
		"toStatus":   string(next),
	})
	return r.appendAudit(&model.PlaylistAuditEvent{
		PlaylistID: draft.PlaylistID,
		DraftID:    draft.ID,
		EventType:  eventType,
		Actor:      actor,
		Summary:    summary,
		Details:    string(details),
	})
}

func (r *playlistDraftRepository) MarkConflicted(draftID string, detail string) error {
	upd := Update(r.tableName).
		Set("status", string(model.DraftStatusConflicted)).
		Set("conflict_detail", detail).
		Set("updated_at", time.Now()).
		Where(Eq{"id": draftID})
	_, err := r.executeSQL(upd)
	return err
}

func (r *playlistDraftRepository) Delete(draftID string) error {
	return r.delete(Eq{"id": draftID})
}

func (r *playlistDraftRepository) GetRecommendationDecisions(playlistID, draftID string) (model.PlaylistRecommendationDecisions, error) {
	sel := Select("*").
		From("playlist_recommendation_decision").
		Where(Eq{"playlist_id": playlistID}).
		Where(Or{
			Eq{"decision": string(model.RecommendationDecisionBlocked)},
			And{
				Eq{"decision": string(model.RecommendationDecisionIgnored)},
				Eq{"draft_id": draftID},
			},
		}).
		OrderBy("created_at desc")
	var decisions model.PlaylistRecommendationDecisions
	if err := r.queryAll(sel, &decisions); err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}
	if decisions == nil {
		decisions = model.PlaylistRecommendationDecisions{}
	}
	return decisions, nil
}

func (r *playlistDraftRepository) SaveRecommendationDecision(decision *model.PlaylistRecommendationDecision) error {
	if decision == nil || !decision.Decision.Valid() {
		return fmt.Errorf("invalid recommendation decision")
	}
	if decision.PlaylistID == "" || decision.SongID == "" {
		return fmt.Errorf("playlistId and songId are required")
	}
	if decision.Decision == model.RecommendationDecisionIgnored && decision.DraftID == "" {
		return fmt.Errorf("draftId is required when ignoring a recommendation")
	}
	auditDraftID := decision.DraftID
	if decision.Decision == model.RecommendationDecisionBlocked {
		// Blocks apply to the configured playlist/profile, not merely the
		// current draft or recommendation type. Canonicalizing the unused
		// fields also keeps one durable block row per song and scope.
		decision.DraftID = ""
		decision.OriginalSongID = ""
		decision.RecommendationType = ""
	}

	scope := And{
		Eq{"playlist_id": decision.PlaylistID},
		Eq{"draft_id": decision.DraftID},
		Eq{"profile_id": decision.ProfileID},
		Eq{"song_id": decision.SongID},
		Eq{"original_song_id": decision.OriginalSongID},
		Eq{"recommendation_type": decision.RecommendationType},
		Eq{"decision": string(decision.Decision)},
	}
	if _, err := r.executeSQL(Delete("playlist_recommendation_decision").Where(scope)); err != nil {
		return err
	}
	if decision.ID == "" {
		decision.ID = id.NewRandom()
	}
	decision.CreatedAt = time.Now()
	ins := Insert("playlist_recommendation_decision").
		Columns("id", "playlist_id", "draft_id", "profile_id", "song_id",
			"original_song_id", "recommendation_type", "decision", "created_by", "created_at").
		Values(decision.ID, decision.PlaylistID, decision.DraftID, decision.ProfileID,
			decision.SongID, decision.OriginalSongID, decision.RecommendationType,
			string(decision.Decision), decision.CreatedBy, decision.CreatedAt)
	if _, err := r.executeSQL(ins); err != nil {
		return err
	}
	eventType := model.AuditRecommendationIgnored
	summary := "AI recommendation ignored"
	if decision.Decision == model.RecommendationDecisionBlocked {
		eventType = model.AuditRecommendationBlocked
		summary = "Song blocked from future recommendations"
	}
	details, _ := json.Marshal(map[string]string{
		"songId":             decision.SongID,
		"originalSongId":     decision.OriginalSongID,
		"recommendationType": decision.RecommendationType,
		"profileId":          decision.ProfileID,
	})
	return r.appendAudit(&model.PlaylistAuditEvent{
		PlaylistID: decision.PlaylistID,
		DraftID:    auditDraftID,
		EventType:  eventType,
		Actor:      decision.CreatedBy,
		Summary:    summary,
		Details:    string(details),
	})
}

func (r *playlistDraftRepository) GetVersions(playlistID string) (model.PlaylistVersions, error) {
	sel := Select("*").
		From("playlist_version").
		Where(Eq{"playlist_id": playlistID}).
		OrderBy("version desc")
	var rows dbPlaylistVersions
	if err := r.queryAll(sel, &rows); err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}
	if rows == nil {
		return model.PlaylistVersions{}, nil
	}
	return rows.toModels(), nil
}

func (r *playlistDraftRepository) GetVersion(versionID string) (*model.PlaylistVersion, error) {
	var row dbPlaylistVersion
	if err := r.queryOne(Select("*").From("playlist_version").Where(Eq{"id": versionID}), &row); err != nil {
		return nil, err
	}
	return &row.PlaylistVersion, nil
}

func (r *playlistDraftRepository) GetAuditEvents(playlistID string) (model.PlaylistAuditEvents, error) {
	sel := Select("*").
		From("playlist_audit_event").
		Where(Eq{"playlist_id": playlistID}).
		OrderBy("created_at desc", "id desc")
	var events model.PlaylistAuditEvents
	if err := r.queryAll(sel, &events); err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}
	if events == nil {
		events = model.PlaylistAuditEvents{}
	}
	return events, nil
}

func (r *playlistDraftRepository) appendAudit(event *model.PlaylistAuditEvent) error {
	if event.ID == "" {
		event.ID = id.NewRandom()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	ins := Insert("playlist_audit_event").
		Columns("id", "playlist_id", "draft_id", "version_id", "version_number",
			"event_type", "actor", "summary", "details", "created_at").
		Values(event.ID, event.PlaylistID, event.DraftID, event.VersionID,
			event.VersionNumber, string(event.EventType), event.Actor, event.Summary,
			event.Details, event.CreatedAt)
	_, err := r.executeSQL(ins)
	return err
}

func (r *playlistDraftRepository) createVersion(
	draft *model.PlaylistDraft,
	previousTrackIDs []string,
	actor string,
	publishedAt time.Time,
) (*model.PlaylistVersion, error) {
	var latest struct {
		Version int `db:"version"`
	}
	if err := r.queryOne(
		Select("coalesce(max(version), 0) as version").
			From("playlist_version").
			Where(Eq{"playlist_id": draft.PlaylistID}),
		&latest,
	); err != nil {
		return nil, err
	}

	modelVersion, indexVersion, rulesetVersion := playlistChangeProvenance(draft.Changes)
	version := &model.PlaylistVersion{
		ID:                  id.NewRandom(),
		PlaylistID:          draft.PlaylistID,
		Version:             latest.Version + 1,
		DraftID:             draft.ID,
		PublishedBy:         actor,
		PublishedAt:         publishedAt,
		ApprovedBy:          draft.ReviewedBy,
		ApprovedAt:          draft.ReviewedAt,
		AIModelVersion:      modelVersion,
		AIIndexVersion:      indexVersion,
		RulesetVersion:      rulesetVersion,
		RollbackFromVersion: draft.RollbackVersionNumber,
		TrackIDs:            slices.Clone(draft.ProposedTrackIDs),
		PreviousTrackIDs:    slices.Clone(previousTrackIDs),
		Changes:             slices.Clone(draft.Changes),
	}
	version.ChangeSummary = summarizeVersionChanges(version.Changes)

	trackJSON, err := json.Marshal(version.TrackIDs)
	if err != nil {
		return nil, err
	}
	previousJSON, err := json.Marshal(version.PreviousTrackIDs)
	if err != nil {
		return nil, err
	}
	changesJSON, err := json.Marshal(version.Changes)
	if err != nil {
		return nil, err
	}
	summaryJSON, err := json.Marshal(version.ChangeSummary)
	if err != nil {
		return nil, err
	}
	ins := Insert("playlist_version").
		Columns("id", "playlist_id", "version", "draft_id", "track_ids",
			"previous_track_ids", "changes", "change_summary", "published_by",
			"published_at", "approved_by", "approved_at", "ai_model_version",
			"ai_index_version", "ruleset_version", "rollback_from_version").
		Values(version.ID, version.PlaylistID, version.Version, version.DraftID,
			string(trackJSON), string(previousJSON), string(changesJSON), string(summaryJSON),
			version.PublishedBy, version.PublishedAt, version.ApprovedBy, version.ApprovedAt,
			version.AIModelVersion, version.AIIndexVersion, version.RulesetVersion,
			version.RollbackFromVersion)
	if _, err := r.executeSQL(ins); err != nil {
		return nil, err
	}
	return version, nil
}

func summarizeVersionChanges(changes []model.PlaylistDraftChange) model.PlaylistVersionChangeSummary {
	var summary model.PlaylistVersionChangeSummary
	for _, change := range changes {
		switch change.Kind {
		case model.DraftChangeAdd:
			summary.Added++
		case model.DraftChangeRemove:
			summary.Removed++
		case model.DraftChangeReplace:
			summary.Replaced++
		case model.DraftChangeReorder:
			summary.Reordered++
		}
		switch strings.ToLower(strings.TrimSpace(change.Source)) {
		case "ai":
			summary.AISelected++
		case "rollback":
			summary.RollbackChanges++
		default:
			summary.ManualSelected++
		}
	}
	summary.Text = fmt.Sprintf(
		"%d added, %d removed, %d replaced, %d reordered",
		summary.Added, summary.Removed, summary.Replaced, summary.Reordered,
	)
	return summary
}

func playlistChangeProvenance(changes []model.PlaylistDraftChange) (string, string, string) {
	models := map[string]struct{}{}
	indexes := map[string]struct{}{}
	rulesets := map[string]struct{}{}
	for _, change := range changes {
		if value := strings.TrimSpace(change.AIModelVersion); value != "" {
			models[value] = struct{}{}
		}
		if value := strings.TrimSpace(change.AIIndexVersion); value != "" {
			indexes[value] = struct{}{}
		}
		if value := strings.TrimSpace(change.RulesetVersion); value != "" {
			rulesets[value] = struct{}{}
		}
	}
	return sortedJoinedKeys(models), sortedJoinedKeys(indexes), sortedJoinedKeys(rulesets)
}

func sortedJoinedKeys(values map[string]struct{}) string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	slices.Sort(keys)
	return strings.Join(keys, ", ")
}

// Publish writes the proposed order onto the live playlist. Everything happens
// inside one immediate transaction, and the source version is re-checked from
// the live playlist *inside* that transaction rather than from the copy read
// earlier: a check performed before the transaction opens could be invalidated
// by a write landing in the gap. If anything fails the transaction rolls back
// and the live playlist is untouched.
func (r *playlistDraftRepository) Publish(draftID string, actor string) error {
	draft, err := r.Get(draftID)
	if err != nil {
		return err
	}
	if draft.Status != model.DraftStatusApproved {
		err = fmt.Errorf("%w: status is %q", model.ErrDraftNotApproved, draft.Status)
		r.recordPublishFailure(draft, actor, err)
		return err
	}

	var stale bool
	var staleDetail string
	err = r.ds.WithTxImmediate(func(tx model.DataStore) error {
		txDrafts := tx.PlaylistDraft(r.ctx).(*playlistDraftRepository)
		currentDraft, err := txDrafts.Get(draftID)
		if err != nil {
			return err
		}
		if currentDraft.Status != model.DraftStatusApproved {
			return fmt.Errorf("%w: status is %q", model.ErrDraftNotApproved, currentDraft.Status)
		}

		live, err := tx.Playlist(r.ctx).GetWithTracks(currentDraft.PlaylistID, false, true)
		if err != nil {
			return err
		}
		if live.IsSmartPlaylist() {
			return model.ErrDraftSmartPlaylist
		}

		liveIDs := playlistTrackIDs(live)
		liveVersion := model.PlaylistContentVersion(liveIDs)
		if liveVersion != currentDraft.SourceVersion {
			stale = true
			staleDetail = fmt.Sprintf(
				"The playlist now has %d tracks and changed since this draft was created (expected version %s, found %s).",
				len(liveIDs), currentDraft.SourceVersion, liveVersion)
			return model.ErrDraftStale
		}

		if err := replacePlaylistTracks(tx, r.ctx, live, currentDraft.ProposedTrackIDs); err != nil {
			return err
		}
		publishedAt := time.Now()
		version, err := txDrafts.createVersion(currentDraft, liveIDs, actor, publishedAt)
		if err != nil {
			return err
		}
		details, _ := json.Marshal(map[string]any{
			"versionId":     version.ID,
			"version":       version.Version,
			"changeSummary": version.ChangeSummary,
		})
		if err := txDrafts.appendAudit(&model.PlaylistAuditEvent{
			PlaylistID:    currentDraft.PlaylistID,
			DraftID:       currentDraft.ID,
			VersionID:     version.ID,
			VersionNumber: version.Version,
			EventType:     model.AuditPublished,
			Actor:         actor,
			Summary:       fmt.Sprintf("Playlist version %d published", version.Version),
			Details:       string(details),
			CreatedAt:     publishedAt,
		}); err != nil {
			return err
		}
		if currentDraft.RollbackVersionID != "" {
			if err := txDrafts.appendAudit(&model.PlaylistAuditEvent{
				PlaylistID:    currentDraft.PlaylistID,
				DraftID:       currentDraft.ID,
				VersionID:     version.ID,
				VersionNumber: version.Version,
				EventType:     model.AuditRollbackPublished,
				Actor:         actor,
				Summary:       fmt.Sprintf("Rollback to version %d published as version %d", currentDraft.RollbackVersionNumber, version.Version),
				Details:       string(details),
				CreatedAt:     publishedAt,
			}); err != nil {
				return err
			}
		}
		return txDrafts.markPublished(draftID, actor, publishedAt)
	})

	if stale {
		// Recorded outside the rolled-back transaction so the reviewer is told
		// why publishing was refused.
		if markErr := r.MarkConflicted(draftID, staleDetail); markErr != nil {
			log.Error(r.ctx, "Could not mark playlist draft conflicted", "draftId", draftID, markErr)
		}
		err = fmt.Errorf("%w: %s", model.ErrDraftStale, staleDetail)
	}
	if err != nil {
		r.recordPublishFailure(draft, actor, err)
	}
	return err
}

// markPublished stamps the draft as published. Called only from inside
// Publish's transaction so the playlist write and this stamp commit together.
func (r *playlistDraftRepository) markPublished(draftID, actor string, now time.Time) error {
	upd := Update(r.tableName).
		Set("status", string(model.DraftStatusPublished)).
		Set("published_by", actor).
		Set("published_at", now).
		Set("updated_at", now).
		Where(Eq{"id": draftID})
	_, err := r.executeSQL(upd)
	return err
}

func (r *playlistDraftRepository) recordPublishFailure(draft *model.PlaylistDraft, actor string, publishErr error) {
	if draft == nil || publishErr == nil {
		return
	}
	details, _ := json.Marshal(map[string]string{"error": publishErr.Error()})
	if err := r.appendAudit(&model.PlaylistAuditEvent{
		PlaylistID: draft.PlaylistID,
		DraftID:    draft.ID,
		EventType:  model.AuditPublicationFailed,
		Actor:      actor,
		Summary:    "Playlist publication failed",
		Details:    string(details),
	}); err != nil {
		log.Error(r.ctx, "Could not record playlist publication failure", "draftId", draft.ID, err)
	}
}

// replacePlaylistTracks makes the live playlist hold exactly the given media
// file IDs, in order. It goes through Put rather than the track repository's
// Add because Add silently skips IDs already present, which would drop a
// deliberate duplicate; Put replaces the whole list verbatim. An empty
// proposal has to clear the tracks directly, since Put treats "no tracks" as
// "tracks not specified" and would leave the playlist untouched.
func replacePlaylistTracks(tx model.DataStore, ctx context.Context, live *model.Playlist, trackIDs []string) error {
	if len(trackIDs) == 0 {
		return tx.Playlist(ctx).Tracks(live.ID, false).DeleteAll()
	}
	updated := *live
	updated.Tracks = nil
	updated.AddMediaFilesByID(trackIDs)
	return tx.Playlist(ctx).Put(&updated)
}

func playlistTrackIDs(pls *model.Playlist) []string {
	tracks := pls.MediaFiles()
	ids := make([]string, 0, len(tracks))
	for _, track := range tracks {
		ids = append(ids, track.ID)
	}
	return ids
}
