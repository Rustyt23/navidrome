package rag

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/navidrome/navidrome/model"
)

type DashboardRAGStatus struct {
	Enabled          bool
	VectorDBOnline   bool
	CollectionExists bool
	IndexedSongs     int64
	Error            string
}

// DashboardReport is a read-only snapshot of library, playlist, RAG, and usage
// health. Additional RAG state fields support useful empty/offline UI states.
type DashboardReport struct {
	LibraryHealthScore       int     `json:"libraryHealthScore"`
	TotalSongs               int     `json:"totalSongs"`
	IndexedSongs             int64   `json:"indexedSongs"`
	IndexedCoveragePercent   float64 `json:"indexedCoveragePercent"`
	SongsWithLyrics          int     `json:"songsWithLyrics"`
	SongsMissingLyrics       int     `json:"songsMissingLyrics"`
	SongsWithGenre           int     `json:"songsWithGenre"`
	SongsMissingGenre        int     `json:"songsMissingGenre"`
	SongsWithYear            int     `json:"songsWithYear"`
	SongsMissingYear         int     `json:"songsMissingYear"`
	SongsWithBPM             int     `json:"songsWithBpm"`
	SongsMissingBPM          int     `json:"songsMissingBpm"`
	SongsWithLUFS            int     `json:"songsWithLufs"`
	SongsMissingLUFS         int     `json:"songsMissingLufs"`
	CleanSongs               int     `json:"cleanSongs"`
	ExplicitSongs            int     `json:"explicitSongs"`
	ReviewNeededSongs        int     `json:"reviewNeededSongs"`
	ExplicitRiskSongs        int     `json:"explicitRiskSongs"`
	DuplicateRiskCount       int     `json:"duplicateRiskCount"`
	LoudnessIssueCount       int     `json:"loudnessIssueCount"`
	OverplayedSongs          int     `json:"overplayedSongs"`
	UnderusedSongs           int     `json:"underusedSongs"`
	PlaylistsTotal           int     `json:"playlistsTotal"`
	PlaylistsAnalyzed        int     `json:"playlistsAnalyzed"`
	PlaylistsWithIssues      int     `json:"playlistsWithIssues"`
	RecommendationsAvailable int     `json:"recommendationsAvailable"`
	RAGEnabled               bool    `json:"ragEnabled"`
	VectorDBOnline           bool    `json:"vectorDbOnline"`
	CollectionExists         bool    `json:"collectionExists"`
	RAGError                 string  `json:"ragError,omitempty"`
}

// BuildDashboardReport performs deterministic read-only calculations over
// repository snapshots and a Qdrant status snapshot.
func BuildDashboardReport(
	ctx context.Context,
	songs model.MediaFiles,
	playlists model.Playlists,
	ragStatus DashboardRAGStatus,
	now time.Time,
) DashboardReport {
	report := DashboardReport{
		TotalSongs: len(songs), PlaylistsTotal: len(playlists),
		RAGEnabled: ragStatus.Enabled, VectorDBOnline: ragStatus.VectorDBOnline,
		CollectionExists: ragStatus.CollectionExists, RAGError: ragStatus.Error,
	}
	if ragStatus.Enabled && ragStatus.VectorDBOnline && ragStatus.CollectionExists && ragStatus.IndexedSongs > 0 {
		report.IndexedSongs = ragStatus.IndexedSongs
		if report.IndexedSongs > int64(report.TotalSongs) {
			report.IndexedSongs = int64(report.TotalSongs)
		}
	}
	if report.TotalSongs > 0 {
		report.IndexedCoveragePercent = percent(int(report.IndexedSongs), report.TotalSongs)
	}

	for _, song := range songs {
		hasLyrics := songHasLyrics(song)
		hasGenre := strings.TrimSpace(song.Genre) != "" || len(cleanTagValues(song.Tags.Values(model.TagGenre))) > 0
		hasYear := song.Year > 0
		hasBPM := song.BPM > 0
		_, hasLUFS := songLUFSValue(song)
		if hasLyrics {
			report.SongsWithLyrics++
		} else {
			report.SongsMissingLyrics++
		}
		if hasGenre {
			report.SongsWithGenre++
		} else {
			report.SongsMissingGenre++
		}
		if hasYear {
			report.SongsWithYear++
		} else {
			report.SongsMissingYear++
		}
		if hasBPM {
			report.SongsWithBPM++
		} else {
			report.SongsMissingBPM++
		}
		if hasLUFS {
			report.SongsWithLUFS++
		} else {
			report.SongsMissingLUFS++
		}

		status := explicitStatusPayload(song.ExplicitStatus)
		switch status {
		case "clean":
			report.CleanSongs++
		case "explicit":
			report.ExplicitSongs++
			report.ExplicitRiskSongs++
		}
		if status == "unknown" || !hasLyrics || !hasGenre || !hasYear || !hasBPM || !hasLUFS {
			report.ReviewNeededSongs++
		}
		metadataComplete := hasLyrics && hasGenre && hasYear && hasBPM && hasLUFS
		if status == "clean" && metadataComplete && song.PlayCount <= 20 {
			report.UnderusedSongs++
		}
		recent := song.PlayDate != nil && song.PlayDate.After(now.AddDate(0, 0, -30))
		if song.PlayCount >= 50 || recent {
			report.OverplayedSongs++
		}
	}

	badFitCount := 0
	for index := range playlists {
		analysis := AnalyzePlaylist(ctx, &playlists[index], nil)
		report.PlaylistsAnalyzed++
		for _, duplicate := range analysis.DuplicateSongs {
			report.DuplicateRiskCount += max(duplicate.Count-1, 0)
		}
		report.LoudnessIssueCount += len(analysis.LoudnessIssues)
		badFitCount += len(analysis.BadFitSongs)
		if analysis.ExplicitRisk.Count > 0 || len(analysis.MetadataIssues) > 0 ||
			len(analysis.DuplicateSongs) > 0 || len(analysis.LoudnessIssues) > 0 || len(analysis.BadFitSongs) > 0 {
			report.PlaylistsWithIssues++
		}
	}
	report.RecommendationsAvailable = report.UnderusedSongs + report.ExplicitRiskSongs + badFitCount
	report.LibraryHealthScore = dashboardHealthScore(report)
	return report
}

func dashboardHealthScore(report DashboardReport) int {
	if report.TotalSongs == 0 {
		return 0
	}
	metadata := average(
		percent(report.SongsWithLyrics, report.TotalSongs),
		percent(report.SongsWithGenre, report.TotalSongs),
		percent(report.SongsWithYear, report.TotalSongs),
		percent(report.SongsWithBPM, report.TotalSongs),
		percent(report.SongsWithLUFS, report.TotalSongs),
	)
	safety := 100 - percent(report.ExplicitSongs+report.ReviewNeededSongs, report.TotalSongs)
	playlistQuality := 100.0
	if report.PlaylistsTotal > 0 {
		playlistQuality = 100 - percent(report.PlaylistsWithIssues, report.PlaylistsTotal)
	}
	score := metadata*0.5 + report.IndexedCoveragePercent*0.2 + max(safety, 0)*0.15 + max(playlistQuality, 0)*0.15
	return int(math.Round(min(max(score, 0), 100)))
}

func percent(value, total int) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(float64(value)*1000/float64(total)) / 10
}

func average(values ...float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}
