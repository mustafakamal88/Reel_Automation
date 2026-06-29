package http

import (
	"errors"
	"net/http"
	"strings"
	"trendcortex/api/internal/oauth"
)

// platformOrder is the canonical set of publishable platforms in display order.
var platformOrder = []string{"youtube", "tiktok", "instagram", "facebook", "threads", "x"}

var platformNames = map[string]string{
	"youtube":   "YouTube Shorts",
	"tiktok":    "TikTok",
	"instagram": "Instagram Reels",
	"facebook":  "Facebook Reels",
	"threads":   "Threads",
	"x":         "X / Twitter",
}

type platformConnectionItem struct {
	Platform   string   `json:"platform"`
	Name       string   `json:"name"`
	Status     string   `json:"status"` // credentials_missing | not_connected | connected | expired
	Scopes     []string `json:"scopes"`
	CanPublish bool     `json:"can_publish"`
}

// handlePlatformAction dispatches POST /platforms/{platform}/disconnect|refresh|test.
func (s *Server) handlePlatformAction(w http.ResponseWriter, r *http.Request) {
	// Path: /platforms/{platform}/{action}
	trimmed := strings.TrimPrefix(r.URL.Path, "/platforms/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		jsonError(w, "invalid path — expected /platforms/{platform}/disconnect|refresh|test", http.StatusBadRequest)
		return
	}
	platform := parts[0]
	action := parts[1]

	if _, err := s.registry.Get(platform); err != nil {
		jsonError(w, "unknown platform: "+platform, http.StatusBadRequest)
		return
	}

	switch action {
	case "disconnect":
		s.platformDisconnect(w, platform)
	case "refresh":
		s.platformRefresh(w, platform)
	case "test":
		s.platformTest(w, platform)
	default:
		jsonError(w, "unknown action: "+action+" (expected disconnect, refresh, or test)", http.StatusBadRequest)
	}
}

// platformDisconnect revokes the stored OAuth tokens for a platform.
//
// POST /platforms/{platform}/disconnect
//
// Response 501: backend worker not yet implemented.
func (s *Server) platformDisconnect(w http.ResponseWriter, platform string) {
	jsonError(w, "disconnect not yet implemented for "+platform+" — token revocation + credential deletion required", http.StatusNotImplemented)
}

// platformRefresh exchanges the stored refresh token for a new access token.
//
// POST /platforms/{platform}/refresh
//
// Response 501: backend worker not yet implemented.
func (s *Server) platformRefresh(w http.ResponseWriter, platform string) {
	jsonError(w, "token refresh not yet implemented for "+platform+" — refresh token exchange required", http.StatusNotImplemented)
}

// platformTest makes a lightweight live API call to verify stored credentials.
//
// POST /platforms/{platform}/test
//
// Response 501: backend worker not yet implemented.
func (s *Server) platformTest(w http.ResponseWriter, platform string) {
	jsonError(w, "credential test not yet implemented for "+platform+" — live API probe required", http.StatusNotImplemented)
}

// handlePlatformConnections returns OAuth connection status for every publishable platform.
// Status "credentials_missing" means the OAuth app credentials are absent from .env.
// Status "not_connected" means credentials are present but the account has not been linked.
//
// GET /platforms/connections
func (s *Server) handlePlatformConnections(w http.ResponseWriter, r *http.Request) {
	type response struct {
		Platforms []platformConnectionItem `json:"platforms"`
	}

	list := make([]platformConnectionItem, 0, len(platformOrder))
	for _, key := range platformOrder {
		adapter, err := s.registry.Get(key)
		if err != nil {
			continue
		}

		status := "not_connected"
		_, err = adapter.AuthStartURL("probe", "probe")
		if errors.Is(err, oauth.ErrCredentialsMissing) {
			status = "credentials_missing"
		}

		list = append(list, platformConnectionItem{
			Platform:   key,
			Name:       platformNames[key],
			Status:     status,
			Scopes:     adapter.RequiredScopes(),
			CanPublish: adapter.CanPublish(),
		})
	}

	jsonOK(w, response{Platforms: list})
}
