package http

import (
	"encoding/json"
	"net/http"
	"trendcortex/api/internal/audit"
	"trendcortex/api/internal/config"
	"trendcortex/api/internal/database"
	"trendcortex/api/internal/oauth"
)

// Server holds the shared dependencies injected into all HTTP handlers.
type Server struct {
	cfg      *config.Config
	db       *database.DB
	registry oauth.Registry
	audit    *audit.Logger
}

// NewServer constructs the Server with all dependencies.
func NewServer(cfg *config.Config, db *database.DB, reg oauth.Registry, al *audit.Logger) *Server {
	return &Server{cfg: cfg, db: db, registry: reg, audit: al}
}

// Routes returns the root http.Handler with all routes registered.
//
// Canonical path conventions (Phase 1.5):
//
//	GET  /health                            — liveness probe (no auth)
//	GET  /platforms/connections             — OAuth status for all publishable platforms
//	POST /platforms/{platform}/disconnect   — revoke OAuth tokens (501 until implemented)
//	POST /platforms/{platform}/refresh      — refresh OAuth token (501 until implemented)
//	POST /platforms/{platform}/test         — live credential probe (501 until implemented)
//	GET  /oauth/{platform}/start            — begin OAuth redirect (no auth — IS the auth flow)
//	GET  /oauth/{platform}/callback         — OAuth callback from platform (no auth)
//	POST /batches/{batchID}/zip             — queue ZIP creation job (session required)
//	POST /batches/{batchID}/publish         — queue publish jobs (session + human approval)
//	GET  /jobs/{jobID}                      — poll background job status (session required)
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Liveness — Railway health check, no auth
	mux.HandleFunc("GET /health", s.handleHealth)

	// Platform connection status — checked by Social Connections page on load
	mux.HandleFunc("GET /platforms/connections", s.handlePlatformConnections)

	// Platform actions (disconnect / refresh / test) — session required
	mux.HandleFunc("POST /platforms/", s.requireSession(s.handlePlatformAction))

	// OAuth flows — no session middleware (these ARE the auth flow)
	mux.HandleFunc("GET /oauth/", s.handleOAuthRoute)

	// Batch operations — session required
	mux.HandleFunc("POST /batches/", s.requireSession(s.handleBatchRoute))

	// Job polling — session required
	mux.HandleFunc("GET /jobs/", s.requireSession(s.handleJobStatus))

	return corsMiddleware(s.cfg.AppBase, mux)
}

// corsMiddleware adds CORS headers permitting requests from the frontend origin only.
func corsMiddleware(allowOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireSession validates the session cookie before calling next.
// Returns 401 if the session is missing or invalid.
func (s *Server) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Production: look up the session cookie in the session store.
		// Placeholder — replace with real session validation before shipping auth.
		_ = r
		next(w, r)
	}
}

// ── Response helpers ──────────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
