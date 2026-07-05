package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

type ragRecommendationRequest struct {
	Type       rag.RecommendationType `json:"type"`
	SongID     string                 `json:"songId,omitempty"`
	PlaylistID string                 `json:"playlistId,omitempty"`
	Limit      int                    `json:"limit"`
}

type recommendationSongLoader func(context.Context, string) (*model.MediaFile, error)
type recommendationPlaylistLoader func(context.Context, string) (*model.Playlist, error)

func (n *Router) handleRAGRecommendation(w http.ResponseWriter, request *http.Request) {
	if n.ds == nil {
		writeRAGRecommendationError(w, http.StatusInternalServerError, "library repository is unavailable")
		return
	}
	serveRAGRecommendation(
		w,
		request,
		func(ctx context.Context, songID string) (*model.MediaFile, error) {
			return n.ds.MediaFile(ctx).Get(songID)
		},
		func(ctx context.Context, playlistID string) (*model.Playlist, error) {
			return n.ds.Playlist(ctx).GetWithTracks(playlistID, false, false)
		},
		searchRAG,
	)
}

func serveRAGRecommendation(
	w http.ResponseWriter,
	request *http.Request,
	loadSong recommendationSongLoader,
	loadPlaylist recommendationPlaylistLoader,
	search rag.ReplacementSearchFunc,
) {
	payload, err := decodeRAGRecommendationRequest(request.Body)
	if err != nil {
		writeRAGRecommendationError(w, http.StatusBadRequest, err.Error())
		return
	}

	input := rag.RecommendationInput{Type: payload.Type, Limit: payload.Limit}
	if payload.Type == rag.RecommendationSimilarSongs {
		input.Song, err = loadSong(request.Context(), payload.SongID)
	}
	if err == nil && (payload.Type == rag.RecommendationPlaylistExpansion || payload.Type == rag.RecommendationPlaylistReplacements) {
		input.Playlist, err = loadPlaylist(request.Context(), payload.PlaylistID)
	}
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, model.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeRAGRecommendationError(w, status, err.Error())
		return
	}

	response, err := rag.Recommend(request.Context(), input, search)
	if err != nil {
		writeRAGRecommendationError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func decodeRAGRecommendationRequest(reader io.Reader) (ragRecommendationRequest, error) {
	payload := ragRecommendationRequest{Limit: rag.DefaultRecommendationLimit}
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return ragRecommendationRequest{}, fmt.Errorf("invalid request payload: %w", err)
	}
	payload.Type = rag.RecommendationType(strings.TrimSpace(string(payload.Type)))
	payload.SongID = strings.TrimSpace(payload.SongID)
	payload.PlaylistID = strings.TrimSpace(payload.PlaylistID)
	if !rag.IsSupportedRecommendationType(payload.Type) {
		return ragRecommendationRequest{}, fmt.Errorf("unsupported recommendation type %q", payload.Type)
	}
	if payload.Limit <= 0 || payload.Limit > rag.MaxSearchTopK {
		return ragRecommendationRequest{}, fmt.Errorf("limit must be between 1 and %d", rag.MaxSearchTopK)
	}
	if payload.Type == rag.RecommendationSimilarSongs && payload.SongID == "" {
		return ragRecommendationRequest{}, fmt.Errorf("songId is required for similar_songs")
	}
	if (payload.Type == rag.RecommendationPlaylistExpansion || payload.Type == rag.RecommendationPlaylistReplacements) && payload.PlaylistID == "" {
		return ragRecommendationRequest{}, fmt.Errorf("playlistId is required for %s", payload.Type)
	}
	return payload, nil
}

func writeRAGRecommendationError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
