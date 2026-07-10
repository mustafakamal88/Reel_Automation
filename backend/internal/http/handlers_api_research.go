package http

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"trendcortex/api/internal/content"
	"trendcortex/api/internal/models"
	"trendcortex/api/internal/research"
	trenddiscovery "trendcortex/api/internal/trends"
)

type youtubeVideoAnalysisRequest struct {
	VideoURL string `json:"video_url"`
}

type youtubeChannelAnalysisRequest struct {
	ChannelURL string `json:"channel_url"`
}

func (s *Server) handleAnalyzeYouTubeVideo(w http.ResponseWriter, r *http.Request) {
	var req youtubeVideoAnalysisRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	req.VideoURL = strings.TrimSpace(req.VideoURL)
	if req.VideoURL == "" {
		jsonError(w, "video_url is required", http.StatusBadRequest)
		return
	}
	provider := research.NewYouTubeProvider(s.cfg.YouTubeAPIKey, nil)
	result, err := provider.AnalyzeVideo(r.Context(), req.VideoURL)
	if err != nil && err != research.ErrNotConfigured {
		jsonOK(w, result)
		return
	}
	jsonOK(w, result)
}

func (s *Server) handleAnalyzeYouTubeChannel(w http.ResponseWriter, r *http.Request) {
	var req youtubeChannelAnalysisRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	req.ChannelURL = strings.TrimSpace(req.ChannelURL)
	if req.ChannelURL == "" {
		jsonError(w, "channel_url is required", http.StatusBadRequest)
		return
	}
	provider := research.NewYouTubeProvider(s.cfg.YouTubeAPIKey, nil)
	result, err := provider.AnalyzeChannel(r.Context(), req.ChannelURL)
	if err != nil && err != research.ErrNotConfigured {
		jsonOK(w, result)
		return
	}
	jsonOK(w, result)
}

func (s *Server) handleResearchProviderStatus(w http.ResponseWriter, r *http.Request) {
	googleStatus := research.ProviderStatus{
		ID:       "google_trends_rss",
		Name:     "Google Trends RSS",
		Platform: "google_trends",
		Status:   research.StatusNotConfigured,
		Message:  "Set TREND_DISCOVERY_PROVIDER=google_trends_rss on the backend to enable Google Trends RSS discovery.",
		Limitations: []string{
			"RSS trend data does not provide exact ranking keywords or platform-specific creator performance.",
		},
	}
	if strings.EqualFold(strings.TrimSpace(s.cfg.TrendDiscoveryProvider), trenddiscovery.ProviderGoogleTrendsRSS) {
		googleStatus.Status = research.StatusActive
		googleStatus.Message = "Configured. Trend Finder can request live Google Trends RSS candidates."
	}

	youtube := research.NewYouTubeProvider(s.cfg.YouTubeAPIKey, nil)
	statuses := []research.ProviderStatus{
		googleStatus,
		youtube.Status(),
		research.TikTokResearchStatus(s.cfg.TikTokResearchClientID, s.cfg.TikTokResearchSecret, s.cfg.TikTokResearchToken),
		research.MetaInstagramStatus(s.cfg.MetaAppID, s.cfg.MetaAppSecret),
		research.XStatus(s.cfg.XClientID, s.cfg.XClientSecret),
		research.FacebookStatus(s.cfg.MetaAppID, s.cfg.MetaAppSecret),
	}
	jsonOK(w, map[string]any{"providers": statuses})
}

