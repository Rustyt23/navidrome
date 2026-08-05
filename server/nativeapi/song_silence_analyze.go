package nativeapi

import (
	"context"
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

// The thresholds a run measures at, surfaced for the settings endpoint.
const (
	silenceThresholdDB      = ffmpeg.PrimaryThresholdDB
	silenceOnsetThresholdDB = ffmpeg.OnsetThresholdDB
)

func (n *Router) startSilenceAnalyze() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids := decodeSilenceSelection(r)
		filter := silenceAnalyzeFilter(ids)

		total, err := n.ds.MediaFile(r.Context()).CountAll(model.QueryOptions{Filters: filter})
		if err != nil {
			log.Warn(r.Context(), "Silence analysis: could not count tracks", err)
		}

		started := silenceAnalyze.begin(r.Context(), total, func(ctx context.Context) {
			n.runSilenceAnalyze(ctx, ids)
		})
		if !started {
			writeSilenceStatus(w, http.StatusConflict,
				silenceAnalyze.status("A silence job is already running"))
			return
		}
		msg := "Analysing the whole library for silence"
		if len(ids) > 0 {
			msg = "Analysing the songs you selected"
		}
		writeSilenceStatus(w, http.StatusAccepted, silenceAnalyze.status(msg))
	}
}

func (n *Router) silenceAnalyzeStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeSilenceStatus(w, http.StatusOK, silenceAnalyze.status(""))
	}
}

func (n *Router) stopSilenceAnalyzeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !silenceAnalyze.running.Load() {
			writeSilenceStatus(w, http.StatusOK, silenceAnalyze.status("No analysis is running"))
			return
		}
		silenceAnalyze.requestStop()
		writeSilenceStatus(w, http.StatusAccepted,
			silenceAnalyze.status("Stopping analysis after active songs finish"))
	}
}

// clearSilenceResults throws away every silence record.
//
// It touches no audio and no other feature's data. What it undoes is knowledge,
// not work: songs already trimmed stay trimmed, and forgetting that they were
// means the next analysis measures them again - correctly, since it measures
// what is on disk now.
func (n *Router) clearSilenceResults() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if silenceAnalyze.running.Load() || silenceTrim.running.Load() {
			writeSilenceStatus(w, http.StatusConflict,
				silenceAnalyze.status("Stop the running job before clearing results"))
			return
		}
		removed, err := n.ds.SilenceAudit(r.Context()).Clear()
		if err != nil {
			log.Error(r.Context(), "Could not clear silence analysis", err)
			http.Error(w, "Could not clear the silence analysis", http.StatusInternalServerError)
			return
		}
		log.Info(r.Context(), "Silence analysis cleared", "rows", removed)
		writeSilenceStatus(w, http.StatusOK, silenceAnalyze.status("Silence analysis cleared"))
	}
}

// silenceAnalyzeFilter selects what a run measures.
//
// A missing file cannot be measured or cut, so it is never listed - the same
// rule the LUFS pages settled on, for the same reason: listing them inflates
// every count on the page and queues work that can only fail.
func silenceAnalyzeFilter(ids []string) squirrel.Sqlizer {
	notMissing := squirrel.Eq{"media_file.missing": false}
	if len(ids) > 0 {
		return squirrel.And{notMissing, squirrel.Eq{"media_file.id": ids}}
	}
	return notMissing
}

