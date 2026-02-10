package nativeapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server/public"
	"github.com/navidrome/navidrome/utils"
)

const (
	retailPlayerDefaultKeyHeader              = "x-retailplayer-apikey"
	retailPlayerLegacyKeyHeader               = "X-API-Key"
	retailPlayerRemoteControlDefaultKeyHeader = "x-retailplayer-rc-apikey"
)

var retailPlayerHTTPClient = &http.Client{Timeout: 15 * time.Second}

type retailPlayerDeviceResolver struct {
	mu      sync.RWMutex
	devices map[string]retailPlayerDevice
}

func newRetailPlayerDeviceResolver() *retailPlayerDeviceResolver {
	return &retailPlayerDeviceResolver{devices: make(map[string]retailPlayerDevice)}
}

func (r *retailPlayerDeviceResolver) Remember(device retailPlayerDevice) {
	if r == nil {
		return
	}
	r.RememberDevices([]retailPlayerDevice{device})
}

func (r *retailPlayerDeviceResolver) RememberDevices(devices []retailPlayerDevice) {
	if r == nil || len(devices) == 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.devices == nil {
		r.devices = make(map[string]retailPlayerDevice, len(devices)*4)
	}

	for _, device := range devices {
		trimmedID := strings.TrimSpace(device.ID)
		if trimmedID == "" {
			continue
		}

		keys := make(map[string]struct{})
		addKey := func(key string) {
			if key != "" {
				keys[key] = struct{}{}
			}
		}

		addKey(makeRetailPlayerExactKey(trimmedID))
		addKey(makeRetailPlayerNormalizedKey(trimmedID))
		addKey(makeRetailPlayerSlugKey(trimmedID))

		candidates := []string{device.Name, device.Channel, device.ChannelList, device.MacAddress, device.Organization}
		for _, candidate := range candidates {
			trimmed := strings.TrimSpace(candidate)
			if trimmed == "" {
				continue
			}
			addKey(makeRetailPlayerExactKey(trimmed))
			addKey(makeRetailPlayerNormalizedKey(trimmed))
			addKey(makeRetailPlayerSlugKey(trimmed))
		}

		for key := range keys {
			r.devices[key] = device
		}
	}
}

func (r *retailPlayerDeviceResolver) Find(identifier string) (retailPlayerDevice, bool) {
	if r == nil {
		return retailPlayerDevice{}, false
	}

	trimmed := strings.TrimSpace(identifier)
	if trimmed == "" {
		return retailPlayerDevice{}, false
	}

	keys := []string{
		makeRetailPlayerExactKey(trimmed),
		makeRetailPlayerNormalizedKey(trimmed),
		makeRetailPlayerSlugKey(trimmed),
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, key := range keys {
		if key == "" {
			continue
		}
		if device, ok := r.devices[key]; ok {
			return device, true
		}
	}

	return retailPlayerDevice{}, false
}

func makeRetailPlayerExactKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return "exact:" + trimmed
}

func makeRetailPlayerNormalizedKey(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return ""
	}
	return "norm:" + normalized
}

func makeRetailPlayerSlugKey(value string) string {
	slug := retailPlayerDeviceSlugKey(value)
	if slug == "" {
		return ""
	}
	return "slug:" + slug
}

func normalizeRetailPlayerIdentifier(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	decoded := trimmed
	for i := 0; i < 5; i++ {
		unescaped, err := url.PathUnescape(decoded)
		if err != nil {
			break
		}
		if unescaped == decoded {
			break
		}
		decoded = unescaped
	}

	return strings.TrimSpace(decoded)
}

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
	MacAddressV1 string `json:"mac_address"`
	TimeZone     string `json:"timeZone"`
	Online       *bool  `json:"online"`
}

type retailPlayerAPIResponse struct {
	Data  []retailPlayerAPIDevice `json:"data"`
	Page  *int                    `json:"page"`
	Total *int                    `json:"total"`
}

type retailPlayerChannelListAPIResponse struct {
	Channels []retailPlayerAPIChannel `json:"channels"`
}

type retailPlayerChannelsAPIResponse struct {
	Data []retailPlayerAPIChannel `json:"data"`
}

type retailPlayerDevice struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	IsLocked        bool     `json:"isLocked"`
	OrganizationID  string   `json:"organizationId,omitempty"`
	OrganisationID  string   `json:"organisationid,omitempty"`
	Channel         string   `json:"channel"`
	ChannelName     string   `json:"channelName,omitempty"`
	ChannelList     string   `json:"channelList"`
	MacAddress      string   `json:"macAddress,omitempty"`
	Organization    string   `json:"organization"`
	TimeZone        string   `json:"timeZone,omitempty"`
	Online          *bool    `json:"online,omitempty"`
	FolderIDs       []string `json:"folderIds,omitempty"`
	RemoteControlID string   `json:"remoteControlId,omitempty"`
}

type retailPlayerDeviceConfigResponse struct {
	ID                  string `json:"id"`
	OrgUnit             string `json:"orgUnit"`
	Organization        string `json:"organization"`
	Name                string `json:"name"`
	Channel             string `json:"channel"`
	ChannelList         string `json:"channelList"`
	OrgButtonTriggerSet string `json:"orgButtonTriggerSet"`
	InstalledFirmware   string `json:"installedFirmwareVersion"`
}

type retailPlayerDevicesResponse struct {
	Data         []retailPlayerDevice       `json:"data"`
	Folders      []retailPlayerFolder       `json:"folders,omitempty"`
	DeviceFolder []retailPlayerDeviceFolder `json:"deviceFolders,omitempty"`
	Page         *int                       `json:"page,omitempty"`
	Total        *int                       `json:"total,omitempty"`
}

type retailPlayerFolder struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ParentID  *string   `json:"parentId,omitempty"`
	IsLocked  bool      `json:"isLocked"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type retailPlayerDeviceFolder struct {
	DeviceID  string    `json:"deviceId"`
	FolderID  string    `json:"folderId"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type retailPlayerFolderPayload struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	ParentID *string `json:"parentId"`
	IsLocked *bool   `json:"isLocked"`
}

type retailPlayerDeleteFoldersRequest struct {
	FolderIDs []string `json:"folderIds"`
}

type retailPlayerAssignDeviceFoldersRequest struct {
	FolderIDs []string `json:"folderIds"`
}

type retailPlayerAPIChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type retailPlayerChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type retailPlayerChannelsResponse struct {
	Channels []retailPlayerChannel `json:"channels"`
}

type retailPlayerTriggerAsset struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	MIME string `json:"mime"`
}

type retailPlayerTrigger struct {
	ID      string                    `json:"id"`
	Name    string                    `json:"name"`
	Ordinal int                       `json:"ordinal"`
	Asset   *retailPlayerTriggerAsset `json:"asset,omitempty"`
}

type retailPlayerTriggerAPIResponse struct {
	Value []retailPlayerTrigger `json:"value"`
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

type retailPlayerRemoteControlConfig struct {
	BaseURL           string
	APIKey            string
	APIKeyHeader      string
	AdditionalHeaders map[string]string
}

type retailPlayerDependentsResponse struct {
	RemoteControls []retailPlayerRemoteControl `json:"remoteControls"`
}

type retailPlayerRemoteControl struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type retailPlayerQRSyncResult struct {
	DeviceID        string `json:"deviceId"`
	DeviceName      string `json:"deviceName"`
	RemoteControlID string `json:"remoteControlId"`
	Created         bool   `json:"created"`
	Error           string `json:"error,omitempty"`
}

type retailPlayerQRDevice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (n *Router) addRetailPlayerPublicRoutes(r chi.Router) {
	r.Route("/retailplayer", func(r chi.Router) {
		r.Get("/devices/{deviceID}/config", n.handleRetailPlayerDeviceConfig())
		r.Get("/devices/{deviceID}/status", n.handleRetailPlayerDeviceStatus())
		r.Get("/devices/{deviceID}/triggers", n.handleRetailPlayerDeviceTriggers())
		r.Post("/devices/{deviceID}/triggers", n.handleRetailPlayerDeviceTriggerAction())
		r.Get("/channel-lists/{channelListID}/channels", n.handleRetailPlayerChannelListChannels())
		r.Post("/devices/{deviceID}/volume", n.handleRetailPlayerDeviceVolume())
		r.Post("/devices/{deviceID}/channel", n.handleRetailPlayerDeviceChannel())
		r.Post("/devices/{deviceID}/channel/toggle", n.handleRetailPlayerDeviceToggleChannel())
		r.Post("/devices/{deviceID}/dislike", n.handleRetailPlayerDeviceDislike())
		r.Post("/rc", n.handleRetailPlayerDeviceByName())
	})
}

func (n *Router) addRetailPlayerPrivateRoutes(r chi.Router) {
	r.Get("/retailplayer/devices", n.handleRetailPlayerDevices())
	r.Patch("/retailplayer/devices/{deviceID}/lock", n.handleUpdateRetailPlayerDeviceLock())
	r.Post("/retailplayer/folders", n.handleCreateRetailPlayerFolder())
	r.Patch("/retailplayer/folders/{folderID}", n.handleUpdateRetailPlayerFolder())
	r.Post("/retailplayer/folders/delete", n.handleDeleteRetailPlayerFolders())
	r.Put("/retailplayer/devices/{deviceID}/folders", n.handleAssignRetailPlayerDeviceFolders())
	r.Post("/retailplayer/qr", n.handleRetailPlayerSyncQR())
	r.Patch("/retailplayer/devices/{deviceID}/remote-control", n.handleUpdateRetailPlayerDeviceRemoteControl())
}

func (n *Router) handleUpdateRetailPlayerDeviceLock() http.HandlerFunc {
	type lockUpdatePayload struct {
		Locked bool `json:"locked"`
	}

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

		var payload lockUpdatePayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid retail player lock payload", http.StatusBadRequest)
			return
		}

		repo := n.ds.RetailPlayerDeviceMapping(ctx)
		if repo == nil {
			http.Error(w, "Retail player device mapping repository not available", http.StatusInternalServerError)
			return
		}

		if err := repo.SetLocked(ctx, deviceID, payload.Locked); err != nil {
			log.Error(ctx, "Unable to update retail player device lock", "deviceID", deviceID, "err", err)
			http.Error(w, "Unable to update retail player device lock", http.StatusInternalServerError)
			return
		}

		mapping, err := repo.FindByIdentifier(ctx, deviceID)
		if err != nil {
			log.Error(ctx, "Unable to load retail player device mapping after lock update", "deviceID", deviceID, "err", err)
			http.Error(w, "Unable to load retail player device", http.StatusInternalServerError)
			return
		}

		writeRetailPlayerJSON(ctx, w, http.StatusOK, map[string]any{"data": mapRetailPlayerMappingToDevice(*mapping)})
	}
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

		if len(response.Data) > 0 {
			if err := n.applyRetailPlayerChannelNames(ctx, response.Data); err != nil {
				log.Warn(ctx, "Unable to fetch retail player channels", "err", err)
			}
		}

		n.devices.RememberDevices(response.Data)
		n.persistRetailPlayerDeviceMappings(ctx, response.Data)

		folders, deviceFolders, err := n.loadRetailPlayerFolderData(ctx)
		if err != nil {
			log.Error(ctx, "Unable to load retail player folder data", "err", err)
			http.Error(w, "Unable to load retail player folders", http.StatusInternalServerError)
			return
		}

		if len(deviceFolders) > 0 {
			folderSet := make(map[string]struct{}, len(folders))
			for _, folder := range folders {
				folderSet[folder.ID] = struct{}{}
			}

			assignments := make(map[string][]string)
			for _, deviceFolder := range deviceFolders {
				if _, ok := folderSet[deviceFolder.FolderID]; !ok {
					continue
				}

				assignments[deviceFolder.DeviceID] = append(assignments[deviceFolder.DeviceID], deviceFolder.FolderID)
			}

			for index := range response.Data {
				id := strings.TrimSpace(response.Data[index].ID)
				if id == "" {
					continue
				}
				if folderIDs, ok := assignments[id]; ok {
					response.Data[index].FolderIDs = append([]string(nil), folderIDs...)
				}
			}
		}

		if repo := n.ds.RetailPlayerDeviceMapping(ctx); repo != nil {
			mappings, err := repo.All(ctx)
			if err != nil && !errors.Is(err, model.ErrNotFound) {
				log.Warn(ctx, "Unable to load retail player device mappings", "err", err)
			}

			if len(mappings) > 0 {
				remoteControlByID := make(map[string]string, len(mappings))
				for _, mapping := range mappings {
					id := strings.TrimSpace(mapping.DeviceID)
					remoteControlID := strings.TrimSpace(mapping.RemoteCtrlID)
					if id == "" {
						continue
					}
					if remoteControlID != "" {
						remoteControlByID[id] = remoteControlID
					}
					for index := range response.Data {
						if strings.TrimSpace(response.Data[index].ID) == id {
							response.Data[index].IsLocked = mapping.IsLocked
							break
						}
					}
				}

				if len(remoteControlByID) > 0 {
					for index := range response.Data {
						id := strings.TrimSpace(response.Data[index].ID)
						if id == "" {
							continue
						}
						if remoteControlID, ok := remoteControlByID[id]; ok {
							response.Data[index].RemoteControlID = remoteControlID
						}
					}
				}
			}
		}

		response.Folders = make([]retailPlayerFolder, 0, len(folders))
		for _, folder := range folders {
			response.Folders = append(response.Folders, mapModelRetailPlayerFolder(folder))
		}

		response.DeviceFolder = make([]retailPlayerDeviceFolder, 0, len(deviceFolders))
		for _, deviceFolder := range deviceFolders {
			response.DeviceFolder = append(response.DeviceFolder, mapModelRetailPlayerDeviceFolder(deviceFolder))
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Error(ctx, "Unable to encode retail player devices response", "err", err)
		}
	}
}

