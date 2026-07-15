package http

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"trendcortex/api/internal/blobstore"
	"trendcortex/api/internal/renderer"
)

type movieEdit struct {
	ID                       string           `json:"id"`
	WorkspaceID              string           `json:"workspace_id"`
	ContentProjectID         string           `json:"content_project_id,omitempty"`
	Name                     string           `json:"name"`
	Status                   string           `json:"status"`
	Version                  int              `json:"version"`
	TimelineSchemaVersion    int              `json:"timeline_schema_version"`
	AspectRatio              string           `json:"aspect_ratio"`
	OutputWidth              int              `json:"output_width"`
	OutputHeight             int              `json:"output_height"`
	FrameRate                int              `json:"frame_rate"`
	QualityPreset            string           `json:"quality_preset"`
	VoiceoverAssetID         string           `json:"voiceover_asset_id,omitempty"`
	MusicAssetID             string           `json:"music_asset_id,omitempty"`
	CaptionSettings          map[string]any   `json:"caption_settings"`
	TransitionSettings       map[string]any   `json:"transition_settings"`
	BrandingSettings         map[string]any   `json:"branding_settings"`
	DurationSeconds          *float64         `json:"duration_seconds,omitempty"`
	LatestSuccessfulRenderID string           `json:"latest_successful_render_id,omitempty"`
	Scenes                   []movieScene     `json:"scenes"`
	Renders                  []movieRenderJob `json:"renders,omitempty"`
	CreatedAt                time.Time        `json:"created_at"`
	UpdatedAt                time.Time        `json:"updated_at"`
}

type movieScene struct {
	ID                        string   `json:"id"`
	MovieEditID               string   `json:"movie_edit_id"`
	SourceProjectSceneID      string   `json:"source_project_scene_id,omitempty"`
	Position                  int      `json:"position"`
	Title                     string   `json:"title"`
	ScriptText                string   `json:"script_text"`
	VoiceoverAssetID          string   `json:"voiceover_asset_id,omitempty"`
	VisualAssetID             string   `json:"visual_asset_id,omitempty"`
	SecondaryAssetID          string   `json:"secondary_asset_id,omitempty"`
	StartSeconds              float64  `json:"start_seconds"`
	DurationSeconds           float64  `json:"duration_seconds"`
	TrimInSeconds             *float64 `json:"trim_in_seconds,omitempty"`
	TrimOutSeconds            *float64 `json:"trim_out_seconds,omitempty"`
	FitMode                   string   `json:"fit_mode"`
	FocalX                    float64  `json:"focal_x"`
	FocalY                    float64  `json:"focal_y"`
	Zoom                      float64  `json:"zoom"`
	MotionPreset              string   `json:"motion_preset"`
	TransitionType            string   `json:"transition_type"`
	TransitionDurationSeconds float64  `json:"transition_duration_seconds"`
	Muted                     bool     `json:"muted"`
	Volume                    float64  `json:"volume"`
	CaptionText               string   `json:"caption_text"`
	MatchReason               string   `json:"match_reason,omitempty"`
	MatchConfidence           *float64 `json:"match_confidence,omitempty"`
}

