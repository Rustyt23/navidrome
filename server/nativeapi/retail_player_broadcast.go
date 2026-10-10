package nativeapi

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/navidrome/navidrome/core/stream"
	"github.com/navidrome/navidrome/log"
)

const (
	retailPlayerListenerWriteTimeout = 5 * time.Second
	retailPlayerBroadcastIdleTimeout = 30 * time.Second
	retailPlayerSongRetryInterval    = 4 * time.Second
	retailPlayerMP3FrameDuration     = 1152 * time.Second / 44100
)

// One independently decodable silent frame, generated with libmp3lame,
// 128 kbps, 44100 Hz, stereo and -reservoir 0. Silence maintains a valid MP3
// timeline while a lookup/transcode is pending or the upstream device is idle.
//
//go:embed retail_player_silence.mp3
var retailPlayerSilence []byte

type retailPlayerAudioFrame struct {
	data     []byte // immutable after publication
	duration time.Duration
	err      error
}

type retailPlayerBroadcastProducer func(context.Context, retailPlayerDevice, func(retailPlayerAudioFrame))

// Each canonical device has a single timeline. All access to broadcasts,
// history and subscriber membership is guarded by mu; socket writes never
// happen under this lock or in the producer goroutine.
type retailPlayerBroadcasts struct {
	mu          sync.Mutex
	active      map[string]*retailPlayerBroadcast
	produce     retailPlayerBroadcastProducer
	buffer      time.Duration
	idleTimeout time.Duration
}

type retailPlayerBroadcast struct {
	device      retailPlayerDevice
	cancel      context.CancelFunc
	finished    chan struct{}
	idle        *time.Timer
	history     []retailPlayerAudioFrame
	duration    time.Duration
	subscribers map[*retailPlayerAudioSubscription]struct{}
}

type retailPlayerAudioSubscription struct {
	hub       *retailPlayerBroadcasts
	broadcast *retailPlayerBroadcast
	frames    chan retailPlayerAudioFrame
	done      chan struct{}
}

func newRetailPlayerBroadcasts(produce retailPlayerBroadcastProducer, buffer time.Duration) *retailPlayerBroadcasts {
	if buffer < time.Second || buffer > 10*time.Second {
		buffer = 3 * time.Second
	}
	return &retailPlayerBroadcasts{
		active: make(map[string]*retailPlayerBroadcast), produce: produce,
		buffer: buffer, idleTimeout: retailPlayerBroadcastIdleTimeout,
	}
}

func (h *retailPlayerBroadcasts) Subscribe(device retailPlayerDevice) *retailPlayerAudioSubscription {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := strings.TrimSpace(device.ID)
	b := h.active[id]
	if b == nil {
		ctx, cancel := context.WithCancel(context.Background())
		b = &retailPlayerBroadcast{
			device: device, cancel: cancel, finished: make(chan struct{}),
			subscribers: make(map[*retailPlayerAudioSubscription]struct{}),
		}
		h.active[id] = b
		go func() {
			defer close(b.finished)
			defer func() {
				h.mu.Lock()
				defer h.mu.Unlock()
				h.stopLocked(b)
			}()
			h.produce(ctx, device, func(frame retailPlayerAudioFrame) { h.publish(b, frame) })
		}()
	}
	if b.idle != nil {
		b.idle.Stop()
		b.idle = nil
	}
	// The recent burst plus at most two seconds of pending live audio.
	capacity := int((h.buffer+2*time.Second)/retailPlayerMP3FrameDuration) + 2
	s := &retailPlayerAudioSubscription{
		hub: h, broadcast: b, frames: make(chan retailPlayerAudioFrame, capacity), done: make(chan struct{}),
	}
	for _, frame := range b.history {
		s.frames <- frame
	}
	b.subscribers[s] = struct{}{}
	return s
}

func (h *retailPlayerBroadcasts) publish(b *retailPlayerBroadcast, frame retailPlayerAudioFrame) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.active[strings.TrimSpace(b.device.ID)] != b {
		return
	}
	b.history = append(b.history, frame)
	b.duration += frame.duration
	for len(b.history) > 1 && b.duration > h.buffer {
		b.duration -= b.history[0].duration
		b.history[0] = retailPlayerAudioFrame{}
		b.history = b.history[1:]
	}
	for s := range b.subscribers {
		select {
		case s.frames <- frame:
		default:
			// A lagging listener must reconnect at the live position; dropping
			// arbitrary bytes would corrupt MP3 frames and accumulate drift.
			delete(b.subscribers, s)
			close(s.done)
			log.Debug("Retail player stream: disconnecting slow listener", "device", b.device.Name)
		}
	}
	h.scheduleIdleLocked(b)
}

func (h *retailPlayerBroadcasts) scheduleIdleLocked(b *retailPlayerBroadcast) {
	if len(b.subscribers) != 0 || b.idle != nil {
		return
	}
	var timer *time.Timer
	timer = time.AfterFunc(h.idleTimeout, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if b.idle == timer && len(b.subscribers) == 0 {
			h.stopLocked(b)
		}
	})
	b.idle = timer
}

func (h *retailPlayerBroadcasts) stopLocked(b *retailPlayerBroadcast) {
	id := strings.TrimSpace(b.device.ID)
	if h.active[id] != b {
		return
	}
	delete(h.active, id)
	if b.idle != nil {
		b.idle.Stop()
	}
	b.cancel()
	for s := range b.subscribers {
		delete(b.subscribers, s)
		close(s.done)
	}
}

