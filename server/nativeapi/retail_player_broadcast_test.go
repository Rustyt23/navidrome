package nativeapi

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/stream"
	"github.com/navidrome/navidrome/model"
)

func testRetailFrame(sequence uint32) retailPlayerAudioFrame {
	data := bytes.Clone(retailPlayerSilence)
	binary.BigEndian.PutUint32(data[len(data)-4:], sequence)
	return retailPlayerAudioFrame{data: data, duration: retailPlayerMP3FrameDuration}
}

func testRetailSequence(frame retailPlayerAudioFrame) uint32 {
	return binary.BigEndian.Uint32(frame.data[len(frame.data)-4:])
}

func testRetailReceive(t *testing.T, sub *retailPlayerAudioSubscription) retailPlayerAudioFrame {
	t.Helper()
	select {
	case frame := <-sub.frames:
		return frame
	case <-sub.done:
		t.Fatal("subscriber unexpectedly disconnected")
	case <-time.After(3 * time.Second):
		t.Fatal("timed out receiving broadcast")
	}
	return retailPlayerAudioFrame{}
}

func testRetailHub(t *testing.T, produce retailPlayerBroadcastProducer, buffer time.Duration) *retailPlayerBroadcasts {
	t.Helper()
	h := newRetailPlayerBroadcasts(produce, buffer)
	t.Cleanup(func() {
		h.mu.Lock()
		var finished []<-chan struct{}
		for _, b := range h.active {
			finished = append(finished, b.finished)
			h.stopLocked(b)
		}
		h.mu.Unlock()
		for _, done := range finished {
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Error("broadcast producer did not stop")
			}
		}
	})
	return h
}

func TestRetailBroadcastFourListenersAndLateJoin(t *testing.T) {
	var starts atomic.Int32
	h := testRetailHub(t, func(ctx context.Context, _ retailPlayerDevice, _ func(retailPlayerAudioFrame)) {
		starts.Add(1)
		<-ctx.Done()
	}, time.Second)
	device := retailPlayerDevice{ID: "pool", Name: "Pool"}
	first := h.Subscribe(device)
	for i := uint32(1); i <= 150; i++ {
		h.publish(first.broadcast, testRetailFrame(i))
		if got := testRetailSequence(testRetailReceive(t, first)); got != i {
			t.Fatalf("first listener frame %d, want %d", got, i)
		}
	}
	// Join after almost four seconds of audio: receive only the last second,
	// not the beginning of the song. Aliases with the same ID share the feed.
	var others []*retailPlayerAudioSubscription
	for i := 0; i < 3; i++ {
		alias := device
		alias.Name = "pool-alias"
		sub := h.Subscribe(alias)
		others = append(others, sub)
		if sub.broadcast != first.broadcast {
			t.Fatal("alias created another producer")
		}
		start := testRetailSequence(testRetailReceive(t, sub))
		if start < 112 || start > 114 {
			t.Fatalf("late join began at %d, want recent audio", start)
		}
		for expected := start + 1; expected <= 150; expected++ {
			if got := testRetailSequence(testRetailReceive(t, sub)); got != expected {
				t.Fatalf("history gap: %d != %d", got, expected)
			}
		}
	}
	for i := uint32(151); i <= 200; i++ {
		h.publish(first.broadcast, testRetailFrame(i))
		for _, sub := range append(others, first) {
			if got := testRetailSequence(testRetailReceive(t, sub)); got != i {
				t.Fatalf("listeners diverged: %d != %d", got, i)
			}
		}
	}
	// The HTTP request that first created the feed must not own its lifetime.
	first.Close()
	h.publish(others[0].broadcast, testRetailFrame(201))
	for _, sub := range others {
		if got := testRetailSequence(testRetailReceive(t, sub)); got != 201 {
			t.Fatal("first disconnect stopped shared feed")
		}
	}
	deadline := time.Now().Add(time.Second)
	for starts.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if starts.Load() != 1 {
		t.Fatalf("started %d producers", starts.Load())
	}
}

