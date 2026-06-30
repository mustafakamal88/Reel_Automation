package http

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/lib/pq"
	"trendcortex/api/internal/models"
	"trendcortex/api/internal/scoring"
)

type scoreRequest struct {
	TrendItemIDs []string `json:"trend_item_ids"`
}

type scoredTrendItemRow struct {
	ID               string
	Topic            string
	Description      string
	PlatformHint     string
	Velocity         float64
	SourceConfidence float64
}

// handleScoreTopics computes a deterministic score for the requested trend
// items (or every "new" trend item in the workspace if none are given) and
// upserts the result into topic_scores.
//
// POST /api/topics/score
func (s *Server) handleScoreTopics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var req scoreRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	var rows *sql.Rows
	if len(req.TrendItemIDs) > 0 {
		rows, err = s.db.QueryContext(ctx, `
			SELECT ti.id, ti.topic, ti.description, ti.platform_hint, ti.velocity, ts.confidence
			FROM trend_items ti
			JOIN trend_sources ts ON ts.id = ti.trend_source_id
			WHERE ti.workspace_id = $1 AND ti.id = ANY($2)`, workspaceID, pq.Array(req.TrendItemIDs))
	} else {
		rows, err = s.db.QueryContext(ctx, `
			SELECT ti.id, ti.topic, ti.description, ti.platform_hint, ti.velocity, ts.confidence
			FROM trend_items ti
			JOIN trend_sources ts ON ts.id = ti.trend_source_id
			WHERE ti.workspace_id = $1 AND ti.status = $2`, workspaceID, models.TrendItemStatusNew)
	}
	if err != nil {
		jsonError(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var candidates []scoredTrendItemRow
	for rows.Next() {
		var c scoredTrendItemRow
		if err := rows.Scan(&c.ID, &c.Topic, &c.Description, &c.PlatformHint, &c.Velocity, &c.SourceConfidence); err != nil {
			jsonError(w, "scan failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		candidates = append(candidates, c)
	}

	results := make([]models.TopicScore, 0, len(candidates))
	for _, c := range candidates {
		result := scoring.Score(scoring.Input{
			Topic:            c.Topic,
			Description:      c.Description,
			PlatformHint:     c.PlatformHint,
			Velocity:         c.Velocity,
			SourceConfidence: c.SourceConfidence,
		})

		breakdown, _ := json.Marshal(map[string]float64{
			"velocity_score":          result.VelocityScore,
			"source_confidence_score": result.SourceConfidenceScore,
			"platform_fit_score":      result.PlatformFitScore,
			"safety_score":            result.SafetyScore,
			"watch_time_score":        result.WatchTimeScore,
			"competition_score":       result.CompetitionScore,
		})

		var ts models.TopicScore
		err := s.db.QueryRowContext(ctx, `
			INSERT INTO topic_scores
				(workspace_id, trend_item_id, total_score, velocity_score, source_confidence_score,
				 platform_fit_score, safety_score, watch_time_score, competition_score, reason, breakdown)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (trend_item_id) DO UPDATE SET
				total_score = EXCLUDED.total_score,
				velocity_score = EXCLUDED.velocity_score,
				source_confidence_score = EXCLUDED.source_confidence_score,
				platform_fit_score = EXCLUDED.platform_fit_score,
				safety_score = EXCLUDED.safety_score,
				watch_time_score = EXCLUDED.watch_time_score,
				competition_score = EXCLUDED.competition_score,
				reason = EXCLUDED.reason,
				breakdown = EXCLUDED.breakdown,
				created_at = NOW()
			RETURNING id, workspace_id, trend_item_id, total_score, velocity_score, source_confidence_score,
				platform_fit_score, safety_score, watch_time_score, competition_score, reason, breakdown, created_at`,
			workspaceID, c.ID, result.TotalScore, result.VelocityScore, result.SourceConfidenceScore,
			result.PlatformFitScore, result.SafetyScore, result.WatchTimeScore, result.CompetitionScore,
			result.Reason, breakdown,
		).Scan(&ts.ID, &ts.WorkspaceID, &ts.TrendItemID, &ts.TotalScore, &ts.VelocityScore, &ts.SourceConfidenceScore,
			&ts.PlatformFitScore, &ts.SafetyScore, &ts.WatchTimeScore, &ts.CompetitionScore, &ts.Reason, &ts.BreakdownJSON, &ts.CreatedAt)
		if err != nil {
			jsonError(w, "upsert score failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		newStatus := models.TrendItemStatusScored
		if result.Flagged {
			newStatus = models.TrendItemStatusRejected
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE trend_items SET status = $1, updated_at = NOW() WHERE id = $2`, newStatus, c.ID); err != nil {
			jsonError(w, "trend item status update failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		results = append(results, ts)
	}

	jsonOK(w, map[string]any{"scored": len(results), "topic_scores": topicScoresWithBreakdown(results)})
}

// GET /api/topics/scores
func (s *Server) handleListTopicScores(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT ts.id, ts.workspace_id, ts.trend_item_id, ts.total_score, ts.velocity_score, ts.source_confidence_score,
			ts.platform_fit_score, ts.safety_score, ts.watch_time_score, ts.competition_score, ts.reason, ts.breakdown, ts.created_at
		FROM topic_scores ts
		WHERE ts.workspace_id = $1
		ORDER BY ts.total_score DESC`, workspaceID)
	if err != nil {
		jsonError(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	scores := []models.TopicScore{}
	for rows.Next() {
		var ts models.TopicScore
		if err := rows.Scan(&ts.ID, &ts.WorkspaceID, &ts.TrendItemID, &ts.TotalScore, &ts.VelocityScore, &ts.SourceConfidenceScore,
			&ts.PlatformFitScore, &ts.SafetyScore, &ts.WatchTimeScore, &ts.CompetitionScore, &ts.Reason, &ts.BreakdownJSON, &ts.CreatedAt); err != nil {
			jsonError(w, "scan failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		scores = append(scores, ts)
	}

	jsonOK(w, map[string]any{"topic_scores": topicScoresWithBreakdown(scores)})
}

// topicScoreJSON re-shapes a TopicScore so the raw JSONB breakdown column
// renders as a nested object instead of a base64 byte string.
type topicScoreJSON struct {
	models.TopicScore
	Breakdown json.RawMessage `json:"breakdown"`
}

func topicScoresWithBreakdown(scores []models.TopicScore) []topicScoreJSON {
	out := make([]topicScoreJSON, 0, len(scores))
	for _, ts := range scores {
		out = append(out, topicScoreJSON{TopicScore: ts, Breakdown: json.RawMessage(ts.BreakdownJSON)})
	}
	return out
}
