package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"time"
)

// A PlaylistDraft holds proposed changes to a live playlist. Nothing an AI
// suggests reaches a playlist directly: the suggestion becomes a draft, a
// person reviews the resulting diff, approves it, and only then is it
// published. Every draft records who proposed, reviewed and published it.
type PlaylistDraft struct {
	ID         string `structs:"id" json:"id"`
	PlaylistID string `structs:"playlist_id" json:"playlistId"`
	// SourceVersion is the live playlist's content version at the moment the
	// draft was taken. Publishing recomputes it; a change means the playlist
	// moved underneath the draft and the draft is stale.
	SourceVersion string              `structs:"source_version" json:"sourceVersion"`
	Status        PlaylistDraftStatus `structs:"status" json:"status"`
	Name          string              `structs:"name" json:"name"`
	CreatedBy     string              `structs:"created_by" json:"createdBy"`
	CreatedAt     time.Time           `structs:"created_at" json:"createdAt"`
	UpdatedAt     time.Time           `structs:"updated_at" json:"updatedAt"`
	// ProposedTrackIDs is the full media file ID list the playlist would have
	// once published, in order. Storing the outcome rather than replaying the
	// operations keeps publishing deterministic.
	ProposedTrackIDs []string `structs:"-" json:"proposedTrackIds"`

	ReviewedBy  string     `structs:"reviewed_by" json:"reviewedBy,omitempty"`
	ReviewedAt  *time.Time `structs:"reviewed_at" json:"reviewedAt,omitempty"`
	PublishedBy string     `structs:"published_by" json:"publishedBy,omitempty"`
	PublishedAt *time.Time `structs:"published_at" json:"publishedAt,omitempty"`
	// ConflictDetail explains, in a reviewer's terms, how the live playlist
	// diverged. Only set when Status is conflicted.
	ConflictDetail string `structs:"conflict_detail" json:"conflictDetail,omitempty"`

	// RollbackVersionID is set only when this draft was created from an
	// immutable historical version. Publishing still follows the normal review
	// path; these fields only preserve provenance for history and audit.
	RollbackVersionID     string `structs:"rollback_version_id" json:"rollbackVersionId,omitempty"`
	RollbackVersionNumber int    `structs:"rollback_version_number" json:"rollbackVersionNumber,omitempty"`

	Changes []PlaylistDraftChange `structs:"-" json:"changes,omitempty"`
}

type PlaylistDrafts []PlaylistDraft

// PlaylistDraftStatus is the review stage a draft is in.
type PlaylistDraftStatus string

const (
	// DraftStatusDraft is being edited; operations may still be applied.
	DraftStatusDraft PlaylistDraftStatus = "draft"
	// DraftStatusReadyForReview is frozen and awaiting a reviewer.
	DraftStatusReadyForReview PlaylistDraftStatus = "ready_for_review"
	// DraftStatusApproved passed review and may be published.
	DraftStatusApproved PlaylistDraftStatus = "approved"
	// DraftStatusPublished was written to the live playlist. Terminal.
	DraftStatusPublished PlaylistDraftStatus = "published"
	// DraftStatusDiscarded was abandoned without publishing. Terminal.
	DraftStatusDiscarded PlaylistDraftStatus = "discarded"
	// DraftStatusConflicted means the live playlist changed after the draft was
	// taken, so publishing it would silently revert someone else's edit.
	DraftStatusConflicted PlaylistDraftStatus = "conflicted"
)

var (
	ErrDraftNotEditable      = errors.New("playlist draft can no longer be edited")
	ErrDraftInvalidStatus    = errors.New("invalid playlist draft status transition")
	ErrDraftStale            = errors.New("playlist changed since this draft was created")
	ErrDraftNotApproved      = errors.New("playlist draft must be approved before publishing")
	ErrDraftSmartPlaylist    = errors.New("smart playlists are generated from rules and cannot be drafted")
	ErrDraftUnknownOperation = errors.New("unknown playlist draft operation")
)

