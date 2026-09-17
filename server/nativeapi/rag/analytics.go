package rag

import (
	"sort"
	"strconv"
	"strings"

	"github.com/navidrome/navidrome/model"
)

type AnalyticsCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type LibraryAnalyticsReport struct {
	TotalSongs         int              `json:"totalSongs"`
	MatchingSongs      int              `json:"matchingSongs"`
	AppliedFilters     SearchFilters    `json:"appliedFilters"`
	CleanSongs         int              `json:"cleanSongs"`
	ExplicitSongs      int              `json:"explicitSongs"`
	UnknownExplicit    int              `json:"unknownExplicit"`
	SongsWithLyrics    int              `json:"songsWithLyrics"`
	SongsWithoutLyrics int              `json:"songsWithoutLyrics"`
	TopArtists         []AnalyticsCount `json:"topArtists"`
	TopGenres          []AnalyticsCount `json:"topGenres"`
	Years              []AnalyticsCount `json:"years"`
}

// BuildLibraryAnalyticsReport computes exact counts from Navidrome records;
// the language model only receives and explains this deterministic report.
func BuildLibraryAnalyticsReport(songs model.MediaFiles, filters SearchFilters) LibraryAnalyticsReport {
	report := LibraryAnalyticsReport{TotalSongs: len(songs), AppliedFilters: filters}
	artistCounts, genreCounts, yearCounts := map[string]int{}, map[string]int{}, map[string]int{}
	artistNames, genreNames := map[string]string{}, map[string]string{}
	for _, song := range songs {
		if !songMatchesAnalyticsFilters(song, filters) {
			continue
		}
		report.MatchingSongs++
		switch explicitStatusPayload(song.ExplicitStatus) {
		case "clean":
			report.CleanSongs++
		case "explicit":
			report.ExplicitSongs++
		default:
			report.UnknownExplicit++
		}
		if songHasLyrics(song) {
			report.SongsWithLyrics++
		} else {
			report.SongsWithoutLyrics++
		}
		countAnalyticsValue(artistCounts, artistNames, song.Artist)
		genres := cleanTagValues(song.Tags.Values(model.TagGenre))
		if len(genres) == 0 && strings.TrimSpace(song.Genre) != "" {
			genres = []string{song.Genre}
		}
		for _, genre := range genres {
			countAnalyticsValue(genreCounts, genreNames, genre)
		}
		if song.Year > 0 {
			yearCounts[strconv.Itoa(song.Year)]++
		}
	}
	report.TopArtists = topAnalyticsCounts(artistCounts, artistNames, 20)
	report.TopGenres = topAnalyticsCounts(genreCounts, genreNames, 20)
	report.Years = topAnalyticsCounts(yearCounts, nil, 30)
	return report
}

func songMatchesAnalyticsFilters(song model.MediaFile, filters SearchFilters) bool {
	status := explicitStatusPayload(song.ExplicitStatus)
	if filters.Explicit != "" && status != filters.Explicit {
		return false
	}
	if (filters.YearMin != nil && song.Year < *filters.YearMin) || (filters.YearMax != nil && song.Year > *filters.YearMax) {
		return false
	}
	if (filters.BPMMin != nil && float64(song.BPM) < *filters.BPMMin) || (filters.BPMMax != nil && float64(song.BPM) > *filters.BPMMax) {
		return false
	}
	if filters.LUFSMin != nil || filters.LUFSMax != nil {
		lufs, ok := songLUFSValue(song)
		if !ok || (filters.LUFSMin != nil && lufs < *filters.LUFSMin) || (filters.LUFSMax != nil && lufs > *filters.LUFSMax) {
			return false
		}
	}
	if (filters.DurationMin != nil && float64(song.Duration) < *filters.DurationMin) || (filters.DurationMax != nil && float64(song.Duration) > *filters.DurationMax) {
		return false
	}
	if (filters.PlayCountMin != nil && song.PlayCount < *filters.PlayCountMin) || (filters.PlayCountMax != nil && song.PlayCount > *filters.PlayCountMax) {
		return false
	}
	genres := append([]string{}, song.Tags.Values(model.TagGenre)...)
	genres = append(genres, song.Genre)
	if filters.Genre != "" && !containsFold(cleanTagValues(genres), filters.Genre) {
		return false
	}
	if filters.Mood != "" && !containsFold(cleanTagValues(song.Tags.Values(model.TagMood)), filters.Mood) {
		return false
	}
	if filters.HasLyrics != nil && songHasLyrics(song) != *filters.HasLyrics {
		return false
	}
	if filters.HasGenre != nil && (strings.TrimSpace(song.Genre) != "" || len(song.Tags.Values(model.TagGenre)) > 0) != *filters.HasGenre {
		return false
	}
	if filters.HasYear != nil && (song.Year > 0) != *filters.HasYear {
		return false
	}
	if filters.HasBPM != nil && (song.BPM > 0) != *filters.HasBPM {
		return false
	}
	if filters.HasLUFS != nil {
		_, hasLUFS := songLUFSValue(song)
		if hasLUFS != *filters.HasLUFS {
			return false
		}
	}
	return true
}

func countAnalyticsValue(counts map[string]int, names map[string]string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	key := strings.ToLower(value)
	counts[key]++
	if names != nil {
		names[key] = value
	}
}

func topAnalyticsCounts(counts map[string]int, names map[string]string, limit int) []AnalyticsCount {
	items := make([]AnalyticsCount, 0, len(counts))
	for key, count := range counts {
		name := key
		if names != nil && names[key] != "" {
			name = names[key]
		}
		items = append(items, AnalyticsCount{Name: name, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Name < items[j].Name
		}
		return items[i].Count > items[j].Count
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func containsFold(values []string, expected string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(expected)) {
			return true
		}
	}
	return false
}
