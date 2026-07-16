package rag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestQdrantOfflineStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client := NewQdrantClient(server.URL, "songs")
	client.httpClient = server.Client()
	server.Close()

	status := client.Status(context.Background(), true)

	if status.VectorDBOnline {
		t.Fatal("expected Qdrant to be offline")
	}
	if status.CollectionExists || status.IndexedCount != 0 {
		t.Fatalf("unexpected offline status: %+v", status)
	}
	if !strings.Contains(status.Error, "Qdrant unavailable") {
		t.Fatalf("expected an unavailable error, got %q", status.Error)
	}
}

func TestQdrantStatusDetectsDimensionDrift(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/collections":
			_, _ = w.Write([]byte(`{"status":"ok","result":{"collections":[{"name":"songs"}]}}`))
		case "/collections/songs":
			_, _ = w.Write([]byte(`{"status":"ok","result":{"points_count":12,"config":{"params":{"vectors":{"size":1536,"distance":"Cosine"}}}}}`))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	schema := ExpectedIndexSchema("gemini:gemini-embedding-001")
	client := NewQdrantClient(server.URL, "songs", QdrantClientOptions{ExpectedSchema: &schema})
	client.httpClient = server.Client()
	status := client.Status(context.Background(), false)
	if !status.VectorDBOnline || !status.CollectionExists || !status.ReindexRequired || !strings.Contains(status.Error, "dimension drift") || !strings.Contains(status.Error, "full RAG reindex") {
		t.Fatalf("expected actionable dimension drift status, got %+v", status)
	}
}

func TestQdrantStatusDetectsEmbeddingModelDrift(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/collections":
			_, _ = w.Write([]byte(`{"status":"ok","result":{"collections":[{"name":"songs"}]}}`))
		case "/collections/songs":
			_, _ = w.Write([]byte(`{"status":"ok","result":{"points_count":2,"config":{"params":{"vectors":{"size":768,"distance":"Cosine"}}}}}`))
		case "/collections/songs/points/" + qdrantPointID(indexMetadataPointID):
			_, _ = w.Write([]byte(`{"status":"ok","result":{"payload":{"indexVersion":2,"embeddingModel":"gemma:old-model","dimensions":768}}}`))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	schema := ExpectedIndexSchema("gemma:new-model")
	client := NewQdrantClient(server.URL, "songs", QdrantClientOptions{ExpectedSchema: &schema})
	client.httpClient = server.Client()
	status := client.Status(context.Background(), false)
	if !status.ReindexRequired || !strings.Contains(status.Error, "schema drift") || !strings.Contains(status.Error, "gemma:old-model") || !strings.Contains(status.Error, "gemma:new-model") {
		t.Fatalf("expected actionable model drift status, got %+v", status)
	}
}

func TestQdrantCreateStoresIndexMetadata(t *testing.T) {
	metadataWritten := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/collections":
			_, _ = w.Write([]byte(`{"status":"ok","result":{"collections":[]}}`))
		case request.Method == http.MethodGet && request.URL.Path == "/collections/songs":
			http.NotFound(w, request)
		case request.Method == http.MethodPut && request.URL.Path == "/collections/songs/index":
			_, _ = w.Write([]byte(`{"status":"ok","result":true}`))
		case request.Method == http.MethodPut && request.URL.Path == "/collections/songs":
			_, _ = w.Write([]byte(`{"status":"ok","result":true}`))
		case request.Method == http.MethodPut && request.URL.Path == "/collections/songs/points":
			var body struct {
				Points []struct {
					Vector  []float32      `json:"vector"`
					Payload map[string]any `json:"payload"`
				} `json:"points"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode metadata point: %v", err)
			}
			if len(body.Points) != 1 || len(body.Points[0].Vector) != GeminiEmbeddingDimensions || body.Points[0].Payload["embeddingModel"] != "gemma:embeddinggemma" {
				t.Fatalf("unexpected metadata point: %+v", body.Points)
			}
			metadataWritten = true
			_, _ = w.Write([]byte(`{"status":"ok","result":{"status":"completed"}}`))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	schema := ExpectedIndexSchema("gemma:embeddinggemma")
	client := NewQdrantClient(server.URL, "songs", QdrantClientOptions{ExpectedSchema: &schema})
	client.httpClient = server.Client()
	status := client.Status(context.Background(), true)
	if status.Error != "" || !metadataWritten {
		t.Fatalf("expected compatible collection creation, status=%+v metadata=%t", status, metadataWritten)
	}
}

func TestQdrantRetriesTransientFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok","result":{"collections":[]}}`))
	}))
	defer server.Close()

	client := NewQdrantClient(server.URL, "songs", QdrantClientOptions{HTTPClientOptions: HTTPClientOptions{
		Timeout: time.Second, MaxRetries: 1, RetryBackoff: time.Millisecond,
	}})
	client.httpClient = server.Client()
	if err := client.CheckReachable(context.Background()); err != nil || attempts != 2 {
		t.Fatalf("expected transient Qdrant retry, attempts=%d err=%v", attempts, err)
	}
}