func (n *Router) handleRetailPlayerDeviceByName() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if !conf.Server.RetailPlayer.Enabled {
			http.Error(w, "Retail player integration disabled", http.StatusNotFound)
			return
		}

		var payload struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid retail player device payload", http.StatusBadRequest)
			return
		}

		deviceName := strings.TrimSpace(payload.Name)
		if deviceName == "" {
			http.Error(w, "Retail player device name is required", http.StatusBadRequest)
			return
		}

		log.Info(ctx, "Fetching retail player device by name from remote API", "name", deviceName)
		response, err := fetchRetailPlayerDevices(ctx)
		if err != nil {
			log.Error(ctx, "Unable to fetch retail player devices", "err", err)
			http.Error(w, "Unable to fetch retail player devices", http.StatusBadGateway)
			return
		}

		if len(response.Data) > 0 {
			if err := n.applyRetailPlayerChannelNames(ctx, response.Data); err != nil {
				log.Warn(ctx, "Unable to fetch retail player channels", "err", err)
			}
		}

		filtered := retailPlayerDevicesResponse{
			Data: make([]retailPlayerDevice, 0, 1),
		}
		for _, device := range response.Data {
			if strings.EqualFold(strings.TrimSpace(device.Name), deviceName) {
				filtered.Data = append(filtered.Data, device)
				break
			}
		}

		if len(filtered.Data) == 0 {
			http.Error(w, "Retail player device not found", http.StatusNotFound)
			return
		}

		n.devices.RememberDevices(filtered.Data)
		n.persistRetailPlayerDeviceMappings(ctx, filtered.Data)

		folders, deviceFolders, err := n.loadRetailPlayerFolderData(ctx)
		if err != nil {
			log.Error(ctx, "Unable to load retail player folder data", "err", err)
			http.Error(w, "Unable to load retail player folders", http.StatusInternalServerError)
			return
		}

		if len(deviceFolders) > 0 {
			folderSet := make(map[string]struct{}, len(folders))
			for _, folder := range folders {
				folderSet[folder.ID] = struct{}{}
			}

			assignments := make(map[string][]string)
			for _, deviceFolder := range deviceFolders {
				if _, ok := folderSet[deviceFolder.FolderID]; !ok {
					continue
				}

				assignments[deviceFolder.DeviceID] = append(assignments[deviceFolder.DeviceID], deviceFolder.FolderID)
			}

			for index := range filtered.Data {
				id := strings.TrimSpace(filtered.Data[index].ID)
				if id == "" {
					continue
				}
				if folderIDs, ok := assignments[id]; ok {
					filtered.Data[index].FolderIDs = append([]string(nil), folderIDs...)
				}
			}
		}

		if repo := n.ds.RetailPlayerDeviceMapping(ctx); repo != nil {
			mappings, err := repo.All(ctx)
			if err != nil && !errors.Is(err, model.ErrNotFound) {
				log.Warn(ctx, "Unable to load retail player device mappings", "err", err)
			}

			if len(mappings) > 0 {
				remoteControlByID := make(map[string]string, len(mappings))
				for _, mapping := range mappings {
					id := strings.TrimSpace(mapping.DeviceID)
					remoteControlID := strings.TrimSpace(mapping.RemoteCtrlID)
					if id == "" {
						continue
					}
					if remoteControlID != "" {
						remoteControlByID[id] = remoteControlID
					}
					for index := range filtered.Data {
						if strings.TrimSpace(filtered.Data[index].ID) == id {
							filtered.Data[index].IsLocked = mapping.IsLocked
							break
						}
					}
				}

				if len(remoteControlByID) > 0 {
					for index := range filtered.Data {
						id := strings.TrimSpace(filtered.Data[index].ID)
						if id == "" {
							continue
						}
						if remoteControlID, ok := remoteControlByID[id]; ok {
							filtered.Data[index].RemoteControlID = remoteControlID
						}
					}
				}
			}
		}

		filtered.Folders = make([]retailPlayerFolder, 0, len(folders))
		for _, folder := range folders {
			filtered.Folders = append(filtered.Folders, mapModelRetailPlayerFolder(folder))
		}

		filtered.DeviceFolder = make([]retailPlayerDeviceFolder, 0, len(deviceFolders))
		for _, deviceFolder := range deviceFolders {
			filtered.DeviceFolder = append(filtered.DeviceFolder, mapModelRetailPlayerDeviceFolder(deviceFolder))
		}

		page := 1
		total := len(filtered.Data)
		filtered.Page = &page
		filtered.Total = &total

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(filtered); err != nil {
			log.Error(ctx, "Unable to encode retail player device response", "err", err)
		}
	}
}

func (n *Router) applyRetailPlayerChannelNames(ctx context.Context, devices []retailPlayerDevice) error {
	if len(devices) == 0 {
		return nil
	}

	response, err := fetchRetailPlayerChannels(ctx)
	if err != nil {
		return err
	}

	if len(response.Channels) == 0 {
		return nil
	}

	channelNameByID := make(map[string]string, len(response.Channels))
	channelNameByName := make(map[string]string, len(response.Channels))

	normalizeKey := func(value string) string {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return ""
		}
		return strings.ToLower(trimmed)
	}

	for _, channel := range response.Channels {
		name := strings.TrimSpace(channel.Name)
		if name == "" {
			continue
		}
		if key := normalizeKey(channel.ID); key != "" {
			channelNameByID[key] = name
		}
		if key := normalizeKey(name); key != "" {
			channelNameByName[key] = name
		}
	}

	for index := range devices {
		channelName := ""
		if key := normalizeKey(devices[index].Channel); key != "" {
			channelName = channelNameByID[key]
		}
		if channelName == "" {
			if key := normalizeKey(devices[index].Channel); key != "" {
				channelName = channelNameByName[key]
			}
		}

		if channelName != "" {
			devices[index].ChannelName = channelName
		}
	}

	return nil
}

func (n *Router) handleCreateRetailPlayerFolder() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if !conf.Server.RetailPlayer.Enabled {
			http.Error(w, "Retail player integration disabled", http.StatusNotFound)
			return
		}

		var payload retailPlayerFolderPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid retail player folder payload", http.StatusBadRequest)
			return
		}

		folderID := strings.TrimSpace(payload.ID)
		if folderID == "" {
			folderID = uuid.NewString()
		}

		name := strings.TrimSpace(payload.Name)
		if name == "" {
			http.Error(w, "Retail player folder name is required", http.StatusBadRequest)
			return
		}

		var parentID *string
		if payload.ParentID != nil {
			trimmed := strings.TrimSpace(*payload.ParentID)
			if trimmed != "" && trimmed != folderID {
				parentID = &trimmed
			}
		}
		isLocked := false
		if payload.IsLocked != nil {
			isLocked = *payload.IsLocked
		}

		var folder model.RetailPlayerFolder
		err := n.ds.WithTx(func(tx model.DataStore) error {
			repo := tx.RetailPlayerFolder(ctx)
			if repo == nil {
				return errors.New("retail player folder repository not available")
			}

			stored, err := repo.Upsert(ctx, model.RetailPlayerFolder{
				ID:       folderID,
				Name:     name,
				ParentID: parentID,
				IsLocked: isLocked,
			})
			if err != nil {
				return err
			}

			folder = stored
			return nil
		})
		if err != nil {
			status := http.StatusInternalServerError
			if isConstraintError(err) {
				status = http.StatusBadRequest
			}
			log.Error(ctx, "Unable to create retail player folder", "err", err)
			http.Error(w, "Unable to create retail player folder", status)
			return
		}

		writeRetailPlayerJSON(ctx, w, http.StatusCreated, map[string]any{"data": mapModelRetailPlayerFolder(folder)})
	}
}

func (n *Router) handleUpdateRetailPlayerFolder() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if !conf.Server.RetailPlayer.Enabled {
			http.Error(w, "Retail player integration disabled", http.StatusNotFound)
			return
		}

		folderID := strings.TrimSpace(chi.URLParam(r, "folderID"))
		if folderID == "" {
			http.Error(w, "Retail player folder id is required", http.StatusBadRequest)
			return
		}

		var payload retailPlayerFolderPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid retail player folder payload", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(payload.Name)
		if name == "" {
			http.Error(w, "Retail player folder name is required", http.StatusBadRequest)
			return
		}

		var parentID *string
		if payload.ParentID != nil {
			trimmed := strings.TrimSpace(*payload.ParentID)
			if trimmed != "" && trimmed != folderID {
				parentID = &trimmed
			}
		}

		var folder model.RetailPlayerFolder
		err := n.ds.WithTx(func(tx model.DataStore) error {
			repo := tx.RetailPlayerFolder(ctx)
			if repo == nil {
				return errors.New("retail player folder repository not available")
			}

			isLocked := false
			if payload.IsLocked != nil {
				isLocked = *payload.IsLocked
			} else if existing, err := repo.Find(ctx, folderID); err == nil {
				isLocked = existing.IsLocked
			} else if !errors.Is(err, model.ErrNotFound) {
				return err
			}

			stored, err := repo.Upsert(ctx, model.RetailPlayerFolder{
				ID:       folderID,
				Name:     name,
				ParentID: parentID,
				IsLocked: isLocked,
			})
			if err != nil {
				return err
			}

			folder = stored
			return nil
		})
		if err != nil {
			status := http.StatusInternalServerError
			if isConstraintError(err) {
				status = http.StatusBadRequest
			}
			log.Error(ctx, "Unable to update retail player folder", "folderID", folderID, "err", err)
			http.Error(w, "Unable to update retail player folder", status)
			return
		}

		writeRetailPlayerJSON(ctx, w, http.StatusOK, map[string]any{"data": mapModelRetailPlayerFolder(folder)})
	}
}

func (n *Router) handleDeleteRetailPlayerFolders() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if !conf.Server.RetailPlayer.Enabled {
			http.Error(w, "Retail player integration disabled", http.StatusNotFound)
			return
		}

		var payload retailPlayerDeleteFoldersRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid retail player folder payload", http.StatusBadRequest)
			return
		}

		normalized := make([]string, 0, len(payload.FolderIDs))
		seen := make(map[string]struct{}, len(payload.FolderIDs))
		for _, id := range payload.FolderIDs {
			trimmed := strings.TrimSpace(id)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			normalized = append(normalized, trimmed)
		}

		if len(normalized) == 0 {
			writeRetailPlayerJSON(ctx, w, http.StatusOK, map[string]any{"data": map[string]any{"deletedFolderIds": []string{}}})
			return
		}

		err := n.ds.WithTx(func(tx model.DataStore) error {
			repo := tx.RetailPlayerFolder(ctx)
			if repo == nil {
				return errors.New("retail player folder repository not available")
			}
			return repo.DeleteMany(ctx, normalized)
		})
		if err != nil {
			log.Error(ctx, "Unable to delete retail player folders", "err", err)
			http.Error(w, "Unable to delete retail player folders", http.StatusInternalServerError)
			return
		}

		writeRetailPlayerJSON(ctx, w, http.StatusOK, map[string]any{"data": map[string]any{"deletedFolderIds": normalized}})
	}
}