func TestRetailBroadcastSlowListenerIsolatedAndHistoryBounded(t *testing.T) {
	h := testRetailHub(t, func(ctx context.Context, _ retailPlayerDevice, _ func(retailPlayerAudioFrame)) { <-ctx.Done() }, time.Second)
	fast := h.Subscribe(retailPlayerDevice{ID: "pool"})
	slow := h.Subscribe(retailPlayerDevice{ID: "pool"})
	for i := uint32(1); i <= 10000; i++ {
		h.publish(fast.broadcast, testRetailFrame(i))
		if got := testRetailSequence(testRetailReceive(t, fast)); got != i {
			t.Fatal("fast listener lost audio")
		}
	}
	select {
	case <-slow.done:
	default:
		t.Fatal("slow listener not evicted")
	}
	if fast.broadcast.duration > time.Second || len(fast.broadcast.history) > 39 {
		t.Fatal("unbounded history")
	}
	if len(slow.frames) != cap(slow.frames) {
		t.Fatal("slow queue not bounded")
	}
	slow.Close() // safe after eviction, and repeated closes must not panic
	slow.Close()
	newListener := h.Subscribe(retailPlayerDevice{ID: "pool"})
	if got := testRetailSequence(testRetailReceive(t, newListener)); got < 9960 {
		t.Fatalf("reconnect too far behind: %d", got)
	}
}

func TestRetailBroadcastReconnectGraceAndIdleCleanup(t *testing.T) {
	h := testRetailHub(t, func(ctx context.Context, _ retailPlayerDevice, _ func(retailPlayerAudioFrame)) { <-ctx.Done() }, time.Second)
	h.idleTimeout = 100 * time.Millisecond
	device := retailPlayerDevice{ID: "pool"}
	sub := h.Subscribe(device)
	original := sub.broadcast
	sub.Close()
	h.publish(original, testRetailFrame(50))
	reconnected := h.Subscribe(device)
	if reconnected.broadcast != original || testRetailSequence(testRetailReceive(t, reconnected)) != 50 {
		t.Fatal("reconnect restarted track")
	}
	reconnected.Close()
	select {
	case <-original.finished:
	case <-time.After(2 * time.Second):
		t.Fatal("idle source leaked")
	}
	if next := h.Subscribe(device); next.broadcast == original {
		t.Fatal("expired broadcast reused")
	}
}

func TestRetailBroadcastConcurrentSubscriptions(t *testing.T) {
	var starts atomic.Int32
	h := testRetailHub(t, func(ctx context.Context, _ retailPlayerDevice, publish func(retailPlayerAudioFrame)) {
		starts.Add(1)
		publish(testRetailFrame(1))
		<-ctx.Done()
	}, time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Go(func() {
			s := h.Subscribe(retailPlayerDevice{ID: "pool"})
			select {
			case <-s.frames:
			case <-time.After(time.Second):
				t.Error("missing shared frame")
			}
			s.Close()
		})
	}
	wg.Wait()
	if starts.Load() != 1 {
		t.Fatalf("concurrent joins started %d producers", starts.Load())
	}
}

func TestRetailBroadcastBufferValidation(t *testing.T) {
	for _, value := range []time.Duration{0, -time.Second, 500 * time.Millisecond, 11 * time.Second} {
		if h := newRetailPlayerBroadcasts(nil, value); h.buffer != 3*time.Second {
			t.Fatalf("unsafe buffer accepted: %v", value)
		}
	}
	if h := newRetailPlayerBroadcasts(nil, 5*time.Second); h.buffer != 5*time.Second {
		t.Fatal("configured buffer ignored")
	}
}

func TestRetailBroadcastChannelsRemainIndependent(t *testing.T) {
	h := testRetailHub(t, func(ctx context.Context, _ retailPlayerDevice, _ func(retailPlayerAudioFrame)) { <-ctx.Done() }, time.Second)
	pool := h.Subscribe(retailPlayerDevice{ID: "pool"})
	lobby := h.Subscribe(retailPlayerDevice{ID: "lobby"})
	h.publish(pool.broadcast, testRetailFrame(10))
	h.publish(lobby.broadcast, testRetailFrame(20))
	if testRetailSequence(testRetailReceive(t, pool)) != 10 || testRetailSequence(testRetailReceive(t, lobby)) != 20 {
		t.Fatal("channels mixed")
	}
	pool.Close()
	h.publish(lobby.broadcast, testRetailFrame(21))
	if testRetailSequence(testRetailReceive(t, lobby)) != 21 {
		t.Fatal("closing one channel stopped another")
	}
}

