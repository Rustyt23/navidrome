package handlers

import (
	"net/http"

	"github.com/navidrome/navidrome/log"
)

type QRHandler struct{}

func NewQRHandler() *QRHandler {
	return &QRHandler{}
}

func (h *QRHandler) PopulateQR(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"started"}`))

	log.Info(r.Context(), "Starting QR fetch job")
	go h.RunQRFetchJob()
}