type movieRenderJob struct {
	ID                    string     `json:"id"`
	MovieEditID           string     `json:"movie_edit_id"`
	Status                string     `json:"status"`
	CurrentStage          string     `json:"current_stage"`
	CompletedSceneCount   int        `json:"completed_scene_count"`
	TotalSceneCount       int        `json:"total_scene_count"`
	OutputAssetID         string     `json:"output_asset_id,omitempty"`
	OutputDurationSeconds *float64   `json:"output_duration_seconds,omitempty"`
	OutputSizeBytes       *int64     `json:"output_size_bytes,omitempty"`
	OutputWidth           *int       `json:"output_width,omitempty"`
	OutputHeight          *int       `json:"output_height,omitempty"`
	OutputVideoCodec      string     `json:"output_video_codec,omitempty"`
	OutputAudioCodec      string     `json:"output_audio_codec,omitempty"`
	QualityPreset         string     `json:"quality_preset"`
	FailureMessage        string     `json:"failure_message,omitempty"`
	StartedAt             *time.Time `json:"started_at,omitempty"`
	CompletedAt           *time.Time `json:"completed_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type movieEditRequest struct {
	ContentProjectID string         `json:"content_project_id"`
	Name             string         `json:"name"`
	QualityPreset    string         `json:"quality_preset"`
	VoiceoverAssetID string         `json:"voiceover_asset_id"`
	MusicAssetID     string         `json:"music_asset_id"`
	CaptionSettings  map[string]any `json:"caption_settings"`
	BrandingSettings map[string]any `json:"branding_settings"`
	Scenes           []movieScene   `json:"scenes"`
}

func (s *Server) handleMovieStudioRoute(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/movie-studio/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		jsonErrorCode(w, "not_found", "movie route not found", http.StatusNotFound)
		return
	}
	switch parts[0] {
	case "edits":
		s.handleMovieEditRoute(w, r, parts[1:])
	case "renders":
		s.handleMovieRenderRoute(w, r, parts[1:])
	case "uploads":
		if r.Method == http.MethodPost && len(parts) == 1 {
			s.handleMovieUpload(w, r)
			return
		}
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
	default:
		jsonErrorCode(w, "not_found", "movie route not found", http.StatusNotFound)
	}
}

func (s *Server) handleMovieEditRoute(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) == 0 || parts[0] == "" {
		switch r.Method {
		case http.MethodGet:
			s.handleListMovieEdits(w, r)
		case http.MethodPost:
			s.handleCreateMovieEdit(w, r)
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	editID := parts[0]
	if !looksLikeUUID(editID) {
		jsonErrorCode(w, "not_found", "movie edit not found", http.StatusNotFound)
		return
	}
	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			s.handleGetMovieEdit(w, r, editID)
		case http.MethodPatch:
			s.handleUpdateMovieEdit(w, r, editID)
		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	if len(parts) == 2 {
		switch parts[1] {
		case "auto-draft":
			if r.Method == http.MethodPost {
				s.handleAutoDraftMovieEdit(w, r, editID)
				return
			}
		case "duplicate":
			if r.Method == http.MethodPost {
				s.handleDuplicateMovieEdit(w, r, editID)
				return
			}
		case "render":
			if r.Method == http.MethodPost {
				s.handleRenderMovieEdit(w, r, editID)
				return
			}
		}
	}
	jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleMovieRenderRoute(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) == 1 && looksLikeUUID(parts[0]) && r.Method == http.MethodGet {
		s.handleGetMovieRender(w, r, parts[0])
		return
	}
	if len(parts) == 2 && looksLikeUUID(parts[0]) && parts[1] == "download" && r.Method == http.MethodGet {
		s.handleDownloadMovieRender(w, r, parts[0])
		return
	}
	jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleListMovieEdits(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	projectID := strings.TrimSpace(r.URL.Query().Get("project_id"))
	where := "workspace_id = $1"
	args := []any{workspaceID}
	if looksLikeUUID(projectID) {
		where += " AND content_project_id = $2"
		args = append(args, projectID)
	}
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id, workspace_id, COALESCE(content_project_id::text, ''), name, status, version, timeline_schema_version,
			aspect_ratio, output_width, output_height, frame_rate, quality_preset,
			COALESCE(voiceover_asset_id::text, ''), COALESCE(music_asset_id::text, ''),
			caption_settings, transition_settings, branding_settings, duration_seconds,
			COALESCE(latest_successful_render_id::text, ''), created_at, updated_at
		FROM movie_edits WHERE `+where+` ORDER BY updated_at DESC LIMIT 100`, args...)
	if err != nil {
		jsonError(w, "movie edits unavailable", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	edits := []movieEdit{}
	for rows.Next() {
		edit, err := scanMovieEdit(rows)
		if err != nil {
			jsonError(w, "movie edit scan failed", http.StatusInternalServerError)
			return
		}
		edits = append(edits, edit)
	}
	jsonOK(w, map[string]any{"edits": edits})
}

func (s *Server) handleCreateMovieEdit(w http.ResponseWriter, r *http.Request) {
	var req movieEditRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	if req.ContentProjectID != "" {
		if _, err := s.getContentProject(r, workspaceID, req.ContentProjectID); errors.Is(err, sql.ErrNoRows) {
			jsonErrorCode(w, "not_found", "content project not found", http.StatusNotFound)
			return
		} else if err != nil {
			jsonError(w, "project lookup failed", http.StatusInternalServerError)
			return
		}
	}
	edit, err := s.insertMovieEdit(r, workspaceID, req)
	if err != nil {
		jsonError(w, "movie edit create failed", http.StatusInternalServerError)
		return
	}
	if req.ContentProjectID != "" {
		_ = s.buildAutoDraftMovieScenes(r, workspaceID, edit.ID, req.ContentProjectID)
		_ = s.db.QueryRowContext(r.Context(), `UPDATE content_projects SET active_movie_edit_id = $3, current_stage = 'video', updated_at = NOW() WHERE workspace_id = $1 AND id = $2 RETURNING id`, workspaceID, req.ContentProjectID, edit.ID).Scan(new(string))
		edit, _ = s.loadMovieEdit(r, workspaceID, edit.ID)
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]any{"edit": edit})
}

func (s *Server) handleGetMovieEdit(w http.ResponseWriter, r *http.Request, editID string) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	edit, err := s.loadMovieEdit(r, workspaceID, editID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "movie edit not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "movie edit lookup failed", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"edit": edit})
}

func (s *Server) handleUpdateMovieEdit(w http.ResponseWriter, r *http.Request, editID string) {
	var req movieEditRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	if _, err := s.loadMovieEdit(r, workspaceID, editID); errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "movie edit not found", http.StatusNotFound)
		return
	} else if err != nil {
		jsonError(w, "movie edit lookup failed", http.StatusInternalServerError)
		return
	}
	if err := s.replaceMovieEditScenes(r, workspaceID, editID, req); err != nil {
		jsonErrorCode(w, "validation_error", err.Error(), http.StatusBadRequest)
		return
	}
	edit, err := s.loadMovieEdit(r, workspaceID, editID)
	if err != nil {
		jsonError(w, "movie edit update failed", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"edit": edit})
}

func (s *Server) handleAutoDraftMovieEdit(w http.ResponseWriter, r *http.Request, editID string) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	edit, err := s.loadMovieEdit(r, workspaceID, editID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "movie edit not found", http.StatusNotFound)
		return
	}
	if edit.ContentProjectID == "" {
		jsonErrorCode(w, "validation_error", "choose a content project before creating an automatic draft", http.StatusBadRequest)
		return
	}
	if err := s.buildAutoDraftMovieScenes(r, workspaceID, edit.ID, edit.ContentProjectID); err != nil {
		jsonError(w, "automatic draft failed", http.StatusInternalServerError)
		return
	}
	edit, _ = s.loadMovieEdit(r, workspaceID, editID)
	jsonOK(w, map[string]any{"edit": edit})
}

func (s *Server) handleDuplicateMovieEdit(w http.ResponseWriter, r *http.Request, editID string) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	source, err := s.loadMovieEdit(r, workspaceID, editID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "movie edit not found", http.StatusNotFound)
		return
	}
	req := movieEditRequest{ContentProjectID: source.ContentProjectID, Name: source.Name + " copy", QualityPreset: source.QualityPreset, VoiceoverAssetID: source.VoiceoverAssetID, MusicAssetID: source.MusicAssetID, CaptionSettings: source.CaptionSettings, BrandingSettings: source.BrandingSettings, Scenes: source.Scenes}
	copyEdit, err := s.insertMovieEdit(r, workspaceID, req)
	if err != nil {
		jsonError(w, "duplicate failed", http.StatusInternalServerError)
		return
	}
	_ = s.replaceMovieEditScenes(r, workspaceID, copyEdit.ID, req)
	copyEdit, _ = s.loadMovieEdit(r, workspaceID, copyEdit.ID)
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]any{"edit": copyEdit})
}

