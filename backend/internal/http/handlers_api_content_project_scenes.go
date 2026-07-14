package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	maxSceneCount          = 40
	maxGeneratedSceneCount = 16
)

type contentProjectScene struct {
	ID                     string    `json:"id"`
	ProjectID              string    `json:"project_id"`
	Position               int       `json:"position"`
	Title                  string    `json:"title"`
	SpokenText             string    `json:"spoken_text"`
	OnScreenText           string    `json:"on_screen_text"`
	VisualDirection        string    `json:"visual_direction"`
	BrollDirection         string    `json:"broll_direction"`
	CameraDirection        string    `json:"camera_direction"`
	TransitionDirection    string    `json:"transition_direction"`
	PlannedDurationSeconds int       `json:"planned_duration_seconds"`
	ProductionNotes        string    `json:"production_notes"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type contentProjectSceneRequest struct {
	Position               int    `json:"position"`
	Title                  string `json:"title"`
	SpokenText             string `json:"spoken_text"`
	OnScreenText           string `json:"on_screen_text"`
	VisualDirection        string `json:"visual_direction"`
	BrollDirection         string `json:"broll_direction"`
	CameraDirection        string `json:"camera_direction"`
	TransitionDirection    string `json:"transition_direction"`
	PlannedDurationSeconds int    `json:"planned_duration_seconds"`
	ProductionNotes        string `json:"production_notes"`
}

type contentProjectScenePatchRequest struct {
	Position               *int    `json:"position"`
	Title                  *string `json:"title"`
	SpokenText             *string `json:"spoken_text"`
	OnScreenText           *string `json:"on_screen_text"`
	VisualDirection        *string `json:"visual_direction"`
	BrollDirection         *string `json:"broll_direction"`
	CameraDirection        *string `json:"camera_direction"`
	TransitionDirection    *string `json:"transition_direction"`
	PlannedDurationSeconds *int    `json:"planned_duration_seconds"`
	ProductionNotes        *string `json:"production_notes"`
}

type contentProjectSceneReorderRequest struct {
	SceneIDs []string `json:"scene_ids"`
}

type contentProjectSceneGenerateRequest struct {
	Mode string `json:"mode"`
}

type generatedScenePlan struct {
	Scenes []contentProjectSceneRequest `json:"scenes"`
}

func (s *Server) handleContentProjectSceneRoute(w http.ResponseWriter, r *http.Request, projectID string, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			s.handleListContentProjectScenes(w, r, projectID)
		case http.MethodPost:
			s.handleCreateContentProjectScene(w, r, projectID)
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) == 1 && parts[0] == "reorder" {
		if r.Method != http.MethodPost {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleReorderContentProjectScenes(w, r, projectID)
		return
	}
	if len(parts) == 1 && parts[0] == "generate" {
		if r.Method != http.MethodPost {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleGenerateContentProjectScenes(w, r, projectID)
		return
	}
	if len(parts) == 1 && looksLikeUUID(parts[0]) {
		switch r.Method {
		case http.MethodPatch:
			s.handleUpdateContentProjectScene(w, r, projectID, parts[0])
		case http.MethodDelete:
			s.handleDeleteContentProjectScene(w, r, projectID, parts[0])
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	jsonErrorCode(w, "not_found", "scene route not found", http.StatusNotFound)
}

func (s *Server) handleListContentProjectScenes(w http.ResponseWriter, r *http.Request, projectID string) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := s.getContentProject(r, workspaceID, projectID); errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "content project not found", http.StatusNotFound)
		return
	} else if err != nil {
		jsonError(w, "content project lookup failed", http.StatusInternalServerError)
		return
	}
	scenes, err := s.listContentProjectScenes(r.Context(), projectID)
	if err != nil {
		jsonError(w, "scene list failed", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"scenes": scenes})
}

func (s *Server) handleCreateContentProjectScene(w http.ResponseWriter, r *http.Request, projectID string) {
	var req contentProjectSceneRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	req = normalizeContentProjectSceneRequest(req)
	if err := validateContentProjectSceneRequest(req); err != nil {
		jsonErrorCode(w, "validation_error", err.Error(), http.StatusBadRequest)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	project, err := s.getContentProject(r, workspaceID, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "content project not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "content project lookup failed", http.StatusInternalServerError)
		return
	}
	if project.ArchivedAt != nil {
		jsonErrorCode(w, "validation_error", "archived projects cannot be edited", http.StatusBadRequest)
		return
	}
	scene, err := s.insertContentProjectScene(r.Context(), projectID, req)
	if err != nil {
		jsonError(w, "scene create failed", http.StatusInternalServerError)
		return
	}
	if err := s.advanceProjectToScenes(r.Context(), workspaceID, projectID); err != nil {
		jsonError(w, "project stage update failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, scene)
}

func (s *Server) handleUpdateContentProjectScene(w http.ResponseWriter, r *http.Request, projectID, sceneID string) {
	var req contentProjectScenePatchRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := s.getContentProject(r, workspaceID, projectID); errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "content project not found", http.StatusNotFound)
		return
	} else if err != nil {
		jsonError(w, "content project lookup failed", http.StatusInternalServerError)
		return
	}
	current, err := s.getContentProjectScene(r.Context(), projectID, sceneID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "scene not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "scene lookup failed", http.StatusInternalServerError)
		return
	}
	next := mergeContentProjectScenePatch(current, req)
	if err := validateContentProjectSceneRequest(sceneToRequest(next)); err != nil {
		jsonErrorCode(w, "validation_error", err.Error(), http.StatusBadRequest)
		return
	}
	scene, err := s.updateContentProjectScene(r.Context(), projectID, sceneID, next)
	if err != nil {
		jsonError(w, "scene update failed", http.StatusInternalServerError)
		return
	}
	jsonOK(w, scene)
}

func (s *Server) handleDeleteContentProjectScene(w http.ResponseWriter, r *http.Request, projectID, sceneID string) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := s.getContentProject(r, workspaceID, projectID); errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "content project not found", http.StatusNotFound)
		return
	} else if err != nil {
		jsonError(w, "content project lookup failed", http.StatusInternalServerError)
		return
	}
	res, err := s.db.ExecContext(r.Context(), `DELETE FROM content_project_scenes WHERE project_id = $1 AND id = $2`, projectID, sceneID)
	if err != nil {
		jsonError(w, "scene delete failed", http.StatusInternalServerError)
		return
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		jsonErrorCode(w, "not_found", "scene not found", http.StatusNotFound)
		return
	}
	scenes, err := s.compactContentProjectScenePositions(r.Context(), projectID)
	if err != nil {
		jsonError(w, "scene reorder failed", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"scenes": scenes})
}

func (s *Server) handleReorderContentProjectScenes(w http.ResponseWriter, r *http.Request, projectID string) {
	var req contentProjectSceneReorderRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateSceneIDList(req.SceneIDs); err != nil {
		jsonErrorCode(w, "validation_error", err.Error(), http.StatusBadRequest)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := s.getContentProject(r, workspaceID, projectID); errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "content project not found", http.StatusNotFound)
		return
	} else if err != nil {
		jsonError(w, "content project lookup failed", http.StatusInternalServerError)
		return
	}
	scenes, err := s.reorderContentProjectScenes(r.Context(), projectID, req.SceneIDs)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonErrorCode(w, "validation_error", "reorder list must include every scene exactly once", http.StatusBadRequest)
			return
		}
		jsonError(w, "scene reorder failed", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"scenes": scenes})
}

func (s *Server) handleGenerateContentProjectScenes(w http.ResponseWriter, r *http.Request, projectID string) {
	var req contentProjectSceneGenerateRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "replace"
	}
	if mode != "replace" && mode != "append" {
		jsonErrorCode(w, "validation_error", "generation mode must be replace or append", http.StatusBadRequest)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	project, err := s.getContentProject(r, workspaceID, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "content project not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "content project lookup failed", http.StatusInternalServerError)
		return
	}
	if strings.TrimSpace(project.Hook+"\n"+project.MainScript) == "" {
		jsonErrorCode(w, "validation_error", "save a script before generating a scene plan", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(s.cfg.OpenAIAPIKey) == "" {
		jsonErrorCode(w, "provider_unavailable", "scene plan generation is unavailable. Add scenes manually or try again later.", http.StatusServiceUnavailable)
		return
	}
	existing, err := s.listContentProjectScenes(r.Context(), projectID)
	if err != nil {
		jsonError(w, "scene list failed", http.StatusInternalServerError)
		return
	}
	generated, err := s.generateScenePlan(r.Context(), project)
	if err != nil {
		jsonErrorCode(w, "provider_unavailable", "scene plan generation is unavailable. Existing scenes were not changed.", http.StatusServiceUnavailable)
		return
	}
	if mode == "append" && len(existing)+len(generated) > maxSceneCount {
		jsonErrorCode(w, "validation_error", "scene plan would exceed the maximum scene count", http.StatusBadRequest)
		return
	}
	scenes, err := s.replaceOrAppendGeneratedScenes(r.Context(), projectID, mode, existing, generated)
	if err != nil {
		jsonError(w, "generated scene save failed", http.StatusInternalServerError)
		return
	}
	if err := s.advanceProjectToScenes(r.Context(), workspaceID, projectID); err != nil {
		jsonError(w, "project stage update failed", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"scenes": scenes})
}

func (s *Server) listContentProjectScenes(ctx context.Context, projectID string) ([]contentProjectScene, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, position, title, spoken_text, on_screen_text, visual_direction,
			broll_direction, camera_direction, transition_direction, planned_duration_seconds,
			production_notes, created_at, updated_at
		FROM content_project_scenes
		WHERE project_id = $1
		ORDER BY position ASC, created_at ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scenes := []contentProjectScene{}
	for rows.Next() {
		scene, err := scanContentProjectScene(rows)
		if err != nil {
			return nil, err
		}
		scenes = append(scenes, scene)
	}
	return scenes, rows.Err()
}

func (s *Server) getContentProjectScene(ctx context.Context, projectID, sceneID string) (contentProjectScene, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, position, title, spoken_text, on_screen_text, visual_direction,
			broll_direction, camera_direction, transition_direction, planned_duration_seconds,
			production_notes, created_at, updated_at
		FROM content_project_scenes
		WHERE project_id = $1 AND id = $2`, projectID, sceneID)
	return scanContentProjectScene(row)
}

