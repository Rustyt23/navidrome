package nativeapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/silencetrim"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("start/end silence trim controls", func() {
	BeforeEach(func() {
		silenceAnalyzeRunning.Store(false)
		silenceApplyRunning.Store(false)
		silenceRestoreRunning.Store(false)
		silenceDecisionRunning.Store(false)
		silenceClearRunning.Store(false)
	})

	It("clears dry-run rows but preserves applied restore provenance", func() {
		ds := &tests.MockDataStore{}
		repo := ds.SilenceTrimAudit(GinkgoT().Context())
		Expect(repo.Put(&model.SilenceTrimAudit{
			MediaFileID: "dry-run",
			Status:      model.SilenceTrimStatusAnalyzed,
		})).To(Succeed())
		Expect(repo.Put(&model.SilenceTrimAudit{
			MediaFileID:  "processed",
			Status:       model.SilenceTrimStatusProcessed,
			HasBackup:    true,
			SourceSHA256: "source",
			ResultSHA256: "result",
		})).To(Succeed())

		response := httptest.NewRecorder()
		(&Router{ds: ds}).clearSilenceTrimAnalysis().ServeHTTP(
			response,
			httptest.NewRequest(http.MethodDelete, "/song/silence-trim/analyze/results", nil),
		)

		Expect(response.Code).To(Equal(http.StatusOK))
		var body map[string]any
		Expect(json.Unmarshal(response.Body.Bytes(), &body)).To(Succeed())
		Expect(body["cleared"]).To(BeNumerically("==", 1))
		_, err := repo.Get("dry-run")
		Expect(err).To(MatchError(model.ErrNotFound))
		proof, err := repo.Get("processed")
		Expect(err).ToNot(HaveOccurred())
		Expect(proof.ResultSHA256).To(Equal("result"))
	})

	It("requires an explicit library scope for analyze all", func() {
		response := httptest.NewRecorder()
		(&Router{}).startSilenceTrimAnalyze().ServeHTTP(
			response,
			httptest.NewRequest(
				http.MethodPost,
				"/song/silence-trim/analyze",
				strings.NewReader(`{"all":true}`),
			),
		)
		Expect(response.Code).To(Equal(http.StatusBadRequest))
	})

	It("requires an explicit library scope for apply all", func() {
		response := httptest.NewRecorder()
		(&Router{}).startSilenceTrimApply().ServeHTTP(
			response,
			httptest.NewRequest(
				http.MethodPost,
				"/song/silence-trim/apply",
				strings.NewReader(`{"all":true}`),
			),
		)
		Expect(response.Code).To(Equal(http.StatusBadRequest))
	})

	It("blocks LUFS from the durable result even before its audit is saved", func() {
		library := GinkgoT().TempDir()
		track := filepath.Join(library, "song.mp3")
		Expect(os.WriteFile(track, []byte("source generation"), 0o600)).To(Succeed())
		sourceHash, err := ffmpeg.BackupSilenceTrimOriginal(
			track,
			0o600,
			library,
			"",
			"journal-guard",
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(os.WriteFile(track, []byte("verified result generation"), 0o600)).To(Succeed())
		resultHash, err := ffmpeg.FileSHA256(track)
		Expect(err).ToNot(HaveOccurred())
		audit := &model.SilenceTrimAudit{
			MediaFileID:  "journal-guard",
			Status:       model.SilenceTrimStatusProcessed,
			Integrity:    model.SilenceTrimIntegrityVerified,
			HasBackup:    true,
			SourceSHA256: sourceHash,
			BackupSHA256: sourceHash,
			ResultSHA256: resultHash,
		}
		Expect(silencetrim.WriteGenerationRecord("", library, audit)).To(Succeed())

		mf := &model.MediaFile{
			ID:               "journal-guard",
			LibraryPath:      library,
			Path:             "song.mp3",
			SilenceTrimAudit: &model.SilenceTrimAudit{MediaFileID: "journal-guard"},
		}
		Expect(silenceTrimBlocksLoudness(mf, track, "")).To(BeTrue())
	})

	It("fails LUFS closed for unreadable uncommitted trim provenance", func() {
		library := GinkgoT().TempDir()
		track := filepath.Join(library, "song.mp3")
		Expect(os.WriteFile(track, []byte("song bytes"), 0o600)).To(Succeed())
		recordPath := silencetrim.GenerationRecordPath("", library, "broken-journal")
		Expect(os.MkdirAll(filepath.Dir(recordPath), 0o755)).To(Succeed())
		Expect(os.WriteFile(recordPath, []byte("{broken"), 0o600)).To(Succeed())

		mf := &model.MediaFile{
			ID:               "broken-journal",
			LibraryPath:      library,
			Path:             "song.mp3",
			SilenceTrimAudit: &model.SilenceTrimAudit{MediaFileID: "broken-journal"},
		}
		Expect(silenceTrimBlocksLoudness(mf, track, "")).To(BeTrue())

		sourceHash, err := ffmpeg.FileSHA256(track)
		Expect(err).ToNot(HaveOccurred())
		mf.SilenceTrimAudit.HasBackup = true
		mf.SilenceTrimAudit.BackupSHA256 = sourceHash
		mf.SilenceTrimAudit.SourceSHA256 = sourceHash
		mf.SilenceTrimAudit.ResultSHA256 = ""
		Expect(silenceTrimBlocksLoudness(mf, track, "")).To(BeFalse())

		Expect(os.WriteFile(track, []byte("uncommitted newer bytes"), 0o600)).To(Succeed())
		Expect(silenceTrimBlocksLoudness(mf, track, "")).To(BeTrue())
	})

	It("never rolls a restore over unrecognized newer bytes", func() {
		dir := GinkgoT().TempDir()
		track := filepath.Join(dir, "song.mp3")
		snapshot := filepath.Join(dir, "verified-result.mp3")
		source := filepath.Join(dir, "verified-source.mp3")
		Expect(os.WriteFile(snapshot, []byte("verified result"), 0o600)).To(Succeed())
		Expect(os.WriteFile(source, []byte("verified source"), 0o600)).To(Succeed())
		Expect(os.WriteFile(track, []byte("newer external bytes"), 0o600)).To(Succeed())
		snapshotHash, err := ffmpeg.FileSHA256(snapshot)
		Expect(err).ToNot(HaveOccurred())
		sourceHash, err := ffmpeg.FileSHA256(source)
		Expect(err).ToNot(HaveOccurred())

		rolledBack, err := restoreSilenceSnapshotIfCurrent(
			track,
			snapshot,
			0o600,
			sourceHash,
			snapshotHash,
		)
		Expect(err).To(HaveOccurred())
		Expect(rolledBack).To(BeFalse())
		bytes, readErr := os.ReadFile(track)
		Expect(readErr).ToNot(HaveOccurred())
		Expect(string(bytes)).To(Equal("newer external bytes"))

		Expect(os.WriteFile(track, []byte("verified source"), 0o600)).To(Succeed())
		rolledBack, err = restoreSilenceSnapshotIfCurrent(
			track,
			snapshot,
			0o600,
			sourceHash,
			snapshotHash,
		)
		Expect(err).ToNot(HaveOccurred())
		Expect(rolledBack).To(BeTrue())
		bytes, readErr = os.ReadFile(track)
		Expect(readErr).ToNot(HaveOccurred())
		Expect(string(bytes)).To(Equal("verified result"))
	})
})