func TestRetailBroadcastEmptyTranscodeIsRetryable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	frames := readRetailPlayerSong(ctx, "empty", func(context.Context, string) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(nil)), nil })
	select {
	case frame := <-frames:
		if frame.err == nil || errors.Is(frame.err, io.EOF) {
			t.Fatalf("empty transcode treated as completed song: %v", frame.err)
		}
	case <-time.After(time.Second):
		t.Fatal("empty transcode hung")
	}
}

type retailDeadlineWriter struct {
	header   http.Header
	deadline time.Time
	writes   int
}

func (w *retailDeadlineWriter) Header() http.Header { return w.header }
func (w *retailDeadlineWriter) WriteHeader(int)     {}
func (w *retailDeadlineWriter) Flush()              {}
func (w *retailDeadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return nil
}
func (w *retailDeadlineWriter) Write([]byte) (int, error) {
	w.writes++
	return 0, os.ErrDeadlineExceeded
}

func TestRetailBroadcastHTTPWriteDeadlineReleasesListener(t *testing.T) {
	previous := conf.Server.RetailPlayer
	t.Cleanup(func() { conf.Server.RetailPlayer = previous })
	conf.Server.RetailPlayer.Enabled = true
	h := testRetailHub(t, func(ctx context.Context, _ retailPlayerDevice, publish func(retailPlayerAudioFrame)) {
		publish(testRetailFrame(1))
		<-ctx.Done()
	}, time.Second)
	router := &Router{devices: newRetailPlayerDeviceResolver(), musicBroadcasts: h, streamer: &testRetailUnusedStreamer{}}
	router.musicBroadcastsOnce.Do(func() {})
	router.devices.Remember(retailPlayerDevice{ID: "pool", Name: "Pool"})
	w := &retailDeadlineWriter{header: make(http.Header)}
	start := time.Now()
	router.RetailPlayerMusicStreamHandler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/pool", nil))
	if w.writes != 1 || w.deadline.Before(start.Add(4*time.Second)) || w.deadline.After(time.Now().Add(6*time.Second)) {
		t.Fatal("HTTP writer did not receive a bounded deadline")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if b := h.active["pool"]; b == nil || len(b.subscribers) != 0 || b.idle == nil {
		t.Fatal("failed socket retained its subscription")
	}
}

func testRetailSource(song string) *retailPlayerPollingSongSource {
	s := &retailPlayerPollingSongSource{ready: make(chan struct{}), changes: make(chan struct{}, 1), cancel: func() {}}
	s.publish(song)
	return s
}

func TestRetailBroadcastSongChangeRetryAndSilence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := testRetailSource("missing")
	var attempts atomic.Int32
	output := make(chan retailPlayerAudioFrame, 100)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runRetailPlayerBroadcast(ctx, source, func(_ context.Context, song string) (io.ReadCloser, error) {
			if song == "missing" && attempts.Add(1) == 1 {
				return nil, errors.New("temporary lookup/transcode failure")
			}
			sequence := uint32(100)
			if song == "next" {
				sequence = 200
			}
			return io.NopCloser(bytes.NewReader(bytes.Repeat(testRetailFrame(sequence).data, 5))), nil
		}, func(f retailPlayerAudioFrame) { output <- f }, 30*time.Millisecond)
	}()
	await := func(want uint32) {
		t.Helper()
		deadline := time.After(2 * time.Second)
		for {
			select {
			case f := <-output:
				if testRetailSequence(f) == want {
					return
				}
			case <-deadline:
				t.Fatalf("never recovered to frame %d", want)
			}
		}
	}
	select {
	case f := <-output:
		if !bytes.Equal(f.data, retailPlayerSilence) {
			t.Fatal("missing song did not produce silence")
		}
	case <-time.After(time.Second):
		t.Fatal("broadcast stalled")
	}
	await(100)
	if attempts.Load() < 2 {
		t.Fatal("failed song was not retried")
	}
	source.publish("next")
	await(200)
	// EOF keeps a valid clock while awaiting the next song, without replaying it.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case f := <-output:
			if bytes.Equal(f.data, retailPlayerSilence) {
				cancel()
				<-done
				return
			}
		case <-deadline:
			t.Fatal("EOF stalled stream")
		}
	}
}

