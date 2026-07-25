package nativeapi

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
)

const (
	retailPlayerSocketDialTimeout = 10 * time.Second
	retailPlayerSocketRetryMin    = time.Second
	retailPlayerSocketRetryMax    = 30 * time.Second
	retailPlayerSocketSubID       = "remote-control"
	retailPlayerSocketTopic       = "device-diff"
)

// retailPlayerSocketSubscribe mirrors the subscribe frame the web player sends
// on the remote-control socket.
type retailPlayerSocketSubscribe struct {
	Type    string                             `json:"type"`
	Payload retailPlayerSocketSubscribePayload `json:"payload"`
}

type retailPlayerSocketSubscribePayload struct {
	SubsID string `json:"subsId"`
	Topic  string `json:"topic"`
	ObjID  string `json:"objId"`
}

// retailPlayerSocketFrame is the slice of a remote-control frame that says what
// the device is playing. Frames are diffs, so status may be absent entirely.
type retailPlayerSocketFrame struct {
	Payload struct {
		Device struct {
			Status map[string]any `json:"status"`
		} `json:"device"`
	} `json:"payload"`
}

// retailPlayerSocketSongName extracts the active song name from a frame,
// returning "" when the frame carries no playback state.
func retailPlayerSocketSongName(data []byte) string {
	var frame retailPlayerSocketFrame
	if err := json.Unmarshal(data, &frame); err != nil {
		return ""
	}
	name := normalizeStatusString(frame.Payload.Device.Status, "activeStreamName")
	if name == "" {
		name = normalizeStatusString(frame.Payload.Device.Status, "activeStream")
	}
	return name
}

// retailPlayerSongTracker keeps one background websocket per actively-streamed
// device and pushes song changes to every listener of that device, so the
// /music stream never has to poll the remote status API.
//
// Connections are reference counted: opened on the first subscriber for a
// device and closed with the last, so idle devices hold no sockets.
type retailPlayerSongTracker struct {
	mu      sync.Mutex
	devices map[string]*retailPlayerTrackedDevice
}

func newRetailPlayerSongTracker() *retailPlayerSongTracker {
	return &retailPlayerSongTracker{
		devices: make(map[string]*retailPlayerTrackedDevice),
	}
}

// retailPlayerTrackedDevice holds the shared state for one device's socket.
// Every mutable field is guarded by the owning tracker's mutex.
type retailPlayerTrackedDevice struct {
	remoteControlID string
	deviceID        string
	deviceName      string
	cancel          context.CancelFunc
	ready           chan struct{}

	refs        int
	readyClosed bool
	songName    string
	subscribers map[*retailPlayerSongSubscription]struct{}
}

// retailPlayerSongSubscription is one listener's view of a tracked device.
type retailPlayerSongSubscription struct {
	tracker *retailPlayerSongTracker
	device  *retailPlayerTrackedDevice
	changes chan struct{}
	closed  bool
}

