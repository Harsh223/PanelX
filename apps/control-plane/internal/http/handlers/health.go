package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthResponse is the payload returned by the health endpoint.
type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
}

// Health returns a simple readiness signal for initial bootstrapping.
func Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(HealthResponse{
		Status:    "ok",
		Service:   "panelx-control-plane",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
