package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/navidrome/navidrome/conf"
)

func testRetailTracked(t *testing.T) (*retailPlayerSongTracker, *retailPlayerTrackedDevice) {
	t.Helper()
	tracker := newRetailPlayerSongTracker()
	tracked := &retailPlayerTrackedDevice{
		remoteControlID: "rc", deviceID: "pool", ready: make(chan struct{}),
		cancel: func() {}, subscribers: make(map[*retailPlayerSongSubscription]struct{}),
	}
	tracker.devices["rc"] = tracked
	return tracker, tracked
}

func retailTestStatus(song string) retailPlayerDeviceStatusResponse {
	return retailPlayerDeviceStatusResponse{Status: map[string]any{"activeStreamName": song}}
}

func TestRetailTrackerFallbackForDisconnectedAndStaleSocket(t *testing.T) {
	tracker, tracked := testRetailTracked(t)
	var calls int
	tracker.fetchStatus = func(context.Context, string) (retailPlayerDeviceStatusResponse, error) {
		calls++
		return retailTestStatus("fallback"), nil
	}
	tracker.pollStatus(context.Background(), tracked)
	if tracked.songName != "fallback" || !tracked.readyClosed || calls != 1 {
		t.Fatal("disconnected socket did not fall back")
	}
	tracked.connected = true
	tracker.publish(tracked, "push")
	tracker.pollStatus(context.Background(), tracked)
	if calls != 1 || tracked.songName != "push" {
		t.Fatal("healthy socket was overwritten by polling")
	}
	tracked.lastMessage = time.Now().Add(-time.Minute)
	tracker.pollStatus(context.Background(), tracked)
	if calls != 2 || tracked.songName != "fallback" {
		t.Fatal("quiet socket did not recover using status")
	}
}

func TestRetailTrackerSlowPollCannotOverwriteNewPushOrReplacement(t *testing.T) {
	for _, replace := range []bool{false, true} {
		tracker, tracked := testRetailTracked(t)
		tracker.fetchStatus = func(context.Context, string) (retailPlayerDeviceStatusResponse, error) {
			if replace {
				tracker.mu.Lock()
				tracker.devices["rc"] = &retailPlayerTrackedDevice{songName: "replacement"}
				tracker.mu.Unlock()
			} else {
				tracker.publish(tracked, "new-push")
			}
			return retailTestStatus("stale-status"), nil
		}
		tracker.pollStatus(context.Background(), tracked)
		if tracked.songName == "stale-status" {
			t.Fatal("stale poll overwrote new state")
		}
		if replace {
			tracker.publish(tracked, "old-socket")
			if tracker.devices["rc"].songName != "replacement" {
				t.Fatal("old socket overwrote replacement")
			}
		}
	}
}

func TestRetailTrackerSharedWebsocketAndCancellation(t *testing.T) {
	previous := conf.Server.RetailPlayer
	defer func() { conf.Server.RetailPlayer = previous }()
	var connections atomic.Int32
	var polls atomic.Int32
	closed := make(chan struct{}, 4)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		defer func() { closed <- struct{}{} }()
		connections.Add(1)
		var request retailPlayerSocketSubscribe
		if err := conn.ReadJSON(&request); err != nil {
			return
		}
		if request.Payload.ObjID != "pool" {
			t.Error("wrong subscription device")
		}
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"payload":{"device":{"status":{"activeStreamName":"song"}}}}`))
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()
	conf.Server.RetailPlayer.RemoteControlSocketURL = "ws" + strings.TrimPrefix(server.URL, "http")
	tracker := newRetailPlayerSongTracker()
	tracker.fetchStatus = func(context.Context, string) (retailPlayerDeviceStatusResponse, error) {
		polls.Add(1)
		return retailTestStatus("song"), nil
	}
	var subs []*retailPlayerSongSubscription
	for i := 0; i < 4; i++ {
		s := tracker.Subscribe(retailPlayerDevice{ID: "pool", RemoteControlID: "rc"})
		subs = append(subs, s)
		defer s.Close()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, s := range subs {
		if !s.WaitReady(ctx, time.Second) || s.CurrentSong() != "song" {
			t.Fatal("listener did not receive shared song")
		}
	}
	deadline := time.Now().Add(time.Second)
	for connections.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if connections.Load() != 1 || polls.Load() > 1 {
		t.Fatalf("duplicate connections/pollers: %d/%d", connections.Load(), polls.Load())
	}
	for _, s := range subs {
		s.Close()
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("last close leaked websocket")
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if len(tracker.devices) != 0 {
		t.Fatal("tracker leaked device")
	}
}

func TestRetailSocketSongNameDiffs(t *testing.T) {
	for _, tc := range []struct{ json, want string }{
		{`{"payload":{"device":{"status":{"activeStreamName":"song"}}}}`, "song"},
		{`{"payload":{"device":{"status":{"activeStream":"legacy"}}}}`, "legacy"},
		{`{"payload":{"device":{"status":{"volume":5}}}}`, ""},
		{`invalid`, ""},
	} {
		if got := retailPlayerSocketSongName(json.RawMessage(tc.json)); got != tc.want {
			t.Fatalf("got %q, want %q", got, tc.want)
		}
	}
}
