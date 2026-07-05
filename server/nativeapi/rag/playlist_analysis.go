package rag

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/navidrome/navidrome/model"
)

const (
	playlistReplacementTopK = 8
	loudnessOutlierLUFS     = 3.0
	bpmOutlierThreshold     = 25.0
)

// PlaylistSummaryItem is a counted genre or artist in a playlist report.
type PlaylistSummaryItem struct {
	Name    string  `json:"name"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

// PlaylistSongReference is the stable, read-only song identity used in issues.
type PlaylistSongReference struct {
	SongID string `json:"songId"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
}

type PlaylistMetadataIssue struct {
	PlaylistSongReference
	Missing []string `json:"missing"`
}

type PlaylistDuplicateSong struct {
	PlaylistSongReference
	Count     int   `json:"count"`
	Positions []int `json:"positions"`
}

type PlaylistLoudnessIssue struct {
	PlaylistSongReference
	LUFS   float64 `json:"lufs"`
	Reason string  `json:"reason"`
}

type PlaylistBadFitSong struct {
	PlaylistSongReference
	Reasons []string `json:"reasons"`
}

type PlaylistExplicitRisk struct {
	Level   string                  `json:"level"`
	Count   int                     `json:"count"`
	Percent float64                 `json:"percent"`
	Songs   []PlaylistSongReference `json:"songs"`
}

type PlaylistReplacementSong struct {
	SongID   string  `json:"songId"`
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	Album    string  `json:"album"`
	Genre    string  `json:"genre"`
	BPM      int     `json:"bpm"`
	LUFS     float64 `json:"lufs"`
	Duration float64 `json:"duration"`
	Score    float64 `json:"score"`
}

type PlaylistReplacementSuggestion struct {
	ForSong     PlaylistSongReference     `json:"forSong"`
	Reasons     []string                  `json:"reasons"`
	Suggestions []PlaylistReplacementSong `json:"suggestions"`
}

// PlaylistAnalysis is a read-only report. It contains no mutation command or
// playlist-write result by design.
type PlaylistAnalysis struct {
	PlaylistID            string                          `json:"playlistId"`
	Name                  string                          `json:"name"`
	Summary               string                          `json:"summary"`
	SongCount             int                             `json:"songCount"`
	Duration              float64                         `json:"duration"`
	GenreSummary          []PlaylistSummaryItem           `json:"genreSummary"`
	ArtistSummary         []PlaylistSummaryItem           `json:"artistSummary"`
	ExplicitRisk          PlaylistExplicitRisk            `json:"explicitRisk"`
	MetadataIssues        []PlaylistMetadataIssue         `json:"metadataIssues"`
	DuplicateSongs        []PlaylistDuplicateSong         `json:"duplicateSongs"`
	LoudnessIssues        []PlaylistLoudnessIssue         `json:"loudnessIssues"`
	BadFitSongs           []PlaylistBadFitSong            `json:"badFitSongs"`
	SuggestedReplacements []PlaylistReplacementSuggestion `json:"suggestedReplacements"`
	Recommendations       []string                        `json:"recommendations"`
}

// ReplacementSearchFunc is the existing hybrid song search contract used to
// find suggestions. The analysis remains useful when this function is nil or
// temporarily unavailable.
type ReplacementSearchFunc func(context.Context, string, int, SearchFilters) ([]SongSearchResult, error)

type playlistNumericSummary struct {
	Count   int
	Missing int
	Min     float64
	Max     float64
	Average float64
	Median  float64
}

type playlistStats struct {
	Genres        []PlaylistSummaryItem
	Artists       []PlaylistSummaryItem
	ExplicitSongs []PlaylistSongReference
	BPM           playlistNumericSummary
	LUFS          playlistNumericSummary
}

