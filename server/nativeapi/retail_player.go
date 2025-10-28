package nativeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/utils"
)

const (
	retailPlayerDefaultKeyHeader = "x-retailplayer-apikey"
	retailPlayerLegacyKeyHeader  = "X-API-Key"
)

var retailPlayerHTTPClient = &http.Client{Timeout: 15 * time.Second}

type retailPlayerAPIDevice struct {
	Ordinal      *int   `json:"ordinal"`
	ID           string `json:"id"`
	Name         string `json:"name"`
	Location     string `json:"location"`
	OrgUnit      string `json:"orgUnit"`
	Organization string `json:"organization"`
	Channel      string `json:"channel"`
	ChannelList  string `json:"channelList"`
	MacAddress   string `json:"macAddress"`
	TimeZone     string `json:"timeZone"`
}

type retailPlayerAPIResponse struct {
	Data  []retailPlayerAPIDevice `json:"data"`
	Page  *int                    `json:"page"`
	Total *int                    `json:"total"`
}

type retailPlayerDevice struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Channel      string `json:"channel"`
	ChannelList  string `json:"channelList"`
	Organization string `json:"organization"`
	TimeZone     string `json:"timeZone,omitempty"`
}

type retailPlayerDevicesResponse struct {
	Data  []retailPlayerDevice `json:"data"`
	Page  *int                 `json:"page,omitempty"`
	Total *int                 `json:"total,omitempty"`
}

type retailPlayerConfig struct {
	BaseURL           string
	OrgID             string
	APIKey            string
	APIKeyHeader      string
	PageSize          int
	Page              int
	Filters           string
	OrderBy           string
	OrderDirection    string
	Search            string
	Fields            []string
	AdditionalHeaders map[string]string
}

func (n *Router) addRetailPlayerRoute(r chi.Router) {
	r.Route("/retailplayer", func(r chi.Router) {
		r.Get("/devices", n.handleRetailPlayerDevices())
		r.Get("/devices/{deviceID}/status", n.handleRetailPlayerDeviceStatus())
	})
}

var errRetailPlayerDeviceNotFound = errors.New("retail player device not found")

func (n *Router) handleRetailPlayerDevices() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if !conf.Server.RetailPlayer.Enabled {
			http.Error(w, "Retail player integration disabled", http.StatusNotFound)
			return
		}

		log.Info(ctx, "Fetching retail player devices from remote API")
		response, err := fetchRetailPlayerDevices(ctx)
		if err != nil {
			log.Error(ctx, "Unable to fetch retail player devices", "err", err)
			http.Error(w, "Unable to fetch retail player devices", http.StatusBadGateway)
			return
		}

		log.Info(ctx, "Retail player devices fetched", "count", len(response.Data))

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Error(ctx, "Unable to encode retail player devices response", "err", err)
		}
	}
}

func (n *Router) handleRetailPlayerDeviceStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if !conf.Server.RetailPlayer.Enabled {
			http.Error(w, "Retail player integration disabled", http.StatusNotFound)
			return
		}

		deviceID := strings.TrimSpace(chi.URLParam(r, "deviceID"))
		if deviceID == "" {
			http.Error(w, "Retail player device id is required", http.StatusBadRequest)
			return
		}

		log.Info(ctx, "Fetching retail player device status from remote API", "deviceID", deviceID)
		response, err := n.fetchRetailPlayerDeviceStatus(ctx, deviceID)
		if err != nil {
			if errors.Is(err, errRetailPlayerDeviceNotFound) {
				log.Info(ctx, "Retail player device not found", "deviceID", deviceID)
				http.Error(w, "Retail player device not found", http.StatusNotFound)
				return
			}

			log.Error(ctx, "Unable to fetch retail player device status", "deviceID", deviceID, "err", err)
			http.Error(w, "Unable to fetch retail player device status", http.StatusBadGateway)
			return
		}

		log.Info(ctx, "Retail player device status fetched", "deviceID", deviceID)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Error(ctx, "Unable to encode retail player device status response", "deviceID", deviceID, "err", err)
		}
	}
}

