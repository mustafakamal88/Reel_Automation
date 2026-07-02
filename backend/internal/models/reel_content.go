package models

type ReelContentGenerationRequest struct {
	TrendCandidateID string          `json:"trend_candidate_id,omitempty"`
	TrendCandidate   *TrendCandidate `json:"trend_candidate,omitempty"`
	PlatformTargets  []string        `json:"platform_targets"`
	DurationTarget   string          `json:"duration_target"`
	ToneStyle        string          `json:"tone_style,omitempty"`
	Language         string          `json:"language"`
	Region           string          `json:"region"`
}

type ReelContentGenerationResponse struct {
	Package ReelContentPackage `json:"package"`
}

type ReelContentPackage struct {
	Title              string               `json:"title"`
	Hook               string               `json:"hook"`
	Script             string               `json:"script"`
	Caption            string               `json:"caption"`
	Hashtags           []string             `json:"hashtags"`
	ThumbnailBrief     string               `json:"thumbnail_brief"`
	InstagramCaption   string               `json:"instagram_caption"`
	TikTokCaption      string               `json:"tiktok_caption"`
	YouTubeTitle       string               `json:"youtube_title"`
	YouTubeDescription string               `json:"youtube_description"`
	FacebookCaption    string               `json:"facebook_caption"`
	XCaption           string               `json:"x_caption"`
	SafetyGrounding    []string             `json:"safety_grounding_notes"`
	ProviderMetadata   ReelProviderMetadata `json:"provider_metadata"`
}

type ReelProviderMetadata struct {
	Provider          string   `json:"provider"`
	Model             string   `json:"model"`
	SourceCandidateID string   `json:"source_candidate_id"`
	Source            string   `json:"source"`
	SourceURL         string   `json:"source_url,omitempty"`
	PlatformTargets   []string `json:"platform_targets"`
	DurationTarget    string   `json:"duration_target"`
	GeneratedAt       string   `json:"generated_at"`
}
