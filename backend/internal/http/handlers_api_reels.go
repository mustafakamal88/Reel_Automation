package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"trendcortex/api/internal/content"
	"trendcortex/api/internal/models"
	trenddiscovery "trendcortex/api/internal/trends"
)

// handleGenerateReelContent creates a script/caption package from a real
// provider-sourced trend candidate. It does not discover trends, persist
// output, or synthesize fallback content.
//
// POST /api/reels/generate-script
func (s *Server) handleGenerateReelContent(w http.ResponseWriter, r *http.Request) {
	var req models.ReelContentGenerationRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.resolveTrendCandidatePayload(r.Context(), &req); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, trenddiscovery.ErrProviderNotConfigured) {
			status = http.StatusServiceUnavailable
		}
		jsonError(w, err.Error(), status)
		return
	}

	genReq, err := content.ValidateRequest(req)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	generator := s.content
	if generator == nil {
		generator = content.OpenAIGenerator{
			APIKey: s.cfg.OpenAIAPIKey,
			Model:  s.cfg.OpenAITextModel,
		}
	}

	pkg, err := generator.Generate(r.Context(), genReq)
	if errors.Is(err, content.ErrProviderNotConfigured) {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}

	jsonOK(w, models.ReelContentGenerationResponse{Package: pkg})
}

func (s *Server) resolveTrendCandidatePayload(ctx context.Context, req *models.ReelContentGenerationRequest) error {
	if req.TrendCandidate != nil || strings.TrimSpace(req.TrendCandidateID) == "" {
		return nil
	}

	discover := s.discover
	if discover == nil {
		timeout, err := time.ParseDuration(s.cfg.TrendDiscoveryTimeout)
		if err != nil {
			timeout = 10 * time.Second
		}
		discoverer := trenddiscovery.NewDiscoverer(trenddiscovery.Config{
			Provider: s.cfg.TrendDiscoveryProvider,
			BaseURL:  s.cfg.TrendDiscoveryBaseURL,
			Timeout:  timeout,
		}, nil)
		discover = discoverer.Discover
	}

	result, err := discover(ctx, req.Region, req.Language, 100)
	if err != nil {
		return err
	}
	id := strings.TrimSpace(req.TrendCandidateID)
	for i := range result.Candidates {
		if result.Candidates[i].ID == id {
			candidate := result.Candidates[i]
			req.TrendCandidate = &candidate
			return nil
		}
	}
	return errors.New("real trend candidate id was not found in the configured provider response")
}

// handlePrepareVideoJob creates a video_job for a reel plan. No render
// provider is connected yet, so the job always lands on
// pending_provider_connection — it is real database state, not a fake
// "rendering" status. Idempotent: calling it again returns the existing job.
//
// POST /api/reels/{id}/prepare-video-job
func (s *Server) handlePrepareVideoJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	reelID := r.PathValue("id")

	var exists string
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM reel_plans WHERE id = $1 AND workspace_id = $2`, reelID, workspaceID).Scan(&exists); err != nil {
		jsonError(w, "reel plan not found", http.StatusNotFound)
		return
	}

	existing, err := s.fetchVideoJobByReelPlan(ctx, reelID)
	if err == nil {
		jsonOK(w, map[string]any{"video_job": existing, "already_existed": true})
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		jsonError(w, "lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var job models.VideoJob
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO video_jobs (workspace_id, reel_plan_id, status, notes)
		VALUES ($1, $2, $3, $4)
		RETURNING id, workspace_id, reel_plan_id, status, provider, notes, created_at, updated_at`,
		workspaceID, reelID, models.VideoJobStatusPendingProvider,
		"No render provider is connected yet. This record marks the request; rendering starts once a provider is wired in.",
	).Scan(&job.ID, &job.WorkspaceID, &job.ReelPlanID, &job.Status, &job.Provider, &job.Notes, &job.CreatedAt, &job.UpdatedAt)
	if err != nil {
		jsonError(w, "video job insert failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := s.db.ExecContext(ctx, `UPDATE reel_plans SET status = $1, updated_at = NOW() WHERE id = $2`, models.ReelPlanStatusVideoRequested, reelID); err != nil {
		jsonError(w, "reel plan status update failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]any{"video_job": job, "already_existed": false})
}

func (s *Server) fetchVideoJobByReelPlan(ctx context.Context, reelID string) (models.VideoJob, error) {
	var job models.VideoJob
	err := s.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, reel_plan_id, status, provider, notes, created_at, updated_at
		FROM video_jobs WHERE reel_plan_id = $1`, reelID,
	).Scan(&job.ID, &job.WorkspaceID, &job.ReelPlanID, &job.Status, &job.Provider, &job.Notes, &job.CreatedAt, &job.UpdatedAt)
	return job, err
}

// handlePublishReel never pretends to publish. Today there is no connected
// OAuth credential for any platform (Phase 1.5's platform_accounts table is
// the source of truth for that), so this always records
// platform_not_connected — a real, honest publish_jobs row.
//
// POST /api/reels/{id}/publish
func (s *Server) handlePublishReel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	reelID := r.PathValue("id")

	var platform string
	err = s.db.QueryRowContext(ctx, `SELECT platform FROM reel_plans WHERE id = $1 AND workspace_id = $2`, reelID, workspaceID).Scan(&platform)
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, "reel plan not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if platform == "" {
		platform = "unspecified"
	}

	var tokenStatus string
	connErr := s.db.QueryRowContext(ctx, `
		SELECT token_status FROM platform_accounts WHERE workspace_id = $1 AND platform = $2`, workspaceID, platform,
	).Scan(&tokenStatus)
	connected := connErr == nil && tokenStatus == string(models.TokenStatusConnected)

	status := models.PublishJobStatusPlatformNotConnected
	if connected {
		// Credential exists, but the actual platform upload pipeline is
		// still not implemented in this phase — never claim success.
		status = models.PublishJobQueued
	}

	var job models.PublishJob
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO publish_jobs (workspace_id, reel_plan_id, platform, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, workspace_id, reel_plan_id, platform, status, retry_count, error_message, platform_post_id, scheduled_for, started_at, completed_at, created_at`,
		workspaceID, reelID, platform, status,
	).Scan(&job.ID, &job.WorkspaceID, &job.ReelPlanID, &job.Platform, &job.Status, &job.RetryCount, &job.ErrorMessage,
		&job.PlatformPostID, &job.ScheduledFor, &job.StartedAt, &job.CompletedAt, &job.CreatedAt)
	if err != nil {
		jsonError(w, "publish job insert failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, job)
}