// AnalyzePlaylist runs deterministic checks and optionally retrieves clean
// replacement candidates. It never writes to Navidrome or Qdrant.
func AnalyzePlaylist(ctx context.Context, playlist *model.Playlist, search ReplacementSearchFunc) PlaylistAnalysis {
	if playlist == nil {
		return PlaylistAnalysis{}
	}

	tracks := playlist.MediaFiles()
	stats := summarizePlaylist(tracks)
	analysis := PlaylistAnalysis{
		PlaylistID:     playlist.ID,
		Name:           playlist.Name,
		SongCount:      len(tracks),
		Duration:       playlistDuration(playlist, tracks),
		GenreSummary:   stats.Genres,
		ArtistSummary:  stats.Artists,
		MetadataIssues: detectMetadataIssues(tracks),
		DuplicateSongs: detectDuplicateSongs(tracks),
		ExplicitRisk:   explicitRisk(stats.ExplicitSongs, len(tracks)),
	}

	analysis.LoudnessIssues = detectLoudnessIssues(tracks, stats.LUFS)
	analysis.BadFitSongs = detectBadFitSongs(tracks, stats)
	analysis.Summary = buildPlaylistAnalysisSummary(analysis)
	analysis.Recommendations = buildPlaylistRecommendations(analysis, stats)
	analysis.SuggestedReplacements, analysis.Recommendations = suggestPlaylistReplacements(
		ctx,
		playlist,
		analysis,
		search,
		analysis.Recommendations,
	)
	return analysis
}

func summarizePlaylist(tracks model.MediaFiles) playlistStats {
	genreCounts := map[string]int{}
	genreNames := map[string]string{}
	artistCounts := map[string]int{}
	artistNames := map[string]string{}
	bpmValues := make([]float64, 0, len(tracks))
	lufsValues := make([]float64, 0, len(tracks))
	stats := playlistStats{}

	for _, song := range tracks {
		genres := cleanTagValues(song.Tags.Values(model.TagGenre))
		if len(genres) == 0 && strings.TrimSpace(song.Genre) != "" {
			genres = []string{song.Genre}
		}
		for _, genre := range genres {
			countName(genreCounts, genreNames, genre)
		}
		countName(artistCounts, artistNames, song.Artist)
		if song.ExplicitStatus == "e" || strings.EqualFold(song.ExplicitStatus, "explicit") {
			stats.ExplicitSongs = append(stats.ExplicitSongs, songReference(song))
		}
		if song.BPM > 0 {
			bpmValues = append(bpmValues, float64(song.BPM))
		}
		if lufs, ok := songLUFSValue(song); ok {
			lufsValues = append(lufsValues, lufs)
		}
	}

	stats.Genres = countedSummary(genreCounts, genreNames, len(tracks))
	stats.Artists = countedSummary(artistCounts, artistNames, len(tracks))
	stats.BPM = numericSummary(bpmValues, len(tracks))
	stats.LUFS = numericSummary(lufsValues, len(tracks))
	return stats
}

func countName(counts map[string]int, names map[string]string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	key := strings.ToLower(value)
	counts[key]++
	if names[key] == "" {
		names[key] = value
	}
}

func countedSummary(counts map[string]int, names map[string]string, total int) []PlaylistSummaryItem {
	items := make([]PlaylistSummaryItem, 0, len(counts))
	for key, count := range counts {
		percent := 0.0
		if total > 0 {
			percent = math.Round(float64(count)*1000/float64(total)) / 10
		}
		items = append(items, PlaylistSummaryItem{Name: names[key], Count: count, Percent: percent})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		}
		return items[i].Count > items[j].Count
	})
	return items
}

