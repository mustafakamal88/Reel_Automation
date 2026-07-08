package http

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	IncludeCaptions bool                             `json:"include_captions"`
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
	SourceID      string                      `json:"source_id"`
	Kind          string                      `json:"kind"`
	OriginalName  string                      `json:"original_name,omitempty"`
	URL           string                      `json:"url,omitempty"`
	FilePath      string                      `json:"file_path,omitempty"`
	ContentType   string                      `json:"content_type,omitempty"`
	SizeBytes     int64                       `json:"size_bytes,omitempty"`
	SourceModel   string                      `json:"source_model,omitempty"`
	Rights        renderer.ClipRightsMetadata `json:"rights"`
	Status        string                      `json:"status"`
	Message       string                      `json:"message,omitempty"`
	CreatedAt     time.Time                   `json:"created_at"`
	DirectVideo   bool                        `json:"direct_video"`
	SupportedType bool                        `json:"supported_type"`
}

type clipStudioSourceRequest struct {
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
	CanRender     bool                     `json:"can_render"`
	DirectVideo   bool                     `json:"direct_video"`
	DownloadReady bool                     `json:"download_ready"`
}

type clipStudioGenerateRequest struct {
	SourceID        string                        `json:"source_id,omitempty"`
	SourceURL       string                        `json:"source_url,omitempty"`
	Prompt          string                        `json:"prompt"`
	ClipLength      string                        `json:"clip_length"`
	ClipCount       int                           `json:"clip_count"`
	Branding        renderer.ClipBrandingSettings `json:"branding"`
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
	HighlightDetection string                   `json:"highlight_detection"`
	GeneratedClipJobs  []clipStudioGeneratedJob `json:"generated_clip_jobs"`
	ZipFilename        string                   `json:"zip_filename,omitempty"`
	DownloadURL        string                   `json:"download_url,omitempty"`
	IncludedFiles      []string                 `json:"included_files"`
}

type aiSceneWorkerStatusResponse struct {
	Configured bool   `json:"configured"`
	Status     string `json:"status"`
	Message    string `json:"message"`
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
	Success          bool                 `json:"success"`
	RenderStatus     string               `json:"render_status"`
	Notes            string               `json:"notes,omitempty"`
	RendererVersion  string               `json:"renderer_version"`
	ClipID           string               `json:"clip_id"`
	WorkerConfigured bool                 `json:"worker_url_configured"`
	ModelHint        string               `json:"model_hint"`
	ScenePrompts     []renderer.ScenePlan `json:"scene_prompts"`
	SceneJobIDs      []string             `json:"scene_job_ids"`
	GenerationStatus string               `json:"generation_status"`
	FallbackReason   string               `json:"fallback_reason,omitempty"`
	ZipFilename      string               `json:"zip_filename,omitempty"`
	DownloadURL      string               `json:"download_url,omitempty"`
	IncludedFiles    []string             `json:"included_files"`
	VideoPath        string               `json:"video_path,omitempty"`
	ThumbnailPath    string               `json:"thumbnail_path,omitempty"`
}