func (s *Server) handleRenderMovieEdit(w http.ResponseWriter, r *http.Request, editID string) {
	var req struct {
		IdempotencyKey string `json:"idempotency_key"`
	}
	_ = decodeJSONBody(r, &req)
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	edit, err := s.loadMovieEdit(r, workspaceID, editID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "movie edit not found", http.StatusNotFound)
		return
	}
	if len(edit.Scenes) == 0 {
		jsonErrorCode(w, "validation_error", "Add at least one scene before rendering.", http.StatusBadRequest)
		return
	}
	job, err := s.createMovieRenderJob(r, workspaceID, edit, req.IdempotencyKey)
	if err != nil {
		jsonError(w, "render job could not be created", http.StatusInternalServerError)
		return
	}
	if job.Status == "completed" {
		jsonOK(w, map[string]any{"render": job})
		return
	}
	job = s.runMovieRenderJob(w, r, workspaceID, edit, job)
	if job.ID == "" {
		return
	}
	status := http.StatusCreated
	if job.Status != "completed" {
		status = http.StatusAccepted
	}
	w.WriteHeader(status)
	jsonOK(w, map[string]any{"render": job})
}

func (s *Server) handleGetMovieRender(w http.ResponseWriter, r *http.Request, renderID string) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	job, err := s.getMovieRenderJob(r, workspaceID, renderID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "render not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "render lookup failed", http.StatusInternalServerError)
		return
	}
	jsonOK(w, map[string]any{"render": job})
}

func (s *Server) handleDownloadMovieRender(w http.ResponseWriter, r *http.Request, renderID string) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	job, err := s.getMovieRenderJob(r, workspaceID, renderID)
	if errors.Is(err, sql.ErrNoRows) || job.OutputAssetID == "" {
		jsonErrorCode(w, "not_found", "render output not found", http.StatusNotFound)
		return
	}
	s.serveAssetObject(w, r, job.OutputAssetID, false)
}

func (s *Server) handleMovieUpload(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	if projectID != "" {
		if _, err := s.getContentProject(r, workspaceID, projectID); err != nil {
			jsonErrorCode(w, "not_found", "content project not found", http.StatusNotFound)
			return
		}
	}
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		jsonErrorCode(w, "validation_error", "Upload a supported media file under the size limit.", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		jsonErrorCode(w, "validation_error", "Choose a media file to upload.", http.StatusBadRequest)
		return
	}
	defer file.Close()
	ct := header.Header.Get("Content-Type")
	if !movieUploadContentTypeAllowed(ct) {
		jsonErrorCode(w, "validation_error", "This media format cannot be processed. Upload an MP4, MOV, WebM, MP3, WAV, PNG, or JPEG file.", http.StatusUnsupportedMediaType)
		return
	}
	store, err := s.ensureMediaStore()
	if err != nil {
		jsonError(w, "media storage unavailable", http.StatusBadGateway)
		return
	}
	key, err := blobstore.JoinKey("movie-studio", workspaceID, strconv.FormatInt(time.Now().UnixNano(), 10)+"-"+blobstore.SafeSegment(header.Filename, "upload"))
	if err != nil {
		jsonError(w, "upload could not be prepared", http.StatusInternalServerError)
		return
	}
	info, err := store.Put(r.Context(), key, file, blobstore.PutOptions{ContentType: ct, OriginalFilename: header.Filename})
	if err != nil {
		jsonError(w, "upload failed", http.StatusBadGateway)
		return
	}
	assetType := "source_video"
	if strings.HasPrefix(ct, "image/") {
		assetType = "thumbnail"
	}
	if strings.HasPrefix(ct, "audio/") {
		assetType = "audio"
	}
	assetID, err := s.upsertMediaAsset(r.Context(), mediaAssetUpsert{
		WorkspaceID: workspaceID, ProjectID: projectID, AssetType: assetType, SourceWorkflow: "movie_studio",
		DisplayName: header.Filename, OriginalName: header.Filename, MimeType: ct, SizeBytes: &info.Size,
		StorageProvider: info.Provider, StorageKey: info.Key, StorageETag: info.ETag, StorageSHA256: info.SHA256, Status: mediaAssetStatusReady,
	})
	if err != nil {
		_ = store.Delete(r.Context(), key)
		jsonError(w, "asset could not be saved", http.StatusInternalServerError)
		return
	}
	asset, _ := s.getMediaAsset(r.Context(), workspaceID, assetID, true)
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]publicMediaAsset{"asset": asset.public()})
}

