package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

type ragRecommendationRequest struct {
	Type       rag.RecommendationType `json:"type"`
	SongID     string                 `json:"songId,omitempty"`
	PlaylistID string                 `json:"playlistId,omitempty"`
	DraftID    string                 `json:"draftId,omitempty"`
	Limit      int                    `json:"limit"`
}

type recommendationSongLoader func(context.Context, string) (*model.MediaFile, error)
type recommendationPlaylistLoader func(context.Context, string) (*model.Playlist, error)
type recommendationDecisionLoader func(context.Context, string, string) (model.PlaylistRecommendationDecisions, error)

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
		func(ctx context.Context, playlistID, draftID string) (model.PlaylistRecommendationDecisions, error) {
			return n.ds.PlaylistDraft(ctx).GetRecommendationDecisions(playlistID, draftID)
		},
	)
}

func serveRAGRecommendation(
	w http.ResponseWriter,
	request *http.Request,
	loadSong recommendationSongLoader,
	loadPlaylist recommendationPlaylistLoader,
	search rag.ReplacementSearchFunc,
	loadDecisions ...recommendationDecisionLoader,
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
	if err == nil && payload.PlaylistID != "" {
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
	if len(loadDecisions) > 0 && payload.PlaylistID != "" {
		decisions, loadErr := loadDecisions[0](request.Context(), payload.PlaylistID, payload.DraftID)
		if loadErr != nil {
			writeRAGRecommendationError(w, http.StatusInternalServerError, loadErr.Error())
			return
		}
		input.ExcludedSongIDs = map[string]struct{}{}
		input.ExcludedRecommendationKeys = map[string]struct{}{}
		for _, decision := range decisions {
			if decision.Decision == model.RecommendationDecisionBlocked {
				input.ExcludedSongIDs[decision.SongID] = struct{}{}
			}
			if decision.Decision == model.RecommendationDecisionIgnored &&
				decision.DraftID == payload.DraftID &&
				(decision.RecommendationType == "" || decision.RecommendationType == string(payload.Type)) {
				key := rag.RecommendationExclusionKey(decision.SongID, decision.OriginalSongID)
				input.ExcludedRecommendationKeys[key] = struct{}{}
			}
		}
	}

	response, err := rag.Recommend(request.Context(), input, search)
	if err != nil {
		writeRAGRecommendationError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	_, modelVersion, _ := ragEmbeddingStatus()
	response.Provenance = rag.RecommendationProvenance{
		AIModelVersion: modelVersion,
		AIIndexVersion: strconv.Itoa(rag.CurrentIndexSchemaVersion),
		RulesetVersion: rag.RecommendationRulesetVersion,
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
	payload.DraftID = strings.TrimSpace(payload.DraftID)
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
