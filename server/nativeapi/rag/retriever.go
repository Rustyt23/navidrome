package rag

import (
	"context"
	"time"
)

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
	SongID       string     `json:"songId"`
	Title        string     `json:"title"`
	Artist       string     `json:"artist"`
	Album        string     `json:"album"`
	Year         int        `json:"year"`
	Genre        string     `json:"genre"`
	Explicit     bool       `json:"explicit"`
	BPM          int        `json:"bpm"`
	LUFS         float64    `json:"lufs"`
	Duration     float64    `json:"duration"`
	PlayCount    int64      `json:"playCount"`
	LastPlayedAt *time.Time `json:"lastPlayedAt,omitempty"`
	HasLyrics    bool       `json:"hasLyrics"`
	HasGenre     bool       `json:"hasGenre"`
	HasYear      bool       `json:"hasYear"`
	HasBPM       bool       `json:"hasBpm"`
	HasLUFS      bool       `json:"hasLufs"`
	Score        float64    `json:"score"`
}

// IndexedSong is the song payload stored in Qdrant without its vector.
type IndexedSong struct {
	SongID             string     `json:"songId"`
	LibraryID          int        `json:"libraryId"`
	FolderID           string     `json:"folderId"`
	Title              string     `json:"title"`
	Artist             string     `json:"artist"`
	ArtistID           string     `json:"artistId"`
	Album              string     `json:"album"`
	AlbumID            string     `json:"albumId"`
	AlbumArtist        string     `json:"albumArtist"`
	AlbumArtistID      string     `json:"albumArtistId"`
	TrackNumber        int        `json:"trackNumber"`
	DiscNumber         int        `json:"discNumber"`
	DiscSubtitle       string     `json:"discSubtitle"`
	Compilation        bool       `json:"compilation"`
	Year               int        `json:"year"`
	Date               string     `json:"date"`
	OriginalYear       int        `json:"originalYear"`
	OriginalDate       string     `json:"originalDate"`
	ReleaseYear        int        `json:"releaseYear"`
	ReleaseDate        string     `json:"releaseDate"`
	Genre              string     `json:"genre"`
	Genres             []string   `json:"genres"`
	Moods              []string   `json:"moods"`
	Groupings          []string   `json:"groupings"`
	Composers          []string   `json:"composers"`
	Lyricists          []string   `json:"lyricists"`
	RecordLabels       []string   `json:"recordLabels"`
	ISRC               []string   `json:"isrc"`
	Explicit           bool       `json:"explicit"`
	ExplicitStatus     string     `json:"explicitStatus"`
	BPM                int        `json:"bpm"`
	LUFS               float64    `json:"lufs"`
	Duration           float64    `json:"duration"`
	PlayCount          int64      `json:"playCount"`
	LastPlayedAt       *time.Time `json:"lastPlayedAt,omitempty"`
	Rating             int        `json:"rating"`
	RatedAt            *time.Time `json:"ratedAt,omitempty"`
	Starred            bool       `json:"starred"`
	StarredAt          *time.Time `json:"starredAt,omitempty"`
	AverageRating      float64    `json:"averageRating"`
	Size               int64      `json:"size"`
	Suffix             string     `json:"suffix"`
	Codec              string     `json:"codec"`
	BitRate            int        `json:"bitRate"`
	SampleRate         int        `json:"sampleRate"`
	BitDepth           int        `json:"bitDepth"`
	Channels           int        `json:"channels"`
	CatalogNum         string     `json:"catalogNum"`
	MBZRecordingID     string     `json:"mbzRecordingId"`
	MBZReleaseID       string     `json:"mbzReleaseId"`
	MBZReleaseTrackID  string     `json:"mbzReleaseTrackId"`
	MBZAlbumID         string     `json:"mbzAlbumId"`
	MBZReleaseGroupID  string     `json:"mbzReleaseGroupId"`
	MBZArtistID        string     `json:"mbzArtistId"`
	MBZAlbumArtistID   string     `json:"mbzAlbumArtistId"`
	MBZAlbumType       string     `json:"mbzAlbumType"`
	SpotifyConfidence  float64    `json:"spotifyConfidence"`
	SpotifyMatch       string     `json:"spotifyMatch"`
	SpotifyArtist      string     `json:"spotifyArtist"`
	SpotifyURL         string     `json:"spotifyUrl"`
	RGAlbumGain        *float64   `json:"rgAlbumGain,omitempty"`
	RGAlbumPeak        *float64   `json:"rgAlbumPeak,omitempty"`
	RGTrackGain        *float64   `json:"rgTrackGain,omitempty"`
	RGTrackPeak        *float64   `json:"rgTrackPeak,omitempty"`
	HasCoverArt        bool       `json:"hasCoverArt"`
	Missing            bool       `json:"missing"`
	CreatedAt          *time.Time `json:"createdAt,omitempty"`
	UpdatedAt          *time.Time `json:"updatedAt,omitempty"`
	HasLyrics          bool       `json:"hasLyrics"`
	HasGenre           bool       `json:"hasGenre"`
	HasMood            bool       `json:"hasMood"`
	HasYear            bool       `json:"hasYear"`
	HasDate            bool       `json:"hasDate"`
	HasBPM             bool       `json:"hasBpm"`
	HasLUFS            bool       `json:"hasLufs"`
	HasReplayGain      bool       `json:"hasReplayGain"`
	HasMusicBrainzIDs  bool       `json:"hasMusicBrainzIds"`
	HasSpotifyMetadata bool       `json:"hasSpotifyMetadata"`
}

// VectorSearcher is the provider-independent search boundary used by Phase 1.
type VectorSearcher interface {
	Search(ctx context.Context, vector []float32, topK int, filters ...SearchFilters) ([]SongSearchResult, error)
}

// SearchSongs embeds a query and performs read-only vector search.
func SearchSongs(
	ctx context.Context,
	embedder Embedder,
	searcher VectorSearcher,
	query string,
	topK int,
	filters ...SearchFilters,
) ([]SongSearchResult, error) {
	vector, err := embedder.EmbedText(ctx, query)
	if err != nil {
		return nil, err
	}
	searchFilters := SearchFilters{}
	if len(filters) > 0 {
		searchFilters = filters[0]
	}
	return searcher.Search(ctx, vector, topK, searchFilters)
}

var _ VectorSearcher = (*QdrantClient)(nil)