func (s *Server) handleSetActiveMovieEdit(w http.ResponseWriter, r *http.Request, projectID string) {
	var req struct {
		EditID string `json:"edit_id"`
	}
	if err := decodeJSONBody(r, &req); err != nil || !looksLikeUUID(req.EditID) {
		jsonErrorCode(w, "validation_error", "Choose a valid movie edit.", http.StatusBadRequest)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	if _, err := s.loadMovieEdit(r, workspaceID, req.EditID); errors.Is(err, sql.ErrNoRows) {
		jsonErrorCode(w, "not_found", "movie edit not found", http.StatusNotFound)
		return
	} else if err != nil {
		jsonError(w, "movie edit lookup failed", http.StatusInternalServerError)
		return
	}
	res, err := s.db.ExecContext(r.Context(), `
		UPDATE content_projects SET active_movie_edit_id = $3, current_stage = 'video', updated_at = NOW()
		WHERE workspace_id = $1 AND id = $2 AND archived_at IS NULL`, workspaceID, projectID, req.EditID)
	if err != nil {
		jsonError(w, "active movie edit could not be saved", http.StatusInternalServerError)
		return
	}
	if count, _ := res.RowsAffected(); count == 0 {
		jsonErrorCode(w, "not_found", "content project not found", http.StatusNotFound)
		return
	}
	jsonOK(w, map[string]string{"active_movie_edit_id": req.EditID})
}

func (s *Server) handleSetActiveProjectVideo(w http.ResponseWriter, r *http.Request, projectID string) {
	var req struct {
		AssetID string `json:"asset_id"`
	}
	if err := decodeJSONBody(r, &req); err != nil || !looksLikeUUID(req.AssetID) {
		jsonErrorCode(w, "validation_error", "Choose a valid rendered video.", http.StatusBadRequest)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed", http.StatusInternalServerError)
		return
	}
	asset, err := s.getMediaAsset(r.Context(), workspaceID, req.AssetID, false)
	if errors.Is(err, sql.ErrNoRows) || asset.AssetType != "rendered_video" {
		jsonErrorCode(w, "not_found", "rendered video not found", http.StatusNotFound)
		return
	}
	res, err := s.db.ExecContext(r.Context(), `
		UPDATE content_projects SET active_rendered_video_asset_id = $3, status = 'rendered', current_stage = 'video', updated_at = NOW()
		WHERE workspace_id = $1 AND id = $2 AND archived_at IS NULL`, workspaceID, projectID, req.AssetID)
	if err != nil {
		jsonError(w, "active video could not be saved", http.StatusInternalServerError)
		return
	}
	if count, _ := res.RowsAffected(); count == 0 {
		jsonErrorCode(w, "not_found", "content project not found", http.StatusNotFound)
		return
	}
	jsonOK(w, map[string]string{"active_rendered_video_asset_id": req.AssetID})
}

func (s *Server) insertMovieEdit(r *http.Request, workspaceID string, req movieEditRequest) (movieEdit, error) {
	name := limitText(firstNonEmpty(req.Name, "Movie draft"), 180)
	quality := normalizeMovieQuality(req.QualityPreset)
	captions, _ := json.Marshal(defaultMovieMap(req.CaptionSettings))
	branding, _ := json.Marshal(defaultMovieMap(req.BrandingSettings))
	emptyJSON, _ := json.Marshal(map[string]any{})
	row := s.db.QueryRowContext(r.Context(), `
		INSERT INTO movie_edits (workspace_id, content_project_id, name, quality_preset, voiceover_asset_id, music_asset_id, caption_settings, transition_settings, branding_settings)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, workspace_id, COALESCE(content_project_id::text, ''), name, status, version, timeline_schema_version,
			aspect_ratio, output_width, output_height, frame_rate, quality_preset, COALESCE(voiceover_asset_id::text, ''),
			COALESCE(music_asset_id::text, ''), caption_settings, transition_settings, branding_settings, duration_seconds,
			COALESCE(latest_successful_render_id::text, ''), created_at, updated_at`,
		workspaceID, nullableString(req.ContentProjectID), name, quality, nullableString(req.VoiceoverAssetID), nullableString(req.MusicAssetID), captions, emptyJSON, branding)
	return scanMovieEdit(row)
}

func (s *Server) loadMovieEdit(r *http.Request, workspaceID, editID string) (movieEdit, error) {
	row := s.db.QueryRowContext(r.Context(), `
		SELECT id, workspace_id, COALESCE(content_project_id::text, ''), name, status, version, timeline_schema_version,
			aspect_ratio, output_width, output_height, frame_rate, quality_preset, COALESCE(voiceover_asset_id::text, ''),
			COALESCE(music_asset_id::text, ''), caption_settings, transition_settings, branding_settings, duration_seconds,
			COALESCE(latest_successful_render_id::text, ''), created_at, updated_at
		FROM movie_edits WHERE workspace_id = $1 AND id = $2`, workspaceID, editID)
	edit, err := scanMovieEdit(row)
	if err != nil {
		return movieEdit{}, err
	}
	edit.Scenes, err = s.listMovieScenes(r, edit.ID)
	if err != nil {
		return movieEdit{}, err
	}
	edit.Renders, _ = s.listMovieRenders(r, workspaceID, edit.ID)
	return edit, nil
}

func scanMovieEdit(row interface{ Scan(dest ...any) error }) (movieEdit, error) {
	var e movieEdit
	var duration sql.NullFloat64
	var captionRaw, transitionRaw, brandingRaw []byte
	err := row.Scan(&e.ID, &e.WorkspaceID, &e.ContentProjectID, &e.Name, &e.Status, &e.Version, &e.TimelineSchemaVersion, &e.AspectRatio, &e.OutputWidth, &e.OutputHeight, &e.FrameRate, &e.QualityPreset, &e.VoiceoverAssetID, &e.MusicAssetID, &captionRaw, &transitionRaw, &brandingRaw, &duration, &e.LatestSuccessfulRenderID, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return movieEdit{}, err
	}
	if duration.Valid {
		e.DurationSeconds = &duration.Float64
	}
	e.CaptionSettings = map[string]any{}
	e.TransitionSettings = map[string]any{}
	e.BrandingSettings = map[string]any{}
	_ = jsonUnmarshalMap(captionRaw, &e.CaptionSettings)
	_ = jsonUnmarshalMap(transitionRaw, &e.TransitionSettings)
	_ = jsonUnmarshalMap(brandingRaw, &e.BrandingSettings)
	return e, nil
}

func (s *Server) listMovieScenes(r *http.Request, editID string) ([]movieScene, error) {
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id, movie_edit_id, COALESCE(source_project_scene_id::text, ''), position, title, script_text,
			COALESCE(voiceover_asset_id::text, ''), COALESCE(visual_asset_id::text, ''), COALESCE(secondary_asset_id::text, ''),
			start_seconds, duration_seconds, trim_in_seconds, trim_out_seconds, fit_mode, focal_x, focal_y, zoom,
			motion_preset, transition_type, transition_duration_seconds, muted, volume, caption_text, match_reason, match_confidence
		FROM movie_scenes WHERE movie_edit_id = $1 ORDER BY position, id`, editID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scenes := []movieScene{}
	for rows.Next() {
		var sc movieScene
		var trimIn, trimOut, confidence sql.NullFloat64
		if err := rows.Scan(&sc.ID, &sc.MovieEditID, &sc.SourceProjectSceneID, &sc.Position, &sc.Title, &sc.ScriptText, &sc.VoiceoverAssetID, &sc.VisualAssetID, &sc.SecondaryAssetID, &sc.StartSeconds, &sc.DurationSeconds, &trimIn, &trimOut, &sc.FitMode, &sc.FocalX, &sc.FocalY, &sc.Zoom, &sc.MotionPreset, &sc.TransitionType, &sc.TransitionDurationSeconds, &sc.Muted, &sc.Volume, &sc.CaptionText, &sc.MatchReason, &confidence); err != nil {
			return nil, err
		}
		if trimIn.Valid {
			sc.TrimInSeconds = &trimIn.Float64
		}
		if trimOut.Valid {
			sc.TrimOutSeconds = &trimOut.Float64
		}
		if confidence.Valid {
			sc.MatchConfidence = &confidence.Float64
		}
		scenes = append(scenes, sc)
	}
	return scenes, rows.Err()
}

func (s *Server) replaceMovieEditScenes(r *http.Request, workspaceID, editID string, req movieEditRequest) error {
	if len(req.Scenes) > 40 {
		return errors.New("Movie edits support up to 40 scenes.")
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	captions, _ := json.Marshal(defaultMovieMap(req.CaptionSettings))
	branding, _ := json.Marshal(defaultMovieMap(req.BrandingSettings))
	duration := 0.0
	for i := range req.Scenes {
		if req.Scenes[i].DurationSeconds <= 0 {
			return errors.New("Each scene needs a positive duration.")
		}
		duration += req.Scenes[i].DurationSeconds
	}
	if _, err := tx.ExecContext(r.Context(), `
		UPDATE movie_edits SET name = COALESCE(NULLIF($3,''), name), quality_preset = $4, voiceover_asset_id = $5, music_asset_id = $6,
			caption_settings = $7, branding_settings = $8, duration_seconds = $9, updated_at = NOW()
		WHERE workspace_id = $1 AND id = $2`, workspaceID, editID, limitText(req.Name, 180), normalizeMovieQuality(req.QualityPreset), nullableString(req.VoiceoverAssetID), nullableString(req.MusicAssetID), captions, branding, duration); err != nil {
		return err
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM movie_scenes WHERE movie_edit_id = $1`, editID); err != nil {
		return err
	}
	start := 0.0
	for i, sc := range req.Scenes {
		if sc.VisualAssetID != "" {
			if _, err := s.getMediaAsset(r.Context(), workspaceID, sc.VisualAssetID, false); err != nil {
				return fmt.Errorf("Scene %d needs an available visual before saving.", i+1)
			}
		}
		_, err := tx.ExecContext(r.Context(), `
			INSERT INTO movie_scenes (movie_edit_id, source_project_scene_id, position, title, script_text, voiceover_asset_id, visual_asset_id, secondary_asset_id,
				start_seconds, duration_seconds, trim_in_seconds, trim_out_seconds, fit_mode, focal_x, focal_y, zoom, motion_preset, transition_type, transition_duration_seconds,
				muted, volume, caption_text, match_reason, match_confidence)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)`,
			editID, nullableString(sc.SourceProjectSceneID), i+1, limitText(sc.Title, 240), limitText(sc.ScriptText, 6000), nullableString(sc.VoiceoverAssetID), nullableString(sc.VisualAssetID), nullableString(sc.SecondaryAssetID), start, sc.DurationSeconds, sc.TrimInSeconds, sc.TrimOutSeconds, normalizeFitMode(sc.FitMode), sc.FocalX, sc.FocalY, defaultFloat(sc.Zoom, 1), normalizeMotion(sc.MotionPreset), normalizeTransition(sc.TransitionType), clampFloat(sc.TransitionDurationSeconds, 0, 2), sc.Muted, defaultFloat(sc.Volume, 1), limitText(sc.CaptionText, 2000), limitText(sc.MatchReason, 300), sc.MatchConfidence)
		if err != nil {
			return err
		}
		start += sc.DurationSeconds
	}
	return tx.Commit()
}