// Subscribe starts (or joins) push tracking for a device. It returns nil when
// push tracking is impossible — no socket URL configured, or the device has no
// remoteControlId — so the caller can fall back to polling.
func (t *retailPlayerSongTracker) Subscribe(device retailPlayerDevice) *retailPlayerSongSubscription {
	base := strings.TrimSpace(conf.Server.RetailPlayer.RemoteControlSocketURL)
	remoteControlID := strings.TrimSpace(device.RemoteControlID)
	if base == "" || remoteControlID == "" {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	tracked, ok := t.devices[remoteControlID]
	if !ok {
		ctx, cancel := context.WithCancel(context.Background())
		tracked = &retailPlayerTrackedDevice{
			remoteControlID: remoteControlID,
			deviceID:        strings.TrimSpace(device.ID),
			deviceName:      strings.TrimSpace(device.Name),
			cancel:          cancel,
			ready:           make(chan struct{}),
			subscribers:     make(map[*retailPlayerSongSubscription]struct{}),
		}
		t.devices[remoteControlID] = tracked
		go t.run(ctx, tracked)
	}

	sub := &retailPlayerSongSubscription{
		tracker: t,
		device:  tracked,
		// Buffered so a publish never blocks on a listener that is busy writing
		// audio; one pending token is all a listener needs.
		changes: make(chan struct{}, 1),
	}
	tracked.refs++
	tracked.subscribers[sub] = struct{}{}
	return sub
}

// publish records a new song for a device and wakes its listeners.
func (t *retailPlayerSongTracker) publish(remoteControlID, songName string) {
	t.mu.Lock()
	tracked, ok := t.devices[remoteControlID]
	if !ok {
		t.mu.Unlock()
		return
	}
	if !tracked.readyClosed {
		tracked.readyClosed = true
		close(tracked.ready)
	}
	if tracked.songName == songName {
		t.mu.Unlock()
		return
	}
	tracked.songName = songName
	log.Debug("Retail player song socket: change received", "device", tracked.deviceName,
		"song", songName, "listeners", len(tracked.subscribers))
	subs := make([]*retailPlayerSongSubscription, 0, len(tracked.subscribers))
	for sub := range tracked.subscribers {
		subs = append(subs, sub)
	}
	t.mu.Unlock()

	for _, sub := range subs {
		select {
		case sub.changes <- struct{}{}:
		default:
		}
	}
}

// run keeps a device's socket alive, reconnecting with backoff until the last
// subscriber goes away and the context is cancelled.
func (t *retailPlayerSongTracker) run(ctx context.Context, tracked *retailPlayerTrackedDevice) {
	backoff := retailPlayerSocketRetryMin
	for {
		if ctx.Err() != nil {
			return
		}

		err := t.connectOnce(ctx, tracked)
		if ctx.Err() != nil {
			return
		}

		log.Debug(ctx, "Retail player song socket dropped, reconnecting",
			"device", tracked.deviceName, "remoteControlId", tracked.remoteControlID,
			"retryIn", backoff, "err", err)

		if !sleepOrDone(ctx, backoff) {
			return
		}
		backoff *= 2
		if backoff > retailPlayerSocketRetryMax {
			backoff = retailPlayerSocketRetryMax
		}
	}
}

// connectOnce dials, subscribes, and pumps frames until the socket fails.
func (t *retailPlayerSongTracker) connectOnce(ctx context.Context, tracked *retailPlayerTrackedDevice) error {
	base := strings.TrimRight(strings.TrimSpace(conf.Server.RetailPlayer.RemoteControlSocketURL), "/")
	endpoint := base + "/" + url.PathEscape(tracked.remoteControlID)

	dialCtx, cancelDial := context.WithTimeout(ctx, retailPlayerSocketDialTimeout)
	conn, resp, err := websocket.DefaultDialer.DialContext(dialCtx, endpoint, nil)
	cancelDial()
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	// ReadMessage blocks; closing the connection is what unblocks it when the
	// last listener leaves. `done` keeps this watcher from outliving the call.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	subscribe := retailPlayerSocketSubscribe{
		Type: "subscribe",
		Payload: retailPlayerSocketSubscribePayload{
			SubsID: retailPlayerSocketSubID,
			Topic:  retailPlayerSocketTopic,
			ObjID:  tracked.deviceID,
		},
	}
	if err := conn.WriteJSON(subscribe); err != nil {
		return err
	}

	log.Debug(ctx, "Retail player song socket subscribed",
		"device", tracked.deviceName, "remoteControlId", tracked.remoteControlID,
		"objId", tracked.deviceID)

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		if songName := retailPlayerSocketSongName(data); songName != "" {
			t.publish(tracked.remoteControlID, songName)
		}
	}
}

// Changes fires whenever the device starts playing something different.
func (s *retailPlayerSongSubscription) Changes() <-chan struct{} {
	return s.changes
}

