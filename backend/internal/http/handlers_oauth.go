package http

import (
	"errors"
	"net/http"
	"strings"
	"trendcortex/api/internal/oauth"
)

// handleOAuthRoute dispatches GET /oauth/{platform}/start and GET /oauth/{platform}/callback.
func (s *Server) handleOAuthRoute(w http.ResponseWriter, r *http.Request) {
	// Path: /oauth/{platform}/{action}
	trimmed := strings.TrimPrefix(r.URL.Path, "/oauth/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		jsonError(w, "invalid oauth path — expected /oauth/{platform}/start or /oauth/{platform}/callback", http.StatusBadRequest)
		return
	}
	platform := parts[0]
	action := parts[1]

	adapter, err := s.registry.Get(platform)
	if err != nil {
		jsonError(w, "unknown platform: "+platform, http.StatusBadRequest)
		return
	}

	switch action {
	case "start":
		s.oauthStart(w, r, platform, adapter)
	case "callback":
		s.oauthCallback(w, r, platform, adapter)
	default:
		jsonError(w, "unknown oauth action: "+action+" (expected start or callback)", http.StatusBadRequest)
	}
}

// oauthStart checks credentials and — when they are configured — returns the
// authorization URL the browser should be redirected to.
//
// GET /oauth/{platform}/start
//
// Response 200: { "authorize_url": "https://..." }
// Response 503: { "error": "credentials_missing: ..." }
// Response 501: { "error": "not yet implemented" }
func (s *Server) oauthStart(w http.ResponseWriter, r *http.Request, platform string, adapter oauth.Adapter) {
	_, err := adapter.AuthStartURL("probe", "probe")
	if errors.Is(err, oauth.ErrCredentialsMissing) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		jsonOK(w, map[string]string{
			"error":  "credentials_missing",
			"detail": "Add " + platform + " client ID/secret to .env and restart the Go backend.",
		})
		return
	}

	// Production: generate crypto state + PKCE code verifier, store in DB,
	// call adapter.AuthStartURL(state, codeChallenge) and redirect.
	jsonError(w, "oauth start not yet implemented — PKCE state storage required", http.StatusNotImplemented)
}

// oauthCallback receives the authorization code from the platform and exchanges
// it for tokens. Tokens are encrypted and stored server-side; the browser is
// then redirected back to the frontend with connection status only.
//
// GET /oauth/{platform}/callback
func (s *Server) oauthCallback(w http.ResponseWriter, _ *http.Request, _ string, _ oauth.Adapter) {
	jsonError(w, "oauth callback not yet implemented — token exchange + encryption required", http.StatusNotImplemented)
}
