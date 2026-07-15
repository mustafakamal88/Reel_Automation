package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"trendcortex/api/internal/blobstore"
)

const (
	projectOutputScopeProject = "project"
	projectOutputScopeScene   = "scene"

	projectOutputTypeGenerated = "generated_clip"
	projectOutputTypeUploaded  = "uploaded_clip"
	projectOutputTypeImported  = "imported_clip"
	projectOutputTypeRendered  = "rendered_video"

	projectOutputStatusQueued      = "queued"
	projectOutputStatusProcessing  = "processing"
	projectOutputStatusCompleted   = "completed"
	projectOutputStatusFailed      = "failed"
	projectOutputStatusUnavailable = "unavailable"
	projectOutputStatusArchived    = "archived"

	projectOutputWorkflowClipGenerator = "clip_generator"
	maxProjectOutputRetries            = 3
)

type contentProjectOutput struct {
	ID               string     `json:"id"`
	ProjectID        string     `json:"project_id"`
	SceneID          *string    `json:"scene_id,omitempty"`
	SceneLabel       string     `json:"scene_label,omitempty"`
	OutputScope      string     `json:"output_scope"`
	OutputType       string     `json:"output_type"`
	SourceWorkflow   string     `json:"source_workflow"`
	RenderJobID      string     `json:"render_job_id,omitempty"`
	Status           string     `json:"status"`
	OriginalFilename string     `json:"original_filename,omitempty"`
	DisplayName      string     `json:"display_name"`
	MimeType         string     `json:"mime_type,omitempty"`
	FileSizeBytes    *int64     `json:"file_size_bytes,omitempty"`
	DurationSeconds  *float64   `json:"duration_seconds,omitempty"`
	Width            *int       `json:"width,omitempty"`
	Height           *int       `json:"height,omitempty"`
	DownloadURL      string     `json:"download_url,omitempty"`
	OpenURL          string     `json:"open_url,omitempty"`
	FailureCategory  string     `json:"failure_category,omitempty"`
	FailureMessage   string     `json:"failure_message,omitempty"`
	Retryable        bool       `json:"retryable"`
	RetryCount       int        `json:"retry_count"`
	Available        bool       `json:"available"`
	ArchivedAt       *time.Time `json:"archived_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

type projectOutputRecord struct {
	contentProjectOutput
	StorageReference string
	StorageProvider  string
	StorageKey       string
	StorageETag      string
	StorageSHA256    string
	RequestPayload   []byte
}

type contentProjectOutputMutation struct {
	OutputScope        string
	OutputType         string
	SourceWorkflow     string
	RenderJobID        string
	Status             string
	OriginalFilename   string
	DisplayName        string
	MimeType           string
	FileSizeBytes      *int64
	DurationSeconds    *float64
	Width              *int
	Height             *int
	StorageReference   string
	StorageProvider    string
	StorageKey         string
	StorageETag        string
	StorageSHA256      string
	FailureCategory    string
	FailureMessage     string
	Retryable          bool
	RequestPayloadJSON []byte
}

func (s *Server) handleContentProjectOutputRoute(w http.ResponseWriter, r *http.Request, projectID string, parts []string) {
	if len(parts) == 0 {
		if r.Method != http.MethodGet {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleListContentProjectOutputs(w, r, projectID)
		return
	}
	outputID := parts[0]
	if !looksLikeUUID(outputID) {
		jsonErrorCode(w, "not_found", "project output not found", http.StatusNotFound)
		return
	}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			s.handleGetContentProjectOutput(w, r, projectID, outputID)
		case http.MethodDelete:
			s.handleArchiveContentProjectOutput(w, r, projectID, outputID)
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) == 2 && parts[1] == "retry" {
		if r.Method != http.MethodPost {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleRetryContentProjectOutput(w, r, projectID, outputID)
		return
	}
	if len(parts) == 2 && parts[1] == "download" {
		if r.Method != http.MethodGet {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDownloadContentProjectOutput(w, r, projectID, outputID)
		return
	}
	jsonErrorCode(w, "not_found", "project output route not found", http.StatusNotFound)
}

func (s *Server) handleListContentProjectOutputs(w http.ResponseWriter, r *http.Request, projectID string) {
	workspaceID, project, ok := s.requireActiveProject(w, r, projectID)
	if !ok {
		return
	}
	_ = workspaceID
	outputs, err := s.listContentProjectOutputs(r.Context(), project.ID)
	if err != nil {
		jsonError(w, "project output list failed", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"outputs": outputs})
}

func (s *Server) handleGetContentProjectOutput(w http.ResponseWriter, r *http.Request, projectID, outputID string) {
	_, project, ok := s.requireActiveProject(w, r, projectID)
	if !ok {
		return
	}
	output, err := s.getContentProjectOutput(r.Context(), project.ID, outputID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "project output not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "project output lookup failed", http.StatusInternalServerError)
		return
	}
	jsonOK(w, output.contentProjectOutput)
}

func (s *Server) handleArchiveContentProjectOutput(w http.ResponseWriter, r *http.Request, projectID, outputID string) {
	_, project, ok := s.requireActiveProject(w, r, projectID)
	if !ok {
		return
	}
	output, err := s.archiveContentProjectOutput(r.Context(), project.ID, outputID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "project output not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "project output archive failed", http.StatusInternalServerError)
		return
	}
	jsonOK(w, output.contentProjectOutput)
}

func (s *Server) handleRetryContentProjectOutput(w http.ResponseWriter, r *http.Request, projectID, outputID string) {
	workspaceID, project, ok := s.requireActiveProject(w, r, projectID)
	if !ok {
		return
	}
	output, err := s.getContentProjectOutput(r.Context(), project.ID, outputID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "project output not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "project output lookup failed", http.StatusInternalServerError)
		return
	}
	if output.Status != projectOutputStatusFailed || !output.Retryable {
		jsonErrorCode(w, "validation_error", "only retryable failed outputs can be retried", http.StatusConflict)
		return
	}
	if output.RetryCount >= maxProjectOutputRetries {
		jsonErrorCode(w, "validation_error", "retry limit reached for this output", http.StatusConflict)
		return
	}
	if len(output.RequestPayload) == 0 {
		jsonErrorCode(w, "validation_error", "this output cannot be retried; start a new generation", http.StatusConflict)
		return
	}
	var req clipStudioGenerateRequest
	if err := json.Unmarshal(output.RequestPayload, &req); err != nil {
		jsonErrorCode(w, "validation_error", "this output cannot be retried; start a new generation", http.StatusConflict)
		return
	}
	if err := s.markContentProjectOutputRetrying(r.Context(), project.ID, output.ID); err != nil {
		jsonError(w, "project output retry failed", http.StatusInternalServerError)
		return
	}
	result, err := s.performClipStudioGeneration(r.Context(), workspaceID, project.ID, &output.ID, req)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, result)
}

func (s *Server) handleDownloadContentProjectOutput(w http.ResponseWriter, r *http.Request, projectID, outputID string) {
	_, project, ok := s.requireActiveProject(w, r, projectID)
	if !ok {
		return
	}
	output, err := s.getContentProjectOutput(r.Context(), project.ID, outputID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "project output not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "project output lookup failed", http.StatusInternalServerError)
		return
	}
	if output.Status != projectOutputStatusCompleted {
		jsonErrorCode(w, "validation_error", "project output is not ready to download", http.StatusConflict)
		return
	}
	filename := safeDownloadFilename(output.DisplayName, output.MimeType)
	contentType := firstNonEmpty(output.MimeType, "application/octet-stream")
	if output.StorageProvider != "" && output.StorageKey != "" {
		store, err := s.ensureMediaStore()
		if err != nil {
			_ = s.markContentProjectOutputUnavailable(r.Context(), project.ID, output.ID)
			jsonErrorCode(w, "unavailable", "project output file is unavailable", http.StatusGone)
			return
		}
		if _, err := store.Stat(r.Context(), output.StorageKey); err != nil {
			if errors.Is(err, blobstore.ErrNotFound) {
				_ = s.markContentProjectOutputUnavailable(r.Context(), project.ID, output.ID)
				jsonErrorCode(w, "unavailable", "project output file is unavailable", http.StatusGone)
				return
			}
			jsonError(w, "project output storage lookup failed", http.StatusBadGateway)
			return
		}
		if output.StorageProvider == blobstore.ProviderS3 {
			u, err := store.PresignGet(r.Context(), output.StorageKey, blobstore.PresignOptions{
				TTL:                s.cfg.MediaStorageSignedURLTTL,
				ContentDisposition: `attachment; filename="` + filename + `"`,
			})
			if err != nil {
				jsonError(w, "project output download could not be prepared", http.StatusBadGateway)
				return
			}
			http.Redirect(w, r, u, http.StatusFound)
			return
		}
		body, _, err := store.Open(r.Context(), output.StorageKey)
		if err != nil {
			if errors.Is(err, blobstore.ErrNotFound) {
				_ = s.markContentProjectOutputUnavailable(r.Context(), project.ID, output.ID)
				jsonErrorCode(w, "unavailable", "project output file is unavailable", http.StatusGone)
				return
			}
			jsonError(w, "project output download failed", http.StatusBadGateway)
			return
		}
		defer body.Close()
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
		_, _ = io.Copy(w, body)
		return
	}
	path := filepath.Clean(output.StorageReference)
	if path == "." || path == "" {
		jsonErrorCode(w, "unavailable", "project output file is unavailable", http.StatusGone)
		return
	}
	if _, err := os.Stat(path); err != nil {
		_ = s.markContentProjectOutputUnavailable(r.Context(), project.ID, output.ID)
		jsonErrorCode(w, "unavailable", "project output file is unavailable", http.StatusGone)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	http.ServeFile(w, r, path)
}

func (s *Server) requireActiveProject(w http.ResponseWriter, r *http.Request, projectID string) (string, contentProject, bool) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return "", contentProject{}, false
	}
	project, err := s.getContentProject(r, workspaceID, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "content project not found", http.StatusNotFound)
		return "", contentProject{}, false
	}
	if err != nil {
		jsonError(w, "content project lookup failed", http.StatusInternalServerError)
		return "", contentProject{}, false
	}
	if project.ArchivedAt != nil {
		jsonErrorCode(w, "validation_error", "archived projects cannot be edited", http.StatusBadRequest)
		return "", contentProject{}, false
	}
	return workspaceID, project, true
}

func (s *Server) validateOutputProjectContext(ctx context.Context, workspaceID, projectID, scope string, sceneID *string) error {
	var archivedAt *time.Time
	if err := s.db.QueryRowContext(ctx, `SELECT archived_at FROM content_projects WHERE workspace_id = $1 AND id = $2`, workspaceID, projectID).Scan(&archivedAt); err != nil {
		return err
	}
	if archivedAt != nil {
		return errors.New("archived projects cannot receive outputs")
	}
	if scope == projectOutputScopeScene {
		if sceneID == nil || strings.TrimSpace(*sceneID) == "" {
			return errors.New("scene output requires a scene")
		}
		var exists bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM content_project_scenes WHERE project_id = $1 AND id = $2)`, projectID, *sceneID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return errors.New("scene does not belong to this project")
		}
		return nil
	}
	if sceneID != nil && strings.TrimSpace(*sceneID) != "" {
		return errors.New("project output cannot include a scene")
	}
	return nil
}

