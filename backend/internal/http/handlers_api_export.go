package http

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"trendcortex/api/internal/models"
	"trendcortex/api/internal/storage"
)

// reelArtifactCheck pairs a reel plan with whether its video/thumbnail
// artifacts were actually found on disk — computed once per export
// request and reused for both the status decision and the ZIP content.
type reelArtifactCheck struct {
	reel         models.ReelPlan
	hasVideo     bool
	hasThumbnail bool
}

// fileExists reports whether path names a real, readable regular file.
// A missing/empty path is never treated as existing — this is the single
// place that decides whether an artifact is "real".
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func checkReelArtifacts(reels []models.ReelPlan) []reelArtifactCheck {
	checks := make([]reelArtifactCheck, len(reels))
	for i, reel := range reels {
		hasVideo := reel.VideoArtifactPath != nil && fileExists(*reel.VideoArtifactPath)
		hasThumbnail := reel.ThumbnailArtifactPath != nil && fileExists(*reel.ThumbnailArtifactPath)
		checks[i] = reelArtifactCheck{reel: reel, hasVideo: hasVideo, hasThumbnail: hasThumbnail}
	}
	return checks
}

// decideExportStatus maps the set of reels missing each artifact type to a
// batch-level export_jobs.status. Returns "" when nothing is missing,
// meaning the ZIP can actually be built.
func decideExportStatus(missingVideoRanks, missingThumbnailRanks []int) string {
	switch {
	case len(missingVideoRanks) > 0 && len(missingThumbnailRanks) > 0:
		return models.ExportJobStatusMediaMissing
	case len(missingVideoRanks) > 0:
		return models.ExportJobStatusVideoMissing
	case len(missingThumbnailRanks) > 0:
		return models.ExportJobStatusThumbnailMissing
	default:
		return ""
	}
}

func joinRanks(ranks []int) string {
	strs := make([]string, len(ranks))
	for i, n := range ranks {
		strs[i] = strconv.Itoa(n)
	}
	return strings.Join(strs, ", ")
}

// buildMissingArtifactMessage produces the human-readable error_message
// stored on the export_jobs row, naming exactly which reels are missing
// which file.
func buildMissingArtifactMessage(missingVideoRanks, missingThumbnailRanks []int) string {
	var parts []string
	if len(missingVideoRanks) > 0 {
		parts = append(parts, fmt.Sprintf("video.mp4 missing for reel(s): %s", joinRanks(missingVideoRanks)))
	}
	if len(missingThumbnailRanks) > 0 {
		parts = append(parts, fmt.Sprintf("thumbnail.png missing for reel(s): %s", joinRanks(missingThumbnailRanks)))
	}
	return strings.Join(parts, "; ")
}

func reelExportMetadata(c reelArtifactCheck) storage.ReelExportMetadata {
	r := c.reel
	meta := storage.ReelExportMetadata{
		ReelPlanID:   r.ID,
		Rank:         r.Rank,
		Platform:     r.Platform,
		TitleIdea:    r.TitleIdea,
		Status:       r.Status,
		HasVideo:     c.hasVideo,
		HasThumbnail: c.hasThumbnail,
		VideoFormat:  r.VideoFormat,
		VideoCodec:   r.VideoCodec,
		AudioCodec:   r.AudioCodec,
		ThumbnailFormat: r.ThumbnailFormat,
	}
	if r.VideoWidth != nil {
		meta.VideoWidth = *r.VideoWidth
	}
	if r.VideoHeight != nil {
		meta.VideoHeight = *r.VideoHeight
	}
	if r.VideoDurationSeconds != nil {
		meta.VideoDurationSeconds = *r.VideoDurationSeconds
	}
	if r.ThumbnailWidth != nil {
		meta.ThumbnailWidth = *r.ThumbnailWidth
	}
	if r.ThumbnailHeight != nil {
		meta.ThumbnailHeight = *r.ThumbnailHeight
	}
	return meta
}

