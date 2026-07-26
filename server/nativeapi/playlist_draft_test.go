package nativeapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/criteria"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/tests"
)

// newDraftTestDataStore builds a live playlist p1 holding s1 and s2, plus a
// third song s3 available to be proposed.
func newDraftTestDataStore() (*tests.MockDataStore, *tests.MockPlaylistRepo, *tests.MockMediaFileRepo) {
	songs := tests.CreateMockMediaFileRepo()
	for _, songID := range []string{"s1", "s2", "s3"} {
		mf := model.MediaFile{ID: songID, Title: "Song " + songID, Artist: "Artist"}
		_ = songs.Put(&mf)
	}

	playlists := tests.CreateMockPlaylistRepo()
	live := &model.Playlist{ID: "p1", Name: "Store Mix", OwnerID: "u1"}
	live.AddMediaFilesByID([]string{"s1", "s2"})
	playlists.Data["p1"] = live

	ds := &tests.MockDataStore{MockedPlaylist: playlists, MockedMediaFile: songs}
	return ds, playlists, songs
}

// livePlaylistTrackIDs reads the mock playlist's current order.
func livePlaylistTrackIDs(playlists *tests.MockPlaylistRepo, playlistID string) []string {
	pls, err := playlists.Get(playlistID)
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(pls.Tracks))
	for _, track := range pls.MediaFiles() {
		ids = append(ids, track.ID)
	}
	return ids
}

// stubDraftRepo records what the handlers ask of the repository so the HTTP
// layer can be tested without a database.
type stubDraftRepo struct {
	drafts        map[string]*model.PlaylistDraft
	lastTrackIDs  []string
	lastChanges   []model.PlaylistDraftChange
	lastStatus    model.PlaylistDraftStatus
	lastActor     string
	publishCalled bool
	publishErr    error
	decisions     model.PlaylistRecommendationDecisions
	versions      model.PlaylistVersions
	auditEvents   model.PlaylistAuditEvents
}

func (s *stubDraftRepo) Get(id string) (*model.PlaylistDraft, error) {
	if draft, ok := s.drafts[id]; ok {
		return draft, nil
	}
	return nil, model.ErrNotFound
}

func (s *stubDraftRepo) GetAll(string) (model.PlaylistDrafts, error) {
	out := model.PlaylistDrafts{}
	for _, draft := range s.drafts {
		out = append(out, *draft)
	}
	return out, nil
}

func (s *stubDraftRepo) Create(draft *model.PlaylistDraft) error {
	if draft.ID == "" {
		draft.ID = "draft-1"
	}
	if draft.Status == "" {
		draft.Status = model.DraftStatusDraft
	}
	copied := *draft
	s.drafts[draft.ID] = &copied
	return nil
}

func (s *stubDraftRepo) SetTracks(id string, trackIDs []string, changes []model.PlaylistDraftChange) error {
	s.lastTrackIDs = trackIDs
	s.lastChanges = changes
	if draft, ok := s.drafts[id]; ok {
		draft.ProposedTrackIDs = trackIDs
		draft.Changes = changes
	}
	return nil
}

func (s *stubDraftRepo) UpdateStatus(id string, next model.PlaylistDraftStatus, actor string) error {
	s.lastStatus, s.lastActor = next, actor
	if draft, ok := s.drafts[id]; ok {
		draft.Status = next
	}
	return nil
}

func (s *stubDraftRepo) MarkConflicted(string, string) error { return nil }
func (s *stubDraftRepo) Delete(string) error                 { return nil }

func (s *stubDraftRepo) Publish(_ string, actor string) error {
	s.publishCalled = true
	s.lastActor = actor
	return s.publishErr
}

func (s *stubDraftRepo) GetRecommendationDecisions(_, _ string) (model.PlaylistRecommendationDecisions, error) {
	return s.decisions, nil
}

func (s *stubDraftRepo) SaveRecommendationDecision(decision *model.PlaylistRecommendationDecision) error {
	s.decisions = append(s.decisions, *decision)
	return nil
}

func (s *stubDraftRepo) GetVersions(string) (model.PlaylistVersions, error) {
	return s.versions, nil
}

func (s *stubDraftRepo) GetVersion(id string) (*model.PlaylistVersion, error) {
	for i := range s.versions {
		if s.versions[i].ID == id {
			return &s.versions[i], nil
		}
	}
	return nil, model.ErrNotFound
}

func (s *stubDraftRepo) GetAuditEvents(string) (model.PlaylistAuditEvents, error) {
	return s.auditEvents, nil
}

