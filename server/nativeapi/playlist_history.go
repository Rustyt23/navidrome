package nativeapi

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
)

type playlistHistoryTrack struct {
	MediaFileID string `json:"mediaFileId"`
	Title       string `json:"title,omitempty"`
	Artist      string `json:"artist,omitempty"`
	Position    int    `json:"position"`
}

type playlistHistoryChange struct {
	Kind           model.PlaylistDraftChangeKind `json:"kind"`
	Source         string                        `json:"source"`
	AISelected     bool                          `json:"aiSelected"`
	MediaFileID    string                        `json:"mediaFileId,omitempty"`
	Title          string                        `json:"title,omitempty"`
	Artist         string                        `json:"artist,omitempty"`
	ReplacedID     string                        `json:"replacedId,omitempty"`
	ReplacedTitle  string                        `json:"replacedTitle,omitempty"`
	ReplacedArtist string                        `json:"replacedArtist,omitempty"`
	FromPosition   int                           `json:"fromPosition,omitempty"`
	ToPosition     int                           `json:"toPosition,omitempty"`
	Reason         string                        `json:"reason,omitempty"`
}

type playlistHistoryVersionView struct {
	model.PlaylistVersion
	Before        []playlistHistoryTrack  `json:"before"`
	After         []playlistHistoryTrack  `json:"after"`
	Added         []playlistHistoryTrack  `json:"added"`
	Removed       []playlistHistoryTrack  `json:"removed"`
	Reordered     []playlistHistoryTrack  `json:"reordered"`
	ChangeDetails []playlistHistoryChange `json:"changeDetails"`
}

func (n *Router) handleListPlaylistHistory(w http.ResponseWriter, r *http.Request) {
	playlistID := strings.TrimSpace(r.URL.Query().Get("playlistId"))
	if playlistID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "playlistId is required"})
		return
	}
	if _, err := n.ds.Playlist(r.Context()).Get(playlistID); err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	versions, err := n.ds.PlaylistDraft(r.Context()).GetVersions(playlistID)
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	views := n.playlistHistoryViews(r, versions)
	writeJSON(w, http.StatusOK, map[string]any{"versions": views})
}

func (n *Router) handleGetPlaylistHistoryVersion(w http.ResponseWriter, r *http.Request) {
	version, err := n.ds.PlaylistDraft(r.Context()).GetVersion(chi.URLParam(r, "id"))
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	if _, err := n.ds.Playlist(r.Context()).Get(version.PlaylistID); err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	views := n.playlistHistoryViews(r, model.PlaylistVersions{*version})
	writeJSON(w, http.StatusOK, views[0])
}

func (n *Router) handleListPlaylistAudit(w http.ResponseWriter, r *http.Request) {
	playlistID := strings.TrimSpace(r.URL.Query().Get("playlistId"))
	if playlistID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "playlistId is required"})
		return
	}
	if _, err := n.ds.Playlist(r.Context()).Get(playlistID); err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	events, err := n.ds.PlaylistDraft(r.Context()).GetAuditEvents(playlistID)
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

// A rollback is deliberately only a draft creation. It snapshots the current
// live content version, proposes the historical order, and then returns to the
// exact same review/approval/publish state machine as any other draft.
func (n *Router) handleCreatePlaylistRollbackDraft(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	version, err := n.ds.PlaylistDraft(ctx).GetVersion(chi.URLParam(r, "id"))
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	live, err := n.ds.Playlist(ctx).GetWithTracks(version.PlaylistID, false, true)
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	if live.IsSmartPlaylist() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": model.ErrDraftSmartPlaylist.Error()})
		return
	}
	liveIDs := mediaFileIDs(live)
	reason := fmt.Sprintf("Rollback to immutable playlist version %d", version.Version)
	changes := model.BuildRollbackChanges(liveIDs, version.TrackIDs, reason)
	user, _ := request.UserFrom(ctx)
	draft := &model.PlaylistDraft{
		PlaylistID:            version.PlaylistID,
		SourceVersion:         model.PlaylistContentVersion(liveIDs),
		Name:                  fmt.Sprintf("Rollback to version %d", version.Version),
		CreatedBy:             user.UserName,
		ProposedTrackIDs:      slices.Clone(version.TrackIDs),
		RollbackVersionID:     version.ID,
		RollbackVersionNumber: version.Version,
	}
	drafts := n.ds.PlaylistDraft(ctx)
	if err := drafts.Create(draft); err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	if err := drafts.SetTracks(draft.ID, draft.ProposedTrackIDs, changes); err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	stored, err := drafts.Get(draft.ID)
	if err != nil {
		writePlaylistDraftError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, stored)
}

func (n *Router) playlistHistoryViews(r *http.Request, versions model.PlaylistVersions) []playlistHistoryVersionView {
	songs := map[string]model.MediaFile{}
	mediaFiles := n.ds.MediaFile(r.Context())
	for _, version := range versions {
		for _, songID := range append(slices.Clone(version.PreviousTrackIDs), version.TrackIDs...) {
			if _, exists := songs[songID]; exists {
				continue
			}
			if song, err := mediaFiles.Get(songID); err == nil {
				songs[songID] = *song
			}
		}
	}

	views := make([]playlistHistoryVersionView, 0, len(versions))
	for _, version := range versions {
		pseudoDraft := &model.PlaylistDraft{
			ID:               version.DraftID,
			PlaylistID:       version.PlaylistID,
			SourceVersion:    model.PlaylistContentVersion(version.PreviousTrackIDs),
			ProposedTrackIDs: version.TrackIDs,
			Changes:          version.Changes,
		}
		reasons := map[string]model.PlaylistDraftChange{}
		for _, change := range version.Changes {
			reasons[change.MediaFileID] = change
		}
		diff := model.BuildDraftDiff(pseudoDraft, version.PreviousTrackIDs, songs, reasons)
		view := playlistHistoryVersionView{
			PlaylistVersion: version,
			Before:          historyTracks(diff.Before),
			After:           historyTracks(diff.After),
			Added:           historyTracks(diff.Added),
			Removed:         historyTracks(diff.Removed),
			Reordered:       historyTracks(diff.Moved),
			ChangeDetails:   historyChanges(version.Changes, songs),
		}
		views = append(views, view)
	}
	return views
}

func historyTracks(entries []model.DraftDiffEntry) []playlistHistoryTrack {
	tracks := make([]playlistHistoryTrack, 0, len(entries))
	for _, entry := range entries {
		tracks = append(tracks, playlistHistoryTrack{
			MediaFileID: entry.MediaFileID,
			Title:       entry.Title,
			Artist:      entry.Artist,
			Position:    entry.Position,
		})
	}
	return tracks
}

func historyChanges(changes []model.PlaylistDraftChange, songs map[string]model.MediaFile) []playlistHistoryChange {
	details := make([]playlistHistoryChange, 0, len(changes))
	for _, change := range changes {
		item := playlistHistoryChange{
			Kind: change.Kind, Source: change.Source,
			AISelected:  strings.EqualFold(change.Source, "ai"),
			MediaFileID: change.MediaFileID, ReplacedID: change.ReplacedID,
			FromPosition: change.FromPosition, ToPosition: change.ToPosition,
			Reason: change.Reason,
		}
		if song, ok := songs[change.MediaFileID]; ok {
			item.Title, item.Artist = song.Title, song.Artist
		}
		if song, ok := songs[change.ReplacedID]; ok {
			item.ReplacedTitle, item.ReplacedArtist = song.Title, song.Artist
		}
		details = append(details, item)
	}
	return details
}