func (n *Router) handleAssignRetailPlayerDeviceFolders() http.HandlerFunc {
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

		var payload retailPlayerAssignDeviceFoldersRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid retail player folder payload", http.StatusBadRequest)
			return
		}

		normalized := make([]string, 0, len(payload.FolderIDs))
		seen := make(map[string]struct{}, len(payload.FolderIDs))
		for _, id := range payload.FolderIDs {
			trimmed := strings.TrimSpace(id)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			normalized = append(normalized, trimmed)
		}

		err := n.ds.WithTx(func(tx model.DataStore) error {
			repo := tx.RetailPlayerFolder(ctx)
			if repo == nil {
				return errors.New("retail player folder repository not available")
			}
			return repo.ReplaceDeviceAssignments(ctx, deviceID, normalized)
		})
		if err != nil {
			status := http.StatusInternalServerError
			if isConstraintError(err) {
				status = http.StatusBadRequest
			}
			log.Error(ctx, "Unable to assign retail player device folders", "deviceID", deviceID, "err", err)
			http.Error(w, "Unable to assign retail player device folders", status)
			return
		}

		writeRetailPlayerJSON(ctx, w, http.StatusOK, map[string]any{"data": map[string]any{"deviceId": deviceID, "folderIds": normalized}})
	}
}

func (n *Router) handleRetailPlayerSyncQR() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if !conf.Server.RetailPlayer.Enabled {
			http.Error(w, "Retail player integration disabled", http.StatusNotFound)
			return
		}

		results, err := n.syncRetailPlayerQR(ctx)
		if err != nil {
			log.Error(ctx, "Unable to sync retail player QR codes", "err", err)
			http.Error(w, "Unable to sync retail player QR codes", http.StatusBadGateway)
			return
		}

		writeRetailPlayerJSON(ctx, w, http.StatusOK, map[string]any{"data": results})
	}
}

func (n *Router) handleUpdateRetailPlayerDeviceRemoteControl() http.HandlerFunc {
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

		var payload struct {
			RemoteControlID string `json:"remoteControlId"`
		}

		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid retail player device payload", http.StatusBadRequest)
			return
		}

		trimmedRemoteID := strings.TrimSpace(payload.RemoteControlID)

		responseDevice, err := n.saveRetailPlayerRemoteControlMapping(ctx, deviceID, trimmedRemoteID, "")
		if err != nil {
			status := http.StatusInternalServerError
			if isConstraintError(err) {
				status = http.StatusBadRequest
			}
			log.Error(ctx, "Unable to update retail player device remote control", "deviceID", deviceID, "err", err)
			http.Error(w, "Unable to update retail player device", status)
			return
		}

		writeRetailPlayerJSON(ctx, w, http.StatusOK, map[string]any{"data": responseDevice})
	}
}

func (n *Router) handleRetailPlayerDeviceTriggers() http.HandlerFunc {
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

		log.Info(ctx, "Fetching retail player device triggers", "deviceID", deviceID)

		triggers, err := fetchRetailPlayerDeviceTriggers(ctx, deviceID)
		if err != nil {
			if errors.Is(err, errRetailPlayerDeviceNotFound) {
				http.Error(w, "Retail player device not found", http.StatusNotFound)
				return
			}

			log.Error(ctx, "Unable to fetch retail player device triggers", "deviceID", deviceID, "err", err)
			http.Error(w, "Unable to fetch retail player device triggers", http.StatusBadGateway)
			return
		}

		writeRetailPlayerJSON(ctx, w, http.StatusOK, map[string]any{"triggers": triggers})
	}
}

func (n *Router) handleRetailPlayerDeviceTriggerAction() http.HandlerFunc {
	type triggerActionRequest struct {
		Action string `json:"action"`
		Value  string `json:"value"`
	}

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

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		var payload triggerActionRequest
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "Invalid trigger payload", http.StatusBadRequest)
			return
		}

		action := strings.TrimSpace(payload.Action)
		value := strings.TrimSpace(payload.Value)
		if action == "" || value == "" {
			http.Error(w, "Trigger action and value are required", http.StatusBadRequest)
			return
		}

		log.Info(ctx, "Sending retail player trigger action", "deviceID", deviceID, "action", action, "value", value)

		if err := sendRetailPlayerTriggerAction(ctx, deviceID, action, value); err != nil {
			if errors.Is(err, errRetailPlayerDeviceNotFound) {
				http.Error(w, "Retail player device not found", http.StatusNotFound)
				return
			}

			log.Error(ctx, "Unable to send retail player trigger action", "deviceID", deviceID, "err", err)
			http.Error(w, "Unable to send trigger action", http.StatusBadGateway)
			return
		}

		writeRetailPlayerJSON(ctx, w, http.StatusOK, map[string]any{
			"success": true,
			"action":  action,
			"value":   value,
		})
	}
}

func (n *Router) handleRetailPlayerDeviceConfig() http.HandlerFunc {
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

		normalizedID := normalizeRetailPlayerIdentifier(deviceID)
		if normalizedID == "" {
			http.Error(w, "Retail player device identifier is required", http.StatusBadRequest)
			return
		}

		log.Info(ctx, "Fetching retail player device config", "identifier", normalizedID, "rawIdentifier", deviceID)

		device, err := n.resolveRetailPlayerDevice(ctx, normalizedID)
		if err != nil {
			if errors.Is(err, errRetailPlayerDeviceNotFound) {
				http.Error(w, "Retail player device not found", http.StatusNotFound)
				return
			}

			log.Error(ctx, "Unable to resolve retail player device for config", "identifier", normalizedID, "rawIdentifier", deviceID, "err", err)
			http.Error(w, "Unable to fetch retail player device config", http.StatusBadGateway)
			return
		}

		resolvedID := strings.TrimSpace(device.ID)
		if resolvedID == "" {
			log.Info(ctx, "Retail player device missing resolved id for config", "identifier", normalizedID)
			http.Error(w, "Retail player device not found", http.StatusNotFound)
			return
		}

		config, err := fetchRetailPlayerDeviceConfig(ctx, resolvedID)
		if err != nil {
			if errors.Is(err, errRetailPlayerDeviceNotFound) {
				http.Error(w, "Retail player device not found", http.StatusNotFound)
				return
			}

			log.Error(ctx, "Unable to fetch retail player device config", "identifier", normalizedID, "rawIdentifier", deviceID, "resolvedID", resolvedID, "err", err)
			http.Error(w, "Unable to fetch retail player device config", http.StatusBadGateway)
			return
		}

		if strings.TrimSpace(config.ID) == "" {
			config.ID = resolvedID
		}

		writeRetailPlayerJSON(ctx, w, http.StatusOK, config)
	}
}

func (n *Router) handleRetailPlayerDeviceStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if !conf.Server.RetailPlayer.Enabled {
			http.Error(w, "Retail player integration disabled", http.StatusNotFound)
			return
		}

		rawIdentifier := chi.URLParam(r, "deviceID")
		deviceIdentifier := normalizeRetailPlayerIdentifier(rawIdentifier)
		if deviceIdentifier == "" {
			http.Error(w, "Retail player device identifier is required", http.StatusBadRequest)
			return
		}

		log.Info(ctx, "Fetching retail player device status from remote API", "identifier", deviceIdentifier, "rawIdentifier", rawIdentifier)

		device, err := n.resolveRetailPlayerDevice(ctx, deviceIdentifier)
		if err != nil {
			if errors.Is(err, errRetailPlayerDeviceNotFound) {
				log.Info(ctx, "Retail player device not found", "identifier", deviceIdentifier, "rawIdentifier", rawIdentifier)
				http.Error(w, "Retail player device not found", http.StatusNotFound)
				return
			}

			log.Error(ctx, "Unable to resolve retail player device", "identifier", deviceIdentifier, "rawIdentifier", rawIdentifier, "err", err)
			http.Error(w, "Unable to fetch retail player device status", http.StatusBadGateway)
			return
		}

		resolvedID := strings.TrimSpace(device.ID)
		if resolvedID == "" {
			log.Info(ctx, "Retail player device missing resolved id", "identifier", deviceIdentifier)
			http.Error(w, "Retail player device not found", http.StatusNotFound)
			return
		}

		response, err := n.fetchRetailPlayerDeviceStatus(ctx, r, resolvedID)
		if err != nil {
			if errors.Is(err, errRetailPlayerDeviceNotFound) {
				log.Info(ctx, "Retail player device not found while fetching status", "identifier", deviceIdentifier, "rawIdentifier", rawIdentifier, "resolvedID", resolvedID)
				http.Error(w, "Retail player device not found", http.StatusNotFound)
				return
			}

			log.Error(ctx, "Unable to fetch retail player device status", "identifier", deviceIdentifier, "rawIdentifier", rawIdentifier, "resolvedID", resolvedID, "err", err)
			http.Error(w, "Unable to fetch retail player device status", http.StatusBadGateway)
			return
		}

		if response.Device != nil {
			n.devices.Remember(*response.Device)
		} else {
			deviceCopy := device
			response.Device = &deviceCopy
		}

		if response.Device != nil {
			n.persistRetailPlayerDeviceMappings(ctx, []retailPlayerDevice{*response.Device})
		}

		log.Info(ctx, "Retail player device status fetched", "identifier", deviceIdentifier, "rawIdentifier", rawIdentifier, "resolvedID", resolvedID)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Error(ctx, "Unable to encode retail player device status response", "identifier", deviceIdentifier, "resolvedID", resolvedID, "err", err)
		}
	}
}

func (n *Router) resolveRetailPlayerDevice(ctx context.Context, identifier string) (retailPlayerDevice, error) {
	normalized := normalizeRetailPlayerIdentifier(identifier)
	if normalized == "" {
		return retailPlayerDevice{}, errRetailPlayerDeviceNotFound
	}

	if device, ok := n.devices.Find(normalized); ok {
		return device, nil
	}

	if mappedDevice, err := n.findRetailPlayerDeviceMapping(ctx, normalized); err == nil {
		n.devices.Remember(mappedDevice)
		return mappedDevice, nil
	} else if err != nil && !errors.Is(err, errRetailPlayerDeviceNotFound) {
		return retailPlayerDevice{}, err
	}

	device, err := fetchRetailPlayerDevice(ctx, normalized)
	if err == nil {
		n.devices.Remember(device)
		n.persistRetailPlayerDeviceMappings(ctx, []retailPlayerDevice{device})
		return device, nil
	}
	if err != nil && !errors.Is(err, errRetailPlayerDeviceNotFound) {
		return retailPlayerDevice{}, err
	}

	log.Info(ctx, "Retail player device not found by id, attempting lookup", "identifier", normalized)
	device, devices, lookupErr := lookupRetailPlayerDevice(ctx, normalized)
	if len(devices) > 0 {
		n.devices.RememberDevices(devices)
		n.persistRetailPlayerDeviceMappings(ctx, devices)
	}
	if lookupErr != nil {
		return retailPlayerDevice{}, lookupErr
	}

	if strings.TrimSpace(device.ID) != "" {
		n.persistRetailPlayerDeviceMappings(ctx, []retailPlayerDevice{device})
	}

	return device, nil
}

func (n *Router) findRetailPlayerDeviceMapping(ctx context.Context, identifier string) (retailPlayerDevice, error) {
	if n.ds == nil {
		return retailPlayerDevice{}, errRetailPlayerDeviceNotFound
	}

	repo := n.ds.RetailPlayerDeviceMapping(ctx)
	if repo == nil {
		return retailPlayerDevice{}, errRetailPlayerDeviceNotFound
	}

	mapping, err := repo.FindByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return retailPlayerDevice{}, errRetailPlayerDeviceNotFound
		}
		return retailPlayerDevice{}, err
	}

	device := mapRetailPlayerMappingToDevice(*mapping)
	if strings.TrimSpace(device.ID) == "" {
		return retailPlayerDevice{}, errRetailPlayerDeviceNotFound
	}

	return device, nil
}

