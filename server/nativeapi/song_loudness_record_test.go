package nativeapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/core/loudness"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
)

func TestOptimizedAudioReportsSaveFailureAndCanBeReanalysed(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is not installed")
	}
	originalOptions := conf.Server.Scanner.LoudnessNormalization
	t.Cleanup(func() { conf.Server.Scanner.LoudnessNormalization = originalOptions; loudness.ResetCache() })
	loudness.ResetCache()
	options := &conf.Server.Scanner.LoudnessNormalization
	options.TargetLUFS, options.TruePeak, options.Tolerance, options.LRA = -12.6, -0.5, 0.2, 11
	options.BackupFolder = t.TempDir()
	library := t.TempDir()
	track := filepath.Join(library, "song.wav")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=2", "-c:a", "pcm_s16le", track).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	originalBytes, err := os.ReadFile(track)
	if err != nil {
		t.Fatal(err)
	}
	ds := &recordTestStore{DataStore: &tests.MockDataStore{}, failures: 3}
	n := ffmpeg.NewLoudnessNormalizer()
	res, err := optimizeOneTrack(context.Background(), ds, n, &model.MediaFile{ID: "song-1", LibraryPath: library, Path: track})
	if !res.Changed || err == nil || !strings.Contains(err.Error(), "could not save LUFS records") {
		t.Fatalf("changed=%v error=%v rejection=%s", res.Changed, err, res.Rejected)
	}
	currentBytes, err := os.ReadFile(track)
	if err != nil {
		t.Fatal(err)
	}
	if string(currentBytes) == string(originalBytes) {
		t.Fatal("fixture did not exercise a changed audio file")
	}
	// Once the database is available, re-analysis rebuilds the current record
	// from disk without applying another gain or limiter pass.
	target := ffmpeg.LoudnessTarget{IntegratedLUFS: options.TargetLUFS, TruePeak: options.TruePeak, LRA: options.LRA}
	audit := loudness.Audit(context.Background(), n, "song-1", library, track, target, options.Tolerance, options.BackupFolder)
	err = saveLoudnessRecord(context.Background(), ds, "song-1", func(ctx context.Context, tx model.DataStore) error { return tx.LoudnessAudit(ctx).Put(audit) })
	if err != nil {
		t.Fatal(err)
	}
	saved, err := ds.LoudnessAudit(context.Background()).Get("song-1")
	if err != nil {
		t.Fatal(err)
	}
	currentLUFS := saved.LufsAfter
	if currentLUFS == nil {
		currentLUFS = saved.LufsBefore
	}
	if currentLUFS == nil || *currentLUFS != res.NewLUFS {
		t.Fatalf("recovered record does not describe current audio: %+v", saved)
	}
	unchanged, err := os.ReadFile(track)
	if err != nil || string(unchanged) != string(currentBytes) {
		t.Fatal("audit recovery rewrote audio")
	}
}

type recordTestStore struct {
	model.DataStore
	attempts int
	failures int
}

func (s *recordTestStore) WithTx(write func(model.DataStore) error, _ ...string) error {
	s.attempts++
	if s.attempts <= s.failures {
		return errors.New("database locked")
	}
	return write(s.DataStore)
}

func TestLoudnessRecordRetriesWithoutReprocessing(t *testing.T) {
	ds := &recordTestStore{DataStore: &tests.MockDataStore{}, failures: 2}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := saveLoudnessRecord(ctx, ds, "song-1", func(ctx context.Context, tx model.DataStore) error {
		if ctx.Err() != nil {
			t.Fatal("audio cancellation must not abandon its record")
		}
		return tx.LoudnessAudit(ctx).Put(&model.LoudnessAudit{MediaFileID: "song-1"})
	})
	if err != nil || ds.attempts != 3 {
		t.Fatalf("attempts=%d error=%v", ds.attempts, err)
	}
	if _, err := ds.LoudnessAudit(context.Background()).Get("song-1"); err != nil {
		t.Fatal(err)
	}
}

func TestLoudnessRecordFailureReportsRecoveryAndDoesNotClaimSuccess(t *testing.T) {
	ds := &recordTestStore{DataStore: &tests.MockDataStore{}, failures: 3}
	err := saveLoudnessRecord(context.Background(), ds, "song-1", func(context.Context, model.DataStore) error {
		t.Fatal("unavailable database should not run the transaction")
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "Re-analyse") || !strings.Contains(err.Error(), "song-1") {
		t.Fatalf("missing recovery details: %v", err)
	}
	libraryLoudness.lastError.set(err.Error())
	defer libraryLoudness.lastError.set("")
	status := currentLibraryLoudnessStatus("")
	if status.Error != err.Error() {
		t.Fatal("status hid the save failure")
	}
}

// Exercise the guard while a real restore handler is inside the repository,
// rather than just checking flags before and after an operation.
type blockedAuditRestore struct {
	model.LoudnessAuditRepository
	entered chan struct{}
	resume  chan struct{}
}

func (r *blockedAuditRestore) RestoreSnapshot(string, string) (*model.LoudnessRestoreReport, error) {
	close(r.entered)
	<-r.resume
	return &model.LoudnessRestoreReport{}, nil
}

type auditRestoreTestStore struct {
	model.DataStore
	repo model.LoudnessAuditRepository
}

func (s *auditRestoreTestStore) LoudnessAudit(context.Context) model.LoudnessAuditRepository {
	return s.repo
}

func TestAuditRestoreReservesAccessUntilItFinishes(t *testing.T) {
	repo := &blockedAuditRestore{entered: make(chan struct{}), resume: make(chan struct{})}
	ds := &auditRestoreTestStore{DataStore: &tests.MockDataStore{}, repo: repo}
	router := &Router{ds: ds}
	done := make(chan struct{})
	response := httptest.NewRecorder()
	go func() {
		defer close(done)
		router.loudnessAuditDbRestore().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"file":"saved.db"}`)))
	}()
	<-repo.entered
	defer func() { close(repo.resume); <-done }()
	if (&Router{}).beginLibraryLoudness(context.Background(), 1, nil) {
		t.Fatal("processing overlapped audit restoration")
	}
	if _, ok := beginLoudnessAnalyze(context.Background()); ok {
		t.Fatal("analysis overlapped audit restoration")
	}
	if _, ok := beginRestoreLoudness(context.Background(), 1); ok {
		t.Fatal("audio restoration overlapped audit restoration")
	}
	for _, request := range []struct {
		handler      http.HandlerFunc
		method, body string
	}{
		{router.clearLoudnessAnalyzeResults(), http.MethodDelete, ""},
		{router.loudnessAuditDbRestore(), http.MethodPost, `{"file":"saved.db"}`},
		{router.setLoudnessDecision(), http.MethodPut, `{"ids":["song-1"],"decision":"skip"}`},
	} {
		w := httptest.NewRecorder()
		request.handler.ServeHTTP(w, httptest.NewRequest(request.method, "/", strings.NewReader(request.body)))
		if w.Code != http.StatusConflict {
			t.Fatalf("conflicting operation returned %d", w.Code)
		}
	}
}