// draftTransitions is the complete set of legal status moves. Publishing and
// discarding are terminal; conflicted is reachable from any live state because
// the live playlist can move at any time.
var draftTransitions = map[PlaylistDraftStatus][]PlaylistDraftStatus{
	DraftStatusDraft:          {DraftStatusReadyForReview, DraftStatusDiscarded, DraftStatusConflicted},
	DraftStatusReadyForReview: {DraftStatusDraft, DraftStatusApproved, DraftStatusDiscarded, DraftStatusConflicted},
	DraftStatusApproved:       {DraftStatusPublished, DraftStatusDraft, DraftStatusDiscarded, DraftStatusConflicted},
	// A conflicted draft is not dead: rebasing it onto the current playlist
	// returns it to draft so the reviewer can re-check the diff.
	DraftStatusConflicted: {DraftStatusDraft, DraftStatusDiscarded},
	DraftStatusPublished:  {},
	DraftStatusDiscarded:  {},
}

func (s PlaylistDraftStatus) Valid() bool {
	_, ok := draftTransitions[s]
	return ok
}

// IsTerminal reports whether the draft has reached a state it cannot leave.
func (s PlaylistDraftStatus) IsTerminal() bool {
	return len(draftTransitions[s]) == 0 && s.Valid()
}

// CanTransitionTo reports whether moving to next is legal.
func (s PlaylistDraftStatus) CanTransitionTo(next PlaylistDraftStatus) bool {
	return slices.Contains(draftTransitions[s], next)
}

// IsEditable reports whether track operations may still be applied. Once a
// draft is out for review its contents are frozen, so what a reviewer approves
// is exactly what gets published.
func (s PlaylistDraftStatus) IsEditable() bool {
	return s == DraftStatusDraft
}

// PlaylistDraftChangeKind is the type of a single proposed operation.
type PlaylistDraftChangeKind string

const (
	DraftChangeAdd     PlaylistDraftChangeKind = "add"
	DraftChangeRemove  PlaylistDraftChangeKind = "remove"
	DraftChangeReplace PlaylistDraftChangeKind = "replace"
	DraftChangeReorder PlaylistDraftChangeKind = "reorder"
)

func (k PlaylistDraftChangeKind) Valid() bool {
	switch k {
	case DraftChangeAdd, DraftChangeRemove, DraftChangeReplace, DraftChangeReorder:
		return true
	default:
		return false
	}
}

// PlaylistDraftChange records one proposed operation and why it was proposed.
// The reason and source travel with the change so a reviewer can judge an AI
// suggestion instead of being handed an unexplained track list.
type PlaylistDraftChange struct {
	ID      string                  `structs:"id" json:"id"`
	DraftID string                  `structs:"draft_id" json:"draftId"`
	Seq     int                     `structs:"seq" json:"seq"`
	Kind    PlaylistDraftChangeKind `structs:"kind" json:"kind"`
	// MediaFileID is the song being added, removed, or moved. For a replace it
	// is the incoming song and ReplacedID is the one being displaced.
	MediaFileID  string `structs:"media_file_id" json:"mediaFileId,omitempty"`
	ReplacedID   string `structs:"replaced_id" json:"replacedId,omitempty"`
	FromPosition int    `structs:"from_position" json:"fromPosition,omitempty"`
	ToPosition   int    `structs:"to_position" json:"toPosition,omitempty"`
	// Reason, Source and Confidence carry the AI recommendation detail.
	Reason     string `structs:"reason" json:"reason,omitempty"`
	Source     string `structs:"source" json:"source,omitempty"`
	Confidence int    `structs:"confidence" json:"confidence,omitempty"`
	// AI provenance is captured with each accepted recommendation so a
	// published version remains reproducible even if the active model or index
	// changes later.
	AIModelVersion string    `structs:"ai_model_version" json:"aiModelVersion,omitempty"`
	AIIndexVersion string    `structs:"ai_index_version" json:"aiIndexVersion,omitempty"`
	RulesetVersion string    `structs:"ruleset_version" json:"rulesetVersion,omitempty"`
	CreatedAt      time.Time `structs:"created_at" json:"createdAt"`
}

// PlaylistContentVersion is the version used for stale detection. It hashes the
// ordered track list only, so edits that do not change which songs are in the
// playlist or their order (a rename, a new comment) never make a draft stale,
// while any real track change always does. Deriving it from content rather than
// a timestamp also keeps it immune to clock skew.
func PlaylistContentVersion(trackIDs []string) string {
	digest := sha256.New()
	for _, id := range trackIDs {
		// The separator keeps ["ab","c"] from hashing the same as ["a","bc"].
		digest.Write([]byte(id))
		digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil))[:32]
}