func (s *Server) buildAutoDraftMovieScenes(r *http.Request, workspaceID, editID, projectID string) error {
	scenes, err := s.listContentProjectScenes(r.Context(), projectID)
	if err != nil {
		return err
	}
	assetsResp, err := s.listMediaAssets(r.Context(), workspaceID, map[string][]string{"project_id": {projectID}, "limit": {"100"}})
	if err != nil {
		return err
	}
	var voiceoverID string
	_ = s.db.QueryRowContext(r.Context(), `SELECT COALESCE(active_voiceover_asset_id::text, '') FROM content_projects WHERE workspace_id = $1 AND id = $2`, workspaceID, projectID).Scan(&voiceoverID)
	visuals := []publicMediaAsset{}
	for _, asset := range assetsResp.Assets {
		if asset.Status == "ready" && (strings.Contains(asset.AssetType, "video") || strings.HasPrefix(asset.MimeType, "image/") || asset.AssetType == "thumbnail") {
			visuals = append(visuals, asset)
		}
	}
	req := movieEditRequest{VoiceoverAssetID: voiceoverID, QualityPreset: "standard", CaptionSettings: map[string]any{"enabled": true, "mode": "phrase"}, BrandingSettings: map[string]any{}}
	for i, src := range scenes {
		duration := float64(src.PlannedDurationSeconds)
		if duration <= 0 {
			duration = estimateSceneDuration(src.SpokenText)
		}
		ms := movieScene{SourceProjectSceneID: src.ID, Position: i + 1, Title: firstNonEmpty(src.Title, fmt.Sprintf("Scene %d", i+1)), ScriptText: src.SpokenText, DurationSeconds: duration, FitMode: "fill_crop", MotionPreset: "slow_zoom_in", TransitionType: "cut", TransitionDurationSeconds: 0.25, Muted: true, Volume: 1, CaptionText: firstNonEmpty(src.OnScreenText, src.SpokenText)}
		if i < len(visuals) {
			ms.VisualAssetID = visuals[i].ID
			ms.MatchReason = "Matched by project association and scene order."
			c := 0.55
			ms.MatchConfidence = &c
		}
		req.Scenes = append(req.Scenes, ms)
	}
	return s.replaceMovieEditScenes(r, workspaceID, editID, req)
}

