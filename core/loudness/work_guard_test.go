package loudness

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFileWorkClaimPreservesExclusionAfterRepeatedRelease(t *testing.T) {
	first, busy := ClaimFileWork("optimisation")
	if busy != "" {
		t.Fatalf("first claim blocked by %q", busy)
	}
	defer first()
	if release, busy := ClaimFileWork("restore"); release != nil || busy != "optimisation" {
		t.Fatalf("overlapping claim = %v, %q", release != nil, busy)
	}
	first()
	second, busy := ClaimFileWork("restore")
	if busy != "" {
		t.Fatal(busy)
	}
	defer second()
	first()
	if got := FileWorkBusy(); got != "restore" {
		t.Fatalf("old release freed current claim: %q", got)
	}
}

func TestFileWorkWaitCancellationDoesNotReleaseCurrentJob(t *testing.T) {
	release, busy := ClaimFileWork("restore")
	if busy != "" {
		t.Fatal(busy)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if acquired, err := WaitForFileWork(ctx, "scanner"); acquired != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled wait = %v, %v", acquired != nil, err)
	}
	if got := FileWorkBusy(); got != "restore" {
		t.Fatalf("cancelled waiter changed current claim: %q", got)
	}
}

func TestFileWorkWaitContinuesAfterActiveJobFinishes(t *testing.T) {
	release, busy := ClaimFileWork("analysis")
	if busy != "" {
		t.Fatal(busy)
	}
	defer release()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	acquired := make(chan func(), 1)
	errors := make(chan error, 1)
	go func() {
		done, err := WaitForFileWork(ctx, "scanner")
		if err != nil {
			errors <- err
			return
		}
		acquired <- done
	}()
	release()
	select {
	case done := <-acquired:
		defer done()
		if got := FileWorkBusy(); got != "scanner" {
			t.Fatalf("waiter did not reserve work: %q", got)
		}
	case err := <-errors:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
