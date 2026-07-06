package rag

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/navidrome/navidrome/model"
)

type RecommendationType string

const (
	RecommendationSimilarSongs         RecommendationType = "similar_songs"
	RecommendationUnderusedSongs       RecommendationType = "underused_songs"
	RecommendationOverplayedSongs      RecommendationType = "overplayed_songs"
	RecommendationPlaylistExpansion    RecommendationType = "playlist_expansion"
	RecommendationPlaylistReplacements RecommendationType = "playlist_replacements"
	RecommendationRetailSafeSongs      RecommendationType = "retail_safe_songs"
)

const DefaultRecommendationLimit = 20

func IsSupportedRecommendationType(value RecommendationType) bool {
	switch value {
	case RecommendationSimilarSongs, RecommendationUnderusedSongs, RecommendationOverplayedSongs,
		RecommendationPlaylistExpansion, RecommendationPlaylistReplacements, RecommendationRetailSafeSongs:
		return true
	default:
		return false
	}
}

type RecommendationInput struct {
	Type     RecommendationType
	Limit    int
	Song     *model.MediaFile
	Playlist *model.Playlist
}

type RecommendationResult struct {
	SongID    string  `json:"songId"`
	Title     string  `json:"title"`
	Artist    string  `json:"artist"`
	Album     string  `json:"album"`
	Genre     string  `json:"genre"`
	Year      int     `json:"year"`
	Explicit  bool    `json:"explicit"`
	BPM       int     `json:"bpm"`
	LUFS      float64 `json:"lufs"`
	PlayCount int64   `json:"playCount"`
	Score     float64 `json:"score"`
	Reason    string  `json:"reason"`
}

type RecommendationResponse struct {
	Type    RecommendationType     `json:"type"`
	Summary string                 `json:"summary"`
	Results []RecommendationResult `json:"results"`
	Count   int                    `json:"count"`
}

// Recommend returns read-only song suggestions over the existing hybrid RAG
// search contract. It does not expose any mutation operation.
func Recommend(ctx context.Context, input RecommendationInput, search ReplacementSearchFunc) (RecommendationResponse, error) {
	response := RecommendationResponse{Type: input.Type, Results: []RecommendationResult{}}
	if input.Limit <= 0 {
		input.Limit = DefaultRecommendationLimit
	}
	if input.Limit > MaxSearchTopK {
		return response, fmt.Errorf("limit must be between 1 and %d", MaxSearchTopK)
	}
	if search == nil {
		return response, fmt.Errorf("RAG search is unavailable")
	}

	var err error
	switch input.Type {
	case RecommendationSimilarSongs:
		response.Summary, response.Results, err = recommendSimilarSongs(ctx, input, search)
	case RecommendationUnderusedSongs:
		response.Summary, response.Results, err = recommendUnderusedSongs(ctx, input.Limit, search)
	case RecommendationOverplayedSongs:
		response.Summary, response.Results, err = recommendOverplayedSongs(ctx, input.Limit, search, time.Now())
	case RecommendationPlaylistExpansion:
		response.Summary, response.Results, err = recommendPlaylistExpansion(ctx, input, search)
	case RecommendationPlaylistReplacements:
		response.Summary, response.Results, err = recommendPlaylistReplacements(ctx, input, search)
	case RecommendationRetailSafeSongs:
		response.Summary, response.Results, err = recommendRetailSafeSongs(ctx, input.Limit, search)
	default:
		return response, fmt.Errorf("unsupported recommendation type %q", input.Type)
	}
	if err != nil {
		return response, err
	}
	response.Results = selectDiverseRecommendations(response.Results, input.Limit)
	response.Count = len(response.Results)
	return response, nil
}