func (n *Router) saveRetailPlayerRemoteControlMapping(ctx context.Context, deviceID, remoteControlID, deviceName string) (retailPlayerDevice, error) {
	trimmedDeviceID := strings.TrimSpace(deviceID)
	if trimmedDeviceID == "" {
		return retailPlayerDevice{}, errors.New("retail player device id is required")
	}

	var responseDevice retailPlayerDevice
	err := n.ds.WithTx(func(tx model.DataStore) error {
		repo := tx.RetailPlayerDeviceMapping(ctx)
		if repo == nil {
			return errors.New("retail player device mapping repository not available")
		}

		existing, _ := repo.FindByIdentifier(ctx, trimmedDeviceID)

		mapping := model.RetailPlayerDeviceMapping{
			DeviceID:     trimmedDeviceID,
			DeviceName:   trimmedDeviceID,
			DeviceSlug:   model.RetailPlayerDeviceSlug(trimmedDeviceID),
			RemoteCtrlID: strings.TrimSpace(remoteControlID),
		}

		if existing != nil {
			mapping.DeviceName = existing.DeviceName
			mapping.DeviceSlug = existing.DeviceSlug
			mapping.Channel = existing.Channel
			mapping.ChannelList = existing.ChannelList
			mapping.Organization = existing.Organization
			mapping.TimeZone = existing.TimeZone
		}

		if name := strings.TrimSpace(deviceName); name != "" {
			mapping.DeviceName = name
			mapping.DeviceSlug = model.RetailPlayerDeviceSlug(name)
		}

		if err := repo.Put(ctx, mapping); err != nil {
			return err
		}

		responseDevice = mapRetailPlayerMappingToDevice(mapping)
		return nil
	})
	if err != nil {
		return retailPlayerDevice{}, err
	}

	return responseDevice, nil
}

func (n *Router) persistRetailPlayerDeviceMappings(ctx context.Context, devices []retailPlayerDevice) {
	if len(devices) == 0 || n.ds == nil {
		return
	}

	repo := n.ds.RetailPlayerDeviceMapping(ctx)
	if repo == nil {
		return
	}

	mappings := make([]model.RetailPlayerDeviceMapping, 0, len(devices))
	for _, device := range devices {
		if mapping, ok := mapRetailPlayerDeviceToMapping(device); ok {
			mappings = append(mappings, mapping)
		}
	}

	if len(mappings) == 0 {
		return
	}

	if err := repo.PutMany(ctx, mappings); err != nil {
		log.Error(ctx, "Unable to persist retail player device mappings", "err", err)
	}
}

func (n *Router) loadRetailPlayerFolderData(ctx context.Context) ([]model.RetailPlayerFolder, []model.RetailPlayerDeviceFolder, error) {
	if n == nil || n.ds == nil {
		return nil, nil, nil
	}

	repo := n.ds.RetailPlayerFolder(ctx)
	if repo == nil {
		return nil, nil, nil
	}

	folders, err := repo.List(ctx)
	if err != nil {
		return nil, nil, err
	}

	assignments, err := repo.Assignments(ctx)
	if err != nil {
		return nil, nil, err
	}

	return folders, assignments, nil
}

func mapModelRetailPlayerFolder(folder model.RetailPlayerFolder) retailPlayerFolder {
	var parentID *string
	if folder.ParentID != nil {
		trimmed := strings.TrimSpace(*folder.ParentID)
		if trimmed != "" {
			parentID = &trimmed
		}
	}

	return retailPlayerFolder{
		ID:        strings.TrimSpace(folder.ID),
		Name:      strings.TrimSpace(folder.Name),
		ParentID:  parentID,
		IsLocked:  folder.IsLocked,
		CreatedAt: folder.CreatedAt,
		UpdatedAt: folder.UpdatedAt,
	}
}

func mapModelRetailPlayerDeviceFolder(deviceFolder model.RetailPlayerDeviceFolder) retailPlayerDeviceFolder {
	return retailPlayerDeviceFolder{
		DeviceID:  strings.TrimSpace(deviceFolder.DeviceID),
		FolderID:  strings.TrimSpace(deviceFolder.FolderID),
		CreatedAt: deviceFolder.CreatedAt,
		UpdatedAt: deviceFolder.UpdatedAt,
	}
}

func isConstraintError(err error) bool {
	if err == nil {
		return false
	}

	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "constraint")
}

func mapRetailPlayerDeviceToMapping(device retailPlayerDevice) (model.RetailPlayerDeviceMapping, bool) {
	id := strings.TrimSpace(device.ID)
	if id == "" {
		return model.RetailPlayerDeviceMapping{}, false
	}

	name := strings.TrimSpace(device.Name)
	slug := model.RetailPlayerDeviceSlug(name)
	if slug == "" {
		slug = model.RetailPlayerDeviceSlug(id)
	}

	return model.RetailPlayerDeviceMapping{
		DeviceID:     id,
		DeviceName:   name,
		DeviceSlug:   slug,
		IsLocked:     device.IsLocked,
		Channel:      strings.TrimSpace(device.Channel),
		ChannelList:  strings.TrimSpace(device.ChannelList),
		Organization: strings.TrimSpace(device.Organization),
		TimeZone:     strings.TrimSpace(device.TimeZone),
		RemoteCtrlID: strings.TrimSpace(device.RemoteControlID),
	}, true
}

func mapRetailPlayerMappingToDevice(mapping model.RetailPlayerDeviceMapping) retailPlayerDevice {
	return retailPlayerDevice{
		ID:              strings.TrimSpace(mapping.DeviceID),
		Name:            strings.TrimSpace(mapping.DeviceName),
		IsLocked:        mapping.IsLocked,
		Channel:         strings.TrimSpace(mapping.Channel),
		ChannelList:     strings.TrimSpace(mapping.ChannelList),
		Organization:    strings.TrimSpace(mapping.Organization),
		TimeZone:        strings.TrimSpace(mapping.TimeZone),
		RemoteControlID: strings.TrimSpace(mapping.RemoteCtrlID),
	}
}

func (n *Router) handleRetailPlayerChannelListChannels() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if !conf.Server.RetailPlayer.Enabled {
			http.Error(w, "Retail player integration disabled", http.StatusNotFound)
			return
		}

		channelListID := strings.TrimSpace(chi.URLParam(r, "channelListID"))
		if channelListID == "" {
			http.Error(w, "Retail player channel list id is required", http.StatusBadRequest)
			return
		}

		log.Info(ctx, "Fetching retail player channel list", "channelListID", channelListID)
		response, err := fetchRetailPlayerChannelListChannels(ctx, channelListID)
		if err != nil {
			log.Error(ctx, "Unable to fetch retail player channel list", "channelListID", channelListID, "err", err)
			http.Error(w, "Unable to fetch retail player channel list", http.StatusBadGateway)
			return
		}

		log.Info(ctx, "Retail player channel list fetched", "channelListID", channelListID, "count", len(response.Channels))

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Error(ctx, "Unable to encode retail player channel list response", "channelListID", channelListID, "err", err)
		}
	}
}

func writeRetailPlayerJSON(ctx context.Context, w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if payload == nil {
		return
	}

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Error(ctx, "Unable to encode retail player response", "err", err)
	}
}

func (n *Router) handleRetailPlayerDeviceVolume() http.HandlerFunc {
	type volumeRequest struct {
		Volume json.Number `json:"volume"`
	}

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

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		var payload volumeRequest
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "Invalid volume payload", http.StatusBadRequest)
			return
		}

		if payload.Volume == "" {
			http.Error(w, "Volume is required", http.StatusBadRequest)
			return
		}

		rawValue, err := payload.Volume.Float64()
		if err != nil {
			http.Error(w, "Volume must be a number", http.StatusBadRequest)
			return
		}

		volume := clampVolume(int(math.Round(rawValue)))

		log.Info(ctx, "Sending retail player volume command", "deviceID", deviceID, "volume", volume)

		command := retailPlayerCommandRequest{
			Type: "set_volume",
			Payload: map[string]any{
				"volume": volume,
			},
		}

		responseBody, err := n.sendRetailPlayerDeviceCommand(ctx, deviceID, command)
		if err != nil {
			log.Error(ctx, "Unable to send retail player volume command", "deviceID", deviceID, "err", err)
			http.Error(w, "Unable to update device volume", http.StatusBadGateway)
			return
		}

		writeRetailPlayerJSON(ctx, w, http.StatusOK, map[string]any{
			"success": true,
			"volume":  volume,
			"message": responseBody,
		})
	}
}

func (n *Router) handleRetailPlayerDeviceChannel() http.HandlerFunc {
	type channelRequest struct {
		Channel string `json:"channel"`
	}

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

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		var payload channelRequest
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "Invalid channel payload", http.StatusBadRequest)
			return
		}

		channelID := strings.TrimSpace(payload.Channel)
		if channelID == "" {
			http.Error(w, "Channel id is required", http.StatusBadRequest)
			return
		}

		log.Info(ctx, "Sending retail player channel command", "deviceID", deviceID, "channelID", channelID)

		command := retailPlayerCommandRequest{
			Type: "set_channel",
			Payload: map[string]any{
				"channel": channelID,
			},
		}

		responseBody, err := n.sendRetailPlayerDeviceCommand(ctx, deviceID, command)
		if err != nil {
			log.Error(ctx, "Unable to send retail player channel command", "deviceID", deviceID, "err", err)
			http.Error(w, "Unable to update device channel", http.StatusBadGateway)
			return
		}

		writeRetailPlayerJSON(ctx, w, http.StatusOK, map[string]any{
			"success": true,
			"channel": channelID,
			"message": responseBody,
		})
	}
}

func (n *Router) handleRetailPlayerDeviceToggleChannel() http.HandlerFunc {
	type toggleRequest struct {
		Channel          string `json:"channel,omitempty"`
		ChannelList      string `json:"channelList,omitempty"`
		AlternateChannel string `json:"alternateChannel,omitempty"`
	}

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

		log.Info(ctx, "Retail player channel toggle requested", "deviceID", deviceID)

		var payload toggleRequest
		if r.Body != nil {
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&payload); err != nil {
				if !errors.Is(err, io.EOF) {
					log.Error(ctx, "Invalid toggle channel payload", "deviceID", deviceID, "err", err)
					http.Error(w, "Invalid toggle channel payload", http.StatusBadRequest)
					return
				}
			}
		}

		currentChannelID := strings.TrimSpace(payload.Channel)
		channelListID := strings.TrimSpace(payload.ChannelList)
		alternateChannelID := strings.TrimSpace(payload.AlternateChannel)

		log.Info(ctx, "Retail player toggle payload received", "deviceID", deviceID, "channel", currentChannelID, "channelList", channelListID, "alternateChannel", alternateChannelID)

		if currentChannelID == "" || channelListID == "" {
			log.Info(ctx, "Fetching device metadata for toggle", "deviceID", deviceID)
			device, err := fetchRetailPlayerDevice(ctx, deviceID)
			if err != nil {
				if errors.Is(err, errRetailPlayerDeviceNotFound) {
					log.Info(ctx, "Retail player device not found during toggle", "deviceID", deviceID)
					http.Error(w, "Retail player device not found", http.StatusNotFound)
					return
				}

				log.Error(ctx, "Unable to fetch retail player device", "deviceID", deviceID, "err", err)
				http.Error(w, "Unable to fetch device information", http.StatusBadGateway)
				return
			}

			log.Info(ctx, "Retail player device metadata fetched for toggle", "deviceID", deviceID, "channel", device.Channel, "channelList", device.ChannelList)

			n.devices.Remember(device)
			n.persistRetailPlayerDeviceMappings(ctx, []retailPlayerDevice{device})

			if currentChannelID == "" {
				currentChannelID = strings.TrimSpace(device.Channel)
			}
			if channelListID == "" {
				channelListID = strings.TrimSpace(device.ChannelList)
			}
		}

		if currentChannelID == "" {
			log.Error(ctx, "Missing channel id for toggle", "deviceID", deviceID)
			http.Error(w, "Channel id is required", http.StatusBadRequest)
			return
		}

		if alternateChannelID != "" && strings.EqualFold(alternateChannelID, currentChannelID) {
			log.Info(ctx, "Ignoring alternate channel identical to current", "deviceID", deviceID, "channel", currentChannelID)
			alternateChannelID = ""
		}

		if alternateChannelID == "" {
			if channelListID == "" {
				log.Error(ctx, "Missing channel list for toggle", "deviceID", deviceID)
				http.Error(w, "Channel list id is required to toggle channel", http.StatusBadRequest)
				return
			}

			log.Info(ctx, "Fetching channel list for toggle", "deviceID", deviceID, "channelListID", channelListID)
			response, err := fetchRetailPlayerChannelListChannels(ctx, channelListID)
			if err != nil {
				log.Error(ctx, "Unable to fetch retail player channel list", "deviceID", deviceID, "channelListID", channelListID, "err", err)
				http.Error(w, "Unable to fetch channel list", http.StatusBadGateway)
				return
			}

			for _, channel := range response.Channels {
				candidate := strings.TrimSpace(channel.ID)
				if candidate == "" {
					candidate = strings.TrimSpace(channel.Name)
				}
				if candidate == "" {
					continue
				}
				if strings.EqualFold(candidate, currentChannelID) {
					continue
				}
				alternateChannelID = candidate
				log.Info(ctx, "Selected alternate channel for toggle", "deviceID", deviceID, "alternateChannelID", alternateChannelID)
				break
			}
		}

		if alternateChannelID == "" {
			log.Error(ctx, "No alternate channel found for toggle", "deviceID", deviceID, "channelListID", channelListID)
			http.Error(w, "No alternate channel available to toggle", http.StatusBadRequest)
			return
		}

		log.Info(ctx, "Toggling retail player channel", "deviceID", deviceID, "currentChannelID", currentChannelID, "alternateChannelID", alternateChannelID)

		alternateCommand := retailPlayerCommandRequest{
			Type: "set_channel",
			Payload: map[string]any{
				"channel": alternateChannelID,
			},
		}

		log.Info(ctx, "Sending alternate channel command", "deviceID", deviceID, "alternateChannelID", alternateChannelID)
		alternateResponse, err := n.sendRetailPlayerDeviceCommand(ctx, deviceID, alternateCommand)
		if err != nil {
			log.Error(ctx, "Unable to send retail player alternate channel command", "deviceID", deviceID, "alternateChannelID", alternateChannelID, "err", err)
			http.Error(w, "Unable to toggle device channel", http.StatusBadGateway)
			return
		}
		log.Info(ctx, "Alternate channel command acknowledged", "deviceID", deviceID, "alternateChannelID", alternateChannelID, "response", strings.TrimSpace(alternateResponse))

		restoreCommand := retailPlayerCommandRequest{
			Type: "set_channel",
			Payload: map[string]any{
				"channel": currentChannelID,
			},
		}

		log.Info(ctx, "Restoring original channel", "deviceID", deviceID, "currentChannelID", currentChannelID)
		restoreResponse, err := n.sendRetailPlayerDeviceCommand(ctx, deviceID, restoreCommand)
		if err != nil {
			log.Error(ctx, "Unable to restore retail player channel", "deviceID", deviceID, "currentChannelID", currentChannelID, "err", err)
			http.Error(w, "Unable to restore device channel", http.StatusBadGateway)
			return
		}
		log.Info(ctx, "Original channel restored", "deviceID", deviceID, "currentChannelID", currentChannelID, "response", strings.TrimSpace(restoreResponse))

		response := map[string]any{
			"success":          true,
			"channel":          currentChannelID,
			"alternateChannel": alternateChannelID,
			"alternateMessage": strings.TrimSpace(alternateResponse),
			"restoreMessage":   strings.TrimSpace(restoreResponse),
		}

		if response["alternateMessage"] == "" {
			delete(response, "alternateMessage")
		}
		if response["restoreMessage"] == "" {
			delete(response, "restoreMessage")
		}

		log.Info(ctx, "Retail player toggle completed", "deviceID", deviceID, "channel", currentChannelID, "alternateChannel", alternateChannelID)
		writeRetailPlayerJSON(ctx, w, http.StatusOK, response)
	}
}