func (s *Server) upsertContentProjectOutput(ctx context.Context, workspaceID, projectID string, sceneID *string, m contentProjectOutputMutation) (projectOutputRecord, error) {
	if s.db == nil || strings.TrimSpace(projectID) == "" {
		return projectOutputRecord{}, nil
	}
	m = normalizeProjectOutputMutation(m)
	if err := validateProjectOutputMutation(m); err != nil {
		return projectOutputRecord{}, err
	}
	if err := s.validateOutputProjectContext(ctx, workspaceID, projectID, m.OutputScope, sceneID); err != nil {
		return projectOutputRecord{}, err
	}
	if err := s.advanceProjectToVideo(ctx, workspaceID, projectID); err != nil {
		return projectOutputRecord{}, err
	}
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO content_project_outputs (
			project_id, scene_id, output_scope, output_type, source_workflow, render_job_id, status,
			original_filename, display_name, mime_type, file_size_bytes, duration_seconds, width, height,
			storage_reference, storage_provider, storage_key, storage_etag, storage_checksum_sha256,
			failure_category, failure_message, retryable, request_payload, completed_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,
			CASE WHEN $7 = 'completed' THEN NOW() ELSE NULL END)
		ON CONFLICT (project_id, render_job_id) WHERE render_job_id IS NOT NULL AND render_job_id <> '' DO UPDATE SET
			status = EXCLUDED.status,
			original_filename = COALESCE(EXCLUDED.original_filename, content_project_outputs.original_filename),
			display_name = EXCLUDED.display_name,
			mime_type = COALESCE(EXCLUDED.mime_type, content_project_outputs.mime_type),
			file_size_bytes = COALESCE(EXCLUDED.file_size_bytes, content_project_outputs.file_size_bytes),
			duration_seconds = COALESCE(EXCLUDED.duration_seconds, content_project_outputs.duration_seconds),
			width = COALESCE(EXCLUDED.width, content_project_outputs.width),
			height = COALESCE(EXCLUDED.height, content_project_outputs.height),
			storage_reference = COALESCE(EXCLUDED.storage_reference, content_project_outputs.storage_reference),
			storage_provider = COALESCE(EXCLUDED.storage_provider, content_project_outputs.storage_provider),
			storage_key = COALESCE(EXCLUDED.storage_key, content_project_outputs.storage_key),
			storage_etag = COALESCE(EXCLUDED.storage_etag, content_project_outputs.storage_etag),
			storage_checksum_sha256 = COALESCE(EXCLUDED.storage_checksum_sha256, content_project_outputs.storage_checksum_sha256),
			failure_category = EXCLUDED.failure_category,
			failure_message = EXCLUDED.failure_message,
			retryable = EXCLUDED.retryable,
			request_payload = COALESCE(EXCLUDED.request_payload, content_project_outputs.request_payload),
			completed_at = CASE WHEN EXCLUDED.status = 'completed' THEN COALESCE(content_project_outputs.completed_at, NOW()) ELSE content_project_outputs.completed_at END,
			updated_at = NOW()
		RETURNING `+contentProjectOutputReturningColumns,
		projectID, sceneID, m.OutputScope, m.OutputType, m.SourceWorkflow, nullableString(m.RenderJobID), m.Status,
		nullableString(m.OriginalFilename), m.DisplayName, nullableString(m.MimeType), m.FileSizeBytes, m.DurationSeconds, m.Width, m.Height,
		nullableString(m.StorageReference), nullableString(m.StorageProvider), nullableString(m.StorageKey), nullableString(m.StorageETag), nullableString(m.StorageSHA256),
		nullableString(m.FailureCategory), nullableString(m.FailureMessage), m.Retryable, nullableJSON(m.RequestPayloadJSON),
	)
	out, err := scanContentProjectOutput(row, projectID)
	if err != nil {
		return projectOutputRecord{}, err
	}
	if err := s.syncProjectOutputAsset(ctx, workspaceID, projectID, sceneID, out, m); err != nil {
		return projectOutputRecord{}, err
	}
	return out, nil
}

func (s *Server) updateContentProjectOutput(ctx context.Context, projectID, outputID string, m contentProjectOutputMutation) (projectOutputRecord, error) {
	m = normalizeProjectOutputMutation(m)
	if err := validateProjectOutputMutation(m); err != nil {
		return projectOutputRecord{}, err
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE content_project_outputs SET
			status = $3,
			output_type = $4,
			display_name = $5,
			mime_type = $6,
			file_size_bytes = $7,
			duration_seconds = $8,
			width = $9,
			height = $10,
			storage_reference = $11,
			storage_provider = $12,
			storage_key = $13,
			storage_etag = $14,
			storage_checksum_sha256 = $15,
			failure_category = $16,
			failure_message = $17,
			retryable = $18,
			completed_at = CASE WHEN $3 = 'completed' THEN NOW() ELSE completed_at END,
			updated_at = NOW()
		WHERE project_id = $1 AND id = $2`,
		projectID, outputID, m.Status, m.OutputType, m.DisplayName, nullableString(m.MimeType), m.FileSizeBytes, m.DurationSeconds,
		m.Width, m.Height, nullableString(m.StorageReference), nullableString(m.StorageProvider), nullableString(m.StorageKey), nullableString(m.StorageETag), nullableString(m.StorageSHA256),
		nullableString(m.FailureCategory), nullableString(m.FailureMessage), m.Retryable,
	)
	if err != nil {
		return projectOutputRecord{}, err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return projectOutputRecord{}, err
	}
	if count == 0 {
		return projectOutputRecord{}, sql.ErrNoRows
	}
	out, err := s.getContentProjectOutput(ctx, projectID, outputID)
	if err != nil {
		return projectOutputRecord{}, err
	}
	var workspaceID string
	if err := s.db.QueryRowContext(ctx, `SELECT workspace_id FROM content_projects WHERE id = $1`, projectID).Scan(&workspaceID); err != nil {
		return projectOutputRecord{}, err
	}
	if err := s.syncProjectOutputAsset(ctx, workspaceID, projectID, out.SceneID, out, m); err != nil {
		return projectOutputRecord{}, err
	}
	return out, nil
}

