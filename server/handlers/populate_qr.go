package handlers

import (
	"encoding/json"
	"net/http"
)

// Handler exposes endpoints for QR population tasks.
type Handler struct{}

// NewQRHandler creates a Handler ready to start QR population jobs.
func NewQRHandler() *Handler {
	return &Handler{}
}

// PopulateQR triggers the QR population job in the background and returns immediately.
func (h *Handler) PopulateQR(w http.ResponseWriter, r *http.Request) {
	go h.RunQRPopulateJob()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}
