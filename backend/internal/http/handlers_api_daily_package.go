package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"trendcortex/api/internal/content"
	"trendcortex/api/internal/models"
	"trendcortex/api/internal/renderer"
	"trendcortex/api/internal/storage"
	trenddiscovery "trendcortex/api/internal/trends"
)

type dailyPackageRenderJob struct {
	ID              string                `json:"id"`
	ReelID          string                `json:"reel_id"`
	Status          string                `json:"status"`
	RenderStatus    string                `json:"render_status"`
	RenderError     string                `json:"render_error,omitempty"`
	Message         string                `json:"message"`
	StartedAt       string                `json:"started_at"`
	CompletedAt     string                `json:"completed_at,omitempty"`
	PackageResponse *dailyPackageResponse `json:"package,omitempty"`
}

type createDailyPackageRequest struct {
	Date            string   `json:"date"`
	Region          string   `json:"region"`
	Language        string   `json:"language"`
	PlatformTargets []string `json:"platform_targets"`
	DurationTarget  string   `json:"duration_target"`
	ToneStyle       string   `json:"tone_style"`
}

type dailyPackageResponse struct {
	Status        string                             `json:"status"`
	Message       string                             `json:"message"`
	Date          string                             `json:"date"`
	ZipFilename   string                             `json:"zip_filename"`
	DownloadURL   string                             `json:"download_url"`
	IncludedFiles []string                           `json:"included_files"`
	Reels         []storage.DailyPackageManifestReel `json:"reels"`
}

// POST /api/daily-package
//
// Builds today's real six-reel package from the configured live trend
// discovery provider. It does not use seeded trends, demo fallbacks, or local
// test videos. If rendering is unavailable, the ZIP still contains the real
// OpenAI-generated text package and trend evidence with an honest render
// status, but no video.mp4 entry for that reel.
func (s *Server) handleCreateDailyPackage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var req createDailyPackageRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	date := strings.TrimSpace(req.Date)
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	region := strings.TrimSpace(req.Region)
	if region == "" {
		region = "US"
	}
	language := strings.TrimSpace(req.Language)
	if language == "" {
		language = "en-US"
	}
	platforms := req.PlatformTargets
	if len(platforms) == 0 {
		platforms = []string{"instagram", "tiktok", "youtube", "facebook", "x"}
	}
	duration := strings.TrimSpace(req.DurationTarget)
	if duration == "" {
		duration = "30s"
	}

	candidates, err := s.selectSixRealTrendCandidates(ctx, region, language)
	if errors.Is(err, trenddiscovery.ErrProviderNotConfigured) {
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	if len(candidates) < 6 {
		jsonError(w, fmt.Sprintf("real trend provider returned %d candidate(s); need 6 to build the daily package", len(candidates)), http.StatusConflict)
		return
	}

	generator := s.content
	if generator == nil {
		generator = content.OpenAIGenerator{
			APIKey: s.cfg.OpenAIAPIKey,
			Model:  s.cfg.OpenAITextModel,
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	reels := make([]storage.DailyPackageReelContent, 0, 6)
	manifestReels := make([]storage.DailyPackageManifestReel, 0, 6)

	for i, candidate := range candidates[:6] {
		genReq, err := content.ValidateRequest(models.ReelContentGenerationRequest{
			TrendCandidate:  &candidate,
			PlatformTargets: platforms,
			DurationTarget:  duration,
			ToneStyle:       req.ToneStyle,
			Language:        language,
			Region:          region,
		})
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		pkg, err := generator.Generate(ctx, genReq)
		if errors.Is(err, content.ErrProviderNotConfigured) {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		if err != nil {
			jsonError(w, fmt.Sprintf("OpenAI generation failed for reel %02d (%s): %v", i+1, candidate.ID, err), http.StatusBadGateway)
			return
		}

		renderStatus := "not_attempted"
		renderNotes := "Daily package builder generated real text/evidence assets only. Video rendering is intentionally deferred to the renderer quality phase; no fake video was created."

		posts := dailyPackagePlatformPosts(pkg)
		description := firstNonEmptyString(pkg.YouTubeDescription, pkg.Caption)
		reel := storage.DailyPackageReelContent{
			Rank:           i + 1,
			Script:         pkg.Script,
			Caption:        pkg.Caption,
			Description:    description,
			Hashtags:       strings.Join(pkg.Hashtags, "\n"),
			ThumbnailBrief: pkg.ThumbnailBrief,
			PlatformPosts:  posts,
			TrendEvidence: storage.DailyPackageTrendEvidence{
				Candidate: candidate,
			},
			Metadata: storage.DailyPackageReelMetadata{
				Rank:            i + 1,
				CandidateID:     candidate.ID,
				Source:          candidate.Source,
				SourceURL:       candidate.SourceURL,
				RenderStatus:    renderStatus,
				RenderNotes:     renderNotes,
				HasVideo:        false,
				HasThumbnail:    false,
				PlatformTargets: pkg.ProviderMetadata.PlatformTargets,
				GeneratedAt:     now,
				Provider:        pkg.ProviderMetadata.Provider,
				ProviderModel:   pkg.ProviderMetadata.Model,
			},
		}
		reels = append(reels, reel)
		manifestReels = append(manifestReels, storage.DailyPackageManifestReel{
			Rank:         i + 1,
			CandidateID:  candidate.ID,
			Title:        pkg.Title,
			Source:       candidate.Source,
			RenderStatus: renderStatus,
			RenderNotes:  renderNotes,
			HasVideo:     false,
		})
	}

	status := "ready_with_render_failures"
	message := "Daily package ready with real text/evidence assets. Video rendering was not attempted, so no video.mp4 files were included."

	manifest := storage.DailyPackageManifest{
		Date:        date,
		GeneratedAt: now,
		Status:      status,
		Message:     message,
		ReelCount:   len(reels),
		Reels:       manifestReels,
	}
	exportDir := filepath.Join(s.cfg.ExportDir, workspaceID, "daily-package")
	_, included, err := storage.BuildDailyReelsPackageZip(exportDir, reels, manifest)
	if err != nil {
		jsonError(w, "daily package ZIP build failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, dailyPackageResponse{
		Status:        status,
		Message:       message,
		Date:          date,
		ZipFilename:   storage.DailyReelsPackageFilename,
		DownloadURL:   "/api/daily-package/download",
		IncludedFiles: included,
		Reels:         manifestReels,
	})
}

func (s *Server) selectSixRealTrendCandidates(ctx context.Context, region, language string) ([]models.TrendCandidate, error) {
	timeout, err := time.ParseDuration(s.cfg.TrendDiscoveryTimeout)
	if err != nil {
		timeout = 10 * time.Second
	}
	discover := s.discover
	if discover == nil {
		discoverer := trenddiscovery.NewDiscoverer(trenddiscovery.Config{
			Provider: s.cfg.TrendDiscoveryProvider,
			BaseURL:  s.cfg.TrendDiscoveryBaseURL,
			Timeout:  timeout,
		}, nil)
		discover = discoverer.Discover
	}
	result, err := discover(ctx, region, language, 100)
	if err != nil {
		return nil, err
	}
	if result.ProviderStatus != trenddiscovery.ProviderStatusOK {
		if result.Message != "" {
			return nil, errors.New(result.Message)
		}
		return nil, fmt.Errorf("trend provider status is %s", result.ProviderStatus)
	}
	candidates := append([]models.TrendCandidate{}, result.Candidates...)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].DiscoveredAt.After(candidates[j].DiscoveredAt)
		}
		return candidates[i].Score > candidates[j].Score
	})
	return candidates, nil
}

