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

func TestDecodeRAGRecommendationRequest(t *testing.T) {
	payload, err := decodeRAGRecommendationRequest(bytes.NewBufferString(`{"type":"similar_songs","songId":"song-1","limit":12}`))
	if err != nil {
		t.Fatalf("decode recommendation request: %v", err)
	}
	if payload.Type != rag.RecommendationSimilarSongs || payload.SongID != "song-1" || payload.Limit != 12 {
		t.Fatalf("unexpected payload: %+v", payload)
	}

	defaulted, err := decodeRAGRecommendationRequest(bytes.NewBufferString(`{"type":"retail_safe_songs"}`))
	if err != nil || defaulted.Limit != rag.DefaultRecommendationLimit {
		t.Fatalf("expected default recommendation limit: %+v err=%v", defaulted, err)
	}
}

func TestDecodeRAGRecommendationRequestValidation(t *testing.T) {
	for _, body := range []string{
		`{"type":"unknown"}`,
		`{"type":"similar_songs"}`,
		`{"type":"playlist_expansion"}`,
		`{"type":"retail_safe_songs","limit":51}`,
		`{"type":"retail_safe_songs","extra":true}`,
	} {
		if _, err := decodeRAGRecommendationRequest(bytes.NewBufferString(body)); err == nil {
			t.Fatalf("expected request to fail validation: %s", body)
		}
	}
}

func TestRAGRecommendationEndpoint(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/ai/rag/recommend", bytes.NewBufferString(`{"type":"underused_songs","limit":5}`))
	recorder := httptest.NewRecorder()
	serveRAGRecommendation(
		recorder,
		request,
		func(context.Context, string) (*model.MediaFile, error) {
			t.Fatal("song loader should not be called")
			return nil, nil
		},
		func(context.Context, string) (*model.Playlist, error) {
			t.Fatal("playlist loader should not be called")
			return nil, nil
		},
		func(context.Context, string, int, rag.SearchFilters) ([]rag.SongSearchResult, error) {
			return []rag.SongSearchResult{{SongID: "song-1", Title: "Hidden Gem", Artist: "Artist", PlayCount: 2, Score: .9}}, nil
		},
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response rag.RecommendationResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Type != rag.RecommendationUnderusedSongs || response.Count != 1 || response.Results[0].Reason == "" {
		t.Fatalf("unexpected recommendation response: %+v", response)
	}
}
