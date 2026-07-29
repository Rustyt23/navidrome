package nativeapi

import (
	"sync"
	"sync/atomic"
)

// Three things rewrite files in the library: a whole-library run, an
// optimisation of hand-picked songs, and a restore. Each locks a track while it
// works, so an overlap cannot corrupt a file - but the order they finish in is
// undefined, and that is enough to lose work:
//
//   - a restore that completes while an optimisation is still working through
//     its list is silently undone when the run reaches that track;
//   - each writes its audit record after releasing the file, so the two can
//     finish in one order and record in the other, leaving the database
//     describing a file that is not the one on disk.
//
// Only the whole-library run used to announce itself, so restore could only be
// held back from that one. All three now claim the same right, and whichever
// asks second is told what is already running.
var (
	loudnessFileWorkMu       sync.Mutex
	selectionLoudnessRunning atomic.Bool
	restoreLoudnessRunning   atomic.Bool
)

// loudnessFileWorkInProgress names the operation currently rewriting library
// files, or "" when none is. Callers must hold loudnessFileWorkMu.
func loudnessFileWorkInProgress() string {
	switch {
	case libraryLoudness.running.Load():
		return "a whole-library LUFS run"
	case selectionLoudnessRunning.Load():
		return "an optimisation of selected songs"
	case restoreLoudnessRunning.Load():
		return "a restore"
	}
	return ""
}

// claimLoudnessFileWork reserves the right to rewrite library files. It returns
// a release function, or the name of the operation already holding it.
func claimLoudnessFileWork(flag *atomic.Bool) (release func(), busy string) {
	loudnessFileWorkMu.Lock()
	defer loudnessFileWorkMu.Unlock()

	if busy := loudnessFileWorkInProgress(); busy != "" {
		return nil, busy
	}
	flag.Store(true)
	return func() { flag.Store(false) }, ""
}

// loudnessFileWorkBusy reports what is rewriting files, for callers that manage
// their own running flag rather than claiming one here.
func loudnessFileWorkBusy() string {
	loudnessFileWorkMu.Lock()
	defer loudnessFileWorkMu.Unlock()
	return loudnessFileWorkInProgress()
}