func TestQdrantRecreatesIncompatibleCollection(t *testing.T) {
	deleted, created, metadataWritten := false, false, false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodDelete && request.URL.Path == "/collections/songs":
			deleted = true
			_, _ = w.Write([]byte(`{"status":"ok","result":true}`))
		case request.Method == http.MethodPut && request.URL.Path == "/collections/songs/index":
			_, _ = w.Write([]byte(`{"status":"ok","result":true}`))
		case request.Method == http.MethodPut && request.URL.Path == "/collections/songs":
			created = true
			_, _ = w.Write([]byte(`{"status":"ok","result":true}`))
		case request.Method == http.MethodPut && request.URL.Path == "/collections/songs/points":
			metadataWritten = true
			_, _ = w.Write([]byte(`{"status":"ok","result":{"status":"completed"}}`))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	schema := ExpectedIndexSchema("gemini:gemini-embedding-001")
	client := NewQdrantClient(server.URL, "songs", QdrantClientOptions{ExpectedSchema: &schema})
	client.httpClient = server.Client()
	if err := client.RecreateCollection(context.Background()); err != nil {
		t.Fatalf("recreate collection: %v", err)
	}
	if !deleted || !created || !metadataWritten {
		t.Fatalf("expected delete/create/metadata sequence, deleted=%t created=%t metadata=%t", deleted, created, metadataWritten)
	}
}

func TestQdrantOnlineCollectionResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/collections":
			_, _ = w.Write([]byte(`{"status":"ok","result":{"collections":[]}}`))
		case "/collections/songs":
			_, _ = w.Write([]byte(`{"status":"ok","result":{"points_count":42}}`))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	client := NewQdrantClient(server.URL, "songs")
	client.httpClient = server.Client()
	status := client.Status(context.Background(), true)

	if !status.VectorDBOnline || !status.CollectionExists {
		t.Fatalf("expected online existing collection, got %+v", status)
	}
	if status.IndexedCount != 42 {
		t.Fatalf("expected 42 indexed points, got %d", status.IndexedCount)
	}
	if status.Error != "" {
		t.Fatalf("expected no error, got %q", status.Error)
	}
}

func TestQdrantCollectionMissingWithoutCreation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/collections" {
			_, _ = w.Write([]byte(`{"status":"ok","result":{"collections":[]}}`))
			return
		}
		http.NotFound(w, request)
	}))
	defer server.Close()

	client := NewQdrantClient(server.URL, "songs")
	client.httpClient = server.Client()
	status := client.Status(context.Background(), false)

	if !status.VectorDBOnline || status.CollectionExists {
		t.Fatalf("expected online missing collection, got %+v", status)
	}
	if status.Error != "" {
		t.Fatalf("expected no error for a missing collection, got %q", status.Error)
	}
}

