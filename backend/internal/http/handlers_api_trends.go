package http

import (
	"net/http"
	"strings"
	"time"

	"trendcortex/api/internal/models"
)

// ── Trend sources ───────────────────────────────────────────────────────────

type trendSourceRequest struct {
	Name       string  `json:"name"`
	SourceType string  `json:"source_type"`
	Confidence float64 `json:"confidence"`
}

// handleTrendSources dispatches GET/POST /api/trend-sources.
//
// GET  /api/trend-sources  — list trend sources for the default workspace
// POST /api/trend-sources  — create a trend source
func (s *Server) handleListTrendSources(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, workspace_id, name, source_type, status, confidence, created_at, updated_at
		FROM trend_sources WHERE workspace_id = $1 ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		jsonError(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	sources := []models.TrendSource{}
	for rows.Next() {
		var ts models.TrendSource
		if err := rows.Scan(&ts.ID, &ts.WorkspaceID, &ts.Name, &ts.SourceType, &ts.Status, &ts.Confidence, &ts.CreatedAt, &ts.UpdatedAt); err != nil {
			jsonError(w, "scan failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		sources = append(sources, ts)
	}

	jsonOK(w, map[string]any{"trend_sources": sources})
}

func (s *Server) handleCreateTrendSource(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req trendSourceRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.SourceType = strings.TrimSpace(strings.ToLower(req.SourceType))
	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.SourceType == "" {
		req.SourceType = models.TrendSourceTypeManual
	}
	if req.Confidence <= 0 {
		req.Confidence = 0.5
	}
	if req.Confidence > 1 {
		req.Confidence = 1
	}

	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var ts models.TrendSource
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO trend_sources (workspace_id, name, source_type, confidence)
		VALUES ($1, $2, $3, $4)
		RETURNING id, workspace_id, name, source_type, status, confidence, created_at, updated_at`,
		workspaceID, req.Name, req.SourceType, req.Confidence,
	).Scan(&ts.ID, &ts.WorkspaceID, &ts.Name, &ts.SourceType, &ts.Status, &ts.Confidence, &ts.CreatedAt, &ts.UpdatedAt)
	if err != nil {
		jsonError(w, "insert failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, ts)
}

// ── Trend discovery ──────────────────────────────────────────────────────────

type discoverItemRequest struct {
	Topic        string  `json:"topic"`
	Description  string  `json:"description"`
	PlatformHint string  `json:"platform_hint"`
	Velocity     float64 `json:"velocity"`
}

type discoverRequest struct {
	TrendSourceID string                 `json:"trend_source_id"`
	Items         []discoverItemRequest  `json:"items"`
}

// handleDiscoverTrends creates trend_items from explicit manual input only.
// Any other source_type is a real future provider integration and reports
// provider_not_connected without writing any rows.
//
// POST /api/trends/discover
func (s *Server) handleDiscoverTrends(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req discoverRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.TrendSourceID == "" {
		jsonError(w, "trend_source_id is required", http.StatusBadRequest)
		return
	}

	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var src models.TrendSource
	err = s.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, name, source_type, status, confidence, created_at, updated_at
		FROM trend_sources WHERE id = $1 AND workspace_id = $2`, req.TrendSourceID, workspaceID,
	).Scan(&src.ID, &src.WorkspaceID, &src.Name, &src.SourceType, &src.Status, &src.Confidence, &src.CreatedAt, &src.UpdatedAt)
	if err != nil {
		jsonError(w, "trend source not found", http.StatusNotFound)
		return
	}

	if !src.IsDiscoverable() {
		jsonOK(w, map[string]any{
			"status":  "provider_not_connected",
			"message": "source_type \"" + src.SourceType + "\" is not connected yet — no live integration is wired in. Use a manual source with explicit items to create trend_items today.",
		})
		return
	}

	items := req.Items
	if len(items) == 0 {
		jsonError(w, "items is required for a manual source", http.StatusBadRequest)
		return
	}

	created := make([]models.TrendItem, 0, len(items))
	now := time.Now().UTC()
	for _, it := range items {
		topic := strings.TrimSpace(it.Topic)
		if topic == "" {
			continue
		}
		var ti models.TrendItem
		err := s.db.QueryRowContext(ctx, `
			INSERT INTO trend_items (workspace_id, trend_source_id, topic, description, platform_hint, velocity, discovered_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id, workspace_id, trend_source_id, topic, description, platform_hint, velocity, status, discovered_at, created_at, updated_at`,
			workspaceID, src.ID, topic, it.Description, it.PlatformHint, it.Velocity, now,
		).Scan(&ti.ID, &ti.WorkspaceID, &ti.TrendSourceID, &ti.Topic, &ti.Description, &ti.PlatformHint, &ti.Velocity, &ti.Status, &ti.DiscoveredAt, &ti.CreatedAt, &ti.UpdatedAt)
		if err != nil {
			jsonError(w, "insert failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		created = append(created, ti)
	}

	jsonOK(w, map[string]any{
		"status": "created",
		"count":  len(created),
		"items":  created,
	})
}

// ── Trend item listing ───────────────────────────────────────────────────────

// GET /api/trends
func (s *Server) handleListTrends(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))

	query := `
		SELECT id, workspace_id, trend_source_id, topic, description, platform_hint, velocity, status, discovered_at, created_at, updated_at
		FROM trend_items WHERE workspace_id = $1`
	args := []any{workspaceID}
	if statusFilter != "" {
		query += " AND status = $2"
		args = append(args, statusFilter)
	}
	query += " ORDER BY discovered_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		jsonError(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := []models.TrendItem{}
	for rows.Next() {
		var ti models.TrendItem
		if err := rows.Scan(&ti.ID, &ti.WorkspaceID, &ti.TrendSourceID, &ti.Topic, &ti.Description, &ti.PlatformHint, &ti.Velocity, &ti.Status, &ti.DiscoveredAt, &ti.CreatedAt, &ti.UpdatedAt); err != nil {
			jsonError(w, "scan failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		items = append(items, ti)
	}

	jsonOK(w, map[string]any{"trend_items": items})
}
