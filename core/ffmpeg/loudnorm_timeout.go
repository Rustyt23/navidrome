package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

const (
	// probeTimeout bounds a container probe. Probing reads headers rather than
	// audio, so it is fast regardless of how long the track is.
	probeTimeout = 2 * time.Minute

	// decodeTimeoutBase is the fixed allowance added to every decode, covering
	// process startup and slow storage.
	decodeTimeoutBase = 2 * time.Minute

	// maxDecodeTimeout stops a nonsensical duration - a corrupt header can
	// report hours for a three-minute song - from disabling the timeout.
	maxDecodeTimeout = 2 * time.Hour

	// unknownDurationTimeout is used when the length is not known.
	unknownDurationTimeout = 30 * time.Minute
)

// DecodeTimeout returns how long a full decode of a track of this length may
// run before it is treated as hung.
//
// Decoding runs many times faster than real time - around 20x for MP3 - so
// allowing a second of wall clock per second of audio is very generous, while
// still bounding a process that has stopped making progress. Without this a
// single corrupt file silently occupies a worker forever, which on a
// multi-day library run is a stall nobody would notice until the end.
func DecodeTimeout(durationSeconds float64) time.Duration {
	if durationSeconds <= 0 {
		return unknownDurationTimeout
	}
	timeout := decodeTimeoutBase + time.Duration(durationSeconds*float64(time.Second))
	if timeout > maxDecodeTimeout {
		return maxDecodeTimeout
	}
	return timeout
}

// runCommand runs an ffmpeg/ffprobe command under a deadline.
//
// An existing earlier deadline on ctx is respected, so a caller that already
// bounded the work keeps its own limit. A timeout is reported distinctly from
// a cancellation: being stopped by the user is not a failure worth flagging,
// but a hung file is.
func runCommand(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, error) {
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > timeout {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput() // #nosec
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return out, fmt.Errorf("%s timed out after %s: the file may be corrupt or unreadable",
			name, timeout)
	}
	return out, err
}