func (s *Server) handleGenerateResearchScript(w http.ResponseWriter, r *http.Request) {
	var req models.ResearchScriptGenerationRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	genReq, candidate, inferredKeywords, inferredNiche, inferredAngle, err := researchScriptGenerateRequest(req)
	if err != nil {
		jsonErrorCode(w, "validation_error", err.Error(), http.StatusBadRequest)
		return
	}

	generator := s.content
	if generator == nil {
		generator = content.OpenAIGenerator{
			APIKey: s.cfg.OpenAIAPIKey,
			Model:  s.cfg.OpenAITextModel,
		}
	}

	pkg, err := generator.Generate(r.Context(), genReq)
	if errors.Is(err, content.ErrProviderNotConfigured) {
		jsonErrorCode(w, "not_configured", err.Error(), http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}

	enrichResearchScriptPackage(&pkg, candidate, inferredKeywords, inferredNiche, inferredAngle)
	jsonOK(w, models.ResearchScriptGenerationResponse{Package: pkg})
}

func researchScriptGenerateRequest(req models.ResearchScriptGenerationRequest) (content.GenerateRequest, models.TrendCandidate, []string, string, string, error) {
	sourceType := normalizeResearchSourceType(req.SourceType)
	if sourceType == "" {
		return content.GenerateRequest{}, models.TrendCandidate{}, nil, "", "", errors.New("source_type must be one of google_trend, youtube_video_analysis, youtube_channel_analysis, niche_idea")
	}

	title := firstResearchText(req.Title, req.Topic, req.SuggestedAngle)
	keywords := cleanStrings(req.Keywords)
	if len(keywords) == 0 && strings.TrimSpace(req.Topic) != "" {
		keywords = []string{strings.TrimSpace(req.Topic)}
	}
	summary := strings.TrimSpace(req.Summary)
	if title == "" && len(keywords) > 0 {
		title = keywords[0]
	}
	if title == "" || (summary == "" && len(keywords) == 0 && strings.TrimSpace(req.InferredAngle) == "" && strings.TrimSpace(req.SuggestedAngle) == "") {
		return content.GenerateRequest{}, models.TrendCandidate{}, nil, "", "", errors.New("analysis needs a title or topic plus summary, keywords, inferred_angle, or suggested_angle")
	}

	region := strings.TrimSpace(req.Region)
	if region == "" {
		region = "US"
	}
	language := strings.TrimSpace(req.Language)
	if language == "" {
		language = "en-US"
	}
	duration := "30s"
	if req.DurationSeconds > 0 {
		duration = fmt.Sprintf("%ds", req.DurationSeconds)
	}
	platforms := req.TargetPlatforms
	if len(platforms) == 0 {
		platforms = []string{"instagram", "tiktok", "youtube", "facebook", "x"}
	}

	candidate := models.TrendCandidate{
		ID:           researchSourceID(sourceType, req.SourceID, req.SourceURL, title),
		Source:       sourceType,
		Region:       region,
		Language:     language,
		Keyword:      firstResearchText(req.Topic, firstString(keywords), title),
		Title:        title,
		Score:        researchScore(req),
		DiscoveredAt: time.Now().UTC(),
		SourceURL:    strings.TrimSpace(req.SourceURL),
		Evidence:     researchEvidence(req, sourceType),
		Status:       models.TrendCandidateStatusDiscovered,
	}

	genReq, err := content.ValidateRequest(models.ReelContentGenerationRequest{
		TrendCandidate:  &candidate,
		PlatformTargets: platforms,
		DurationTarget:  duration,
		ToneStyle:       strings.TrimSpace(req.ContentStyle),
		Language:        language,
		Region:          region,
	})
	if err != nil {
		return content.GenerateRequest{}, models.TrendCandidate{}, nil, "", "", err
	}
	return genReq, candidate, keywords, strings.TrimSpace(req.InferredNiche), firstResearchText(req.InferredAngle, req.SuggestedAngle), nil
}

func enrichResearchScriptPackage(pkg *models.ReelContentPackage, candidate models.TrendCandidate, keywords []string, niche, angle string) {
	if pkg.Description == "" {
		pkg.Description = pkg.YouTubeDescription
	}
	if pkg.PlatformPosts == nil {
		pkg.PlatformPosts = map[string]string{}
	}
	if pkg.InstagramCaption != "" {
		pkg.PlatformPosts["instagram"] = pkg.InstagramCaption
	}
	if pkg.TikTokCaption != "" {
		pkg.PlatformPosts["tiktok"] = pkg.TikTokCaption
	}
	if pkg.YouTubeDescription != "" {
		pkg.PlatformPosts["youtube"] = pkg.YouTubeDescription
	}
	if pkg.FacebookCaption != "" {
		pkg.PlatformPosts["facebook"] = pkg.FacebookCaption
	}
	if pkg.XCaption != "" {
		pkg.PlatformPosts["x"] = pkg.XCaption
	}
	pkg.GroundingEvidence = firstResearchText(pkg.GroundingEvidence, candidate.Evidence)
	pkg.SourceType = firstResearchText(pkg.SourceType, candidate.Source)
	pkg.SourceURL = firstResearchText(pkg.SourceURL, candidate.SourceURL)
	pkg.CreatedAt = firstResearchText(pkg.CreatedAt, time.Now().UTC().Format(time.RFC3339))
	pkg.InferredKeywords = keywords
	pkg.InferredNiche = niche
	pkg.InferredAngle = angle
	if pkg.ProviderMetadata.Source == "" {
		pkg.ProviderMetadata.Source = candidate.Source
	}
	if pkg.ProviderMetadata.SourceCandidateID == "" {
		pkg.ProviderMetadata.SourceCandidateID = candidate.ID
	}
	if pkg.ProviderMetadata.SourceURL == "" {
		pkg.ProviderMetadata.SourceURL = candidate.SourceURL
	}
	if pkg.ProviderMetadata.GeneratedAt == "" {
		pkg.ProviderMetadata.GeneratedAt = pkg.CreatedAt
	}
}

func normalizeResearchSourceType(sourceType string) string {
	switch strings.TrimSpace(strings.ToLower(sourceType)) {
	case "google_trend", "google_trends_rss":
		return "google_trend"
	case "youtube_video_analysis":
		return "youtube_video_analysis"
	case "youtube_channel_analysis":
		return "youtube_channel_analysis"
	case "niche_idea":
		return "niche_idea"
	default:
		return ""
	}
}

func researchSourceID(sourceType, sourceID, sourceURL, title string) string {
	sourceID = strings.TrimSpace(sourceID)
	if sourceID != "" {
		return sourceType + "-" + sourceID
	}
	h := sha1.Sum([]byte(sourceType + "|" + sourceURL + "|" + title))
	return sourceType + "-" + hex.EncodeToString(h[:])[:12]
}

func researchEvidence(req models.ResearchScriptGenerationRequest, sourceType string) string {
	payload := map[string]any{
		"source_type":         sourceType,
		"source_id":           strings.TrimSpace(req.SourceID),
		"source_url":          strings.TrimSpace(req.SourceURL),
		"topic":               strings.TrimSpace(req.Topic),
		"title":               strings.TrimSpace(req.Title),
		"summary":             strings.TrimSpace(req.Summary),
		"keywords":            cleanStrings(req.Keywords),
		"inferred_niche":      strings.TrimSpace(req.InferredNiche),
		"inferred_angle":      strings.TrimSpace(req.InferredAngle),
		"suggested_angle":     strings.TrimSpace(req.SuggestedAngle),
		"performance_signals": req.PerformanceSignals,
		"evidence":            req.Evidence,
		"metadata":            req.Metadata,
		"limitations":         researchLimitations(req.Limitations, sourceType),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "Research analysis evidence could not be serialized."
	}
	return string(b)
}

func researchLimitations(limitations []string, sourceType string) []string {
	out := cleanStrings(limitations)
	if sourceType == "youtube_video_analysis" || sourceType == "youtube_channel_analysis" {
		out = append(out, "YouTube keywords are inferred from public metadata, not exact search ranking terms.")
	}
	return uniqueStrings(out)
}

func researchScore(req models.ResearchScriptGenerationRequest) float64 {
	if v, ok := req.Metadata["score"].(float64); ok && v > 0 {
		return v
	}
	if v, ok := req.PerformanceSignals["score"].(float64); ok && v > 0 {
		return v
	}
	return 60
}

func cleanStrings(values []string) []string {
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return uniqueStrings(out)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func firstResearchText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
