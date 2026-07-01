package http

import (
	"encoding/json"
	"net/http"
	"strings"
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

	// ── Phase 4A: trend discovery → scoring → daily batch → reel plan ──────
	// pipeline. Session middleware is not yet attached here (requireSession
	// is currently a no-op everywhere it's used); see defaultWorkspaceID for
	// the single-workspace placeholder this phase runs against until real
	// auth lands.
	mux.HandleFunc("GET /api/trend-sources", s.handleListTrendSources)
	mux.HandleFunc("POST /api/trend-sources", s.handleCreateTrendSource)

	mux.HandleFunc("POST /api/trends/discover", s.handleDiscoverTrends)
	mux.HandleFunc("GET /api/trends", s.handleListTrends)

	mux.HandleFunc("POST /api/topics/score", s.handleScoreTopics)
	mux.HandleFunc("GET /api/topics/scores", s.handleListTopicScores)

	mux.HandleFunc("POST /api/batches/daily", s.handleCreateDailyBatch)
	mux.HandleFunc("GET /api/batches", s.handleListDailyBatches)
	mux.HandleFunc("GET /api/batches/{id}", s.handleGetDailyBatch)
	mux.HandleFunc("GET /api/batches/{id}/reels", s.handleListBatchReels)
	mux.HandleFunc("POST /api/batches/{id}/export", s.handleCreateExportJob)

	mux.HandleFunc("POST /api/reels/{id}/prepare-video-job", s.handlePrepareVideoJob)
	mux.HandleFunc("POST /api/reels/{id}/render", s.handleRenderReel)
	mux.HandleFunc("POST /api/reels/{id}/publish", s.handlePublishReel)

	mux.HandleFunc("GET /api/video-jobs", s.handleListVideoJobs)
	mux.HandleFunc("GET /api/video-jobs/{id}", s.handleGetVideoJob)
	mux.HandleFunc("GET /api/render-jobs", s.handleListRenderJobs)
	mux.HandleFunc("GET /api/render-jobs/{id}", s.handleGetRenderJob)

	mux.HandleFunc("GET /api/export-jobs", s.handleListExportJobs)
	mux.HandleFunc("GET /api/export-jobs/{id}", s.handleGetExportJob)
	mux.HandleFunc("GET /api/export-jobs/{id}/download", s.handleDownloadExportJob)

	mux.HandleFunc("GET /api/publish-jobs", s.handleListPublishJobs)

	return corsMiddleware(s.cfg, mux)
}

// corsMiddleware permits configured frontend/API origins and local Vite dev.
// Once the production frontend origin is known, set APP_BASE_URL to that origin.
func corsMiddleware(cfg *config.Config, next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, origin := range []string{
		cfg.AppBase,
		cfg.APIBase,
		"http://localhost:5173",
		"http://127.0.0.1:5173",
	} {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin != "" {
			allowed[origin] = true
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimRight(r.Header.Get("Origin"), "/")
		if allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Vary", "Origin")
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
