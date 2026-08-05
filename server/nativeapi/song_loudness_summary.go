package nativeapi

import (
	"encoding/json"
	"net/http"

	"github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/persistence"
)

// loudnessSummary is the whole library in one line.
//
// The table answers questions one song at a time, which is the wrong shape for
// the only question anyone actually asks first: is the library at the target?
// Counting that by scrolling is not an answer, and a page of six hundred rows
// with no total reads as a tool rather than as a report.
type loudnessSummary struct {
	Target    float64 `json:"target"`
	Tolerance float64 `json:"tolerance"`

	Songs int64 `json:"songs"`
	// OnTarget counts songs measuring within tolerance of the target, whether
	// they had to be changed to get there or were already fine.
	//
	// OnTarget, LevelTwo, Exceptions and NotMeasured partition the library: every
	// song is in exactly one. "Short of target" used to sit alongside them and
	// did not partition anything - it overlapped both LevelTwo and Exceptions, so
	// the panel invited adding numbers that double-counted.
	OnTarget int64 `json:"onTarget"`
	// NotMeasured is songs nothing is known about yet - the work still to do.
	NotMeasured int64 `json:"notMeasured"`
	// Changed counts songs whose files were rewritten. The difference between
	// this and OnTarget is the point: songs already on target were never opened.
	Changed int64 `json:"changed"`
	// Restorable counts songs whose untouched original is still stored.
	Restorable int64 `json:"restorable"`
	Exceptions int64 `json:"exceptions"`
	// LevelTwo is tracks held to the wider tolerance: left untouched because
	// correcting them was not worth a re-encode. They are not exceptions - nobody
	// has to do anything about them - but they are not silently on target either.
	LevelTwo int64 `json:"levelTwo"`
}

// loudnessSummaryHandler counts the library by outcome.
//
// Every count runs through the same filters the list itself uses, so the
// headline can never disagree with what the table shows when someone clicks
// into it. Six counting queries against indexed columns, and nothing is held in
// memory - the alternative, reading every row to tally it, is what the cursor
// in the analysis sweep exists to avoid.
func (n *Router) loudnessSummaryHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		w.Header().Set("Content-Type", "application/json")

		options := conf.Server.Scanner.LoudnessNormalization
		repo := n.ds.MediaFile(ctx)
		summary := loudnessSummary{
			Target:    options.TargetLUFS,
			Tolerance: effectiveManualLoudnessTolerance(options.Tolerance),
		}

		present := squirrel.Eq{"media_file.missing": false}
		count := func(where squirrel.Sqlizer) int64 {
			total, err := repo.CountAll(model.QueryOptions{Filters: where})
			if err != nil {
				log.Warn(ctx, "Could not count songs for the LUFS summary", err)
				return 0
			}
			return total
		}
		withOutcome := func(outcome string) squirrel.Sqlizer {
			return squirrel.And{present, persistence.LoudnessOutcomeFilter(outcome)}
		}

		summary.Songs = count(present)
		summary.OnTarget = count(withOutcome("on_target"))
		summary.NotMeasured = count(withOutcome("not_measured"))
		summary.Changed = count(squirrel.And{present,
			squirrel.Eq{"media_file_loudness.status": model.LoudnessStatusProcessed}})
		summary.Restorable = count(squirrel.And{present,
			squirrel.Eq{"media_file_loudness.has_backup": true}})
		// Both through the list's own expressions. A second copy here is how the
		// headline came to disagree with the page underneath it.
		summary.Exceptions = count(squirrel.And{present, persistence.LoudnessExceptionFilter()})
		summary.LevelTwo = count(squirrel.And{present, persistence.LoudnessLevelTwoFilter()})

		_ = json.NewEncoder(w).Encode(summary)
	}
}