func numericSummary(values []float64, total int) playlistNumericSummary {
	result := playlistNumericSummary{Count: len(values), Missing: max(total-len(values), 0)}
	if len(values) == 0 {
		return result
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	result.Min = sorted[0]
	result.Max = sorted[len(sorted)-1]
	for _, value := range sorted {
		result.Average += value
	}
	result.Average /= float64(len(sorted))
	middle := len(sorted) / 2
	if len(sorted)%2 == 0 {
		result.Median = (sorted[middle-1] + sorted[middle]) / 2
	} else {
		result.Median = sorted[middle]
	}
	return result
}

func detectMetadataIssues(tracks model.MediaFiles) []PlaylistMetadataIssue {
	issues := make([]PlaylistMetadataIssue, 0)
	for _, song := range tracks {
		missing := make([]string, 0, 5)
		if !songHasLyrics(song) {
			missing = append(missing, "lyrics")
		}
		if strings.TrimSpace(song.Genre) == "" && len(cleanTagValues(song.Tags.Values(model.TagGenre))) == 0 {
			missing = append(missing, "genre")
		}
		if song.Year <= 0 {
			missing = append(missing, "year")
		}
		if song.BPM <= 0 {
			missing = append(missing, "BPM")
		}
		if _, ok := songLUFSValue(song); !ok {
			missing = append(missing, "LUFS")
		}
		if len(missing) > 0 {
			issues = append(issues, PlaylistMetadataIssue{PlaylistSongReference: songReference(song), Missing: missing})
		}
	}
	return issues
}

func detectDuplicateSongs(tracks model.MediaFiles) []PlaylistDuplicateSong {
	type occurrence struct {
		song      model.MediaFile
		positions []int
	}
	seen := map[string]*occurrence{}
	order := make([]string, 0)
	for index, song := range tracks {
		key := strings.TrimSpace(song.ID)
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(song.Title)) + "\x00" + strings.ToLower(strings.TrimSpace(song.Artist))
		}
		if key == "\x00" {
			continue
		}
		if seen[key] == nil {
			seen[key] = &occurrence{song: song}
			order = append(order, key)
		}
		seen[key].positions = append(seen[key].positions, index+1)
	}
	duplicates := make([]PlaylistDuplicateSong, 0)
	for _, key := range order {
		item := seen[key]
		if len(item.positions) < 2 {
			continue
		}
		duplicates = append(duplicates, PlaylistDuplicateSong{
			PlaylistSongReference: songReference(item.song),
			Count:                 len(item.positions),
			Positions:             item.positions,
		})
	}
	return duplicates
}

func detectLoudnessIssues(tracks model.MediaFiles, summary playlistNumericSummary) []PlaylistLoudnessIssue {
	if summary.Count < 3 {
		return []PlaylistLoudnessIssue{}
	}
	issues := make([]PlaylistLoudnessIssue, 0)
	for _, song := range tracks {
		lufs, ok := songLUFSValue(song)
		if !ok || math.Abs(lufs-summary.Median) <= loudnessOutlierLUFS {
			continue
		}
		issues = append(issues, PlaylistLoudnessIssue{
			PlaylistSongReference: songReference(song),
			LUFS:                  lufs,
			Reason:                fmt.Sprintf("%.1f LUFS differs from the playlist median %.1f LUFS", lufs, summary.Median),
		})
	}
	return issues
}

func detectBadFitSongs(tracks model.MediaFiles, stats playlistStats) []PlaylistBadFitSong {
	reasonsBySong := map[string][]string{}
	order := make([]string, 0)
	songs := map[string]model.MediaFile{}
	addReason := func(song model.MediaFile, reason string) {
		key := songKey(song)
		if _, ok := songs[key]; !ok {
			order = append(order, key)
			songs[key] = song
		}
		if !slices.Contains(reasonsBySong[key], reason) {
			reasonsBySong[key] = append(reasonsBySong[key], reason)
		}
	}

	genreCounts := map[string]int{}
	for _, item := range stats.Genres {
		genreCounts[strings.ToLower(item.Name)] = item.Count
	}
	dominantGenre := ""
	if len(stats.Genres) > 0 && stats.Genres[0].Count >= 2 {
		dominantGenre = stats.Genres[0].Name
	}
	artistLimit := max(3, int(math.Ceil(float64(len(tracks))*0.4)))
	artistSeen := map[string]int{}

	for _, song := range tracks {
		if stats.BPM.Count >= 3 && song.BPM > 0 && math.Abs(float64(song.BPM)-stats.BPM.Median) > bpmOutlierThreshold {
			addReason(song, fmt.Sprintf("BPM %d is far from the playlist median %.0f", song.BPM, stats.BPM.Median))
		}
		genre := strings.TrimSpace(song.Genre)
		if values := cleanTagValues(song.Tags.Values(model.TagGenre)); len(values) > 0 {
			genre = values[0]
		}
		if len(tracks) >= 4 && dominantGenre != "" && genre != "" && !strings.EqualFold(genre, dominantGenre) && genreCounts[strings.ToLower(genre)] == 1 {
			addReason(song, fmt.Sprintf("genre %s is an outlier from dominant genre %s", genre, dominantGenre))
		}
		artistKey := strings.ToLower(strings.TrimSpace(song.Artist))
		if artistKey != "" {
			artistSeen[artistKey]++
			if artistSeen[artistKey] > artistLimit {
				addReason(song, fmt.Sprintf("artist %s is repeated more than %d times", song.Artist, artistLimit))
			}
		}
	}
	for _, issue := range detectLoudnessIssues(tracks, stats.LUFS) {
		for _, song := range tracks {
			if song.ID == issue.SongID {
				addReason(song, issue.Reason)
				break
			}
		}
	}

	badFit := make([]PlaylistBadFitSong, 0, len(order))
	for _, key := range order {
		badFit = append(badFit, PlaylistBadFitSong{
			PlaylistSongReference: songReference(songs[key]),
			Reasons:               reasonsBySong[key],
		})
	}
	return badFit
}

