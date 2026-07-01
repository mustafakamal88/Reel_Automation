package http

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"trendcortex/api/internal/models"
)

type createDailyBatchRequest struct {
	Date string `json:"date"`
}

type eligibleTopic struct {
	TrendItemID  string
	Topic        string
	Description  string
	PlatformHint string
	TopicScoreID string
}

// handleCreateDailyBatch selects up to the 6 highest-scored eligible trend
// items and turns each into a reel_plan. Idempotent per (workspace, date):
// calling it again for a date that already has a batch returns the
// existing batch instead of creating a duplicate.
//
// POST /api/batches/daily
func (s *Server) handleCreateDailyBatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var req createDailyBatchRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	date := strings.TrimSpace(req.Date)
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}

	existing, err := s.fetchDailyBatchByDate(ctx, workspaceID, date)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		jsonError(w, "lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err == nil {
		reels, rerr := s.fetchReelPlansForBatch(ctx, existing.ID)
		if rerr != nil {
			jsonError(w, "reel plan lookup failed: "+rerr.Error(), http.StatusInternalServerError)
			return
		}
		jsonOK(w, map[string]any{"batch": existing, "reel_plans": reels, "already_existed": true})
		return
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT ti.id, ti.topic, ti.description, ti.platform_hint, ts.id
		FROM trend_items ti
		JOIN topic_scores ts ON ts.trend_item_id = ti.id
		WHERE ti.workspace_id = $1 AND ti.status = $2
		ORDER BY ts.total_score DESC
		LIMIT 6`, workspaceID, models.TrendItemStatusScored)
	if err != nil {
		jsonError(w, "candidate query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var candidates []eligibleTopic
	for rows.Next() {
		var c eligibleTopic
		if err := rows.Scan(&c.TrendItemID, &c.Topic, &c.Description, &c.PlatformHint, &c.TopicScoreID); err != nil {
			rows.Close()
			jsonError(w, "scan failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		candidates = append(candidates, c)
	}
	rows.Close()

	status := models.DailyBatchStatusPlanned
	if len(candidates) > 0 {
		status = models.DailyBatchStatusReady
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		jsonError(w, "transaction start failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var batch models.DailyBatch
	err = tx.QueryRowContext(ctx, `
		INSERT INTO daily_batches (workspace_id, batch_date, status, reel_count)
		VALUES ($1, $2, $3, $4)
		RETURNING id, workspace_id, batch_date::text, status, reel_count, created_at, updated_at`,
		workspaceID, date, status, len(candidates),
	).Scan(&batch.ID, &batch.WorkspaceID, &batch.BatchDate, &batch.Status, &batch.ReelCount, &batch.CreatedAt, &batch.UpdatedAt)
	if err != nil {
		jsonError(w, "batch insert failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	reelPlans := make([]models.ReelPlan, 0, len(candidates))
	for i, c := range candidates {
		var rp models.ReelPlan
		err := tx.QueryRowContext(ctx, `
			INSERT INTO reel_plans
				(workspace_id, daily_batch_id, trend_item_id, topic_score_id, rank, platform,
				 title_idea, script_outline, description_draft, hashtags_draft, thumbnail_idea)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING `+reelPlanColumns,
			workspaceID, batch.ID, c.TrendItemID, c.TopicScoreID, i+1, c.PlatformHint,
			draftTitleIdea(c.Topic), draftScriptOutline(c.Topic, c.Description), draftDescription(c.Description),
			draftHashtags(c.Topic), draftThumbnailIdea(c.Topic),
		).Scan(reelPlanScanDest(&rp)...)
		if err != nil {
			jsonError(w, "reel plan insert failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		reelPlans = append(reelPlans, rp)

		if _, err := tx.ExecContext(ctx, `UPDATE trend_items SET status = $1, updated_at = NOW() WHERE id = $2`, models.TrendItemStatusBatched, c.TrendItemID); err != nil {
			jsonError(w, "trend item status update failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		jsonError(w, "commit failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]any{"batch": batch, "reel_plans": reelPlans, "already_existed": false})
}

func (s *Server) fetchDailyBatchByDate(ctx context.Context, workspaceID, date string) (models.DailyBatch, error) {
	var b models.DailyBatch
	err := s.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, batch_date::text, status, reel_count, created_at, updated_at
		FROM daily_batches WHERE workspace_id = $1 AND batch_date = $2`, workspaceID, date,
	).Scan(&b.ID, &b.WorkspaceID, &b.BatchDate, &b.Status, &b.ReelCount, &b.CreatedAt, &b.UpdatedAt)
	return b, err
}