func (s *Server) syncProjectOutputAsset(ctx context.Context, workspaceID, projectID string, sceneID *string, out projectOutputRecord, m contentProjectOutputMutation) error {
	if strings.TrimSpace(out.StorageProvider) == "" || strings.TrimSpace(out.StorageKey) == "" {
		return nil
	}
	status := mediaAssetStatusReady
	switch out.Status {
	case projectOutputStatusQueued, projectOutputStatusProcessing:
		status = mediaAssetStatusProcessing
	case projectOutputStatusFailed:
		status = mediaAssetStatusFailed
	case projectOutputStatusUnavailable:
		status = mediaAssetStatusUnavailable
	case projectOutputStatusArchived:
		status = mediaAssetStatusArchived
	}
	assetType := "generated_video"
	if out.OutputType == projectOutputTypeUploaded || out.OutputType == projectOutputTypeImported {
		assetType = "source_video"
	} else if out.OutputType == projectOutputTypeRendered {
		assetType = "rendered_video"
	} else if out.MimeType == "application/zip" {
		assetType = "package"
	}
	assetID, err := s.upsertMediaAsset(ctx, mediaAssetUpsert{
		WorkspaceID:     workspaceID,
		ProjectID:       projectID,
		SceneID:         sceneID,
		AssetType:       assetType,
		SourceWorkflow:  "clip_generator",
		DisplayName:     out.DisplayName,
		OriginalName:    out.OriginalFilename,
		MimeType:        out.MimeType,
		SizeBytes:       out.FileSizeBytes,
		DurationSeconds: out.DurationSeconds,
		Width:           out.Width,
		Height:          out.Height,
		StorageProvider: out.StorageProvider,
		StorageKey:      out.StorageKey,
		StorageETag:     out.StorageETag,
		StorageSHA256:   out.StorageSHA256,
		Status:          status,
		FailureCategory: out.FailureCategory,
		FailureMessage:  out.FailureMessage,
	})
	if err != nil || assetID == "" {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE content_project_outputs SET asset_id = $3, updated_at = NOW() WHERE project_id = $1 AND id = $2`, projectID, out.ID, assetID)
	_ = m
	return err
}

func (s *Server) listContentProjectOutputs(ctx context.Context, projectID string) ([]contentProjectOutput, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+contentProjectOutputSelectColumns+`
		FROM content_project_outputs o
		LEFT JOIN content_project_scenes sc ON sc.id = o.scene_id
		WHERE o.project_id = $1 AND o.archived_at IS NULL
		ORDER BY o.updated_at DESC, o.created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	outputs := []contentProjectOutput{}
	for rows.Next() {
		output, err := scanContentProjectOutput(rows, projectID)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, output.contentProjectOutput)
	}
	return outputs, rows.Err()
}