func explicitRisk(songs []PlaylistSongReference, total int) PlaylistExplicitRisk {
	percent := 0.0
	if total > 0 {
		percent = math.Round(float64(len(songs))*1000/float64(total)) / 10
	}
	level := "none"
	if len(songs) > 0 {
		level = "medium"
	}
	if percent >= 20 {
		level = "high"
	}
	return PlaylistExplicitRisk{Level: level, Count: len(songs), Percent: percent, Songs: songs}
}

func buildPlaylistAnalysisSummary(analysis PlaylistAnalysis) string {
	if analysis.SongCount == 0 {
		return "This playlist is empty."
	}
	genre := "mixed or unknown genres"
	if len(analysis.GenreSummary) > 0 {
		genre = analysis.GenreSummary[0].Name
	}
	return fmt.Sprintf(
		"%d songs led by %s, with %d metadata issues, %d duplicate entries, %d loudness outliers, and %d explicit-risk songs.",
		analysis.SongCount,
		genre,
		len(analysis.MetadataIssues),
		len(analysis.DuplicateSongs),
		len(analysis.LoudnessIssues),
		analysis.ExplicitRisk.Count,
	)
}

func buildPlaylistRecommendations(analysis PlaylistAnalysis, stats playlistStats) []string {
	recommendations := make([]string, 0, 6)
	if len(analysis.DuplicateSongs) > 0 {
		recommendations = append(recommendations, fmt.Sprintf("Review %d duplicated songs.", len(analysis.DuplicateSongs)))
	}
	if analysis.ExplicitRisk.Count > 0 {
		recommendations = append(recommendations, fmt.Sprintf("Review %d explicit-risk songs and consider the clean alternatives below.", analysis.ExplicitRisk.Count))
	}
	if len(analysis.MetadataIssues) > 0 {
		recommendations = append(recommendations, fmt.Sprintf("Complete missing metadata for %d songs before relying on precise playlist matching.", len(analysis.MetadataIssues)))
	}
	if len(analysis.LoudnessIssues) > 0 {
		recommendations = append(recommendations, fmt.Sprintf("Review %d LUFS outliers for smoother playback loudness.", len(analysis.LoudnessIssues)))
	}
	if len(analysis.BadFitSongs) > 0 {
		recommendations = append(recommendations, fmt.Sprintf("Review %d songs that differ from the playlist's dominant profile.", len(analysis.BadFitSongs)))
	}
	if len(stats.Artists) > 0 && stats.Artists[0].Percent > 40 {
		recommendations = append(recommendations, fmt.Sprintf("Increase artist variety; %s represents %.1f%% of the playlist.", stats.Artists[0].Name, stats.Artists[0].Percent))
	}
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "No major rule-based issues were detected.")
	}
	return recommendations
}