func (s *retailPlayerAudioSubscription) Close() {
	h := s.hub
	h.mu.Lock()
	defer h.mu.Unlock()
	b := s.broadcast
	if _, ok := b.subscribers[s]; !ok {
		return
	}
	delete(b.subscribers, s)
	close(s.done)
	h.scheduleIdleLocked(b)
}

func (n *Router) produceRetailPlayerBroadcast(ctx context.Context, device retailPlayerDevice, publish func(retailPlayerAudioFrame)) {
	if strings.TrimSpace(device.RemoteControlID) == "" {
		device.RemoteControlID = n.storedRetailPlayerRemoteControlID(ctx, device.ID)
	}
	var source retailPlayerSongSource
	if sub := n.songTracker.Subscribe(device); sub != nil {
		source = sub
	} else {
		source = newRetailPlayerPollingSongSource(device.ID)
	}
	defer source.Close()
	log.Info(ctx, "Retail player broadcast started", "device", device.Name, "deviceID", device.ID)
	defer log.Info(ctx, "Retail player broadcast stopped", "device", device.Name, "deviceID", device.ID)
	runRetailPlayerBroadcast(ctx, source, n.openRetailPlayerBroadcastSong, publish, retailPlayerSongRetryInterval)
}

func (n *Router) openRetailPlayerBroadcastSong(ctx context.Context, song string) (io.ReadCloser, error) {
	mf := n.findRetailPlayerMediaFileByName(ctx, song)
	if mf == nil {
		return nil, fmt.Errorf("retail player song not found: %s", song)
	}
	return n.streamer.NewStream(ctx, mf, stream.Request{
		Format: retailPlayerStreamFormat, BitRate: retailPlayerStreamBitRate,
		SampleRate: retailPlayerStreamSampleRate, Channels: retailPlayerStreamChannels,
	})
}

type retailPlayerSongOpener func(context.Context, string) (io.ReadCloser, error)

// A single wall clock drives every listener, including silence and transitions.
// Lookup/transcoding/reads run separately so a slow or broken source cannot stop
// the broadcast clock. Readers are cancelled and closed on a song change.
func runRetailPlayerBroadcast(ctx context.Context, source retailPlayerSongSource, open retailPlayerSongOpener, publish func(retailPlayerAudioFrame), retryInterval time.Duration) {
	silence := retailPlayerAudioFrame{data: retailPlayerSilence, duration: retailPlayerMP3FrameDuration}
	var frames <-chan retailPlayerAudioFrame
	var cancelSong context.CancelFunc
	defer func() {
		if cancelSong != nil {
			cancelSong()
		}
	}()
	var song string
	var retryAt time.Time
	finished := false
	next := time.Now()
	for ctx.Err() == nil {
		current := source.CurrentSong()
		if current != song {
			if cancelSong != nil {
				cancelSong()
			}
			song, frames, finished, retryAt = current, nil, false, time.Time{}
		}
		if song != "" && frames == nil && !finished && !time.Now().Before(retryAt) {
			frames, cancelSong = startRetailPlayerSong(ctx, song, open)
		}
		frame := silence
		select {
		case result := <-frames:
			if result.err != nil {
				cancelSong()
				frames = nil
				finished = errors.Is(result.err, io.EOF)
				retryAt = time.Now().Add(retryInterval)
				if !finished {
					log.Warn(ctx, "Retail player broadcast: retrying song", "song", song, "err", result.err)
				}
			} else {
				frame = result
			}
		default:
		}
		publish(frame)
		next = next.Add(frame.duration)
		// Rebase after a scheduling stall instead of sending an unbounded burst.
		if now := time.Now(); next.Before(now) {
			next = now
		}
		if !sleepOrDone(ctx, time.Until(next)) {
			return
		}
	}
}

func startRetailPlayerSong(ctx context.Context, song string, open retailPlayerSongOpener) (<-chan retailPlayerAudioFrame, context.CancelFunc) {
	songCtx, cancel := context.WithCancel(ctx)
	return readRetailPlayerSong(songCtx, song, open), cancel
}

func readRetailPlayerSong(ctx context.Context, song string, open retailPlayerSongOpener) <-chan retailPlayerAudioFrame {
	frames := make(chan retailPlayerAudioFrame, 8)
	go func() {
		send := func(frame retailPlayerAudioFrame) bool {
			select {
			case frames <- frame:
				return true
			case <-ctx.Done():
				return false
			}
		}
		r, err := open(ctx, song)
		if err != nil {
			send(retailPlayerAudioFrame{err: err})
			return
		}
		var closeOnce sync.Once
		closeReader := func() { closeOnce.Do(func() { _ = r.Close() }) }
		stop := context.AfterFunc(ctx, closeReader)
		defer stop()
		defer closeReader()
		reader := newRetailPlayerMP3Reader(r)
		readAudio := false
		for ctx.Err() == nil {
			data, duration, err := reader.Next()
			if errors.Is(err, io.EOF) && !readAudio {
				err = errors.New("song contains no MP3 frames")
			}
			readAudio = readAudio || len(data) > 0
			if !send(retailPlayerAudioFrame{data: data, duration: duration, err: err}) || err != nil {
				return
			}
		}
	}()
	return frames
}