func (n *Router) runSilenceAnalyze(ctx context.Context, ids []string) {
	defer silenceAnalyze.finish()

	log.Info(ctx, "Silence analysis started", "selected", len(ids))
	cursor, err := n.ds.MediaFile(ctx).GetCursor(
		model.QueryOptions{Filters: silenceAnalyzeFilter(ids)})
	if err != nil {
		log.Error(ctx, "Silence analysis: could not read media files", err)
		return
	}

	workers := runtime.NumCPU()
	work := make(chan model.MediaFile)

	// Album grouping is collected as the run goes and judged at the end. It
	// cannot be decided per track: whether an album is continuous is a fact
	// about the whole record, and no single track can see it.
	var seamMu sync.Mutex
	seams := map[string][]silence.TrackSeam{}

	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			detector := ffmpeg.NewSilenceDetector()
			measurer := ffmpeg.NewPeakMeasurer()
			for mf := range work {
				silenceAnalyze.inFlight.Add(1)
				audit, err := silence.Analyze(ctx, detector, measurer, &mf, false, silence.Options{})
				silenceAnalyze.inFlight.Add(-1)

				switch {
				case err != nil && ctx.Err() != nil:
					// Abandoned when the grace period ran out. Nothing was
					// written, so the next run picks it up: postponed, not lost.
					silenceAnalyze.cancelled.Add(1)
					continue
				case err != nil:
					silenceAnalyze.failed.Add(1)
					log.Warn(ctx, "Silence analysis: could not measure track", "path", mf.Path, err)
				case audit.Verdict == model.SilenceVerdictTrimmable:
					silenceAnalyze.changed.Add(1)
					silenceAnalyze.addRemoved(audit.TotalTrim())
				default:
					silenceAnalyze.skipped.Add(1)
				}

				if audit != nil {
					if err := n.ds.SilenceAudit(ctx).Put(audit); err != nil {
						log.Error(ctx, "Could not save the silence record", "path", mf.Path, err)
					}
					if mf.AlbumID != "" && audit.Status != model.SilenceStatusFailed {
						seamMu.Lock()
						seams[mf.AlbumID] = append(seams[mf.AlbumID], silence.TrackSeam{
							MediaFileID: mf.ID,
							DiscNumber:  mf.DiscNumber,
							TrackNumber: mf.TrackNumber,
							LeadSilence: audit.LeadSilence,
							TailSilence: audit.TrailSilence,
						})
						seamMu.Unlock()
					}
				}
				silenceAnalyze.processed.Add(1)
			}
		}()
	}

	stop := silenceAnalyze.stopSignal()
dispatch:
	for mf, err := range cursor {
		if err != nil {
			log.Error(ctx, "Silence analysis aborted: error reading media files", err)
			break
		}
		if silenceAnalyze.stopping.Load() {
			log.Info(ctx, "Silence analysis stopped on request",
				"processed", silenceAnalyze.processed.Load())
			break
		}
		// Both are needed: the stored path is library-relative, so without the
		// library path there is nothing to resolve it against and the file
		// cannot be found.
		if strings.TrimSpace(mf.Path) == "" || strings.TrimSpace(mf.LibraryPath) == "" {
			// Counted as processed as well as skipped: the total came from the
			// same query, so dropping one without counting it leaves the
			// progress bar permanently short of its total.
			silenceAnalyze.skipped.Add(1)
			silenceAnalyze.processed.Add(1)
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

	n.markGaplessAlbums(ctx, seams)

	log.Info(ctx, "Silence analysis finished",
		"processed", silenceAnalyze.processed.Load(),
		"trimmable", silenceAnalyze.changed.Load(),
		"skipped", silenceAnalyze.skipped.Load(),
		"failed", silenceAnalyze.failed.Load(),
		"cancelled", silenceAnalyze.cancelled.Load())
}

// markGaplessAlbums is the second look that per-track analysis cannot take.
//
// An album whose tracks run into one another has its planned trims withdrawn
// and recorded as skipped-for-gapless, so the page explains why those songs are
// not on the trimmable list rather than silently omitting them.
func (n *Router) markGaplessAlbums(ctx context.Context, seams map[string][]silence.TrackSeam) {
	repo := n.ds.SilenceAudit(ctx)
	for albumID, tracks := range seams {
		if !silence.DetectGaplessAlbum(tracks) {
			continue
		}
		log.Info(ctx, "Silence analysis: album plays continuously, holding its songs back",
			"albumId", albumID, "tracks", len(tracks))
		for _, track := range tracks {
			audit, err := repo.Get(track.MediaFileID)
			if err != nil || audit == nil {
				continue
			}
			wasTrimmable := audit.Verdict == model.SilenceVerdictTrimmable
			silence.ApplyGaplessVerdict([]*model.SilenceAudit{audit}, true)
			if err := repo.Put(audit); err != nil {
				log.Error(ctx, "Could not mark a song as part of a continuous album",
					"id", track.MediaFileID, err)
				continue
			}
			// The headline counts were tallied before the album was judged, so
			// a track moved out of "trimmable" is moved in the counters too.
			if wasTrimmable {
				silenceAnalyze.changed.Add(-1)
				silenceAnalyze.skipped.Add(1)
			}
		}
	}
}
