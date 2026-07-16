package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/model"
)

const (
	lyricsResultAvailable = "available"
	lyricsResultFailed    = "failed"
)

type aiLyricsJobStartRequest struct {
	SongIDs []string `json:"songIds"`
}

type aiLyricsJobSongResult struct {
	SongID  string `json:"songId"`
	Success bool   `json:"success"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

type aiLyricsJobStatus struct {
	Running       bool                    `json:"running"`
	Status        string                  `json:"status"`
	SongIDs       []string                `json:"songIds,omitempty"`
	CurrentSongID string                  `json:"currentSongId,omitempty"`
	CurrentTitle  string                  `json:"currentTitle,omitempty"`
	Done          int                     `json:"done"`
	Total         int                     `json:"total"`
	StartedAt     *time.Time              `json:"startedAt,omitempty"`
	FinishedAt    *time.Time              `json:"finishedAt,omitempty"`
	Results       []aiLyricsJobSongResult `json:"results,omitempty"`
	Error         string                  `json:"error,omitempty"`
}

type lyricsFetchJob struct {
	mu                    sync.Mutex
	cancel                context.CancelFunc
	activeWhisperRequests int
	status                aiLyricsJobStatus
}

func newLyricsFetchJob() *lyricsFetchJob {
	return &lyricsFetchJob{status: aiLyricsJobStatus{Status: "idle"}}
}

func (j *lyricsFetchJob) Snapshot() aiLyricsJobStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	return cloneLyricsJobStatus(j.status)
}

func (j *lyricsFetchJob) Start(songIDs []string, repo model.MediaFileRepository) aiLyricsJobStatus {
	j.mu.Lock()
	if j.status.Running {
		defer j.mu.Unlock()
		return cloneLyricsJobStatus(j.status)
	}

	ctx, cancel := context.WithCancel(context.Background())
	startedAt := time.Now()
	j.cancel = cancel
	j.status = aiLyricsJobStatus{
		Running:   true,
		Status:    "running",
		SongIDs:   append([]string(nil), songIDs...),
		Total:     len(songIDs),
		StartedAt: &startedAt,
		Results:   []aiLyricsJobSongResult{},
	}
	j.mu.Unlock()

	go j.run(ctx, songIDs, repo)
	return j.Snapshot()
}

func (j *lyricsFetchJob) Stop() aiLyricsJobStatus {
	j.mu.Lock()
	if j.cancel != nil && j.status.Running {
		j.status.Status = "stopping"
		j.cancel()
	}
	defer j.mu.Unlock()
	return cloneLyricsJobStatus(j.status)
}

func (j *lyricsFetchJob) run(ctx context.Context, songIDs []string, repo model.MediaFileRepository) {
	failed := false
	for _, songID := range songIDs {
		if ctx.Err() != nil {
			j.finish("stopped")
			return
		}
		songID = strings.TrimSpace(songID)
		if songID == "" {
			failed = true
			j.recordResult(aiLyricsJobSongResult{Status: lyricsResultFailed, Error: "song id is empty"})
			continue
		}

		mf, err := repo.Get(songID)
		if err != nil {
			failed = true
			j.recordResult(aiLyricsJobSongResult{SongID: songID, Status: lyricsResultFailed, Error: "song not found"})
			continue
		}
		if text, _ := lyricsText(mf); strings.TrimSpace(text) != "" {
			j.recordResult(aiLyricsJobSongResult{SongID: songID, Success: true, Status: lyricsResultAvailable})
			continue
		}

		j.setCurrent(songID, mf.Title)
		_, err = fetchAndSaveWhisperLyrics(ctx, repo, mf)
		resultStatus := lyricsResultFailed
		if err == nil {
			resultStatus = lyricsResultAvailable
		}
		result := aiLyricsJobSongResult{
			SongID: songID, Success: err == nil,
			Status: resultStatus,
		}
		if err != nil {
			failed = true
			result.Error = err.Error()
		}
		j.recordResult(result)
		if ctx.Err() != nil {
			j.finish("stopped")
			return
		}
	}
	if failed {
		j.finish("failed")
		return
	}
	j.finish("complete")
}

func (j *lyricsFetchJob) setCurrent(songID, title string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status.CurrentSongID = songID
	j.status.CurrentTitle = title
}

func (j *lyricsFetchJob) recordResult(result aiLyricsJobSongResult) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status.Done++
	j.status.Results = append(j.status.Results, result)
	if result.Error != "" && j.status.Error == "" {
		j.status.Error = result.Error
	}
}

func (j *lyricsFetchJob) finish(status string) {
	finishedAt := time.Now()
	j.mu.Lock()
	defer j.mu.Unlock()
	j.status.Running = false
	j.status.Status = status
	j.status.FinishedAt = &finishedAt
	switch status {
	case "stopped":
		j.status.CurrentTitle = "Stopped"
	case "failed":
		j.status.CurrentTitle = "Failed"
	default:
		j.status.CurrentTitle = "Complete"
	}
	j.cancel = nil
}

func cloneLyricsJobStatus(status aiLyricsJobStatus) aiLyricsJobStatus {
	status.SongIDs = append([]string(nil), status.SongIDs...)
	status.Results = append([]aiLyricsJobSongResult(nil), status.Results...)
	return status
}

func (j *lyricsFetchJob) BeginWhisperRequest() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.activeWhisperRequests++
}

func (j *lyricsFetchJob) EndWhisperRequest() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.activeWhisperRequests > 0 {
		j.activeWhisperRequests--
	}
}

func (j *lyricsFetchJob) Busy() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.status.Running || j.activeWhisperRequests > 0
}

// fetchAndSaveWhisperLyrics sends the complete song to Whisper and writes
// lyrics only after Whisper has returned a successful whole-file response.
// Cancellation or any error returns before UpdateLyrics is called.
func fetchAndSaveWhisperLyrics(ctx context.Context, repo model.MediaFileRepository, mf *model.MediaFile) (aiLyricsResponse, error) {
	whisperURL := strings.TrimSpace(conf.Server.WhisperAPIURL)
	if whisperURL == "" {
		return aiLyricsResponse{}, fmt.Errorf("Whisper API URL is not configured")
	}

	songDuration := float64(mf.Duration)
	fetchCtx, cancel := context.WithTimeout(ctx, whisperTimeoutForDuration(songDuration))
	defer cancel()

	result, err := fetchWhisperLyrics(fetchCtx, whisperURL, mf.AbsolutePath(), selectedWhisperModel(), 0)
	if err != nil {
		return aiLyricsResponse{}, err
	}
	if err := fetchCtx.Err(); err != nil {
		return aiLyricsResponse{}, err
	}

	// verbose_json reports the duration of the audio Whisper processed. Reject a
	// response that ended materially before the stored song duration.
	processedDuration := result.Duration
	if processedDuration <= 0 {
		processedDuration = songDuration
	}
	if songDuration > 0 && processedDuration+2 < songDuration {
		return aiLyricsResponse{}, fmt.Errorf(
			"Whisper did not finish the complete song (processed %.1f of %.1f seconds)",
			processedDuration, songDuration,
		)
	}

	lyrics := aiLyricsResponse{
		Language: result.Language,
		Text:     result.Text,
		Status:   lyricsResultAvailable,
	}
	structured, err := model.ToLyrics(lyrics.Language, lyrics.Text)
	if err != nil {
		return aiLyricsResponse{}, fmt.Errorf("invalid lyrics response")
	}
	lyricsJSON, err := json.Marshal(model.LyricList{*structured})
	if err != nil {
		return aiLyricsResponse{}, fmt.Errorf("could not save lyrics")
	}
	if err := fetchCtx.Err(); err != nil {
		return aiLyricsResponse{}, err
	}

	if err := saveWhisperLyricsFile(conf.Server.WhisperLyricsFolder, mf.ID, lyrics.Text); err != nil {
		return aiLyricsResponse{}, err
	}
	if err := repo.UpdateLyrics(mf.ID, string(lyricsJSON)); err != nil {
		return aiLyricsResponse{}, fmt.Errorf("could not save lyrics")
	}
	return lyrics, nil
}

func (n *Router) lyricsJobManager() *lyricsFetchJob {
	if n.lyricsJob == nil {
		n.lyricsJob = newLyricsFetchJob()
	}
	return n.lyricsJob
}

func (n *Router) handleLyricsJobStart(w http.ResponseWriter, request *http.Request) {
	if n.ds == nil {
		writeAIChatError(w, http.StatusInternalServerError, "media repository is unavailable")
		return
	}
	var payload aiLyricsJobStartRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeAIChatError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	seen := map[string]struct{}{}
	songIDs := make([]string, 0, len(payload.SongIDs))
	for _, songID := range payload.SongIDs {
		songID = strings.TrimSpace(songID)
		if songID == "" {
			continue
		}
		if _, ok := seen[songID]; ok {
			continue
		}
		seen[songID] = struct{}{}
		songIDs = append(songIDs, songID)
	}
	if len(songIDs) == 0 {
		writeAIChatError(w, http.StatusBadRequest, "songIds is required")
		return
	}

	status := n.lyricsJobManager().Start(songIDs, n.ds.MediaFile(context.Background()))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func (n *Router) handleLyricsJobStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(n.lyricsJobManager().Snapshot())
}

func (n *Router) handleLyricsJobStop(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(n.lyricsJobManager().Stop())
}