func TestRetailBroadcastCancelsBlockedReadOnSongChange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := testRetailSource("blocked")
	reader, writer := io.Pipe()
	defer writer.Close()
	opened := make(chan struct{})
	output := make(chan retailPlayerAudioFrame, 100)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runRetailPlayerBroadcast(ctx, source, func(_ context.Context, song string) (io.ReadCloser, error) {
			if song == "blocked" {
				close(opened)
				return reader, nil
			}
			return io.NopCloser(bytes.NewReader(testRetailFrame(77).data)), nil
		}, func(f retailPlayerAudioFrame) { output <- f }, time.Second)
	}()
	select {
	case <-opened:
	case <-time.After(time.Second):
		t.Fatal("song not opened")
	}
	source.publish("working")
	deadline := time.After(2 * time.Second)
	for {
		select {
		case frame := <-output:
			if testRetailSequence(frame) == 77 {
				if _, err := writer.Write([]byte{1}); !errors.Is(err, io.ErrClosedPipe) {
					t.Fatalf("old reader not closed: %v", err)
				}
				cancel()
				<-done
				return
			}
		case <-deadline:
			t.Fatal("blocked read prevented song change")
		}
	}
}

func TestRetailBroadcastPacingDoesNotAccumulateAcrossSongs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	source := testRetailSource("first")
	var duration time.Duration
	count := 0
	start := time.Now()
	runRetailPlayerBroadcast(ctx, source, func(context.Context, string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bytes.Repeat(retailPlayerSilence, 100))), nil
	}, func(f retailPlayerAudioFrame) {
		duration += f.duration
		count++
		if count%3 == 0 {
			source.publish(time.Now().String())
		}
	}, time.Second)
	if difference := duration - time.Since(start); difference > 2*retailPlayerMP3FrameDuration || difference < -150*time.Millisecond {
		t.Fatalf("audio drifted %v across rapid song changes", difference)
	}
}

func TestRetailBroadcastHTTPFourClientsAndHead(t *testing.T) {
	previous := conf.Server.RetailPlayer
	t.Cleanup(func() { conf.Server.RetailPlayer = previous })
	conf.Server.RetailPlayer.Enabled = true
	var starts atomic.Int32
	h := testRetailHub(t, func(ctx context.Context, _ retailPlayerDevice, publish func(retailPlayerAudioFrame)) {
		starts.Add(1)
		for i := uint32(1); ctx.Err() == nil; i++ {
			publish(testRetailFrame(i))
			if !sleepOrDone(ctx, 5*time.Millisecond) {
				return
			}
		}
	}, time.Second)
	router := &Router{devices: newRetailPlayerDeviceResolver(), musicBroadcasts: h, streamer: &testRetailUnusedStreamer{}}
	router.musicBroadcastsOnce.Do(func() {})
	router.devices.Remember(retailPlayerDevice{ID: "pool-id", Name: "AlilaMarea_Pool"})
	server := httptest.NewServer(router.RetailPlayerMusicStreamHandler())
	defer server.Close()
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Head(server.URL + "/AlilaMarea_Pool")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 || starts.Load() != 0 {
		t.Fatal("HEAD started a producer")
	}
	if response.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatal("proxy buffering not disabled")
	}
	var responses []*http.Response
	defer func() {
		for _, r := range responses {
			r.Body.Close()
		}
	}()
	for i := 0; i < 4; i++ {
		path := "/AlilaMarea_Pool"
		if i%2 == 0 {
			path = "/pool-id"
		}
		r, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		responses = append(responses, r)
		if r.StatusCode != 200 || r.Header.Get("Content-Type") != "audio/mpeg" {
			t.Fatal("invalid stream response")
		}
	}
	for _, r := range responses {
		reader := newRetailPlayerMP3Reader(r.Body)
		for want := uint32(1); want <= 50; want++ {
			data, _, err := reader.Next()
			if err != nil {
				t.Fatal(err)
			}
			if got := testRetailSequence(retailPlayerAudioFrame{data: data}); got != want {
				t.Fatalf("HTTP clients differ: %d != %d", got, want)
			}
		}
	}
	responses[0].Body.Close()
	late, err := client.Get(server.URL + "/AlilaMarea_Pool")
	if err != nil {
		t.Fatal(err)
	}
	defer late.Body.Close()
	data, _, err := newRetailPlayerMP3Reader(late.Body).Next()
	if err != nil {
		t.Fatal(err)
	}
	if got := testRetailSequence(retailPlayerAudioFrame{data: data}); got <= 1 {
		t.Fatalf("late HTTP listener restarted: %d", got)
	}
	if starts.Load() != 1 {
		t.Fatalf("HTTP clients started %d producers", starts.Load())
	}
}

