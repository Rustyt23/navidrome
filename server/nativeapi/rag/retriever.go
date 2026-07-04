package rag

import "context"

const MaxSearchTopK = 50
const MaxListLimit = 500

// RetrievalResult is a provider-independent document retrieval result.
type RetrievalResult struct {
	Document RAGDocument `json:"document"`
	Score    float64     `json:"score"`
}

// Retriever defines a provider-independent read-only retrieval boundary.
type Retriever interface {
	Retrieve(context.Context, string, int) ([]RetrievalResult, error)
}

// SongSearchResult is the read-only song metadata returned by Qdrant search
// and exposed as a source to the test endpoint and AI chat.
type SongSearchResult struct {
	SongID   string  `json:"songId"`
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	Album    string  `json:"album"`
	Year     int     `json:"year"`
	Genre    string  `json:"genre"`
	Explicit bool    `json:"explicit"`
	BPM      int     `json:"bpm"`
	LUFS     float64 `json:"lufs"`
	Score    float64 `json:"score"`
}

// IndexedSong is the song payload stored in Qdrant without its vector.
type IndexedSong struct {
	SongID   string  `json:"songId"`
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	Album    string  `json:"album"`
	Year     int     `json:"year"`
	Genre    string  `json:"genre"`
	Explicit bool    `json:"explicit"`
	BPM      int     `json:"bpm"`
	LUFS     float64 `json:"lufs"`
}

// VectorSearcher is the provider-independent search boundary used by Phase 1.
type VectorSearcher interface {
	Search(ctx context.Context, vector []float32, topK int) ([]SongSearchResult, error)
}

// SearchSongs embeds a query and performs read-only vector search.
func SearchSongs(
	ctx context.Context,
	embedder Embedder,
	searcher VectorSearcher,
	query string,
	topK int,
) ([]SongSearchResult, error) {
	vector, err := embedder.EmbedText(ctx, query)
	if err != nil {
		return nil, err
	}
	return searcher.Search(ctx, vector, topK)
}

var _ VectorSearcher = (*QdrantClient)(nil)
