package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/log"

	_ "github.com/mattn/go-sqlite3"
)

type deviceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type qrCodeResponse struct {
	QRCode *struct {
		ID string `json:"id"`
	} `json:"qrCode"`
}

type deviceResponse struct {
	Data    []deviceInfo `json:"data"`
	Devices []deviceInfo `json:"devices"`
}

type deviceUpdate struct {
	deviceID string
	qrID     string
}

func (h *QRHandler) RunQRFetchJob() {
	ctx := log.NewContext(context.Background(), "job", "qr_fetch")

	loginURL := conf.GetString("qr_sync.rpp_login_url")
	baseURL := strings.TrimSuffix(conf.GetString("qr_sync.rpp_base_url"), "/")
	username := conf.GetString("qr_sync.rpp_username")
	password := conf.GetString("qr_sync.rpp_password")
	tenant := conf.GetString("qr_sync.tenant")

	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Error(ctx, "Failed to create cookie jar", err)
		return
	}

	client := &http.Client{Jar: jar}

	if err := h.login(ctx, client, loginURL, tenant, username, password); err != nil {
		log.Error(ctx, "Failed to login to RPP", err)
		return
	}

	devices, err := h.fetchDevices(ctx, client, baseURL)
	if err != nil {
		log.Error(ctx, "Failed to fetch devices", err)
		return
	}

	updates := h.collectUpdates(ctx, client, baseURL, devices)
	if len(updates) == 0 {
		log.Warn(ctx, "No QR codes found for devices")
		return
	}

	h.updateDatabase(ctx, updates)
}

func (h *QRHandler) login(ctx context.Context, client *http.Client, loginURL, tenant, username, password string) error {
	payload := map[string]string{
		"tenant":   tenant,
		"username": username,
		"password": password,
		"platform": "WEB",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal login payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create login request: %w", err)
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

	return nil
}

func (h *QRHandler) fetchDevices(ctx context.Context, client *http.Client, baseURL string) ([]deviceInfo, error) {
	devicesURL := fmt.Sprintf("%s/device-table?pageSize=500&page=1", baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, devicesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create devices request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute devices request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected devices status: %d", resp.StatusCode)
	}

	var dResp deviceResponse
	if err := json.NewDecoder(resp.Body).Decode(&dResp); err != nil {
		return nil, fmt.Errorf("decode devices response: %w", err)
	}

	if len(dResp.Data) > 0 {
		return dResp.Data, nil
	}

	return dResp.Devices, nil
}

func (h *QRHandler) collectUpdates(ctx context.Context, client *http.Client, baseURL string, devices []deviceInfo) []deviceUpdate {
	var updates []deviceUpdate

	for _, device := range devices {
		log.Info(ctx, "Updating device QR", "id", device.ID, "name", device.Name)
		qrID, err := h.fetchQRCode(ctx, client, baseURL, device.ID)
		if err != nil {
			log.Error(ctx, "Failed to fetch QR code", err, "deviceID", device.ID)
			continue
		}
		if qrID == "" {
			log.Warn(ctx, "Missing QR code for device", "deviceID", device.ID)
			continue
		}
		updates = append(updates, deviceUpdate{deviceID: device.ID, qrID: qrID})
	}

	return updates
}

func (h *QRHandler) fetchQRCode(ctx context.Context, client *http.Client, baseURL, deviceID string) (string, error) {
	qrURL := fmt.Sprintf("%s/device/%s/qr-code", baseURL, deviceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, qrURL, nil)
	if err != nil {
		return "", fmt.Errorf("create qr request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute qr request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected qr status: %d", resp.StatusCode)
	}

	var qrResp qrCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&qrResp); err != nil {
		return "", fmt.Errorf("decode qr response: %w", err)
	}

	if qrResp.QRCode == nil {
		return "", nil
	}

	return qrResp.QRCode.ID, nil
}

func (h *QRHandler) updateDatabase(ctx context.Context, updates []deviceUpdate) {
	db, err := sql.Open("sqlite3", "./data/navidrome.db")
	if err != nil {
		log.Error(ctx, "Failed to open database", err)
		return
	}
	defer db.Close()

	if err := ensureRemoteControlColumn(ctx, db); err != nil {
		log.Error(ctx, "Failed to ensure remote_control_id column", err)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		log.Error(ctx, "Failed to start transaction", err)
		return
	}

	stmt, err := tx.Prepare("UPDATE retail_player_device_mapping SET remote_control_id = ? WHERE device_id = ?")
	if err != nil {
		log.Error(ctx, "Failed to prepare statement", err)
		_ = tx.Rollback()
		return
	}
	defer stmt.Close()

	for _, upd := range updates {
		log.Info(ctx, "Updating device", "id", upd.deviceID)
		if _, err := stmt.Exec(upd.qrID, upd.deviceID); err != nil {
			log.Error(ctx, "Failed to update device", err, "id", upd.deviceID)
			continue
		}
	}

	if err := tx.Commit(); err != nil {
		log.Error(ctx, "Failed to commit transaction", err)
	}
}

func ensureRemoteControlColumn(ctx context.Context, db *sql.DB) error {
	rows, err := db.Query("PRAGMA table_info(retail_player_device_mapping)")
	if err != nil {
		return fmt.Errorf("inspect table info: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan table info: %w", err)
		}
		if name == "remote_control_id" {
			return nil
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table info: %w", err)
	}

	_, err = db.Exec("ALTER TABLE retail_player_device_mapping ADD COLUMN remote_control_id TEXT DEFAULT ''")
	return err
}