func fetchRetailPlayerDevices(ctx context.Context) (retailPlayerDevicesResponse, error) {
	cfg := conf.Server.RetailPlayer
	if cfg.BaseURL == "" || cfg.OrgID == "" {
		return retailPlayerDevicesResponse{}, errors.New("retail player API not configured")
	}

	requestConfig := retailPlayerConfig{
		BaseURL:           cfg.BaseURL,
		OrgID:             cfg.OrgID,
		APIKey:            cfg.APIKey,
		APIKeyHeader:      cfg.APIKeyHeader,
		PageSize:          cfg.PageSize,
		Page:              cfg.Page,
		Filters:           cfg.Filters,
		OrderBy:           cfg.OrderBy,
		OrderDirection:    cfg.OrderDirection,
		Search:            cfg.Search,
		Fields:            cfg.Fields,
		AdditionalHeaders: cfg.AdditionalHeaders,
	}

	req, err := buildRetailPlayerRequest(ctx, requestConfig)
	if err != nil {
		return retailPlayerDevicesResponse{}, err
	}

	resp, err := retailPlayerHTTPClient.Do(req)
	if err != nil {
		return retailPlayerDevicesResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return retailPlayerDevicesResponse{}, fmt.Errorf("retail player API request failed with status %d", resp.StatusCode)
	}

	var apiPayload retailPlayerAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiPayload); err != nil {
		return retailPlayerDevicesResponse{}, err
	}

	devices := make([]retailPlayerDevice, 0, len(apiPayload.Data))
	for _, item := range apiPayload.Data {
		if device, ok := simplifyRetailPlayerDevice(item); ok {
			devices = append(devices, device)
		}
	}

	return retailPlayerDevicesResponse{
		Data:  devices,
		Page:  apiPayload.Page,
		Total: apiPayload.Total,
	}, nil
}

type retailPlayerDeviceStatusResponse struct {
	Status         map[string]any             `json:"status"`
	StreamMetadata []map[string]any           `json:"streamMetadata"`
	Artwork        *retailPlayerStatusArtwork `json:"artwork,omitempty"`
}

type retailPlayerStatusArtwork struct {
	MediaFileID string `json:"mediaFileId,omitempty"`
	ArtworkID   string `json:"artworkId,omitempty"`
}

func (n *Router) fetchRetailPlayerDeviceStatus(ctx context.Context, deviceID string) (retailPlayerDeviceStatusResponse, error) {
	cfg := conf.Server.RetailPlayer
	if cfg.BaseURL == "" || cfg.OrgID == "" {
		return retailPlayerDeviceStatusResponse{}, errors.New("retail player API not configured")
	}

	trimmedID := strings.TrimSpace(deviceID)
	if trimmedID == "" {
		return retailPlayerDeviceStatusResponse{}, errors.New("retail player device id is empty")
	}

	requestConfig := retailPlayerConfig{
		BaseURL:           cfg.BaseURL,
		OrgID:             cfg.OrgID,
		APIKey:            cfg.APIKey,
		APIKeyHeader:      cfg.APIKeyHeader,
		AdditionalHeaders: cfg.AdditionalHeaders,
	}

	req, err := buildRetailPlayerRequest(ctx, requestConfig, trimmedID, "status")
	if err != nil {
		return retailPlayerDeviceStatusResponse{}, err
	}

	resp, err := retailPlayerHTTPClient.Do(req)
	if err != nil {
		return retailPlayerDeviceStatusResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return retailPlayerDeviceStatusResponse{}, errRetailPlayerDeviceNotFound
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return retailPlayerDeviceStatusResponse{}, fmt.Errorf("retail player API request failed with status %d", resp.StatusCode)
	}

	var payload retailPlayerDeviceStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return retailPlayerDeviceStatusResponse{}, err
	}

	n.populateRetailPlayerStatusArtwork(ctx, &payload)

	return payload, nil
}

func (n *Router) populateRetailPlayerStatusArtwork(ctx context.Context, payload *retailPlayerDeviceStatusResponse) {
	if payload == nil {
		return
	}

	streamName := normalizeStatusString(payload.Status, "activeStreamName")
	if streamName == "" {
		streamName = normalizeStatusString(payload.Status, "activeStream")
	}
	if streamName == "" {
		return
	}

	cleaned := strings.ReplaceAll(streamName, "\\", "/")
	baseName := utils.BaseName(cleaned)
	if baseName == "" {
		baseName = strings.TrimSpace(streamName)
	}
	if baseName == "" {
		return
	}

	repo := n.ds.MediaFile(ctx)
	if repo == nil {
		return
	}

	queries := buildStreamSearchQueries(baseName)
	for _, query := range queries {
		if query == "" {
			continue
		}

		files, err := repo.Search(query, 0, 5)
		if err != nil {
			log.Debug(ctx, "Retail player artwork search failed", "query", query, "err", err)
			continue
		}
		if len(files) == 0 {
			continue
		}

		matched := selectBestMediaFileMatch(baseName, files)
		if matched == nil {
			continue
		}

		coverArtID := matched.CoverArtID().String()
		if coverArtID == "" {
			continue
		}

		payload.Artwork = &retailPlayerStatusArtwork{
			MediaFileID: matched.ID,
			ArtworkID:   coverArtID,
		}
		return
	}
}

func normalizeStatusString(status map[string]any, key string) string {
	if status == nil {
		return ""
	}

	value := status[key]
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case []byte:
		return strings.TrimSpace(string(v))
	default:
		return ""
	}
}

