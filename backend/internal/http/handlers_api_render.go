package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"trendcortex/api/internal/models"
	"trendcortex/api/internal/renderer"
)

// POST /api/reels/{id}/render
//
// Attempts a real server-side media render. It never writes a completed job
// or reel artifact paths unless real video.mp4 and thumbnail.png files exist.
func (s *Server) handleRenderReel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	reelID := r.PathValue("id")

	reel, err := s.fetchReelPlanByID(ctx, workspaceID, reelID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, "reel plan not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	job, err := s.upsertVideoJob(ctx, workspaceID, reelID, models.VideoJobStatusRendering, s.cfg.RenderProvider, "Render requested; checking real provider and renderer prerequisites.")
	if err != nil {
		jsonError(w, "render job upsert failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	result := renderer.RenderReel(ctx, renderer.Config{
		Provider:     s.cfg.RenderProvider,
		OutputDir:    s.cfg.MediaOutputDir,
		OpenAIAPIKey: s.cfg.OpenAIAPIKey,
		TTSModel:     s.cfg.OpenAITTSModel,
		ImageModel:   s.cfg.OpenAIImageModel,
		FFmpegPath:   s.cfg.FFmpegPath,
		FFprobePath:  s.cfg.FFprobePath,
	}, renderer.ReelInput{
		WorkspaceID:    reel.WorkspaceID,
		ReelPlanID:     reel.ID,
		Rank:           reel.Rank,
		Title:          reel.TitleIdea,
		Script:         reel.ScriptOutline,
		Description:    reel.DescriptionDraft,
		Hashtags:       reel.HashtagsDraft,
		ThumbnailBrief: reel.ThumbnailIdea,
	})

	job, err = s.updateVideoJobResult(ctx, job.ID, result.Status, s.cfg.RenderProvider, result.Notes)
	if err != nil {
		jsonError(w, "render job update failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.applyRenderResultToReel(ctx, reel.ID, result); err != nil {
		jsonError(w, "reel artifact update failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	status := http.StatusCreated
	if result.Status != renderer.StatusCompleted {
		status = http.StatusAccepted
	}
	w.WriteHeader(status)
	jsonOK(w, map[string]any{"render_job": job, "status": result.Status, "notes": result.Notes})
}

func (s *Server) fetchReelPlanByID(ctx context.Context, workspaceID, reelID string) (models.ReelPlan, error) {
	var rp models.ReelPlan
	err := s.db.QueryRowContext(ctx, `
		SELECT `+reelPlanColumns+`
		FROM reel_plans WHERE id = $1 AND workspace_id = $2`, reelID, workspaceID,
	).Scan(reelPlanScanDest(&rp)...)
	return rp, err
}

func (s *Server) upsertVideoJob(ctx context.Context, workspaceID, reelID, status, provider, notes string) (models.VideoJob, error) {
	var job models.VideoJob
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO video_jobs (workspace_id, reel_plan_id, status, provider, notes)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (reel_plan_id) DO UPDATE SET
			status = EXCLUDED.status,
			provider = EXCLUDED.provider,
			notes = EXCLUDED.notes,
			updated_at = NOW()
		RETURNING id, workspace_id, reel_plan_id, status, provider, notes, created_at, updated_at`,
		workspaceID, reelID, status, provider, notes,
	).Scan(&job.ID, &job.WorkspaceID, &job.ReelPlanID, &job.Status, &job.Provider, &job.Notes, &job.CreatedAt, &job.UpdatedAt)
	return job, err
}

func (s *Server) updateVideoJobResult(ctx context.Context, jobID, status, provider, notes string) (models.VideoJob, error) {
	var job models.VideoJob
	err := s.db.QueryRowContext(ctx, `
		UPDATE video_jobs SET status = $1, provider = $2, notes = $3, updated_at = NOW()
		WHERE id = $4
		RETURNING id, workspace_id, reel_plan_id, status, provider, notes, created_at, updated_at`,
		status, provider, notes, jobID,
	).Scan(&job.ID, &job.WorkspaceID, &job.ReelPlanID, &job.Status, &job.Provider, &job.Notes, &job.CreatedAt, &job.UpdatedAt)
	return job, err
}

func (s *Server) applyRenderResultToReel(ctx context.Context, reelID string, result renderer.Result) error {
	exportStatus, exportError := exportStatusForRenderResult(result)
	if result.Status != renderer.StatusCompleted {
		_, err := s.db.ExecContext(ctx, `
			UPDATE reel_plans SET export_status = $1, export_error = $2, updated_at = NOW()
			WHERE id = $3`,
			exportStatus, exportError, reelID,
		)
		return err
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE reel_plans SET
			video_artifact_path = $1,
			video_format = $2,
			video_width = $3,
			video_height = $4,
			video_duration_seconds = $5,
			video_codec = $6,
			audio_codec = $7,
			thumbnail_artifact_path = $8,
			thumbnail_format = $9,
			thumbnail_width = $10,
			thumbnail_height = $11,
			export_status = $12,
			export_error = NULL,
			updated_at = NOW()
		WHERE id = $13`,
		result.VideoPath,
		result.VideoFormat,
		result.VideoWidth,
		result.VideoHeight,
		result.VideoDurationSeconds,
		result.VideoCodec,
		result.AudioCodec,
		result.ThumbnailPath,
		result.ThumbnailFormat,
		result.ThumbnailWidth,
		result.ThumbnailHeight,
		exportStatus,
		reelID,
	)
	return err
}

func exportStatusForRenderResult(result renderer.Result) (string, *string) {
	if result.Status == renderer.StatusCompleted {
		return models.ReelExportStatusReady, nil
	}
	msg := result.Notes
	if msg == "" {
		msg = result.Status
	}
	switch result.Status {
	case renderer.StatusThumbnailMissing:
		return models.ReelExportStatusArtifactMissing, &msg
	case renderer.StatusAudioArtifactMissing, renderer.StatusFailed, renderer.StatusProviderNotConnected, renderer.StatusRendererNotAvailable:
		return models.ReelExportStatusArtifactMissing, &msg
	default:
		return models.ReelExportStatusArtifactMissing, &msg
	}
}

// GET /api/render-jobs
func (s *Server) handleListRenderJobs(w http.ResponseWriter, r *http.Request) {
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

	jsonOK(w, map[string]any{"render_jobs": jobs})
}

// GET /api/render-jobs/{id}
func (s *Server) handleGetRenderJob(w http.ResponseWriter, r *http.Request) {
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
		jsonError(w, "render job not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, j)
}
