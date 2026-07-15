package http

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"trendcortex/api/internal/blobstore"
	"trendcortex/api/internal/renderer"
	"trendcortex/api/internal/storage"
)

type clipStudioRenderRequest struct {
	SourceModel     string                           `json:"source_model"`
	SourceID        string                           `json:"source_id,omitempty"`
	SourceVideoPath string                           `json:"source_video_path"`
	Rights          renderer.ClipRightsMetadata      `json:"rights"`
	Branding        renderer.ClipBrandingSettings    `json:"branding"`
	ManualRange     renderer.ClipManualRange         `json:"manual_range"`
	Captions        string                           `json:"captions"`
	CaptionText     string                           `json:"caption_text"`
	IncludeCaptions bool                             `json:"include_captions"`
	LayoutMode      string                           `json:"layout_mode"`
	AIHighlights    renderer.ClipAIHighlightMetadata `json:"ai_highlights"`
}

type clipStudioRenderResponse struct {
	Success       bool     `json:"success"`
	RenderStatus  string   `json:"render_status"`
	Notes         string   `json:"notes,omitempty"`
	ClipID        string   `json:"clip_id"`
	ZipFilename   string   `json:"zip_filename,omitempty"`
	DownloadURL   string   `json:"download_url,omitempty"`
	IncludedFiles []string `json:"included_files"`
	VideoPath     string   `json:"video_path,omitempty"`
	ThumbnailPath string   `json:"thumbnail_path,omitempty"`
}

type clipStudioSourceMetadata struct {
	SourceID        string                      `json:"source_id"`
	Kind            string                      `json:"kind"`
	OriginalName    string                      `json:"original_name,omitempty"`
	URL             string                      `json:"url,omitempty"`
	FilePath        string                      `json:"file_path,omitempty"`
	StorageProvider string                      `json:"-"`
	StorageKey      string                      `json:"-"`
	StorageETag     string                      `json:"-"`
	StorageSHA256   string                      `json:"-"`
	ContentType     string                      `json:"content_type,omitempty"`
	SizeBytes       int64                       `json:"size_bytes,omitempty"`
	SourceModel     string                      `json:"source_model,omitempty"`
	Rights          renderer.ClipRightsMetadata `json:"rights"`
	Status          string                      `json:"status"`
	Message         string                      `json:"message,omitempty"`
	CreatedAt       time.Time                   `json:"created_at"`
	DirectVideo     bool                        `json:"direct_video"`
	SupportedType   bool                        `json:"supported_type"`
}

type clipStudioSourceRequest struct {
	ProjectID       string                      `json:"project_id,omitempty"`
	SceneID         string                      `json:"scene_id,omitempty"`
	OutputScope     string                      `json:"output_scope,omitempty"`
	SourceURL       string                      `json:"source_url"`
	RightsConfirmed bool                        `json:"rights_confirmed"`
	Rights          renderer.ClipRightsMetadata `json:"rights"`
	SourceTitle     string                      `json:"source_title,omitempty"`
	SourceCreator   string                      `json:"source_creator,omitempty"`
	SourceLicense   string                      `json:"source_license,omitempty"`
	PlatformSource  string                      `json:"platform_source,omitempty"`
}

type clipStudioSourceResponse struct {
	SourceID      string                   `json:"source_id"`
	Status        string                   `json:"status"`
	Message       string                   `json:"message,omitempty"`
	Metadata      clipStudioSourceMetadata `json:"metadata"`
	ProjectOutput *contentProjectOutput    `json:"project_output,omitempty"`
	CanRender     bool                     `json:"can_render"`
	DirectVideo   bool                     `json:"direct_video"`
	DownloadReady bool                     `json:"download_ready"`
}

type clipStudioGenerateRequest struct {
	ProjectID       string                        `json:"project_id,omitempty"`
	SceneID         string                        `json:"scene_id,omitempty"`
	OutputScope     string                        `json:"output_scope,omitempty"`
	IdempotencyKey  string                        `json:"idempotency_key,omitempty"`
	SourceID        string                        `json:"source_id,omitempty"`
	SourceURL       string                        `json:"source_url,omitempty"`
	Prompt          string                        `json:"prompt"`
	ClipLength      string                        `json:"clip_length"`
	ClipCount       int                           `json:"clip_count"`
	Branding        renderer.ClipBrandingSettings `json:"branding"`
	CaptionText     string                        `json:"caption_text,omitempty"`
	LayoutMode      string                        `json:"layout_mode,omitempty"`
	Rights          renderer.ClipRightsMetadata   `json:"rights"`
	RightsConfirmed bool                          `json:"rights_confirmed"`
	Advanced        clipStudioGenerateAdvanced    `json:"advanced"`
}

type clipStudioGenerateAdvanced struct {
	SourceModel          string `json:"source_model,omitempty"`
	SourceTitle          string `json:"source_title,omitempty"`
	SourceCreator        string `json:"source_creator,omitempty"`
	SourceLicense        string `json:"source_license,omitempty"`
	AttributionText      string `json:"attribution_text,omitempty"`
	CopyrightOverlayText string `json:"copyright_overlay_text,omitempty"`
	PlatformSource       string `json:"platform_source,omitempty"`
}

type clipStudioGeneratedJob struct {
	ClipID        string                   `json:"clip_id"`
	RenderStatus  string                   `json:"render_status"`
	Notes         string                   `json:"notes,omitempty"`
	ManualRange   renderer.ClipManualRange `json:"manual_range"`
	VideoPath     string                   `json:"video_path,omitempty"`
	ThumbnailPath string                   `json:"thumbnail_path,omitempty"`
}

type clipStudioGenerateResponse struct {
	Success            bool                     `json:"success"`
	RenderStatus       string                   `json:"render_status"`
	Notes              string                   `json:"notes,omitempty"`
	SourceID           string                   `json:"source_id,omitempty"`
	ProjectOutput      *contentProjectOutput    `json:"project_output,omitempty"`
	HighlightDetection string                   `json:"highlight_detection"`
	GeneratedClipJobs  []clipStudioGeneratedJob `json:"generated_clip_jobs"`
	ZipFilename        string                   `json:"zip_filename,omitempty"`
	DownloadURL        string                   `json:"download_url,omitempty"`
	IncludedFiles      []string                 `json:"included_files"`
}

type aiSceneWorkerStatusResponse struct {
	Configured            bool                 `json:"configured"`
	Status                string               `json:"status"`
	Message               string               `json:"message"`
	DashboardURL          string               `json:"dashboard_url,omitempty"`
	GeneratorMode         string               `json:"generator_mode,omitempty"`
	AutoCommandConfigured bool                 `json:"auto_command_configured,omitempty"`
	ModelHint             string               `json:"model_hint,omitempty"`
	OutputDir             string               `json:"output_dir,omitempty"`
	Jobs                  []renderer.WorkerJob `json:"jobs,omitempty"`
}

type aiScenePlanRequest struct {
	Topic               string `json:"topic"`
	Prompt              string `json:"prompt"`
	StylePreset         string `json:"style_preset"`
	TargetLengthSeconds int    `json:"target_length_seconds"`
}

type aiScenePlanResponse struct {
	RendererVersion string               `json:"renderer_version"`
	Scenes          []renderer.ScenePlan `json:"scenes"`
	ModelHint       string               `json:"model_hint"`
}

type aiSceneGenerateRequest struct {
	Topic               string                        `json:"topic"`
	Prompt              string                        `json:"prompt"`
	StylePreset         string                        `json:"style_preset"`
	TargetLengthSeconds int                           `json:"target_length_seconds"`
	Branding            renderer.ClipBrandingSettings `json:"branding"`
}

type aiSceneGenerateResponse struct {
	GenerationID     string                    `json:"generation_id,omitempty"`
	Success          bool                      `json:"success"`
	RenderStatus     string                    `json:"render_status"`
	Notes            string                    `json:"notes,omitempty"`
	RendererVersion  string                    `json:"renderer_version"`
	ClipID           string                    `json:"clip_id"`
	WorkerConfigured bool                      `json:"worker_url_configured"`
	ModelHint        string                    `json:"model_hint"`
	ScenePrompts     []renderer.ScenePlan      `json:"scene_prompts"`
	SceneJobIDs      []string                  `json:"scene_job_ids"`
	SceneJobs        []renderer.WorkerSceneJob `json:"scene_jobs"`
	GenerationStatus string                    `json:"generation_status"`
	FallbackReason   string                    `json:"fallback_reason,omitempty"`
	ManualOutputPath string                    `json:"manual_output_path,omitempty"`
	TimeoutSeconds   int                       `json:"timeout_seconds,omitempty"`
	NextAction       string                    `json:"next_action,omitempty"`
	ZipFilename      string                    `json:"zip_filename,omitempty"`
	DownloadURL      string                    `json:"download_url,omitempty"`
	IncludedFiles    []string                  `json:"included_files"`
	VideoPath        string                    `json:"video_path,omitempty"`
	ThumbnailPath    string                    `json:"thumbnail_path,omitempty"`
}

