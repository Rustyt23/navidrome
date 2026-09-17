package nativeapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

func TestRAGLyricSearchReturnsHighlightedSnippet(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()
	conf.Server.EnableRAG = true
	request := httptest.NewRequest(http.MethodPost, "/api/ai/rag/lyrics/search", bytes.NewBufferString(`{"query":"song that goes 'midnight train'","topK":5}`))
	recorder := httptest.NewRecorder()
	serveRAGLyricSearch(recorder, request, func(_ context.Context, _ string, topK int, filters rag.SearchFilters) ([]rag.SongSearchResult, error) {
		if topK != 5 || filters.HasLyrics == nil || !*filters.HasLyrics {
			t.Fatalf("unexpected lyric search arguments: topK=%d filters=%+v", topK, filters)
		}
		return []rag.SongSearchResult{{SongID: "song-1", Title: "Journey", LyricsText: "take the midnight train going anywhere"}}, nil
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Results []rag.SongSearchResult `json:"results"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Results) != 1 || !strings.Contains(response.Results[0].LyricSnippet, "⟦midnight train⟧") {
		t.Fatalf("unexpected lyric results: %+v", response.Results)
	}
}

func TestPrepareAIChatMessageAnalyticsModeSkipsRetrieval(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()
	conf.Server.EnableRAG = true
	provider := staticAIChatProvider{answer: `{"mode":"analytics","query":"clean songs from 2015","filters":{"explicit":"clean","yearMin":2015,"yearMax":2015}}`}
	prompt, sources, err := prepareAIChatMessageWithFeatures(context.Background(), "how many clean songs from 2015?", nil, provider,
		func(context.Context, string, int, rag.SearchFilters) ([]rag.SongSearchResult, error) {
			t.Fatal("analytics mode must not use vector retrieval")
			return nil, nil
		}, ragChatFeatures{analytics: func(_ context.Context, filters rag.SearchFilters) (string, error) {
			if filters.Explicit != "clean" || filters.YearMin == nil || *filters.YearMin != 2015 {
				t.Fatalf("unexpected analytics filters: %+v", filters)
			}
			return `{"totalSongs":100,"matchingSongs":12}`, nil
		}})
	if err != nil || len(sources) != 0 || !strings.Contains(prompt, "Computed library analytics") || !strings.Contains(prompt, `"matchingSongs":12`) {
		t.Fatalf("unexpected analytics prompt: prompt=%q sources=%+v err=%v", prompt, sources, err)
	}
}

func TestPrepareAIChatMessageLyricModeAddsSnippet(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()
	conf.Server.EnableRAG = true
	conf.Server.RAGMinScore = 0
	provider := staticAIChatProvider{answer: `{"mode":"lyrics","query":"midnight train","filters":{}}`}
	prompt, sources, err := prepareAIChatMessageWithFeatures(context.Background(), "find the song that goes 'midnight train'", nil, provider,
		func(_ context.Context, query string, _ int, filters rag.SearchFilters) ([]rag.SongSearchResult, error) {
			if query != "midnight train" || filters.HasLyrics == nil || !*filters.HasLyrics {
				t.Fatalf("unexpected lyric plan: query=%q filters=%+v", query, filters)
			}
			return []rag.SongSearchResult{{SongID: "song-1", Title: "Journey", LyricsText: "take the midnight train going anywhere", Score: .9}}, nil
		}, ragChatFeatures{})
	if err != nil || len(sources) != 1 || !strings.Contains(sources[0].LyricSnippet, "⟦midnight train⟧") || !strings.Contains(prompt, "matching lyric:") {
		t.Fatalf("unexpected lyric chat result: prompt=%q sources=%+v err=%v", prompt, sources, err)
	}
}

func TestPrepareAIChatMessageDuplicateModeUsesCleanupReport(t *testing.T) {
	restoreConfig := conf.SnapshotConfig()
	defer restoreConfig()
	conf.Server.EnableRAG = true
	provider := staticAIChatProvider{answer: `{"mode":"duplicates","query":"duplicate songs","filters":{}}`}
	prompt, _, err := prepareAIChatMessageWithFeatures(context.Background(), "show duplicate rips", nil, provider,
		func(context.Context, string, int, rag.SearchFilters) ([]rag.SongSearchResult, error) {
			t.Fatal("duplicate mode must not use ordinary retrieval")
			return nil, nil
		}, ragChatFeatures{duplicates: func(context.Context) (string, error) {
			return `{"count":1,"candidates":[{"kind":"alternate_version"}]}`, nil
		}})
	if err != nil || !strings.Contains(prompt, "duplicate and alternate-version candidates") || !strings.Contains(prompt, "alternate_version") {
		t.Fatalf("unexpected duplicate prompt: %q err=%v", prompt, err)
	}
}
