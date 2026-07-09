package http

import (
	"net/http"
	"strings"
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
	if strings.TrimSpace(s.cfg.YouTubeAPIKey) == "" {
		jsonOK(w, map[string]any{
			"status":    "not_configured",
			"message":   "Add YouTube Data API key in Settings to analyze videos.",
			"video_url": req.VideoURL,
		})
		return
	}
	jsonOK(w, map[string]any{
		"status":    "not_configured",
		"message":   "YouTube video analysis is not implemented yet. The endpoint will use public YouTube Data API metadata only.",
		"video_url": req.VideoURL,
	})
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
	if strings.TrimSpace(s.cfg.YouTubeAPIKey) == "" {
		jsonOK(w, map[string]any{
			"status":      "not_configured",
			"message":     "Add YouTube Data API key in Settings to analyze channels.",
			"channel_url": req.ChannelURL,
		})
		return
	}
	jsonOK(w, map[string]any{
		"status":      "not_configured",
		"message":     "YouTube channel analysis is not implemented yet. The endpoint will use public YouTube Data API metadata only.",
		"channel_url": req.ChannelURL,
	})
}