func recommendSimilarSongs(ctx context.Context, input RecommendationInput, search ReplacementSearchFunc) (string, []RecommendationResult, error) {
	if input.Song == nil {
		return "", nil, fmt.Errorf("songId is required for similar_songs")
	}
	filters := SearchFilters{}
	if genre := preferredSongGenre(*input.Song); genre != "" {
		filters.Genre = genre
	}
	if input.Song.BPM > 0 {
		minimum := max(float64(input.Song.BPM)-20, 0)
		maximum := float64(input.Song.BPM) + 20
		filters.BPMMin, filters.BPMMax = &minimum, &maximum
	}
	if lufs, ok := songLUFSValue(*input.Song); ok {
		minimum, maximum := lufs-2.5, lufs+2.5
		filters.LUFSMin, filters.LUFSMax = &minimum, &maximum
	}
	results, err := search(ctx, DocumentFromMediaFile(*input.Song).Text, expandedLimit(input.Limit), filters)
	if err != nil {
		return "", nil, err
	}
	items := mapSearchResults(results, func(result SongSearchResult) string {
		return fmt.Sprintf("Similar genre and audio profile to %s", input.Song.Title)
	}, map[string]struct{}{input.Song.ID: {}})
	return fmt.Sprintf("Songs similar to %s by %s.", input.Song.Title, input.Song.Artist), items, nil
}

func recommendUnderusedSongs(ctx context.Context, limit int, search ReplacementSearchFunc) (string, []RecommendationResult, error) {
	playCountMax := int64(20)
	required := true
	filters := SearchFilters{
		Explicit: "clean", PlayCountMax: &playCountMax,
		HasLyrics: &required, HasGenre: &required, HasYear: &required, HasBPM: &required, HasLUFS: &required,
	}
	results, err := search(ctx, "high quality underused clean songs with complete metadata", expandedLimit(limit), filters)
	if err != nil {
		return "", nil, err
	}
	items := mapSearchResults(results, func(result SongSearchResult) string {
		return fmt.Sprintf("Underused clean track with %d plays and complete metadata", result.PlayCount)
	}, nil)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].PlayCount == items[j].PlayCount {
			return items[i].Score > items[j].Score
		}
		return items[i].PlayCount < items[j].PlayCount
	})
	return "Clean, metadata-complete songs with no more than 20 plays.", items, nil
}

func recommendOverplayedSongs(ctx context.Context, limit int, search ReplacementSearchFunc, now time.Time) (string, []RecommendationResult, error) {
	results, err := search(ctx, "frequently played and recently played songs", MaxSearchTopK, SearchFilters{})
	if err != nil {
		return "", nil, err
	}
	qualifying := make([]SongSearchResult, 0, len(results))
	for _, result := range results {
		if result.PlayCount >= 50 || (result.LastPlayedAt != nil && result.LastPlayedAt.After(now.AddDate(0, 0, -30))) {
			qualifying = append(qualifying, result)
		}
	}
	if len(qualifying) == 0 {
		qualifying = results
	}
	items := mapSearchResults(qualifying, func(result SongSearchResult) string {
		recent := result.LastPlayedAt != nil && result.LastPlayedAt.After(now.AddDate(0, 0, -30))
		switch {
		case result.PlayCount >= 50 && recent:
			return fmt.Sprintf("High play count (%d) and played within the last 30 days", result.PlayCount)
		case result.PlayCount >= 50:
			return fmt.Sprintf("High play count (%d)", result.PlayCount)
		case recent:
			return fmt.Sprintf("Recently played with %d total plays", result.PlayCount)
		default:
			return fmt.Sprintf("Among the highest retrieved play counts (%d)", result.PlayCount)
		}
	}, nil)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].PlayCount == items[j].PlayCount {
			return items[i].Score > items[j].Score
		}
		return items[i].PlayCount > items[j].PlayCount
	})
	return "Songs with high play counts or recent playback activity.", items, nil
}

func recommendPlaylistExpansion(ctx context.Context, input RecommendationInput, search ReplacementSearchFunc) (string, []RecommendationResult, error) {
	if input.Playlist == nil {
		return "", nil, fmt.Errorf("playlistId is required for playlist_expansion")
	}
	results, err := search(ctx, DocumentFromPlaylist(*input.Playlist).Text, expandedLimit(input.Limit), SearchFilters{})
	if err != nil {
		return "", nil, err
	}
	excluded := playlistSongIDs(input.Playlist)
	items := mapSearchResults(results, func(result SongSearchResult) string {
		return fmt.Sprintf("Matches the genres, artists, mood, or audio profile of %s", input.Playlist.Name)
	}, excluded)
	return fmt.Sprintf("Additional songs that fit %s without duplicating its current tracks.", input.Playlist.Name), items, nil
}