func reelExportContent(c reelArtifactCheck) storage.ReelExportContent {
	r := c.reel
	content := storage.ReelExportContent{
		Rank:           r.Rank,
		Title:          r.TitleIdea,
		Description:    r.DescriptionDraft,
		Hashtags:       r.HashtagsDraft,
		Script:         r.ScriptOutline,
		ThumbnailBrief: r.ThumbnailIdea,
		Metadata:       reelExportMetadata(c),
	}
	if c.hasVideo {
		content.VideoSrcPath = *r.VideoArtifactPath
	}
	if c.hasThumbnail {
		content.ThumbnailSrcPath = *r.ThumbnailArtifactPath
	}
	return content
}

func buildBatchSummary(batch models.DailyBatch, checks []reelArtifactCheck) storage.BatchExportSummary {
	entries := make([]storage.BatchExportReelEntry, len(checks))
	for i, c := range checks {
		entries[i] = storage.BatchExportReelEntry{
			Rank:         c.reel.Rank,
			Title:        c.reel.TitleIdea,
			Platform:     c.reel.Platform,
			HasVideo:     c.hasVideo,
			HasThumbnail: c.hasThumbnail,
		}
	}
	return storage.BatchExportSummary{
		BatchID:     batch.ID,
		Date:        batch.BatchDate,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		ReelCount:   len(checks),
		Reels:       entries,
	}
}

// insertExportJobRow records one export attempt. zipPath/errMsg may be nil;
// completed controls whether completed_at is stamped with NOW().
func (s *Server) insertExportJobRow(ctx context.Context, workspaceID, batchID, status string, zipPath, errMsg *string, completed bool) (models.ExportJob, error) {
	var job models.ExportJob
	var err error
	if completed {
		err = s.db.QueryRowContext(ctx, `
			INSERT INTO export_jobs (workspace_id, daily_batch_id, status, zip_path, error_message, completed_at)
			VALUES ($1, $2, $3, $4, $5, NOW())
			RETURNING id, workspace_id, daily_batch_id, status, zip_path, error_message, created_at, completed_at`,
			workspaceID, batchID, status, zipPath, errMsg,
		).Scan(&job.ID, &job.WorkspaceID, &job.DailyBatchID, &job.Status, &job.ZipPath, &job.ErrorMessage, &job.CreatedAt, &job.CompletedAt)
	} else {
		err = s.db.QueryRowContext(ctx, `
			INSERT INTO export_jobs (workspace_id, daily_batch_id, status, zip_path, error_message)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, workspace_id, daily_batch_id, status, zip_path, error_message, created_at, completed_at`,
			workspaceID, batchID, status, zipPath, errMsg,
		).Scan(&job.ID, &job.WorkspaceID, &job.DailyBatchID, &job.Status, &job.ZipPath, &job.ErrorMessage, &job.CreatedAt, &job.CompletedAt)
	}
	return job, err
}

