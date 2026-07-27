package ffmpeg

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDecodeTimeoutScalesWithLength(t *testing.T) {
	// A four-minute song: generous next to the ~12s a decode really takes.
	if got := DecodeTimeout(240); got != decodeTimeoutBase+240*time.Second {
		t.Errorf("4 min track -> %s", got)
	}
	// A two-hour DJ set must not be killed part-way through.
	if got := DecodeTimeout(7200); got <= 30*time.Minute {
		t.Errorf("2 hour set -> %s, too tight", got)
	}
	// A corrupt header reporting a nonsense length must not disable the bound.
	if got := DecodeTimeout(9999999); got != maxDecodeTimeout {
		t.Errorf("absurd duration -> %s, want the cap %s", got, maxDecodeTimeout)
	}
	// Unknown length still gets a bound rather than running forever.
	if got := DecodeTimeout(0); got != unknownDurationTimeout {
		t.Errorf("unknown duration -> %s", got)
	}
}

// A process that stops making progress must be killed rather than occupying a
// worker for the rest of the run.
func TestRunCommandKillsAHungProcess(t *testing.T) {
	start := time.Now()
	_, err := runCommand(context.Background(), 300*time.Millisecond, "sleep", "60")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected the hung command to be killed")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error should name the timeout, got: %v", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("took %s to give up: the process was not killed promptly", elapsed)
	}
	t.Logf("hung command killed after %s: %v", elapsed.Round(time.Millisecond), err)
}

// Being stopped by the user is not a timeout and must not be reported as one.
func TestRunCommandDistinguishesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	_, err := runCommand(ctx, time.Minute, "sleep", "60")
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "timed out") {
		t.Errorf("a user cancellation was misreported as a timeout: %v", err)
	}
}

// A caller that already set a tighter deadline keeps it.
func TestRunCommandRespectsAnEarlierDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := runCommand(ctx, time.Hour, "sleep", "60"); err == nil {
		t.Fatal("expected an error")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("the caller's own deadline was ignored (took %s)", elapsed)
	}
}