func recommendPlaylistReplacements(ctx context.Context, input RecommendationInput, search ReplacementSearchFunc) (string, []RecommendationResult, error) {
	if input.Playlist == nil {
		return "", nil, fmt.Errorf("playlistId is required for playlist_replacements")
	}
	analysis := AnalyzePlaylist(ctx, input.Playlist, search)
	items := make([]RecommendationResult, 0)
	seen := map[string]struct{}{}
	for _, replacement := range analysis.SuggestedReplacements {
		for _, song := range replacement.Suggestions {
			if _, exists := seen[song.SongID]; exists {
				continue
			}
			seen[song.SongID] = struct{}{}
			items = append(items, RecommendationResult{
				SongID: song.SongID, Title: song.Title, Artist: song.Artist, Album: song.Album,
				Genre: song.Genre, Year: song.Year, Explicit: song.Explicit, BPM: song.BPM,
				LUFS: song.LUFS, PlayCount: song.PlayCount, Score: song.Score,
				Reason: fmt.Sprintf("Suggested for %s: %s", replacement.ForSong.Title, strings.Join(replacement.Reasons, "; ")),
			})
		}
	}
	return fmt.Sprintf("Clean alternatives for risky or bad-fit songs in %s.", input.Playlist.Name), items, nil
}

func recommendRetailSafeSongs(ctx context.Context, limit int, search ReplacementSearchFunc) (string, []RecommendationResult, error) {
	required := true
	lufsMin, lufsMax := -14.0, -10.0
	filters := SearchFilters{
		Explicit: "clean", HasLyrics: &required, HasGenre: &required, HasYear: &required,
		HasBPM: &required, HasLUFS: &required, LUFSMin: &lufsMin, LUFSMax: &lufsMax,
	}
	results, err := search(ctx, "retail safe clean consistent songs suitable for public playback", expandedLimit(limit), filters)
	if err != nil {
		return "", nil, err
	}
	items := mapSearchResults(results, func(result SongSearchResult) string {
		return fmt.Sprintf("Clean, complete metadata, and retail-friendly loudness at %.1f LUFS", result.LUFS)
	}, nil)
	return "Clean, metadata-complete songs in the -14 to -10 LUFS retail range.", items, nil
}

func mapSearchResults(results []SongSearchResult, reason func(SongSearchResult) string, excluded map[string]struct{}) []RecommendationResult {
	items := make([]RecommendationResult, 0, len(results))
	seen := map[string]struct{}{}
	for _, result := range results {
		if result.SongID == "" {
			continue
		}
		if _, skip := excluded[result.SongID]; skip {
			continue
		}
		if _, duplicate := seen[result.SongID]; duplicate {
			continue
		}
		seen[result.SongID] = struct{}{}
		items = append(items, RecommendationResult{
			SongID: result.SongID, Title: result.Title, Artist: result.Artist, Album: result.Album,
			Genre: result.Genre, Year: result.Year, Explicit: result.Explicit, BPM: result.BPM,
			LUFS: result.LUFS, PlayCount: result.PlayCount, Score: result.Score, Reason: reason(result),
		})
	}
	return items
}

func expandedLimit(limit int) int {
	return min(max(limit*2, limit), MaxSearchTopK)
}

func playlistSongIDs(playlist *model.Playlist) map[string]struct{} {
	ids := map[string]struct{}{}
	if playlist == nil {
		return ids
	}
	for _, song := range playlist.MediaFiles() {
		ids[song.ID] = struct{}{}
	}
	return ids
}

func preferredSongGenre(song model.MediaFile) string {
	if values := cleanTagValues(song.Tags.Values(model.TagGenre)); len(values) > 0 {
		return values[0]
	}
	return strings.TrimSpace(song.Genre)
}
