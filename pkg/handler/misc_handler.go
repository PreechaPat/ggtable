// Handler for miscellaneous endpoints such as health check

package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

type HealthResponse struct {
	Health    string    `json:"health"`
	Timestamp time.Time `json:"timestamp"`
}

func HealthCheck(w http.ResponseWriter, r *http.Request) {

	response := HealthResponse{
		Health:    "ok",
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

}
