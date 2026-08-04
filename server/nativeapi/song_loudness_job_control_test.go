package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/navidrome/navidrome/core/loudness"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LUFS job controls", func() {
	resetJobs := func() {
		finishLoudnessAnalyze()
		loudnessAnalyze.startedAt.Store(0)
		loudnessAnalyze.total.Store(0)
		loudnessAnalyze.processed.Store(0)
		loudnessAnalyze.failed.Store(0)
		loudnessAnalyze.inFlight.Store(0)
		loudnessAnalyze.cancelled.Store(0)

		finishLibraryLoudness()
		libraryLoudness.phase.Store(0)
		libraryLoudness.startedAt.Store(0)
		libraryLoudness.total.Store(0)
		libraryLoudness.processed.Store(0)
		libraryLoudness.normalized.Store(0)
		libraryLoudness.skipped.Store(0)
		libraryLoudness.failed.Store(0)
		libraryLoudness.inFlight.Store(0)
		libraryLoudness.cancelled.Store(0)

		restoreLoudnessRunning.Store(false)
	}

	BeforeEach(resetJobs)
	AfterEach(resetJobs)

	// Puts a job into the state a real run would be in, without starting one.
	armLibraryRun := func() context.Context {
		ctx, cancel := context.WithCancel(context.Background())
		libraryLoudness.stopMu.Lock()
		libraryLoudness.running.Store(true)
		libraryLoudness.stop = make(chan struct{})
		libraryLoudness.cancel = cancel
		libraryLoudness.stopMu.Unlock()
		return ctx
	}

	Describe("stopping a run", func() {
		BeforeEach(func() {
			original := stopGracePeriod
			stopGracePeriod = 10 * time.Millisecond
			DeferCleanup(func() { stopGracePeriod = original })
		})

		It("kills the tracks still running once the grace period is up", func() {
			ctx := armLibraryRun()

			stopLibraryLoudness()

			// Before the grace period the tracks in flight are left alone.
			Expect(ctx.Err()).ToNot(HaveOccurred())
			Eventually(ctx.Done()).Should(BeClosed())
			Expect(ctx.Err()).To(MatchError(context.Canceled))
		})

		It("does the same for an analysis sweep", func() {
			ctx, ok := beginLoudnessAnalyze(context.Background())
			Expect(ok).To(BeTrue())

			stopLoudnessAnalyze()

			Eventually(ctx.Done()).Should(BeClosed())
		})

		It("releases the run context when a run ends on its own", func() {
			ctx := armLibraryRun()

			finishLibraryLoudness()

			Expect(ctx.Err()).To(MatchError(context.Canceled))
			Expect(currentLibraryLoudnessStatus("").Running).To(BeFalse())
		})

		It("reports how many tracks are still open, so a stop can be seen working", func() {
			armLibraryRun()
			libraryLoudness.inFlight.Store(3)

			Expect(currentLibraryLoudnessStatus("").InFlight).To(Equal(int64(3)))
		})
	})

	It("requests a cooperative stop for library analysis", func() {
		_, ok := beginLoudnessAnalyze(GinkgoT().Context())
		Expect(ok).To(BeTrue())
		stopSignal := loudnessAnalyzeStopSignal()
		handler := (&Router{}).stopLoudnessAnalyzeHandler()
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/song/loudness/analyze/stop", nil))

		Expect(response.Code).To(Equal(http.StatusAccepted))
		Expect(currentLoudnessAnalyzeStatus("").Stopping).To(BeTrue())
		select {
		case <-stopSignal:
		default:
			Fail("analysis stop signal was not closed")
		}
	})

	It("requests a cooperative stop for library optimisation", func() {
		libraryLoudness.stopMu.Lock()
		libraryLoudness.running.Store(true)
		libraryLoudness.stop = make(chan struct{})
		stopSignal := libraryLoudness.stop
		libraryLoudness.stopMu.Unlock()

		handler := (&Router{}).stopLibraryLoudnessHandler()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/song/loudness/library/stop", nil))

		Expect(response.Code).To(Equal(http.StatusAccepted))
		Expect(currentLibraryLoudnessStatus("").Stopping).To(BeTrue())
		select {
		case <-stopSignal:
		default:
			Fail("optimisation stop signal was not closed")
		}
	})

	It("clears derived audit data without deleting media records", func() {
		ds := &tests.MockDataStore{
			MockedMediaFile: tests.CreateMockMediaFileRepo(),
		}
		repo := ds.LoudnessAudit(GinkgoT().Context())
		Expect(repo.Put(&model.LoudnessAudit{MediaFileID: "song-1"})).To(Succeed())
		Expect(repo.Put(&model.LoudnessAudit{MediaFileID: "song-2"})).To(Succeed())

		handler := (&Router{ds: ds}).clearLoudnessAnalyzeResults()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/song/loudness/analyze/results", nil))

		Expect(response.Code).To(Equal(http.StatusOK))
		var body map[string]any
		Expect(json.Unmarshal(response.Body.Bytes(), &body)).To(Succeed())
		Expect(body["cleared"]).To(BeNumerically("==", 2))
		_, err := repo.Get("song-1")
		Expect(err).To(MatchError(model.ErrNotFound))
		Expect(ds.MediaFile(GinkgoT().Context())).ToNot(BeNil())
	})

	It("refuses to clear while a LUFS job is active", func() {
		ds := &tests.MockDataStore{}
		repo := ds.LoudnessAudit(GinkgoT().Context())
		Expect(repo.Put(&model.LoudnessAudit{MediaFileID: "song-1"})).To(Succeed())
		_, ok := beginLoudnessAnalyze(GinkgoT().Context())
		Expect(ok).To(BeTrue())

		handler := (&Router{ds: ds}).clearLoudnessAnalyzeResults()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/song/loudness/analyze/results", nil))

		Expect(response.Code).To(Equal(http.StatusConflict))
		audit, err := repo.Get("song-1")
		Expect(err).ToNot(HaveOccurred())
		Expect(audit).ToNot(BeNil())
	})

	// Whether originals are kept decides what happens to files the moment the
	// next track is opened, and cannot be applied to work already done. Changing
	// it mid-run would leave one half of a run restorable and the other half
	// not, with nothing in the record saying where the line falls.
	It("refuses to change whether originals are kept while a run is going", func() {
		ds := &tests.MockDataStore{}
		armLibraryRun()

		handler := (&Router{ds: ds}).updateLoudnessSettings()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut,
			"/song/loudness/settings", strings.NewReader(`{"backup":false}`)))

		Expect(response.Code).To(Equal(http.StatusConflict))
	})

	// The switch shows a run, not a preference, and a run turns it off when it
	// ends. Saving "on" for a run that never started left it claiming one for
	// ever, with nothing able to clear it.
	It("does not leave 'Optimise all' on when the run could not start", func() {
		ds := &tests.MockDataStore{}
		loudness.ResetCache()
		DeferCleanup(loudness.ResetCache)
		release, busy := claimLoudnessFileWork(&restoreLoudnessRunning)
		Expect(busy).To(BeEmpty())
		DeferCleanup(release)

		handler := (&Router{ds: ds}).updateLoudnessSettings()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut,
			"/song/loudness/settings", strings.NewReader(`{"enabled":true}`)))

		Expect(response.Code).To(Equal(http.StatusConflict))
		Expect(response.Body.String()).To(ContainSubstring("a restore"))
		Expect(loudness.Enabled(GinkgoT().Context(), ds)).To(BeFalse())
	})

	It("rejects a settings request that sets nothing", func() {
		handler := (&Router{ds: &tests.MockDataStore{}}).updateLoudnessSettings()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPut,
			"/song/loudness/settings", strings.NewReader(`{}`)))

		Expect(response.Code).To(Equal(http.StatusBadRequest))
	})

	It("does not start analysis and optimisation at the same time", func() {
		_, ok := beginLoudnessAnalyze(GinkgoT().Context())
		Expect(ok).To(BeTrue())
		Expect((&Router{}).beginLibraryLoudness(GinkgoT().Context(), 1, nil)).To(BeFalse())
	})

	// Analysis never writes to a library file, so it looks harmless next to a
	// restore. It is not: it measures the file, then writes the record saying
	// what that file is. Let a restore swap the file in between and whichever
	// writes last wins, leaving the record describing audio that is no longer
	// there - with nothing on screen to say so.
	Describe("analysis and restore", func() {
		It("does not start an analysis while a restore is running", func() {
			release, busy := claimLoudnessFileWork(&restoreLoudnessRunning)
			Expect(busy).To(BeEmpty())
			DeferCleanup(release)

			_, ok := beginLoudnessAnalyze(GinkgoT().Context())
			Expect(ok).To(BeFalse())
		})

		It("does not start a restore while an analysis is running", func() {
			_, ok := beginLoudnessAnalyze(GinkgoT().Context())
			Expect(ok).To(BeTrue())

			release, busy := claimLoudnessFileWork(&restoreLoudnessRunning)
			Expect(release).To(BeNil())
			Expect(busy).To(Equal("a LUFS analysis"))
		})

		It("tells the caller which job is holding the library", func() {
			release, busy := claimLoudnessFileWork(&restoreLoudnessRunning)
			Expect(busy).To(BeEmpty())
			DeferCleanup(release)

			handler := (&Router{}).startLoudnessAnalyze()
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost,
				"/song/loudness/analyze", strings.NewReader(`{"all":true}`)))

			Expect(response.Code).To(Equal(http.StatusConflict))
			Expect(response.Body.String()).To(ContainSubstring("a restore"))
		})

		// Clearing the audit rows while a restore is writing one deletes the
		// record of work that just happened.
		It("refuses to clear analysis data while a restore is running", func() {
			ds := &tests.MockDataStore{}
			repo := ds.LoudnessAudit(GinkgoT().Context())
			Expect(repo.Put(&model.LoudnessAudit{MediaFileID: "song-1"})).To(Succeed())
			release, busy := claimLoudnessFileWork(&restoreLoudnessRunning)
			Expect(busy).To(BeEmpty())
			DeferCleanup(release)

			handler := (&Router{ds: ds}).clearLoudnessAnalyzeResults()
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodDelete,
				"/song/loudness/analyze/results", nil))

			Expect(response.Code).To(Equal(http.StatusConflict))
			audit, err := repo.Get("song-1")
			Expect(err).ToNot(HaveOccurred())
			Expect(audit).ToNot(BeNil())
		})
	})
})
