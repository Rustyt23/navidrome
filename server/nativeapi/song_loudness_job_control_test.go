package nativeapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

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

		finishLibraryLoudness()
		libraryLoudness.phase.Store(0)
		libraryLoudness.startedAt.Store(0)
		libraryLoudness.total.Store(0)
		libraryLoudness.processed.Store(0)
		libraryLoudness.normalized.Store(0)
		libraryLoudness.skipped.Store(0)
		libraryLoudness.failed.Store(0)
	}

	BeforeEach(resetJobs)
	AfterEach(resetJobs)

	It("requests a cooperative stop for library analysis", func() {
		Expect(beginLoudnessAnalyze()).To(BeTrue())
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
		Expect(beginLoudnessAnalyze()).To(BeTrue())

		handler := (&Router{ds: ds}).clearLoudnessAnalyzeResults()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodDelete, "/song/loudness/analyze/results", nil))

		Expect(response.Code).To(Equal(http.StatusConflict))
		audit, err := repo.Get("song-1")
		Expect(err).ToNot(HaveOccurred())
		Expect(audit).ToNot(BeNil())
	})

	It("does not start analysis and optimisation at the same time", func() {
		Expect(beginLoudnessAnalyze()).To(BeTrue())
		Expect((&Router{}).beginLibraryLoudness(GinkgoT().Context(), 1)).To(BeFalse())
	})
})