func TestQdrantCreatesMissingCollection(t *testing.T) {
	created := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/collections":
			_, _ = w.Write([]byte(`{"status":"ok","result":{"collections":[]}}`))
		case request.Method == http.MethodGet && request.URL.Path == "/collections/songs":
			http.NotFound(w, request)
		case request.Method == http.MethodPut && request.URL.Path == "/collections/songs/index":
			_, _ = w.Write([]byte(`{"status":"ok","result":true}`))
		case request.Method == http.MethodPut && request.URL.Path == "/collections/songs":
			var body struct {
				Vectors struct {
					Size     int    `json:"size"`
					Distance string `json:"distance"`
				} `json:"vectors"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode collection request: %v", err)
			}
			if body.Vectors.Size != GeminiEmbeddingDimensions || body.Vectors.Distance != "Cosine" {
				t.Errorf("unexpected vector config: %+v", body.Vectors)
			}
			if request.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected JSON content type")
			}
			created = true
			_, _ = w.Write([]byte(`{"status":"ok","result":true}`))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	client := NewQdrantClient(server.URL, "songs")
	client.httpClient = server.Client()
	status := client.Status(context.Background(), true)

	if !created {
		t.Fatal("expected missing collection to be created")
	}
	if !status.VectorDBOnline || !status.CollectionExists || status.IndexedCount != 0 {
		t.Fatalf("unexpected created collection status: %+v", status)
	}
	if status.Error != "" {
		t.Fatalf("expected no error, got %q", status.Error)
	}
}

func TestQdrantUpsertRequestShape(t *testing.T) {
	var requestBody struct {
		Points []struct {
			ID      string         `json:"id"`
			Vector  []float32      `json:"vector"`
			Payload map[string]any `json:"payload"`
		} `json:"points"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut || request.URL.Path != "/collections/songs/points" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		if request.URL.Query().Get("wait") != "true" {
			t.Error("expected wait=true")
		}
		if err := json.NewDecoder(request.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode point request: %v", err)
		}
		_, _ = w.Write([]byte(`{"status":"ok","result":{"status":"completed"}}`))
	}))
	defer server.Close()

	client := NewQdrantClient(server.URL, "songs")
	client.httpClient = server.Client()
	payload := map[string]any{
		"songId": "abc",
		"type":   "song",
		"title":  "A Song",
	}
	if err := client.UpsertPoint(context.Background(), StableSongPointID("abc"), []float32{0.1, 0.2}, payload); err != nil {
		t.Fatalf("upsert point: %v", err)
	}

	if len(requestBody.Points) != 1 {
		t.Fatalf("expected one point, got %d", len(requestBody.Points))
	}
	point := requestBody.Points[0]
	if point.ID != qdrantPointID("song:abc") {
		t.Fatalf("unexpected physical point ID: %q", point.ID)
	}
	if point.Payload["ragId"] != "song:abc" || point.Payload["songId"] != "abc" || point.Payload["type"] != "song" {
		t.Fatalf("unexpected point payload: %#v", point.Payload)
	}
	if len(point.Vector) != 2 {
		t.Fatalf("unexpected vector: %#v", point.Vector)
	}
}

func TestStableSongPointID(t *testing.T) {
	if got := StableSongPointID("abc"); got != "song:abc" {
		t.Fatalf("expected logical song ID, got %q", got)
	}
	if first, second := qdrantPointID("song:abc"), qdrantPointID("song:abc"); first != second {
		t.Fatalf("expected deterministic Qdrant ID, got %q and %q", first, second)
	}
}

func TestAllIndexedSongIDsHonorsLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/collections/songs/points/scroll" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		var body struct {
			Limit int `json:"limit"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode scroll request: %v", err)
		}
		if body.Limit != 2 {
			t.Fatalf("expected Qdrant page limit 2, got %d", body.Limit)
		}
		_, _ = w.Write([]byte(`{
          "status":"ok",
          "result":{"points":[
            {"payload":{"ragId":"song:one"}},
            {"payload":{"ragId":"song:two"}},
            {"payload":{"ragId":"song:three"}}
          ],"next_page_offset":"next"}
        }`))
	}))
	defer server.Close()

	client := NewQdrantClient(server.URL, "songs")
	client.httpClient = server.Client()
	ids, err := client.AllIndexedSongIDs(context.Background(), 2)
	if err != nil {
		t.Fatalf("list indexed song IDs: %v", err)
	}
	if !reflect.DeepEqual(ids, []string{"song:one", "song:two"}) {
		t.Fatalf("expected exactly two IDs, got %#v", ids)
	}
}

func TestListSongsFiltersQdrantLyricsText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/collections/songs/points/scroll" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		var body struct {
			Filter map[string]any `json:"filter"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode lyrics scroll request: %v", err)
		}
		encodedFilter, _ := json.Marshal(body.Filter)
		filterJSON := string(encodedFilter)
		for _, expected := range []string{`"key":"type"`, `"key":"hasLyrics"`, `"key":"lyricsText"`, `"text":"vikash"`} {
			if !strings.Contains(filterJSON, expected) {
				t.Fatalf("expected %s in Qdrant filter: %s", expected, filterJSON)
			}
		}
		_, _ = w.Write([]byte(`{"status":"ok","result":{"points":[{"payload":{"songId":"song-1","title":"Vikash Song","hasLyrics":true,"lyricsText":"hello vikash"}}]}}`))
	}))
	defer server.Close()

	client := NewQdrantClient(server.URL, "songs")
	client.httpClient = server.Client()
	required := true
	songs, err := client.ListSongs(context.Background(), 10, SearchFilters{LyricsContains: "vikash", HasLyrics: &required})
	if err != nil {
		t.Fatalf("list Qdrant lyrics: %v", err)
	}
	if len(songs) != 1 || songs[0].LyricsText != "hello vikash" {
		t.Fatalf("unexpected Qdrant lyrics: %+v", songs)
	}
}

func TestQdrantSearchRequestAndResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/collections/songs/points/query" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		var body struct {
			Query       []float32      `json:"query"`
			Limit       int            `json:"limit"`
			WithPayload bool           `json:"with_payload"`
			WithVector  bool           `json:"with_vector"`
			Filter      map[string]any `json:"filter"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode search request: %v", err)
		}
		must, _ := body.Filter["must"].([]any)
		if len(body.Query) != 2 || body.Limit != 5 || !body.WithPayload || body.WithVector || len(must) != 1 {
			t.Fatalf("unexpected search request: %+v", body)
		}
		condition, _ := must[0].(map[string]any)
		match, _ := condition["match"].(map[string]any)
		if condition["key"] != "type" || match["value"] != "song" {
			t.Fatalf("expected search to be restricted to song documents: %+v", body.Filter)
		}
		_, _ = w.Write([]byte(`{
          "status":"ok",
          "result":{"points":[{
            "id":"00000000-0000-5000-8000-000000000001",
            "score":0.87,
            "payload":{
              "songId":"song-1","type":"song","title":"Bright Song",
              "artist":"Artist","album":"Album","year":2020,"genre":"Pop",
              "explicit":false,"bpm":100,"lufs":-12.5
            }
          }]}
        }`))
	}))
	defer server.Close()

	client := NewQdrantClient(server.URL, "songs")
	client.httpClient = server.Client()
	results, err := client.Search(context.Background(), []float32{0.1, 0.2}, 5)
	if err != nil {
		t.Fatalf("search Qdrant: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	expected := SongSearchResult{
		SongID: "song-1", Title: "Bright Song", Artist: "Artist", Album: "Album",
		Year: 2020, Genre: "Pop", Explicit: false, BPM: 100, LUFS: -12.5, Score: 0.87,
	}
	if results[0] != expected {
		t.Fatalf("expected %+v, got %+v", expected, results[0])
	}
}

func TestQdrantSearchIncludesPayloadFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		var body struct {
			Filter map[string]any `json:"filter"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode search request: %v", err)
		}
		if body.Filter == nil {
			t.Fatal("expected Qdrant payload filter")
		}
		_, _ = w.Write([]byte(`{"result":{"points":[]}}`))
	}))
	defer server.Close()

	client := NewQdrantClient(server.URL, "songs")
	client.httpClient = server.Client()
	yearMin := 2000
	if _, err := client.Search(context.Background(), []float32{0.1}, 5, SearchFilters{Explicit: "clean", YearMin: &yearMin}); err != nil {
		t.Fatalf("search Qdrant: %v", err)
	}
}

func TestQdrantListSongsRequestAndResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/collections/songs/points/scroll" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		var body struct {
			Limit       int  `json:"limit"`
			WithPayload bool `json:"with_payload"`
			WithVector  bool `json:"with_vector"`
			Filter      struct {
				Must []struct {
					Key   string `json:"key"`
					Match struct {
						Value string `json:"value"`
					} `json:"match"`
				} `json:"must"`
			} `json:"filter"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode scroll request: %v", err)
		}
		if body.Limit != 100 || !body.WithPayload || body.WithVector || len(body.Filter.Must) != 1 || body.Filter.Must[0].Key != "type" || body.Filter.Must[0].Match.Value != "song" {
			t.Fatalf("unexpected scroll request: %+v", body)
		}
		_, _ = w.Write([]byte(`{
          "result":{"points":[{
            "id":"point-1",
            "payload":{"songId":"song-1","libraryId":2,"folderId":"folder-1","title":"Bright Song","artist":"Artist","artistId":"artist-1","album":"Album","albumId":"album-1","albumArtist":"Album Artist","trackNumber":3,"discNumber":1,"year":2020,"genre":"Pop","genres":["Pop","Dance"],"moods":["Upbeat"],"explicit":false,"explicitStatus":"clean","bpm":100,"lufs":-12.5,"duration":215,"playCount":7,"hasLyrics":true,"codec":"flac","bitRate":900,"mbzRecordingId":"recording-1","spotifyUrl":"https://open.spotify.com/track/1"}
          }]}
        }`))
	}))
	defer server.Close()

	client := NewQdrantClient(server.URL, "songs")
	client.httpClient = server.Client()
	songs, err := client.ListSongs(context.Background(), 100)
	if err != nil {
		t.Fatalf("list Qdrant songs: %v", err)
	}
	if len(songs) != 1 || songs[0].SongID != "song-1" || songs[0].Title != "Bright Song" || songs[0].LUFS != -12.5 ||
		songs[0].LibraryID != 2 || songs[0].AlbumArtist != "Album Artist" || songs[0].TrackNumber != 3 ||
		len(songs[0].Genres) != 2 || songs[0].Moods[0] != "Upbeat" || songs[0].ExplicitStatus != "clean" ||
		songs[0].Duration != 215 || songs[0].PlayCount != 7 || !songs[0].HasLyrics || songs[0].Codec != "flac" ||
		songs[0].MBZRecordingID != "recording-1" || songs[0].SpotifyURL == "" {
		t.Fatalf("unexpected songs: %+v", songs)
	}
}

func TestQdrantCountDocumentsByType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/collections/library/points/count" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode count request: %v", err)
		}
		if body["exact"] != true || body["filter"] == nil {
			t.Fatalf("expected exact filtered count request: %#v", body)
		}
		_, _ = w.Write([]byte(`{"result":{"count":42},"status":"ok"}`))
	}))
	defer server.Close()
	client := NewQdrantClient(server.URL, "library")
	client.httpClient = server.Client()
	count, err := client.CountDocumentsByType(context.Background(), "song")
	if err != nil || count != 42 {
		t.Fatalf("unexpected count=%d err=%v", count, err)
	}
}

func TestQdrantListSongVectors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/collections/songs/points/scroll" {
			http.NotFound(w, request)
			return
		}
		_, _ = w.Write([]byte(`{"result":{"points":[{"vector":[0.1,0.2],"payload":{"songId":"song-1","title":"Song","artist":"Artist"}}],"next_page_offset":null},"status":"ok"}`))
	}))
	defer server.Close()
	client := NewQdrantClient(server.URL, "songs")
	client.httpClient = server.Client()
	items, err := client.ListSongVectors(context.Background(), 10)
	if err != nil || len(items) != 1 || items[0].Song.SongID != "song-1" || len(items[0].Vector) != 2 {
		t.Fatalf("unexpected song vectors: items=%+v err=%v", items, err)
	}
}