func (n *Router) handleRetailPlayerDeviceDislike() http.HandlerFunc {
	type dislikeRequest struct {
		TrackTitle   string `json:"trackTitle"`
		PlaylistName string `json:"playlistName"`
	}

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

		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		var payload dislikeRequest
		if err := decoder.Decode(&payload); err != nil {
			http.Error(w, "Invalid dislike payload", http.StatusBadRequest)
			return
		}

		trackTitle := strings.TrimSpace(payload.TrackTitle)
		playlistName := strings.TrimSpace(payload.PlaylistName)

		log.Info(ctx, "Received retail player dislike", "deviceID", deviceID, "trackTitle", trackTitle, "playlistName", playlistName)

		syncPlaylistPath, err := updateSyncPlaylistForDislike(ctx, playlistName, trackTitle)
		if err != nil {
			log.Warn(ctx, "Unable to update sync playlist for dislike", "deviceID", deviceID, "playlistName", playlistName, "trackTitle", trackTitle, "err", err)
		}

		notifications := conf.Server.RetailPlayer.Notifications
		notified := false

		if notifications.Enabled {
			clientIP := extractClientIP(r)

			if err := sendRetailPlayerDislikeNotification(ctx, clientIP, trackTitle, playlistName, syncPlaylistPath); err != nil {
				log.Error(ctx, "Unable to send retail player dislike notification", "deviceID", deviceID, "err", err)
				http.Error(w, "Unable to send dislike notification", http.StatusBadGateway)
				return
			}

			notified = true
		}

		writeRetailPlayerJSON(ctx, w, http.StatusOK, map[string]any{
			"success":  true,
			"notified": notified,
		})
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
		Fields:            ensureRetailPlayerDeviceFields(cfg.Fields),
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

func lookupRetailPlayerDevice(ctx context.Context, identifier string) (retailPlayerDevice, []retailPlayerDevice, error) {
	normalizedIdentifier := strings.TrimSpace(identifier)
	if normalizedIdentifier == "" {
		return retailPlayerDevice{}, nil, errRetailPlayerDeviceNotFound
	}

	slugKey := retailPlayerDeviceSlugKey(normalizedIdentifier)
	lowerIdentifier := strings.ToLower(normalizedIdentifier)

	response, err := fetchRetailPlayerDevices(ctx)
	if err != nil {
		return retailPlayerDevice{}, nil, err
	}

	for _, device := range response.Data {
		if strings.EqualFold(strings.TrimSpace(device.ID), normalizedIdentifier) {
			return device, response.Data, nil
		}
		if strings.EqualFold(strings.TrimSpace(device.Name), normalizedIdentifier) {
			return device, response.Data, nil
		}

		if slugKey == "" {
			continue
		}

		if retailPlayerDeviceSlugKey(device.Name) == slugKey {
			return device, response.Data, nil
		}

		if retailPlayerDeviceSlugKey(device.ID) == slugKey {
			return device, response.Data, nil
		}

		if lowerIdentifier != "" {
			if retailPlayerDeviceSlugKey(device.Channel) == slugKey {
				return device, response.Data, nil
			}
			if retailPlayerDeviceSlugKey(device.ChannelList) == slugKey {
				return device, response.Data, nil
			}
			if retailPlayerDeviceSlugKey(device.Organization) == slugKey {
				return device, response.Data, nil
			}
		}
	}

	return retailPlayerDevice{}, response.Data, errRetailPlayerDeviceNotFound
}

func retailPlayerDeviceSlugKey(value string) string {
	return model.RetailPlayerDeviceSlug(value)
}

func fetchRetailPlayerDevice(ctx context.Context, deviceID string) (retailPlayerDevice, error) {
	cfg := conf.Server.RetailPlayer
	if cfg.BaseURL == "" || cfg.OrgID == "" {
		return retailPlayerDevice{}, errors.New("retail player API not configured")
	}

	trimmedID := strings.TrimSpace(deviceID)
	if trimmedID == "" {
		return retailPlayerDevice{}, errors.New("retail player device id is empty")
	}

	requestConfig := retailPlayerConfig{
		BaseURL:           cfg.BaseURL,
		OrgID:             cfg.OrgID,
		APIKey:            cfg.APIKey,
		APIKeyHeader:      cfg.APIKeyHeader,
		AdditionalHeaders: cfg.AdditionalHeaders,
	}

	req, err := buildRetailPlayerRequest(ctx, requestConfig, trimmedID)
	if err != nil {
		return retailPlayerDevice{}, err
	}

	resp, err := retailPlayerHTTPClient.Do(req)
	if err != nil {
		return retailPlayerDevice{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		return retailPlayerDevice{}, errRetailPlayerDeviceNotFound
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return retailPlayerDevice{}, fmt.Errorf("retail player API request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return retailPlayerDevice{}, err
	}

	var apiDevice retailPlayerAPIDevice
	if err := json.Unmarshal(body, &apiDevice); err != nil || isRetailPlayerAPIDeviceEmpty(apiDevice) {
		var wrapped struct {
			Data retailPlayerAPIDevice `json:"data"`
		}
		if err := json.Unmarshal(body, &wrapped); err != nil || isRetailPlayerAPIDeviceEmpty(wrapped.Data) {
			return retailPlayerDevice{}, errors.New("unable to parse retail player device response")
		}
		apiDevice = wrapped.Data
	}

	device, ok := simplifyRetailPlayerDevice(apiDevice)
	if !ok {
		return retailPlayerDevice{}, errors.New("unable to simplify retail player device")
	}

	return device, nil
}

func fetchRetailPlayerDeviceConfig(ctx context.Context, deviceID string) (retailPlayerDeviceConfigResponse, error) {
	cfg := conf.Server.RetailPlayer
	if cfg.BaseURL == "" || cfg.OrgID == "" {
		return retailPlayerDeviceConfigResponse{}, errors.New("retail player API not configured")
	}

	trimmedID := strings.TrimSpace(deviceID)
	if trimmedID == "" {
		return retailPlayerDeviceConfigResponse{}, errors.New("retail player device id is empty")
	}

	requestConfig := retailPlayerConfig{
		BaseURL:           cfg.BaseURL,
		OrgID:             cfg.OrgID,
		APIKey:            cfg.APIKey,
		APIKeyHeader:      cfg.APIKeyHeader,
		AdditionalHeaders: cfg.AdditionalHeaders,
	}

	req, err := buildRetailPlayerRequest(ctx, requestConfig, trimmedID)
	if err != nil {
		return retailPlayerDeviceConfigResponse{}, err
	}

	resp, err := retailPlayerHTTPClient.Do(req)
	if err != nil {
		return retailPlayerDeviceConfigResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		return retailPlayerDeviceConfigResponse{}, errRetailPlayerDeviceNotFound
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return retailPlayerDeviceConfigResponse{}, fmt.Errorf("retail player API request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return retailPlayerDeviceConfigResponse{}, err
	}

	var config retailPlayerDeviceConfigResponse
	if err := json.Unmarshal(body, &config); err != nil || isRetailPlayerDeviceConfigEmpty(config) {
		var wrapped struct {
			Data retailPlayerDeviceConfigResponse `json:"data"`
		}
		if err := json.Unmarshal(body, &wrapped); err != nil || isRetailPlayerDeviceConfigEmpty(wrapped.Data) {
			return retailPlayerDeviceConfigResponse{}, errors.New("unable to parse retail player device config response")
		}
		config = wrapped.Data
	}

	if strings.TrimSpace(config.ID) == "" {
		config.ID = trimmedID
	}

	return config, nil
}

func fetchRetailPlayerDeviceTriggers(ctx context.Context, deviceID string) ([]retailPlayerTrigger, error) {
	cfg := conf.Server.RetailPlayer
	baseURL := strings.TrimSpace(cfg.RemoteControlBaseURL)
	if baseURL == "" {
		baseURL = cfg.BaseURL
	}

	apiKey := strings.TrimSpace(cfg.RemoteControlAPIKey)
	if apiKey == "" {
		apiKey = cfg.APIKey
	}

	apiKeyHeader := strings.TrimSpace(cfg.RemoteControlAPIKeyHeader)
	if apiKeyHeader == "" {
		apiKeyHeader = retailPlayerRemoteControlDefaultKeyHeader
	}

	if baseURL == "" {
		return nil, errors.New("retail player remote control API not configured")
	}

	trimmedID := strings.TrimSpace(deviceID)
	if trimmedID == "" {
		return nil, errors.New("retail player device id is empty")
	}

	remoteControlID, err := fetchRetailPlayerRemoteControlID(ctx, trimmedID)
	if err != nil {
		return nil, err
	}

	requestConfig := retailPlayerRemoteControlConfig{
		BaseURL:           baseURL,
		APIKey:            apiKey,
		APIKeyHeader:      apiKeyHeader,
		AdditionalHeaders: cfg.AdditionalHeaders,
	}

	req, err := buildRetailPlayerRemoteControlRequest(ctx, requestConfig, remoteControlID, "triggers")
	if err != nil {
		return nil, err
	}

	resp, err := retailPlayerHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		return nil, errRetailPlayerDeviceNotFound
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("retail player triggers request failed with status %d", resp.StatusCode)
	}

	var payload retailPlayerTriggerAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	triggers := make([]retailPlayerTrigger, 0, len(payload.Value))
	for _, trigger := range payload.Value {
		id := strings.TrimSpace(trigger.ID)
		name := strings.TrimSpace(trigger.Name)
		if id == "" && name == "" {
			continue
		}

		ordinal := trigger.Ordinal
		if ordinal < 0 {
			ordinal = 0
		}

		var asset *retailPlayerTriggerAsset
		if trigger.Asset != nil {
			assetID := strings.TrimSpace(trigger.Asset.ID)
			assetName := strings.TrimSpace(trigger.Asset.Name)
			assetMIME := strings.TrimSpace(trigger.Asset.MIME)
			if assetID != "" || assetName != "" || assetMIME != "" {
				asset = &retailPlayerTriggerAsset{ID: assetID, Name: assetName, MIME: assetMIME}
			}
		}

		triggers = append(triggers, retailPlayerTrigger{
			ID:      id,
			Name:    name,
			Ordinal: ordinal,
			Asset:   asset,
		})
	}

	return triggers, nil
}

func sendRetailPlayerTriggerAction(ctx context.Context, deviceID, action, value string) error {
	cfg := conf.Server.RetailPlayer
	baseURL := strings.TrimSpace(cfg.RemoteControlBaseURL)
	if baseURL == "" {
		baseURL = cfg.BaseURL
	}

	apiKey := strings.TrimSpace(cfg.RemoteControlAPIKey)
	if apiKey == "" {
		apiKey = cfg.APIKey
	}

	apiKeyHeader := strings.TrimSpace(cfg.RemoteControlAPIKeyHeader)
	if apiKeyHeader == "" {
		apiKeyHeader = retailPlayerRemoteControlDefaultKeyHeader
	}

	if baseURL == "" {
		return errors.New("retail player remote control API not configured")
	}

	trimmedID := strings.TrimSpace(deviceID)
	if trimmedID == "" {
		return errors.New("retail player device id is empty")
	}

	remoteControlID, err := fetchRetailPlayerRemoteControlID(ctx, trimmedID)
	if err != nil {
		return err
	}

	requestConfig := retailPlayerRemoteControlConfig{
		BaseURL:           baseURL,
		APIKey:            apiKey,
		APIKeyHeader:      apiKeyHeader,
		AdditionalHeaders: cfg.AdditionalHeaders,
	}

	body, err := json.Marshal(map[string]string{"action": action, "value": value})
	if err != nil {
		return err
	}

	req, err := buildRetailPlayerRemoteControlRequestWithMethod(
		ctx,
		requestConfig,
		remoteControlID,
		http.MethodPost,
		bytes.NewReader(body),
		"triggers",
	)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := retailPlayerHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		return errRetailPlayerDeviceNotFound
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(
			"retail player trigger action failed with status %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(bodyBytes)),
		)
	}

	io.Copy(io.Discard, resp.Body)

	return nil
}

func fetchRetailPlayerDeviceOrganizationID(ctx context.Context, deviceID string) (string, error) {
	cfg := conf.Server.RetailPlayer
	if cfg.BaseURL == "" || cfg.OrgID == "" {
		return "", errors.New("retail player API not configured")
	}

	trimmedID := strings.TrimSpace(deviceID)
	if trimmedID == "" {
		return "", errors.New("retail player device id is empty")
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
		return "", err
	}

	resp, err := retailPlayerHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", errRetailPlayerDeviceNotFound
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("retail player API request failed with status %d", resp.StatusCode)
	}

	var payload retailPlayerDeviceStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}

	if payload.Device != nil {
		if orgID := strings.TrimSpace(payload.Device.OrganizationID); orgID != "" {
			return orgID, nil
		}
		if orgID := strings.TrimSpace(payload.Device.OrganisationID); orgID != "" {
			return orgID, nil
		}
		if orgID := strings.TrimSpace(payload.Device.Organization); orgID != "" {
			return orgID, nil
		}
	}

	return "", errors.New("retail player device organization id not found")
}