// Drain clears a pending change signal. Callers do this before streaming a song
// so a stale token can't cut the new song off immediately.
func (s *retailPlayerSongSubscription) Drain() {
	select {
	case <-s.changes:
	default:
	}
}

// CurrentSong returns the device's latest reported song name ("" until the
// first frame arrives).
func (s *retailPlayerSongSubscription) CurrentSong() string {
	s.tracker.mu.Lock()
	defer s.tracker.mu.Unlock()
	return s.device.songName
}

// WaitReady blocks until the socket has reported a song at least once, the
// context ends, or timeout elapses. Reports whether a song is known.
func (s *retailPlayerSongSubscription) WaitReady(ctx context.Context, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-s.device.ready:
		return true
	case <-ctx.Done():
		return false
	case <-timer.C:
		return false
	}
}

// Close releases this listener, shutting the device's socket down when it was
// the last one.
func (s *retailPlayerSongSubscription) Close() {
	t := s.tracker
	t.mu.Lock()
	defer t.mu.Unlock()

	if s.closed {
		return
	}
	s.closed = true

	tracked := s.device
	delete(tracked.subscribers, s)
	tracked.refs--
	if tracked.refs <= 0 {
		delete(t.devices, tracked.remoteControlID)
		tracked.cancel()
	}
}

// retailPlayerSongSource is what the /music stream needs in order to follow a
// device: the current song, plus a signal when it changes. The websocket-backed
// subscription is the primary implementation; polling is the fallback for the
// (currently majority) of devices that have no remoteControlId.
type retailPlayerSongSource interface {
	CurrentSong() string
	Changes() <-chan struct{}
	Drain()
	WaitReady(ctx context.Context, timeout time.Duration) bool
	Close()
}

// retailPlayerPollingSongSource follows a device by polling the status API. It
// is only used when push tracking is impossible.
type retailPlayerPollingSongSource struct {
	cancel context.CancelFunc

	mu          sync.Mutex
	songName    string
	readyClosed bool

	ready   chan struct{}
	changes chan struct{}
}

const retailPlayerPollingInterval = 4 * time.Second

func newRetailPlayerPollingSongSource(deviceID string) *retailPlayerPollingSongSource {
	ctx, cancel := context.WithCancel(context.Background())
	p := &retailPlayerPollingSongSource{
		cancel:  cancel,
		ready:   make(chan struct{}),
		changes: make(chan struct{}, 1),
	}
	go p.run(ctx, deviceID)
	return p
}

func (p *retailPlayerPollingSongSource) run(ctx context.Context, deviceID string) {
	for {
		payload, err := fetchRetailPlayerDeviceStatusRaw(ctx, deviceID)
		if err == nil {
			if songName := retailPlayerStatusSongName(payload); songName != "" {
				p.publish(songName)
			}
		} else if ctx.Err() == nil {
			log.Debug(ctx, "Retail player polling song source: status fetch failed", "deviceID", deviceID, "err", err)
		}

		if !sleepOrDone(ctx, retailPlayerPollingInterval) {
			return
		}
	}
}

func (p *retailPlayerPollingSongSource) publish(songName string) {
	p.mu.Lock()
	if !p.readyClosed {
		p.readyClosed = true
		close(p.ready)
	}
	if p.songName == songName {
		p.mu.Unlock()
		return
	}
	p.songName = songName
	p.mu.Unlock()

	select {
	case p.changes <- struct{}{}:
	default:
	}
}

func (p *retailPlayerPollingSongSource) CurrentSong() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.songName
}

func (p *retailPlayerPollingSongSource) Changes() <-chan struct{} { return p.changes }

func (p *retailPlayerPollingSongSource) Drain() {
	select {
	case <-p.changes:
	default:
	}
}

func (p *retailPlayerPollingSongSource) WaitReady(ctx context.Context, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-p.ready:
		return true
	case <-ctx.Done():
		return false
	case <-timer.C:
		return false
	}
}

func (p *retailPlayerPollingSongSource) Close() { p.cancel() }
