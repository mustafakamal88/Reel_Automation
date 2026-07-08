package http

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"trendcortex/api/internal/renderer"
	"trendcortex/api/internal/storage"
)

type clipStudioRenderRequest struct {
	SourceModel     string                           `json:"source_model"`
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

func clipStudioRenderForTest(ctx context.Context, cfg renderer.Config, input renderer.ClipInput) renderer.Result {
	return renderer.RenderManualClip(ctx, cfg, input)
}
