package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"trendcortex/api/internal/renderer"
	"trendcortex/api/internal/storage"
)

type renderExportTestRequest struct {
	Topic              string   `json:"topic"`
	Title              string   `json:"title"`
	Script             string   `json:"script"`
	Caption            string   `json:"caption"`
	TargetPlatforms    []string `json:"target_platforms"`
	Style              string   `json:"style"`
	Format             string   `json:"format"`
	NumberOfReels      int      `json:"number_of_reels"`
	AllowLocalFallback bool     `json:"allow_local_fallback"`
}

type renderExportTestResponse struct {
	Success        bool     `json:"success"`
	RenderStatus   string   `json:"render_status"`
	ExportStatus   string   `json:"export_status"`
	ZipFilename    string   `json:"zip_filename"`
	ZipPath        string   `json:"zip_path"`
	DownloadURL    string   `json:"download_url"`
	IncludedFiles  []string `json:"included_files"`
	Provider       string   `json:"provider"`
	FallbackReason string   `json:"fallback_reason,omitempty"`
}

// POST /api/reels/export-test
//
// Runs a single-request render + ZIP flow for manual end-to-end validation.
// It first tries the real provider-backed renderer. If provider credentials
// are unavailable and FFmpeg/FFprobe are installed, it can generate clearly
// labeled local test media so ZIP packaging remains testable without faking
// provider success.
func (s *Server) handleRenderExportTest(w http.ResponseWriter, r *http.Request) {
	var req renderExportTestRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid JSON request: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	res, err := s.runRenderExportTest(r.Context(), req)
	if err != nil {
		jsonError(w, "export test failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, res)
}

func (s *Server) runRenderExportTest(ctx context.Context, req renderExportTestRequest) (renderExportTestResponse, error) {
	req = normalizeRenderExportTestRequest(req)
	workspaceID := "export-test"
	if s.db != nil {
		if id, err := s.defaultWorkspaceID(ctx); err == nil {
			workspaceID = id
		} else if err != sql.ErrNoRows {
			return renderExportTestResponse{}, fmt.Errorf("workspace lookup failed: %w", err)
		}
	}

	date := time.Now().UTC().Format("2006-01-02")
	batchID := "export-test-" + time.Now().UTC().Format("20060102-150405")
	exportDir := filepath.Join(s.cfg.ExportDir, workspaceID, "export-test")

	reels := make([]storage.ReelExportContent, 0, req.NumberOfReels)
	entries := make([]storage.BatchExportReelEntry, 0, req.NumberOfReels)
	renderStatus := renderer.StatusCompleted
	provider := s.cfg.RenderProvider
	fallbackReason := ""

	for i := 1; i <= req.NumberOfReels; i++ {
		reelID := fmt.Sprintf("%s-reel-%02d", batchID, i)
		input := renderer.ReelInput{
			WorkspaceID:    workspaceID,
			ReelPlanID:     reelID,
			Rank:           i,
			Title:          numberedTitle(req.Title, i, req.NumberOfReels),
			Script:         req.Script,
			Description:    req.Caption,
			Hashtags:       "#trendcortex #reels",
			ThumbnailBrief: req.Style,
		}

		result := renderer.RenderReel(ctx, renderer.Config{
			Provider:     s.cfg.RenderProvider,
			OutputDir:    s.cfg.MediaOutputDir,
			OpenAIAPIKey: s.cfg.OpenAIAPIKey,
			TTSModel:     s.cfg.OpenAITTSModel,
			ImageModel:   s.cfg.OpenAIImageModel,
			FFmpegPath:   s.cfg.FFmpegPath,
			FFprobePath:  s.cfg.FFprobePath,
		}, input)
		reelProvider := s.cfg.RenderProvider
		reelFallbackReason := ""

		if result.Status != renderer.StatusCompleted && req.AllowLocalFallback {
			reelFallbackReason = result.Notes
			fallbackResult := renderer.RenderLocalTestReel(ctx, renderer.Config{
				OutputDir:   s.cfg.MediaOutputDir,
				FFmpegPath:  s.cfg.FFmpegPath,
				FFprobePath: s.cfg.FFprobePath,
			}, input)
			if fallbackResult.Status == renderer.StatusCompleted {
				result = fallbackResult
				reelProvider = "local_ffmpeg_test"
			} else {
				fallbackResult.Notes = strings.TrimSpace(reelFallbackReason + "; local fallback unavailable: " + fallbackResult.Notes)
				result = fallbackResult
			}
		}

		if result.Status != renderer.StatusCompleted {
			renderStatus = result.Status
			if fallbackReason == "" {
				fallbackReason = result.Notes
			}
		} else if reelProvider == "local_ffmpeg_test" {
			renderStatus = renderer.StatusCompleted
			provider = reelProvider
			if fallbackReason == "" {
				fallbackReason = reelFallbackReason
			}
		}

		content := exportTestReelContent(input, req, result, reelProvider, reelFallbackReason)
		reels = append(reels, content)
		entries = append(entries, storage.BatchExportReelEntry{
			Rank:         content.Rank,
			Title:        content.Title,
			Platform:     strings.Join(req.TargetPlatforms, ","),
			HasVideo:     content.VideoSrcPath != "",
			HasThumbnail: content.ThumbnailSrcPath != "",
		})
	}

	summary := storage.BatchExportSummary{
		BatchID:     batchID,
		Date:        date,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		ReelCount:   len(reels),
		Reels:       entries,
	}
	manifest := storage.ExportManifest{
		BatchID:        batchID,
		Date:           date,
		GeneratedAt:    summary.GeneratedAt,
		ExportStatus:   "completed",
		RenderStatus:   renderStatus,
		Provider:       provider,
		FallbackReason: fallbackReason,
	}

	zipPath, included, err := storage.BuildReelBatchZipWithManifest(exportDir, date, summary, reels, &manifest)
	if err != nil {
		return renderExportTestResponse{}, err
	}
	filename := filepath.Base(zipPath)
	return renderExportTestResponse{
		Success:        true,
		RenderStatus:   renderStatus,
		ExportStatus:   "completed",
		ZipFilename:    filename,
		ZipPath:        zipPath,
		DownloadURL:    "/api/reels/export-test/download/" + filename,
		IncludedFiles:  included,
		Provider:       provider,
		FallbackReason: fallbackReason,
	}, nil
}

func normalizeRenderExportTestRequest(req renderExportTestRequest) renderExportTestRequest {
	req.Topic = strings.TrimSpace(req.Topic)
	req.Title = strings.TrimSpace(req.Title)
	req.Script = strings.TrimSpace(req.Script)
	req.Caption = strings.TrimSpace(req.Caption)
	req.Style = strings.TrimSpace(req.Style)
	req.Format = strings.TrimSpace(req.Format)
	if req.Topic == "" {
		req.Topic = "TrendCortex export test"
	}
	if req.Title == "" {
		req.Title = req.Topic
	}
	if req.Script == "" {
		req.Script = "This is a TrendCortex end-to-end render and ZIP export test."
	}
	if req.Caption == "" {
		req.Caption = req.Script
	}
	if req.Style == "" {
		req.Style = "vertical editorial social video"
	}
	if req.Format == "" {
		req.Format = "vertical_9_16"
	}
	if req.NumberOfReels <= 0 {
		req.NumberOfReels = 1
	}
	if req.NumberOfReels > 6 {
		req.NumberOfReels = 6
	}
	req.TargetPlatforms = normalizePlatforms(req.TargetPlatforms)
	return req
}

func normalizePlatforms(platforms []string) []string {
	allowed := map[string]bool{"youtube": true, "tiktok": true, "instagram": true, "facebook": true, "x": true}
	var out []string
	seen := map[string]bool{}
	for _, p := range platforms {
		p = strings.ToLower(strings.TrimSpace(p))
		if allowed[p] && !seen[p] {
			out = append(out, p)
			seen[p] = true
		}
	}
	if len(out) == 0 {
		return []string{"youtube", "tiktok", "instagram", "facebook", "x"}
	}
	return out
}

func numberedTitle(title string, rank, total int) string {
	if total <= 1 {
		return title
	}
	return title + " #" + strconv.Itoa(rank)
}

func exportTestReelContent(input renderer.ReelInput, req renderExportTestRequest, result renderer.Result, provider, fallbackReason string) storage.ReelExportContent {
	hasVideo := result.Status == renderer.StatusCompleted && fileExists(result.VideoPath)
	hasThumbnail := result.Status == renderer.StatusCompleted && fileExists(result.ThumbnailPath)
	metadata := storage.ReelExportMetadata{
		ReelPlanID:      input.ReelPlanID,
		Rank:            input.Rank,
		Platform:        strings.Join(req.TargetPlatforms, ","),
		Platforms:       req.TargetPlatforms,
		TitleIdea:       input.Title,
		Status:          "export_test",
		RenderStatus:    result.Status,
		Provider:        provider,
		FallbackReason:  fallbackReason,
		HasVideo:        hasVideo,
		HasThumbnail:    hasThumbnail,
		VideoFormat:     result.VideoFormat,
		VideoCodec:      result.VideoCodec,
		AudioCodec:      result.AudioCodec,
		ThumbnailFormat: result.ThumbnailFormat,
	}
	if result.VideoWidth != 0 {
		metadata.VideoWidth = result.VideoWidth
	}
	if result.VideoHeight != 0 {
		metadata.VideoHeight = result.VideoHeight
	}
	if result.VideoDurationSeconds != nil {
		metadata.VideoDurationSeconds = *result.VideoDurationSeconds
	}
	if result.ThumbnailWidth != 0 {
		metadata.ThumbnailWidth = result.ThumbnailWidth
	}
	if result.ThumbnailHeight != 0 {
		metadata.ThumbnailHeight = result.ThumbnailHeight
	}

	content := storage.ReelExportContent{
		Rank:             input.Rank,
		Title:            input.Title,
		Description:      req.Caption,
		Hashtags:         "#trendcortex #reels",
		Script:           input.Script,
		ThumbnailBrief:   input.ThumbnailBrief,
		Caption:          req.Caption,
		PlatformCaptions: platformCaptions(req.Caption, req.TargetPlatforms),
		Metadata:         metadata,
	}
	if hasVideo {
		content.VideoSrcPath = result.VideoPath
	}
	if hasThumbnail {
		content.ThumbnailSrcPath = result.ThumbnailPath
	}
	return content
}

func platformCaptions(caption string, platforms []string) map[string]string {
	out := map[string]string{}
	for _, p := range platforms {
		switch p {
		case "youtube":
			out[p] = caption + "\n\n#Shorts"
		case "tiktok":
			out[p] = caption + "\n\n#fyp #trendcortex"
		case "instagram":
			out[p] = caption + "\n\n#reels #trendcortex"
		case "facebook":
			out[p] = caption
		case "x":
			out[p] = caption
		}
	}
	return out
}

// GET /api/reels/export-test/download/{filename}
func (s *Server) handleDownloadRenderExportTest(w http.ResponseWriter, r *http.Request) {
	filename := filepath.Base(r.PathValue("filename"))
	if filename == "." || filename == "/" || !strings.HasSuffix(filename, ".zip") {
		jsonError(w, "invalid export-test ZIP filename", http.StatusBadRequest)
		return
	}
	root := filepath.Join(s.cfg.ExportDir, "export-test", "export-test")
	if s.db != nil {
		if workspaceID, err := s.defaultWorkspaceID(r.Context()); err == nil {
			root = filepath.Join(s.cfg.ExportDir, workspaceID, "export-test")
		}
	}
	zipPath := filepath.Join(root, filename)
	if !fileExists(zipPath) {
		jsonError(w, "export-test ZIP file is missing on disk — re-run the export test", http.StatusNotFound)
		return
	}
	info, err := os.Stat(zipPath)
	if err != nil || info.IsDir() {
		jsonError(w, "export-test ZIP file is unavailable", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	http.ServeFile(w, r, zipPath)
}
