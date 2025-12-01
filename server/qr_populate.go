package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

const (
	qrPopulateLoginURL  = "https://rpp.jareddietch.com/web/api/v1/login"
	qrPopulateBaseURL   = "https://rpp.jareddietch.com/web/api/v2/org/1aa59b04-5365-4efe-afb3-deb23c414add"
	qrPopulateUsername  = "vishal"
	qrPopulatePassword  = "Musicmatters25!"
	qrPopulateTenant    = "barix"
	qrPopulatePageLimit = 500
)

type qrPopulateDevice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type qrPopulateDeviceResponse struct {
	Data []qrPopulateDevice `json:"data"`
}

type qrPopulateLoginRequest struct {
	Tenant   string `json:"tenant"`
	Username string `json:"username"`
	Password string `json:"password"`
	Platform string `json:"platform"`
}

type qrPopulateLoginResponse struct {
	Token string `json:"token"`
}

type qrPopulateQR struct {
	ID string `json:"id"`
}

type qrPopulateQRResponse struct {
	Data []qrPopulateQR `json:"data"`
}

func (s *Server) mountQRPopulateRoute() {
	s.router.Get("/qr_populate", s.handleQRPopulate())
}

func (s *Server) handleQRPopulate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		jar, err := cookiejar.New(nil)
		if err != nil {
			http.Error(w, "Unable to create cookie jar", http.StatusInternalServerError)
			return
		}

		client := &http.Client{Jar: jar, Timeout: 30 * time.Second}

		if err := s.qrPopulateLogin(ctx, client); err != nil {
			log.Error(ctx, "RPP login failed", "err", err)
			http.Error(w, "RPP login failed", http.StatusBadGateway)
			return
		}

		devices, err := s.qrPopulateFetchDevices(ctx, client)
		if err != nil {
			log.Error(ctx, "Unable to fetch RPP devices", "err", err)
			http.Error(w, "Unable to fetch RPP devices", http.StatusBadGateway)
			return
		}

		processed := 0
		for _, device := range devices {
			deviceID := strings.TrimSpace(device.ID)
			deviceName := strings.TrimSpace(device.Name)
			if deviceID == "" || deviceName == "" {
				continue
			}

			qrID, err := s.qrPopulateEnsureQR(ctx, client, deviceID, deviceName)
			if err != nil {
				log.Error(ctx, "Unable to create or fetch QR code", "deviceID", deviceID, "err", err)
				continue
			}

			if err := s.qrPopulateStoreMapping(ctx, deviceID, deviceName, qrID); err != nil {
				log.Error(ctx, "Unable to store QR mapping", "deviceID", deviceID, "err", err)
				continue
			}

			processed++
		}

		message := fmt.Sprintf("QR populate completed. Updated %d devices.", processed)
		log.Info(ctx, message)
		_, _ = w.Write([]byte(message))
	}
}

func (s *Server) qrPopulateLogin(ctx context.Context, client *http.Client) error {
	payload := qrPopulateLoginRequest{
		Tenant:   qrPopulateTenant,
		Username: qrPopulateUsername,
		Password: qrPopulatePassword,
		Platform: "WEB",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, qrPopulateLoginURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login returned status %d", resp.StatusCode)
	}

	var loginResp qrPopulateLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return err
	}

	if strings.TrimSpace(loginResp.Token) == "" {
		return errors.New("login token missing")
	}

	return nil
}

func (s *Server) qrPopulateFetchDevices(ctx context.Context, client *http.Client) ([]qrPopulateDevice, error) {
	url := fmt.Sprintf("%s/device-table?pageSize=%d&page=1", qrPopulateBaseURL, qrPopulatePageLimit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device fetch returned status %d", resp.StatusCode)
	}

	var deviceResp qrPopulateDeviceResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceResp); err != nil {
		return nil, err
	}

	return deviceResp.Data, nil
}

func (s *Server) qrPopulateEnsureQR(ctx context.Context, client *http.Client, deviceID, deviceName string) (string, error) {
	existing, err := s.qrPopulateFetchQR(ctx, client, deviceID)
	if err == nil && existing != "" {
		return existing, nil
	}

	newQR := strings.ToLower(uuid.New().String())
	payload := map[string]any{
		"id":                   newQR,
		"name":                 fmt.Sprintf("QR Code for %s", deviceName),
		"enabled":              true,
		"channelChangeEnabled": true,
		"cuePlayEnabled":       true,
		"volumeChangeEnabled":  true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/device/%s/qr-code/%s", qrPopulateBaseURL, deviceID, newQR)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("qr creation returned status %d", resp.StatusCode)
	}

	return newQR, nil
}

func (s *Server) qrPopulateFetchQR(ctx context.Context, client *http.Client, deviceID string) (string, error) {
	url := fmt.Sprintf("%s/device/%s/qr-codes", qrPopulateBaseURL, deviceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("qr fetch returned status %d", resp.StatusCode)
	}

	var qrResp qrPopulateQRResponse
	decoderErr := json.NewDecoder(resp.Body).Decode(&qrResp)
	if decoderErr != nil {
		return "", decoderErr
	}

	if len(qrResp.Data) > 0 {
		id := strings.TrimSpace(qrResp.Data[0].ID)
		if id != "" {
			return id, nil
		}
	}

	return "", errors.New("qr not found")
}

func (s *Server) qrPopulateStoreMapping(ctx context.Context, deviceID, deviceName, qrID string) error {
	if qrID == "" {
		return errors.New("empty qr id")
	}

	return s.ds.WithTx(func(tx model.DataStore) error {
		repo := tx.RetailPlayerDeviceMapping(ctx)
		if repo == nil {
			return errors.New("retail player device mapping repository not available")
		}

		mapping := model.RetailPlayerDeviceMapping{
			DeviceID:     deviceID,
			DeviceName:   deviceName,
			DeviceSlug:   model.RetailPlayerDeviceSlug(deviceName),
			RemoteCtrlID: qrID,
		}

		return repo.Put(ctx, mapping)
	})
}
