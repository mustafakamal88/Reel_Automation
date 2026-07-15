package http

import (
	"encoding/json"
	"net/http"
	"trendcortex/api/internal/blobstore"
)

type healthResponse struct {
	OK           bool                `json:"ok"`
	Service      string              `json:"service"`
	MediaStorage *mediaStorageStatus `json:"media_storage,omitempty"`
}

type mediaStorageStatus struct {
	Provider string `json:"provider"`
	Durable  bool   `json:"durable"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	store, _ := s.ensureMediaStore()
	status := &mediaStorageStatus{Provider: "unconfigured", Durable: false}
	if store != nil {
		status.Provider = store.Provider()
		status.Durable = store.Provider() == blobstore.ProviderS3
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healthResponse{
		OK:           true,
		Service:      "trendcortex-api",
		MediaStorage: status,
	})
}
