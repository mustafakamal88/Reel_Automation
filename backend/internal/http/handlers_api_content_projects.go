package http

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lib/pq"
)

type contentProject struct {
	ID                    string            `json:"id"`
	Title                 string            `json:"title"`
	Topic                 string            `json:"topic"`
	SourceType            string            `json:"source_type"`
	SourceReference       string            `json:"source_reference,omitempty"`
	SourceLabel           string            `json:"source_label,omitempty"`
	Status                string            `json:"status"`
	CurrentStage          string            `json:"current_stage"`
	TargetPlatforms       []string          `json:"target_platforms"`
	ContentFormat         string            `json:"content_format"`
	TargetDurationSeconds int               `json:"target_duration_seconds"`
	Language              string            `json:"language"`
	CreativeBrief         map[string]any    `json:"creative_brief,omitempty"`
	Hook                  string            `json:"hook,omitempty"`
	MainScript            string            `json:"main_script,omitempty"`
	Caption               string            `json:"caption,omitempty"`
	Hashtags              []string          `json:"hashtags"`
	PlatformText          map[string]string `json:"platform_text"`
	LegacyImported        bool              `json:"legacy_imported"`
	LegacySavedAt         *time.Time        `json:"legacy_saved_at,omitempty"`
	ArchivedAt            *time.Time        `json:"archived_at,omitempty"`
	CreatedAt             time.Time         `json:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at"`
}

type contentProjectRequest struct {
	Title                 string            `json:"title"`
	Topic                 string            `json:"topic"`
	SourceType            string            `json:"source_type"`
	SourceReference       string            `json:"source_reference"`
	SourceLabel           string            `json:"source_label"`
	Status                string            `json:"status"`
	CurrentStage          string            `json:"current_stage"`
	TargetPlatforms       []string          `json:"target_platforms"`
	ContentFormat         string            `json:"content_format"`
	TargetDurationSeconds int               `json:"target_duration_seconds"`
	Language              string            `json:"language"`
	CreativeBrief         map[string]any    `json:"creative_brief"`
	Hook                  string            `json:"hook"`
	MainScript            string            `json:"main_script"`
	Caption               string            `json:"caption"`
	Hashtags              []string          `json:"hashtags"`
	PlatformText          map[string]string `json:"platform_text"`
}

type legacyContentProjectImportRequest struct {
	ImportKey             string            `json:"import_key"`
	Title                 string            `json:"title"`
	Topic                 string            `json:"topic"`
	SourceType            string            `json:"source_type"`
	SourceReference       string            `json:"source_reference"`
	SourceLabel           string            `json:"source_label"`
	TargetPlatforms       []string          `json:"target_platforms"`
	ContentFormat         string            `json:"content_format"`
	TargetDurationSeconds int               `json:"target_duration_seconds"`
	Language              string            `json:"language"`
	Hook                  string            `json:"hook"`
	MainScript            string            `json:"main_script"`
	Caption               string            `json:"caption"`
	Hashtags              []string          `json:"hashtags"`
	PlatformText          map[string]string `json:"platform_text"`
	SavedAt               string            `json:"saved_at"`
}

var validProjectStatuses = map[string]bool{
	"idea": true, "brief_ready": true, "draft": true, "needs_review": true, "approved": true,
	"in_production": true, "rendered": true, "scheduled": true, "published": true, "archived": true,
}

var validProjectStages = map[string]bool{
	"idea": true, "brief": true, "script": true, "scenes": true, "voice": true,
	"video": true, "thumbnail": true, "publishing": true, "complete": true,
}

var validProjectPlatforms = map[string]bool{
	"youtube": true, "tiktok": true, "instagram": true, "facebook": true, "x": true, "threads": true,
}

var validProjectFormats = map[string]bool{
	"short_video": true, "long_video": true, "carousel": true, "post": true, "thread": true,
}