func fetchRetailPlayerRemoteControlID(ctx context.Context, deviceID string) (string, error) {
	deviceKey := strings.TrimSpace(deviceID)
	if deviceKey == "" {
		return "", errors.New("retail player device id is empty")
	}

	config, err := fetchRetailPlayerDeviceConfig(ctx, deviceKey)
	if err != nil {
		return "", err
	}

	organizationID := strings.TrimSpace(config.OrgUnit)
	if organizationID == "" {
		organizationID = strings.TrimSpace(config.Organization)
	}
	if organizationID == "" {
		organizationID, err = fetchRetailPlayerDeviceOrganizationID(ctx, deviceKey)
		if err != nil {
			return "", err
		}
	}

	dependents, err := fetchRetailPlayerDependents(ctx, organizationID)
	if err != nil {
		return "", err
	}

	for _, remoteControl := range dependents.RemoteControls {
		trimmedID := strings.TrimSpace(remoteControl.ID)
		if trimmedID == "" {
			continue
		}

		trimmedName := strings.TrimSpace(remoteControl.Name)
		if strings.EqualFold(trimmedID, deviceKey) || strings.EqualFold(trimmedName, deviceKey) {
			return trimmedID, nil
		}
	}

	for _, remoteControl := range dependents.RemoteControls {
		trimmedID := strings.TrimSpace(remoteControl.ID)
		if trimmedID != "" {
			return trimmedID, nil
		}
	}

	return "", errors.New("retail player remote control id not found")
}

func fetchRetailPlayerDependents(ctx context.Context, orgID string) (retailPlayerDependentsResponse, error) {
	cfg := conf.Server.RetailPlayer
	if cfg.BaseURL == "" {
		return retailPlayerDependentsResponse{}, errors.New("retail player API not configured")
	}

	trimmedOrgID := strings.TrimSpace(orgID)
	if trimmedOrgID == "" {
		return retailPlayerDependentsResponse{}, errors.New("retail player organization id is empty")
	}

	requestConfig := retailPlayerConfig{
		BaseURL:           cfg.BaseURL,
		OrgID:             trimmedOrgID,
		APIKey:            cfg.APIKey,
		APIKeyHeader:      cfg.APIKeyHeader,
		AdditionalHeaders: cfg.AdditionalHeaders,
	}

	req, err := buildRetailPlayerDependentsRequest(ctx, requestConfig)
	if err != nil {
		return retailPlayerDependentsResponse{}, err
	}

	resp, err := retailPlayerHTTPClient.Do(req)
	if err != nil {
		return retailPlayerDependentsResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return retailPlayerDependentsResponse{}, fmt.Errorf("retail player dependents request failed with status %d", resp.StatusCode)
	}

	var payload retailPlayerDependentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return retailPlayerDependentsResponse{}, err
	}

	return payload, nil
}

func fetchRetailPlayerChannels(ctx context.Context) (retailPlayerChannelsResponse, error) {
	cfg := conf.Server.RetailPlayer
	if cfg.BaseURL == "" || cfg.OrgID == "" {
		return retailPlayerChannelsResponse{}, errors.New("retail player API not configured")
	}

	requestConfig := retailPlayerConfig{
		BaseURL:           cfg.BaseURL,
		OrgID:             cfg.OrgID,
		APIKey:            cfg.APIKey,
		APIKeyHeader:      cfg.APIKeyHeader,
		AdditionalHeaders: cfg.AdditionalHeaders,
	}

	req, err := buildRetailPlayerOrgRequest(ctx, requestConfig, "channels")
	if err != nil {
		return retailPlayerChannelsResponse{}, err
	}

	resp, err := retailPlayerHTTPClient.Do(req)
	if err != nil {
		return retailPlayerChannelsResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return retailPlayerChannelsResponse{}, fmt.Errorf("retail player API request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return retailPlayerChannelsResponse{}, err
	}

	var channels []retailPlayerAPIChannel
	var payload retailPlayerChannelsAPIResponse
	if err := json.Unmarshal(body, &payload); err == nil && len(payload.Data) > 0 {
		channels = payload.Data
	} else {
		var listPayload retailPlayerChannelListAPIResponse
		if err := json.Unmarshal(body, &listPayload); err == nil && len(listPayload.Channels) > 0 {
			channels = listPayload.Channels
		} else {
			var rawChannels []retailPlayerAPIChannel
			if unmarshalErr := json.Unmarshal(body, &rawChannels); unmarshalErr != nil {
				return retailPlayerChannelsResponse{}, err
			}
			channels = rawChannels
		}
	}

	normalizedChannels := make([]retailPlayerChannel, 0, len(channels))
	for _, item := range channels {
		if channel, ok := simplifyRetailPlayerChannel(item); ok {
			normalizedChannels = append(normalizedChannels, channel)
		}
	}

	return retailPlayerChannelsResponse{Channels: normalizedChannels}, nil
}

func fetchRetailPlayerChannelListChannels(ctx context.Context, channelListID string) (retailPlayerChannelsResponse, error) {
	cfg := conf.Server.RetailPlayer
	if cfg.BaseURL == "" || cfg.OrgID == "" {
		return retailPlayerChannelsResponse{}, errors.New("retail player API not configured")
	}

	trimmedID := strings.TrimSpace(channelListID)
	if trimmedID == "" {
		return retailPlayerChannelsResponse{}, errors.New("retail player channel list id is empty")
	}

	requestConfig := retailPlayerConfig{
		BaseURL:           cfg.BaseURL,
		OrgID:             cfg.OrgID,
		APIKey:            cfg.APIKey,
		APIKeyHeader:      cfg.APIKeyHeader,
		AdditionalHeaders: cfg.AdditionalHeaders,
	}

	req, err := buildRetailPlayerChannelListRequest(ctx, requestConfig, trimmedID, "channels")
	if err != nil {
		return retailPlayerChannelsResponse{}, err
	}

	resp, err := retailPlayerHTTPClient.Do(req)
	if err != nil {
		return retailPlayerChannelsResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return retailPlayerChannelsResponse{}, fmt.Errorf("retail player API request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return retailPlayerChannelsResponse{}, err
	}

	var payload retailPlayerChannelListAPIResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		var rawChannels []retailPlayerAPIChannel
		if unmarshalErr := json.Unmarshal(body, &rawChannels); unmarshalErr != nil {
			return retailPlayerChannelsResponse{}, err
		}
		payload.Channels = rawChannels
	}

	channels := make([]retailPlayerChannel, 0, len(payload.Channels))
	for _, item := range payload.Channels {
		if channel, ok := simplifyRetailPlayerChannel(item); ok {
			channels = append(channels, channel)
		}
	}

	return retailPlayerChannelsResponse{Channels: channels}, nil
}

type retailPlayerStreamMetadata []map[string]any

func (m *retailPlayerStreamMetadata) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*m = nil
		return nil
	}

	switch trimmed[0] {
	case '[':
		var slice []map[string]any
		if err := json.Unmarshal(trimmed, &slice); err != nil {
			return err
		}
		*m = slice
	case '{':
		var obj map[string]any
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			return err
		}
		if len(obj) == 0 {
			*m = nil
		} else {
			*m = []map[string]any{obj}
		}
	default:
		*m = nil
	}

	return nil
}

type retailPlayerDeviceStatusResponse struct {
	Status         map[string]any             `json:"status"`
	StreamMetadata retailPlayerStreamMetadata `json:"streamMetadata"`
	Artwork        *retailPlayerStatusArtwork `json:"artwork,omitempty"`
	Device         *retailPlayerDevice        `json:"device,omitempty"`
}

type retailPlayerCommandRequest struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

type retailPlayerStatusArtwork struct {
	MediaFileID string `json:"mediaFileId,omitempty"`
	ArtworkID   string `json:"artworkId,omitempty"`
	URL         string `json:"url,omitempty"`
}

