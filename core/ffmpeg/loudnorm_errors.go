package ffmpeg

import (
	"fmt"
	"strings"
)

const (
	// errorDetailLines is how many of ffmpeg's last lines are kept. The reason
	// for a failure is printed at the end.
	errorDetailLines = 3
	// errorDetailRunes caps the kept text, so one long line cannot fill a page.
	errorDetailRunes = 300
)

// commandError wraps a failed ffmpeg run with the part of its output that says
// why it failed.
//
// The whole output used to be attached. ffmpeg prints a progress line every
// half second, so a song stopped a minute in carried hundreds of them - a page
// of numbers on screen and in the song's record, with the actual reason, if
// there was one, somewhere at the end.
//
// Only the error text is shortened. The output a successful run parses for its
// measurements is untouched: making ffmpeg itself print less would also stop it
// printing the loudness figures, and every measurement would fail.
func commandError(action string, err error, output []byte) error {
	if detail := errorDetail(output); detail != "" {
		return fmt.Errorf("%s: %w: %s", action, err, detail)
	}
	return fmt.Errorf("%s: %w", action, err)
}

// errorDetail keeps the last few lines of ffmpeg output that are not progress
// or its standard description of the input and output.
func errorDetail(output []byte) string {
	lines := strings.FieldsFunc(string(output), func(r rune) bool { return r == '\n' || r == '\r' })
	var kept []string
	for _, line := range lines {
		if isRoutineOutput(line) {
			continue
		}
		kept = append(kept, strings.TrimSpace(line))
	}
	if len(kept) > errorDetailLines {
		kept = kept[len(kept)-errorDetailLines:]
	}
	detail := []rune(strings.Join(kept, " | "))
	if len(detail) > errorDetailRunes {
		// The end is kept: that is where ffmpeg says what went wrong.
		detail = append([]rune("…"), detail[len(detail)-errorDetailRunes:]...)
	}
	return string(detail)
}

// isRoutineOutput reports lines ffmpeg prints on every run, failed or not.
// Anything not recognised here is kept, so a format change in a future ffmpeg
// can only make the detail longer, never hide an error.
func isRoutineOutput(line string) bool {
	trimmed := strings.TrimSpace(line)
	switch {
	case trimmed == "":
		return true
	// Metadata and stream descriptions are indented under their heading.
	case line[0] == ' ' || line[0] == '\t':
		return true
	// The progress line: "size=N/A time=00:01:27.90 bitrate=N/A speed=3.6x".
	case (strings.HasPrefix(trimmed, "size=") || strings.HasPrefix(trimmed, "frame=")) &&
		strings.Contains(trimmed, "time="):
		return true
	case strings.HasPrefix(trimmed, "Input #"),
		strings.HasPrefix(trimmed, "Output #"),
		trimmed == "Stream mapping:",
		strings.HasPrefix(trimmed, "Press [q]"):
		return true
	}
	return false
}