func (s *Server) getContentProjectOutput(ctx context.Context, projectID, outputID string) (projectOutputRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT `+contentProjectOutputSelectColumns+`
		FROM content_project_outputs o
		LEFT JOIN content_project_scenes sc ON sc.id = o.scene_id
		WHERE o.project_id = $1 AND o.id = $2 AND o.archived_at IS NULL`, projectID, outputID)
	return scanContentProjectOutput(row, projectID)
}

func (s *Server) archiveContentProjectOutput(ctx context.Context, projectID, outputID string) (projectOutputRecord, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE content_project_outputs
		SET status = 'archived', archived_at = NOW(), updated_at = NOW()
		WHERE project_id = $1 AND id = $2 AND archived_at IS NULL`, projectID, outputID)
	if err != nil {
		return projectOutputRecord{}, err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return projectOutputRecord{}, err
	}
	if count == 0 {
		return projectOutputRecord{}, sql.ErrNoRows
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT `+contentProjectOutputSelectColumns+`
		FROM content_project_outputs o
		LEFT JOIN content_project_scenes sc ON sc.id = o.scene_id
		WHERE o.project_id = $1 AND o.id = $2`, projectID, outputID)
	return scanContentProjectOutput(row, projectID)
}

func (s *Server) markContentProjectOutputRetrying(ctx context.Context, projectID, outputID string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE content_project_outputs
		SET status = 'processing', retry_count = retry_count + 1, retryable = false, updated_at = NOW()
		WHERE project_id = $1 AND id = $2 AND status = 'failed' AND retryable = true AND retry_count < $3`,
		projectID, outputID, maxProjectOutputRetries)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("output retry could not begin")
	}
	return nil
}

