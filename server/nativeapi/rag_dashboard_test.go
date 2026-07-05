package nativeapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

func TestRAGDashboardReportEndpointStructure(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/ai/rag/reports/dashboard", nil)
	recorder := httptest.NewRecorder()
	serveRAGDashboardReport(recorder, request, func(context.Context) (rag.DashboardReport, error) {
		return rag.DashboardReport{
			LibraryHealthScore: 84, TotalSongs: 100, IndexedSongs: 80,
			IndexedCoveragePercent: 80, PlaylistsTotal: 4, PlaylistsAnalyzed: 4,
		}, nil
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	for _, key := range []string{
		"libraryHealthScore", "totalSongs", "indexedSongs", "indexedCoveragePercent",
		"songsWithLyrics", "songsMissingLyrics", "songsWithGenre", "songsMissingGenre",
		"songsWithYear", "songsMissingYear", "songsWithBpm", "songsMissingBpm",
		"songsWithLufs", "songsMissingLufs", "cleanSongs", "explicitSongs",
		"reviewNeededSongs", "explicitRiskSongs", "duplicateRiskCount", "loudnessIssueCount",
		"overplayedSongs", "underusedSongs", "playlistsTotal", "playlistsAnalyzed",
		"playlistsWithIssues", "recommendationsAvailable",
	} {
		if _, ok := payload[key]; !ok {
			t.Errorf("dashboard response missing %q", key)
		}
	}
}
