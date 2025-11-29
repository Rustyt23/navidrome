package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"
)

type retailDevice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type retailDeviceResponse struct {
	Data []retailDevice `json:"data"`
}

type rppLoginRequest struct {
	Tenant   string `json:"tenant"`
	Username string `json:"username"`
	Password string `json:"password"`
	Platform string `json:"platform"`
}

type qrRequest struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Enabled              bool   `json:"enabled"`
	ChannelChangeEnabled bool   `json:"channelChangeEnabled"`
	CuePlayEnabled       bool   `json:"cuePlayEnabled"`
	VolumeChangeEnabled  bool   `json:"volumeChangeEnabled"`
}

// RunQRPopulateJob logs into the RPP portal, creates QR codes, and updates Navidrome mappings.
func (h *Handler) RunQRPopulateJob() {
	ctx := context.Background()

	loginURL := conf.GetString("qr_sync.rpp_login_url")
	baseURL := conf.GetString("qr_sync.rpp_base_url")
	username := conf.GetString("qr_sync.rpp_username")
	password := conf.GetString("qr_sync.rpp_password")
	tenant := conf.GetString("qr_sync.tenant")

	if loginURL == "" || baseURL == "" || username == "" || password == "" || tenant == "" {
		log.Error(ctx, "QR sync configuration is incomplete")
		return
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Error(ctx, "Unable to create cookie jar", err)
		return
	}

	client := &http.Client{Jar: jar, Timeout: 30 * time.Second}

	if err := h.loginToRPP(ctx, client, loginURL, tenant, username, password); err != nil {
		log.Error(ctx, "Failed to login to RPP", err)
		return
	}

	devices, err := h.fetchDevices(ctx, client, baseURL)
	if err != nil {
		log.Error(ctx, "Failed to fetch devices", err)
		return
	}

	dbPath := filepath.Join(".", "data", "navidrome.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Error(ctx, "Failed to open database", err)
		return
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		log.Error(ctx, "Failed to begin transaction", err)
		return
	}

	stmt, err := tx.Prepare("UPDATE retail_player_device_mapping SET remote_control_id = ? WHERE device_id = ?;")
	if err != nil {
		log.Error(ctx, "Failed to prepare update statement", err)
		_ = tx.Rollback()
		return
	}
	defer stmt.Close()

	for _, device := range devices {
		qrID := strings.ToLower(uuid.New().String())
		if err := h.createQRCode(ctx, client, baseURL, device, qrID); err != nil {
			log.Error(ctx, "Failed to create QR code", "device", device.ID, "err", err)
			continue
		}

		if _, err := stmt.Exec(qrID, device.ID); err != nil {
			log.Error(ctx, "Failed to update remote_control_id", "device", device.ID, "err", err)
			continue
		}

		log.Info(ctx, "Updated QR mapping", "device", device.ID)
	}

	if err := tx.Commit(); err != nil {
		log.Error(ctx, "Failed to commit QR updates", err)
	}
}

func (h *Handler) loginToRPP(ctx context.Context, client *http.Client, loginURL, tenant, username, password string) error {
	payload := rppLoginRequest{
		Tenant:   tenant,
		Username: username,
		Password: password,
		Platform: "WEB",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal login payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("build login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("execute login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected login status: %d", resp.StatusCode)
	}

	log.Info(ctx, "Logged into RPP successfully")
	return nil
}

func (h *Handler) fetchDevices(ctx context.Context, client *http.Client, baseURL string) ([]retailDevice, error) {
	url := fmt.Sprintf("%s/device-table?pageSize=500&page=1", strings.TrimRight(baseURL, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build device request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch devices: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected device status: %d", resp.StatusCode)
	}

	var payload retailDeviceResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode device response: %w", err)
	}

	return payload.Data, nil
}

func (h *Handler) createQRCode(ctx context.Context, client *http.Client, baseURL string, device retailDevice, qrID string) error {
	payload := qrRequest{
		ID:                   qrID,
		Name:                 fmt.Sprintf("QR Code for %s", device.Name),
		Enabled:              true,
		ChannelChangeEnabled: true,
		CuePlayEnabled:       true,
		VolumeChangeEnabled:  true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal QR payload: %w", err)
	}

	url := fmt.Sprintf("%s/device/%s/qr-code/%s", strings.TrimRight(baseURL, "/"), device.ID, qrID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("build QR request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("create QR: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected QR status: %d", resp.StatusCode)
	}

	return nil
}
