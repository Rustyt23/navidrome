package nativeapi

import (
	"context"
	"errors"
	"net/http"
	"runtime"
	"strings"
	"sync"

	"github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/silence"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

func (n *Router) startSilenceTrim() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids := decodeSilenceSelection(r)
		filter := silenceTrimFilter(ids)

		total, err := n.ds.MediaFile(r.Context()).CountAll(model.QueryOptions{Filters: filter})
		if err != nil {
			log.Warn(r.Context(), "Silence trim: could not count tracks", err)
		}
		if total == 0 {
			writeSilenceStatus(w, http.StatusOK,
				silenceTrim.status("Nothing to trim - analyse the songs first, or none of them need it"))
			return
		}

		started := silenceTrim.begin(r.Context(), total, func(ctx context.Context) {
			n.runSilenceTrim(ctx, ids)
		})
		if !started {
			writeSilenceStatus(w, http.StatusConflict,
				silenceTrim.status("A silence job is already running"))
			return
		}
		msg := "Trimming every song that needs it"
		if len(ids) > 0 {
			msg = "Trimming the songs you selected"
		}
		writeSilenceStatus(w, http.StatusAccepted, silenceTrim.status(msg))
	}
}

func (n *Router) silenceTrimStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeSilenceStatus(w, http.StatusOK, silenceTrim.status(""))
	}
}

func (n *Router) stopSilenceTrimHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !silenceTrim.running.Load() {
			writeSilenceStatus(w, http.StatusOK, silenceTrim.status("No trim is running"))
			return
		}
		silenceTrim.requestStop()
		writeSilenceStatus(w, http.StatusAccepted,
			silenceTrim.status("Stopping the trim after active songs finish"))
	}
}

// silenceTrimFilter selects what a trim run will actually cut.
//
// The gate is the stored analysis, not the selection: a song is trimmed only if
// it was measured, found trimmable, and has not been cut already. A selection
// narrows that set - it does not override it. This is the opposite of the LUFS
// page's rule, and deliberately so: there, picking a song by hand is a way to
// retry work that was refused, and retrying costs nothing but time. Here the
// refusals are the fade guard and the length caps, and overriding them by
// selecting a song would let a click remove the opening of a track with no
// backup to put it back.
func silenceTrimFilter(ids []string) squirrel.Sqlizer {
	conditions := squirrel.And{
		squirrel.Eq{"media_file.missing": false},
		squirrel.Eq{"media_file_silence.verdict": model.SilenceVerdictTrimmable},
		squirrel.Eq{"media_file_silence.trimmed_at": nil},
	}
	if len(ids) > 0 {
		return append(conditions, squirrel.Eq{"media_file.id": ids})
	}
	return conditions
}

func (n *Router) runSilenceTrim(ctx context.Context, ids []string) {
	defer silenceTrim.finish()

	log.Info(ctx, "Silence trim started", "selected", len(ids))
	cursor, err := n.ds.MediaFile(ctx).GetCursor(
		model.QueryOptions{Filters: silenceTrimFilter(ids)})
	if err != nil {
		log.Error(ctx, "Silence trim: could not read media files", err)
		return
	}

	workers := runtime.NumCPU()
	work := make(chan model.MediaFile)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			trimmer := ffmpeg.NewSilenceTrimmer()
			for mf := range work {
				silenceTrim.inFlight.Add(1)
				n.trimOneTrack(ctx, trimmer, &mf)
				silenceTrim.inFlight.Add(-1)
				silenceTrim.processed.Add(1)
			}
		}()
	}

	stop := silenceTrim.stopSignal()
dispatch:
	for mf, err := range cursor {
		if err != nil {
			log.Error(ctx, "Silence trim aborted: error reading media files", err)
			break
		}
		if silenceTrim.stopping.Load() {
			log.Info(ctx, "Silence trim stopped on request", "processed", silenceTrim.processed.Load())
			break
		}
		// As in the analysis run: the stored path is library-relative and
		// useless on its own.
		if strings.TrimSpace(mf.Path) == "" || strings.TrimSpace(mf.LibraryPath) == "" {
			silenceTrim.skipped.Add(1)
			silenceTrim.processed.Add(1)
			continue
		}
		select {
		case <-stop:
			break dispatch
		case work <- mf:
		}
	}
	close(work)
	wg.Wait()

	log.Info(ctx, "Silence trim finished",
		"processed", silenceTrim.processed.Load(),
		"trimmed", silenceTrim.changed.Load(),
		"skipped", silenceTrim.skipped.Load(),
		"failed", silenceTrim.failed.Load(),
		"secondsRemoved", float64(silenceTrim.millisRemoved.Load())/1000,
		"cancelled", silenceTrim.cancelled.Load())
}

func (n *Router) trimOneTrack(ctx context.Context, trimmer ffmpeg.SilenceTrimmer, mf *model.MediaFile) {
	repo := n.ds.SilenceAudit(ctx)
	audit, err := repo.Get(mf.ID)
	if err != nil || audit == nil {
		// The query selected it on the strength of a record that is no longer
		// readable. Skipped rather than failed: nothing is wrong with the audio.
		silenceTrim.skipped.Add(1)
		return
	}

	updated, res, err := silence.Trim(ctx, trimmer, mf, audit)
	switch {
	case errors.Is(err, silence.ErrNothingToTrim), errors.Is(err, silence.ErrAlreadyTrimmed):
		silenceTrim.skipped.Add(1)
		return
	case err != nil && ctx.Err() != nil:
		// Killed when the grace period ran out. The trim writes to a temp file
		// and only renames at the end, so the song is untouched and the next run
		// picks it up.
		silenceTrim.cancelled.Add(1)
		return
	case err != nil:
		silenceTrim.failed.Add(1)
		log.Warn(ctx, "Silence trim: could not trim track", "path", mf.Path, err)
	default:
		silenceTrim.changed.Add(1)
		silenceTrim.addRemoved(res.LeadTrim + res.TrailTrim)
	}

	if updated != nil {
		if err := repo.Put(updated); err != nil {
			log.Error(ctx, "Could not save the silence record after trimming", "path", mf.Path, err)
		}
	}

	// The file's duration and size have changed, so what the library believes
	// about it is now wrong. Refreshing it here keeps the player and the song
	// list honest without waiting for the next scan.
	if res.Trimmed {
		if err := n.refreshTrimmedMediaFile(ctx, mf, res); err != nil {
			log.Warn(ctx, "Trimmed the song but could not update the library record",
				"path", mf.Path, err)
		}
	}
}

// refreshTrimmedMediaFile updates the stored duration and size of a song that
// was just cut.
//
// Without this the library keeps the pre-trim length: the player would draw a
// progress bar longer than the audio, and every duration shown in the app would
// be wrong until someone happened to run a scan.
func (n *Router) refreshTrimmedMediaFile(ctx context.Context, mf *model.MediaFile,
	res silence.TrimResult) error {
	repo := n.ds.MediaFile(ctx)
	current, err := repo.Get(mf.ID)
	if err != nil {
		return err
	}
	current.Duration = float32(res.DurationAfter)
	current.Size = res.SizeAfter
	return repo.Put(current)
}
