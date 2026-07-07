package http

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"trendcortex/api/internal/content"
	"trendcortex/api/internal/models"
	"trendcortex/api/internal/renderer"
	"trendcortex/api/internal/storage"
	trenddiscovery "trendcortex/api/internal/trends"
)

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
	allRendered := true

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

		renderResult := renderer.RenderReel(ctx, renderer.Config{
			Provider:     s.cfg.RenderProvider,
			OutputDir:    s.cfg.MediaOutputDir,
			OpenAIAPIKey: s.cfg.OpenAIAPIKey,
			TTSModel:     s.cfg.OpenAITTSModel,
			ImageModel:   s.cfg.OpenAIImageModel,
			FFmpegPath:   s.cfg.FFmpegPath,
			FFprobePath:  s.cfg.FFprobePath,
		}, renderer.ReelInput{
			WorkspaceID:    workspaceID,
			ReelPlanID:     fmt.Sprintf("daily-package-%s-reel-%02d-%s", date, i+1, candidate.ID),
			Rank:           i + 1,
			Title:          pkg.Title,
			Script:         pkg.Script,
			Description:    firstNonEmptyString(pkg.YouTubeDescription, pkg.Caption),
			Hashtags:       strings.Join(pkg.Hashtags, " "),
			ThumbnailBrief: pkg.ThumbnailBrief,
		})
		hasVideo := renderResult.Status == renderer.StatusCompleted && fileExists(renderResult.VideoPath)
		hasThumbnail := renderResult.Status == renderer.StatusCompleted && fileExists(renderResult.ThumbnailPath)
		if !hasVideo {
			allRendered = false
		}

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
				RenderStatus:    renderResult.Status,
				RenderNotes:     renderResult.Notes,
				HasVideo:        hasVideo,
				HasThumbnail:    hasThumbnail,
				PlatformTargets: pkg.ProviderMetadata.PlatformTargets,
				GeneratedAt:     now,
				Provider:        pkg.ProviderMetadata.Provider,
				ProviderModel:   pkg.ProviderMetadata.Model,
				VideoFormat:     renderResult.VideoFormat,
				VideoWidth:      renderResult.VideoWidth,
				VideoHeight:     renderResult.VideoHeight,
				ThumbnailFormat: renderResult.ThumbnailFormat,
				ThumbnailWidth:  renderResult.ThumbnailWidth,
				ThumbnailHeight: renderResult.ThumbnailHeight,
			},
		}
		if hasVideo {
			reel.VideoSrcPath = renderResult.VideoPath
		}
		if hasThumbnail {
			reel.ThumbnailSrcPath = renderResult.ThumbnailPath
		}
		reels = append(reels, reel)
		manifestReels = append(manifestReels, storage.DailyPackageManifestReel{
			Rank:         i + 1,
			CandidateID:  candidate.ID,
			Title:        pkg.Title,
			Source:       candidate.Source,
			RenderStatus: renderResult.Status,
			RenderNotes:  renderResult.Notes,
			HasVideo:     hasVideo,
		})
	}

	status := "ready_with_render_failures"
	message := "Daily package ready with text/evidence assets; one or more reels did not render video.mp4."
	if allRendered {
		status = "ready"
		message = "Daily package ready with rendered video.mp4 for all 6 reels."
	}

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