func (s *Server) fetchDailyBatchByID(ctx context.Context, workspaceID, id string) (models.DailyBatch, error) {
	var b models.DailyBatch
	err := s.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, batch_date::text, status, reel_count, created_at, updated_at
		FROM daily_batches WHERE id = $1 AND workspace_id = $2`, id, workspaceID,
	).Scan(&b.ID, &b.WorkspaceID, &b.BatchDate, &b.Status, &b.ReelCount, &b.CreatedAt, &b.UpdatedAt)
	return b, err
}

// reelPlanColumns is the full reel_plans column list (including Phase 4B
// artifact-tracking columns), shared by every query that scans into a
// models.ReelPlan so the SELECT/RETURNING list and reelPlanScanDest stay in
// lockstep.
const reelPlanColumns = `
	id, workspace_id, daily_batch_id, trend_item_id, topic_score_id, rank, platform,
	title_idea, script_outline, description_draft, hashtags_draft, thumbnail_idea, status,
	video_artifact_path, video_format, video_width, video_height, video_duration_seconds, video_codec, audio_codec,
	thumbnail_artifact_path, thumbnail_format, thumbnail_width, thumbnail_height,
	export_status, export_error, created_at, updated_at`

// reelPlanScanDest returns the Scan() destination slice matching
// reelPlanColumns, in the same order.
func reelPlanScanDest(rp *models.ReelPlan) []any {
	return []any{
		&rp.ID, &rp.WorkspaceID, &rp.DailyBatchID, &rp.TrendItemID, &rp.TopicScoreID, &rp.Rank, &rp.Platform,
		&rp.TitleIdea, &rp.ScriptOutline, &rp.DescriptionDraft, &rp.HashtagsDraft, &rp.ThumbnailIdea, &rp.Status,
		&rp.VideoArtifactPath, &rp.VideoFormat, &rp.VideoWidth, &rp.VideoHeight, &rp.VideoDurationSeconds, &rp.VideoCodec, &rp.AudioCodec,
		&rp.ThumbnailArtifactPath, &rp.ThumbnailFormat, &rp.ThumbnailWidth, &rp.ThumbnailHeight,
		&rp.ExportStatus, &rp.ExportError, &rp.CreatedAt, &rp.UpdatedAt,
	}
}

func (s *Server) fetchReelPlansForBatch(ctx context.Context, batchID string) ([]models.ReelPlan, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+reelPlanColumns+`
		FROM reel_plans WHERE daily_batch_id = $1 ORDER BY rank ASC`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	plans := []models.ReelPlan{}
	for rows.Next() {
		var rp models.ReelPlan
		if err := rows.Scan(reelPlanScanDest(&rp)...); err != nil {
			return nil, err
		}
		plans = append(plans, rp)
	}
	return plans, nil
}

// GET /api/batches
func (s *Server) handleListDailyBatches(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, workspace_id, batch_date::text, status, reel_count, created_at, updated_at
		FROM daily_batches WHERE workspace_id = $1 ORDER BY batch_date DESC`, workspaceID)
	if err != nil {
		jsonError(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	batches := []models.DailyBatch{}
	for rows.Next() {
		var b models.DailyBatch
		if err := rows.Scan(&b.ID, &b.WorkspaceID, &b.BatchDate, &b.Status, &b.ReelCount, &b.CreatedAt, &b.UpdatedAt); err != nil {
			jsonError(w, "scan failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		batches = append(batches, b)
	}

	jsonOK(w, map[string]any{"daily_batches": batches})
}

// GET /api/batches/{id}
func (s *Server) handleGetDailyBatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	id := r.PathValue("id")

	var b models.DailyBatch
	err = s.db.QueryRowContext(ctx, `
		SELECT id, workspace_id, batch_date::text, status, reel_count, created_at, updated_at
		FROM daily_batches WHERE id = $1 AND workspace_id = $2`, id, workspaceID,
	).Scan(&b.ID, &b.WorkspaceID, &b.BatchDate, &b.Status, &b.ReelCount, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, "daily batch not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, b)
}

// GET /api/batches/{id}/reels
func (s *Server) handleListBatchReels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	id := r.PathValue("id")

	var exists string
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM daily_batches WHERE id = $1 AND workspace_id = $2`, id, workspaceID).Scan(&exists); err != nil {
		jsonError(w, "daily batch not found", http.StatusNotFound)
		return
	}

	plans, err := s.fetchReelPlansForBatch(ctx, id)
	if err != nil {
		jsonError(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, map[string]any{"reel_plans": plans})
}

// POST /api/batches/{id}/export is implemented in handlers_api_export.go —
// it needs the full reel artifact-checking + ZIP-building logic, which is
// large enough to warrant its own file.

// ── Draft content placeholders ───────────────────────────────────────────────
// Deterministic, clearly-labeled placeholders — never auto-published, always
// awaiting human review before anything leaves the database.

func draftTitleIdea(topic string) string {
	t := strings.TrimSpace(topic)
	if len(t) > 100 {
		t = t[:97] + "..."
	}
	return t
}

func draftScriptOutline(topic, description string) string {
	body := strings.TrimSpace(description)
	if body == "" {
		body = "Expand on: " + topic
	}
	return fmt.Sprintf("Hook (0-3s): %s\nBody: %s\nCTA (last 3s): Follow for more daily ideas.", topic, body)
}

func draftDescription(description string) string {
	d := strings.TrimSpace(description)
	if d == "" {
		return "Placeholder description — review and rewrite before publishing."
	}
	return d
}

func draftHashtags(topic string) string {
	words := strings.Fields(topic)
	tags := make([]string, 0, 4)
	for _, w := range words {
		clean := strings.ToLower(strings.Trim(w, ".,!?\"'"))
		if len(clean) < 3 {
			continue
		}
		tags = append(tags, "#"+clean)
		if len(tags) == 3 {
			break
		}
	}
	tags = append(tags, "#shorts")
	return strings.Join(tags, " ")
}

func draftThumbnailIdea(topic string) string {
	t := strings.TrimSpace(topic)
	if len(t) > 60 {
		t = t[:57] + "..."
	}
	return fmt.Sprintf("Bold text overlay: \"%s\" on a high-contrast background.", t)
}
