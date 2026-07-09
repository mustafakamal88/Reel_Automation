package http

import (
	"net/http"
	"strings"

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
