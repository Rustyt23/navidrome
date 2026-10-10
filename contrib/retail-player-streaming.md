# Shared RetailPlayer audio streams

`/music/{device}` now creates one broadcast per resolved device ID, per Navidrome
process. All listeners receive the same paced MP3 frames (128 kbps target,
44.1 kHz, stereo). Device-name/ID aliases use the same broadcast.

New listeners receive the recent audio buffer and then join the ongoing feed,
instead of restarting the current song. A broadcast stays alive for 30 seconds
after its last listener disconnects, allowing brief reconnects without restarting
the source. After that idle period, its reader, tracker and buffer are released.

This is HTTP unicast distribution, not IP multicast or clock-synchronized audio.
Players retain control of their own playback buffers. Similar players and buffer
settings should be used when aiming for a few seconds of difference. A broadcast
does not know the original RetailPlayer device's position within a song; the first
listener after a cold start starts that song from the beginning. Subsequent
listeners join this shared timeline. Exact speaker synchronization requires a
single audio output/distribution system or synchronization-capable players.

## Configuration

The default recent-audio buffer is 3 seconds. Merge this option into the existing
RetailPlayer configuration, preserving its API credentials and other settings:

```toml
[RetailPlayer]
StreamBuffer = "3s"
```

Environment equivalent: `ND_RETAILPLAYER_STREAMBUFFER=3s`. Allowed values are
1–10 seconds; an out-of-range value uses the 3-second default. Configuration
changes require a restart. This is the server's recent-audio burst for late
joiners, not a command that sets a player's internal buffer. A cold broadcast
builds its buffer in real time.

Keep transcoding caching enabled and size `TranscodingCacheSize` for the active
rotation and available disk space. It is a top-level setting, outside the
`[RetailPlayer]` section. One hour at 128 kbps is approximately 58 MB. The `/music`
profile includes sample rate and channels, so ordinary web-player cache entries
may not match it. The transcoding concurrency limit controls encoding jobs, not
the number of listeners; a low limit may cause retries and temporary silence.

## Reverse proxy

The application sends `X-Accel-Buffering: no`. Nginx normally honors this header;
check that `proxy_ignore_headers` does not disable it. An example location is:

```nginx
location /music/ {
    # Replace with the actual Navidrome upstream. No trailing URI rewrite.
    proxy_pass http://127.0.0.1:4533;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header Connection "";
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 300s;
    gzip off;
}
```

Adapt the prefix if using a Navidrome base path. Run `nginx -t` before reloading.
This repository change does not modify the production proxy or deploy a server.
If multiple Navidrome replicas are used, route a given channel to one instance;
the broadcast clock and recent-audio buffer are process-local.

## Recovery and capacity

- One source tracker/poller and one song reader serve all listeners of a device.
- WebSocket tracking has shared status-poll fallback for disconnected, initially
  unready, or quiet (30 seconds without a song update) sockets. Polls run at a
  four-second interval plus request time. A stale poll cannot overwrite a newer
  WebSocket update.
- Missing songs, failed transcodes, or a stalled reader produce valid silent MP3
  frames. Failures retry after four seconds. At normal end-of-file, silence
  continues until the upstream device reports its next song. This keeps the
  connection alive; it cannot restore music that is unavailable at the source.
- A song change cancels the previous reader. The broadcast clock continues
  across transitions and retries, without giving every song a fresh head start.
- Slow listeners have bounded queues and five-second socket-write deadlines.
  A listener that falls too far behind is disconnected and must reconnect using
  its player's automatic-retry behavior. Other listeners keep playing.
- HEAD probes return headers without opening a broadcast or tracker.

Four devices use approximately 512 kbps of audio payload in total. Allow 1–2 Mbps
of stable server upload bandwidth for these streams. Same-venue devices also
share the venue's internet download capacity. Wired connections usually make
testing and operation more predictable.

## Verification

Automated tests cover four listeners, aliases, late joins, short reconnects,
concurrent subscriptions, bounded slow-client queues, idle cleanup, source
failures/retries, cancellation of blocked reads, frame pacing, WebSocket
fallback, fragmented MP3 input, and real FFmpeg decoding of silence/transitions.

```sh
go test -tags netgo,sqlite_fts5 -race ./server/nativeapi -run '^TestRetail' -count=3 -shuffle=on
go test -tags netgo,sqlite_fts5 ./server/nativeapi ./core/stream ./utils/cache ./conf
```

For final deployment acceptance, use the actual four players: start two, join
the others mid-song, disconnect/reconnect one, then leave them playing through
several song changes. Compare audible positions and check server logs for
retries or slow-listener disconnects. Browser/firmware buffering and real venue
network behavior cannot be established by server-side tests alone.

The embedded silent frame was generated from FFmpeg's `anullsrc`, with
`-ar 44100 -ac 2 -c:a libmp3lame -b:a 128k -reservoir 0 -write_xing 0
-id3v2_version 0 -f mp3`; the second complete MPEG frame is retained. It contains
no external audio. The FFmpeg test verifies that repetitions decode to zero PCM.