func dailyPackagePlatformPosts(pkg models.ReelContentPackage) storage.DailyPackagePlatformPosts {
	var posts storage.DailyPackagePlatformPosts
	posts.Instagram = firstNonEmptyString(pkg.InstagramCaption, pkg.Caption)
	posts.TikTok = firstNonEmptyString(pkg.TikTokCaption, pkg.Caption)
	posts.YouTube.Title = firstNonEmptyString(pkg.YouTubeTitle, pkg.Title)
	posts.YouTube.Description = firstNonEmptyString(pkg.YouTubeDescription, pkg.Caption)
	posts.Facebook = firstNonEmptyString(pkg.FacebookCaption, pkg.Caption)
	posts.X = firstNonEmptyString(pkg.XCaption, pkg.Caption)
	return posts
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// POST /api/daily-package/reels/{id}/render
func (s *Server) handleRenderDailyPackageReel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	reelID := strings.TrimSpace(r.PathValue("id"))
	rank, err := dailyPackageRankFromID(reelID)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if rank != 1 {
		jsonError(w, "single-reel renderer is currently enabled for reel-01 only", http.StatusBadRequest)
		return
	}

	exportDir := filepath.Join(s.cfg.ExportDir, workspaceID, "daily-package")
	zipPath := filepath.Join(exportDir, storage.DailyReelsPackageFilename)
	if !fileExists(zipPath) {
		jsonError(w, "daily package ZIP is not available yet — generate today's 6 first", http.StatusNotFound)
		return
	}

	job := dailyPackageRenderJob{
		ID:           newDailyPackageRenderJobID(),
		ReelID:       fmt.Sprintf("reel-%02d", rank),
		Status:       "rendering",
		RenderStatus: "not_attempted",
		Message:      "Render started for reel-01.",
		StartedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	s.setDailyPackageRenderJob(job)

	go s.runDailyPackageReelRender(workspaceID, exportDir, zipPath, rank, job.ID)

	w.WriteHeader(http.StatusAccepted)
	jsonOK(w, job)
}

// GET /api/daily-package/render-jobs/{id}
func (s *Server) handleGetDailyPackageRenderJob(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	s.dailyRenderMu.Lock()
	job, ok := s.dailyRenderJobs[id]
	s.dailyRenderMu.Unlock()
	if !ok {
		jsonError(w, "daily package render job not found", http.StatusNotFound)
		return
	}
	jsonOK(w, job)
}

func (s *Server) runDailyPackageReelRender(workspaceID, exportDir, zipPath string, rank int, jobID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	reels, manifest, err := storage.ReadDailyReelsPackageZip(zipPath)
	if err != nil {
		s.finishDailyPackageRenderJob(jobID, "failed", "failed", "read latest daily package: "+err.Error(), nil)
		return
	}
	idx := -1
	for i := range reels {
		if reels[i].Rank == rank {
			idx = i
			break
		}
	}
	if idx == -1 {
		s.finishDailyPackageRenderJob(jobID, "failed", "failed", fmt.Sprintf("reel-%02d not found in latest daily package", rank), nil)
		return
	}

	title := dailyManifestTitle(manifest, rank)
	result := renderer.RenderSimpleTextReel(ctx, renderer.Config{
		OutputDir:   s.cfg.MediaOutputDir,
		FFmpegPath:  s.cfg.FFmpegPath,
		FFprobePath: s.cfg.FFprobePath,
	}, renderer.ReelInput{
		WorkspaceID:    workspaceID,
		ReelPlanID:     fmt.Sprintf("daily-package-reel-%02d", rank),
		Rank:           rank,
		Title:          title,
		Script:         reels[idx].Script,
		Description:    reels[idx].Description,
		Hashtags:       reels[idx].Hashtags,
		ThumbnailBrief: reels[idx].ThumbnailBrief,
	})

	now := time.Now().UTC().Format(time.RFC3339)
	if result.Status != renderer.StatusCompleted {
		msg := result.Notes
		if msg == "" {
			msg = result.Status
		}
		reels[idx].VideoSrcPath = ""
		reels[idx].ThumbnailSrcPath = ""
		reels[idx].Metadata.RenderStatus = "failed"
		reels[idx].Metadata.RenderError = msg
		reels[idx].Metadata.RenderNotes = "Single-reel render failed. No video.mp4 was included."
		reels[idx].Metadata.HasVideo = false
		reels[idx].Metadata.HasThumbnail = false
		reels[idx].Metadata.VideoFile = ""
		reels[idx].Metadata.ThumbnailFile = ""
		reels[idx].Metadata.DurationSeconds = nil
		reels[idx].Metadata.Resolution = ""
		reels[idx].Metadata.RendererVersion = renderer.SimpleRendererVersion
		reels[idx].Metadata.GeneratedAt = now
		updateDailyManifestReel(&manifest, rank, reels[idx].Metadata)
		manifest.Status = "ready_with_render_failures"
		manifest.Message = "Daily package ready with text/evidence assets. Single-reel render failed; no fake video was included."
		manifest.GeneratedAt = now
		if _, _, rebuildErr := storage.BuildDailyReelsPackageZip(exportDir, reels, manifest); rebuildErr != nil {
			msg += "; ZIP metadata rebuild failed: " + rebuildErr.Error()
		}
		s.finishDailyPackageRenderJob(jobID, "failed", "failed", msg, nil)
		return
	}

	reels[idx].VideoSrcPath = result.VideoPath
	reels[idx].ThumbnailSrcPath = result.ThumbnailPath
	reels[idx].Metadata.RenderStatus = "rendered"
	reels[idx].Metadata.RenderError = ""
	reels[idx].Metadata.RenderNotes = result.Notes
	reels[idx].Metadata.HasVideo = true
	reels[idx].Metadata.HasThumbnail = result.ThumbnailPath != ""
	reels[idx].Metadata.VideoFile = "video.mp4"
	reels[idx].Metadata.ThumbnailFile = "thumbnail.png"
	reels[idx].Metadata.VideoFormat = result.VideoFormat
	reels[idx].Metadata.VideoWidth = result.VideoWidth
	reels[idx].Metadata.VideoHeight = result.VideoHeight
	reels[idx].Metadata.DurationSeconds = result.VideoDurationSeconds
	reels[idx].Metadata.Resolution = fmt.Sprintf("%dx%d", result.VideoWidth, result.VideoHeight)
	reels[idx].Metadata.RendererVersion = result.RendererVersion
	reels[idx].Metadata.ThumbnailFormat = result.ThumbnailFormat
	reels[idx].Metadata.ThumbnailWidth = result.ThumbnailWidth
	reels[idx].Metadata.ThumbnailHeight = result.ThumbnailHeight
	reels[idx].Metadata.GeneratedAt = now
	updateDailyManifestReel(&manifest, rank, reels[idx].Metadata)
	manifest.Status = "ready_with_render_failures"
	manifest.Message = "Daily package ready. reel-01 includes rendered video.mp4 and thumbnail.png; other reels remain not_attempted."
	manifest.GeneratedAt = now

	_, included, err := storage.BuildDailyReelsPackageZip(exportDir, reels, manifest)
	if err != nil {
		s.finishDailyPackageRenderJob(jobID, "failed", "failed", "render completed but ZIP rebuild failed: "+err.Error(), nil)
		return
	}
	manifest.IncludedFiles = included
	pkg := dailyPackageResponseFromManifest(manifest)
	s.finishDailyPackageRenderJob(jobID, "completed", "rendered", "reel-01 rendered and added to the latest daily package ZIP.", &pkg)
}

func dailyPackageRankFromID(id string) (int, error) {
	id = strings.TrimSpace(strings.ToLower(id))
	if strings.HasPrefix(id, "reel-") {
		id = strings.TrimPrefix(id, "reel-")
	}
	rank, err := strconv.Atoi(id)
	if err != nil || rank <= 0 {
		return 0, errors.New("reel id must be reel-01")
	}
	return rank, nil
}

func dailyManifestTitle(manifest storage.DailyPackageManifest, rank int) string {
	for _, reel := range manifest.Reels {
		if reel.Rank == rank {
			return reel.Title
		}
	}
	return fmt.Sprintf("Reel %02d", rank)
}

func updateDailyManifestReel(manifest *storage.DailyPackageManifest, rank int, metadata storage.DailyPackageReelMetadata) {
	for i := range manifest.Reels {
		if manifest.Reels[i].Rank != rank {
			continue
		}
		manifest.Reels[i].RenderStatus = metadata.RenderStatus
		manifest.Reels[i].RenderNotes = metadata.RenderNotes
		manifest.Reels[i].RenderError = metadata.RenderError
		manifest.Reels[i].HasVideo = metadata.HasVideo
		manifest.Reels[i].VideoFile = metadata.VideoFile
		manifest.Reels[i].ThumbnailFile = metadata.ThumbnailFile
		manifest.Reels[i].DurationSeconds = metadata.DurationSeconds
		manifest.Reels[i].Resolution = metadata.Resolution
		manifest.Reels[i].RendererVersion = metadata.RendererVersion
		return
	}
}

func dailyPackageResponseFromManifest(manifest storage.DailyPackageManifest) dailyPackageResponse {
	return dailyPackageResponse{
		Status:        manifest.Status,
		Message:       manifest.Message,
		Date:          manifest.Date,
		ZipFilename:   storage.DailyReelsPackageFilename,
		DownloadURL:   "/api/daily-package/download",
		IncludedFiles: manifest.IncludedFiles,
		Reels:         manifest.Reels,
	}
}

func (s *Server) setDailyPackageRenderJob(job dailyPackageRenderJob) {
	s.dailyRenderMu.Lock()
	defer s.dailyRenderMu.Unlock()
	s.dailyRenderJobs[job.ID] = job
}

func (s *Server) finishDailyPackageRenderJob(jobID, status, renderStatus, message string, pkg *dailyPackageResponse) {
	s.dailyRenderMu.Lock()
	defer s.dailyRenderMu.Unlock()
	job := s.dailyRenderJobs[jobID]
	job.Status = status
	job.RenderStatus = renderStatus
	job.Message = message
	if status == "failed" {
		job.RenderError = message
	}
	job.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	job.PackageResponse = pkg
	s.dailyRenderJobs[jobID] = job
}

func newDailyPackageRenderJobID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("daily-render-%d", time.Now().UTC().UnixNano())
	}
	return "daily-render-" + hex.EncodeToString(b[:])
}

// GET /api/daily-package/download
func (s *Server) handleDownloadDailyPackage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	workspaceID, err := s.defaultWorkspaceID(ctx)
	if err != nil {
		jsonError(w, "workspace lookup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	zipPath := filepath.Join(s.cfg.ExportDir, workspaceID, "daily-package", storage.DailyReelsPackageFilename)
	if !fileExists(zipPath) {
		jsonError(w, "daily package ZIP is not available yet — generate today's 6 first", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", storage.DailyReelsPackageFilename))
	http.ServeFile(w, r, zipPath)
}