type aiSceneGenerationJob struct {
	GenerationID        string
	WorkspaceID         string
	ClipID              string
	Request             aiSceneGenerateRequest
	StartedAt           time.Time
	UpdatedAt           time.Time
	Status              string
	ProgressPercent     int
	CurrentStep         string
	EstimatedNextAction string
	Downloadable        bool
	Response            aiSceneGenerateResponse
	VideoDownloadURL    string
	VideoDownloadName   string
	Err                 string
}

type aiSceneGenerationStatusResponse struct {
	GenerationID        string                    `json:"generation_id"`
	ProgressPercent     int                       `json:"progress_percent"`
	CurrentStep         string                    `json:"current_step"`
	Status              string                    `json:"status"`
	EstimatedNextAction string                    `json:"estimated_next_action"`
	Downloadable        bool                      `json:"downloadable"`
	VideoURL            string                    `json:"video_url,omitempty"`
	VideoFilename       string                    `json:"video_filename,omitempty"`
	ZipURL              string                    `json:"zip_url,omitempty"`
	ZipFilename         string                    `json:"zip_filename,omitempty"`
	Notes               string                    `json:"notes,omitempty"`
	RendererVersion     string                    `json:"renderer_version"`
	ClipID              string                    `json:"clip_id"`
	WorkerConfigured    bool                      `json:"worker_url_configured"`
	ModelHint           string                    `json:"model_hint"`
	ScenePrompts        []renderer.ScenePlan      `json:"scene_prompts,omitempty"`
	SceneJobIDs         []string                  `json:"scene_job_ids,omitempty"`
	SceneJobs           []renderer.WorkerSceneJob `json:"scene_jobs,omitempty"`
	IncludedFiles       []string                  `json:"included_files,omitempty"`
}

var clipStudioHTTPClient = http.DefaultClient

const (
	clipStudioMaxSourceBytes   int64 = 512 << 20
	clipStudioURLImportTimeout       = 20 * time.Second
	clipStudioWatchURLMessage        = "This is a platform watch URL. Upload the source file or provide a direct downloadable video URL."
	clipStudioImportedMessage        = "Video imported and ready"
)