// DraftDiffEntry is one line of the before/after comparison shown to a
// reviewer.
type DraftDiffEntry struct {
	Kind PlaylistDraftChangeKind `json:"kind"`
	// Position is where the entry sits in the proposed order, or where it was
	// in the live order for a removal.
	Position     int    `json:"position"`
	FromPosition int    `json:"fromPosition,omitempty"`
	MediaFileID  string `json:"mediaFileId"`
	Title        string `json:"title,omitempty"`
	Artist       string `json:"artist,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Source       string `json:"source,omitempty"`
}

// PlaylistDraftDiff is the complete before/after review payload.
type PlaylistDraftDiff struct {
	DraftID       string           `json:"draftId"`
	PlaylistID    string           `json:"playlistId"`
	Stale         bool             `json:"stale"`
	SourceVersion string           `json:"sourceVersion"`
	LiveVersion   string           `json:"liveVersion"`
	Before        []DraftDiffEntry `json:"before"`
	After         []DraftDiffEntry `json:"after"`
	Added         []DraftDiffEntry `json:"added"`
	Removed       []DraftDiffEntry `json:"removed"`
	Moved         []DraftDiffEntry `json:"moved"`
	Unchanged     int              `json:"unchanged"`
}

// PlaylistDraftRepository stores drafts server-side. Nothing here writes to a
// live playlist except Publish.
type PlaylistDraftRepository interface {
	Get(id string) (*PlaylistDraft, error)
	GetAll(playlistID string) (PlaylistDrafts, error)
	Create(draft *PlaylistDraft) error
	// SetTracks replaces the proposed order and the operations that produced
	// it. Only valid while the draft is editable.
	SetTracks(id string, trackIDs []string, changes []PlaylistDraftChange) error
	// UpdateStatus moves the draft through review, recording the actor. It
	// rejects illegal transitions.
	UpdateStatus(id string, next PlaylistDraftStatus, actor string) error
	MarkConflicted(id string, detail string) error
	Delete(id string) error
	// Publish writes the proposed order onto the live playlist atomically,
	// re-checking the source version inside the transaction. A stale draft
	// leaves the playlist untouched and returns ErrDraftStale.
	Publish(id string, actor string) error
	// Recommendation decisions are kept beside drafts because Ignore is scoped
	// to one working draft while Block is scoped to the playlist (or a future
	// client profile). They never mutate either the draft track list or the
	// live playlist.
	GetRecommendationDecisions(playlistID, draftID string) (PlaylistRecommendationDecisions, error)
	SaveRecommendationDecision(decision *PlaylistRecommendationDecision) error
	// Playlist versions and audit events are append-only. The repository
	// intentionally exposes no update or delete operation for either.
	GetVersions(playlistID string) (PlaylistVersions, error)
	GetVersion(id string) (*PlaylistVersion, error)
	GetAuditEvents(playlistID string) (PlaylistAuditEvents, error)
}

type PlaylistRecommendationDecisionKind string

const (
	RecommendationDecisionIgnored PlaylistRecommendationDecisionKind = "ignored"
	RecommendationDecisionBlocked PlaylistRecommendationDecisionKind = "blocked"
)

func (d PlaylistRecommendationDecisionKind) Valid() bool {
	return d == RecommendationDecisionIgnored || d == RecommendationDecisionBlocked
}

// PlaylistRecommendationDecision records a user's response to an AI result.
// An ignored result is hidden only in its current draft analysis. A blocked
// result is excluded from future recommendations for the playlist or profile.
type PlaylistRecommendationDecision struct {
	ID                 string                             `structs:"id" json:"id"`
	PlaylistID         string                             `structs:"playlist_id" json:"playlistId"`
	DraftID            string                             `structs:"draft_id" json:"draftId,omitempty"`
	ProfileID          string                             `structs:"profile_id" json:"profileId,omitempty"`
	SongID             string                             `structs:"song_id" json:"songId"`
	OriginalSongID     string                             `structs:"original_song_id" json:"originalSongId,omitempty"`
	RecommendationType string                             `structs:"recommendation_type" json:"recommendationType"`
	Decision           PlaylistRecommendationDecisionKind `structs:"decision" json:"decision"`
	CreatedBy          string                             `structs:"created_by" json:"createdBy"`
	CreatedAt          time.Time                          `structs:"created_at" json:"createdAt"`
}

type PlaylistRecommendationDecisions []PlaylistRecommendationDecision

// PlaylistVersion is an immutable snapshot created in the same transaction
// that publishes a draft. PreviousTrackIDs makes the before/after view
// self-contained even for version 1.
type PlaylistVersion struct {
	ID                  string     `structs:"id" json:"id"`
	PlaylistID          string     `structs:"playlist_id" json:"playlistId"`
	Version             int        `structs:"version" json:"version"`
	DraftID             string     `structs:"draft_id" json:"draftId"`
	PublishedBy         string     `structs:"published_by" json:"publishedBy"`
	PublishedAt         time.Time  `structs:"published_at" json:"publishedAt"`
	ApprovedBy          string     `structs:"approved_by" json:"approvedBy,omitempty"`
	ApprovedAt          *time.Time `structs:"approved_at" json:"approvedAt,omitempty"`
	AIModelVersion      string     `structs:"ai_model_version" json:"aiModelVersion,omitempty"`
	AIIndexVersion      string     `structs:"ai_index_version" json:"aiIndexVersion,omitempty"`
	RulesetVersion      string     `structs:"ruleset_version" json:"rulesetVersion,omitempty"`
	RollbackFromVersion int        `structs:"rollback_from_version" json:"rollbackFromVersion,omitempty"`

	TrackIDs         []string                     `structs:"-" json:"trackIds"`
	PreviousTrackIDs []string                     `structs:"-" json:"previousTrackIds"`
	Changes          []PlaylistDraftChange        `structs:"-" json:"changes"`
	ChangeSummary    PlaylistVersionChangeSummary `structs:"-" json:"changeSummary"`
}

type PlaylistVersions []PlaylistVersion

type PlaylistVersionChangeSummary struct {
	Added           int    `json:"added"`
	Removed         int    `json:"removed"`
	Replaced        int    `json:"replaced"`
	Reordered       int    `json:"reordered"`
	AISelected      int    `json:"aiSelected"`
	ManualSelected  int    `json:"manualSelected"`
	RollbackChanges int    `json:"rollbackChanges"`
	Text            string `json:"text"`
}

type PlaylistAuditEventType string

const (
	AuditDraftCreated           PlaylistAuditEventType = "draft_created"
	AuditRecommendationAccepted PlaylistAuditEventType = "recommendation_accepted"
	AuditRecommendationIgnored  PlaylistAuditEventType = "recommendation_ignored"
	AuditRecommendationBlocked  PlaylistAuditEventType = "recommendation_blocked"
	AuditManualDraftEdit        PlaylistAuditEventType = "manual_draft_edit"
	AuditReviewRequested        PlaylistAuditEventType = "review_requested"
	AuditApproved               PlaylistAuditEventType = "approved"
	AuditRejected               PlaylistAuditEventType = "rejected"
	AuditPublished              PlaylistAuditEventType = "published"
	AuditPublicationFailed      PlaylistAuditEventType = "publication_failed"
	AuditDraftDiscarded         PlaylistAuditEventType = "draft_discarded"
	AuditRollbackCreated        PlaylistAuditEventType = "rollback_created"
	AuditRollbackPublished      PlaylistAuditEventType = "rollback_published"
)

// PlaylistAuditEvent is append-only operational evidence. Details is JSON text
// so events can carry operation-specific context without schema churn.
type PlaylistAuditEvent struct {
	ID            string                 `structs:"id" json:"id"`
	PlaylistID    string                 `structs:"playlist_id" json:"playlistId"`
	DraftID       string                 `structs:"draft_id" json:"draftId,omitempty"`
	VersionID     string                 `structs:"version_id" json:"versionId,omitempty"`
	VersionNumber int                    `structs:"version_number" json:"versionNumber,omitempty"`
	EventType     PlaylistAuditEventType `structs:"event_type" json:"eventType"`
	Actor         string                 `structs:"actor" json:"actor"`
	Summary       string                 `structs:"summary" json:"summary"`
	Details       string                 `structs:"details" json:"details,omitempty"`
	CreatedAt     time.Time              `structs:"created_at" json:"createdAt"`
}

type PlaylistAuditEvents []PlaylistAuditEvent

// ApplyOperations folds a set of proposed operations onto a starting track
// order and returns the resulting order. It is deliberately pure so the same
// code can preview a diff and compute what will be published.
func ApplyOperations(current []string, ops []PlaylistDraftChange) ([]string, error) {
	next := slices.Clone(current)
	for _, op := range ops {
		var err error
		switch op.Kind {
		case DraftChangeAdd:
			next, err = draftApplyAdd(next, op)
		case DraftChangeRemove:
			next, err = draftApplyRemove(next, op)
		case DraftChangeReplace:
			next, err = draftApplyReplace(next, op)
		case DraftChangeReorder:
			next, err = draftApplyReorder(next, op)
		default:
			err = fmt.Errorf("%w: %q", ErrDraftUnknownOperation, op.Kind)
		}
		if err != nil {
			return nil, err
		}
	}
	return next, nil
}

func draftApplyAdd(order []string, op PlaylistDraftChange) ([]string, error) {
	if op.MediaFileID == "" {
		return nil, fmt.Errorf("add operation requires a mediaFileId")
	}
	// A position outside the list appends, so callers can add without first
	// knowing the length.
	at := op.ToPosition
	if at < 0 || at > len(order) {
		at = len(order)
	}
	return slices.Insert(order, at, op.MediaFileID), nil
}

func draftApplyRemove(order []string, op PlaylistDraftChange) ([]string, error) {
	if op.MediaFileID == "" {
		return nil, fmt.Errorf("remove operation requires a mediaFileId")
	}
	// Remove by position when given one, so a playlist holding the same song
	// twice can have one specific copy removed.
	if op.FromPosition >= 0 && op.FromPosition < len(order) && order[op.FromPosition] == op.MediaFileID {
		return slices.Delete(slices.Clone(order), op.FromPosition, op.FromPosition+1), nil
	}
	at := slices.Index(order, op.MediaFileID)
	if at < 0 {
		return nil, fmt.Errorf("cannot remove %q: not in the playlist", op.MediaFileID)
	}
	return slices.Delete(slices.Clone(order), at, at+1), nil
}

func draftApplyReplace(order []string, op PlaylistDraftChange) ([]string, error) {
	if op.MediaFileID == "" || op.ReplacedID == "" {
		return nil, fmt.Errorf("replace operation requires both mediaFileId and replacedId")
	}
	at := op.FromPosition
	if at < 0 || at >= len(order) || order[at] != op.ReplacedID {
		at = slices.Index(order, op.ReplacedID)
	}
	if at < 0 {
		return nil, fmt.Errorf("cannot replace %q: not in the playlist", op.ReplacedID)
	}
	next := slices.Clone(order)
	next[at] = op.MediaFileID
	return next, nil
}

func draftApplyReorder(order []string, op PlaylistDraftChange) ([]string, error) {
	from, to := op.FromPosition, op.ToPosition
	if from < 0 || from >= len(order) {
		return nil, fmt.Errorf("reorder from position %d is out of range", from)
	}
	if to < 0 || to >= len(order) {
		return nil, fmt.Errorf("reorder to position %d is out of range", to)
	}
	if from == to {
		return order, nil
	}
	next := slices.Clone(order)
	moved := next[from]
	next = slices.Delete(next, from, from+1)
	return slices.Insert(next, to, moved), nil
}

// BuildRollbackChanges creates an explanatory operation log for a rollback
// draft. The draft stores the exact target order separately, so these changes
// describe the count and ordering differences without being used as the source
// of truth for publication.
func BuildRollbackChanges(current, target []string, reason string) []PlaylistDraftChange {
	currentCounts := map[string]int{}
	targetCounts := map[string]int{}
	for _, songID := range current {
		currentCounts[songID]++
	}
	for _, songID := range target {
		targetCounts[songID]++
	}

	changes := make([]PlaylistDraftChange, 0)
	seen := map[string]int{}
	for position, songID := range current {
		seen[songID]++
		if seen[songID] > targetCounts[songID] {
			changes = append(changes, PlaylistDraftChange{
				Kind: DraftChangeRemove, MediaFileID: songID,
				FromPosition: position, ToPosition: -1,
				Reason: reason, Source: "rollback",
			})
		}
	}
	seen = map[string]int{}
	for position, songID := range target {
		seen[songID]++
		if seen[songID] > currentCounts[songID] {
			changes = append(changes, PlaylistDraftChange{
				Kind: DraftChangeAdd, MediaFileID: songID,
				FromPosition: -1, ToPosition: position,
				Reason: reason, Source: "rollback",
			})
			continue
		}
		if from := draftNthIndex(current, songID, seen[songID]); from != position {
			changes = append(changes, PlaylistDraftChange{
				Kind: DraftChangeReorder, MediaFileID: songID,
				FromPosition: from, ToPosition: position,
				Reason: reason, Source: "rollback",
			})
		}
	}
	return changes
}

// BuildDraftDiff compares the live order against the proposed order and
// produces the reviewer-facing before/after. Counting by occurrence rather
// than by set membership matters because a playlist may legitimately contain
// the same song more than once: adding a second copy of a track already
// present has to show up as an addition, not as unchanged.
func BuildDraftDiff(draft *PlaylistDraft, liveOrder []string, songs map[string]MediaFile, reasons map[string]PlaylistDraftChange) PlaylistDraftDiff {
	liveVersion := PlaylistContentVersion(liveOrder)
	diff := PlaylistDraftDiff{
		DraftID:       draft.ID,
		PlaylistID:    draft.PlaylistID,
		SourceVersion: draft.SourceVersion,
		LiveVersion:   liveVersion,
		Stale:         liveVersion != draft.SourceVersion,
		Before:        draftEntries(liveOrder, songs, nil),
		After:         draftEntries(draft.ProposedTrackIDs, songs, reasons),
		Added:         []DraftDiffEntry{},
		Removed:       []DraftDiffEntry{},
		Moved:         []DraftDiffEntry{},
	}

	liveCounts := map[string]int{}
	for _, id := range liveOrder {
		liveCounts[id]++
	}
	proposedCounts := map[string]int{}
	for _, id := range draft.ProposedTrackIDs {
		proposedCounts[id]++
	}

	seen := map[string]int{}
	for position, id := range draft.ProposedTrackIDs {
		seen[id]++
		if seen[id] > liveCounts[id] {
			diff.Added = append(diff.Added, draftEntry(DraftChangeAdd, position, -1, id, songs, reasons))
			continue
		}
		// Present in both: report it as moved only if this copy sits at a
		// different index than the corresponding live copy.
		if from := draftNthIndex(liveOrder, id, seen[id]); from != position {
			diff.Moved = append(diff.Moved, draftEntry(DraftChangeReorder, position, from, id, songs, reasons))
			continue
		}
		diff.Unchanged++
	}

	seen = map[string]int{}
	for position, id := range liveOrder {
		seen[id]++
		if seen[id] > proposedCounts[id] {
			diff.Removed = append(diff.Removed, draftEntry(DraftChangeRemove, position, position, id, songs, reasons))
		}
	}
	return diff
}

// draftNthIndex returns the index of the nth (1-based) occurrence of id.
func draftNthIndex(order []string, id string, nth int) int {
	count := 0
	for i, candidate := range order {
		if candidate != id {
			continue
		}
		count++
		if count == nth {
			return i
		}
	}
	return -1
}

func draftEntries(order []string, songs map[string]MediaFile, reasons map[string]PlaylistDraftChange) []DraftDiffEntry {
	entries := make([]DraftDiffEntry, 0, len(order))
	for position, id := range order {
		entries = append(entries, draftEntry("", position, -1, id, songs, reasons))
	}
	return entries
}

func draftEntry(kind PlaylistDraftChangeKind, position, from int, id string, songs map[string]MediaFile, reasons map[string]PlaylistDraftChange) DraftDiffEntry {
	entry := DraftDiffEntry{Kind: kind, Position: position, FromPosition: from, MediaFileID: id}
	if song, ok := songs[id]; ok {
		entry.Title = song.Title
		entry.Artist = song.Artist
	}
	if change, ok := reasons[id]; ok {
		entry.Reason = change.Reason
		entry.Source = change.Source
	}
	return entry
}