func (s *Server) markContentProjectOutputUnavailable(ctx context.Context, projectID, outputID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE content_project_outputs
		SET status = 'unavailable', retryable = false, updated_at = NOW()
		WHERE project_id = $1 AND id = $2 AND status = 'completed'`, projectID, outputID)
	return err
}

func (s *Server) advanceProjectToVideo(ctx context.Context, workspaceID, projectID string) error {
	stageRank := map[string]int{"idea": 0, "brief": 1, "script": 2, "scenes": 3, "voice": 4, "video": 5, "thumbnail": 6, "publishing": 7, "complete": 8}
	var current string
	if err := s.db.QueryRowContext(ctx, `SELECT current_stage FROM content_projects WHERE workspace_id = $1 AND id = $2 AND archived_at IS NULL`, workspaceID, projectID).Scan(&current); err != nil {
		return err
	}
	if stageRank[current] >= stageRank["video"] {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE content_projects SET current_stage = 'video', updated_at = NOW() WHERE workspace_id = $1 AND id = $2 AND archived_at IS NULL`, workspaceID, projectID)
	return err
}

const contentProjectOutputSelectColumns = `
	o.id, o.project_id, o.scene_id, COALESCE(sc.title, ''), o.output_scope, o.output_type, o.source_workflow,
	COALESCE(o.render_job_id, ''), o.status, COALESCE(o.original_filename, ''), o.display_name, COALESCE(o.mime_type, ''),
	o.file_size_bytes, o.duration_seconds, o.width, o.height, COALESCE(o.storage_reference, ''),
	COALESCE(o.storage_provider, ''), COALESCE(o.storage_key, ''), COALESCE(o.storage_etag, ''), COALESCE(o.storage_checksum_sha256, ''),
	COALESCE(o.failure_category, ''), COALESCE(o.failure_message, ''), o.retryable, o.retry_count,
	o.request_payload, o.archived_at, o.created_at, o.updated_at, o.completed_at`

