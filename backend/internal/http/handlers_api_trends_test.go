package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"trendcortex/api/internal/config"
)

func TestHandleDiscoverTrendCandidates_NoProviderConfigured(t *testing.T) {
	srv := NewServer(&config.Config{}, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/trends/discover?region=US&limit=20", nil)
	rec := httptest.NewRecorder()

	srv.handleDiscoverTrendCandidates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		ProviderStatus string `json:"provider_status"`
		Region         string `json:"region"`
		Candidates     []any  `json:"candidates"`
		Message        string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ProviderStatus != "provider_not_configured" {
		t.Fatalf("provider_status = %q", body.ProviderStatus)
	}
	if body.Region != "US" {
		t.Fatalf("region = %q", body.Region)
	}
	if len(body.Candidates) != 0 {
		t.Fatalf("candidates length = %d, want 0", len(body.Candidates))
	}
	if body.Message == "" {
		t.Fatalf("message is empty")
	}
}
