package rag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