func (s *Server) createMovieRenderJob(r *http.Request, workspaceID string, edit movieEdit, idempotencyKey string) (movieRenderJob, error) {
	idempotencyKey = limitText(strings.TrimSpace(idempotencyKey), 160)
	if idempotencyKey != "" {
		if existing, err := s.getMovieRenderByIdempotency(r, workspaceID, edit.ID, idempotencyKey); err == nil {
			return existing, nil
		}
	}
	var job movieRenderJob
	err := s.db.QueryRowContext(r.Context(), `
		INSERT INTO movie_render_jobs (movie_edit_id, workspace_id, status, current_stage, total_scene_count, quality_preset, idempotency_key)
		VALUES ($1,$2,'queued','queued',$3,$4,$5)
		RETURNING id, movie_edit_id, status, current_stage, completed_scene_count, total_scene_count, COALESCE(output_asset_id::text, ''),
			output_duration_seconds, output_size_bytes, output_width, output_height, COALESCE(output_video_codec, ''), COALESCE(output_audio_codec, ''),
			quality_preset, COALESCE(failure_message, ''), started_at, completed_at, created_at, updated_at`,
		edit.ID, workspaceID, len(edit.Scenes), edit.QualityPreset, nullableString(idempotencyKey)).Scan(movieRenderDest(&job)...)
	return job, err
}