func (n *Router) sendRetailPlayerDeviceCommand(ctx context.Context, deviceID string, command retailPlayerCommandRequest) (string, error) {
	cfg := conf.Server.RetailPlayer
	if cfg.BaseURL == "" || cfg.OrgID == "" {
		return "", errors.New("retail player API not configured")
	}

	trimmedID := strings.TrimSpace(deviceID)
	if trimmedID == "" {
		return "", errors.New("retail player device id is empty")
	}

	payload, err := json.Marshal(command)
	if err != nil {
		return "", err
	}

	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	endpoint := fmt.Sprintf("%s/orgs/%s/devices/%s/command", baseURL, url.PathEscape(cfg.OrgID), url.PathEscape(trimmedID))

	bodyReader := bytes.NewReader(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bodyReader)
	if err != nil {
		return "", err
	}

	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(payload)), nil
	}
	req.ContentLength = int64(len(payload))
	req.Header.Set("Content-Type", "application/json")

	applyRetailPlayerHeaders(req, retailPlayerConfig{
		BaseURL:           cfg.BaseURL,
		OrgID:             cfg.OrgID,
		APIKey:            cfg.APIKey,
		APIKeyHeader:      cfg.APIKeyHeader,
		AdditionalHeaders: cfg.AdditionalHeaders,
	})

	resp, err := retailPlayerHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		if len(body) > 0 {
			return "", fmt.Errorf("retail player command failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		return "", fmt.Errorf("retail player command failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	trimmedBody := strings.TrimSpace(string(body))
	if trimmedBody != "" {
		log.Info(ctx, "Retail player command response", "deviceID", trimmedID, "body", trimmedBody)
	}

	return trimmedBody, nil
}

func (n *Router) fetchRetailPlayerDeviceStatus(ctx context.Context, r *http.Request, deviceID string) (retailPlayerDeviceStatusResponse, error) {
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

	n.populateRetailPlayerStatusArtwork(ctx, r, &payload)

	return payload, nil
}

func (n *Router) populateRetailPlayerStatusArtwork(ctx context.Context, r *http.Request, payload *retailPlayerDeviceStatusResponse) {
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

		artID := matched.CoverArtID()
		coverArtID := artID.String()
		if coverArtID == "" {
			continue
		}

		artworkURL := public.ImageURL(r, artID, 300)
		if artworkURL != "" {
			if strings.Contains(artworkURL, "?") {
				artworkURL += "&square=true"
			} else {
				artworkURL += "?square=true"
			}
		}

		payload.Artwork = &retailPlayerStatusArtwork{
			MediaFileID: matched.ID,
			ArtworkID:   coverArtID,
			URL:         artworkURL,
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

	return uniqueStringsInsensitive(queries)
}

func uniqueStringsInsensitive(values []string) []string {
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

func ensureRetailPlayerDeviceFields(configured []string) []string {
	if len(configured) == 0 {
		return nil
	}

	fields := make([]string, 0, len(configured)+1)
	hasMacAddress := false
	for _, field := range configured {
		trimmed := strings.TrimSpace(field)
		if trimmed == "" {
			continue
		}
		if strings.EqualFold(trimmed, "macAddress") || strings.EqualFold(trimmed, "mac_address") {
			hasMacAddress = true
		}
		fields = append(fields, trimmed)
	}

	if len(fields) == 0 {
		return nil
	}

	if !hasMacAddress {
		fields = append(fields, "macAddress")
	}

	return fields
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

	applyRetailPlayerHeaders(req, cfg)

	return req, nil
}

func buildRetailPlayerOrgRequest(ctx context.Context, cfg retailPlayerConfig, pathParts ...string) (*http.Request, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		return nil, errors.New("retail player base URL is empty")
	}

	endpoint := fmt.Sprintf("%s/orgs/%s", baseURL, url.PathEscape(cfg.OrgID))
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

	applyRetailPlayerHeaders(req, cfg)

	return req, nil
}

func buildRetailPlayerDependentsRequest(ctx context.Context, cfg retailPlayerConfig) (*http.Request, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		return nil, errors.New("retail player base URL is empty")
	}

	endpoint := fmt.Sprintf("%s/orgs/%s/dependents", baseURL, url.PathEscape(cfg.OrgID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	applyRetailPlayerHeaders(req, cfg)

	return req, nil
}

func buildRetailPlayerChannelListRequest(ctx context.Context, cfg retailPlayerConfig, channelListID string, pathParts ...string) (*http.Request, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		return nil, errors.New("retail player base URL is empty")
	}

	endpoint := fmt.Sprintf("%s/orgs/%s/channel-lists/%s", baseURL, url.PathEscape(cfg.OrgID), url.PathEscape(channelListID))
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

	applyRetailPlayerHeaders(req, cfg)

	return req, nil
}

func applyRetailPlayerHeaders(req *http.Request, cfg retailPlayerConfig) {
	if req == nil {
		return
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
}

func buildRetailPlayerRemoteControlRequest(ctx context.Context, cfg retailPlayerRemoteControlConfig, deviceID string, pathParts ...string) (*http.Request, error) {
	return buildRetailPlayerRemoteControlRequestWithMethod(ctx, cfg, deviceID, http.MethodGet, nil, pathParts...)
}

func buildRetailPlayerRemoteControlRequestWithMethod(
	ctx context.Context,
	cfg retailPlayerRemoteControlConfig,
	deviceID string,
	method string,
	body io.Reader,
	pathParts ...string,
) (*http.Request, error) {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		return nil, errors.New("retail player remote control base URL is empty")
	}

	trimmedID := strings.TrimSpace(deviceID)
	if trimmedID == "" {
		return nil, errors.New("retail player device id is empty")
	}

	endpoint := fmt.Sprintf("%s/device-control/%s", baseURL, url.PathEscape(trimmedID))
	for _, part := range pathParts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		endpoint = fmt.Sprintf("%s/%s", endpoint, url.PathEscape(trimmed))
	}

	verb := strings.TrimSpace(method)
	if verb == "" {
		verb = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, verb, endpoint, body)
	if err != nil {
		return nil, err
	}

	applyRetailPlayerRemoteControlHeaders(req, cfg)

	return req, nil
}

func applyRetailPlayerRemoteControlHeaders(req *http.Request, cfg retailPlayerRemoteControlConfig) {
	if req == nil {
		return
	}

	req.Header.Set("Accept", "application/json")

	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey != "" {
		headerName := strings.TrimSpace(cfg.APIKeyHeader)
		if headerName == "" {
			headerName = retailPlayerRemoteControlDefaultKeyHeader
		}

		req.Header.Set(headerName, apiKey)

		if !strings.EqualFold(headerName, retailPlayerRemoteControlDefaultKeyHeader) {
			req.Header.Set(retailPlayerRemoteControlDefaultKeyHeader, apiKey)
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
}

func isRetailPlayerDeviceConfigEmpty(config retailPlayerDeviceConfigResponse) bool {
	if strings.TrimSpace(config.ID) != "" {
		return false
	}

	if strings.TrimSpace(config.OrgUnit) != "" {
		return false
	}

	if strings.TrimSpace(config.Organization) != "" {
		return false
	}

	if strings.TrimSpace(config.Name) != "" {
		return false
	}

	if strings.TrimSpace(config.Channel) != "" {
		return false
	}

	if strings.TrimSpace(config.ChannelList) != "" {
		return false
	}

	if strings.TrimSpace(config.OrgButtonTriggerSet) != "" {
		return false
	}

	if strings.TrimSpace(config.InstalledFirmware) != "" {
		return false
	}

	return true
}

func isRetailPlayerAPIDeviceEmpty(device retailPlayerAPIDevice) bool {
	if device.Ordinal != nil {
		return false
	}

	if strings.TrimSpace(device.ID) != "" {
		return false
	}
	if strings.TrimSpace(device.MacAddress) != "" {
		return false
	}
	if strings.TrimSpace(device.MacAddressV1) != "" {
		return false
	}
	if strings.TrimSpace(device.Name) != "" {
		return false
	}
	if strings.TrimSpace(device.Location) != "" {
		return false
	}
	if strings.TrimSpace(device.OrgUnit) != "" {
		return false
	}
	if strings.TrimSpace(device.Organization) != "" {
		return false
	}
	if strings.TrimSpace(device.Channel) != "" {
		return false
	}
	if strings.TrimSpace(device.ChannelList) != "" {
		return false
	}
	if strings.TrimSpace(device.TimeZone) != "" {
		return false
	}
	if device.Online != nil {
		return false
	}

	return true
}

func simplifyRetailPlayerDevice(device retailPlayerAPIDevice) (retailPlayerDevice, bool) {
	id := strings.TrimSpace(device.ID)
	macAddress := firstNonEmpty(
		strings.TrimSpace(device.MacAddress),
		strings.TrimSpace(device.MacAddressV1),
	)

	if id == "" {
		id = macAddress
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
		MacAddress:   macAddress,
		Organization: organization,
		TimeZone:     strings.TrimSpace(device.TimeZone),
		Online:       device.Online,
	}, true
}

func simplifyRetailPlayerChannel(channel retailPlayerAPIChannel) (retailPlayerChannel, bool) {
	id := strings.TrimSpace(channel.ID)
	name := strings.TrimSpace(channel.Name)

	if id == "" && name == "" {
		return retailPlayerChannel{}, false
	}

	if name == "" {
		name = id
	}

	return retailPlayerChannel{ID: id, Name: name}, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func clampVolume(value int) int {
	switch {
	case value < 0:
		return 0
	case value > 100:
		return 100
	default:
		return value
	}
}

func extractClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}

	remoteAddr := strings.TrimSpace(r.RemoteAddr)
	if remoteAddr == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil && host != "" {
		return host
	}

	return remoteAddr
}

func sendRetailPlayerDislikeNotification(ctx context.Context, clientIP, trackTitle, playlistName, playlistPath string) error {
	notifications := conf.Server.RetailPlayer.Notifications
	if !notifications.Enabled {
		return nil
	}

	smtpServer := strings.TrimSpace(notifications.SMTPServer)
	username := strings.TrimSpace(notifications.Username)
	password := notifications.Password
	recipient := strings.TrimSpace(notifications.To)

	if smtpServer == "" || username == "" || password == "" || recipient == "" {
		return errors.New("retail player dislike notifications are not fully configured")
	}

	port := notifications.SMTPPort
	if port <= 0 {
		port = 587
	}

	subject := strings.TrimSpace(notifications.Subject)
	if subject == "" {
		subject = "Song 👎"
	}

	ipLabel := strings.TrimSpace(clientIP)
	if ipLabel == "" {
		ipLabel = "unknown IP"
	}

	trackLabel := strings.TrimSpace(trackTitle)
	if trackLabel == "" {
		trackLabel = "unknown song"
	}

	playlistLabel := strings.TrimSpace(playlistName)
	if playlistLabel == "" {
		playlistLabel = "unknown playlist"
	}

	locationLabel := ""
	if ipLabel != "" && ipLabel != "unknown IP" {
		if info, err := fetchIPInfo(ctx, ipLabel); err == nil {
			locationLabel = strings.TrimSpace(strings.Join([]string{
				strings.TrimSpace(info.City),
				strings.TrimSpace(info.Country),
			}, ", "))
			locationLabel = strings.Trim(locationLabel, ", ")
		}
	}

	bodyLines := []string{fmt.Sprintf("%s disliked %s from %s", ipLabel, trackLabel, playlistLabel)}
	if locationLabel != "" {
		bodyLines = append(bodyLines, fmt.Sprintf("Location: %s", locationLabel))
	}
	body := strings.Join(bodyLines, "\n")
	message := fmt.Sprintf("Subject: %s\n\n%s", subject, body)

	if attachment, err := buildPlaylistAttachmentMessage(subject, body, playlistPath); err == nil && attachment != "" {
		message = attachment
	}

	args := []string{
		"--url", fmt.Sprintf("smtp://%s:%d", smtpServer, port),
		"--ssl-reqd",
		"--mail-from", username,
		"--mail-rcpt", recipient,
		"--user", fmt.Sprintf("%s:%s", username, password),
		"--tlsv1.2",
		"-T", "-",
	}

	cmd := exec.CommandContext(ctx, "curl", args...)
	cmd.Stdin = strings.NewReader(message)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("curl command failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

type ipInfo struct {
	City    string `json:"city"`
	Country string `json:"country"`
}

func fetchIPInfo(ctx context.Context, clientIP string) (ipInfo, error) {
	if strings.TrimSpace(clientIP) == "" {
		return ipInfo{}, errors.New("client IP is empty")
	}

	cmd := exec.CommandContext(ctx, "curl", "-s", fmt.Sprintf("https://ipinfo.io/%s/json", clientIP))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ipInfo{}, fmt.Errorf("ipinfo curl failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	var info ipInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return ipInfo{}, fmt.Errorf("unable to decode ipinfo response: %w", err)
	}

	return info, nil
}

func updateSyncPlaylistForDislike(ctx context.Context, playlistName, trackTitle string) (string, error) {
	if strings.TrimSpace(conf.Server.SyncFolder) == "" {
		return "", nil
	}

	normalizedPlaylistName := strings.TrimSpace(playlistName)
	normalizedTrackTitle := strings.TrimSpace(trackTitle)
	if normalizedPlaylistName == "" || normalizedTrackTitle == "" {
		return "", nil
	}

	if filepath.Ext(normalizedPlaylistName) == "" {
		normalizedPlaylistName += ".m3u"
	}

	playlistPath, rootPath, err := findPlaylistFile(conf.Server.PlaylistsPath, normalizedPlaylistName)
	if err != nil {
		return "", err
	}
	if playlistPath == "" {
		return "", fmt.Errorf("playlist not found: %s", normalizedPlaylistName)
	}

	data, err := os.ReadFile(playlistPath)
	if err != nil {
		return "", fmt.Errorf("unable to read playlist: %w", err)
	}

	updated, removed := removeTrackFromM3U(data, normalizedTrackTitle)
	if !removed {
		return "", nil
	}

	syncPath := buildSyncPlaylistPath(conf.Server.SyncFolder, rootPath, playlistPath)
	if err := os.MkdirAll(filepath.Dir(syncPath), 0o755); err != nil {
		return "", fmt.Errorf("unable to create sync playlist dir: %w", err)
	}
	if err := os.WriteFile(syncPath, updated, 0o644); err != nil {
		return "", fmt.Errorf("unable to write sync playlist: %w", err)
	}

	log.Info(ctx, "Synced playlist updated after dislike", "playlist", normalizedPlaylistName, "syncPath", syncPath, "trackTitle", normalizedTrackTitle)
	return syncPath, nil
}

func findPlaylistFile(playlistsPath, playlistFile string) (string, string, error) {
	if strings.TrimSpace(playlistsPath) == "" {
		return "", "", errors.New("playlists path not configured")
	}

	paths := strings.Split(playlistsPath, string(filepath.ListSeparator))
	sentinel := errors.New("playlist found")
	var foundPath string
	var foundRoot string

	for _, root := range paths {
		root = strings.TrimSuffix(root, "**")
		root = strings.TrimSuffix(root, string(os.PathSeparator))
		if root == "" {
			continue
		}
		if absRoot, err := filepath.Abs(root); err == nil {
			root = absRoot
		}

		candidate := filepath.Join(root, playlistFile)
		if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
			return candidate, root, nil
		}

		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if d.Name() == playlistFile {
				foundPath = path
				foundRoot = root
				return sentinel
			}
			return nil
		})
		if err != nil && !errors.Is(err, sentinel) {
			return "", "", err
		}
		if foundPath != "" {
			break
		}
	}

	return foundPath, foundRoot, nil
}

func buildSyncPlaylistPath(syncFolder, rootPath, playlistPath string) string {
	if syncFolder == "" {
		return ""
	}
	if rootPath != "" {
		if rel, err := filepath.Rel(rootPath, playlistPath); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.Join(syncFolder, rel)
		}
	}
	return filepath.Join(syncFolder, filepath.Base(playlistPath))
}