type testRetailUnusedStreamer struct{ stream.MediaStreamer }

func TestRetailBroadcastSilenceAndTransitionsDecode(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	cmd := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000", "-t", "0.3", "-ar", "44100", "-ac", "2", "-c:a", "libmp3lame", "-b:a", "128k", "-f", "mp3", "pipe:1")
	song, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var audio bytes.Buffer
	var songFrames [][]byte
	reader := newRetailPlayerMP3Reader(bytes.NewReader(song))
	for {
		data, _, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		audio.Write(data)
		songFrames = append(songFrames, data)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"silence", bytes.Repeat(retailPlayerSilence, 50)},
		{"song-silence-song", bytes.Join([][]byte{audio.Bytes(), bytes.Repeat(retailPlayerSilence, 20), audio.Bytes()}, nil)},
		{"late-join", bytes.Join(songFrames[len(songFrames)/2:], nil)},
		{"interrupted-song", bytes.Join([][]byte{
			bytes.Join(songFrames[:len(songFrames)/2], nil),
			bytes.Repeat(retailPlayerSilence, 20),
			bytes.Join(songFrames[len(songFrames)/2:], nil),
		}, nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decode := exec.Command(ffmpeg, "-v", "error", "-f", "mp3", "-i", "pipe:0", "-f", "s16le", "pipe:1")
			decode.Stdin = bytes.NewReader(tc.data)
			var stderr bytes.Buffer
			decode.Stderr = &stderr
			pcm, err := decode.Output()
			if err != nil || stderr.Len() != 0 {
				t.Fatalf("decode failed: %v: %s", err, &stderr)
			}
			if len(pcm) == 0 {
				t.Fatal("empty decoded audio")
			}
			if tc.name == "silence" && !bytes.Equal(pcm, make([]byte, len(pcm))) {
				t.Fatal("fallback is not silent")
			}
		})
	}
}

type retailBroadcastTestStore struct {
	model.DataStore
	repo *retailBroadcastTestRepo
}

func (s retailBroadcastTestStore) MediaFile(context.Context) model.MediaFileRepository { return s.repo }

type retailBroadcastTestRepo struct{ model.MediaFileRepository }

func (r *retailBroadcastTestRepo) Search(q string, _ ...model.QueryOptions) (model.MediaFiles, error) {
	return model.MediaFiles{{ID: q, Title: q, Path: q + ".mp3"}}, nil
}

type retailBroadcastTestStreamer struct{ opens atomic.Int32 }