func draftTestRequest(t *testing.T, router *Router, method, target string, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	router.addPlaylistDraftRoutes(r)

	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("{}")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	req = req.WithContext(request.WithUser(req.Context(), model.User{ID: "u1", UserName: "reviewer"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestCreatePlaylistDraftDoesNotTouchTheLivePlaylist(t *testing.T) {
	ds, playlists, _ := newDraftTestDataStore()
	repo := &stubDraftRepo{drafts: map[string]*model.PlaylistDraft{}}
	ds.MockedPlaylistDraft = repo
	router := &Router{ds: ds}

	body := `{"playlistId":"p1","name":"AI cleanup","operations":[
		{"kind":"add","mediaFileId":"s3","reason":"clean alternative","source":"ai","confidence":90}
	]}`
	rec := draftTestRequest(t, router, http.MethodPost, "/playlist-draft", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// The proposal folds the operation onto the live order...
	if got := repo.lastTrackIDs; len(got) != 3 || got[2] != "s3" {
		t.Fatalf("unexpected proposed order: %v", got)
	}
	// ...and the AI's reason is stored with the change, not discarded.
	if len(repo.lastChanges) != 1 || repo.lastChanges[0].Reason != "clean alternative" {
		t.Fatalf("expected the AI reason to be recorded, got %+v", repo.lastChanges)
	}
	// The live playlist is untouched: only a draft was written.
	if live := livePlaylistTrackIDs(playlists, "p1"); len(live) != 2 {
		t.Fatalf("live playlist must not change, got %v", live)
	}
}

func TestCreatePlaylistDraftRejectsSmartPlaylists(t *testing.T) {
	ds, playlists, _ := newDraftTestDataStore()
	playlists.Data["p1"].Rules = &criteria.Criteria{Expression: criteria.All{}}
	ds.MockedPlaylistDraft = &stubDraftRepo{drafts: map[string]*model.PlaylistDraft{}}
	router := &Router{ds: ds}

	rec := draftTestRequest(t, router, http.MethodPost, "/playlist-draft", `{"playlistId":"p1"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a smart playlist, got %d", rec.Code)
	}
}

func TestCreatePlaylistDraftRejectsAnImpossibleOperation(t *testing.T) {
	ds, _, _ := newDraftTestDataStore()
	ds.MockedPlaylistDraft = &stubDraftRepo{drafts: map[string]*model.PlaylistDraft{}}
	router := &Router{ds: ds}

	body := `{"playlistId":"p1","operations":[{"kind":"remove","mediaFileId":"not-in-playlist"}]}`
	rec := draftTestRequest(t, router, http.MethodPost, "/playlist-draft", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// Publishing must not be reachable by asking for a status change: it is the
// only operation that writes to a live playlist and has its own endpoint.
func TestPlaylistDraftStatusCannotPublish(t *testing.T) {
	ds, _, _ := newDraftTestDataStore()
	repo := &stubDraftRepo{drafts: map[string]*model.PlaylistDraft{
		"d1": {ID: "d1", PlaylistID: "p1", Status: model.DraftStatusApproved},
	}}
	ds.MockedPlaylistDraft = repo
	router := &Router{ds: ds}

	rec := draftTestRequest(t, router, http.MethodPost, "/playlist-draft/d1/status", `{"status":"published"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if repo.publishCalled {
		t.Fatal("status endpoint must never publish")
	}
}

func TestPublishPlaylistDraftReportsStaleAsConflict(t *testing.T) {
	ds, _, _ := newDraftTestDataStore()
	repo := &stubDraftRepo{
		drafts:     map[string]*model.PlaylistDraft{"d1": {ID: "d1", PlaylistID: "p1", Status: model.DraftStatusApproved}},
		publishErr: model.ErrDraftStale,
	}
	ds.MockedPlaylistDraft = repo
	router := &Router{ds: ds}

	rec := draftTestRequest(t, router, http.MethodPost, "/playlist-draft/d1/publish", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for a stale draft, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPublishPlaylistDraftRecordsThePublisher(t *testing.T) {
	ds, _, _ := newDraftTestDataStore()
	repo := &stubDraftRepo{drafts: map[string]*model.PlaylistDraft{
		"d1": {ID: "d1", PlaylistID: "p1", Status: model.DraftStatusApproved},
	}}
	ds.MockedPlaylistDraft = repo
	router := &Router{ds: ds}

	rec := draftTestRequest(t, router, http.MethodPost, "/playlist-draft/d1/publish", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !repo.publishCalled || repo.lastActor != "reviewer" {
		t.Fatalf("expected publish by %q, got called=%v actor=%q", "reviewer", repo.publishCalled, repo.lastActor)
	}
}

func TestPlaylistDraftDiffMarksStale(t *testing.T) {
	ds, _, _ := newDraftTestDataStore()
	// A source version that does not match the live playlist's content.
	ds.MockedPlaylistDraft = &stubDraftRepo{drafts: map[string]*model.PlaylistDraft{
		"d1": {
			ID: "d1", PlaylistID: "p1", Status: model.DraftStatusApproved,
			SourceVersion:    "stale-version",
			ProposedTrackIDs: []string{"s1"},
		},
	}}
	router := &Router{ds: ds}

	rec := draftTestRequest(t, router, http.MethodGet, "/playlist-draft/d1/diff", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var diff model.PlaylistDraftDiff
	if err := json.Unmarshal(rec.Body.Bytes(), &diff); err != nil {
		t.Fatalf("decoding diff: %v", err)
	}
	if !diff.Stale {
		t.Fatal("expected the diff to report the draft as stale")
	}
	if len(diff.Removed) != 1 || diff.Removed[0].MediaFileID != "s2" {
		t.Fatalf("expected s2 to be reported as removed, got %+v", diff.Removed)
	}
}

func TestRecommendationDecisionIsRecordedWithoutChangingLivePlaylist(t *testing.T) {
	ds, playlists, _ := newDraftTestDataStore()
	repo := &stubDraftRepo{drafts: map[string]*model.PlaylistDraft{
		"draft-1": {ID: "draft-1", PlaylistID: "p1", Status: model.DraftStatusDraft},
	}}
	ds.MockedPlaylistDraft = repo
	router := &Router{ds: ds}
	before := livePlaylistTrackIDs(playlists, "p1")

	rec := draftTestRequest(
		t,
		router,
		http.MethodPost,
		"/playlist-draft/recommendation-decisions",
		`{"playlistId":"p1","draftId":"draft-1","songId":"s3","recommendationType":"playlist_expansion","decision":"ignored"}`,
	)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(repo.decisions) != 1 || repo.decisions[0].Decision != model.RecommendationDecisionIgnored ||
		repo.decisions[0].DraftID != "draft-1" || repo.decisions[0].CreatedBy != "reviewer" {
		t.Fatalf("unexpected recorded decision: %+v", repo.decisions)
	}
	if got := livePlaylistTrackIDs(playlists, "p1"); !slices.Equal(got, before) {
		t.Fatalf("decision changed live playlist: before=%v after=%v", before, got)
	}
}

func TestRollbackCreatesDraftWithoutChangingLivePlaylistOrHistory(t *testing.T) {
	ds, playlists, _ := newDraftTestDataStore()
	repo := &stubDraftRepo{
		drafts: map[string]*model.PlaylistDraft{},
		versions: model.PlaylistVersions{{
			ID: "version-1", PlaylistID: "p1", Version: 1,
			TrackIDs: []string{"s2"},
		}},
	}
	ds.MockedPlaylistDraft = repo
	router := &Router{ds: ds}
	before := livePlaylistTrackIDs(playlists, "p1")

	rec := draftTestRequest(
		t,
		router,
		http.MethodPost,
		"/playlist-history/version-1/rollback",
		"",
	)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var draft model.PlaylistDraft
	if err := json.Unmarshal(rec.Body.Bytes(), &draft); err != nil {
		t.Fatalf("decode rollback draft: %v", err)
	}
	if draft.RollbackVersionID != "version-1" || draft.RollbackVersionNumber != 1 ||
		draft.Status != model.DraftStatusDraft || !slices.Equal(draft.ProposedTrackIDs, []string{"s2"}) {
		t.Fatalf("unexpected rollback draft: %+v", draft)
	}
	if got := livePlaylistTrackIDs(playlists, "p1"); !slices.Equal(got, before) {
		t.Fatalf("rollback creation changed live playlist: before=%v after=%v", before, got)
	}
	if len(repo.versions) != 1 {
		t.Fatalf("rollback must not delete history: %+v", repo.versions)
	}
	if len(repo.lastChanges) == 0 || repo.lastChanges[0].Source != "rollback" {
		t.Fatalf("expected explanatory rollback changes, got %+v", repo.lastChanges)
	}
}

func TestPlaylistHistoryReturnsBeforeAfterAndAudit(t *testing.T) {
	ds, _, _ := newDraftTestDataStore()
	repo := &stubDraftRepo{
		drafts: map[string]*model.PlaylistDraft{},
		versions: model.PlaylistVersions{{
			ID: "version-1", PlaylistID: "p1", Version: 1, DraftID: "d1",
			PreviousTrackIDs: []string{"s1", "s2"},
			TrackIDs:         []string{"s1", "s3"},
			Changes: []model.PlaylistDraftChange{{
				Kind: model.DraftChangeReplace, MediaFileID: "s3", ReplacedID: "s2", Source: "ai",
			}},
		}},
		auditEvents: model.PlaylistAuditEvents{{
			ID: "event-1", PlaylistID: "p1", EventType: model.AuditPublished,
		}},
	}
	ds.MockedPlaylistDraft = repo
	router := &Router{ds: ds}

	rec := draftTestRequest(t, router, http.MethodGet, "/playlist-history?playlistId=p1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Versions []playlistHistoryVersionView `json:"versions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if len(payload.Versions) != 1 || len(payload.Versions[0].Before) != 2 ||
		len(payload.Versions[0].After) != 2 || len(payload.Versions[0].ChangeDetails) != 1 ||
		payload.Versions[0].ChangeDetails[0].ReplacedTitle != "Song s2" {
		t.Fatalf("unexpected history view: %+v", payload.Versions)
	}

	audit := draftTestRequest(t, router, http.MethodGet, "/playlist-history/audit?playlistId=p1", "")
	if audit.Code != http.StatusOK || !strings.Contains(audit.Body.String(), "published") {
		t.Fatalf("unexpected audit response %d: %s", audit.Code, audit.Body.String())
	}
}