func (s *Server) runMovieRenderJob(w http.ResponseWriter, r *http.Request, workspaceID string, edit movieEdit, job movieRenderJob) movieRenderJob {
	stage := func(status, current string, completed int, msg string) {
		_, _ = s.db.ExecContext(r.Context(), `UPDATE movie_render_jobs SET status = $2, current_stage = $3, completed_scene_count = $4, failure_message = $5, started_at = COALESCE(started_at, NOW()), updated_at = NOW() WHERE id = $1`, job.ID, status, current, completed, nullableString(msg))
	}
	stage("preparing_assets", "Preparing assets", 0, "")
	input, cleanups, err := s.compileMovieRenderInput(r, workspaceID, edit)
	defer func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
	}()
	if err != nil {
		return s.failMovieRender(r, job.ID, workspaceID, "validation_error", err.Error())
	}
	stage("rendering_scenes", "Rendering scenes", 0, "")
	result := renderer.RenderMovie(r.Context(), renderer.Config{OutputDir: s.cfg.MediaOutputDir, FFmpegPath: s.cfg.FFmpegPath, FFprobePath: s.cfg.FFprobePath}, input)
	if result.Status != renderer.StatusCompleted {
		return s.failMovieRender(r, job.ID, workspaceID, "render_failed", safeMovieFailure(result.Notes))
	}
	stage("uploading", "Saving video", len(edit.Scenes), "")
	store, err := s.ensureMediaStore()
	if err != nil {
		return s.failMovieRender(r, job.ID, workspaceID, "storage_unavailable", "The completed video could not be saved.")
	}
	key, err := blobstore.JoinKey("movie-studio", workspaceID, edit.ID, job.ID+".mp4")
	if err != nil {
		return s.failMovieRender(r, job.ID, workspaceID, "storage_key", "The completed video could not be saved.")
	}
	info, err := store.PutFile(r.Context(), key, result.VideoPath, blobstore.PutOptions{ContentType: "video/mp4", OriginalFilename: edit.Name + ".mp4"})
	if err != nil {
		return s.failMovieRender(r, job.ID, workspaceID, "storage_failed", "The completed video could not be saved.")
	}
	width, height := result.VideoWidth, result.VideoHeight
	assetID, err := s.upsertMediaAsset(r.Context(), mediaAssetUpsert{WorkspaceID: workspaceID, ProjectID: edit.ContentProjectID, AssetType: "rendered_video", SourceWorkflow: "movie_studio", DisplayName: edit.Name, OriginalName: edit.Name + ".mp4", MimeType: "video/mp4", SizeBytes: &info.Size, DurationSeconds: result.VideoDurationSeconds, Width: &width, Height: &height, StorageProvider: info.Provider, StorageKey: info.Key, StorageETag: info.ETag, StorageSHA256: info.SHA256, Status: mediaAssetStatusReady})
	if err != nil {
		_ = store.Delete(r.Context(), key)
		return s.failMovieRender(r, job.ID, workspaceID, "asset_save_failed", "The completed video could not be saved.")
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		return s.failMovieRender(r, job.ID, workspaceID, "database_failed", "The completed video could not be saved.")
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(r.Context(), `
		UPDATE movie_render_jobs SET status='completed', current_stage='Completed', completed_scene_count=$2, output_asset_id=$3,
			output_duration_seconds=$4, output_size_bytes=$5, output_width=$6, output_height=$7, output_video_codec=$8, output_audio_codec=$9,
			completed_at=NOW(), updated_at=NOW()
		WHERE id=$1`, job.ID, len(edit.Scenes), assetID, result.VideoDurationSeconds, info.Size, result.VideoWidth, result.VideoHeight, result.VideoCodec, result.AudioCodec); err != nil {
		return s.failMovieRender(r, job.ID, workspaceID, "database_failed", "The completed video could not be saved.")
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE movie_edits SET status='rendered', latest_successful_render_id=$2, duration_seconds=$3, updated_at=NOW() WHERE id=$1`, edit.ID, job.ID, result.VideoDurationSeconds); err != nil {
		return s.failMovieRender(r, job.ID, workspaceID, "database_failed", "The completed video could not be saved.")
	}
	if edit.ContentProjectID != "" {
		_, _ = tx.ExecContext(r.Context(), `UPDATE content_projects SET active_movie_edit_id=$2, active_rendered_video_asset_id=$3, status='rendered', current_stage='video', updated_at=NOW() WHERE id=$1`, edit.ContentProjectID, edit.ID, assetID)
	}
	_, _ = tx.ExecContext(r.Context(), `INSERT INTO movie_render_usage (workspace_id, movie_edit_id, render_job_id, status, output_duration_seconds, output_width, output_height, quality_preset, scene_count, captions_enabled, music_enabled, storage_bytes_created) VALUES ($1,$2,$3,'completed',$4,$5,$6,$7,$8,$9,$10,$11)`, workspaceID, edit.ID, job.ID, result.VideoDurationSeconds, result.VideoWidth, result.VideoHeight, edit.QualityPreset, len(edit.Scenes), captionEnabled(edit.CaptionSettings), edit.MusicAssetID != "", info.Size)
	if err := tx.Commit(); err != nil {
		return s.failMovieRender(r, job.ID, workspaceID, "database_failed", "The completed video could not be saved.")
	}
	job, _ = s.getMovieRenderJob(r, workspaceID, job.ID)
	return job
}

func (s *Server) compileMovieRenderInput(r *http.Request, workspaceID string, edit movieEdit) (renderer.MovieRenderInput, []func(), error) {
	tempDir, err := os.MkdirTemp("", "trendcortex-movie-*")
	if err != nil {
		return renderer.MovieRenderInput{}, nil, err
	}
	cleanups := []func(){func() { _ = os.RemoveAll(tempDir) }}
	store, err := s.ensureMediaStore()
	if err != nil {
		return renderer.MovieRenderInput{}, cleanups, err
	}
	input := renderer.MovieRenderInput{WorkspaceID: workspaceID, MovieEditID: edit.ID, Title: edit.Name, Width: edit.OutputWidth, Height: edit.OutputHeight, FrameRate: edit.FrameRate, QualityPreset: edit.QualityPreset, CaptionsEnabled: captionEnabled(edit.CaptionSettings), NarrationVolume: 1, MusicVolume: 0.18, MusicLoop: true, BrandingText: stringSetting(edit.BrandingSettings, "text"), BrandingPosition: stringSetting(edit.BrandingSettings, "position")}
	if edit.VoiceoverAssetID != "" {
		path, cleanup, _, err := s.materializeAsset(r, store, workspaceID, edit.VoiceoverAssetID, tempDir)
		if err != nil {
			return input, cleanups, errors.New("The selected narration is no longer available. Choose another voiceover.")
		}
		cleanups = append(cleanups, cleanup)
		input.VoiceoverPath = path
	}
	if edit.MusicAssetID != "" {
		path, cleanup, _, err := s.materializeAsset(r, store, workspaceID, edit.MusicAssetID, tempDir)
		if err != nil {
			return input, cleanups, errors.New("The selected music is no longer available. Choose another music asset.")
		}
		cleanups = append(cleanups, cleanup)
		input.MusicPath = path
	}
	for i, sc := range edit.Scenes {
		if sc.VisualAssetID == "" {
			return input, cleanups, fmt.Errorf("Scene %d needs a visual before rendering.", i+1)
		}
		path, cleanup, asset, err := s.materializeAsset(r, store, workspaceID, sc.VisualAssetID, tempDir)
		if err != nil {
			return input, cleanups, fmt.Errorf("Scene %d visual is no longer available. Choose another visual.", i+1)
		}
		cleanups = append(cleanups, cleanup)
		input.Scenes = append(input.Scenes, renderer.MovieSceneInput{ID: sc.ID, Title: sc.Title, ScriptText: sc.ScriptText, VisualPath: path, VisualMimeType: asset.MimeType.String, Duration: sc.DurationSeconds, TrimIn: sc.TrimInSeconds, TrimOut: sc.TrimOutSeconds, FitMode: sc.FitMode, MotionPreset: sc.MotionPreset, CaptionText: sc.CaptionText, TransitionType: sc.TransitionType})
	}
	return input, cleanups, nil
}

func (s *Server) materializeAsset(r *http.Request, store blobstore.Store, workspaceID, assetID, dir string) (string, func(), mediaAssetRecord, error) {
	asset, err := s.getMediaAsset(r.Context(), workspaceID, assetID, false)
	if err != nil {
		return "", func() {}, mediaAssetRecord{}, err
	}
	if asset.Status != mediaAssetStatusReady || store.Provider() != asset.StorageProvider {
		return "", func() {}, asset, errors.New("asset unavailable")
	}
	name := asset.ID + filepath.Ext(firstNonEmpty(asset.OriginalName.String, asset.DisplayName, "asset"))
	path, cleanup, _, err := store.Materialize(r.Context(), asset.StorageKey, dir, name)
	return path, cleanup, asset, err
}

func (s *Server) failMovieRender(r *http.Request, jobID, workspaceID, code, message string) movieRenderJob {
	_, _ = s.db.ExecContext(r.Context(), `UPDATE movie_render_jobs SET status='failed', current_stage='Failed', error_code=$2, failure_message=$3, completed_at=NOW(), updated_at=NOW() WHERE id=$1`, jobID, code, safeMovieFailure(message))
	job, _ := s.getMovieRenderJob(r, workspaceID, jobID)
	return job
}

func (s *Server) getMovieRenderJob(r *http.Request, workspaceID, renderID string) (movieRenderJob, error) {
	var job movieRenderJob
	err := s.db.QueryRowContext(r.Context(), `
		SELECT id, movie_edit_id, status, current_stage, completed_scene_count, total_scene_count, COALESCE(output_asset_id::text, ''),
			output_duration_seconds, output_size_bytes, output_width, output_height, COALESCE(output_video_codec, ''), COALESCE(output_audio_codec, ''),
			quality_preset, COALESCE(failure_message, ''), started_at, completed_at, created_at, updated_at
		FROM movie_render_jobs WHERE workspace_id = $1 AND id = $2`, workspaceID, renderID).Scan(movieRenderDest(&job)...)
	return job, err
}

func (s *Server) getMovieRenderByIdempotency(r *http.Request, workspaceID, editID, key string) (movieRenderJob, error) {
	var job movieRenderJob
	err := s.db.QueryRowContext(r.Context(), `
		SELECT id, movie_edit_id, status, current_stage, completed_scene_count, total_scene_count, COALESCE(output_asset_id::text, ''),
			output_duration_seconds, output_size_bytes, output_width, output_height, COALESCE(output_video_codec, ''), COALESCE(output_audio_codec, ''),
			quality_preset, COALESCE(failure_message, ''), started_at, completed_at, created_at, updated_at
		FROM movie_render_jobs WHERE workspace_id = $1 AND movie_edit_id = $2 AND idempotency_key = $3`, workspaceID, editID, key).Scan(movieRenderDest(&job)...)
	return job, err
}

func (s *Server) listMovieRenders(r *http.Request, workspaceID, editID string) ([]movieRenderJob, error) {
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id, movie_edit_id, status, current_stage, completed_scene_count, total_scene_count, COALESCE(output_asset_id::text, ''),
			output_duration_seconds, output_size_bytes, output_width, output_height, COALESCE(output_video_codec, ''), COALESCE(output_audio_codec, ''),
			quality_preset, COALESCE(failure_message, ''), started_at, completed_at, created_at, updated_at
		FROM movie_render_jobs WHERE workspace_id = $1 AND movie_edit_id = $2 ORDER BY created_at DESC LIMIT 20`, workspaceID, editID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []movieRenderJob{}
	for rows.Next() {
		var job movieRenderJob
		if err := rows.Scan(movieRenderDest(&job)...); err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, rows.Err()
}

func movieRenderDest(job *movieRenderJob) []any {
	var duration sql.NullFloat64
	var size sql.NullInt64
	var width, height sql.NullInt64
	var started, completed sql.NullTime
	return []any{&job.ID, &job.MovieEditID, &job.Status, &job.CurrentStage, &job.CompletedSceneCount, &job.TotalSceneCount, &job.OutputAssetID, scanFloatPtr(&duration, &job.OutputDurationSeconds), scanInt64Ptr(&size, &job.OutputSizeBytes), scanIntPtr(&width, &job.OutputWidth), scanIntPtr(&height, &job.OutputHeight), &job.OutputVideoCodec, &job.OutputAudioCodec, &job.QualityPreset, &job.FailureMessage, scanTimePtr(&started, &job.StartedAt), scanTimePtr(&completed, &job.CompletedAt), &job.CreatedAt, &job.UpdatedAt}
}

type scanFloatTarget struct {
	ns  *sql.NullFloat64
	out **float64
}

func scanFloatPtr(ns *sql.NullFloat64, out **float64) *scanFloatTarget {
	return &scanFloatTarget{ns: ns, out: out}
}
func (sft *scanFloatTarget) Scan(v any) error {
	if err := sft.ns.Scan(v); err != nil {
		return err
	}
	if sft.ns.Valid {
		*sft.out = &sft.ns.Float64
	}
	return nil
}

type scanInt64Target struct {
	ns  *sql.NullInt64
	out **int64
}

func scanInt64Ptr(ns *sql.NullInt64, out **int64) *scanInt64Target {
	return &scanInt64Target{ns: ns, out: out}
}
func (sit *scanInt64Target) Scan(v any) error {
	if err := sit.ns.Scan(v); err != nil {
		return err
	}
	if sit.ns.Valid {
		*sit.out = &sit.ns.Int64
	}
	return nil
}

type scanIntTarget struct {
	ns  *sql.NullInt64
	out **int
}

func scanIntPtr(ns *sql.NullInt64, out **int) *scanIntTarget { return &scanIntTarget{ns: ns, out: out} }
func (sit *scanIntTarget) Scan(v any) error {
	if err := sit.ns.Scan(v); err != nil {
		return err
	}
	if sit.ns.Valid {
		vv := int(sit.ns.Int64)
		*sit.out = &vv
	}
	return nil
}

type scanTimeTarget struct {
	ns  *sql.NullTime
	out **time.Time
}

func scanTimePtr(ns *sql.NullTime, out **time.Time) *scanTimeTarget {
	return &scanTimeTarget{ns: ns, out: out}
}
func (stt *scanTimeTarget) Scan(v any) error {
	if err := stt.ns.Scan(v); err != nil {
		return err
	}
	if stt.ns.Valid {
		*stt.out = &stt.ns.Time
	}
	return nil
}

func jsonUnmarshalMap(raw []byte, target *map[string]any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, target)
}

func defaultMovieMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func normalizeMovieQuality(value string) string {
	switch strings.TrimSpace(value) {
	case "draft", "high":
		return value
	default:
		return "standard"
	}
}

func normalizeFitMode(value string) string {
	switch value {
	case "fit_background", "original":
		return value
	default:
		return "fill_crop"
	}
}

func normalizeMotion(value string) string {
	switch value {
	case "slow_zoom_in", "slow_zoom_out", "pan_left", "pan_right", "pan_up", "pan_down":
		return value
	default:
		return "none"
	}
}

func normalizeTransition(value string) string {
	switch value {
	case "crossfade", "fade_black", "slide":
		return value
	default:
		return "cut"
	}
}

func estimateSceneDuration(text string) float64 {
	words := len(strings.Fields(text))
	if words == 0 {
		return 4
	}
	v := float64(words) / 2.7
	return clampFloat(v, 3, 12)
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func defaultFloat(v, fallback float64) float64 {
	if v == 0 {
		return fallback
	}
	return v
}

func captionEnabled(settings map[string]any) bool {
	if v, ok := settings["enabled"].(bool); ok {
		return v
	}
	return true
}

func stringSetting(settings map[string]any, key string) string {
	if v, ok := settings[key].(string); ok {
		return limitText(v, 120)
	}
	return ""
}

func movieUploadContentTypeAllowed(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	return ct == "video/mp4" || ct == "video/quicktime" || ct == "video/webm" || ct == "image/png" || ct == "image/jpeg" || ct == "audio/mpeg" || ct == "audio/wav" || ct == "audio/x-wav" || ct == "audio/mp4"
}

func safeMovieFailure(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "The render failed. Your edit has been saved and can be retried."
	}
	if strings.Contains(strings.ToLower(message), "ffmpeg") {
		return "The render failed during video processing. Your edit has been saved and can be retried."
	}
	return limitText(message, 500)
}