func buildStreamSearchQueries(baseName string) []string {
	trimmed := strings.TrimSpace(baseName)
	if trimmed == "" {
		return nil
	}

	queries := []string{trimmed}

	if replaced := strings.ReplaceAll(trimmed, "_", " "); replaced != trimmed {
		queries = append(queries, replaced)
	}

	if ext := path.Ext(trimmed); ext != "" {
		withoutExt := strings.TrimSuffix(trimmed, ext)
		if withoutExt != "" {
			queries = append(queries, withoutExt)
			if replaced := strings.ReplaceAll(withoutExt, "_", " "); replaced != withoutExt {
				queries = append(queries, replaced)
			}
		}
	}

	parts := strings.Split(trimmed, " - ")
	if len(parts) > 1 {
		title := strings.TrimSpace(strings.Join(parts[1:], " - "))
		if title != "" {
			queries = append(queries, title)
		}
	}

	return uniqueStrings(queries)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if _, exists := seen[lower]; exists {
			continue
		}
		seen[lower] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func selectBestMediaFileMatch(baseName string, files model.MediaFiles) *model.MediaFile {
	if len(files) == 0 {
		return nil
	}

	trimmedBase := strings.TrimSpace(baseName)
	lowerBase := strings.ToLower(trimmedBase)
	if lowerBase != "" {
		for _, file := range files {
			fileBase := strings.ToLower(utils.BaseName(file.Path))
			if fileBase == lowerBase {
				return &file
			}
			title := strings.TrimSpace(file.Title)
			if strings.EqualFold(title, trimmedBase) {
				return &file
			}
			if title != "" && strings.Contains(lowerBase, strings.ToLower(title)) {
				return &file
			}
		}
	}

	return &files[0]
}

func buildRetailPlayerRequest(ctx context.Context, cfg retailPlayerConfig, pathParts ...string) (*http.Request, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		return nil, errors.New("retail player base URL is empty")
	}

	endpoint := fmt.Sprintf("%s/orgs/%s/devices", baseURL, url.PathEscape(cfg.OrgID))
	for _, part := range pathParts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		endpoint = fmt.Sprintf("%s/%s", endpoint, url.PathEscape(trimmed))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	if len(pathParts) == 0 {
		query := req.URL.Query()
		if cfg.PageSize > 0 {
			query.Set("pageSize", strconv.Itoa(cfg.PageSize))
		}
		if cfg.Page > 0 {
			query.Set("page", strconv.Itoa(cfg.Page))
		}
		if cfg.Filters != "" {
			query.Set("filters", cfg.Filters)
		}
		if cfg.OrderBy != "" {
			query.Set("orderBy", cfg.OrderBy)
		}
		if cfg.OrderDirection != "" {
			query.Set("orderDirection", cfg.OrderDirection)
		}
		if cfg.Search != "" {
			query.Set("search", cfg.Search)
		}
		for _, field := range cfg.Fields {
			trimmed := strings.TrimSpace(field)
			if trimmed != "" {
				query.Add("fields", trimmed)
			}
		}
		req.URL.RawQuery = query.Encode()
	}

	req.Header.Set("Accept", "application/json")

	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey != "" {
		headerName := strings.TrimSpace(cfg.APIKeyHeader)
		if headerName == "" {
			headerName = retailPlayerDefaultKeyHeader
		}

		req.Header.Set(headerName, apiKey)

		if !strings.EqualFold(headerName, retailPlayerDefaultKeyHeader) {
			req.Header.Set(retailPlayerDefaultKeyHeader, apiKey)
		}
		if !strings.EqualFold(headerName, retailPlayerLegacyKeyHeader) {
			req.Header.Set(retailPlayerLegacyKeyHeader, apiKey)
		}
	}

	for key, value := range cfg.AdditionalHeaders {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey != "" && trimmedValue != "" {
			req.Header.Set(trimmedKey, trimmedValue)
		}
	}

	return req, nil
}

func simplifyRetailPlayerDevice(device retailPlayerAPIDevice) (retailPlayerDevice, bool) {
	id := strings.TrimSpace(device.ID)
	if id == "" {
		id = strings.TrimSpace(device.MacAddress)
	}
	if id == "" && device.Ordinal != nil {
		id = strconv.Itoa(*device.Ordinal)
	}
	if id == "" {
		return retailPlayerDevice{}, false
	}

	name := strings.TrimSpace(device.Name)
	if name == "" {
		name = id
	}

	organization := firstNonEmpty(
		strings.TrimSpace(device.Organization),
		strings.TrimSpace(device.OrgUnit),
		strings.TrimSpace(device.Location),
	)

	return retailPlayerDevice{
		ID:           id,
		Name:         name,
		Channel:      strings.TrimSpace(device.Channel),
		ChannelList:  strings.TrimSpace(device.ChannelList),
		Organization: organization,
		TimeZone:     strings.TrimSpace(device.TimeZone),
	}, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