const contentProjectOutputReturningColumns = `
	id, project_id, scene_id, '', output_scope, output_type, source_workflow,
	COALESCE(render_job_id, ''), status, COALESCE(original_filename, ''), display_name, COALESCE(mime_type, ''),
	file_size_bytes, duration_seconds, width, height, COALESCE(storage_reference, ''),
	COALESCE(storage_provider, ''), COALESCE(storage_key, ''), COALESCE(storage_etag, ''), COALESCE(storage_checksum_sha256, ''),
	COALESCE(failure_category, ''), COALESCE(failure_message, ''), retryable, retry_count,
	request_payload, archived_at, created_at, updated_at, completed_at`

type projectOutputScanner interface {
	Scan(dest ...any) error
}

func scanContentProjectOutput(row projectOutputScanner, projectID string) (projectOutputRecord, error) {
	var out projectOutputRecord
	var sceneID sql.NullString
	var fileSize sql.NullInt64
	var duration sql.NullFloat64
	var width, height sql.NullInt64
	var requestPayload []byte
	err := row.Scan(&out.ID, &out.ProjectID, &sceneID, &out.SceneLabel, &out.OutputScope, &out.OutputType, &out.SourceWorkflow,
		&out.RenderJobID, &out.Status, &out.OriginalFilename, &out.DisplayName, &out.MimeType,
		&fileSize, &duration, &width, &height, &out.StorageReference,
		&out.StorageProvider, &out.StorageKey, &out.StorageETag, &out.StorageSHA256,
		&out.FailureCategory, &out.FailureMessage, &out.Retryable, &out.RetryCount,
		&requestPayload, &out.ArchivedAt, &out.CreatedAt, &out.UpdatedAt, &out.CompletedAt)
	if err != nil {
		return projectOutputRecord{}, err
	}
	if sceneID.Valid {
		out.SceneID = &sceneID.String
	}
	if fileSize.Valid {
		out.FileSizeBytes = &fileSize.Int64
	}
	if duration.Valid {
		out.DurationSeconds = &duration.Float64
	}
	if width.Valid {
		v := int(width.Int64)
		out.Width = &v
	}
	if height.Valid {
		v := int(height.Int64)
		out.Height = &v
	}
	out.RequestPayload = requestPayload
	out.Available = outputStorageAvailable(out.Status, out.StorageReference, out.StorageProvider, out.StorageKey)
	if out.Status == projectOutputStatusCompleted && out.Available {
		out.DownloadURL = fmt.Sprintf("/api/content-projects/%s/outputs/%s/download", projectID, out.ID)
		out.OpenURL = out.DownloadURL
	}
	if out.Status == projectOutputStatusCompleted && !out.Available {
		out.Status = projectOutputStatusUnavailable
		out.FailureCategory = firstNonEmpty(out.FailureCategory, "file_unavailable")
		out.FailureMessage = firstNonEmpty(out.FailureMessage, "The output record is saved, but the generated file is no longer available.")
	}
	return out, nil
}