func (s *retailBroadcastTestStreamer) NewStream(ctx context.Context, mf *model.MediaFile, request stream.Request) (*stream.Stream, error) {
	if request.Format != "mp3" || request.BitRate != 128 || request.SampleRate != 44100 || request.Channels != 2 {
		return nil, errors.New("incorrect broadcast encoding profile")
	}
	s.opens.Add(1)
	var data bytes.Buffer
	for i := uint32(1); i <= 400; i++ {
		sequence := i
		if mf.ID == "second" {
			sequence += 1000
		}
		data.Write(testRetailFrame(sequence).data)
	}
	return stream.NewStream(mf, "mp3", 128, io.NopCloser(bytes.NewReader(data.Bytes()))), nil
}

func TestRetailBroadcastEndToEndSharedPollingAndTranscode(t *testing.T) {
	previous := conf.Server.RetailPlayer
	t.Cleanup(func() { conf.Server.RetailPlayer = previous })
	var song atomic.Value
	song.Store("first")
	var polls atomic.Int32
	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		polls.Add(1)
		_, _ = io.WriteString(w, `{"status":{"activeStreamName":"`+song.Load().(string)+`"}}`)
	}))
	defer portal.Close()
	conf.Server.RetailPlayer.Enabled = true
	conf.Server.RetailPlayer.RemoteControlSocketURL = ""
	conf.Server.RetailPlayer.BaseURL = portal.URL
	conf.Server.RetailPlayer.OrgID = "test"
	streamer := &retailBroadcastTestStreamer{}
	router := &Router{
		ds: retailBroadcastTestStore{repo: &retailBroadcastTestRepo{}}, streamer: streamer,
		devices: newRetailPlayerDeviceResolver(), songTracker: newRetailPlayerSongTracker(),
	}
	// Supplying an RC ID avoids an irrelevant persisted-mapping lookup; an
	// empty socket URL intentionally exercises the real status-poll fallback.
	router.devices.Remember(retailPlayerDevice{ID: "pool", Name: "Pool", RemoteControlID: "rc"})
	router.musicBroadcasts = testRetailHub(t, router.produceRetailPlayerBroadcast, time.Second)
	router.musicBroadcastsOnce.Do(func() {})
	server := httptest.NewServer(router.RetailPlayerMusicStreamHandler())
	defer server.Close()
	client := &http.Client{Timeout: 10 * time.Second}
	var responses []*http.Response
	defer func() {
		for _, r := range responses {
			r.Body.Close()
		}
	}()
	connect := func(path string) *retailPlayerMP3Reader {
		t.Helper()
		r, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		responses = append(responses, r)
		return newRetailPlayerMP3Reader(r.Body)
	}
	readUntil := func(r *retailPlayerMP3Reader, want uint32) uint32 {
		t.Helper()
		for {
			data, _, err := r.Next()
			if err != nil {
				t.Fatal(err)
			}
			got := testRetailSequence(retailPlayerAudioFrame{data: data})
			if got >= want {
				return got
			}
		}
	}
	first := connect("/Pool")
	readUntil(first, 10)
	second := connect("/pool")
	third := connect("/Pool")
	fourth := connect("/pool")
	for _, r := range []*retailPlayerMP3Reader{first, second, third, fourth} {
		if got := readUntil(r, 70); got != 70 {
			t.Fatalf("listeners did not share audio timeline: %d", got)
		}
	}
	if streamer.opens.Load() != 1 || polls.Load() != 1 {
		t.Fatalf("per-listener work remains: %d transcodes, %d polls", streamer.opens.Load(), polls.Load())
	}
	responses[0].Body.Close()
	rejoined := connect("/Pool")
	data, _, err := rejoined.Next()
	if err != nil {
		t.Fatal(err)
	}
	if got := testRetailSequence(retailPlayerAudioFrame{data: data}); got < 30 || got > 75 {
		t.Fatalf("rejoin not at recent playback position: %d", got)
	}
	song.Store("second")
	for _, r := range []*retailPlayerMP3Reader{second, third, fourth, rejoined} {
		if got := readUntil(r, 1001); got != 1001 {
			t.Fatalf("song transition differed: %d", got)
		}
	}
	if streamer.opens.Load() != 2 {
		t.Fatalf("song change created %d total transcodes, want 2", streamer.opens.Load())
	}
}
