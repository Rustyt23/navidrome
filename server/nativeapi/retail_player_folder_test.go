package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/db"
	"github.com/navidrome/navidrome/persistence"
)

type retailPlayerDevicesPayload struct {
	Data    []map[string]any `json:"data"`
	Folders []map[string]any `json:"folders"`
}

func setupRetailPlayerTest(t *testing.T) func() {
	t.Helper()
	conf.Server.DbPath = path.Join(t.TempDir(), "test.db")
	conf.Server.RetailPlayer.Enabled = true
	conf.Server.RetailPlayer.OrgID = "test-org"
	conf.Server.RetailPlayer.BaseURL = "http://example.com"
	conf.Server.RetailPlayer.APIKeyHeader = "X-Test-Key"
	conf.Server.RetailPlayer.APIKey = "secret"

	cleanup := db.Init(context.Background())
	return cleanup
}

func TestRetailPlayerCreateFolderPersists(t *testing.T) {
	cleanup := setupRetailPlayerTest(t)
	defer cleanup()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orgs/test-org/devices" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	prevClient := retailPlayerHTTPClient
	retailPlayerHTTPClient = ts.Client()
	defer func() { retailPlayerHTTPClient = prevClient }()

	conf.Server.RetailPlayer.BaseURL = ts.URL

	ds := persistence.New(db.Db())
	router := &Router{ds: ds, devices: newRetailPlayerDeviceResolver()}

	createHandler := router.handleCreateRetailPlayerFolder()
	req := httptest.NewRequest(http.MethodPost, "/retailplayer/folders", strings.NewReader(`{"name":"Test Folder"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	createHandler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("unexpected status: %d", rr.Code)
	}

	getHandler := router.handleRetailPlayerDevices()
	getReq := httptest.NewRequest(http.MethodGet, "/retailplayer/devices", nil)
	getRec := httptest.NewRecorder()
	getHandler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("unexpected status from devices: %d", getRec.Code)
	}

	var payload retailPlayerDevicesPayload
	if err := json.Unmarshal(getRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	if len(payload.Folders) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(payload.Folders))
	}
	if name, _ := payload.Folders[0]["name"].(string); name != "Test Folder" {
		t.Fatalf("unexpected folder name: %s", name)
	}
}