func normalizeProjectOutputMutation(m contentProjectOutputMutation) contentProjectOutputMutation {
	m.OutputScope = normalizeDefault(m.OutputScope, projectOutputScopeProject)
	m.OutputType = normalizeDefault(m.OutputType, projectOutputTypeGenerated)
	m.SourceWorkflow = normalizeDefault(m.SourceWorkflow, projectOutputWorkflowClipGenerator)
	m.Status = normalizeDefault(m.Status, projectOutputStatusQueued)
	m.DisplayName = limitText(strings.TrimSpace(m.DisplayName), 180)
	if m.DisplayName == "" {
		m.DisplayName = readableOutputType(m.OutputType)
	}
	m.OriginalFilename = limitText(strings.TrimSpace(m.OriginalFilename), 255)
	m.MimeType = limitText(strings.TrimSpace(m.MimeType), 120)
	m.FailureCategory = limitText(strings.TrimSpace(m.FailureCategory), 80)
	m.FailureMessage = safeFailureMessage(m.FailureMessage)
	m.RenderJobID = limitText(strings.TrimSpace(m.RenderJobID), 160)
	m.StorageReference = strings.TrimSpace(m.StorageReference)
	m.StorageProvider = limitText(strings.TrimSpace(m.StorageProvider), 20)
	m.StorageKey = strings.TrimSpace(m.StorageKey)
	m.StorageETag = limitText(strings.TrimSpace(m.StorageETag), 200)
	m.StorageSHA256 = limitText(strings.TrimSpace(m.StorageSHA256), 64)
	return m
}

