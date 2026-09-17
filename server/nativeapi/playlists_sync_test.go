package nativeapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/core/playlists"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
)

// syncFailPlaylists is a Playlists whose track mutations succeed but whose
// Update (the disk-mirror step, called via syncPlaylist) always fails, modeling
// an unwritable/unconfigured PlaylistsPath.
type syncFailPlaylists struct {
	playlists.Playlists
	updateCalled bool
}

func (f *syncFailPlaylists) AddTracks(_ context.Context, _ string, ids []string) (int, error) {
	return len(ids), nil
}
func (f *syncFailPlaylists) AddAlbums(context.Context, string, []string) (int, error) {
	return 0, nil
}
func (f *syncFailPlaylists) AddArtists(context.Context, string, []string) (int, error) {
	return 0, nil
}
func (f *syncFailPlaylists) AddDiscs(context.Context, string, []model.DiscID) (int, error) {
	return 0, nil
}
func (f *syncFailPlaylists) Update(context.Context, string, *string, *string, *bool, []string, []int) error {
	f.updateCalled = true
	return errors.New("playlists path not configured")
}

// Adding tracks to a playlist must succeed even when mirroring the playlist to
// disk fails, because the tracks are already persisted before the mirror runs.
// This is the server half of the "create new playlist then add tracks" flow.
func TestAddToPlaylistSucceedsWhenDiskMirrorFails(t *testing.T) {
	svc := &syncFailPlaylists{}
	handler := addToPlaylist(svc)

	body := strings.NewReader(`{"ids":["s1","s2"]}`)
	// URLParamsMiddleware surfaces chi params as ":"-prefixed query values, which
	// is what req.Params(...).String(":playlistId") reads.
	req := httptest.NewRequest("POST", "/playlist/pl-1/tracks?:playlistId=pl-1", body)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200 despite mirror failure, got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"added":2`) {
		t.Fatalf("expected added:2 in response, got %q", w.Body.String())
	}
	if !svc.updateCalled {
		t.Fatal("expected the disk-mirror sync to have been attempted")
	}
}

// captureResource models the raw persistence resource used by createPlaylist. It
// rejects a save with an empty OwnerID exactly like the real playlist table's
// NOT NULL foreign key to user(id), so the test fails if the handler forgets to
// set the owner.
type captureResource struct {
	rest.Repository
	rest.Persistable
	saved *model.Playlist
}

func (c *captureResource) NewInstance() interface{} { return &model.Playlist{} }
func (c *captureResource) Save(entity interface{}) (string, error) {
	pls := entity.(*model.Playlist)
	if pls.OwnerID == "" {
		return "", errors.New("FOREIGN KEY constraint failed")
	}
	pls.ID = "pl-new"
	c.saved = pls
	return "pl-new", nil
}

type captureDS struct {
	model.DataStore
	res *captureResource
}

func (d *captureDS) Resource(context.Context, any) model.ResourceRepository { return d.res }

func TestCreatePlaylistSetsOwnerFromContext(t *testing.T) {
	res := &captureResource{}
	ds := &captureDS{res: res}
	// Update fails (disk mirror), which must not affect creation now that it is
	// best-effort — the FK/owner fix is what matters here.
	handler := createPlaylist(ds, &syncFailPlaylists{})

	body := strings.NewReader(`{"name":"40","public":false}`)
	req := httptest.NewRequest("POST", "/api/playlist", body)
	req.Header.Set("Content-type", "application/json")
	req = req.WithContext(request.WithUser(context.Background(), model.User{ID: "user-42"}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	if res.saved == nil {
		t.Fatal("playlist was never saved")
	}
	if res.saved.OwnerID != "user-42" {
		t.Fatalf("expected owner set from context, got %q", res.saved.OwnerID)
	}
	if !res.saved.Sync {
		t.Fatal("expected Sync=true to be preserved")
	}
	if !strings.Contains(w.Body.String(), `"id":"pl-new"`) {
		t.Fatalf("expected created id in response, got %q", w.Body.String())
	}
}
