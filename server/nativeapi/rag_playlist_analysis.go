package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

type ragPlaylistAnalyzeRequest struct {
	PlaylistID string `json:"playlistId"`
}

type playlistAnalysisLoader func(context.Context, string) (*model.Playlist, error)

func (n *Router) handleRAGPlaylistAnalyze(w http.ResponseWriter, request *http.Request) {
	if n.ds == nil {
		writeRAGPlaylistAnalysisError(w, http.StatusInternalServerError, "playlist repository is unavailable")
		return
	}
	serveRAGPlaylistAnalysis(
		w,
		request,
		func(ctx context.Context, playlistID string) (*model.Playlist, error) {
			// Both flags are false so analysis cannot refresh a smart playlist or
			// include unresolved/missing filesystem entries.
			return n.ds.Playlist(ctx).GetWithTracks(playlistID, false, false)
		},
		searchRAG,
	)
}

func serveRAGPlaylistAnalysis(
	w http.ResponseWriter,
	request *http.Request,
	load playlistAnalysisLoader,
	search rag.ReplacementSearchFunc,
) {
	var payload ragPlaylistAnalyzeRequest
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeRAGPlaylistAnalysisError(w, http.StatusBadRequest, fmt.Sprintf("invalid request payload: %v", err))
		return
	}
	payload.PlaylistID = strings.TrimSpace(payload.PlaylistID)
	if payload.PlaylistID == "" {
		writeRAGPlaylistAnalysisError(w, http.StatusBadRequest, "playlistId is required")
		return
	}

	playlist, err := load(request.Context(), payload.PlaylistID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, model.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeRAGPlaylistAnalysisError(w, status, err.Error())
		return
	}
	analysis := rag.AnalyzePlaylist(request.Context(), playlist, search)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(analysis)
}

func writeRAGPlaylistAnalysisError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
