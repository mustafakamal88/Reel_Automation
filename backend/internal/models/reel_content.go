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

type ResearchScriptGenerationRequest struct {
	SourceType         string         `json:"source_type"`
	SourceID           string         `json:"source_id,omitempty"`
	SourceURL          string         `json:"source_url,omitempty"`
	Topic              string         `json:"topic,omitempty"`
	Title              string         `json:"title,omitempty"`
	Summary            string         `json:"summary,omitempty"`
	Keywords           []string       `json:"keywords,omitempty"`
	InferredNiche      string         `json:"inferred_niche,omitempty"`
	InferredAngle      string         `json:"inferred_angle,omitempty"`
	PerformanceSignals map[string]any `json:"performance_signals,omitempty"`
	SuggestedAngle     string         `json:"suggested_angle,omitempty"`
	TargetPlatforms    []string       `json:"target_platforms,omitempty"`
	ContentStyle       string         `json:"content_style,omitempty"`
	DurationSeconds    int            `json:"duration_seconds,omitempty"`
	Evidence           map[string]any `json:"evidence,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	Limitations        []string       `json:"limitations,omitempty"`
	Language           string         `json:"language,omitempty"`
	Region             string         `json:"region,omitempty"`
}

type ResearchScriptGenerationResponse struct {
	Package ReelContentPackage `json:"package"`
}

type ReelContentPackage struct {
	Title              string               `json:"title"`
	Hook               string               `json:"hook"`
	Script             string               `json:"script"`
	Caption            string               `json:"caption"`
	Description        string               `json:"description,omitempty"`
	Hashtags           []string             `json:"hashtags"`
	PlatformPosts      map[string]string    `json:"platform_posts,omitempty"`
	ThumbnailBrief     string               `json:"thumbnail_brief"`
	InstagramCaption   string               `json:"instagram_caption"`
	TikTokCaption      string               `json:"tiktok_caption"`
	YouTubeTitle       string               `json:"youtube_title"`
	YouTubeDescription string               `json:"youtube_description"`
	FacebookCaption    string               `json:"facebook_caption"`
	XCaption           string               `json:"x_caption"`
	SafetyGrounding    []string             `json:"safety_grounding_notes"`
	GroundingEvidence  string               `json:"grounding,omitempty"`
	SourceType         string               `json:"source_type,omitempty"`
	SourceURL          string               `json:"source_url,omitempty"`
	CreatedAt          string               `json:"created_at,omitempty"`
	InferredKeywords   []string             `json:"inferred_keywords,omitempty"`
	InferredNiche      string               `json:"inferred_niche,omitempty"`
	InferredAngle      string               `json:"inferred_angle,omitempty"`
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
