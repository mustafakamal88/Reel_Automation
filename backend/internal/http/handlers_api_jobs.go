package http

import (
	"database/sql"
	"errors"
	"net/http"

	"trendcortex/api/internal/models"
)

// GET /api/video-jobs
func (s *Server) handleListVideoJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, workspace_id, reel_plan_id, status, provider, notes, created_at, updated_at
		FROM video_jobs WHERE workspace_id = $1 ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		jsonError(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	jobs := []models.VideoJob{}
	for rows.Next() {
		var j models.VideoJob
		if err := rows.Scan(&j.ID, &j.WorkspaceID, &j.ReelPlanID, &j.Status, &j.Provider, &j.Notes, &j.CreatedAt, &j.UpdatedAt); err != nil {
			jsonError(w, "scan failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		jobs = append(jobs, j)
	}

	jsonOK(w, map[string]any{"video_jobs": jobs})
}

// GET /api/video-jobs/{id}
func (s *Server) handleGetVideoJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	id := r.PathValue("id")

	var j models.VideoJob
	err = s.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, reel_plan_id, status, provider, notes, created_at, updated_at
		FROM video_jobs WHERE id = $1 AND workspace_id = $2`, id, workspaceID,
	).Scan(&j.ID, &j.WorkspaceID, &j.ReelPlanID, &j.Status, &j.Provider, &j.Notes, &j.CreatedAt, &j.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, "video job not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, j)
}

// GET /api/export-jobs
func (s *Server) handleListExportJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, workspace_id, daily_batch_id, status, zip_path, error_message, created_at, completed_at
		FROM export_jobs WHERE workspace_id = $1 ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		jsonError(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	jobs := []models.ExportJob{}
	for rows.Next() {
		var j models.ExportJob
		if err := rows.Scan(&j.ID, &j.WorkspaceID, &j.DailyBatchID, &j.Status, &j.ZipPath, &j.ErrorMessage, &j.CreatedAt, &j.CompletedAt); err != nil {
			jsonError(w, "scan failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		jobs = append(jobs, j)
	}

	jsonOK(w, map[string]any{"export_jobs": jobs})
}

// GET /api/export-jobs/{id}
func (s *Server) handleGetExportJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	id := r.PathValue("id")

	var j models.ExportJob
	err = s.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, daily_batch_id, status, zip_path, error_message, created_at, completed_at
		FROM export_jobs WHERE id = $1 AND workspace_id = $2`, id, workspaceID,
	).Scan(&j.ID, &j.WorkspaceID, &j.DailyBatchID, &j.Status, &j.ZipPath, &j.ErrorMessage, &j.CreatedAt, &j.CompletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, "export job not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, j)
}

// GET /api/publish-jobs
// Only returns Phase 4A reel-sourced publish jobs (reel_plan_id IS NOT
// NULL) — the Phase 1.5 video_asset-sourced jobs are a separate flow.
func (s *Server) handleListPublishJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, workspace_id, reel_plan_id, platform, status, retry_count, error_message,
			platform_post_id, scheduled_for, started_at, completed_at, created_at
		FROM publish_jobs WHERE workspace_id = $1 AND reel_plan_id IS NOT NULL ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		jsonError(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	jobs := []models.PublishJob{}
	for rows.Next() {
		var j models.PublishJob
		if err := rows.Scan(&j.ID, &j.WorkspaceID, &j.ReelPlanID, &j.Platform, &j.Status, &j.RetryCount, &j.ErrorMessage,
			&j.PlatformPostID, &j.ScheduledFor, &j.StartedAt, &j.CompletedAt, &j.CreatedAt); err != nil {
			jsonError(w, "scan failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		jobs = append(jobs, j)
	}

	jsonOK(w, map[string]any{"publish_jobs": jobs})
}