func (s *Server) handleRenderClipStudio(w http.ResponseWriter, r *http.Request) {
	var req clipStudioRenderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if strings.TrimSpace(req.SourceID) != "" {
		meta, err := s.readClipStudioSource(r.Context(), workspaceID, req.SourceID)
		if err != nil {
			jsonError(w, "clip studio source not found: "+err.Error(), http.StatusBadRequest)
			return
		}
		path, cleanup, err := s.materializeClipStudioSource(r.Context(), workspaceID, meta)
		if err != nil {
			jsonError(w, "source media unavailable", http.StatusGone)
			return
		}
		defer cleanup()
		req.SourceVideoPath = path
		if req.Rights.SourceURL == "" {
			req.Rights = meta.Rights
		}
		if req.SourceModel == "" {
			req.SourceModel = meta.SourceModel
		}
	} else if strings.TrimSpace(req.SourceVideoPath) != "" && s.cfg.AppEnv == "production" {
		jsonError(w, "source_id is required for rendering", http.StatusBadRequest)
		return
	}
	clipID := "clip-" + time.Now().UTC().Format("20060102-150405")
	input := renderer.ClipInput{
		WorkspaceID:     workspaceID,
		ClipID:          clipID,
		SourceModel:     req.SourceModel,
		SourceVideoPath: req.SourceVideoPath,
		Rights:          req.Rights,
		Branding:        req.Branding,
		ManualRange:     req.ManualRange,
		Captions:        req.Captions,
		CaptionText:     req.CaptionText,
		IncludeCaptions: req.IncludeCaptions,
		LayoutMode:      req.LayoutMode,
		AIHighlights:    req.AIHighlights,
	}
	if input.AIHighlights.TranscriptionStatus == "" {
		input.AIHighlights = renderer.DefaultClipAIHighlightMetadata()
	}

	result := renderer.RenderManualClip(r.Context(), renderer.Config{
		OutputDir:   s.cfg.MediaOutputDir,
		FFmpegPath:  s.cfg.FFmpegPath,
		FFprobePath: s.cfg.FFprobePath,
	}, input)
	if result.Status != renderer.StatusCompleted {
		jsonOK(w, clipStudioRenderResponse{
			Success:      false,
			RenderStatus: result.Status,
			Notes:        result.Notes,
			ClipID:       clipID,
		})
		return
	}

	exportDir := filepath.Join(s.cfg.ExportDir, workspaceID, "clip-studio")
	meta := storage.ClipStudioExportMetadata{
		ClipID:          clipID,
		SourceModel:     input.SourceModel,
		Rights:          input.Rights,
		Branding:        input.Branding,
		ManualRange:     input.ManualRange,
		CaptionText:     input.CaptionText,
		LayoutMode:      input.LayoutMode,
		AIHighlights:    input.AIHighlights,
		RendererVersion: result.RendererVersion,
	}
	zipPath, included, err := storage.BuildClipStudioExportZip(exportDir, storage.ClipStudioExportContent{
		ClipID:           clipID,
		VideoSrcPath:     result.VideoPath,
		ThumbnailSrcPath: result.ThumbnailPath,
		Metadata:         meta,
	})
	if err != nil {
		jsonError(w, "clip studio export failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	objectKey, err := clipStudioOutputObjectKey(workspaceID, "manual-renders", clipID, filepath.Base(zipPath))
	if err != nil {
		jsonError(w, "clip studio export key failed", http.StatusInternalServerError)
		return
	}
	if _, err := s.storeMediaFile(r.Context(), workspaceID, objectKey, zipPath, "application/zip", filepath.Base(zipPath)); err != nil {
		jsonError(w, "clip studio export storage failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonOK(w, clipStudioRenderResponse{
		Success:       true,
		RenderStatus:  result.Status,
		Notes:         result.Notes,
		ClipID:        clipID,
		ZipFilename:   filepath.Base(zipPath),
		DownloadURL:   "/api/clip-studio/download/" + filepath.Base(zipPath),
		IncludedFiles: included,
	})
}

func (s *Server) handleUploadClipStudioSource(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		jsonError(w, "invalid multipart upload: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("video")
	if err != nil {
		jsonError(w, "video upload is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !isSupportedClipVideoExt(ext) {
		jsonError(w, "unsupported upload type; use mp4, mov, or webm", http.StatusBadRequest)
		return
	}
	sourceID := newClipStudioSourceID()
	dstDir := s.clipStudioSourceDir(workspaceID, sourceID)
	if err := os.MkdirAll(dstDir, 0750); err != nil {
		jsonError(w, "create source storage failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	dstPath := filepath.Join(dstDir, "source"+ext)
	size, err := copyUploadedClipSource(file, dstPath)
	if err != nil {
		jsonError(w, "store upload failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	objectKey, err := clipStudioSourceObjectKey(workspaceID, sourceID, ext)
	if err != nil {
		jsonError(w, "create source object key failed", http.StatusInternalServerError)
		return
	}
	contentType := normalizeMediaType(header.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "video/mp4"
	}
	objectInfo, err := s.storeMediaFile(r.Context(), workspaceID, objectKey, dstPath, contentType, header.Filename)
	if err != nil {
		jsonError(w, "store upload in media storage failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	meta := clipStudioSourceMetadata{
		SourceID:        sourceID,
		Kind:            "upload",
		OriginalName:    header.Filename,
		FilePath:        dstPath,
		StorageProvider: objectInfo.Provider,
		StorageKey:      objectInfo.Key,
		StorageETag:     objectInfo.ETag,
		StorageSHA256:   objectInfo.SHA256,
		ContentType:     contentType,
		SizeBytes:       size,
		SourceModel:     renderer.ClipSourceUserUpload,
		Rights:          renderer.ClipRightsMetadata{UserConfirmedRights: false, PlatformSource: "upload"},
		Status:          "ready",
		Message:         "Upload ready for clip generation.",
		CreatedAt:       time.Now().UTC(),
		DirectVideo:     true,
		SupportedType:   true,
	}
	if err := s.writeClipStudioSource(workspaceID, meta); err != nil {
		jsonError(w, "store source metadata failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var projectOutput *contentProjectOutput
	projectID := strings.TrimSpace(r.FormValue("project_id"))
	if projectID != "" && s.db != nil {
		sceneID := optionalSceneID(r.FormValue("scene_id"))
		scope := outputScopeFromRequest(r.FormValue("output_scope"), sceneID)
		output, err := s.upsertContentProjectOutput(r.Context(), workspaceID, projectID, sceneID, contentProjectOutputMutation{
			OutputScope:      scope,
			OutputType:       projectOutputTypeUploaded,
			SourceWorkflow:   projectOutputWorkflowClipGenerator,
			RenderJobID:      meta.SourceID,
			Status:           projectOutputStatusCompleted,
			OriginalFilename: header.Filename,
			DisplayName:      safeOutputDisplayName(header.Filename, "Uploaded clip"),
			MimeType:         firstNonEmpty(meta.ContentType, "video/mp4"),
			FileSizeBytes:    &size,
			StorageProvider:  meta.StorageProvider,
			StorageKey:       meta.StorageKey,
			StorageETag:      meta.StorageETag,
			StorageSHA256:    meta.StorageSHA256,
			StorageReference: dstPath,
			Retryable:        false,
		})
		if err != nil {
			jsonErrorCode(w, "validation_error", err.Error(), http.StatusBadRequest)
			return
		}
		projectOutput = &output.contentProjectOutput
	}
	jsonOK(w, clipStudioSourceResponse{
		SourceID:      sourceID,
		Status:        meta.Status,
		Message:       meta.Message,
		Metadata:      publicClipStudioSourceMetadata(meta),
		ProjectOutput: projectOutput,
		CanRender:     true,
		DirectVideo:   true,
		DownloadReady: true,
	})
}

func (s *Server) handleCreateClipStudioSource(w http.ResponseWriter, r *http.Request) {
	var req clipStudioSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	sourceURL := strings.TrimSpace(req.SourceURL)
	if sourceURL == "" {
		jsonError(w, "source_url is required", http.StatusBadRequest)
		return
	}
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		jsonError(w, "source_url must be a valid URL", http.StatusBadRequest)
		return
	}
	rights := req.Rights
	rights.SourceURL = sourceURL
	rights.UserConfirmedRights = req.RightsConfirmed || rights.UserConfirmedRights
	if rights.SourceTitle == "" {
		rights.SourceTitle = req.SourceTitle
	}
	if rights.SourceCreator == "" {
		rights.SourceCreator = req.SourceCreator
	}
	if rights.SourceLicense == "" {
		rights.SourceLicense = req.SourceLicense
	}
	if rights.PlatformSource == "" {
		rights.PlatformSource = firstNonEmpty(req.PlatformSource, parsed.Host)
	}

	sourceID := newClipStudioSourceID()
	ext := strings.ToLower(filepath.Ext(parsed.Path))
	watchURL := isPlatformWatchURL(parsed)
	direct := !watchURL && isSupportedClipVideoExt(ext)
	message := clipStudioReferenceOnlyMessage(parsed.Host)
	meta := clipStudioSourceMetadata{
		SourceID:      sourceID,
		Kind:          "url",
		URL:           sourceURL,
		SourceModel:   renderer.ClipSourceExternalURLPendingRightsConfirmation,
		Rights:        rights,
		Status:        "metadata_only",
		Message:       message,
		CreatedAt:     time.Now().UTC(),
		DirectVideo:   direct,
		SupportedType: direct,
	}
	if direct {
		if !rights.UserConfirmedRights {
			meta.Status = "rights_required"
			meta.Message = "Confirm rights before downloading this direct video URL."
			if err := s.writeClipStudioSource(workspaceID, meta); err != nil {
				jsonError(w, "store source metadata failed: "+err.Error(), http.StatusInternalServerError)
				return
			}
			jsonOK(w, clipStudioSourceResponse{SourceID: sourceID, Status: meta.Status, Message: meta.Message, Metadata: publicClipStudioSourceMetadata(meta), DirectVideo: true})
			return
		}
		dstDir := s.clipStudioSourceDir(workspaceID, sourceID)
		if err := os.MkdirAll(dstDir, 0750); err != nil {
			jsonError(w, "create source storage failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		dstPath := filepath.Join(dstDir, "source"+ext)
		size, contentType, err := downloadDirectClipSource(r.Context(), sourceURL, dstPath)
		if err != nil {
			jsonError(w, "download direct video failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		objectKey, err := clipStudioSourceObjectKey(workspaceID, sourceID, ext)
		if err != nil {
			jsonError(w, "create source object key failed", http.StatusInternalServerError)
			return
		}
		objectInfo, err := s.storeMediaFile(r.Context(), workspaceID, objectKey, dstPath, contentType, filepath.Base(parsed.Path))
		if err != nil {
			jsonError(w, "store source in media storage failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		meta.FilePath = dstPath
		meta.StorageProvider = objectInfo.Provider
		meta.StorageKey = objectInfo.Key
		meta.StorageETag = objectInfo.ETag
		meta.StorageSHA256 = objectInfo.SHA256
		meta.SizeBytes = size
		meta.ContentType = contentType
		meta.Status = "ready"
		meta.Message = "Direct video URL downloaded and ready for clip generation."
	}
	if err := s.writeClipStudioSource(workspaceID, meta); err != nil {
		jsonError(w, "store source metadata failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, clipStudioSourceResponse{
		SourceID:      sourceID,
		Status:        meta.Status,
		Message:       meta.Message,
		Metadata:      publicClipStudioSourceMetadata(meta),
		CanRender:     meta.Status == "ready",
		DirectVideo:   meta.DirectVideo,
		DownloadReady: meta.FilePath != "",
	})
}

func (s *Server) handleImportClipStudioURL(w http.ResponseWriter, r *http.Request) {
	var req clipStudioSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	source, err := s.createClipStudioSourceFromURL(r.Context(), workspaceID, req)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	var projectOutput *contentProjectOutput
	if strings.TrimSpace(req.ProjectID) != "" && s.db != nil {
		sceneID := optionalSceneID(req.SceneID)
		scope := outputScopeFromRequest(req.OutputScope, sceneID)
		status := projectOutputStatusCompleted
		outputType := projectOutputTypeImported
		failureCategory := ""
		failureMessage := ""
		storageReference := source.FilePath
		mimeType := firstNonEmpty(source.ContentType, "video/mp4")
		var size *int64
		if source.SizeBytes >= 0 {
			size = &source.SizeBytes
		}
		retryable := false
		if source.FilePath == "" || source.Status != "ready" {
			status = projectOutputStatusUnavailable
			failureCategory = "source_unavailable"
			failureMessage = source.Message
			storageReference = ""
			mimeType = ""
			size = nil
		}
		output, err := s.upsertContentProjectOutput(r.Context(), workspaceID, req.ProjectID, sceneID, contentProjectOutputMutation{
			OutputScope:      scope,
			OutputType:       outputType,
			SourceWorkflow:   projectOutputWorkflowClipGenerator,
			RenderJobID:      source.SourceID,
			Status:           status,
			OriginalFilename: filepath.Base(source.URL),
			DisplayName:      safeOutputDisplayName(filepath.Base(source.URL), "Imported clip"),
			MimeType:         mimeType,
			FileSizeBytes:    size,
			StorageReference: storageReference,
			StorageProvider:  source.StorageProvider,
			StorageKey:       source.StorageKey,
			StorageETag:      source.StorageETag,
			StorageSHA256:    source.StorageSHA256,
			FailureCategory:  failureCategory,
			FailureMessage:   failureMessage,
			Retryable:        retryable,
		})
		if err != nil {
			jsonErrorCode(w, "validation_error", err.Error(), http.StatusBadRequest)
			return
		}
		projectOutput = &output.contentProjectOutput
	}
	jsonOK(w, clipStudioSourceResponse{
		SourceID:      source.SourceID,
		Status:        source.Status,
		Message:       source.Message,
		Metadata:      publicClipStudioSourceMetadata(source),
		ProjectOutput: projectOutput,
		CanRender:     source.Status == "ready" && source.FilePath != "",
		DirectVideo:   source.DirectVideo,
		DownloadReady: source.FilePath != "",
	})
}

func (s *Server) handleGenerateClipStudio(w http.ResponseWriter, r *http.Request) {
	var req clipStudioGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if !req.RightsConfirmed && !req.Rights.UserConfirmedRights {
		jsonError(w, "I confirm I have rights or permission to use this source.", http.StatusForbidden)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	response, err := s.performClipStudioGeneration(r.Context(), workspaceID, strings.TrimSpace(req.ProjectID), nil, req)
	if err != nil {
		jsonErrorCode(w, "validation_error", err.Error(), http.StatusBadRequest)
		return
	}
	jsonOK(w, response)
}

func (s *Server) performClipStudioGeneration(ctx context.Context, workspaceID, projectID string, existingOutputID *string, req clipStudioGenerateRequest) (clipStudioGenerateResponse, error) {
	var source clipStudioSourceMetadata
	if strings.TrimSpace(req.SourceID) != "" {
		var err error
		source, err = s.readClipStudioSource(ctx, workspaceID, req.SourceID)
		if err != nil {
			return clipStudioGenerateResponse{}, fmt.Errorf("clip studio source not found: %w", err)
		}
	} else if strings.TrimSpace(req.SourceURL) != "" {
		sourceReq := clipStudioSourceRequest{SourceURL: req.SourceURL, RightsConfirmed: req.RightsConfirmed, Rights: req.Rights}
		var err error
		source, err = s.createClipStudioSourceFromURL(ctx, workspaceID, sourceReq)
		if err != nil {
			return clipStudioGenerateResponse{}, err
		}
	} else {
		return clipStudioGenerateResponse{}, errors.New("source_id or source_url is required")
	}
	batchID := generationBatchID(req.IdempotencyKey)
	var projectOutput *contentProjectOutput
	var outputID string
	if projectID != "" && s.db != nil {
		sceneID := optionalSceneID(req.SceneID)
		scope := outputScopeFromRequest(req.OutputScope, sceneID)
		payload, _ := json.Marshal(req)
		output, err := s.upsertContentProjectOutput(ctx, workspaceID, projectID, sceneID, contentProjectOutputMutation{
			OutputScope:        scope,
			OutputType:         projectOutputTypeGenerated,
			SourceWorkflow:     projectOutputWorkflowClipGenerator,
			RenderJobID:        batchID,
			Status:             projectOutputStatusProcessing,
			DisplayName:        "Generated clip package",
			MimeType:           "application/zip",
			Retryable:          false,
			RequestPayloadJSON: payload,
		})
		if err != nil {
			return clipStudioGenerateResponse{}, err
		}
		outputID = output.ID
		projectOutput = &output.contentProjectOutput
		if existingOutputID != nil {
			outputID = *existingOutputID
		}
	}
	if (source.FilePath == "" && source.StorageKey == "") || source.Status != "ready" {
		response := clipStudioGenerateResponse{
			Success:            false,
			RenderStatus:       "unsupported_source",
			Notes:              "Upload the source file or connect an approved source before rendering.",
			SourceID:           source.SourceID,
			ProjectOutput:      projectOutput,
			HighlightDetection: "not_run",
		}
		if outputID != "" {
			updated, _ := s.updateContentProjectOutput(ctx, projectID, outputID, contentProjectOutputMutation{
				OutputScope:     outputScopeFromRequest(req.OutputScope, optionalSceneID(req.SceneID)),
				OutputType:      projectOutputTypeGenerated,
				Status:          projectOutputStatusFailed,
				DisplayName:     "Generated clip package",
				MimeType:        "application/zip",
				FailureCategory: "unsupported_source",
				FailureMessage:  response.Notes,
				Retryable:       false,
			})
			response.ProjectOutput = &updated.contentProjectOutput
		}
		return response, nil
	}
	sourcePath, cleanupSource, err := s.materializeClipStudioSource(ctx, workspaceID, source)
	if err != nil {
		return clipStudioGenerateResponse{}, fmt.Errorf("source media unavailable: %w", err)
	}
	defer cleanupSource()

	rights := source.Rights
	rights.UserConfirmedRights = true
	applyAdvancedRights(&rights, req.Advanced)
	clipLengthSeconds := clipLengthToSeconds(req.ClipLength)
	clipCount := normalizeClipCount(req.ClipCount)
	duration := renderer.ProbeDuration(ctx, s.cfg.FFprobePath, sourcePath)
	ranges := evenlySpacedClipRanges(duration, clipLengthSeconds, clipCount)
	jobs := make([]clipStudioGeneratedJob, 0, len(ranges))
	generated := make([]storage.ClipStudioGeneratedClip, 0, len(ranges))
	overallStatus := renderer.StatusCompleted
	for i, manualRange := range ranges {
		clipID := fmt.Sprintf("%s-%02d", batchID, i+1)
		input := renderer.ClipInput{
			WorkspaceID:     workspaceID,
			ClipID:          clipID,
			SourceModel:     firstNonEmpty(req.Advanced.SourceModel, source.SourceModel, renderer.ClipSourceUserUpload),
			SourceVideoPath: sourcePath,
			Rights:          rights,
			Branding:        req.Branding,
			ManualRange:     manualRange,
			CaptionText:     req.CaptionText,
			IncludeCaptions: strings.TrimSpace(req.CaptionText) != "",
			LayoutMode:      req.LayoutMode,
			AIHighlights:    renderer.DefaultClipAIHighlightMetadata(),
		}
		result := renderer.RenderManualClip(ctx, renderer.Config{
			OutputDir:   s.cfg.MediaOutputDir,
			FFmpegPath:  s.cfg.FFmpegPath,
			FFprobePath: s.cfg.FFprobePath,
		}, input)
		jobs = append(jobs, clipStudioGeneratedJob{
			ClipID:       clipID,
			RenderStatus: result.Status,
			Notes:        result.Notes,
			ManualRange:  manualRange,
		})
		if result.Status != renderer.StatusCompleted {
			overallStatus = result.Status
			continue
		}
		generated = append(generated, storage.ClipStudioGeneratedClip{
			ClipID:           clipID,
			Folder:           fmt.Sprintf("clip-%02d", i+1),
			VideoSrcPath:     result.VideoPath,
			ThumbnailSrcPath: result.ThumbnailPath,
			Metadata: storage.ClipStudioExportMetadata{
				ClipID:          clipID,
				SourceModel:     input.SourceModel,
				Rights:          input.Rights,
				Branding:        input.Branding,
				ManualRange:     input.ManualRange,
				CaptionText:     input.CaptionText,
				LayoutMode:      input.LayoutMode,
				AIHighlights:    input.AIHighlights,
				RendererVersion: result.RendererVersion,
			},
		})
	}
	if len(generated) == 0 {
		response := clipStudioGenerateResponse{
			Success:            false,
			RenderStatus:       overallStatus,
			Notes:              "No clips rendered.",
			SourceID:           source.SourceID,
			ProjectOutput:      projectOutput,
			HighlightDetection: "not_run",
			GeneratedClipJobs:  jobs,
		}
		if outputID != "" {
			updated, _ := s.updateContentProjectOutput(ctx, projectID, outputID, contentProjectOutputMutation{
				OutputScope:     outputScopeFromRequest(req.OutputScope, optionalSceneID(req.SceneID)),
				OutputType:      projectOutputTypeGenerated,
				Status:          projectOutputStatusFailed,
				DisplayName:     "Generated clip package",
				MimeType:        "application/zip",
				FailureCategory: "render_failed",
				FailureMessage:  response.Notes,
				Retryable:       true,
			})
			response.ProjectOutput = &updated.contentProjectOutput
		}
		return response, nil
	}
	manifest := storage.ClipStudioGeneratedManifest{
		SourceID:           source.SourceID,
		Prompt:             req.Prompt,
		ClipLength:         firstNonEmpty(req.ClipLength, "auto"),
		ClipCount:          len(generated),
		HighlightDetection: "not_run",
		Clips:              generated,
	}
	exportDir := filepath.Join(s.cfg.ExportDir, workspaceID, "clip-studio")
	zipPath, included, err := storage.BuildClipStudioGeneratedExportZip(exportDir, batchID, manifest)
	if err != nil {
		if outputID != "" {
			_, _ = s.updateContentProjectOutput(ctx, projectID, outputID, contentProjectOutputMutation{
				OutputScope:     outputScopeFromRequest(req.OutputScope, optionalSceneID(req.SceneID)),
				OutputType:      projectOutputTypeGenerated,
				Status:          projectOutputStatusFailed,
				DisplayName:     "Generated clip package",
				MimeType:        "application/zip",
				FailureCategory: "packaging_failed",
				FailureMessage:  "Clip Studio export failed.",
				Retryable:       true,
			})
		}
		return clipStudioGenerateResponse{}, fmt.Errorf("clip studio export failed: %w", err)
	}
	var size *int64
	if stat, err := os.Stat(zipPath); err == nil {
		v := stat.Size()
		size = &v
	}
	response := clipStudioGenerateResponse{
		Success:            overallStatus == renderer.StatusCompleted,
		RenderStatus:       overallStatus,
		Notes:              "Generated clips with evenly spaced segments; highlight_detection: not_run.",
		SourceID:           source.SourceID,
		ProjectOutput:      projectOutput,
		HighlightDetection: "not_run",
		GeneratedClipJobs:  jobs,
		ZipFilename:        filepath.Base(zipPath),
		DownloadURL:        "/api/clip-studio/download/" + filepath.Base(zipPath),
		IncludedFiles:      included,
	}
	if outputID != "" {
		status := projectOutputStatusCompleted
		failureCategory := ""
		failureMessage := ""
		retryable := false
		if overallStatus != renderer.StatusCompleted {
			status = projectOutputStatusFailed
			failureCategory = "partial_render"
			failureMessage = "One or more clips failed to render."
			retryable = true
		}
		objectKey, keyErr := clipStudioOutputObjectKey(workspaceID, "outputs", outputID, filepath.Base(zipPath))
		var objectInfo blobstore.ObjectInfo
		if keyErr == nil {
			objectInfo, keyErr = s.storeMediaFile(ctx, workspaceID, objectKey, zipPath, "application/zip", filepath.Base(zipPath))
		}
		if keyErr != nil {
			updated, _ := s.updateContentProjectOutput(ctx, projectID, outputID, contentProjectOutputMutation{
				OutputScope:     outputScopeFromRequest(req.OutputScope, optionalSceneID(req.SceneID)),
				OutputType:      projectOutputTypeGenerated,
				Status:          projectOutputStatusFailed,
				DisplayName:     "Generated clip package",
				MimeType:        "application/zip",
				FailureCategory: "storage_upload_failed",
				FailureMessage:  "Generated media could not be stored durably.",
				Retryable:       true,
			})
			response.ProjectOutput = &updated.contentProjectOutput
			response.Success = false
			response.RenderStatus = renderer.StatusFailed
			response.Notes = "Generated media could not be stored durably."
			return response, nil
		}
		updated, _ := s.updateContentProjectOutput(ctx, projectID, outputID, contentProjectOutputMutation{
			OutputScope:      outputScopeFromRequest(req.OutputScope, optionalSceneID(req.SceneID)),
			OutputType:       projectOutputTypeGenerated,
			Status:           status,
			DisplayName:      filepath.Base(zipPath),
			MimeType:         "application/zip",
			FileSizeBytes:    size,
			StorageReference: zipPath,
			StorageProvider:  objectInfo.Provider,
			StorageKey:       objectInfo.Key,
			StorageETag:      objectInfo.ETag,
			StorageSHA256:    objectInfo.SHA256,
			FailureCategory:  failureCategory,
			FailureMessage:   failureMessage,
			Retryable:        retryable,
		})
		response.ProjectOutput = &updated.contentProjectOutput
	}
	return response, nil
}

func (s *Server) handleAISceneWorkerStatus(w http.ResponseWriter, r *http.Request) {
	if !renderer.WorkerConfigured(s.cfg.LocalAIWorkerURL) {
		jsonOK(w, aiSceneWorkerStatusResponse{
			Configured: false,
			Status:     "not_connected",
			Message:    "Local AI worker not connected",
		})
		return
	}
	client := renderer.WorkerClient{BaseURL: s.cfg.LocalAIWorkerURL, Token: s.cfg.LocalAIWorkerToken}
	health, err := client.Health(r.Context())
	if err != nil {
		jsonOK(w, aiSceneWorkerStatusResponse{
			Configured:   true,
			Status:       "error",
			Message:      err.Error(),
			DashboardURL: strings.TrimRight(s.cfg.LocalAIWorkerURL, "/"),
		})
		return
	}
	jobs, jobsErr := client.Jobs(r.Context())
	message := firstNonEmpty(health.Message, "Local AI worker connected")
	if jobsErr != nil {
		message += "; jobs unavailable: " + jobsErr.Error()
	}
	jsonOK(w, aiSceneWorkerStatusResponse{
		Configured:            true,
		Status:                firstNonEmpty(health.Status, "ok"),
		Message:               message,
		DashboardURL:          strings.TrimRight(s.cfg.LocalAIWorkerURL, "/"),
		GeneratorMode:         health.GeneratorMode,
		AutoCommandConfigured: health.AutoCommandConfigured,
		ModelHint:             health.ModelHint,
		OutputDir:             health.OutputDir,
		Jobs:                  jobs,
	})
}

func (s *Server) handlePlanAIScenes(w http.ResponseWriter, r *http.Request) {
	var req aiScenePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	scenes := renderer.PlanLocalAIScenes(req.Topic, "", req.Prompt, req.StylePreset, req.TargetLengthSeconds)
	jsonOK(w, aiScenePlanResponse{
		RendererVersion: renderer.LocalAISceneRendererVersion,
		Scenes:          scenes,
		ModelHint:       "auto",
	})
}

func (s *Server) handleGenerateAIScenes(w http.ResponseWriter, r *http.Request) {
	var req aiSceneGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	generationID := newAISceneGenerationID()
	clipID := "ai-scenes-" + time.Now().UTC().Format("20060102-150405")
	scenes := renderer.PlanLocalAIScenes(req.Topic, "", req.Prompt, req.StylePreset, req.TargetLengthSeconds)
	now := time.Now().UTC()
	job := &aiSceneGenerationJob{
		GenerationID:        generationID,
		WorkspaceID:         workspaceID,
		ClipID:              clipID,
		Request:             req,
		StartedAt:           now,
		UpdatedAt:           now,
		Status:              "creating_scenes",
		ProgressPercent:     15,
		CurrentStep:         "Creating scenes",
		EstimatedNextAction: "Sending scene prompts to local AI worker.",
		Response: aiSceneGenerateResponse{
			GenerationID:     generationID,
			Success:          false,
			RenderStatus:     "queued",
			RendererVersion:  renderer.LocalAISceneRendererVersion,
			ClipID:           clipID,
			WorkerConfigured: renderer.WorkerConfigured(s.cfg.LocalAIWorkerURL),
			ModelHint:        "auto",
			ScenePrompts:     scenes,
			GenerationStatus: "creating_scenes",
			IncludedFiles:    []string{},
		},
	}
	s.storeAISceneGeneration(job)
	go s.runAISceneGeneration(generationID)
	jsonOK(w, job.Response)
}

func (s *Server) runAISceneGeneration(generationID string) {
	job := s.getAISceneGeneration(generationID)
	if job == nil {
		return
	}
	s.updateAISceneGeneration(generationID, func(job *aiSceneGenerationJob) {
		job.Status = "sending_to_worker"
		job.ProgressPercent = 30
		job.CurrentStep = "Sending to local AI worker"
		job.EstimatedNextAction = "Waiting for the local AI generator."
		job.Response.GenerationStatus = job.Status
		job.UpdatedAt = time.Now().UTC()
	})
	time.Sleep(200 * time.Millisecond)
	s.updateAISceneGeneration(generationID, func(job *aiSceneGenerationJob) {
		job.Status = "running"
		job.ProgressPercent = 50
		job.CurrentStep = "Generating video scenes"
		job.EstimatedNextAction = "The local AI worker is rendering scenes."
		job.Response.GenerationStatus = job.Status
		job.UpdatedAt = time.Now().UTC()
	})
	result := renderer.RenderLocalAISceneReel(context.Background(), renderer.Config{
		OutputDir:          s.cfg.MediaOutputDir,
		FFmpegPath:         s.cfg.FFmpegPath,
		FFprobePath:        s.cfg.FFprobePath,
		LocalAIWorkerURL:   s.cfg.LocalAIWorkerURL,
		LocalAIWorkerToken: s.cfg.LocalAIWorkerToken,
	}, renderer.LocalAISceneInput{
		WorkspaceID:         job.WorkspaceID,
		ClipID:              job.ClipID,
		Topic:               job.Request.Topic,
		Prompt:              job.Request.Prompt,
		StylePreset:         job.Request.StylePreset,
		TargetLengthSeconds: job.Request.TargetLengthSeconds,
		Branding:            job.Request.Branding,
	})
	response := aiSceneGenerateResponse{
		GenerationID:     generationID,
		Success:          result.Status == renderer.StatusCompleted,
		RenderStatus:     result.Status,
		Notes:            result.Notes,
		RendererVersion:  renderer.LocalAISceneRendererVersion,
		ClipID:           job.ClipID,
		WorkerConfigured: result.WorkerURLConfigured,
		ModelHint:        result.ModelHint,
		ScenePrompts:     result.ScenePrompts,
		SceneJobIDs:      result.SceneJobIDs,
		SceneJobs:        result.SceneJobs,
		GenerationStatus: result.GenerationStatus,
		FallbackReason:   result.FallbackReason,
		TimeoutSeconds:   120,
		NextAction:       "Manual worker mode: generate this scene in Pinokio/Wan2GP, then save it as output.mp4 in the shown job folder.",
		VideoPath:        result.VideoPath,
		ThumbnailPath:    result.ThumbnailPath,
	}
	if len(result.SceneJobs) > 0 {
		response.ManualOutputPath = result.SceneJobs[0].ManualOutputPath
		response.TimeoutSeconds = result.SceneJobs[0].TimeoutSeconds
		response.NextAction = result.SceneJobs[0].NextAction
	}
	if result.Status != renderer.StatusCompleted {
		progress, step, nextAction := aiSceneProgressForStatus(response.GenerationStatus, false)
		s.updateAISceneGeneration(generationID, func(job *aiSceneGenerationJob) {
			job.Status = response.GenerationStatus
			job.ProgressPercent = progress
			job.CurrentStep = step
			job.EstimatedNextAction = nextAction
			job.Response = response
			job.Err = response.Notes
			job.UpdatedAt = time.Now().UTC()
		})
		return
	}
	s.updateAISceneGeneration(generationID, func(job *aiSceneGenerationJob) {
		job.Status = "stitching"
		job.ProgressPercent = 75
		job.CurrentStep = "Stitching final video"
		job.EstimatedNextAction = "Packaging the final video."
		job.Response = response
		job.UpdatedAt = time.Now().UTC()
	})
	sceneMetadataPath := filepath.Join(filepath.Dir(result.VideoPath), "scene-metadata.json")
	exportDir := filepath.Join(s.cfg.ExportDir, job.WorkspaceID, "clip-studio")
	s.updateAISceneGeneration(generationID, func(job *aiSceneGenerationJob) {
		job.Status = "packaging"
		job.ProgressPercent = 90
		job.CurrentStep = "Packaging download"
		job.EstimatedNextAction = "Preparing download files."
		job.Response = response
		job.UpdatedAt = time.Now().UTC()
	})
	zipPath, included, err := storage.BuildLocalAISceneExportZip(exportDir, job.ClipID, result.VideoPath, result.ThumbnailPath, sceneMetadataPath, storage.LocalAISceneExportManifest{
		ClipID:              job.ClipID,
		Prompt:              job.Request.Prompt,
		StylePreset:         job.Request.StylePreset,
		TargetLengthSeconds: job.Request.TargetLengthSeconds,
		Branding:            job.Request.Branding,
		Metadata: renderer.LocalAISceneMetadata{
			RendererVersion:     renderer.LocalAISceneRendererVersion,
			WorkerURLConfigured: result.WorkerURLConfigured,
			ModelHint:           result.ModelHint,
			ScenePrompts:        result.ScenePrompts,
			SceneJobIDs:         result.SceneJobIDs,
			SceneJobs:           result.SceneJobs,
			GenerationStatus:    result.GenerationStatus,
			FallbackReason:      result.FallbackReason,
		},
	})
	if err != nil {
		response.Success = false
		response.RenderStatus = renderer.StatusFailed
		response.GenerationStatus = "failed"
		response.Notes = "local AI scene export failed: " + err.Error()
		s.updateAISceneGeneration(generationID, func(job *aiSceneGenerationJob) {
			job.Status = "failed"
			job.ProgressPercent = 90
			job.CurrentStep = "Packaging failed"
			job.EstimatedNextAction = "Retry generation."
			job.Response = response
			job.Err = response.Notes
			job.UpdatedAt = time.Now().UTC()
		})
		return
	}
	response.ZipFilename = filepath.Base(zipPath)
	response.DownloadURL = "/api/clip-studio/download/" + filepath.Base(zipPath)
	response.IncludedFiles = included
	videoFilename := aiSceneVideoFilename(time.Now().UTC())
	s.updateAISceneGeneration(generationID, func(job *aiSceneGenerationJob) {
		job.Status = "completed"
		job.ProgressPercent = 100
		job.CurrentStep = "Video ready"
		job.EstimatedNextAction = "Download the generated video or ZIP package."
		job.Downloadable = true
		job.Response = response
		job.VideoDownloadURL = "/api/clip-studio/ai-scenes/generations/" + generationID + "/video"
		job.VideoDownloadName = videoFilename
		job.UpdatedAt = time.Now().UTC()
	})
}

func (s *Server) handleGetAISceneGeneration(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	job := s.getAISceneGeneration(id)
	if job == nil {
		jsonError(w, "AI scene generation not found", http.StatusNotFound)
		return
	}
	jsonOK(w, aiSceneGenerationStatus(job))
}

func (s *Server) handleDownloadAISceneVideo(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	job := s.getAISceneGeneration(id)
	if job == nil {
		jsonError(w, "AI scene generation not found", http.StatusNotFound)
		return
	}
	if !job.Downloadable || strings.TrimSpace(job.Response.VideoPath) == "" {
		jsonError(w, "AI scene video is not ready", http.StatusConflict)
		return
	}
	if _, err := os.Stat(job.Response.VideoPath); err != nil {
		jsonError(w, "AI scene video not found", http.StatusNotFound)
		return
	}
	filename := firstNonEmpty(job.VideoDownloadName, aiSceneVideoFilename(job.StartedAt))
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	http.ServeFile(w, r, job.Response.VideoPath)
}

func (s *Server) handleDownloadClipStudio(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := s.defaultWorkspaceID(r.Context())
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	filename := strings.TrimPrefix(r.URL.Path, "/api/clip-studio/download/")
	filename = filepath.Base(filename)
	if filename == "." || filename == "/" || filename == "" {
		jsonError(w, "clip studio export filename is required", http.StatusBadRequest)
		return
	}
	path := filepath.Join(s.cfg.ExportDir, workspaceID, "clip-studio", filename)
	if _, err := os.Stat(path); err != nil {
		jsonError(w, "clip studio export not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	http.ServeFile(w, r, path)
}

func (s *Server) createClipStudioSourceFromURL(ctx context.Context, workspaceID string, req clipStudioSourceRequest) (clipStudioSourceMetadata, error) {
	sourceURL := strings.TrimSpace(req.SourceURL)
	parsed, err := url.Parse(sourceURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return clipStudioSourceMetadata{}, fmt.Errorf("source_url must be a valid URL")
	}
	rights := req.Rights
	rights.SourceURL = sourceURL
	rights.UserConfirmedRights = req.RightsConfirmed || rights.UserConfirmedRights
	if rights.PlatformSource == "" {
		rights.PlatformSource = firstNonEmpty(req.PlatformSource, parsed.Host)
	}

	sourceID := newClipStudioSourceID()
	ext := strings.ToLower(filepath.Ext(parsed.Path))
	watchURL := isPlatformWatchURL(parsed)
	direct := !watchURL && isSupportedClipVideoExt(ext)
	message := clipStudioReferenceOnlyMessage(parsed.Host)
	meta := clipStudioSourceMetadata{
		SourceID:      sourceID,
		Kind:          "url",
		URL:           sourceURL,
		SourceModel:   renderer.ClipSourceExternalURLPendingRightsConfirmation,
		Rights:        rights,
		Status:        "metadata_only",
		Message:       message,
		CreatedAt:     time.Now().UTC(),
		DirectVideo:   direct,
		SupportedType: direct,
	}
	if watchURL {
		meta.Message = clipStudioWatchURLMessage
		if err := s.writeClipStudioSource(workspaceID, meta); err != nil {
			return clipStudioSourceMetadata{}, fmt.Errorf("store source metadata failed: %w", err)
		}
		return meta, nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, clipStudioURLImportTimeout)
	defer cancel()
	probe, err := probeDirectClipURL(probeCtx, sourceURL)
	if err != nil {
		return clipStudioSourceMetadata{}, err
	}
	direct = direct || probe.DirectVideo
	meta.DirectVideo = direct
	meta.SupportedType = direct
	if direct {
		if ext == "" || !isSupportedClipVideoExt(ext) {
			ext = extFromVideoContentType(probe.ContentType)
		}
	}
	if !rights.UserConfirmedRights {
		meta.Status = "rights_required"
		meta.Message = "Confirm rights before downloading this direct video URL."
		if err := s.writeClipStudioSource(workspaceID, meta); err != nil {
			return clipStudioSourceMetadata{}, fmt.Errorf("store source metadata failed: %w", err)
		}
		return meta, nil
	}
	dstDir := s.clipStudioSourceDir(workspaceID, sourceID)
	if err := os.MkdirAll(dstDir, 0750); err != nil {
		return clipStudioSourceMetadata{}, fmt.Errorf("create source storage failed: %w", err)
	}
	dstPath := filepath.Join(dstDir, "source"+ext)
	downloadCtx, cancel := context.WithTimeout(ctx, clipStudioURLImportTimeout)
	defer cancel()
	size, contentType, err := downloadDirectClipSource(downloadCtx, sourceURL, dstPath)
	if err != nil {
		return clipStudioSourceMetadata{}, fmt.Errorf("download direct video failed: %w", err)
	}
	objectKey, err := clipStudioSourceObjectKey(workspaceID, sourceID, ext)
	if err != nil {
		return clipStudioSourceMetadata{}, fmt.Errorf("create source object key failed: %w", err)
	}
	objectInfo, err := s.storeMediaFile(ctx, workspaceID, objectKey, dstPath, contentType, filepath.Base(parsed.Path))
	if err != nil {
		return clipStudioSourceMetadata{}, fmt.Errorf("store source in media storage failed: %w", err)
	}
	meta.FilePath = dstPath
	meta.StorageProvider = objectInfo.Provider
	meta.StorageKey = objectInfo.Key
	meta.StorageETag = objectInfo.ETag
	meta.StorageSHA256 = objectInfo.SHA256
	meta.SizeBytes = size
	meta.ContentType = contentType
	meta.SourceModel = renderer.ClipSourceUserUpload
	meta.Status = "ready"
	meta.Message = clipStudioImportedMessage
	if err := s.writeClipStudioSource(workspaceID, meta); err != nil {
		return clipStudioSourceMetadata{}, fmt.Errorf("store source metadata failed: %w", err)
	}
	return meta, nil
}

func (s *Server) clipStudioSourceDir(workspaceID, sourceID string) string {
	return filepath.Join(s.cfg.MediaOutputDir, workspaceID, "clip-studio-sources", filepath.Base(sourceID))
}

func clipStudioSourceObjectKey(workspaceID, sourceID, ext string) (string, error) {
	if ext == "" {
		ext = ".mp4"
	}
	return blobstore.JoinKey("workspaces", blobstore.SafeSegment(workspaceID, "workspace"), "clip-studio", "sources", blobstore.SafeSegment(sourceID, "source"), "source"+ext)
}

func clipStudioOutputObjectKey(workspaceID, kind, id, filename string) (string, error) {
	return blobstore.JoinKey("workspaces", blobstore.SafeSegment(workspaceID, "workspace"), "clip-studio", kind, blobstore.SafeSegment(id, "output"), blobstore.SafeSegment(filename, "package.zip"))
}

func (s *Server) storeMediaFile(ctx context.Context, workspaceID, key, path, contentType, displayName string) (blobstore.ObjectInfo, error) {
	store, err := s.ensureMediaStore()
	if err != nil {
		return blobstore.ObjectInfo{}, err
	}
	info, err := store.PutFile(ctx, key, path, blobstore.PutOptions{
		ContentType:        blobstore.DetectContentType(path, contentType),
		ContentDisposition: `attachment; filename="` + safeDownloadFilename(displayName, contentType) + `"`,
		OriginalFilename:   displayName,
	})
	if err != nil {
		return blobstore.ObjectInfo{}, err
	}
	return info, nil
}

func (s *Server) materializeClipStudioSource(ctx context.Context, workspaceID string, meta clipStudioSourceMetadata) (string, func(), error) {
	_ = workspaceID
	if meta.StorageKey == "" {
		if meta.FilePath == "" {
			return "", nil, errors.New("source media is unavailable")
		}
		return meta.FilePath, func() {}, nil
	}
	store, err := s.ensureMediaStore()
	if err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp("", "trendcortex-source-*")
	if err != nil {
		return "", nil, err
	}
	cleanupDir := func() { _ = os.RemoveAll(dir) }
	ext := filepath.Ext(meta.OriginalName)
	if ext == "" && meta.URL != "" {
		ext = filepath.Ext(meta.URL)
	}
	if ext == "" {
		ext = ".mp4"
	}
	path, cleanupFile, _, err := store.Materialize(ctx, meta.StorageKey, dir, "source"+ext)
	if err != nil {
		cleanupDir()
		return "", nil, err
	}
	return path, func() {
		if cleanupFile != nil {
			cleanupFile()
		}
		cleanupDir()
	}, nil
}

func (s *Server) writeClipStudioSource(workspaceID string, meta clipStudioSourceMetadata) error {
	if s.db != nil {
		rights, _ := json.Marshal(meta.Rights)
		_, err := s.db.Exec(`
			INSERT INTO clip_studio_sources (
				id, workspace_id, source_kind, original_filename, original_url, storage_provider, storage_key,
				storage_etag, storage_checksum_sha256, content_type, size_bytes, source_model, rights_metadata,
				status, status_message, direct_video, supported_type, created_at, updated_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,NOW())
			ON CONFLICT (id) DO UPDATE SET
				original_filename = EXCLUDED.original_filename,
				original_url = EXCLUDED.original_url,
				storage_provider = EXCLUDED.storage_provider,
				storage_key = EXCLUDED.storage_key,
				storage_etag = EXCLUDED.storage_etag,
				storage_checksum_sha256 = EXCLUDED.storage_checksum_sha256,
				content_type = EXCLUDED.content_type,
				size_bytes = EXCLUDED.size_bytes,
				source_model = EXCLUDED.source_model,
				rights_metadata = EXCLUDED.rights_metadata,
				status = EXCLUDED.status,
				status_message = EXCLUDED.status_message,
				direct_video = EXCLUDED.direct_video,
				supported_type = EXCLUDED.supported_type,
				updated_at = NOW()`,
			meta.SourceID, workspaceID, meta.Kind, nullableString(meta.OriginalName), nullableString(meta.URL),
			nullableString(meta.StorageProvider), nullableString(meta.StorageKey), nullableString(meta.StorageETag),
			nullableString(meta.StorageSHA256), nullableString(meta.ContentType), nullableInt64(meta.SizeBytes),
			nullableString(meta.SourceModel), rights, meta.Status, nullableString(meta.Message), meta.DirectVideo, meta.SupportedType, meta.CreatedAt)
		if err != nil {
			return err
		}
	}
	dir := s.clipStudioSourceDir(workspaceID, meta.SourceID)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "source.json"), payload, 0640)
}

func (s *Server) readClipStudioSource(ctx context.Context, workspaceID, sourceID string) (clipStudioSourceMetadata, error) {
	if s.db != nil {
		var meta clipStudioSourceMetadata
		var rights []byte
		var originalName, originalURL, storageProvider, storageKey, storageETag, storageSHA, contentType, sourceModel, message sql.NullString
		var size sql.NullInt64
		var createdAt, updatedAt time.Time
		err := s.db.QueryRowContext(ctx, `
			SELECT id, source_kind, original_filename, original_url, storage_provider, storage_key, storage_etag,
				storage_checksum_sha256, content_type, size_bytes, source_model, rights_metadata, status,
				status_message, direct_video, supported_type, created_at, updated_at
			FROM clip_studio_sources
			WHERE workspace_id = $1 AND id = $2`, workspaceID, sourceID).
			Scan(&meta.SourceID, &meta.Kind, &originalName, &originalURL, &storageProvider, &storageKey, &storageETag,
				&storageSHA, &contentType, &size, &sourceModel, &rights, &meta.Status, &message, &meta.DirectVideo, &meta.SupportedType, &createdAt, &updatedAt)
		if err == nil {
			meta.OriginalName = originalName.String
			meta.URL = originalURL.String
			meta.StorageProvider = storageProvider.String
			meta.StorageKey = storageKey.String
			meta.StorageETag = storageETag.String
			meta.StorageSHA256 = storageSHA.String
			meta.ContentType = contentType.String
			meta.SizeBytes = size.Int64
			meta.SourceModel = sourceModel.String
			meta.Message = message.String
			meta.CreatedAt = createdAt
			_ = updatedAt
			_ = json.Unmarshal(rights, &meta.Rights)
			return meta, nil
		}
	}
	path := filepath.Join(s.clipStudioSourceDir(workspaceID, sourceID), "source.json")
	f, err := os.Open(path)
	if err != nil {
		return clipStudioSourceMetadata{}, err
	}
	defer f.Close()
	var meta clipStudioSourceMetadata
	if err := json.NewDecoder(f).Decode(&meta); err != nil {
		return clipStudioSourceMetadata{}, err
	}
	return meta, nil
}

func newClipStudioSourceID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "src-" + time.Now().UTC().Format("20060102150405")
	}
	return fmt.Sprintf("src-%s-%x", time.Now().UTC().Format("20060102150405"), b)
}

func newAISceneGenerationID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "gen-" + time.Now().UTC().Format("20060102150405")
	}
	return fmt.Sprintf("gen-%s-%x", time.Now().UTC().Format("20060102150405"), b)
}

func (s *Server) ensureAISceneJobs() {
	if s.aiSceneJobs == nil {
		s.aiSceneJobs = map[string]*aiSceneGenerationJob{}
	}
}

func (s *Server) storeAISceneGeneration(job *aiSceneGenerationJob) {
	s.aiSceneMu.Lock()
	defer s.aiSceneMu.Unlock()
	s.ensureAISceneJobs()
	s.aiSceneJobs[job.GenerationID] = job
}

func (s *Server) getAISceneGeneration(id string) *aiSceneGenerationJob {
	s.aiSceneMu.Lock()
	defer s.aiSceneMu.Unlock()
	s.ensureAISceneJobs()
	job := s.aiSceneJobs[id]
	if job == nil {
		return nil
	}
	copy := *job
	return &copy
}

func (s *Server) updateAISceneGeneration(id string, fn func(*aiSceneGenerationJob)) {
	s.aiSceneMu.Lock()
	defer s.aiSceneMu.Unlock()
	s.ensureAISceneJobs()
	if job := s.aiSceneJobs[id]; job != nil {
		fn(job)
	}
}

func aiSceneGenerationStatus(job *aiSceneGenerationJob) aiSceneGenerationStatusResponse {
	resp := job.Response
	progress := job.ProgressPercent
	step := job.CurrentStep
	nextAction := job.EstimatedNextAction
	if progress == 0 || step == "" || nextAction == "" {
		progress, step, nextAction = aiSceneProgressForStatus(job.Status, job.Downloadable)
	}
	return aiSceneGenerationStatusResponse{
		GenerationID:        job.GenerationID,
		ProgressPercent:     progress,
		CurrentStep:         step,
		Status:              firstNonEmpty(job.Status, resp.GenerationStatus, resp.RenderStatus),
		EstimatedNextAction: nextAction,
		Downloadable:        job.Downloadable,
		VideoURL:            job.VideoDownloadURL,
		VideoFilename:       job.VideoDownloadName,
		ZipURL:              resp.DownloadURL,
		ZipFilename:         resp.ZipFilename,
		Notes:               firstNonEmpty(resp.Notes, job.Err),
		RendererVersion:     firstNonEmpty(resp.RendererVersion, renderer.LocalAISceneRendererVersion),
		ClipID:              resp.ClipID,
		WorkerConfigured:    resp.WorkerConfigured,
		ModelHint:           resp.ModelHint,
		ScenePrompts:        resp.ScenePrompts,
		SceneJobIDs:         resp.SceneJobIDs,
		SceneJobs:           resp.SceneJobs,
		IncludedFiles:       resp.IncludedFiles,
	}
}

func aiSceneProgressForStatus(status string, downloadable bool) (int, string, string) {
	if downloadable || status == "completed" {
		return 100, "Video ready", "Download the generated video or ZIP package."
	}
	switch status {
	case "creating_scenes", "planned":
		return 15, "Creating scenes", "Sending scene prompts to local AI worker."
	case "sending_to_worker", "waiting_for_manual_output", "pending", "queued":
		return 30, "Sending to local AI worker", "Waiting for local generation."
	case "running":
		return 50, "Generating video scenes", "The local AI worker is rendering scenes."
	case "stitching":
		return 75, "Stitching final video", "Packaging the final video."
	case "packaging":
		return 90, "Packaging download", "Preparing download files."
	case "generator_not_configured":
		return 0, "Generator not configured", "Configure generator"
	case "failed", "timed_out":
		return 30, "Generation failed", "Retry after fixing the generator."
	default:
		return 0, "Preparing scene plan", "Preparing scene plan."
	}
}

func aiSceneVideoFilename(t time.Time) string {
	if t.IsZero() {
		t = time.Now().UTC()
	}
	return "trendcortex-ai-video-" + t.UTC().Format("20060102-1504") + ".mp4"
}

func isSupportedClipVideoExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".mp4", ".mov", ".webm":
		return true
	default:
		return false
	}
}

func clipStudioReferenceOnlyMessage(host string) string {
	normalized := strings.TrimPrefix(strings.ToLower(host), "www.")
	if normalized == "youtu.be" || normalized == "youtube.com" || strings.HasSuffix(normalized, ".youtube.com") {
		return clipStudioWatchURLMessage
	}
	return "URL saved as reference only. Upload the source video file or connect an approved source before generating clips."
}

func copyUploadedClipSource(file multipart.File, dstPath string) (int64, error) {
	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0640)
	if err != nil {
		return 0, err
	}
	defer dst.Close()
	return io.Copy(dst, io.LimitReader(file, clipStudioMaxSourceBytes))
}

type clipStudioURLProbe struct {
	ContentType   string
	ContentLength int64
	DirectVideo   bool
}

func probeDirectClipURL(ctx context.Context, sourceURL string) (clipStudioURLProbe, error) {
	probe, err := requestClipURLProbe(ctx, http.MethodHead, sourceURL)
	if err == nil {
		return probe, nil
	}
	if strings.Contains(err.Error(), "HTTP 405") || strings.Contains(err.Error(), "HTTP 501") {
		return requestClipURLProbe(ctx, http.MethodGet, sourceURL)
	}
	return clipStudioURLProbe{}, err
}

func requestClipURLProbe(ctx context.Context, method, sourceURL string) (clipStudioURLProbe, error) {
	req, err := http.NewRequestWithContext(ctx, method, sourceURL, nil)
	if err != nil {
		return clipStudioURLProbe{}, err
	}
	res, err := clipStudioHTTPClient.Do(req)
	if err != nil {
		return clipStudioURLProbe{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return clipStudioURLProbe{}, fmt.Errorf("source returned HTTP %d", res.StatusCode)
	}
	contentType := normalizeMediaType(res.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "text/html") {
		return clipStudioURLProbe{}, fmt.Errorf("source URL is an HTML page, not a direct video file")
	}
	if contentType == "" || !strings.HasPrefix(contentType, "video/") {
		return clipStudioURLProbe{}, fmt.Errorf("source URL must return a video content-type")
	}
	if res.ContentLength > clipStudioMaxSourceBytes {
		return clipStudioURLProbe{}, fmt.Errorf("source video exceeds the 512MB limit")
	}
	return clipStudioURLProbe{ContentType: contentType, ContentLength: res.ContentLength, DirectVideo: true}, nil
}

func downloadDirectClipSource(ctx context.Context, sourceURL, dstPath string) (int64, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return 0, "", err
	}
	res, err := clipStudioHTTPClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return 0, "", fmt.Errorf("source returned HTTP %d", res.StatusCode)
	}
	contentType := normalizeMediaType(res.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "text/html") {
		return 0, "", fmt.Errorf("source URL is an HTML page, not a direct video file")
	}
	if contentType == "" || !strings.HasPrefix(contentType, "video/") {
		return 0, "", fmt.Errorf("source URL must return a video content-type")
	}
	if res.ContentLength > clipStudioMaxSourceBytes {
		return 0, "", fmt.Errorf("source video exceeds the 512MB limit")
	}
	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0640)
	if err != nil {
		return 0, "", err
	}
	defer dst.Close()
	size, err := io.Copy(dst, io.LimitReader(res.Body, clipStudioMaxSourceBytes+1))
	if err != nil {
		return 0, "", err
	}
	if size > clipStudioMaxSourceBytes {
		_ = os.Remove(dstPath)
		return 0, "", fmt.Errorf("source video exceeds the 512MB limit")
	}
	return size, contentType, nil
}

func normalizeMediaType(value string) string {
	if idx := strings.Index(value, ";"); idx >= 0 {
		value = value[:idx]
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func extFromVideoContentType(contentType string) string {
	switch normalizeMediaType(contentType) {
	case "video/quicktime", "video/mov":
		return ".mov"
	case "video/webm":
		return ".webm"
	default:
		return ".mp4"
	}
}

func isPlatformWatchURL(parsed *url.URL) bool {
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	if host == "youtu.be" || host == "youtube.com" || strings.HasSuffix(host, ".youtube.com") {
		return true
	}
	if host == "vimeo.com" || strings.HasSuffix(host, ".vimeo.com") {
		return true
	}
	if strings.Contains(strings.ToLower(parsed.Path), "/watch") {
		return true
	}
	socialHosts := []string{"tiktok.com", "instagram.com", "facebook.com", "fb.watch", "x.com", "twitter.com", "threads.net"}
	for _, socialHost := range socialHosts {
		if host == socialHost || strings.HasSuffix(host, "."+socialHost) {
			return true
		}
	}
	return false
}

func clipLengthToSeconds(value string) float64 {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "15s", "15":
		return 15
	case "30s", "30":
		return 30
	case "60s", "60":
		return 60
	case "3min", "3m", "180":
		return 180
	default:
		return 15
	}
}

func normalizeClipCount(v int) int {
	switch v {
	case 1, 3, 6:
		return v
	default:
		return 3
	}
}

func evenlySpacedClipRanges(duration *float64, clipLength float64, count int) []renderer.ClipManualRange {
	if clipLength <= 0 {
		clipLength = 15
	}
	if duration == nil || *duration <= 0 {
		ranges := make([]renderer.ClipManualRange, 0, count)
		for i := 0; i < count; i++ {
			start := float64(i) * clipLength
			ranges = append(ranges, renderer.ClipManualRange{StartSeconds: start, EndSeconds: start + clipLength})
		}
		return ranges
	}
	total := *duration
	if clipLength > total {
		clipLength = total
	}
	if clipLength < 0.5 {
		clipLength = total
	}
	ranges := make([]renderer.ClipManualRange, 0, count)
	maxStart := total - clipLength
	if maxStart < 0 {
		maxStart = 0
	}
	for i := 0; i < count; i++ {
		start := 0.0
		if count > 1 {
			start = maxStart * float64(i) / float64(count-1)
		}
		end := start + clipLength
		if end > total {
			end = total
		}
		if end <= start {
			end = start + 0.5
		}
		ranges = append(ranges, renderer.ClipManualRange{StartSeconds: start, EndSeconds: end})
	}
	return ranges
}

func applyAdvancedRights(rights *renderer.ClipRightsMetadata, advanced clipStudioGenerateAdvanced) {
	rights.SourceTitle = firstNonEmpty(advanced.SourceTitle, rights.SourceTitle)
	rights.SourceCreator = firstNonEmpty(advanced.SourceCreator, rights.SourceCreator)
	rights.SourceLicense = firstNonEmpty(advanced.SourceLicense, rights.SourceLicense)
	rights.AttributionText = firstNonEmpty(advanced.AttributionText, rights.AttributionText)
	rights.CopyrightOverlayText = firstNonEmpty(advanced.CopyrightOverlayText, rights.CopyrightOverlayText)
	rights.PlatformSource = firstNonEmpty(advanced.PlatformSource, rights.PlatformSource)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func optionalSceneID(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func outputScopeFromRequest(value string, sceneID *string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == projectOutputScopeScene || (trimmed == "" && sceneID != nil) {
		return projectOutputScopeScene
	}
	return projectOutputScopeProject
}

func generationBatchID(idempotencyKey string) string {
	key := strings.TrimSpace(idempotencyKey)
	if key != "" {
		key = strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				return r
			}
			return '-'
		}, key)
		return "clips-" + limitText(key, 120)
	}
	return "clips-" + time.Now().UTC().Format("20060102-150405")
}

func safeOutputDisplayName(filename, fallback string) string {
	name := strings.TrimSpace(filepath.Base(filename))
	if name == "." || name == "" {
		return fallback
	}
	return name
}

func publicClipStudioSourceMetadata(meta clipStudioSourceMetadata) clipStudioSourceMetadata {
	meta.FilePath = ""
	return meta
}

func clipStudioRenderForTest(ctx context.Context, cfg renderer.Config, input renderer.ClipInput) renderer.Result {
	return renderer.RenderManualClip(ctx, cfg, input)
}
