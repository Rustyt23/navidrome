package nativeapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
)

const dashboardPageSize = 500

type dashboardReportBuilder func(context.Context) (rag.DashboardReport, error)

func (n *Router) handleRAGDashboardReport(w http.ResponseWriter, request *http.Request) {
	if n.ds == nil {
		writeRAGDashboardError(w, http.StatusInternalServerError, "library repository is unavailable")
		return
	}
	serveRAGDashboardReport(w, request, func(ctx context.Context) (rag.DashboardReport, error) {
		songs, err := loadDashboardSongs(n.ds.MediaFile(ctx))
		if err != nil {
			return rag.DashboardReport{}, err
		}
		playlists, err := loadDashboardPlaylists(n.ds.Playlist(ctx))
		if err != nil {
			return rag.DashboardReport{}, err
		}
		ragStatus := dashboardRAGStatus(ctx)
		return rag.BuildDashboardReport(ctx, songs, playlists, ragStatus, time.Now()), nil
	})
}

func serveRAGDashboardReport(w http.ResponseWriter, request *http.Request, build dashboardReportBuilder) {
	report, err := build(request.Context())
	if err != nil {
		writeRAGDashboardError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}

func loadDashboardSongs(repository model.MediaFileRepository) (model.MediaFiles, error) {
	all := model.MediaFiles{}
	for offset := 0; ; offset += dashboardPageSize {
		page, err := repository.GetAll(model.QueryOptions{
			Sort: "id", Order: "ASC", Max: dashboardPageSize, Offset: offset,
		})
		if err != nil {
			return nil, fmt.Errorf("could not read songs for dashboard: %w", err)
		}
		all = append(all, page...)
		if len(page) < dashboardPageSize {
			return all, nil
		}
	}
}

func loadDashboardPlaylists(repository model.PlaylistRepository) (model.Playlists, error) {
	all := model.Playlists{}
	for offset := 0; ; offset += dashboardPageSize {
		page, err := repository.GetAll(model.QueryOptions{
			Sort: "id", Order: "ASC", Max: dashboardPageSize, Offset: offset,
		})
		if err != nil {
			return nil, fmt.Errorf("could not read playlists for dashboard: %w", err)
		}
		for index := range page {
			playlist, loadErr := repository.GetWithTracks(page[index].ID, false, false)
			if loadErr != nil {
				return nil, fmt.Errorf("could not read playlist %q for dashboard: %w", page[index].ID, loadErr)
			}
			all = append(all, *playlist)
		}
		if len(page) < dashboardPageSize {
			return all, nil
		}
	}
}

func dashboardRAGStatus(ctx context.Context) rag.DashboardRAGStatus {
	status := rag.DashboardRAGStatus{Enabled: ragEnabled()}
	if !status.Enabled {
		status.Error = "RAG is disabled"
		return status
	}
	client := newRAGQdrantClient()
	qdrantStatus := client.Status(ctx, false)
	status.VectorDBOnline = qdrantStatus.VectorDBOnline
	status.CollectionExists = qdrantStatus.CollectionExists
	status.Error = qdrantStatus.Error
	if !status.VectorDBOnline || !status.CollectionExists || status.Error != "" {
		return status
	}
	count, err := client.CountDocumentsByType(ctx, "song")
	if err != nil {
		status.Error = fmt.Sprintf("Could not count indexed songs: %v", err)
		return status
	}
	status.IndexedSongs = count
	return status
}

func writeRAGDashboardError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
