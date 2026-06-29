package http

import (
	"encoding/json"
	"net/http"
)

type healthResponse struct {
	OK      bool   `json:"ok"`
	Service string `json:"service"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healthResponse{
		OK:      true,
		Service: "trendcortex-api",
	})
}