func suggestPlaylistReplacements(
	ctx context.Context,
	playlist *model.Playlist,
	analysis PlaylistAnalysis,
	search ReplacementSearchFunc,
	recommendations []string,
) ([]PlaylistReplacementSuggestion, []string) {
	if search == nil {
		return []PlaylistReplacementSuggestion{}, recommendations
	}

	trackByID := map[string]model.MediaFile{}
	inPlaylist := map[string]struct{}{}
	for _, song := range playlist.MediaFiles() {
		trackByID[song.ID] = song
		inPlaylist[song.ID] = struct{}{}
	}
	reasons := map[string][]string{}
	order := make([]string, 0)
	addCandidate := func(songID string, reasonValues []string) {
		if songID == "" {
			return
		}
		if _, ok := reasons[songID]; !ok {
			order = append(order, songID)
		}
		for _, reason := range reasonValues {
			if !slices.Contains(reasons[songID], reason) {
				reasons[songID] = append(reasons[songID], reason)
			}
		}
	}
	for _, song := range analysis.ExplicitRisk.Songs {
		addCandidate(song.SongID, []string{"explicit-risk song"})
	}
	for _, song := range analysis.BadFitSongs {
		addCandidate(song.SongID, song.Reasons)
	}

	suggestions := make([]PlaylistReplacementSuggestion, 0, len(order))
	searchFailed := false
	for _, songID := range order {
		song, ok := trackByID[songID]
		if !ok {
			continue
		}
		filters := replacementFilters(song)
		query := fmt.Sprintf("clean song similar to %s by %s for playlist %s", song.Title, song.Artist, playlist.Name)
		results, err := search(ctx, query, playlistReplacementTopK, filters)
		if err != nil {
			searchFailed = true
			continue
		}
		options := make([]PlaylistReplacementSong, 0, 3)
		seen := map[string]struct{}{}
		for _, result := range results {
			if result.Explicit || result.SongID == song.ID {
				continue
			}
			if _, exists := inPlaylist[result.SongID]; exists {
				continue
			}
			if _, exists := seen[result.SongID]; exists {
				continue
			}
			seen[result.SongID] = struct{}{}
			options = append(options, PlaylistReplacementSong{
				SongID: result.SongID, Title: result.Title, Artist: result.Artist,
				Album: result.Album, Genre: result.Genre, BPM: result.BPM, LUFS: result.LUFS,
				Duration: result.Duration, Score: result.Score,
			})
			if len(options) == 3 {
				break
			}
		}
		suggestions = append(suggestions, PlaylistReplacementSuggestion{
			ForSong: songReference(song), Reasons: reasons[songID], Suggestions: options,
		})
	}
	if searchFailed {
		recommendations = append(recommendations, "Some replacement searches were unavailable; the rule-based analysis is still complete.")
	}
	return suggestions, recommendations
}

func replacementFilters(song model.MediaFile) SearchFilters {
	filters := SearchFilters{Explicit: "clean"}
	genre := strings.TrimSpace(song.Genre)
	if values := cleanTagValues(song.Tags.Values(model.TagGenre)); len(values) > 0 {
		genre = values[0]
	}
	if genre != "" {
		filters.Genre = genre
	}
	if song.BPM > 0 {
		minimum := max(float64(song.BPM)-15, 0)
		maximum := float64(song.BPM) + 15
		filters.BPMMin = &minimum
		filters.BPMMax = &maximum
	}
	if lufs, ok := songLUFSValue(song); ok {
		minimum := lufs - 2
		maximum := lufs + 2
		filters.LUFSMin = &minimum
		filters.LUFSMax = &maximum
	}
	return filters
}

func playlistDuration(playlist *model.Playlist, tracks model.MediaFiles) float64 {
	if playlist.Duration > 0 {
		return float64(playlist.Duration)
	}
	duration := 0.0
	for _, song := range tracks {
		duration += float64(song.Duration)
	}
	return duration
}

func songReference(song model.MediaFile) PlaylistSongReference {
	return PlaylistSongReference{SongID: song.ID, Title: song.Title, Artist: song.Artist}
}

func songKey(song model.MediaFile) string {
	if song.ID != "" {
		return song.ID
	}
	return strings.ToLower(strings.TrimSpace(song.Title)) + "\x00" + strings.ToLower(strings.TrimSpace(song.Artist))
}