func removeTrackFromM3U(data []byte, trackTitle string) ([]byte, bool) {
	if len(data) == 0 {
		return data, false
	}
	normalized := strings.ToLower(strings.TrimSpace(trackTitle))
	if normalized == "" {
		return data, false
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(content, "\n")
	kept := make([]string, 0, len(lines))
	removed := false

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSuffix(lines[i], "\r")
		lowerLine := strings.ToLower(line)
		if strings.HasPrefix(lowerLine, "#extinf") && strings.Contains(lowerLine, normalized) {
			removed = true
			if i+1 < len(lines) {
				i++
			}
			continue
		}
		if line != "" && !strings.HasPrefix(line, "#") && strings.Contains(lowerLine, normalized) {
			removed = true
			continue
		}
		kept = append(kept, line)
	}

	output := strings.Join(kept, "\n")
	if strings.HasSuffix(content, "\n") {
		output += "\n"
	}

	return []byte(output), removed
}

func buildPlaylistAttachmentMessage(subject, body, playlistPath string) (string, error) {
	if strings.TrimSpace(playlistPath) == "" {
		return "", nil
	}
	stat, err := os.Stat(playlistPath)
	if err != nil || stat.IsDir() {
		return "", nil
	}

	data, err := os.ReadFile(playlistPath)
	if err != nil {
		return "", err
	}

	filename := filepath.Base(playlistPath)
	boundary := fmt.Sprintf("mixed-%d", time.Now().UnixNano())
	encoded := base64.StdEncoding.EncodeToString(data)
	encoded = chunkBase64(encoded, 76)

	message := strings.Join([]string{
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		fmt.Sprintf("Content-Type: multipart/mixed; boundary=%q", boundary),
		"",
		fmt.Sprintf("--%s", boundary),
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
		"",
		fmt.Sprintf("--%s", boundary),
		fmt.Sprintf("Content-Type: audio/x-mpegurl; name=%q", filename),
		fmt.Sprintf("Content-Disposition: attachment; filename=%q", filename),
		"Content-Transfer-Encoding: base64",
		"",
		encoded,
		"",
		fmt.Sprintf("--%s--", boundary),
		"",
	}, "\n")

	return message, nil
}

func chunkBase64(encoded string, width int) string {
	if width <= 0 || len(encoded) <= width {
		return encoded
	}
	var builder strings.Builder
	for len(encoded) > width {
		builder.WriteString(encoded[:width])
		builder.WriteString("\n")
		encoded = encoded[width:]
	}
	builder.WriteString(encoded)
	return builder.String()
}

func (n *Router) syncRetailPlayerQR(ctx context.Context) ([]retailPlayerQRSyncResult, error) {
	cfg := conf.Server.RetailPlayer
	loginURL := strings.TrimSpace(cfg.QRLoginURL)
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.QRBaseURL), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	}

	if loginURL == "" || baseURL == "" {
		return nil, errors.New("retail player QR API not configured")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: retailPlayerHTTPClient.Timeout, Jar: jar}

	if err := performRetailPlayerLogin(ctx, client, loginURL, cfg.QRTenant, cfg.QRUsername, cfg.QRPassword, cfg.QRPlatform); err != nil {
		return nil, err
	}

	pageSize := cfg.QRPageSize
	if pageSize <= 0 {
		pageSize = cfg.PageSize
	}
	if pageSize <= 0 {
		pageSize = 500
	}

	page := cfg.QRPage
	if page <= 0 {
		page = cfg.Page
	}
	if page <= 0 {
		page = 1
	}

	devices, err := fetchRetailPlayerDeviceTable(ctx, client, baseURL, pageSize, page)
	if err != nil {
		return nil, err
	}

	results := make([]retailPlayerQRSyncResult, 0, len(devices))
	for _, device := range devices {
		deviceID := strings.TrimSpace(device.ID)
		deviceName := strings.TrimSpace(device.Name)
		if deviceID == "" || deviceName == "" {
			results = append(results, retailPlayerQRSyncResult{DeviceID: deviceID, DeviceName: deviceName, Error: "device data is incomplete"})
			continue
		}

		qrID, created, err := ensureRetailPlayerQRCode(ctx, client, baseURL, deviceID, deviceName)
		if err != nil {
			log.Error(ctx, "Unable to sync retail player QR code", "deviceID", deviceID, "err", err)
			results = append(results, retailPlayerQRSyncResult{DeviceID: deviceID, DeviceName: deviceName, Error: err.Error()})
			continue
		}

		mappedDevice, err := n.saveRetailPlayerRemoteControlMapping(ctx, deviceID, qrID, deviceName)
		if err != nil {
			log.Error(ctx, "Unable to persist retail player remote control mapping", "deviceID", deviceID, "err", err)
			results = append(results, retailPlayerQRSyncResult{DeviceID: deviceID, DeviceName: deviceName, RemoteControlID: qrID, Created: created, Error: err.Error()})
			continue
		}

		results = append(results, retailPlayerQRSyncResult{DeviceID: mappedDevice.ID, DeviceName: mappedDevice.Name, RemoteControlID: qrID, Created: created})
	}

	return results, nil
}

func performRetailPlayerLogin(ctx context.Context, client *http.Client, loginURL, tenant, username, password, platform string) error {
	if client == nil {
		return errors.New("retail player HTTP client is not configured")
	}

	trimmedLoginURL := strings.TrimSpace(loginURL)
	if trimmedLoginURL == "" {
		return errors.New("retail player login URL is required")
	}

	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return errors.New("retail player login credentials are required")
	}

	loginPayload := map[string]string{
		"tenant":   strings.TrimSpace(tenant),
		"username": strings.TrimSpace(username),
		"password": strings.TrimSpace(password),
		"platform": strings.TrimSpace(platform),
	}

	if loginPayload["platform"] == "" {
		loginPayload["platform"] = "WEB"
	}

	encodedPayload, err := json.Marshal(loginPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, trimmedLoginURL, bytes.NewReader(encodedPayload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("retail player login failed with status %d", resp.StatusCode)
	}

	return nil
}

func fetchRetailPlayerDeviceTable(ctx context.Context, client *http.Client, baseURL string, pageSize, page int) ([]retailPlayerQRDevice, error) {
	if client == nil {
		return nil, errors.New("retail player HTTP client is not configured")
	}

	trimmedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmedBaseURL == "" {
		return nil, errors.New("retail player base URL is required")
	}

	endpoint := fmt.Sprintf("%s/device-table?pageSize=%d&page=%d", trimmedBaseURL, pageSize, page)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("retail player device table request failed with status %d", resp.StatusCode)
	}

	var payload struct {
		Data []retailPlayerQRDevice `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	return payload.Data, nil
}

func ensureRetailPlayerQRCode(ctx context.Context, client *http.Client, baseURL, deviceID, deviceName string) (string, bool, error) {
	existingID, err := fetchRetailPlayerQRCodeID(ctx, client, baseURL, deviceID)
	if err != nil {
		return "", false, err
	}

	if existingID != "" {
		return existingID, false, nil
	}

	generatedID := strings.ToLower(uuid.New().String())
	if err := createRetailPlayerQRCode(ctx, client, baseURL, deviceID, deviceName, generatedID); err != nil {
		return "", false, err
	}

	return generatedID, true, nil
}

func fetchRetailPlayerQRCodeID(ctx context.Context, client *http.Client, baseURL, deviceID string) (string, error) {
	if client == nil {
		return "", errors.New("retail player HTTP client is not configured")
	}

	trimmedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmedBaseURL == "" {
		return "", errors.New("retail player base URL is required")
	}

	trimmedDeviceID := strings.TrimSpace(deviceID)
	if trimmedDeviceID == "" {
		return "", errors.New("retail player device id is required")
	}

	endpoint := fmt.Sprintf("%s/device/%s/qr-codes", trimmedBaseURL, url.PathEscape(trimmedDeviceID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices && resp.StatusCode != http.StatusNotFound {
		return "", fmt.Errorf("retail player QR fetch failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var wrapped struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &wrapped); err == nil {
		for _, item := range wrapped.Data {
			if id := strings.TrimSpace(item.ID); id != "" {
				return id, nil
			}
		}
	}

	var items []struct {
		ID string `json:"id"`
	}

	if err := json.Unmarshal(body, &items); err == nil {
		for _, item := range items {
			if id := strings.TrimSpace(item.ID); id != "" {
				return id, nil
			}
		}
	}

	return "", nil
}

func createRetailPlayerQRCode(ctx context.Context, client *http.Client, baseURL, deviceID, deviceName, qrID string) error {
	if client == nil {
		return errors.New("retail player HTTP client is not configured")
	}

	trimmedBaseURL := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmedBaseURL == "" {
		return errors.New("retail player base URL is required")
	}

	trimmedDeviceID := strings.TrimSpace(deviceID)
	trimmedQRID := strings.TrimSpace(qrID)
	if trimmedDeviceID == "" || trimmedQRID == "" {
		return errors.New("retail player device id and qr id are required")
	}

	endpoint := fmt.Sprintf("%s/device/%s/qr-code/%s", trimmedBaseURL, url.PathEscape(trimmedDeviceID), url.PathEscape(trimmedQRID))
	payload := map[string]any{
		"id":                   trimmedQRID,
		"name":                 fmt.Sprintf("QR Code for %s", strings.TrimSpace(deviceName)),
		"enabled":              true,
		"channelChangeEnabled": true,
		"cuePlayEnabled":       true,
		"volumeChangeEnabled":  true,
	}

	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encodedPayload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("retail player QR creation failed with status %d", resp.StatusCode)
	}

	return nil
}