func (s *Server) insertContentProjectScene(ctx context.Context, projectID string, req contentProjectSceneRequest) (contentProjectScene, error) {
	position := req.Position
	if position <= 0 {
		position = 1
		if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) + 1 FROM content_project_scenes WHERE project_id = $1`, projectID).Scan(&position); err != nil {
			return contentProjectScene{}, err
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE content_project_scenes SET position = position + 1, updated_at = NOW() WHERE project_id = $1 AND position >= $2`, projectID, position); err != nil {
		return contentProjectScene{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO content_project_scenes (
			project_id, position, title, spoken_text, on_screen_text, visual_direction,
			broll_direction, camera_direction, transition_direction, planned_duration_seconds, production_notes
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, project_id, position, title, spoken_text, on_screen_text, visual_direction,
			broll_direction, camera_direction, transition_direction, planned_duration_seconds,
			production_notes, created_at, updated_at`,
		projectID, position, req.Title, req.SpokenText, req.OnScreenText, req.VisualDirection,
		req.BrollDirection, req.CameraDirection, req.TransitionDirection, req.PlannedDurationSeconds, req.ProductionNotes)
	scene, err := scanContentProjectScene(row)
	if err != nil {
		return contentProjectScene{}, err
	}
	_, _ = s.compactContentProjectScenePositions(ctx, projectID)
	return scene, nil
}

func (s *Server) updateContentProjectScene(ctx context.Context, projectID, sceneID string, scene contentProjectScene) (contentProjectScene, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE content_project_scenes
		SET title = $3, spoken_text = $4, on_screen_text = $5, visual_direction = $6,
			broll_direction = $7, camera_direction = $8, transition_direction = $9,
			planned_duration_seconds = $10, production_notes = $11, updated_at = NOW()
		WHERE project_id = $1 AND id = $2
		RETURNING id, project_id, position, title, spoken_text, on_screen_text, visual_direction,
			broll_direction, camera_direction, transition_direction, planned_duration_seconds,
			production_notes, created_at, updated_at`,
		projectID, sceneID, scene.Title, scene.SpokenText, scene.OnScreenText, scene.VisualDirection,
		scene.BrollDirection, scene.CameraDirection, scene.TransitionDirection, scene.PlannedDurationSeconds, scene.ProductionNotes)
	return scanContentProjectScene(row)
}

func (s *Server) compactContentProjectScenePositions(ctx context.Context, projectID string) ([]contentProjectScene, error) {
	scenes, err := s.listContentProjectScenes(ctx, projectID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(scenes))
	for i, scene := range scenes {
		ids[i] = scene.ID
	}
	if len(ids) == 0 {
		return scenes, nil
	}
	return s.reorderContentProjectScenes(ctx, projectID, ids)
}

func (s *Server) reorderContentProjectScenes(ctx context.Context, projectID string, ids []string) ([]contentProjectScene, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM content_project_scenes WHERE project_id = $1`, projectID).Scan(&count); err != nil {
		return nil, err
	}
	if count != len(ids) {
		return nil, sql.ErrNoRows
	}
	for i, id := range ids {
		res, err := tx.ExecContext(ctx, `UPDATE content_project_scenes SET position = $3, updated_at = NOW() WHERE project_id = $1 AND id = $2`, projectID, id, i+1)
		if err != nil {
			return nil, err
		}
		if affected, _ := res.RowsAffected(); affected != 1 {
			return nil, sql.ErrNoRows
		}
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id, project_id, position, title, spoken_text, on_screen_text, visual_direction,
			broll_direction, camera_direction, transition_direction, planned_duration_seconds,
			production_notes, created_at, updated_at
		FROM content_project_scenes
		WHERE project_id = $1
		ORDER BY position ASC, created_at ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scenes := []contentProjectScene{}
	for rows.Next() {
		scene, err := scanContentProjectScene(rows)
		if err != nil {
			return nil, err
		}
		scenes = append(scenes, scene)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return scenes, nil
}

func (s *Server) replaceOrAppendGeneratedScenes(ctx context.Context, projectID, mode string, existing []contentProjectScene, generated []contentProjectSceneRequest) ([]contentProjectScene, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	start := 1
	if mode == "replace" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM content_project_scenes WHERE project_id = $1`, projectID); err != nil {
			return nil, err
		}
	} else {
		start = len(existing) + 1
	}
	for i, scene := range generated {
		scene.Position = start + i
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO content_project_scenes (
				project_id, position, title, spoken_text, on_screen_text, visual_direction,
				broll_direction, camera_direction, transition_direction, planned_duration_seconds, production_notes
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			projectID, scene.Position, scene.Title, scene.SpokenText, scene.OnScreenText, scene.VisualDirection,
			scene.BrollDirection, scene.CameraDirection, scene.TransitionDirection, scene.PlannedDurationSeconds, scene.ProductionNotes); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.listContentProjectScenes(ctx, projectID)
}