func (s *Server) handleContentProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListContentProjects(w, r)
	case http.MethodPost:
		s.handleCreateContentProject(w, r)
	default:
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleContentProjectRoute(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/content-projects/")
	id = strings.Trim(id, "/")
	if id == "" || strings.Contains(id, "/") || !looksLikeUUID(id) {
		jsonErrorCode(w, "not_found", "content project not found", http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleGetContentProject(w, r, id)
	case http.MethodPatch:
		s.handleUpdateContentProject(w, r, id)
	case http.MethodDelete:
		s.handleArchiveContentProject(w, r, id)
	default:
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleListContentProjects(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" && !validProjectStatuses[status] {
		jsonErrorCode(w, "validation_error", "invalid status filter", http.StatusBadRequest)
		return
	}
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	query := `
		SELECT id, title, topic, source_type, source_reference, source_label, status, current_stage,
			target_platforms, content_format, target_duration_seconds, language, creative_brief,
			hook, main_script, caption, hashtags, platform_text, legacy_import_key, legacy_saved_at,
			archived_at, created_at, updated_at
		FROM content_projects
		WHERE workspace_id = $1 AND archived_at IS NULL
			AND ($2 = '' OR status = $2)
			AND ($3 = '' OR title ILIKE '%' || $3 || '%' OR topic ILIKE '%' || $3 || '%')
		ORDER BY updated_at DESC, created_at DESC
		LIMIT 200`
	rows, err := s.db.QueryContext(r.Context(), query, workspaceID, status, search)
	if err != nil {
		jsonError(w, "content project list failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	projects := []contentProject{}
	for rows.Next() {
		project, err := scanContentProject(rows)
		if err != nil {
			jsonError(w, "content project scan failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		jsonError(w, "content project list failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"projects": projects})
}

func (s *Server) handleCreateContentProject(w http.ResponseWriter, r *http.Request) {
	var req contentProjectRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	req = normalizeContentProjectRequest(req, false)
	if err := validateContentProjectRequest(req, false); err != nil {
		jsonErrorCode(w, "validation_error", err.Error(), http.StatusBadRequest)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	project, err := s.insertContentProject(r, workspaceID, req, "", nil)
	if err != nil {
		jsonError(w, "content project create failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, project)
}

func (s *Server) handleGetContentProject(w http.ResponseWriter, r *http.Request, id string) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	project, err := s.getContentProject(r, workspaceID, id)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "content project not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "content project lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, project)
}

func (s *Server) handleUpdateContentProject(w http.ResponseWriter, r *http.Request, id string) {
	var req contentProjectRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	req = normalizeContentProjectRequest(req, true)
	if err := validateContentProjectRequest(req, true); err != nil {
		jsonErrorCode(w, "validation_error", err.Error(), http.StatusBadRequest)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	briefJSON, platformJSON, err := contentProjectJSON(req.CreativeBrief, req.PlatformText)
	if err != nil {
		jsonErrorCode(w, "validation_error", err.Error(), http.StatusBadRequest)
		return
	}
	query := `
		UPDATE content_projects
		SET title = $3, topic = $4, source_type = $5, source_reference = $6, source_label = $7,
			status = $8, current_stage = $9, target_platforms = $10, content_format = $11,
			target_duration_seconds = $12, language = $13, creative_brief = $14,
			hook = $15, main_script = $16, caption = $17, hashtags = $18, platform_text = $19,
			updated_at = NOW()
		WHERE workspace_id = $1 AND id = $2 AND archived_at IS NULL
		RETURNING id, title, topic, source_type, source_reference, source_label, status, current_stage,
			target_platforms, content_format, target_duration_seconds, language, creative_brief,
			hook, main_script, caption, hashtags, platform_text, legacy_import_key, legacy_saved_at,
			archived_at, created_at, updated_at`
	row := s.db.QueryRowContext(r.Context(), query, workspaceID, id, req.Title, req.Topic, req.SourceType,
		req.SourceReference, req.SourceLabel, req.Status, req.CurrentStage, pq.Array(req.TargetPlatforms),
		req.ContentFormat, req.TargetDurationSeconds, req.Language, briefJSON, req.Hook, req.MainScript,
		req.Caption, pq.Array(req.Hashtags), platformJSON)
	project, err := scanContentProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "content project not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "content project update failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, project)
}

func (s *Server) handleArchiveContentProject(w http.ResponseWriter, r *http.Request, id string) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	row := s.db.QueryRowContext(r.Context(), `
		UPDATE content_projects
		SET status = 'archived', archived_at = NOW(), updated_at = NOW()
		WHERE workspace_id = $1 AND id = $2 AND archived_at IS NULL
		RETURNING id, title, topic, source_type, source_reference, source_label, status, current_stage,
			target_platforms, content_format, target_duration_seconds, language, creative_brief,
			hook, main_script, caption, hashtags, platform_text, legacy_import_key, legacy_saved_at,
			archived_at, created_at, updated_at`, workspaceID, id)
	project, err := scanContentProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "content project not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "content project archive failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, project)
}

func (s *Server) handleImportLegacyContentProject(w http.ResponseWriter, r *http.Request) {
	var req legacyContentProjectImportRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	projectReq, importKey, savedAt, err := normalizeLegacyContentProjectImport(req)
	if err != nil {
		jsonErrorCode(w, "validation_error", err.Error(), http.StatusBadRequest)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	project, err := s.insertContentProject(r, workspaceID, projectReq, importKey, savedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			project, err = s.getContentProjectByLegacyKey(r, workspaceID, importKey)
		}
	}
	if err != nil {
		jsonError(w, "legacy content import failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, project)
}

func (s *Server) insertContentProject(r *http.Request, workspaceID string, req contentProjectRequest, legacyKey string, legacySavedAt *time.Time) (contentProject, error) {
	briefJSON, platformJSON, err := contentProjectJSON(req.CreativeBrief, req.PlatformText)
	if err != nil {
		return contentProject{}, err
	}
	row := s.db.QueryRowContext(r.Context(), `
		INSERT INTO content_projects (
			workspace_id, title, topic, source_type, source_reference, source_label, status, current_stage,
			target_platforms, content_format, target_duration_seconds, language, creative_brief,
			hook, main_script, caption, hashtags, platform_text, legacy_import_key, legacy_saved_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
		ON CONFLICT (workspace_id, legacy_import_key) WHERE legacy_import_key <> '' DO UPDATE
		SET updated_at = content_projects.updated_at
		RETURNING id, title, topic, source_type, source_reference, source_label, status, current_stage,
			target_platforms, content_format, target_duration_seconds, language, creative_brief,
			hook, main_script, caption, hashtags, platform_text, legacy_import_key, legacy_saved_at,
			archived_at, created_at, updated_at`,
		workspaceID, req.Title, req.Topic, req.SourceType, req.SourceReference, req.SourceLabel, req.Status,
		req.CurrentStage, pq.Array(req.TargetPlatforms), req.ContentFormat, req.TargetDurationSeconds,
		req.Language, briefJSON, req.Hook, req.MainScript, req.Caption, pq.Array(req.Hashtags),
		platformJSON, legacyKey, legacySavedAt)
	return scanContentProject(row)
}

func (s *Server) getContentProject(r *http.Request, workspaceID, id string) (contentProject, error) {
	row := s.db.QueryRowContext(r.Context(), `
		SELECT id, title, topic, source_type, source_reference, source_label, status, current_stage,
			target_platforms, content_format, target_duration_seconds, language, creative_brief,
			hook, main_script, caption, hashtags, platform_text, legacy_import_key, legacy_saved_at,
			archived_at, created_at, updated_at
		FROM content_projects
		WHERE workspace_id = $1 AND id = $2 AND archived_at IS NULL`, workspaceID, id)
	return scanContentProject(row)
}

func (s *Server) getContentProjectByLegacyKey(r *http.Request, workspaceID, key string) (contentProject, error) {
	row := s.db.QueryRowContext(r.Context(), `
		SELECT id, title, topic, source_type, source_reference, source_label, status, current_stage,
			target_platforms, content_format, target_duration_seconds, language, creative_brief,
			hook, main_script, caption, hashtags, platform_text, legacy_import_key, legacy_saved_at,
			archived_at, created_at, updated_at
		FROM content_projects
		WHERE workspace_id = $1 AND legacy_import_key = $2 AND archived_at IS NULL`, workspaceID, key)
	return scanContentProject(row)
}

type contentProjectScanner interface {
	Scan(dest ...any) error
}

func scanContentProject(row contentProjectScanner) (contentProject, error) {
	var p contentProject
	var targetPlatforms, hashtags pq.StringArray
	var briefRaw, platformRaw []byte
	var legacyKey string
	if err := row.Scan(&p.ID, &p.Title, &p.Topic, &p.SourceType, &p.SourceReference, &p.SourceLabel,
		&p.Status, &p.CurrentStage, &targetPlatforms, &p.ContentFormat, &p.TargetDurationSeconds,
		&p.Language, &briefRaw, &p.Hook, &p.MainScript, &p.Caption, &hashtags, &platformRaw,
		&legacyKey, &p.LegacySavedAt, &p.ArchivedAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return contentProject{}, err
	}
	p.TargetPlatforms = []string(targetPlatforms)
	p.Hashtags = []string(hashtags)
	p.LegacyImported = legacyKey != ""
	p.CreativeBrief = map[string]any{}
	if len(briefRaw) > 0 {
		_ = json.Unmarshal(briefRaw, &p.CreativeBrief)
	}
	p.PlatformText = map[string]string{}
	if len(platformRaw) > 0 {
		_ = json.Unmarshal(platformRaw, &p.PlatformText)
	}
	return p, nil
}

func normalizeContentProjectRequest(req contentProjectRequest, update bool) contentProjectRequest {
	req.Title = limitText(req.Title, 180)
	req.Topic = limitText(req.Topic, 220)
	req.SourceType = normalizeDefault(req.SourceType, "manual")
	req.SourceReference = limitText(req.SourceReference, 500)
	req.SourceLabel = limitText(req.SourceLabel, 80)
	req.Status = normalizeDefault(req.Status, "idea")
	req.CurrentStage = normalizeDefault(req.CurrentStage, "idea")
	req.TargetPlatforms = cleanProjectStrings(req.TargetPlatforms, 8, 32)
	req.ContentFormat = normalizeDefault(req.ContentFormat, "short_video")
	if req.TargetDurationSeconds == 0 {
		req.TargetDurationSeconds = 30
	}
	req.Language = limitText(normalizeDefault(req.Language, "en-US"), 32)
	req.Hook = limitText(req.Hook, 2000)
	req.MainScript = limitText(req.MainScript, 20000)
	req.Caption = limitText(req.Caption, 4000)
	req.Hashtags = cleanProjectStrings(req.Hashtags, 60, 80)
	req.PlatformText = cleanPlatformText(req.PlatformText)
	if update && req.CreativeBrief == nil {
		req.CreativeBrief = map[string]any{}
	}
	return req
}

func validateContentProjectRequest(req contentProjectRequest, update bool) error {
	if strings.TrimSpace(req.Title) == "" {
		return errors.New("title is required")
	}
	if len(req.Title) > 180 {
		return errors.New("title is too long")
	}
	if len(req.Topic) > 220 {
		return errors.New("topic is too long")
	}
	if !validProjectStatuses[req.Status] {
		return errors.New("invalid project status")
	}
	if !validProjectStages[req.CurrentStage] {
		return errors.New("invalid project stage")
	}
	if len(req.TargetPlatforms) == 0 {
		return errors.New("at least one target platform is required")
	}
	for _, platform := range req.TargetPlatforms {
		if !validProjectPlatforms[platform] {
			return fmt.Errorf("invalid target platform: %s", platform)
		}
	}
	if !validProjectFormats[req.ContentFormat] {
		return errors.New("invalid content format")
	}
	if req.TargetDurationSeconds < 5 || req.TargetDurationSeconds > 7200 {
		return errors.New("target_duration_seconds must be between 5 and 7200")
	}
	if len(req.Hook) > 2000 || len(req.MainScript) > 20000 || len(req.Caption) > 4000 {
		return errors.New("script fields exceed maximum length")
	}
	return nil
}

func normalizeLegacyContentProjectImport(req legacyContentProjectImportRequest) (contentProjectRequest, string, *time.Time, error) {
	projectReq := normalizeContentProjectRequest(contentProjectRequest{
		Title:                 req.Title,
		Topic:                 req.Topic,
		SourceType:            normalizeDefault(req.SourceType, "legacy_script_studio"),
		SourceReference:       req.SourceReference,
		SourceLabel:           req.SourceLabel,
		Status:                "draft",
		CurrentStage:          "script",
		TargetPlatforms:       req.TargetPlatforms,
		ContentFormat:         normalizeDefault(req.ContentFormat, "short_video"),
		TargetDurationSeconds: req.TargetDurationSeconds,
		Language:              req.Language,
		Hook:                  req.Hook,
		MainScript:            req.MainScript,
		Caption:               req.Caption,
		Hashtags:              req.Hashtags,
		PlatformText:          req.PlatformText,
	}, false)
	if strings.TrimSpace(projectReq.Title) == "" {
		projectReq.Title = firstNonEmptyContentProject(projectReq.Topic, projectReq.Hook, "Imported script")
	}
	if len(projectReq.TargetPlatforms) == 0 {
		projectReq.TargetPlatforms = []string{"youtube", "tiktok", "instagram"}
	}
	if !legacyImportHasContent(projectReq) {
		return contentProjectRequest{}, "", nil, errors.New("legacy content import requires hook, script, caption, hashtags, or platform text")
	}
	if err := validateContentProjectRequest(projectReq, false); err != nil {
		return contentProjectRequest{}, "", nil, err
	}
	importKey := strings.TrimSpace(req.ImportKey)
	if importKey == "" {
		importKey = legacyImportKey(projectReq, req.SavedAt)
	}
	var savedAt *time.Time
	if strings.TrimSpace(req.SavedAt) != "" {
		if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.SavedAt)); err == nil {
			savedAt = &parsed
		}
	}
	return projectReq, importKey, savedAt, nil
}

func legacyImportHasContent(req contentProjectRequest) bool {
	return strings.TrimSpace(req.Hook) != "" || strings.TrimSpace(req.MainScript) != "" ||
		strings.TrimSpace(req.Caption) != "" || len(req.Hashtags) > 0 || len(req.PlatformText) > 0
}

func legacyImportKey(req contentProjectRequest, savedAt string) string {
	hash := sha256.Sum256([]byte(strings.Join([]string{
		req.Title, req.Topic, req.Hook, req.MainScript, req.Caption, strings.Join(req.Hashtags, ","),
		strings.TrimSpace(savedAt),
	}, "\x00")))
	return hex.EncodeToString(hash[:])
}

func contentProjectJSON(brief map[string]any, platform map[string]string) ([]byte, []byte, error) {
	if brief == nil {
		brief = map[string]any{}
	}
	if platform == nil {
		platform = map[string]string{}
	}
	briefJSON, err := json.Marshal(brief)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid creative brief")
	}
	platformJSON, err := json.Marshal(platform)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid platform text")
	}
	if len(briefJSON) > 20000 || len(platformJSON) > 30000 {
		return nil, nil, fmt.Errorf("structured content is too large")
	}
	return briefJSON, platformJSON, nil
}

func cleanProjectStrings(values []string, maxItems, maxLen int) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		cleaned := normalizeProjectPlatform(strings.ToLower(strings.TrimSpace(value)))
		cleaned = strings.TrimPrefix(cleaned, "#")
		if cleaned == "" || seen[cleaned] {
			continue
		}
		if len(cleaned) > maxLen {
			cleaned = cleaned[:maxLen]
		}
		out = append(out, cleaned)
		seen[cleaned] = true
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

func normalizeProjectPlatform(value string) string {
	switch value {
	case "yt":
		return "youtube"
	case "tt":
		return "tiktok"
	case "ig":
		return "instagram"
	case "fb":
		return "facebook"
	default:
		return value
	}
}

func cleanPlatformText(values map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range values {
		cleanKey := strings.ToLower(strings.TrimSpace(key))
		if !validProjectPlatforms[cleanKey] {
			continue
		}
		out[cleanKey] = limitText(value, 6000)
	}
	return out
}

func normalizeDefault(value, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	return value
}

func limitText(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func firstNonEmptyContentProject(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func looksLikeUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, char := range value {
		switch i {
		case 8, 13, 18, 23:
			if char != '-' {
				return false
			}
		default:
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
				return false
			}
		}
	}
	return true
}
