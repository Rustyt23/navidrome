package nativeapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

func TestRAGPlaylistAnalysisEndpoint(t *testing.T) {
	playlist := &model.Playlist{
		ID: "playlist-1", Name: "Read Only Mix",
		Tracks: model.PlaylistTracks{{MediaFile: model.MediaFile{ID: "song-1", Title: "Song", Artist: "Artist"}}},
	}
	request := httptest.NewRequest(http.MethodPost, "/api/ai/rag/playlist/analyze", bytes.NewBufferString(`{"playlistId":"playlist-1"}`))
	recorder := httptest.NewRecorder()
	serveRAGPlaylistAnalysis(
		recorder,
		request,
		func(_ context.Context, id string) (*model.Playlist, error) {
			if id != playlist.ID {
				t.Fatalf("unexpected playlist ID: %s", id)
			}
			return playlist, nil
		},
		func(context.Context, string, int, rag.SearchFilters) ([]rag.SongSearchResult, error) { return nil, nil },
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response rag.PlaylistAnalysis
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.PlaylistID != playlist.ID || response.Name != playlist.Name || response.SongCount != 1 || len(response.MetadataIssues) != 1 {
		t.Fatalf("unexpected analysis response: %+v", response)
	}
}

func TestRAGPlaylistAnalysisRequiresID(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/ai/rag/playlist/analyze", bytes.NewBufferString(`{}`))
	recorder := httptest.NewRecorder()
	serveRAGPlaylistAnalysis(recorder, request, func(context.Context, string) (*model.Playlist, error) {
		t.Fatal("loader should not be called")
		return nil, nil
	}, nil)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}