// POST /api/batches/{id}/export
//
// Loads the batch and its reel plans, checks every reel for a real
// video.mp4 / thumbnail.png on disk, and either:
//   - reports exactly which reels are missing which artifact
//     (video_artifact_missing / thumbnail_artifact_missing / media_artifacts_missing), or
//   - builds the real ZIP and reports completed, or
//   - reports failed if ZIP generation itself errors.
//
// Every reel's export_status/export_error is updated to reflect this same
// check. Nothing here ever fabricates a video.mp4 or thumbnail.png.
func (s *Server) handleCreateExportJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	batchID := r.PathValue("id")

	batch, err := s.fetchDailyBatchByID(ctx, workspaceID, batchID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, "daily batch not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	reels, err := s.fetchReelPlansForBatch(ctx, batchID)
	if err != nil {
		jsonError(w, "reel plan lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if len(reels) == 0 {
		jsonError(w, "daily batch has no reel plans — generate today's batch first", http.StatusBadRequest)
		return
	}

	checks := checkReelArtifacts(reels)

	var missingVideoRanks, missingThumbnailRanks []int
	for _, c := range checks {
		exportStatus, exportErr := reelExportStatusFor(c)
		if _, uerr := s.db.ExecContext(ctx, `
			UPDATE reel_plans SET export_status = $1, export_error = $2, updated_at = NOW() WHERE id = $3`,
			exportStatus, exportErr, c.reel.ID); uerr != nil {
			jsonError(w, "reel export status update failed: "+uerr.Error(), http.StatusInternalServerError)
			return
		}
		if !c.hasVideo {
			missingVideoRanks = append(missingVideoRanks, c.reel.Rank)
		}
		if !c.hasThumbnail {
			missingThumbnailRanks = append(missingThumbnailRanks, c.reel.Rank)
		}
	}

	if preStatus := decideExportStatus(missingVideoRanks, missingThumbnailRanks); preStatus != "" {
		msg := buildMissingArtifactMessage(missingVideoRanks, missingThumbnailRanks)
		job, err := s.insertExportJobRow(ctx, workspaceID, batchID, preStatus, nil, &msg, false)
		if err != nil {
			jsonError(w, "export job insert failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		jsonOK(w, map[string]any{
			"export_job":              job,
			"missing_video_reels":     emptyIfNil(missingVideoRanks),
			"missing_thumbnail_reels": emptyIfNil(missingThumbnailRanks),
		})
		return
	}

	exportDir := filepath.Join(s.cfg.ExportDir, workspaceID)
	content := make([]storage.ReelExportContent, len(checks))
	for i, c := range checks {
		content[i] = reelExportContent(c)
	}
	summary := buildBatchSummary(batch, checks)

	zipPath, zerr := storage.BuildReelBatchZip(exportDir, batch.BatchDate, summary, content)
	if zerr != nil {
		msg := zerr.Error()
		job, err := s.insertExportJobRow(ctx, workspaceID, batchID, models.ExportJobStatusFailed, nil, &msg, false)
		if err != nil {
			jsonError(w, "export job insert failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		jsonOK(w, map[string]any{
			"export_job":              job,
			"missing_video_reels":     []int{},
			"missing_thumbnail_reels": []int{},
		})
		return
	}

	job, err := s.insertExportJobRow(ctx, workspaceID, batchID, models.ExportJobStatusCompleted, &zipPath, nil, true)
	if err != nil {
		jsonError(w, "export job insert failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]any{
		"export_job":              job,
		"missing_video_reels":     []int{},
		"missing_thumbnail_reels": []int{},
	})
}

func reelExportStatusFor(c reelArtifactCheck) (status string, errMsg *string) {
	switch {
	case c.hasVideo && c.hasThumbnail:
		return models.ReelExportStatusReady, nil
	case !c.hasVideo && !c.hasThumbnail:
		msg := "video.mp4 and thumbnail.png are both missing"
		return models.ReelExportStatusArtifactMissing, &msg
	case !c.hasVideo:
		msg := "video.mp4 is missing"
		return models.ReelExportStatusVideoMissing, &msg
	default:
		msg := "thumbnail.png is missing"
		return models.ReelExportStatusThumbnailMissing, &msg
	}
}

func emptyIfNil(ranks []int) []int {
	if ranks == nil {
		return []int{}
	}
	return ranks
}

// GET /api/export-jobs/{id}/download
//
// Streams the real ZIP file for a completed export job. Refuses to serve
// anything unless export_jobs.status = completed and the ZIP still exists
// on disk — never fabricates a download.
func (s *Server) handleDownloadExportJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	id := r.PathValue("id")

	var job models.ExportJob
	var batchDate string
	err = s.db.QueryRowContext(ctx, `
		SELECT ej.id, ej.workspace_id, ej.daily_batch_id, ej.status, ej.zip_path, ej.error_message, ej.created_at, ej.completed_at, db.batch_date::text
		FROM export_jobs ej
		JOIN daily_batches db ON db.id = ej.daily_batch_id
		WHERE ej.id = $1 AND ej.workspace_id = $2`, id, workspaceID,
	).Scan(&job.ID, &job.WorkspaceID, &job.DailyBatchID, &job.Status, &job.ZipPath, &job.ErrorMessage, &job.CreatedAt, &job.CompletedAt, &batchDate)
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, "export job not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if job.Status != models.ExportJobStatusCompleted || job.ZipPath == nil {
		jsonError(w, "export job is not completed — no ZIP is available for download (status: "+job.Status+")", http.StatusConflict)
		return
	}
	if !fileExists(*job.ZipPath) {
		jsonError(w, "export ZIP file is missing on disk — re-run the export", http.StatusNotFound)
		return
	}

	filename := storage.ExportZipFilename(batchDate)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	http.ServeFile(w, r, *job.ZipPath)
}