func validateProjectOutputMutation(m contentProjectOutputMutation) error {
	if m.OutputScope != projectOutputScopeProject && m.OutputScope != projectOutputScopeScene {
		return errors.New("invalid output scope")
	}
	if !map[string]bool{projectOutputTypeGenerated: true, projectOutputTypeUploaded: true, projectOutputTypeImported: true, projectOutputTypeRendered: true}[m.OutputType] {
		return errors.New("invalid output type")
	}
	if !map[string]bool{projectOutputStatusQueued: true, projectOutputStatusProcessing: true, projectOutputStatusCompleted: true, projectOutputStatusFailed: true, projectOutputStatusUnavailable: true, projectOutputStatusArchived: true}[m.Status] {
		return errors.New("invalid output status")
	}
	if m.FileSizeBytes != nil && *m.FileSizeBytes < 0 {
		return errors.New("file size cannot be negative")
	}
	if m.DurationSeconds != nil && (*m.DurationSeconds < 0 || math.IsNaN(*m.DurationSeconds) || math.IsInf(*m.DurationSeconds, 0)) {
		return errors.New("duration cannot be negative")
	}
	if m.Width != nil && *m.Width < 0 {
		return errors.New("width cannot be negative")
	}
	if m.Height != nil && *m.Height < 0 {
		return errors.New("height cannot be negative")
	}
	if m.StorageProvider != "" && m.StorageProvider != "local" && m.StorageProvider != "s3" {
		return errors.New("invalid storage provider")
	}
	if m.StorageKey != "" {
		if strings.HasPrefix(m.StorageKey, "/") || strings.Contains(m.StorageKey, "\\") || strings.Contains(m.StorageKey, "..") {
			return errors.New("invalid storage key")
		}
	}
	if m.StorageSHA256 != "" && len(m.StorageSHA256) != 64 {
		return errors.New("invalid storage checksum")
	}
	return nil
}

func outputStorageAvailable(status, storageReference, storageProvider, storageKey string) bool {
	if status != projectOutputStatusCompleted {
		return false
	}
	if strings.TrimSpace(storageProvider) != "" && strings.TrimSpace(storageKey) != "" {
		return true
	}
	if strings.TrimSpace(storageReference) == "" {
		return false
	}
	_, err := os.Stat(storageReference)
	return err == nil
}

func safeFailureMessage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = limitText(value, 300)
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return value
}

func safeDownloadFilename(displayName, mimeType string) string {
	name := filepath.Base(strings.TrimSpace(displayName))
	if name == "." || name == "" {
		name = "trendcortex-output"
	}
	name = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '-'
		default:
			return r
		}
	}, name)
	ext := filepath.Ext(name)
	if ext == "" {
		switch mimeType {
		case "application/zip":
			name += ".zip"
		case "video/mp4":
			name += ".mp4"
		}
	}
	return name
}

func readableOutputType(outputType string) string {
	switch outputType {
	case projectOutputTypeUploaded:
		return "Uploaded clip"
	case projectOutputTypeImported:
		return "Imported clip"
	case projectOutputTypeRendered:
		return "Rendered video"
	default:
		return "Generated clips"
	}
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullableJSON(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func nullableInt64(value int64) any {
	if value < 0 {
		return nil
	}
	return value
}