func (s *Server) advanceProjectToScenes(ctx context.Context, workspaceID, projectID string) error {
	stageRank := map[string]int{"idea": 0, "brief": 1, "script": 2, "scenes": 3, "voice": 4, "video": 5, "thumbnail": 6, "publishing": 7, "complete": 8}
	var current string
	if err := s.db.QueryRowContext(ctx, `SELECT current_stage FROM content_projects WHERE workspace_id = $1 AND id = $2 AND archived_at IS NULL`, workspaceID, projectID).Scan(&current); err != nil {
		return err
	}
	if stageRank[current] >= stageRank["scenes"] {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE content_projects SET current_stage = 'scenes', updated_at = NOW() WHERE workspace_id = $1 AND id = $2 AND archived_at IS NULL`, workspaceID, projectID)
	return err
}

func scanContentProjectScene(row interface{ Scan(dest ...any) error }) (contentProjectScene, error) {
	var scene contentProjectScene
	err := row.Scan(&scene.ID, &scene.ProjectID, &scene.Position, &scene.Title, &scene.SpokenText,
		&scene.OnScreenText, &scene.VisualDirection, &scene.BrollDirection, &scene.CameraDirection,
		&scene.TransitionDirection, &scene.PlannedDurationSeconds, &scene.ProductionNotes,
		&scene.CreatedAt, &scene.UpdatedAt)
	return scene, err
}

func normalizeContentProjectSceneRequest(req contentProjectSceneRequest) contentProjectSceneRequest {
	req.Title = strings.TrimSpace(req.Title)
	req.SpokenText = strings.TrimSpace(req.SpokenText)
	req.OnScreenText = strings.TrimSpace(req.OnScreenText)
	req.VisualDirection = strings.TrimSpace(req.VisualDirection)
	req.BrollDirection = strings.TrimSpace(req.BrollDirection)
	req.CameraDirection = strings.TrimSpace(req.CameraDirection)
	req.TransitionDirection = strings.TrimSpace(req.TransitionDirection)
	req.ProductionNotes = strings.TrimSpace(req.ProductionNotes)
	if req.PlannedDurationSeconds == 0 {
		req.PlannedDurationSeconds = 5
	}
	return req
}

func validateContentProjectSceneRequest(req contentProjectSceneRequest) error {
	if req.Position < 0 || req.Position > maxSceneCount {
		return errors.New("scene position is out of range")
	}
	if req.PlannedDurationSeconds < 1 {
		return errors.New("planned_duration_seconds must be positive")
	}
	if req.PlannedDurationSeconds > 600 {
		return errors.New("planned_duration_seconds must be 600 seconds or less")
	}
	if strings.TrimSpace(req.Title+req.SpokenText+req.OnScreenText+req.VisualDirection+req.BrollDirection+req.CameraDirection+req.TransitionDirection+req.ProductionNotes) == "" {
		return errors.New("scene requires at least one content field")
	}
	if len(req.Title) > 120 || len(req.SpokenText) > 5000 || len(req.OnScreenText) > 500 ||
		len(req.VisualDirection) > 2000 || len(req.BrollDirection) > 1200 ||
		len(req.CameraDirection) > 1000 || len(req.TransitionDirection) > 700 || len(req.ProductionNotes) > 1500 {
		return errors.New("scene fields exceed maximum length")
	}
	return nil
}

func validateSceneIDList(ids []string) error {
	if len(ids) == 0 || len(ids) > maxSceneCount {
		return errors.New("reorder list must include one to forty scenes")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !looksLikeUUID(id) {
			return errors.New("reorder includes an invalid scene id")
		}
		if seen[id] {
			return errors.New("reorder includes duplicate scene ids")
		}
		seen[id] = true
	}
	return nil
}

func mergeContentProjectScenePatch(current contentProjectScene, req contentProjectScenePatchRequest) contentProjectScene {
	if req.Position != nil {
		current.Position = *req.Position
	}
	if req.Title != nil {
		current.Title = strings.TrimSpace(*req.Title)
	}
	if req.SpokenText != nil {
		current.SpokenText = strings.TrimSpace(*req.SpokenText)
	}
	if req.OnScreenText != nil {
		current.OnScreenText = strings.TrimSpace(*req.OnScreenText)
	}
	if req.VisualDirection != nil {
		current.VisualDirection = strings.TrimSpace(*req.VisualDirection)
	}
	if req.BrollDirection != nil {
		current.BrollDirection = strings.TrimSpace(*req.BrollDirection)
	}
	if req.CameraDirection != nil {
		current.CameraDirection = strings.TrimSpace(*req.CameraDirection)
	}
	if req.TransitionDirection != nil {
		current.TransitionDirection = strings.TrimSpace(*req.TransitionDirection)
	}
	if req.PlannedDurationSeconds != nil {
		current.PlannedDurationSeconds = *req.PlannedDurationSeconds
	}
	if req.ProductionNotes != nil {
		current.ProductionNotes = strings.TrimSpace(*req.ProductionNotes)
	}
	return current
}

func sceneToRequest(scene contentProjectScene) contentProjectSceneRequest {
	return contentProjectSceneRequest{
		Position:               scene.Position,
		Title:                  scene.Title,
		SpokenText:             scene.SpokenText,
		OnScreenText:           scene.OnScreenText,
		VisualDirection:        scene.VisualDirection,
		BrollDirection:         scene.BrollDirection,
		CameraDirection:        scene.CameraDirection,
		TransitionDirection:    scene.TransitionDirection,
		PlannedDurationSeconds: scene.PlannedDurationSeconds,
		ProductionNotes:        scene.ProductionNotes,
	}
}

func (s *Server) generateScenePlan(ctx context.Context, project contentProject) ([]contentProjectSceneRequest, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	script := strings.TrimSpace(strings.Join([]string{project.Hook, project.MainScript}, "\n\n"))
	prompt := map[string]any{
		"task":                    "Turn the saved creator script into an ordered production scene plan.",
		"title":                   project.Title,
		"topic":                   project.Topic,
		"hook":                    project.Hook,
		"script":                  script,
		"target_duration_seconds": project.TargetDurationSeconds,
		"content_format":          project.ContentFormat,
		"target_platforms":        project.TargetPlatforms,
		"rules": []string{
			"Use only the saved hook and script as source material.",
			"Return 3 to 12 scenes unless the script clearly needs fewer or more.",
			"Every scene needs positive planned_duration_seconds.",
			"Keep total planned duration close to target_duration_seconds.",
			"Use concise production language suitable for a creator preparing a short video.",
		},
	}
	promptJSON, err := json.MarshalIndent(prompt, "", "  ")
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"model": strings.TrimSpace(s.cfg.OpenAITextModel),
		"messages": []map[string]string{
			{"role": "system", "content": "You create production scene plans from saved scripts. Return valid JSON only and do not include provider details."},
			{"role": "user", "content": string(promptJSON)},
		},
		"temperature":     0.35,
		"response_format": scenePlanResponseFormat(),
	}
	if strings.TrimSpace(s.cfg.OpenAITextModel) == "" {
		body["model"] = "gpt-4o-mini"
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+s.cfg.OpenAIAPIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 90 * time.Second}).Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	resBody, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("scene provider returned HTTP %d", res.StatusCode)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(resBody, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Choices) == 0 {
		return nil, errors.New("empty scene provider response")
	}
	var plan generatedScenePlan
	if err := json.Unmarshal([]byte(parsed.Choices[0].Message.Content), &plan); err != nil {
		return nil, err
	}
	return normalizeGeneratedScenePlan(plan.Scenes, project.TargetDurationSeconds)
}

func normalizeGeneratedScenePlan(input []contentProjectSceneRequest, target int) ([]contentProjectSceneRequest, error) {
	if len(input) == 0 || len(input) > maxGeneratedSceneCount {
		return nil, errors.New("generated scene count is out of range")
	}
	scenes := make([]contentProjectSceneRequest, 0, len(input))
	total := 0
	for i, scene := range input {
		scene = normalizeContentProjectSceneRequest(scene)
		scene.Position = i + 1
		if scene.PlannedDurationSeconds < 1 {
			scene.PlannedDurationSeconds = 5
		}
		if scene.PlannedDurationSeconds > 120 {
			scene.PlannedDurationSeconds = 120
		}
		if err := validateContentProjectSceneRequest(scene); err != nil {
			return nil, err
		}
		total += scene.PlannedDurationSeconds
		scenes = append(scenes, scene)
	}
	if target >= 5 && total > 0 {
		ratio := float64(target) / float64(total)
		if ratio < 0.65 || ratio > 1.35 {
			total = 0
			for i := range scenes {
				scenes[i].PlannedDurationSeconds = int(math.Round(float64(scenes[i].PlannedDurationSeconds) * ratio))
				if scenes[i].PlannedDurationSeconds < 1 {
					scenes[i].PlannedDurationSeconds = 1
				}
				if scenes[i].PlannedDurationSeconds > 120 {
					scenes[i].PlannedDurationSeconds = 120
				}
				total += scenes[i].PlannedDurationSeconds
			}
		}
	}
	return scenes, nil
}

func scenePlanResponseFormat() map[string]any {
	stringSchema := map[string]any{"type": "string"}
	sceneProperties := map[string]any{
		"title":                    stringSchema,
		"spoken_text":              stringSchema,
		"on_screen_text":           stringSchema,
		"visual_direction":         stringSchema,
		"broll_direction":          stringSchema,
		"camera_direction":         stringSchema,
		"transition_direction":     stringSchema,
		"planned_duration_seconds": map[string]any{"type": "integer"},
		"production_notes":         stringSchema,
	}
	return map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "scene_plan",
			"strict": true,
			"schema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"scenes": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"properties":           sceneProperties,
							"required":             []string{"title", "spoken_text", "on_screen_text", "visual_direction", "broll_direction", "camera_direction", "transition_direction", "planned_duration_seconds", "production_notes"},
						},
					},
				},
				"required": []string{"scenes"},
			},
		},
	}
}

func sortScenesByPosition(scenes []contentProjectScene) {
	sort.SliceStable(scenes, func(i, j int) bool {
		if scenes[i].Position == scenes[j].Position {
			return scenes[i].CreatedAt.Before(scenes[j].CreatedAt)
		}
		return scenes[i].Position < scenes[j].Position
	})
}