var clipStudioHTTPClient = http.DefaultClient

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
	if strings.TrimSpace(req.SourceVideoPath) == "" && strings.TrimSpace(req.SourceID) != "" {
		meta, err := s.readClipStudioSource(r.Context(), workspaceID, req.SourceID)
		if err != nil {
			jsonError(w, "clip studio source not found: "+err.Error(), http.StatusBadRequest)
			return
		}
		req.SourceVideoPath = meta.FilePath
		if req.Rights.SourceURL == "" {
			req.Rights = meta.Rights
		}
		if req.SourceModel == "" {
			req.SourceModel = meta.SourceModel
		}
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
		IncludeCaptions: req.IncludeCaptions,
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

	jsonOK(w, clipStudioRenderResponse{
		Success:       true,
		RenderStatus:  result.Status,
		Notes:         result.Notes,
		ClipID:        clipID,
		ZipFilename:   filepath.Base(zipPath),
		DownloadURL:   "/api/clip-studio/download/" + filepath.Base(zipPath),
		IncludedFiles: included,
		VideoPath:     result.VideoPath,
		ThumbnailPath: result.ThumbnailPath,
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

	meta := clipStudioSourceMetadata{
		SourceID:      sourceID,
		Kind:          "upload",
		OriginalName:  header.Filename,
		FilePath:      dstPath,
		ContentType:   header.Header.Get("Content-Type"),
		SizeBytes:     size,
		SourceModel:   renderer.ClipSourceUserUpload,
		Rights:        renderer.ClipRightsMetadata{UserConfirmedRights: false, PlatformSource: "upload"},
		Status:        "ready",
		Message:       "Upload ready for clip generation.",
		CreatedAt:     time.Now().UTC(),
		DirectVideo:   true,
		SupportedType: true,
	}
	if err := s.writeClipStudioSource(workspaceID, meta); err != nil {
		jsonError(w, "store source metadata failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, clipStudioSourceResponse{
		SourceID:      sourceID,
		Status:        meta.Status,
		Message:       meta.Message,
		Metadata:      meta,
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
	direct := isSupportedClipVideoExt(ext)
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
			jsonOK(w, clipStudioSourceResponse{SourceID: sourceID, Status: meta.Status, Message: meta.Message, Metadata: meta, DirectVideo: true})
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
		meta.FilePath = dstPath
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
		Metadata:      meta,
		CanRender:     meta.Status == "ready",
		DirectVideo:   meta.DirectVideo,
		DownloadReady: meta.FilePath != "",
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

	var source clipStudioSourceMetadata
	if strings.TrimSpace(req.SourceID) != "" {
		source, err = s.readClipStudioSource(r.Context(), workspaceID, req.SourceID)
		if err != nil {
			jsonError(w, "clip studio source not found: "+err.Error(), http.StatusBadRequest)
			return
		}
	} else if strings.TrimSpace(req.SourceURL) != "" {
		sourceReq := clipStudioSourceRequest{SourceURL: req.SourceURL, RightsConfirmed: req.RightsConfirmed, Rights: req.Rights}
		source, err = s.createClipStudioSourceFromURL(r.Context(), workspaceID, sourceReq)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		jsonError(w, "source_id or source_url is required", http.StatusBadRequest)
		return
	}
	if source.FilePath == "" || source.Status != "ready" {
		jsonOK(w, clipStudioGenerateResponse{
			Success:            false,
			RenderStatus:       "unsupported_source",
			Notes:              "Upload the source file or connect an approved source before rendering.",
			SourceID:           source.SourceID,
			HighlightDetection: "not_run",
		})
		return
	}

	rights := source.Rights
	rights.UserConfirmedRights = true
	applyAdvancedRights(&rights, req.Advanced)
	clipLengthSeconds := clipLengthToSeconds(req.ClipLength)
	clipCount := normalizeClipCount(req.ClipCount)
	duration := renderer.ProbeDuration(r.Context(), s.cfg.FFprobePath, source.FilePath)
	ranges := evenlySpacedClipRanges(duration, clipLengthSeconds, clipCount)
	batchID := "clips-" + time.Now().UTC().Format("20060102-150405")
	jobs := make([]clipStudioGeneratedJob, 0, len(ranges))
	generated := make([]storage.ClipStudioGeneratedClip, 0, len(ranges))
	overallStatus := renderer.StatusCompleted
	for i, manualRange := range ranges {
		clipID := fmt.Sprintf("%s-%02d", batchID, i+1)
		input := renderer.ClipInput{
			WorkspaceID:     workspaceID,
			ClipID:          clipID,
			SourceModel:     firstNonEmpty(req.Advanced.SourceModel, source.SourceModel, renderer.ClipSourceUserUpload),
			SourceVideoPath: source.FilePath,
			Rights:          rights,
			Branding:        req.Branding,
			ManualRange:     manualRange,
			Captions:        req.Prompt,
			IncludeCaptions: strings.TrimSpace(req.Prompt) != "",
			AIHighlights:    renderer.DefaultClipAIHighlightMetadata(),
		}
		result := renderer.RenderManualClip(r.Context(), renderer.Config{
			OutputDir:   s.cfg.MediaOutputDir,
			FFmpegPath:  s.cfg.FFmpegPath,
			FFprobePath: s.cfg.FFprobePath,
		}, input)
		jobs = append(jobs, clipStudioGeneratedJob{
			ClipID:        clipID,
			RenderStatus:  result.Status,
			Notes:         result.Notes,
			ManualRange:   manualRange,
			VideoPath:     result.VideoPath,
			ThumbnailPath: result.ThumbnailPath,
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
				AIHighlights:    input.AIHighlights,
				RendererVersion: result.RendererVersion,
			},
		})
	}
	if len(generated) == 0 {
		jsonOK(w, clipStudioGenerateResponse{
			Success:            false,
			RenderStatus:       overallStatus,
			Notes:              "No clips rendered.",
			SourceID:           source.SourceID,
			HighlightDetection: "not_run",
			GeneratedClipJobs:  jobs,
		})
		return
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
		jsonError(w, "clip studio export failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonOK(w, clipStudioGenerateResponse{
		Success:            overallStatus == renderer.StatusCompleted,
		RenderStatus:       overallStatus,
		Notes:              "Generated clips with evenly spaced segments; highlight_detection: not_run.",
		SourceID:           source.SourceID,
		HighlightDetection: "not_run",
		GeneratedClipJobs:  jobs,
		ZipFilename:        filepath.Base(zipPath),
		DownloadURL:        "/api/clip-studio/download/" + filepath.Base(zipPath),
		IncludedFiles:      included,
	})
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
			Configured: true,
			Status:     "error",
			Message:    err.Error(),
		})
		return
	}
	jsonOK(w, aiSceneWorkerStatusResponse{
		Configured: true,
		Status:     firstNonEmpty(health.Status, "ok"),
		Message:    firstNonEmpty(health.Message, "Local AI worker connected"),
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
	clipID := "ai-scenes-" + time.Now().UTC().Format("20060102-150405")
	result := renderer.RenderLocalAISceneReel(r.Context(), renderer.Config{
		OutputDir:          s.cfg.MediaOutputDir,
		FFmpegPath:         s.cfg.FFmpegPath,
		FFprobePath:        s.cfg.FFprobePath,
		LocalAIWorkerURL:   s.cfg.LocalAIWorkerURL,
		LocalAIWorkerToken: s.cfg.LocalAIWorkerToken,
	}, renderer.LocalAISceneInput{
		WorkspaceID:         workspaceID,
		ClipID:              clipID,
		Topic:               req.Topic,
		Prompt:              req.Prompt,
		StylePreset:         req.StylePreset,
		TargetLengthSeconds: req.TargetLengthSeconds,
		Branding:            req.Branding,
	})
	response := aiSceneGenerateResponse{
		Success:          result.Status == renderer.StatusCompleted,
		RenderStatus:     result.Status,
		Notes:            result.Notes,
		RendererVersion:  renderer.LocalAISceneRendererVersion,
		ClipID:           clipID,
		WorkerConfigured: result.WorkerURLConfigured,
		ModelHint:        result.ModelHint,
		ScenePrompts:     result.ScenePrompts,
		SceneJobIDs:      result.SceneJobIDs,
		GenerationStatus: result.GenerationStatus,
		FallbackReason:   result.FallbackReason,
		VideoPath:        result.VideoPath,
		ThumbnailPath:    result.ThumbnailPath,
	}
	if result.Status != renderer.StatusCompleted {
		jsonOK(w, response)
		return
	}
	sceneMetadataPath := filepath.Join(filepath.Dir(result.VideoPath), "scene-metadata.json")
	exportDir := filepath.Join(s.cfg.ExportDir, workspaceID, "clip-studio")
	zipPath, included, err := storage.BuildLocalAISceneExportZip(exportDir, clipID, result.VideoPath, result.ThumbnailPath, sceneMetadataPath, storage.LocalAISceneExportManifest{
		ClipID:              clipID,
		Prompt:              req.Prompt,
		StylePreset:         req.StylePreset,
		TargetLengthSeconds: req.TargetLengthSeconds,
		Branding:            req.Branding,
		Metadata: renderer.LocalAISceneMetadata{
			RendererVersion:     renderer.LocalAISceneRendererVersion,
			WorkerURLConfigured: result.WorkerURLConfigured,
			ModelHint:           result.ModelHint,
			ScenePrompts:        result.ScenePrompts,
			SceneJobIDs:         result.SceneJobIDs,
			GenerationStatus:    result.GenerationStatus,
			FallbackReason:      result.FallbackReason,
		},
	})
	if err != nil {
		jsonError(w, "local AI scene export failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	response.ZipFilename = filepath.Base(zipPath)
	response.DownloadURL = "/api/clip-studio/download/" + filepath.Base(zipPath)
	response.IncludedFiles = included
	jsonOK(w, response)
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
	direct := isSupportedClipVideoExt(ext)
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
	if !direct {
		if err := s.writeClipStudioSource(workspaceID, meta); err != nil {
			return clipStudioSourceMetadata{}, fmt.Errorf("store source metadata failed: %w", err)
		}
		return meta, nil
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
	size, contentType, err := downloadDirectClipSource(ctx, sourceURL, dstPath)
	if err != nil {
		return clipStudioSourceMetadata{}, fmt.Errorf("download direct video failed: %w", err)
	}
	meta.FilePath = dstPath
	meta.SizeBytes = size
	meta.ContentType = contentType
	meta.Status = "ready"
	meta.Message = "Direct video URL downloaded and ready for clip generation."
	if err := s.writeClipStudioSource(workspaceID, meta); err != nil {
		return clipStudioSourceMetadata{}, fmt.Errorf("store source metadata failed: %w", err)
	}
	return meta, nil
}

func (s *Server) clipStudioSourceDir(workspaceID, sourceID string) string {
	return filepath.Join(s.cfg.MediaOutputDir, workspaceID, "clip-studio-sources", filepath.Base(sourceID))
}

func (s *Server) writeClipStudioSource(workspaceID string, meta clipStudioSourceMetadata) error {
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
	_ = ctx
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
		return "For YouTube links, upload the source video file or connect your own/approved channel source. This app does not auto-rip YouTube videos."
	}
	return "URL saved as reference only. Upload the source video file or connect an approved source before generating clips."
}

func copyUploadedClipSource(file multipart.File, dstPath string) (int64, error) {
	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0640)
	if err != nil {
		return 0, err
	}
	defer dst.Close()
	return io.Copy(dst, io.LimitReader(file, 512<<20))
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
	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0640)
	if err != nil {
		return 0, "", err
	}
	defer dst.Close()
	size, err := io.Copy(dst, io.LimitReader(res.Body, 512<<20))
	return size, res.Header.Get("Content-Type"), err
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

func clipStudioRenderForTest(ctx context.Context, cfg renderer.Config, input renderer.ClipInput) renderer.Result {
	return renderer.RenderManualClip(ctx, cfg, input)
}
