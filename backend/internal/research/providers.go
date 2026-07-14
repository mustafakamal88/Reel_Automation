package research

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	StatusActive        = "active"
	StatusNotConfigured = "not_configured"
	StatusUnavailable   = "unavailable"
	StatusOK            = "ok"
)

var ErrNotConfigured = errors.New("research provider is not configured")

const channelAnalysisSchemaVersion = "channel_analysis_v1_launch_intelligence"

const (
	ProviderErrorQuota       = "quota"
	ProviderErrorCredentials = "credentials"
	ProviderErrorTemporary   = "temporary"
	ProviderErrorTimeout     = "timeout"
	ProviderErrorMalformed   = "malformed_response"
	ProviderErrorHTTP        = "http_error"
)

type ProviderError struct {
	Code       string
	HTTPStatus int
	Reason     string
	Message    string
	Err        error
}

func (e *ProviderError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{e.Code}
	if e.HTTPStatus > 0 {
		parts = append(parts, "http_"+strconv.Itoa(e.HTTPStatus))
	}
	if e.Reason != "" {
		parts = append(parts, e.Reason)
	}
	if e.Err != nil {
		parts = append(parts, e.Err.Error())
	} else if e.Message != "" {
		parts = append(parts, e.Message)
	}
	return strings.Join(parts, ": ")
}

func (e *ProviderError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type TrendProvider interface {
	Status() ProviderStatus
}

type VideoAnalyzerProvider interface {
	Status() ProviderStatus
	AnalyzeVideo(ctx context.Context, videoURL string) (VideoAnalysisResult, error)
}

type ChannelAnalyzerProvider interface {
	Status() ProviderStatus
	AnalyzeChannel(ctx context.Context, channelURL string) (ChannelAnalysisResult, error)
}

type NicheAnalyzerProvider interface {
	Status() ProviderStatus
}

type ProviderStatus struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Platform    string   `json:"platform"`
	Status      string   `json:"status"`
	Message     string   `json:"message"`
	Scopes      []string `json:"scopes,omitempty"`
	Limitations []string `json:"limitations,omitempty"`
}

func GoogleAdsKeywordPlannerStatus(developerToken, customerID, clientID, clientSecret, refreshToken string) ProviderStatus {
	configured := strings.TrimSpace(developerToken) != "" &&
		strings.TrimSpace(customerID) != "" &&
		strings.TrimSpace(clientID) != "" &&
		strings.TrimSpace(clientSecret) != "" &&
		strings.TrimSpace(refreshToken) != ""
	if !configured {
		return ProviderStatus{
			ID:       "google_ads_keyword_planner",
			Name:     "Google Ads Keyword Planner",
			Platform: "google_ads",
			Status:   StatusNotConfigured,
			Message:  "Connect Google Ads Keyword Planner for stronger monetization estimates.",
			Scopes:   []string{"avg_monthly_searches", "competition", "top_of_page_bid"},
			Limitations: []string{
				"Google Ads API credentials are not configured, so monetization uses category and commercial-intent heuristics with low confidence.",
			},
		}
	}
	return ProviderStatus{
		ID:       "google_ads_keyword_planner",
		Name:     "Google Ads Keyword Planner",
		Platform: "google_ads",
		Status:   StatusUnavailable,
		Message:  "Credentials are present, but Keyword Planner fetching is not enabled in this build. Monetization still uses proxy heuristics.",
		Scopes:   []string{"avg_monthly_searches", "competition", "top_of_page_bid"},
		Limitations: []string{
			"No Google Ads keyword volume, CPC, or bid data is returned until the Keyword Planner provider is implemented.",
		},
	}
}

func YouTubeAnalyticsStatus() ProviderStatus {
	return ProviderStatus{
		ID:       "youtube_analytics",
		Name:     "YouTube Analytics",
		Platform: "youtube",
		Status:   StatusNotConfigured,
		Message:  "Not connected. Future owned-channel analytics can provide authorized revenue/RPM for channels you own.",
		Scopes:   []string{"owned channel revenue", "owned channel RPM", "traffic sources"},
		Limitations: []string{
			"Exact YouTube revenue/RPM is unavailable for public niches without authorized owned-channel analytics.",
		},
	}
}

type ProviderResultMetadata struct {
	SourceProvider string    `json:"source_provider"`
	SourceURL      string    `json:"source_url,omitempty"`
	EvidenceURL    string    `json:"evidence_url,omitempty"`
	Region         string    `json:"region,omitempty"`
	Language       string    `json:"language,omitempty"`
	Country        string    `json:"country,omitempty"`
	Platform       string    `json:"platform,omitempty"`
	Topic          string    `json:"topic,omitempty"`
	Keyword        string    `json:"keyword,omitempty"`
	Score          float64   `json:"score,omitempty"`
	Velocity       *float64  `json:"velocity,omitempty"`
	Volume         *int64    `json:"volume,omitempty"`
	Views          *uint64   `json:"views,omitempty"`
	Confidence     float64   `json:"confidence"`
	Limitations    []string  `json:"limitations"`
	FetchedAt      time.Time `json:"fetched_at"`
	ScoreReason    string    `json:"score_reason,omitempty"`
}

type VideoAnalysisResult struct {
	Status                     string                 `json:"status"`
	Message                    string                 `json:"message"`
	VideoURL                   string                 `json:"video_url"`
	VideoID                    string                 `json:"video_id,omitempty"`
	ThumbnailURL               string                 `json:"thumbnail_url,omitempty"`
	Title                      string                 `json:"title,omitempty"`
	ChannelTitle               string                 `json:"channel_title,omitempty"`
	ChannelID                  string                 `json:"channel_id,omitempty"`
	PublishedAt                string                 `json:"published_at,omitempty"`
	Description                string                 `json:"description,omitempty"`
	Tags                       []string               `json:"tags,omitempty"`
	Category                   string                 `json:"category,omitempty"`
	Duration                   string                 `json:"duration,omitempty"`
	Views                      *uint64                `json:"views,omitempty"`
	Likes                      *uint64                `json:"likes,omitempty"`
	Comments                   *uint64                `json:"comments,omitempty"`
	PublicTopicDetails         []string               `json:"public_topic_details,omitempty"`
	ExtractedKeywords          []string               `json:"extracted_keywords,omitempty"`
	InferredNiche              string                 `json:"inferred_niche,omitempty"`
	InferredContentAngle       string                 `json:"inferred_content_angle,omitempty"`
	HookAnalysis               string                 `json:"hook_analysis,omitempty"`
	TitleStructureAnalysis     string                 `json:"title_structure_analysis,omitempty"`
	DescriptionHashtagAnalysis string                 `json:"description_hashtag_analysis,omitempty"`
	PerformanceSignals         map[string]any         `json:"performance_signals,omitempty"`
	VideoSnapshot              map[string]any         `json:"video_snapshot,omitempty"`
	FormattedMetadata          FormattedVideoMetadata `json:"formatted_metadata,omitempty"`
	ScoreDimensions            []ScoreDimension       `json:"score_dimensions,omitempty"`
	AnalysisConfidence         ScoreDimension         `json:"analysis_confidence,omitempty"`
	PerformanceProfile         []PerformanceMetric    `json:"performance_profile,omitempty"`
	RevenueEstimate            RevenueEstimate        `json:"revenue_estimate,omitempty"`
	EvidenceBasis              []EvidenceBasis        `json:"evidence_basis,omitempty"`
	SchemaVersion              string                 `json:"schema_version,omitempty"`
	KeywordIntelligence        KeywordIntelligence    `json:"keyword_intelligence,omitempty"`
	HookIntelligence           HookIntelligence       `json:"hook_intelligence,omitempty"`
	NicheAnalysis              NicheAnalysis          `json:"niche_analysis,omitempty"`
	CreatorOpportunities       CreatorOpportunities   `json:"creator_opportunities,omitempty"`
	SuggestedRemakeAngles      []string               `json:"suggested_remake_angles,omitempty"`
	Limitations                []string               `json:"limitations"`
	Metadata                   ProviderResultMetadata `json:"metadata"`
}

type ChannelAnalysisResult struct {
	Status                  string                 `json:"status"`
	Message                 string                 `json:"message"`
	ChannelURL              string                 `json:"channel_url"`
	SchemaVersion           string                 `json:"schema_version,omitempty"`
	ChannelID               string                 `json:"channel_id,omitempty"`
	ChannelTitle            string                 `json:"channel_title,omitempty"`
	ChannelHandle           string                 `json:"channel_handle,omitempty"`
	CanonicalChannelURL     string                 `json:"canonical_channel_url,omitempty"`
	AvatarURL               string                 `json:"avatar_url,omitempty"`
	BannerURL               string                 `json:"banner_url,omitempty"`
	Description             string                 `json:"description,omitempty"`
	Subscribers             *uint64                `json:"subscribers,omitempty"`
	Views                   *uint64                `json:"views,omitempty"`
	VideoCount              *uint64                `json:"video_count,omitempty"`
	Country                 string                 `json:"country,omitempty"`
	PublicTopicDetails      []string               `json:"public_topic_details,omitempty"`
	RecentVideos            []ChannelVideoSummary  `json:"recent_videos,omitempty"`
	TopVideosSummary        []ChannelVideoSummary  `json:"top_videos_summary,omitempty"`
	ChannelSnapshot         map[string]any         `json:"channel_snapshot,omitempty"`
	ChannelNiche            string                 `json:"channel_niche,omitempty"`
	NicheAnalysis           NicheAnalysis          `json:"niche_analysis,omitempty"`
	ContentPillars          []string               `json:"content_pillars,omitempty"`
	KeywordIntelligence     KeywordIntelligence    `json:"keyword_intelligence,omitempty"`
	KeywordClusters         []KeywordCluster       `json:"keyword_clusters,omitempty"`
	FormatPatterns          []string               `json:"format_patterns,omitempty"`
	TitlePatterns           []string               `json:"title_patterns,omitempty"`
	PerformanceDistribution map[string]any         `json:"performance_distribution,omitempty"`
	UploadFrequency         string                 `json:"upload_frequency,omitempty"`
	TopVideoTopics          []string               `json:"top_video_topics,omitempty"`
	RepeatedKeywords        []string               `json:"repeated_keywords,omitempty"`
	ViewDistribution        map[string]any         `json:"view_distribution,omitempty"`
	SubscriberViewRatio     *float64               `json:"subscriber_view_ratio,omitempty"`
	LikelyStrategy          string                 `json:"likely_strategy,omitempty"`
	Opportunities           []string               `json:"opportunities,omitempty"`
	SuggestedContentIdeas   []string               `json:"suggested_content_ideas,omitempty"`
	SuggestedShortClipIdeas []string               `json:"suggested_short_clip_ideas,omitempty"`
	OpportunityScore        ScoreDimension         `json:"opportunity_score,omitempty"`
	AnalysisConfidence      ScoreDimension         `json:"analysis_confidence,omitempty"`
	ScoreDimensions         []ScoreDimension       `json:"score_dimensions,omitempty"`
	PerformanceMetrics      []PerformanceMetric    `json:"performance_metrics,omitempty"`
	RevenueEstimate         RevenueEstimate        `json:"revenue_estimate,omitempty"`
	ChannelPillars          []ChannelContentPillar `json:"channel_pillars,omitempty"`
	PerformanceCharts       ChannelCharts          `json:"performance_charts,omitempty"`
	TopVideoGroups          []ChannelVideoGroup    `json:"top_video_groups,omitempty"`
	PackagingAnalysis       ChannelPackaging       `json:"packaging_analysis,omitempty"`
	GrowthOpportunities     []ChannelOpportunity   `json:"growth_opportunities,omitempty"`
	ContentPlan             []ChannelPlanWeek      `json:"content_plan,omitempty"`
	CtaContext              ChannelCTAContext      `json:"cta_context,omitempty"`
	AnalysisDetails         ChannelAnalysisDetails `json:"analysis_details,omitempty"`
	Cache                   ChannelCacheInfo       `json:"cache,omitempty"`
	Limitations             []string               `json:"limitations"`
	Metadata                ProviderResultMetadata `json:"metadata"`
}

type ChannelVideoSummary struct {
	VideoID      string  `json:"video_id"`
	Title        string  `json:"title"`
	Description  string  `json:"description,omitempty"`
	ChannelID    string  `json:"channel_id,omitempty"`
	ChannelTitle string  `json:"channel_title,omitempty"`
	PublishedAt  string  `json:"published_at"`
	Duration     string  `json:"duration,omitempty"`
	ThumbnailURL string  `json:"thumbnail_url,omitempty"`
	Views        *uint64 `json:"views,omitempty"`
	Likes        *uint64 `json:"likes,omitempty"`
	Comments     *uint64 `json:"comments,omitempty"`
	Format       string  `json:"format,omitempty"`
	ViewsPerDay  float64 `json:"views_per_day,omitempty"`
	Pillar       string  `json:"pillar,omitempty"`
	CanonicalURL string  `json:"canonical_url,omitempty"`
}

type ChannelContentPillar struct {
	Name              string               `json:"name"`
	ShareOfUploads    float64              `json:"share_of_uploads"`
	UploadCount       int                  `json:"upload_count"`
	MedianViews       *float64             `json:"median_views,omitempty"`
	StrongestExample  *ChannelVideoSummary `json:"strongest_example,omitempty"`
	Consistency       string               `json:"consistency"`
	OpportunityStatus string               `json:"opportunity_status"`
	Recommendation    string               `json:"recommendation"`
}

type ChannelChartPoint struct {
	Label       string  `json:"label"`
	Date        string  `json:"date,omitempty"`
	Title       string  `json:"title,omitempty"`
	Views       *uint64 `json:"views,omitempty"`
	Value       float64 `json:"value,omitempty"`
	Duration    string  `json:"duration,omitempty"`
	Format      string  `json:"format,omitempty"`
	Pillar      string  `json:"pillar,omitempty"`
	VideoID     string  `json:"video_id,omitempty"`
	Description string  `json:"description,omitempty"`
}

type ChannelCharts struct {
	UploadPerformance []ChannelChartPoint `json:"upload_performance,omitempty"`
	ViewsDistribution []ChannelChartPoint `json:"views_distribution,omitempty"`
	UploadCadence     []ChannelChartPoint `json:"upload_cadence,omitempty"`
	TopicPerformance  []ChannelChartPoint `json:"topic_performance,omitempty"`
	FormatPerformance []ChannelChartPoint `json:"format_performance,omitempty"`
}

type ChannelVideoGroup struct {
	ID          string                `json:"id"`
	Label       string                `json:"label"`
	Explanation string                `json:"explanation"`
	Videos      []ChannelVideoSummary `json:"videos"`
}

type ChannelPackaging struct {
	StrongestPattern          string   `json:"strongest_pattern,omitempty"`
	WeakestHabit              string   `json:"weakest_habit,omitempty"`
	RepeatedWinningStructure  string   `json:"repeated_winning_structure,omitempty"`
	RecommendedTitleFramework string   `json:"recommended_title_framework,omitempty"`
	AverageTitleLength        float64  `json:"average_title_length,omitempty"`
	QuestionTitleShare        float64  `json:"question_title_share,omitempty"`
	NumberTitleShare          float64  `json:"number_title_share,omitempty"`
	ThumbnailAvailability     float64  `json:"thumbnail_availability,omitempty"`
	Evidence                  []string `json:"evidence,omitempty"`
}

type ChannelOpportunity struct {
	Title             string   `json:"title"`
	Why               string   `json:"why"`
	Evidence          []string `json:"evidence"`
	RecommendedFormat string   `json:"recommended_format"`
	SuggestedAudience string   `json:"suggested_audience"`
	Confidence        string   `json:"confidence"`
	SampleTitle       string   `json:"sample_title"`
	NextAction        string   `json:"next_action"`
}

type ChannelPlanIdea struct {
	WorkingTitle      string `json:"working_title"`
	ContentPillar     string `json:"content_pillar"`
	Format            string `json:"format"`
	Objective         string `json:"objective"`
	Evidence          string `json:"evidence"`
	HookDirection     string `json:"hook_direction"`
	RecommendedTiming string `json:"recommended_timing"`
}

type ChannelPlanWeek struct {
	Week      int               `json:"week"`
	Theme     string            `json:"theme"`
	Cadence   string            `json:"cadence"`
	Ideas     []ChannelPlanIdea `json:"ideas"`
	Rationale string            `json:"rationale"`
}

type ChannelCTAContext struct {
	ActionLabel             string   `json:"action_label,omitempty"`
	ChannelID               string   `json:"channel_id,omitempty"`
	ChannelTitle            string   `json:"channel_title,omitempty"`
	CanonicalURL            string   `json:"canonical_url,omitempty"`
	CleanContentPillars     []string `json:"clean_content_pillars,omitempty"`
	PerformanceEvidence     []string `json:"performance_evidence,omitempty"`
	SelectedOpportunity     string   `json:"selected_opportunity,omitempty"`
	SelectedAudience        string   `json:"selected_audience,omitempty"`
	RecommendedFormat       string   `json:"recommended_format,omitempty"`
	PublicDataLimitations   []string `json:"public_data_limitations,omitempty"`
	ScriptGenerationEnabled bool     `json:"script_generation_enabled"`
}

type ChannelAnalysisDetails struct {
	SampledVideoCount    int      `json:"sampled_video_count"`
	SampleStart          string   `json:"sample_start,omitempty"`
	SampleEnd            string   `json:"sample_end,omitempty"`
	ProviderAvailability string   `json:"provider_availability"`
	HiddenMetricNotes    []string `json:"hidden_metric_notes,omitempty"`
	ScoringMethodology   []string `json:"scoring_methodology,omitempty"`
	TopicMethodology     []string `json:"topic_methodology,omitempty"`
	RevenueMethodology   []string `json:"revenue_methodology,omitempty"`
	ClassificationRules  []string `json:"classification_rules,omitempty"`
	AnalysisTimestamp    string   `json:"analysis_timestamp"`
}

type ChannelCacheInfo struct {
	Hit           bool      `json:"hit"`
	CacheHit      bool      `json:"cache_hit"`
	SchemaVersion string    `json:"schema_version,omitempty"`
	Key           string    `json:"key,omitempty"`
	StoredAt      time.Time `json:"stored_at,omitempty"`
	TTL           string    `json:"ttl,omitempty"`
	Freshness     string    `json:"freshness,omitempty"`
}

type KeywordIntelligence struct {
	PrimaryKeywords       []string `json:"primary_keywords,omitempty"`
	SecondaryKeywords     []string `json:"secondary_keywords,omitempty"`
	LongTailPhrases       []string `json:"long_tail_phrases,omitempty"`
	PrimaryTopics         []string `json:"primary_topics,omitempty"`
	SupportingTerms       []string `json:"supporting_terms,omitempty"`
	SearchPhrases         []string `json:"search_phrases,omitempty"`
	Hashtags              []string `json:"hashtags,omitempty"`
	RejectedNoiseTerms    []string `json:"rejected_noise_terms,omitempty"`
	InferredSearchIntent  string   `json:"inferred_search_intent,omitempty"`
	MetadataStrengthScore int      `json:"metadata_strength_score,omitempty"`
}

type NicheAnalysis struct {
	BroadCategory        string   `json:"broad_category,omitempty"`
	PrimaryNiche         string   `json:"primary_niche,omitempty"`
	Niche                string   `json:"niche,omitempty"`
	SubNiche             string   `json:"sub_niche,omitempty"`
	SpecificTopic        string   `json:"specific_topic,omitempty"`
	AudienceType         string   `json:"audience_type,omitempty"`
	ContentFormat        string   `json:"content_format,omitempty"`
	Confidence           float64  `json:"confidence,omitempty"`
	EvidenceTerms        []string `json:"evidence_terms,omitempty"`
	TargetAudience       string   `json:"target_audience,omitempty"`
	InferredContentAngle string   `json:"inferred_content_angle,omitempty"`
}

type HookIntelligence struct {
	HookType             string   `json:"hook_type,omitempty"`
	TitleLength          int      `json:"title_length,omitempty"`
	TitlePattern         string   `json:"title_pattern,omitempty"`
	EmotionalTriggers    []string `json:"emotional_triggers,omitempty"`
	ClarityScore         int      `json:"clarity_score,omitempty"`
	SpecificityScore     int      `json:"specificity_score,omitempty"`
	CuriosityScore       int      `json:"curiosity_score,omitempty"`
	AudienceSignalScore  int      `json:"audience_signal_score,omitempty"`
	ValuePromiseScore    int      `json:"value_promise_score,omitempty"`
	RemakePotentialScore int      `json:"remake_potential_score,omitempty"`
	Explanation          string   `json:"explanation,omitempty"`
}

type CreatorOpportunities struct {
	SuggestedRemakeAngles []string `json:"suggested_remake_angles,omitempty"`
	TitleIdeas            []string `json:"title_ideas,omitempty"`
	ShortFormClipIdeas    []string `json:"short_form_clip_ideas,omitempty"`
	ScriptPrompts         []string `json:"script_prompts,omitempty"`
	ContentGaps           []string `json:"content_gaps,omitempty"`
	UnderusedTopics       []string `json:"underused_topics,omitempty"`
	LocalizationOptions   []string `json:"localization_options,omitempty"`
}

type KeywordCluster struct {
	Name     string   `json:"name"`
	Terms    []string `json:"terms"`
	Evidence []string `json:"evidence,omitempty"`
}

type YouTubeProvider struct {
	apiKey string
	client *http.Client
	now    func() time.Time
}

func NewYouTubeProvider(apiKey string, client *http.Client) *YouTubeProvider {
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	return &YouTubeProvider{apiKey: strings.TrimSpace(apiKey), client: client, now: func() time.Time { return time.Now().UTC() }}
}

func (p *YouTubeProvider) Status() ProviderStatus {
	if p.apiKey == "" {
		return ProviderStatus{
			ID:       "youtube_data_api",
			Name:     "YouTube Data API",
			Platform: "youtube",
			Status:   StatusNotConfigured,
			Message:  "Add YOUTUBE_API_KEY in Settings/Railway variables to enable YouTube research.",
			Scopes:   []string{"public YouTube Data API metadata"},
			Limitations: []string{
				"Public metadata only; no private analytics or exact ranking keywords.",
				"TrendCortex does not download or scrape YouTube videos.",
			},
		}
	}
	return ProviderStatus{
		ID:       "youtube_data_api",
		Name:     "YouTube Data API",
		Platform: "youtube",
		Status:   StatusActive,
		Message:  "Configured. Analysis uses official YouTube Data API public metadata.",
		Scopes:   []string{"videos.list", "channels.list", "search.list"},
		Limitations: []string{
			"Ranking keywords are inferred from public metadata.",
			"Exact search terms require authorized analytics or platform data.",
		},
	}
}

func (p *YouTubeProvider) AnalyzeVideo(ctx context.Context, videoURL string) (VideoAnalysisResult, error) {
	videoURL = strings.TrimSpace(videoURL)
	result := VideoAnalysisResult{
		Status:   StatusNotConfigured,
		Message:  "Add YOUTUBE_API_KEY in Settings/Railway variables to analyze YouTube videos.",
		VideoURL: videoURL,
		Limitations: []string{
			"Ranking keywords are inferred from public metadata. Exact search ranking terms require authorized analytics or platform data.",
			"TrendCortex does not download or scrape YouTube videos.",
		},
		Metadata: ProviderResultMetadata{
			SourceProvider: "youtube_data_api",
			Platform:       "youtube",
			Confidence:     0,
			Limitations:    []string{"Provider is not configured."},
			FetchedAt:      p.now(),
		},
	}
	if p.apiKey == "" {
		return result, ErrNotConfigured
	}
	videoID, err := ExtractYouTubeVideoID(videoURL)
	if err != nil {
		result.Status = "invalid_input"
		result.Message = err.Error()
		return result, nil
	}
	apiURL := youtubeAPIURL("videos", map[string]string{
		"part": "snippet,statistics,contentDetails,topicDetails",
		"id":   videoID,
		"key":  p.apiKey,
	})
	var res youtubeVideosResponse
	if err := p.getJSON(ctx, apiURL, &res); err != nil {
		result.Status = "provider_error"
		result.Message = "YouTube Data API request failed: " + err.Error()
		return result, err
	}
	if len(res.Items) == 0 {
		result.Status = "no_data"
		result.Message = "YouTube Data API returned no public video metadata for that video ID."
		return result, nil
	}
	item := res.Items[0]
	views := parseUintPtr(item.Statistics.ViewCount)
	likes := parseUintPtr(item.Statistics.LikeCount)
	comments := parseUintPtr(item.Statistics.CommentCount)
	keywordIntel := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title:        item.Snippet.Title,
		Description:  item.Snippet.Description,
		Tags:         item.Snippet.Tags,
		ChannelTitle: item.Snippet.ChannelTitle,
		Category:     item.Snippet.CategoryID,
		TopicDetails: item.TopicDetails.TopicCategories,
	})
	keywordIntel = p.enhanceVideoKeywords(ctx, keywordIntel, item.Snippet.Title, item.Snippet.ChannelTitle)
	keywordIntel = sanitizeKeywordIntelligenceOutput(keywordIntel, KeywordExtractionInput{
		Title:        item.Snippet.Title,
		Description:  item.Snippet.Description,
		Tags:         item.Snippet.Tags,
		ChannelTitle: item.Snippet.ChannelTitle,
		Category:     item.Snippet.CategoryID,
		TopicDetails: item.TopicDetails.TopicCategories,
	})
	keywords := append(append([]string{}, keywordIntel.PrimaryKeywords...), keywordIntel.SecondaryKeywords...)
	nicheAnalysis := ClassifyNiche(KeywordExtractionInput{
		Title:        item.Snippet.Title,
		Description:  item.Snippet.Description,
		Tags:         item.Snippet.Tags,
		ChannelTitle: item.Snippet.ChannelTitle,
		Category:     item.Snippet.CategoryID,
		TopicDetails: item.TopicDetails.TopicCategories,
	}, keywordIntel)
	hookIntel := AnalyzeHookIntelligence(item.Snippet.Title)
	opps := BuildCreatorOpportunities(nicheAnalysis, keywordIntel, hookIntel, item.Snippet.Title)
	formattedMetadata := formatVideoMetadata(item, views, likes, comments, p.now, videoURL)
	scoreDimensions, analysisConfidence := buildScoreDimensions(item, keywordIntel, hookIntel, nicheAnalysis, views, likes, comments, p.now, videoURL)
	performanceProfile := buildPerformanceProfile(item.Snippet.PublishedAt, views, likes, comments, p.now)
	revenueEstimate := estimateRevenue(item, nicheAnalysis, views, p.now, videoURL)
	niche := nicheAnalysis.PrimaryNiche
	angle := nicheAnalysis.InferredContentAngle
	signals := performanceSignals(item.Snippet.PublishedAt, views, likes, comments, p.now)
	score, reason := ScoreEvidence(ScoringInput{
		SourceConfidence: 0.92,
		Views:            views,
		PublishedAt:      item.Snippet.PublishedAt,
		KeywordCount:     len(keywords),
		HasEvidenceURL:   true,
		RegionMatch:      0.5,
		NicheMatch:       0.5,
	})
	result = VideoAnalysisResult{
		Status:                     StatusOK,
		Message:                    "Analyzed public YouTube Data API metadata. Inferences are labeled and do not include private analytics.",
		VideoURL:                   videoURL,
		VideoID:                    videoID,
		ThumbnailURL:               bestVideoThumbnail(item),
		Title:                      item.Snippet.Title,
		ChannelTitle:               item.Snippet.ChannelTitle,
		ChannelID:                  item.Snippet.ChannelID,
		PublishedAt:                item.Snippet.PublishedAt,
		Description:                cleanMetadataText(item.Snippet.Description),
		Tags:                       item.Snippet.Tags,
		Category:                   item.Snippet.CategoryID,
		Duration:                   item.ContentDetails.Duration,
		Views:                      views,
		Likes:                      likes,
		Comments:                   comments,
		PublicTopicDetails:         item.TopicDetails.TopicCategories,
		ExtractedKeywords:          keywords,
		InferredNiche:              niche,
		InferredContentAngle:       angle,
		HookAnalysis:               analyzeHook(item.Snippet.Title),
		TitleStructureAnalysis:     analyzeTitleStructure(item.Snippet.Title),
		DescriptionHashtagAnalysis: analyzeDescriptionHashtags(item.Snippet.Description),
		PerformanceSignals:         signals,
		VideoSnapshot: map[string]any{
			"title":        item.Snippet.Title,
			"channel":      item.Snippet.ChannelTitle,
			"thumbnail":    bestVideoThumbnail(item),
			"published_at": item.Snippet.PublishedAt,
			"duration":     item.ContentDetails.Duration,
			"views":        views,
			"likes":        likes,
			"comments":     comments,
		},
		FormattedMetadata:     formattedMetadata,
		ScoreDimensions:       scoreDimensions,
		AnalysisConfidence:    analysisConfidence,
		PerformanceProfile:    performanceProfile,
		RevenueEstimate:       revenueEstimate,
		EvidenceBasis:         evidenceBasis(),
		SchemaVersion:         videoAnalysisSchemaVersion,
		KeywordIntelligence:   keywordIntel,
		HookIntelligence:      hookIntel,
		NicheAnalysis:         nicheAnalysis,
		CreatorOpportunities:  opps,
		SuggestedRemakeAngles: opps.SuggestedRemakeAngles,
		Limitations: []string{
			"Public metadata only: official YouTube Data API fields are used; no scraping, downloads, private analytics, retention, revenue, or traffic sources.",
			"Keyword ranking is inferred from public metadata only; this is not an exact YouTube search ranking report.",
			"Like/comment counts can be unavailable when hidden or restricted by YouTube.",
		},
		Metadata: ProviderResultMetadata{
			SourceProvider: "youtube_data_api",
			SourceURL:      "https://www.youtube.com/watch?v=" + videoID,
			EvidenceURL:    apiURLWithoutKey(apiURL),
			Platform:       "youtube",
			Topic:          niche,
			Keyword:        firstKeyword(keywords),
			Score:          score,
			Views:          views,
			Confidence:     0.78,
			Limitations:    []string{"Public metadata only; ranking terms are inferred."},
			FetchedAt:      p.now(),
			ScoreReason:    reason,
		},
	}
	return result, nil
}

func (p *YouTubeProvider) AnalyzeChannel(ctx context.Context, channelURL string) (ChannelAnalysisResult, error) {
	channelURL = strings.TrimSpace(channelURL)
	result := ChannelAnalysisResult{
		Status:        StatusNotConfigured,
		Message:       "Add YOUTUBE_API_KEY in Settings/Railway variables to analyze YouTube channels.",
		ChannelURL:    channelURL,
		SchemaVersion: channelAnalysisSchemaVersion,
		Limitations: []string{
			"Analysis uses public metadata only. Private retention, traffic source, revenue, and exact search terms are unavailable.",
			"TrendCortex does not download or scrape YouTube videos.",
		},
		Metadata: ProviderResultMetadata{
			SourceProvider: "youtube_data_api",
			Platform:       "youtube",
			Confidence:     0,
			Limitations:    []string{"Provider is not configured."},
			FetchedAt:      p.now(),
		},
	}
	if p.apiKey == "" {
		return result, ErrNotConfigured
	}
	resolved, err := p.resolveChannelIdentity(ctx, channelURL)
	if err != nil {
		result.Status = "invalid_input"
		result.Message = err.Error()
		return result, nil
	}
	channelID := resolved.ChannelID
	channelAPIURL := youtubeAPIURL("channels", map[string]string{
		"part": "snippet,statistics,contentDetails,topicDetails,brandingSettings",
		"id":   channelID,
		"key":  p.apiKey,
	})
	var channelRes youtubeChannelsResponse
	if err := p.getJSON(ctx, channelAPIURL, &channelRes); err != nil {
		result.Status = "provider_error"
		result.Message = "YouTube Data API channel request failed: " + err.Error()
		return result, err
	}
	if len(channelRes.Items) == 0 {
		result.Status = "no_data"
		result.Message = "YouTube Data API returned no public channel metadata for that channel."
		return result, nil
	}
	channel := channelRes.Items[0]
	recentVideos, _ := p.fetchChannelVideos(ctx, channelID, "date", 25)
	topVideos, _ := p.fetchChannelVideos(ctx, channelID, "viewCount", 25)
	recentVideos = enrichChannelVideos(recentVideos, p.now)
	topVideos = enrichChannelVideos(topVideos, p.now)
	videos := mergeChannelVideos(recentVideos, topVideos)
	videos = enrichChannelVideos(videos, p.now)
	keywordIntel := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title:             channel.Snippet.Title,
		Description:       cleanMetadataText(channel.Snippet.Description),
		ChannelTitle:      channel.Snippet.Title,
		TopicDetails:      channel.TopicDetails.TopicCategories,
		RecentVideoTitles: videoTitles(videos),
	})
	keywordIntel = removeChannelIdentityKeywords(keywordIntel, channel.Snippet.Title)
	keywordIntel = p.enhanceChannelKeywords(ctx, keywordIntel, channel.Snippet.Title)
	keywordIntel = removeChannelIdentityKeywords(keywordIntel, channel.Snippet.Title)
	keywords := append(append([]string{}, keywordIntel.PrimaryKeywords...), keywordIntel.SecondaryKeywords...)
	nicheAnalysis := ClassifyNiche(KeywordExtractionInput{
		Title:             channel.Snippet.Title,
		Description:       channel.Snippet.Description,
		ChannelTitle:      channel.Snippet.Title,
		TopicDetails:      channel.TopicDetails.TopicCategories,
		RecentVideoTitles: videoTitles(videos),
	}, keywordIntel)
	pillars := topN(keywordIntel.PrimaryKeywords, 6)
	if len(pillars) == 0 {
		pillars = topN(keywords, 6)
	}
	pillarObjects := buildChannelPillars(pillars, videos)
	assignVideoPillars(videos, pillarObjects)
	assignVideoPillars(recentVideos, pillarObjects)
	assignVideoPillars(topVideos, pillarObjects)
	viewDistribution, ratio := channelSignals(channel.Statistics.SubscriberCount, videos)
	performanceDistribution := PerformanceDistribution(videos)
	patterns := titlePatterns(videos)
	formats := formatPatternsFromVideos(videos)
	channelOpps := ChannelOpportunities(nicheAnalysis, pillars, keywordIntel)
	ideas := SuggestedChannelIdeas(nicheAnalysis, keywordIntel, pillars)
	shortIdeas := SuggestedShortClipIdeas(nicheAnalysis, keywordIntel, pillars)
	dimensions, opportunityScore, analysisConfidence := buildChannelScoreModel(channel, videos, pillarObjects, keywordIntel, nicheAnalysis, p.now)
	performanceMetrics := channelPerformanceMetrics(videos, channel.Statistics.SubscriberCount, p.now)
	revenueEstimate := estimateChannelRevenue(channel, videos, nicheAnalysis)
	charts := buildChannelCharts(videos, pillarObjects)
	videoGroups := buildChannelVideoGroups(videos)
	packaging := buildChannelPackaging(videos, pillarObjects)
	growthOpps := buildChannelGrowthOpportunities(nicheAnalysis, pillarObjects, videoGroups, packaging)
	contentPlan := buildChannelContentPlan(videos, pillarObjects, growthOpps, nicheAnalysis, p.now)
	details := buildChannelAnalysisDetails(videos, channel, p.now)
	canonicalURL := "https://www.youtube.com/channel/" + channelID
	cacheKey := "youtube_channel:" + channelAnalysisSchemaVersion + ":" + channelID
	score := float64(opportunityScore.Score)
	reason := opportunityScore.Explanation
	handle := firstNonEmpty(channel.Snippet.CustomURL, resolved.Handle)
	country := firstNonEmpty(channel.Snippet.Country, channel.BrandingSettings.Channel.Country)
	result = ChannelAnalysisResult{
		Status:              StatusOK,
		Message:             "Analyzed public YouTube Data API channel metadata. Strategy notes are inferred from public channel and recent video metadata.",
		ChannelURL:          channelURL,
		SchemaVersion:       channelAnalysisSchemaVersion,
		ChannelID:           channelID,
		ChannelHandle:       handle,
		CanonicalChannelURL: canonicalURL,
		AvatarURL:           bestChannelThumbnail(channel),
		BannerURL:           channel.BrandingSettings.Image.BannerExternalURL,
		ChannelTitle:        channel.Snippet.Title,
		Description:         cleanMetadataText(channel.Snippet.Description),
		Subscribers:         parseUintPtr(channel.Statistics.SubscriberCount),
		Views:               parseUintPtr(channel.Statistics.ViewCount),
		VideoCount:          parseUintPtr(channel.Statistics.VideoCount),
		Country:             country,
		PublicTopicDetails:  channel.TopicDetails.TopicCategories,
		RecentVideos:        recentVideos,
		TopVideosSummary:    topNChannelVideos(topVideos, 10),
		ChannelSnapshot: map[string]any{
			"subscribers":           parseUintPtr(channel.Statistics.SubscriberCount),
			"total_views":           parseUintPtr(channel.Statistics.ViewCount),
			"video_count":           parseUintPtr(channel.Statistics.VideoCount),
			"country":               country,
			"channel_age":           channelAge(channel.Snippet.PublishedAt, p.now),
			"created_at":            channel.Snippet.PublishedAt,
			"last_public_upload_at": latestVideoDate(videos),
			"recent_upload_cadence": uploadFrequency(videos, p.now),
			"primary_format":        primaryChannelFormat(videos),
			"dominant_topic":        firstPhrase(pillars),
		},
		ChannelNiche:            nicheAnalysis.PrimaryNiche,
		NicheAnalysis:           nicheAnalysis,
		ContentPillars:          pillars,
		KeywordIntelligence:     keywordIntel,
		KeywordClusters:         ChannelKeywordClusters(keywordIntel, videos),
		FormatPatterns:          formats,
		TitlePatterns:           patterns,
		PerformanceDistribution: performanceDistribution,
		UploadFrequency:         uploadFrequency(videos, p.now),
		TopVideoTopics:          topN(keywords, 8),
		RepeatedKeywords:        keywords,
		ViewDistribution:        viewDistribution,
		SubscriberViewRatio:     ratio,
		LikelyStrategy:          ChannelStrategy(nicheAnalysis, pillars, patterns, performanceDistribution),
		Opportunities:           append(append(channelOpps.ContentGaps, channelOpps.UnderusedTopics...), channelOpps.LocalizationOptions...),
		SuggestedContentIdeas:   ideas,
		SuggestedShortClipIdeas: shortIdeas,
		OpportunityScore:        opportunityScore,
		AnalysisConfidence:      analysisConfidence,
		ScoreDimensions:         dimensions,
		PerformanceMetrics:      performanceMetrics,
		RevenueEstimate:         revenueEstimate,
		ChannelPillars:          pillarObjects,
		PerformanceCharts:       charts,
		TopVideoGroups:          videoGroups,
		PackagingAnalysis:       packaging,
		GrowthOpportunities:     growthOpps,
		ContentPlan:             contentPlan,
		CtaContext:              buildChannelCTAContext(channelID, channel.Snippet.Title, canonicalURL, pillarObjects, growthOpps, performanceMetrics, result.Limitations),
		AnalysisDetails:         details,
		Cache: ChannelCacheInfo{
			Hit:           false,
			CacheHit:      false,
			SchemaVersion: channelAnalysisSchemaVersion,
			Key:           cacheKey,
			StoredAt:      p.now(),
			TTL:           "not persisted",
			Freshness:     "fresh",
		},
		Limitations: []string{
			"Public metadata only: official YouTube Data API fields are used; no scraping, downloads, private analytics, retention, revenue, or traffic sources.",
			"Keyword clusters and strategy are inferred from public titles, descriptions, topics, tags where available, and visible counts; no exact search ranking keywords are claimed.",
			"Recent/top video selection is based on official API responses and available public counts.",
			"Public channel revenue is an estimate from visible views and broad RPM assumptions, not actual YouTube earnings.",
		},
		Metadata: ProviderResultMetadata{
			SourceProvider: "youtube_data_api",
			SourceURL:      canonicalURL,
			EvidenceURL:    apiURLWithoutKey(channelAPIURL),
			Country:        country,
			Platform:       "youtube",
			Topic:          nicheAnalysis.PrimaryNiche,
			Keyword:        firstKeyword(keywords),
			Score:          score,
			Views:          parseUintPtr(channel.Statistics.ViewCount),
			Confidence:     float64(analysisConfidence.Score) / 100,
			Limitations:    []string{"Strategy is inferred from public metadata only."},
			FetchedAt:      p.now(),
			ScoreReason:    reason,
		},
	}
	result.CtaContext.PublicDataLimitations = result.Limitations
	return result, nil
}

func bestChannelThumbnail(item youtubeChannelItem) string {
	if item.Snippet.Thumbnails.High.URL != "" {
		return item.Snippet.Thumbnails.High.URL
	}
	if item.Snippet.Thumbnails.Medium.URL != "" {
		return item.Snippet.Thumbnails.Medium.URL
	}
	return item.Snippet.Thumbnails.Default.URL
}

func enrichChannelVideos(videos []ChannelVideoSummary, now func() time.Time) []ChannelVideoSummary {
	out := make([]ChannelVideoSummary, len(videos))
	for i, video := range videos {
		video.Description = cleanMetadataText(video.Description)
		video.Format = classifyVideoFormat(video.Duration, "https://www.youtube.com/watch?v="+video.VideoID)
		video.CanonicalURL = "https://www.youtube.com/watch?v=" + video.VideoID
		if video.Views != nil {
			if t, err := time.Parse(time.RFC3339, video.PublishedAt); err == nil {
				days := now().Sub(t).Hours() / 24
				if days < 1 {
					days = 1
				}
				video.ViewsPerDay = float64(*video.Views) / days
			}
		}
		out[i] = video
	}
	return out
}

func assignVideoPillars(videos []ChannelVideoSummary, pillars []ChannelContentPillar) {
	for i := range videos {
		videos[i].Pillar = matchPillar(videos[i], pillars)
	}
}

func matchPillar(video ChannelVideoSummary, pillars []ChannelContentPillar) string {
	lower := strings.ToLower(video.Title + " " + video.Description)
	for _, pillar := range pillars {
		if strings.Contains(lower, strings.ToLower(pillar.Name)) {
			return pillar.Name
		}
		for _, token := range strings.Fields(strings.ToLower(pillar.Name)) {
			if len(token) > 3 && strings.Contains(lower, token) {
				return pillar.Name
			}
		}
	}
	if len(pillars) > 0 {
		return pillars[0].Name
	}
	return ""
}

func buildChannelPillars(seed []string, videos []ChannelVideoSummary) []ChannelContentPillar {
	candidates := validatePhraseList(seed, nil, nil, 6, true)
	if len(candidates) == 0 {
		return []ChannelContentPillar{}
	}
	total := len(videos)
	out := []ChannelContentPillar{}
	for _, name := range candidates {
		matches := []ChannelVideoSummary{}
		for _, video := range videos {
			if strings.Contains(strings.ToLower(video.Title+" "+video.Description), strings.ToLower(name)) || phraseOverlapsSimple(name, video.Title) {
				matches = append(matches, video)
			}
		}
		if len(matches) == 0 {
			for _, video := range videos {
				if phraseOverlapsSimple(name, video.Title) || phraseOverlapsSimple(name, video.Description) {
					matches = append(matches, video)
				}
			}
		}
		views := videoViewFloats(matches)
		median := medianFloatPtr(views)
		strongest := strongestVideo(matches, "views_per_day")
		status := "Weak evidence"
		consistency := "Limited sample"
		if len(matches) >= 4 {
			consistency = "Consistent"
		} else if len(matches) >= 2 {
			consistency = "Some repetition"
		}
		if median != nil && strongest != nil {
			switch {
			case len(matches) >= 3 && strongest.ViewsPerDay > 0:
				status = "Proven winner"
			case len(matches) >= 3:
				status = "Reliable"
			case len(matches) == 2:
				status = "Emerging"
			default:
				status = "Underused"
			}
		}
		share := 0.0
		if total > 0 {
			share = float64(len(matches)) / float64(total)
		}
		out = append(out, ChannelContentPillar{
			Name:              applyAcronymCasing(name),
			ShareOfUploads:    share,
			UploadCount:       len(matches),
			MedianViews:       median,
			StrongestExample:  strongest,
			Consistency:       consistency,
			OpportunityStatus: status,
			Recommendation:    pillarRecommendation(name, status),
		})
	}
	return out
}

func phraseOverlapsSimple(a, b string) bool {
	ta := tokenizeUseful(a, map[string]bool{})
	tb := tokenSet(tokenizeUseful(b, map[string]bool{}))
	for _, token := range ta {
		if len(token) > 3 && tb[token] {
			return true
		}
	}
	return false
}

func pillarRecommendation(name, status string) string {
	switch status {
	case "Proven winner":
		return "Turn this into a repeatable series and test a clearer follow-up title around " + name + "."
	case "Emerging":
		return "Publish one focused follow-up to check whether " + name + " repeats without relying on a single outlier."
	case "Underused":
		return "Use this as a controlled test only after proven pillars are covered."
	default:
		return "Keep the pillar, but pair it with a stronger hook and clearer audience promise."
	}
}

func buildChannelScoreModel(channel youtubeChannelItem, videos []ChannelVideoSummary, pillars []ChannelContentPillar, kw KeywordIntelligence, niche NicheAnalysis, now func() time.Time) ([]ScoreDimension, ScoreDimension, ScoreDimension) {
	metrics := channelRawMetrics(videos, channel.Statistics.SubscriberCount, now)
	sampleSize := len(videoViewFloats(videos))
	topicClarity := clampInt(35 + len(pillars)*8 + int(niche.Confidence*20))
	if len(kw.PrimaryKeywords) >= 4 {
		topicClarity += 8
	}
	packaging := scorePackaging(videos)
	consistency := scoreConsistency(metrics.uploadsPerMonth, metrics.daysSinceLastUpload)
	repeatability := scoreRepeatability(videos, pillars)
	momentum := scoreMomentum(videos)
	formatEfficiency := scoreFormatEfficiency(videos)
	monetisation := monetisationPotentialScore(niche, primaryChannelFormat(videos), parseUintPtr(channel.Statistics.ViewCount))
	evidence := 35
	if sampleSize >= 8 {
		evidence += 22
	}
	if sampleSize >= 20 {
		evidence += 15
	}
	if channel.Statistics.SubscriberCount != "" {
		evidence += 8
	}
	if visibleEngagementCount(videos) >= 6 {
		evidence += 10
	}
	evidence = clampInt(evidence)
	dims := []ScoreDimension{
		scoreDimension("topic_clarity", "Topic clarity", topicClarity, "Measures whether titles, channel description, topics, and repeated phrases point to a coherent creator lane.", firstPhrase(kw.PrimaryKeywords)),
		scoreDimension("packaging_strength", "Packaging strength", packaging, "Title structure score from specificity, numbers, questions, repeated winning forms, and avoidable repetition. Thumbnail claims are limited to URL availability.", ""),
		scoreDimension("publishing_consistency", "Publishing consistency", consistency, "Uses dated public uploads in the sampled set: uploads per month and days since last public upload.", fmt.Sprintf("%.1f uploads/month", metrics.uploadsPerMonth)),
		scoreDimension("content_repeatability", "Content repeatability", repeatability, "Rewards repeatable pillars with multiple uploads and resists one-off outlier dependence.", fmt.Sprintf("%d pillars", len(pillars))),
		scoreDimension("format_efficiency", "Format efficiency", formatEfficiency, "Compares median views per detected format only when duration classification is available.", primaryChannelFormat(videos)),
		scoreDimension("recent_momentum", "Recent momentum", momentum, "Uses recent views-per-day and share of sampled uploads above the sample median. It does not infer subscriber growth history.", ""),
		scoreDimension("monetisation_potential", "Monetisation potential", monetisation, "Estimated from public views, detected format, and broad niche RPM assumptions. It is not actual revenue.", niche.PrimaryNiche),
		scoreDimension("evidence_quality", "Evidence quality", evidence, "Caps the overall score when public evidence is incomplete or visible engagement counts are hidden.", fmt.Sprintf("%d sampled videos", sampleSize)),
	}
	weighted := float64(topicClarity)*0.14 + float64(packaging)*0.12 + float64(consistency)*0.14 + float64(repeatability)*0.14 + float64(formatEfficiency)*0.10 + float64(momentum)*0.14 + float64(monetisation)*0.10 + float64(evidence)*0.12
	if evidence < 50 && weighted > 62 {
		weighted = 62
	}
	if sampleSize < 5 && weighted > 55 {
		weighted = 55
	}
	score := scoreDimension("channel_opportunity", "Channel opportunity", clampInt(int(weighted)), "Weighted score across momentum, consistency, topic clarity, repeatability, packaging, format efficiency, monetisation potential, and evidence quality. It does not reward size alone.", "")
	conf := scoreDimension("analysis_confidence", "Analysis confidence", evidence, "Confidence reflects sampled public videos, visible statistics, channel metadata completeness, and deterministic topic agreement.", "")
	return dims, score, conf
}

type channelMetrics struct {
	avgViews            float64
	medianViews         float64
	medianDuration      float64
	uploadsPerMonth     float64
	daysSinceLastUpload float64
	aboveMedianShare    float64
	likesPer1000        *float64
	commentsPer1000     *float64
	subscriberViewRatio *float64
}

func channelRawMetrics(videos []ChannelVideoSummary, subscribersRaw string, now func() time.Time) channelMetrics {
	views := videoViewFloats(videos)
	metrics := channelMetrics{medianViews: medianFloat(views), avgViews: averageFloat(views)}
	if len(views) > 0 {
		above := 0
		for _, v := range views {
			if v >= metrics.medianViews {
				above++
			}
		}
		metrics.aboveMedianShare = float64(above) / float64(len(views))
	}
	likesRate, commentsRate := engagementRates(videos)
	metrics.likesPer1000 = likesRate
	metrics.commentsPer1000 = commentsRate
	metrics.medianDuration = medianDurationSeconds(videos)
	metrics.uploadsPerMonth, metrics.daysSinceLastUpload = cadenceMetrics(videos, now)
	if subs := parseUintPtr(subscribersRaw); subs != nil && *subs > 0 && metrics.medianViews > 0 {
		ratio := metrics.medianViews / float64(*subs)
		metrics.subscriberViewRatio = &ratio
	}
	return metrics
}

func channelPerformanceMetrics(videos []ChannelVideoSummary, subscribersRaw string, now func() time.Time) []PerformanceMetric {
	m := channelRawMetrics(videos, subscribersRaw, now)
	out := []PerformanceMetric{}
	if m.avgViews > 0 {
		out = append(out, PerformanceMetric{ID: "average_views", Label: "Average views", Value: formatFloat(m.avgViews, 0), RawValue: m.avgViews, Score: scoreViewsPerDay(m.avgViews / 30), Explanation: "Raw metric from sampled public videos. Median should be preferred when outliers dominate."})
	}
	if m.medianViews > 0 {
		out = append(out, PerformanceMetric{ID: "median_views", Label: "Median views", Value: formatFloat(m.medianViews, 0), RawValue: m.medianViews, Score: scoreViewsPerDay(m.medianViews / 30), Explanation: "Median public views across sampled videos, used to reduce outlier distortion."})
	}
	if m.likesPer1000 != nil {
		out = append(out, PerformanceMetric{ID: "likes_per_1000_views", Label: "Likes per 1,000 views", Value: formatFloat(*m.likesPer1000, 1), RawValue: *m.likesPer1000, Score: scoreRate(*m.likesPer1000, 5, 60), Explanation: "Visible likes per 1,000 public views. Hidden likes are unavailable, not zero."})
	}
	if m.commentsPer1000 != nil {
		out = append(out, PerformanceMetric{ID: "comments_per_1000_views", Label: "Comments per 1,000 views", Value: formatFloat(*m.commentsPer1000, 1), RawValue: *m.commentsPer1000, Score: scoreRate(*m.commentsPer1000, 0.5, 10), Explanation: "Visible comments per 1,000 public views. Hidden comments are unavailable, not zero."})
	}
	if m.uploadsPerMonth > 0 {
		out = append(out, PerformanceMetric{ID: "uploads_per_month", Label: "Uploads per month", Value: formatFloat(m.uploadsPerMonth, 1), RawValue: m.uploadsPerMonth, Score: scoreConsistency(m.uploadsPerMonth, m.daysSinceLastUpload), Explanation: "Dated public uploads per month within the sampled set."})
	}
	if m.medianDuration > 0 {
		out = append(out, PerformanceMetric{ID: "median_duration", Label: "Median duration", Value: formatDurationSeconds(int(m.medianDuration)), RawValue: m.medianDuration, Score: 50, Explanation: "Raw median duration from public contentDetails. It is not normalized as a quality score."})
	}
	if m.subscriberViewRatio != nil {
		out = append(out, PerformanceMetric{ID: "subscriber_view_relationship", Label: "Median views per subscriber", Value: formatFloat(*m.subscriberViewRatio, 2), RawValue: *m.subscriberViewRatio, Score: scoreRate(*m.subscriberViewRatio, 0.05, 1.5), Explanation: "Median sampled public views divided by public subscriber count. Hidden subscribers make this unavailable."})
	}
	if m.daysSinceLastUpload > 0 {
		out = append(out, PerformanceMetric{ID: "days_since_last_upload", Label: "Days since last upload", Value: formatFloat(m.daysSinceLastUpload, 0), RawValue: m.daysSinceLastUpload, Score: clampInt(100 - int(m.daysSinceLastUpload*2)), Explanation: "Raw recency metric from the latest sampled public upload."})
	}
	return out
}

func estimateChannelRevenue(channel youtubeChannelItem, videos []ChannelVideoSummary, niche NicheAnalysis) RevenueEstimate {
	format := primaryChannelFormat(videos)
	rpmLow, rpmHigh := rpmRange(niche, format)
	if format == "short_form" {
		rpmLow *= 0.12
		rpmHigh *= 0.18
	}
	totalViews := parseUintPtr(channel.Statistics.ViewCount)
	publicViews := uint64(0)
	if totalViews != nil {
		publicViews = *totalViews
	}
	recentViews := uint64(0)
	for _, video := range videos {
		if video.Views != nil {
			recentViews += *video.Views
		}
	}
	low := float64(publicViews) / 1000 * rpmLow
	high := float64(publicViews) / 1000 * rpmHigh
	confidence := 42
	if publicViews > 0 {
		confidence += 12
	}
	if len(videos) >= 10 {
		confidence += 12
	}
	if format != "unknown" {
		confidence += 8
	}
	if visibleEngagementCount(videos) < 4 {
		confidence -= 6
	}
	confidence = clampInt(confidence)
	return RevenueEstimate{
		Source:                     "public_estimate",
		ModelType:                  "public_channel_views_x_estimated_rpm_range",
		Currency:                   "USD",
		Low:                        roundMoney(low),
		Midpoint:                   roundMoney((low + high) / 2),
		High:                       roundMoney(high),
		FormattedRange:             formatMoneyRange(low, high),
		RPMLow:                     roundMoney(rpmLow),
		RPMHigh:                    roundMoney(rpmHigh),
		EstimatedRevenuePer1000:    fmt.Sprintf("$%.2f–$%.2f estimated RPM", rpmLow, rpmHigh),
		Confidence:                 ratingForScore(confidence),
		ConfidenceScore:            confidence,
		CalculationBasis:           fmt.Sprintf("Estimated lifetime public ad revenue uses %s public channel views multiplied by a broad estimated RPM range. Sampled recent videos account for %s public views but are not treated as a complete history.", formatUint(publicViews), formatUint(recentViews)),
		Assumptions:                []string{"Only public views are used.", "RPM varies by audience geography, ad fill, topic, format, seasonality, and monetisation eligibility.", "Short-form RPM is lowered only when sampled duration classification indicates a Shorts-heavy channel."},
		Exclusions:                 []string{"Actual YouTube Analytics revenue", "Sponsorships", "Affiliate revenue", "Memberships", "Merchandise", "Courses", "Private, deleted, hidden, or unlisted videos", "Invalid traffic and Premium adjustments"},
		MonetisationEligibility:    "Unknown from public metadata",
		ActualAnalyticsUnavailable: true,
	}
}

func buildChannelCharts(videos []ChannelVideoSummary, pillars []ChannelContentPillar) ChannelCharts {
	sorted := append([]ChannelVideoSummary{}, videos...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].PublishedAt < sorted[j].PublishedAt })
	perf := []ChannelChartPoint{}
	for _, video := range sorted {
		if video.Views == nil || video.PublishedAt == "" {
			continue
		}
		perf = append(perf, ChannelChartPoint{Label: formatPublishedDate(video.PublishedAt), Date: video.PublishedAt, Title: video.Title, Views: video.Views, Value: float64(*video.Views), Duration: formatISO8601Duration(video.Duration), Format: video.Format, Pillar: video.Pillar, VideoID: video.VideoID})
	}
	dist := []ChannelChartPoint{}
	views := videoViewFloats(videos)
	sort.Float64s(views)
	for i, v := range views {
		dist = append(dist, ChannelChartPoint{Label: fmt.Sprintf("Video %d", i+1), Value: v})
	}
	cadenceCounts := map[string]int{}
	for _, video := range videos {
		if t, err := time.Parse(time.RFC3339, video.PublishedAt); err == nil {
			key := t.Format("2006-01")
			cadenceCounts[key]++
		}
	}
	cadenceKeys := mapKeysInt(cadenceCounts)
	sort.Strings(cadenceKeys)
	cadence := []ChannelChartPoint{}
	for _, key := range cadenceKeys {
		cadence = append(cadence, ChannelChartPoint{Label: key, Value: float64(cadenceCounts[key])})
	}
	topic := []ChannelChartPoint{}
	for _, pillar := range pillars {
		if pillar.MedianViews != nil {
			topic = append(topic, ChannelChartPoint{Label: pillar.Name, Value: *pillar.MedianViews, Description: pillar.OpportunityStatus})
		}
	}
	formatMedian := map[string][]float64{}
	for _, video := range videos {
		if video.Views != nil && video.Format != "" && video.Format != "unknown" {
			formatMedian[video.Format] = append(formatMedian[video.Format], float64(*video.Views))
		}
	}
	formatPoints := []ChannelChartPoint{}
	for _, key := range mapKeysFloatSlice(formatMedian) {
		formatPoints = append(formatPoints, ChannelChartPoint{Label: strings.ReplaceAll(key, "_", " "), Value: medianFloat(formatMedian[key])})
	}
	return ChannelCharts{UploadPerformance: perf, ViewsDistribution: dist, UploadCadence: cadence, TopicPerformance: topic, FormatPerformance: formatPoints}
}

func buildChannelVideoGroups(videos []ChannelVideoSummary) []ChannelVideoGroup {
	if len(videos) == 0 {
		return nil
	}
	median := medianFloat(videoViewFloats(videos))
	recent := append([]ChannelVideoSummary{}, videos...)
	sort.Slice(recent, func(i, j int) bool { return recent[i].ViewsPerDay > recent[j].ViewsPerDay })
	evergreen := append([]ChannelVideoSummary{}, videos...)
	sort.Slice(evergreen, func(i, j int) bool {
		iv, jv := uint64(0), uint64(0)
		if evergreen[i].Views != nil {
			iv = *evergreen[i].Views
		}
		if evergreen[j].Views != nil {
			jv = *evergreen[j].Views
		}
		return iv > jv
	})
	under := []ChannelVideoSummary{}
	for _, video := range videos {
		if video.Views != nil && median > 0 && float64(*video.Views) < median*0.5 {
			under = append(under, video)
		}
	}
	return []ChannelVideoGroup{
		{ID: "recent_winners", Label: "Recent winners", Explanation: "Sorted by public views per day, so older videos do not automatically dominate.", Videos: topNChannelVideos(recent, 6)},
		{ID: "evergreen_performers", Label: "Evergreen performers", Explanation: "Highest visible lifetime views in the sampled set. Age still matters when interpreting these.", Videos: topNChannelVideos(evergreen, 6)},
		{ID: "underperformers", Label: "Underperformers", Explanation: "Videos below half the sampled median views. This is a robust median-based threshold, not a platform benchmark.", Videos: topNChannelVideos(under, 6)},
	}
}

func buildChannelPackaging(videos []ChannelVideoSummary, pillars []ChannelContentPillar) ChannelPackaging {
	if len(videos) == 0 {
		return ChannelPackaging{}
	}
	words, questions, numbers, thumbs := 0, 0, 0, 0
	evidence := []string{}
	for _, video := range videos {
		title := strings.TrimSpace(video.Title)
		words += len(strings.Fields(title))
		if strings.Contains(title, "?") {
			questions++
		}
		if regexp.MustCompile(`\d`).MatchString(title) {
			numbers++
		}
		if video.ThumbnailURL != "" {
			thumbs++
		}
		if len(evidence) < 4 && title != "" {
			evidence = append(evidence, title)
		}
	}
	avg := float64(words) / float64(len(videos))
	strong := "Direct topic-led titles"
	if numbers > len(videos)/3 {
		strong = "Number-led titles"
	} else if questions > len(videos)/3 {
		strong = "Question-led titles"
	}
	weak := "Some titles may be too broad to signal the payoff quickly."
	if avg > 12 {
		weak = "Long titles may bury the payoff on mobile surfaces."
	} else if avg < 5 {
		weak = "Very short titles may not give enough specificity."
	}
	core := firstPhraseFromPillars(pillars)
	return ChannelPackaging{
		StrongestPattern:          strong,
		WeakestHabit:              weak,
		RepeatedWinningStructure:  strong + " around " + firstNonEmpty(core, "the channel's main topic"),
		RecommendedTitleFramework: fmt.Sprintf("[Specific viewer/problem] + [clear outcome] + [proof or format], e.g. \"How to fix %s without starting over\"", firstNonEmpty(core, "this problem")),
		AverageTitleLength:        avg,
		QuestionTitleShare:        float64(questions) / float64(len(videos)),
		NumberTitleShare:          float64(numbers) / float64(len(videos)),
		ThumbnailAvailability:     float64(thumbs) / float64(len(videos)),
		Evidence:                  evidence,
	}
}

func buildChannelGrowthOpportunities(niche NicheAnalysis, pillars []ChannelContentPillar, groups []ChannelVideoGroup, packaging ChannelPackaging) []ChannelOpportunity {
	out := []ChannelOpportunity{}
	core := firstPhraseFromPillars(pillars)
	if core == "" {
		core = firstNonEmpty(niche.SpecificTopic, niche.SubNiche, niche.PrimaryNiche)
	}
	bestEvidence := []string{}
	if len(groups) > 0 && len(groups[0].Videos) > 0 {
		bestEvidence = append(bestEvidence, groups[0].Videos[0].Title)
	}
	out = append(out, ChannelOpportunity{
		Title:             "Expand the proven pillar",
		Why:               "The sampled titles repeatedly point to " + core + ", giving the channel a clearer repeatable lane.",
		Evidence:          bestEvidence,
		RecommendedFormat: "Long-form follow-up with a Shorts cutdown",
		SuggestedAudience: firstNonEmpty(niche.TargetAudience, niche.AudienceType, "Current viewers"),
		Confidence:        "Good",
		SampleTitle:       "The next step after " + titleCase(core),
		NextAction:        "Choose the strongest recent winner and make a direct follow-up with a sharper payoff.",
	})
	if packaging.WeakestHabit != "" {
		out = append(out, ChannelOpportunity{
			Title:             "Repair packaging on a strong subject",
			Why:               packaging.WeakestHabit,
			Evidence:          packaging.Evidence,
			RecommendedFormat: primaryOpportunityFormat(groups),
			SuggestedAudience: firstNonEmpty(niche.TargetAudience, "Search-led viewers"),
			Confidence:        "Moderate",
			SampleTitle:       "How to " + core + " without the common mistake",
			NextAction:        "Rewrite one upcoming title with a clearer viewer problem and outcome.",
		})
	}
	out = append(out, ChannelOpportunity{
		Title:             "Create a beginner entry point",
		Why:               "Repeatable channels need an obvious first video for new viewers; public metadata can support a beginner version without private audience assumptions.",
		Evidence:          topPillarNames(pillars, 3),
		RecommendedFormat: "Beginner guide",
		SuggestedAudience: "New viewers entering " + firstNonEmpty(niche.PrimaryNiche, "this topic"),
		Confidence:        "Moderate",
		SampleTitle:       titleCase(core) + " for beginners: the simple version",
		NextAction:        "Turn the strongest pillar into a glossary, checklist, or first-principles explainer.",
	})
	return out
}

func buildChannelContentPlan(videos []ChannelVideoSummary, pillars []ChannelContentPillar, opps []ChannelOpportunity, niche NicheAnalysis, now func() time.Time) []ChannelPlanWeek {
	uploadsPerMonth, _ := cadenceMetrics(videos, now)
	ideasPerWeek := 1
	if uploadsPerMonth >= 8 {
		ideasPerWeek = 2
	}
	core := firstPhraseFromPillars(pillars)
	if core == "" {
		core = firstNonEmpty(niche.SpecificTopic, niche.SubNiche, niche.PrimaryNiche, "core topic")
	}
	weeks := []ChannelPlanWeek{}
	for i := 1; i <= 4; i++ {
		theme := []string{"Proven-topic follow-up", "Packaging test", "Adjacent topic expansion", "Remake or series continuation"}[i-1]
		ideas := []ChannelPlanIdea{}
		for j := 0; j < ideasPerWeek; j++ {
			opp := ChannelOpportunity{RecommendedFormat: "Long-form", Title: theme}
			if len(opps) > 0 {
				opp = opps[(i+j-1)%len(opps)]
			}
			ideas = append(ideas, ChannelPlanIdea{
				WorkingTitle:      firstNonEmpty(opp.SampleTitle, fmt.Sprintf("%s: %s", theme, titleCase(core))),
				ContentPillar:     core,
				Format:            firstNonEmpty(opp.RecommendedFormat, "Long-form"),
				Objective:         opp.Title,
				Evidence:          strings.Join(topN(opp.Evidence, 2), " · "),
				HookDirection:     "Open with the viewer problem, then show the concrete payoff.",
				RecommendedTiming: fmt.Sprintf("Week %d, slot %d", i, j+1),
			})
		}
		weeks = append(weeks, ChannelPlanWeek{Week: i, Theme: theme, Cadence: fmt.Sprintf("%d upload(s)", ideasPerWeek), Ideas: ideas, Rationale: "Cadence is based on recent public publishing volume; no calendar dates are invented."})
	}
	return weeks
}

func buildChannelCTAContext(channelID, title, canonical string, pillars []ChannelContentPillar, opps []ChannelOpportunity, metrics []PerformanceMetric, limitations []string) ChannelCTAContext {
	opportunity := ""
	audience := ""
	format := ""
	if len(opps) > 0 {
		opportunity = opps[0].Title
		audience = opps[0].SuggestedAudience
		format = opps[0].RecommendedFormat
	}
	evidence := []string{}
	for _, metric := range topNPerformance(metrics, 4) {
		evidence = append(evidence, metric.Label+": "+metric.Value)
	}
	return ChannelCTAContext{
		ActionLabel:             "Build a channel strategy from this analysis",
		ChannelID:               channelID,
		ChannelTitle:            title,
		CanonicalURL:            canonical,
		CleanContentPillars:     topPillarNames(pillars, 6),
		PerformanceEvidence:     evidence,
		SelectedOpportunity:     opportunity,
		SelectedAudience:        audience,
		RecommendedFormat:       format,
		PublicDataLimitations:   limitations,
		ScriptGenerationEnabled: true,
	}
}

func buildChannelAnalysisDetails(videos []ChannelVideoSummary, channel youtubeChannelItem, now func() time.Time) ChannelAnalysisDetails {
	hidden := []string{}
	if channel.Statistics.SubscriberCount == "" {
		hidden = append(hidden, "Subscriber count is hidden or unavailable.")
	}
	if visibleLikesCount(videos) < len(videos) {
		hidden = append(hidden, "Some like counts are hidden or unavailable and are not treated as zero.")
	}
	if visibleCommentsCount(videos) < len(videos) {
		hidden = append(hidden, "Some comment counts are hidden or unavailable and are not treated as zero.")
	}
	start, end := sampleRange(videos)
	return ChannelAnalysisDetails{
		SampledVideoCount:    len(videos),
		SampleStart:          start,
		SampleEnd:            end,
		ProviderAvailability: "youtube_data_api_public_metadata",
		HiddenMetricNotes:    hidden,
		ScoringMethodology:   []string{"Channel opportunity is bounded 0-100.", "Weights: topic clarity, packaging, consistency, repeatability, format efficiency, momentum, monetisation potential, and evidence quality.", "Evidence quality caps low-sample or hidden-stat analyses."},
		TopicMethodology:     []string{"Pillars are derived from cleaned channel metadata and sampled video titles/descriptions.", "ValidCreatorPhrase filters boilerplate, malformed n-grams, placeholders, and near-duplicates."},
		RevenueMethodology:   []string{"Public channel views multiplied by estimated RPM ranges.", "Shorts RPM is only lowered when duration classification supports a Shorts-heavy sample.", "Actual YouTube Analytics and non-ad revenue are excluded."},
		ClassificationRules:  []string{"Short-form: duration up to 180 seconds.", "Long-form: duration above 180 seconds.", "Livestream only when duration/source signals support it; otherwise unknown."},
		AnalysisTimestamp:    now().Format(time.RFC3339),
	}
}

func videoViewFloats(videos []ChannelVideoSummary) []float64 {
	out := []float64{}
	for _, video := range videos {
		if video.Views != nil {
			out = append(out, float64(*video.Views))
		}
	}
	return out
}

func medianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	values = append([]float64{}, values...)
	sort.Float64s(values)
	mid := len(values) / 2
	if len(values)%2 == 0 {
		return (values[mid-1] + values[mid]) / 2
	}
	return values[mid]
}

func medianFloatPtr(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	v := medianFloat(values)
	return &v
}

func averageFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func strongestVideo(videos []ChannelVideoSummary, mode string) *ChannelVideoSummary {
	if len(videos) == 0 {
		return nil
	}
	best := videos[0]
	for _, video := range videos[1:] {
		switch mode {
		case "views_per_day":
			if video.ViewsPerDay > best.ViewsPerDay {
				best = video
			}
		default:
			if video.Views != nil && (best.Views == nil || *video.Views > *best.Views) {
				best = video
			}
		}
	}
	return &best
}

func engagementRates(videos []ChannelVideoSummary) (*float64, *float64) {
	var likes, likeViews, comments, commentViews uint64
	for _, video := range videos {
		if video.Views == nil || *video.Views == 0 {
			continue
		}
		if video.Likes != nil {
			likes += *video.Likes
			likeViews += *video.Views
		}
		if video.Comments != nil {
			comments += *video.Comments
			commentViews += *video.Views
		}
	}
	var likeRate *float64
	if likeViews > 0 {
		v := float64(likes) / float64(likeViews) * 1000
		likeRate = &v
	}
	var commentRate *float64
	if commentViews > 0 {
		v := float64(comments) / float64(commentViews) * 1000
		commentRate = &v
	}
	return likeRate, commentRate
}

func medianDurationSeconds(videos []ChannelVideoSummary) float64 {
	values := []float64{}
	for _, video := range videos {
		if seconds, ok := parseISO8601DurationSeconds(video.Duration); ok && seconds > 0 {
			values = append(values, float64(seconds))
		}
	}
	return medianFloat(values)
}

func cadenceMetrics(videos []ChannelVideoSummary, now func() time.Time) (float64, float64) {
	var newest, oldest time.Time
	count := 0
	for _, video := range videos {
		t, err := time.Parse(time.RFC3339, video.PublishedAt)
		if err != nil {
			continue
		}
		count++
		if newest.IsZero() || t.After(newest) {
			newest = t
		}
		if oldest.IsZero() || t.Before(oldest) {
			oldest = t
		}
	}
	if count == 0 {
		return 0, 0
	}
	spanDays := newest.Sub(oldest).Hours() / 24
	if spanDays < 7 {
		spanDays = 30
	}
	uploadsPerMonth := float64(count) / (spanDays / 30)
	daysSinceLast := now().Sub(newest).Hours() / 24
	if daysSinceLast < 0 {
		daysSinceLast = 0
	}
	return uploadsPerMonth, daysSinceLast
}

func scoreConsistency(uploadsPerMonth, daysSinceLast float64) int {
	score := 35
	switch {
	case uploadsPerMonth >= 12:
		score += 35
	case uploadsPerMonth >= 4:
		score += 28
	case uploadsPerMonth >= 1:
		score += 18
	default:
		score += 5
	}
	switch {
	case daysSinceLast <= 14:
		score += 25
	case daysSinceLast <= 45:
		score += 15
	case daysSinceLast <= 120:
		score += 5
	default:
		score -= 10
	}
	return clampInt(score)
}

func scorePackaging(videos []ChannelVideoSummary) int {
	if len(videos) == 0 {
		return 25
	}
	score := 42
	specific := 0
	withNumber := 0
	withQuestion := 0
	for _, video := range videos {
		title := strings.TrimSpace(video.Title)
		words := len(strings.Fields(title))
		if words >= 5 && words <= 12 {
			specific++
		}
		if regexp.MustCompile(`\d`).MatchString(title) {
			withNumber++
		}
		if strings.Contains(title, "?") {
			withQuestion++
		}
	}
	score += int(float64(specific) / float64(len(videos)) * 28)
	score += int(float64(withNumber+withQuestion) / float64(len(videos)) * 18)
	return clampInt(score)
}

func scoreRepeatability(videos []ChannelVideoSummary, pillars []ChannelContentPillar) int {
	if len(videos) == 0 || len(pillars) == 0 {
		return 25
	}
	score := 35 + len(pillars)*5
	for _, pillar := range pillars {
		if pillar.UploadCount >= 3 {
			score += 7
		}
	}
	return clampInt(score)
}

func scoreMomentum(videos []ChannelVideoSummary) int {
	if len(videos) == 0 {
		return 25
	}
	views := videoViewFloats(videos)
	median := medianFloat(views)
	score := 35
	above := 0
	for _, video := range videos {
		if video.Views != nil && median > 0 && float64(*video.Views) >= median {
			above++
		}
		if video.ViewsPerDay >= 1000 {
			score += 2
		}
	}
	score += int(float64(above) / float64(len(videos)) * 30)
	return clampInt(score)
}

func scoreFormatEfficiency(videos []ChannelVideoSummary) int {
	formatViews := map[string][]float64{}
	for _, video := range videos {
		if video.Views != nil && video.Format != "" && video.Format != "unknown" {
			formatViews[video.Format] = append(formatViews[video.Format], float64(*video.Views))
		}
	}
	if len(formatViews) == 0 {
		return 40
	}
	best := 0.0
	for _, values := range formatViews {
		if med := medianFloat(values); med > best {
			best = med
		}
	}
	return clampInt(40 + int(scoreViewsPerDay(best/30)/2))
}

func visibleEngagementCount(videos []ChannelVideoSummary) int {
	count := 0
	for _, video := range videos {
		if video.Likes != nil || video.Comments != nil {
			count++
		}
	}
	return count
}

func visibleLikesCount(videos []ChannelVideoSummary) int {
	count := 0
	for _, video := range videos {
		if video.Likes != nil {
			count++
		}
	}
	return count
}

func visibleCommentsCount(videos []ChannelVideoSummary) int {
	count := 0
	for _, video := range videos {
		if video.Comments != nil {
			count++
		}
	}
	return count
}

func primaryChannelFormat(videos []ChannelVideoSummary) string {
	counts := map[string]int{}
	for _, video := range videos {
		if video.Format != "" && video.Format != "unknown" {
			counts[video.Format]++
		}
	}
	best := ""
	for key, count := range counts {
		if best == "" || count > counts[best] {
			best = key
		}
	}
	if best == "" {
		return "unknown"
	}
	return best
}

func latestVideoDate(videos []ChannelVideoSummary) string {
	latest := ""
	for _, video := range videos {
		if video.PublishedAt > latest {
			latest = video.PublishedAt
		}
	}
	return latest
}

func sampleRange(videos []ChannelVideoSummary) (string, string) {
	start, end := "", ""
	for _, video := range videos {
		if video.PublishedAt == "" {
			continue
		}
		if start == "" || video.PublishedAt < start {
			start = video.PublishedAt
		}
		if end == "" || video.PublishedAt > end {
			end = video.PublishedAt
		}
	}
	return start, end
}

func formatDurationSeconds(seconds int) string {
	if seconds <= 0 {
		return "Unavailable"
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func mapKeysInt(in map[string]int) []string {
	out := make([]string, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	return out
}

func mapKeysFloatSlice(in map[string][]float64) []string {
	out := make([]string, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func firstPhraseFromPillars(pillars []ChannelContentPillar) string {
	if len(pillars) == 0 {
		return ""
	}
	return pillars[0].Name
}

func topPillarNames(pillars []ChannelContentPillar, n int) []string {
	out := []string{}
	for _, pillar := range pillars {
		if pillar.Name != "" {
			out = append(out, pillar.Name)
		}
	}
	return topN(out, n)
}

func primaryOpportunityFormat(groups []ChannelVideoGroup) string {
	for _, group := range groups {
		for _, video := range group.Videos {
			if video.Format != "" && video.Format != "unknown" {
				return strings.ReplaceAll(video.Format, "_", " ")
			}
		}
	}
	return "Long-form"
}

func topNPerformance(metrics []PerformanceMetric, n int) []PerformanceMetric {
	if n > len(metrics) {
		n = len(metrics)
	}
	out := make([]PerformanceMetric, n)
	copy(out, metrics[:n])
	return out
}

type ScoringInput struct {
	SourceConfidence float64
	Views            *uint64
	PublishedAt      string
	KeywordCount     int
	HasEvidenceURL   bool
	RegionMatch      float64
	NicheMatch       float64
}

func ScoreEvidence(in ScoringInput) (float64, string) {
	recency := 0.35
	if in.PublishedAt != "" {
		if t, err := time.Parse(time.RFC3339, in.PublishedAt); err == nil {
			ageHours := time.Since(t).Hours()
			switch {
			case ageHours <= 72:
				recency = 1
			case ageHours <= 24*30:
				recency = 0.7
			case ageHours <= 24*180:
				recency = 0.45
			}
		}
	}
	engagement := 0.35
	if in.Views != nil {
		switch {
		case *in.Views >= 1000000:
			engagement = 1
		case *in.Views >= 100000:
			engagement = 0.75
		case *in.Views >= 10000:
			engagement = 0.55
		default:
			engagement = 0.35
		}
	}
	evidence := 0.4
	if in.HasEvidenceURL {
		evidence = 0.85
	}
	keyword := minFloat(1, float64(in.KeywordCount)/12)
	score := 100 * ((0.22 * in.SourceConfidence) + (0.18 * recency) + (0.18 * engagement) + (0.14 * in.RegionMatch) + (0.14 * in.NicheMatch) + (0.14 * ((evidence + keyword) / 2)))
	return score, "Score combines source confidence, recency, public engagement, region/language fit, inferred niche relevance, and evidence quality. It is not an exact platform ranking."
}

func TikTokResearchStatus(clientID, secret, token string) ProviderStatus {
	configured := strings.TrimSpace(token) != "" || (strings.TrimSpace(clientID) != "" && strings.TrimSpace(secret) != "")
	if !configured {
		return ProviderStatus{ID: "tiktok_research_api", Name: "TikTok Research API", Platform: "tiktok", Status: StatusNotConfigured, Message: "Connect TikTok Research API credentials and scopes to enable this source.", Scopes: []string{"region_code", "hashtag_names", "view_count", "like_count", "comment_count", "share_count"}}
	}
	return ProviderStatus{ID: "tiktok_research_api", Name: "TikTok Research API", Platform: "tiktok", Status: StatusUnavailable, Message: "Credentials are present, but this build only includes the provider foundation. No TikTok trend data is returned yet.", Scopes: []string{"region_code", "hashtag_names", "view_count", "like_count", "comment_count", "share_count"}}
}

func MetaInstagramStatus(appID, appSecret string) ProviderStatus {
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(appSecret) == "" {
		return ProviderStatus{ID: "instagram_graph_api", Name: "Instagram Graph / Meta", Platform: "instagram", Status: StatusNotConfigured, Message: "Connect Graph API credentials in Settings/Railway variables to enable this source."}
	}
	return ProviderStatus{ID: "instagram_graph_api", Name: "Instagram Graph / Meta", Platform: "instagram", Status: StatusUnavailable, Message: "Credentials are present, but no official trend integration is enabled in this build."}
}

func XStatus(clientID, clientSecret string) ProviderStatus {
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" {
		return ProviderStatus{ID: "x_api", Name: "X API", Platform: "x", Status: StatusNotConfigured, Message: "Connect X API credentials in Settings/Railway variables to enable this source."}
	}
	return ProviderStatus{ID: "x_api", Name: "X API", Platform: "x", Status: StatusUnavailable, Message: "Credentials are present, but no official trend integration is enabled in this build."}
}

func FacebookStatus(appID, appSecret string) ProviderStatus {
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(appSecret) == "" {
		return ProviderStatus{ID: "facebook_graph_api", Name: "Facebook / Meta", Platform: "facebook", Status: StatusNotConfigured, Message: "Connect Meta Graph API credentials in Settings/Railway variables to enable this source."}
	}
	return ProviderStatus{ID: "facebook_graph_api", Name: "Facebook / Meta", Platform: "facebook", Status: StatusUnavailable, Message: "Credentials are present, but no official trend integration is enabled in this build."}
}

type youtubeVideosResponse struct {
	Items []youtubeVideoItem `json:"items"`
}

type youtubeVideoItem struct {
	ID      string `json:"id"`
	Snippet struct {
		PublishedAt  string   `json:"publishedAt"`
		ChannelID    string   `json:"channelId"`
		Title        string   `json:"title"`
		Description  string   `json:"description"`
		ChannelTitle string   `json:"channelTitle"`
		Tags         []string `json:"tags"`
		CategoryID   string   `json:"categoryId"`
		Thumbnails   struct {
			Default struct {
				URL string `json:"url"`
			} `json:"default"`
			Medium struct {
				URL string `json:"url"`
			} `json:"medium"`
			High struct {
				URL string `json:"url"`
			} `json:"high"`
		} `json:"thumbnails"`
	} `json:"snippet"`
	Statistics struct {
		ViewCount    string `json:"viewCount"`
		LikeCount    string `json:"likeCount"`
		CommentCount string `json:"commentCount"`
	} `json:"statistics"`
	ContentDetails struct {
		Duration string `json:"duration"`
	} `json:"contentDetails"`
	TopicDetails struct {
		TopicCategories []string `json:"topicCategories"`
	} `json:"topicDetails"`
}

func bestVideoThumbnail(item youtubeVideoItem) string {
	if item.Snippet.Thumbnails.High.URL != "" {
		return item.Snippet.Thumbnails.High.URL
	}
	if item.Snippet.Thumbnails.Medium.URL != "" {
		return item.Snippet.Thumbnails.Medium.URL
	}
	return item.Snippet.Thumbnails.Default.URL
}

type youtubeChannelsResponse struct {
	Items []youtubeChannelItem `json:"items"`
}

type youtubeChannelItem struct {
	ID      string `json:"id"`
	Snippet struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Country     string `json:"country"`
		PublishedAt string `json:"publishedAt"`
		CustomURL   string `json:"customUrl"`
		Thumbnails  struct {
			Default struct {
				URL string `json:"url"`
			} `json:"default"`
			Medium struct {
				URL string `json:"url"`
			} `json:"medium"`
			High struct {
				URL string `json:"url"`
			} `json:"high"`
		} `json:"thumbnails"`
	} `json:"snippet"`
	Statistics struct {
		ViewCount       string `json:"viewCount"`
		SubscriberCount string `json:"subscriberCount"`
		VideoCount      string `json:"videoCount"`
	} `json:"statistics"`
	ContentDetails struct {
		RelatedPlaylists struct {
			Uploads string `json:"uploads"`
		} `json:"relatedPlaylists"`
	} `json:"contentDetails"`
	TopicDetails struct {
		TopicCategories []string `json:"topicCategories"`
	} `json:"topicDetails"`
	BrandingSettings struct {
		Image struct {
			BannerExternalURL string `json:"bannerExternalUrl"`
		} `json:"image"`
		Channel struct {
			Country string `json:"country"`
		} `json:"channel"`
	} `json:"brandingSettings"`
}

type youtubeSearchResponse struct {
	Items []struct {
		ID struct {
			VideoID   string `json:"videoId"`
			ChannelID string `json:"channelId"`
		} `json:"id"`
		Snippet struct {
			PublishedAt  string `json:"publishedAt"`
			ChannelID    string `json:"channelId"`
			ChannelTitle string `json:"channelTitle"`
			Title        string `json:"title"`
		} `json:"snippet"`
	} `json:"items"`
}

func (p *YouTubeProvider) getJSON(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "TrendCortex/creator-research")
	res, err := p.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded {
			return &ProviderError{Code: ProviderErrorTimeout, Err: err}
		}
		return &ProviderError{Code: ProviderErrorTemporary, Err: err}
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return &ProviderError{Code: ProviderErrorTemporary, HTTPStatus: res.StatusCode, Err: err}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return classifyProviderHTTPError(res.StatusCode, body)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return &ProviderError{Code: ProviderErrorMalformed, HTTPStatus: res.StatusCode, Err: err}
	}
	return nil
}

func classifyProviderHTTPError(status int, body []byte) error {
	reason, message := extractProviderErrorReason(body)
	code := ProviderErrorHTTP
	switch {
	case reason == "quotaExceeded" || reason == "dailyLimitExceeded" || reason == "rateLimitExceeded":
		code = ProviderErrorQuota
	case reason == "keyInvalid" || reason == "badRequest" || reason == "authError" || reason == "forbidden" || reason == "accessNotConfigured" || reason == "ipRefererBlocked" || reason == "apiKeyServiceBlocked" || reason == "API_KEY_INVALID":
		code = ProviderErrorCredentials
	case status == http.StatusTooManyRequests:
		code = ProviderErrorQuota
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		code = ProviderErrorCredentials
	case status >= 500:
		code = ProviderErrorTemporary
	}
	if message == "" {
		message = http.StatusText(status)
	}
	return &ProviderError{Code: code, HTTPStatus: status, Reason: reason, Message: message}
}

func extractProviderErrorReason(body []byte) (string, string) {
	var decoded struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
			Errors  []struct {
				Reason  string `json:"reason"`
				Message string `json:"message"`
			} `json:"errors"`
			Details []struct {
				Reason string `json:"reason"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return "", ""
	}
	if len(decoded.Error.Errors) > 0 {
		reason := strings.TrimSpace(decoded.Error.Errors[0].Reason)
		message := strings.TrimSpace(decoded.Error.Errors[0].Message)
		if message == "" {
			message = strings.TrimSpace(decoded.Error.Message)
		}
		return reason, message
	}
	if len(decoded.Error.Details) > 0 {
		return strings.TrimSpace(decoded.Error.Details[0].Reason), strings.TrimSpace(decoded.Error.Message)
	}
	return strings.TrimSpace(decoded.Error.Status), strings.TrimSpace(decoded.Error.Message)
}

type resolvedChannelIdentity struct {
	ChannelID string
	Handle    string
	InputKind string
}

func (p *YouTubeProvider) resolveChannelID(ctx context.Context, raw string) (string, error) {
	resolved, err := p.resolveChannelIdentity(ctx, raw)
	if err != nil {
		return "", err
	}
	return resolved.ChannelID, nil
}

func (p *YouTubeProvider) resolveChannelIdentity(ctx context.Context, raw string) (resolvedChannelIdentity, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return resolvedChannelIdentity{}, errors.New("channel_url is required")
	}
	if strings.HasPrefix(raw, "UC") && len(raw) >= 20 && !strings.Contains(raw, "/") {
		return resolvedChannelIdentity{ChannelID: raw, InputKind: "channel_id"}, nil
	}
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		host := strings.ToLower(strings.TrimPrefix(u.Host, "www."))
		if !strings.Contains(host, "youtube.com") && host != "youtu.be" {
			return resolvedChannelIdentity{}, errors.New("enter a YouTube channel URL, @handle, or channel ID")
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 2 && parts[0] == "channel" && strings.HasPrefix(parts[1], "UC") {
			return resolvedChannelIdentity{ChannelID: parts[1], InputKind: "channel_url"}, nil
		}
		for _, part := range parts {
			if strings.HasPrefix(part, "@") {
				id, err := p.resolveChannelByHandle(ctx, part)
				return resolvedChannelIdentity{ChannelID: id, Handle: part, InputKind: "handle_url"}, err
			}
		}
		if len(parts) >= 2 && (parts[0] == "c" || parts[0] == "user") {
			id, err := p.resolveLegacyChannelPath(ctx, parts[1])
			return resolvedChannelIdentity{ChannelID: id, InputKind: parts[0]}, err
		}
		return resolvedChannelIdentity{}, errors.New("unsupported YouTube channel URL. Use /channel/ID, /@handle, /user/name, /c/name, @handle, or a channel ID")
	}
	if strings.HasPrefix(raw, "@") {
		id, err := p.resolveChannelByHandle(ctx, raw)
		return resolvedChannelIdentity{ChannelID: id, Handle: raw, InputKind: "handle"}, err
	}
	return resolvedChannelIdentity{}, errors.New("enter a YouTube channel URL, @handle, or channel ID. Plain search terms are not accepted because they can resolve to the wrong channel")
}

func (p *YouTubeProvider) resolveChannelByHandle(ctx context.Context, handle string) (string, error) {
	handle = strings.TrimSpace(handle)
	if !strings.HasPrefix(handle, "@") || len(strings.TrimPrefix(handle, "@")) < 3 {
		return "", errors.New("invalid YouTube handle")
	}
	endpoint := youtubeAPIURL("channels", map[string]string{"part": "id", "forHandle": handle, "key": p.apiKey})
	var res youtubeChannelsResponse
	if err := p.getJSON(ctx, endpoint, &res); err == nil && len(res.Items) > 0 {
		return res.Items[0].ID, nil
	}
	return "", errors.New("could not resolve that YouTube handle")
}

func (p *YouTubeProvider) resolveLegacyChannelPath(ctx context.Context, pathValue string) (string, error) {
	pathValue = strings.TrimSpace(pathValue)
	if pathValue == "" {
		return "", errors.New("legacy channel path is missing a channel name")
	}
	id, err := p.resolveChannelByQuery(ctx, pathValue)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (p *YouTubeProvider) resolveChannelByQuery(ctx context.Context, query string) (string, error) {
	endpoint := youtubeAPIURL("search", map[string]string{"part": "snippet", "type": "channel", "q": query, "maxResults": "1", "key": p.apiKey})
	var res youtubeSearchResponse
	if err := p.getJSON(ctx, endpoint, &res); err != nil {
		return "", err
	}
	if len(res.Items) == 0 {
		return "", errors.New("could not resolve a YouTube channel ID from that input")
	}
	id := res.Items[0].ID.ChannelID
	if id == "" {
		id = res.Items[0].Snippet.ChannelID
	}
	if id == "" {
		return "", errors.New("could not resolve a YouTube channel ID from that input")
	}
	return id, nil
}

func (p *YouTubeProvider) fetchRecentVideos(ctx context.Context, channelID string, limit int) ([]ChannelVideoSummary, error) {
	return p.fetchChannelVideos(ctx, channelID, "date", limit)
}

func (p *YouTubeProvider) fetchChannelVideos(ctx context.Context, channelID, order string, limit int) ([]ChannelVideoSummary, error) {
	if limit <= 0 || limit > 50 {
		limit = 25
	}
	if order == "" {
		order = "date"
	}
	searchURL := youtubeAPIURL("search", map[string]string{"part": "snippet", "channelId": channelID, "type": "video", "order": order, "maxResults": strconv.Itoa(limit), "key": p.apiKey})
	var search youtubeSearchResponse
	if err := p.getJSON(ctx, searchURL, &search); err != nil {
		return nil, err
	}
	ids := []string{}
	titles := map[string]string{}
	published := map[string]string{}
	channelTitles := map[string]string{}
	for _, item := range search.Items {
		if item.ID.VideoID == "" {
			continue
		}
		ids = append(ids, item.ID.VideoID)
		titles[item.ID.VideoID] = item.Snippet.Title
		published[item.ID.VideoID] = item.Snippet.PublishedAt
		channelTitles[item.ID.VideoID] = item.Snippet.ChannelTitle
	}
	if len(ids) == 0 {
		return []ChannelVideoSummary{}, nil
	}
	videosURL := youtubeAPIURL("videos", map[string]string{"part": "snippet,statistics,contentDetails", "id": strings.Join(ids, ","), "key": p.apiKey})
	var videos youtubeVideosResponse
	if err := p.getJSON(ctx, videosURL, &videos); err != nil {
		return nil, err
	}
	out := make([]ChannelVideoSummary, 0, len(videos.Items))
	for _, video := range videos.Items {
		if video.ID == "" {
			continue
		}
		title := video.Snippet.Title
		if title == "" {
			title = titles[video.ID]
		}
		published[video.ID] = video.Snippet.PublishedAt
		out = append(out, ChannelVideoSummary{
			VideoID:      video.ID,
			Title:        title,
			Description:  video.Snippet.Description,
			ChannelID:    video.Snippet.ChannelID,
			ChannelTitle: firstNonEmpty(video.Snippet.ChannelTitle, channelTitles[video.ID]),
			PublishedAt:  published[video.ID],
			Duration:     video.ContentDetails.Duration,
			ThumbnailURL: bestThumbnail(video),
			Views:        parseUintPtr(video.Statistics.ViewCount),
			Likes:        parseUintPtr(video.Statistics.LikeCount),
			Comments:     parseUintPtr(video.Statistics.CommentCount),
		})
	}
	return out, nil
}

func (p *YouTubeProvider) SearchVideos(ctx context.Context, query, region, language string, limit int) ([]ChannelVideoSummary, error) {
	if p.apiKey == "" {
		return nil, ErrNotConfigured
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []ChannelVideoSummary{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 25
	}
	params := map[string]string{
		"part":       "snippet",
		"type":       "video",
		"q":          query,
		"maxResults": strconv.Itoa(limit),
		"order":      "relevance",
		"key":        p.apiKey,
	}
	if strings.TrimSpace(region) != "" {
		params["regionCode"] = strings.TrimSpace(region)
	}
	if strings.TrimSpace(language) != "" {
		params["relevanceLanguage"] = strings.Split(strings.TrimSpace(language), "-")[0]
	}
	searchURL := youtubeAPIURL("search", params)
	var search youtubeSearchResponse
	if err := p.getJSON(ctx, searchURL, &search); err != nil {
		return nil, err
	}
	ids := []string{}
	titles := map[string]string{}
	published := map[string]string{}
	channelIDs := map[string]string{}
	channelTitles := map[string]string{}
	for _, item := range search.Items {
		if item.ID.VideoID == "" {
			continue
		}
		ids = append(ids, item.ID.VideoID)
		titles[item.ID.VideoID] = item.Snippet.Title
		published[item.ID.VideoID] = item.Snippet.PublishedAt
		channelIDs[item.ID.VideoID] = item.Snippet.ChannelID
		channelTitles[item.ID.VideoID] = item.Snippet.ChannelTitle
	}
	if len(ids) == 0 {
		return []ChannelVideoSummary{}, nil
	}
	videosURL := youtubeAPIURL("videos", map[string]string{"part": "snippet,statistics,contentDetails", "id": strings.Join(ids, ","), "key": p.apiKey})
	var videos youtubeVideosResponse
	if err := p.getJSON(ctx, videosURL, &videos); err != nil {
		return nil, err
	}
	out := make([]ChannelVideoSummary, 0, len(videos.Items))
	for _, video := range videos.Items {
		if video.ID == "" {
			continue
		}
		title := firstNonEmpty(video.Snippet.Title, titles[video.ID])
		out = append(out, ChannelVideoSummary{
			VideoID:      video.ID,
			Title:        title,
			Description:  video.Snippet.Description,
			ChannelID:    firstNonEmpty(video.Snippet.ChannelID, channelIDs[video.ID]),
			ChannelTitle: firstNonEmpty(video.Snippet.ChannelTitle, channelTitles[video.ID]),
			PublishedAt:  firstNonEmpty(video.Snippet.PublishedAt, published[video.ID]),
			Duration:     video.ContentDetails.Duration,
			ThumbnailURL: bestThumbnail(video),
			Views:        parseUintPtr(video.Statistics.ViewCount),
			Likes:        parseUintPtr(video.Statistics.LikeCount),
			Comments:     parseUintPtr(video.Statistics.CommentCount),
		})
	}
	return out, nil
}

func bestThumbnail(video youtubeVideoItem) string {
	if video.Snippet.Thumbnails.High.URL != "" {
		return video.Snippet.Thumbnails.High.URL
	}
	if video.Snippet.Thumbnails.Medium.URL != "" {
		return video.Snippet.Thumbnails.Medium.URL
	}
	return video.Snippet.Thumbnails.Default.URL
}

func mergeChannelVideos(groups ...[]ChannelVideoSummary) []ChannelVideoSummary {
	seen := map[string]bool{}
	out := []ChannelVideoSummary{}
	for _, group := range groups {
		for _, video := range group {
			if video.VideoID != "" && seen[video.VideoID] {
				continue
			}
			if video.VideoID != "" {
				seen[video.VideoID] = true
			}
			out = append(out, video)
		}
	}
	return out
}

func topNChannelVideos(videos []ChannelVideoSummary, n int) []ChannelVideoSummary {
	if n > len(videos) {
		n = len(videos)
	}
	if n < 0 {
		n = 0
	}
	out := make([]ChannelVideoSummary, n)
	copy(out, videos[:n])
	return out
}

func videoTitles(videos []ChannelVideoSummary) []string {
	out := make([]string, 0, len(videos))
	for _, video := range videos {
		if strings.TrimSpace(video.Title) != "" {
			out = append(out, video.Title)
		}
	}
	return out
}

func (p *YouTubeProvider) enhanceVideoKeywords(ctx context.Context, kw KeywordIntelligence, title, channel string) KeywordIntelligence {
	return p.enhanceKeywordsWithOpenAI(ctx, kw, map[string]any{
		"analysis_type": "youtube_video",
		"title":         title,
		"channel":       channel,
	})
}

func (p *YouTubeProvider) enhanceChannelKeywords(ctx context.Context, kw KeywordIntelligence, channel string) KeywordIntelligence {
	return p.enhanceKeywordsWithOpenAI(ctx, kw, map[string]any{
		"analysis_type": "youtube_channel",
		"channel":       channel,
	})
}

func (p *YouTubeProvider) enhanceKeywordsWithOpenAI(ctx context.Context, kw KeywordIntelligence, contextPayload map[string]any) KeywordIntelligence {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return kw
	}
	original := kw
	payload := map[string]any{
		"model": strings.TrimSpace(os.Getenv("OPENAI_TEXT_MODEL")),
		"messages": []map[string]string{
			{"role": "system", "content": "Improve YouTube creator intelligence using only the provided public metadata summary. Return compact JSON only. Every phrase must be concise natural English, evidence-grounded, creator-facing, and readable on its own. Do not include legal disclaimers, financial-risk boilerplate, sponsorship text, URLs, raw n-grams, placeholders, invented metrics, invented demographics, private analytics, exact ranking terms, retention, revenue, or traffic sources. Prefer keeping strong existing phrases over replacing them."},
			{"role": "user", "content": mustJSON(map[string]any{"context": contextPayload, "keyword_intelligence": kw})},
		},
		"response_format": map[string]string{"type": "json_object"},
	}
	if payload["model"] == "" {
		payload["model"] = "gpt-4o-mini"
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return kw
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return kw
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := p.client.Do(req)
	if err != nil {
		return kw
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return kw
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&decoded); err != nil || len(decoded.Choices) == 0 {
		return kw
	}
	var enhanced struct {
		PrimaryKeywords      []string `json:"primary_keywords"`
		SecondaryKeywords    []string `json:"secondary_keywords"`
		LongTailPhrases      []string `json:"long_tail_phrases"`
		InferredSearchIntent string   `json:"inferred_search_intent"`
	}
	if err := json.Unmarshal([]byte(decoded.Choices[0].Message.Content), &enhanced); err != nil {
		return kw
	}
	if cleaned := cleanOpenAIKeywordList(enhanced.PrimaryKeywords); len(cleaned) > 0 && phraseListQuality(cleaned) >= phraseListQuality(original.PrimaryKeywords) {
		kw.PrimaryKeywords = cleaned
		kw.PrimaryTopics = kw.PrimaryKeywords
	}
	if cleaned := cleanOpenAIKeywordList(enhanced.SecondaryKeywords); len(cleaned) > 0 {
		kw.SecondaryKeywords = cleaned
		kw.SupportingTerms = kw.SecondaryKeywords
	}
	if cleaned := cleanOpenAIKeywordList(enhanced.LongTailPhrases); len(cleaned) > 0 {
		kw.LongTailPhrases = cleaned
		kw.SearchPhrases = kw.LongTailPhrases
	}
	if strings.TrimSpace(enhanced.InferredSearchIntent) != "" {
		kw.InferredSearchIntent = strings.TrimSpace(enhanced.InferredSearchIntent)
	}
	return kw
}

func cleanOpenAIKeywordList(values []string) []string {
	out := []string{}
	for _, value := range cleanStringList(values) {
		phrase := normalizeTopicPhrase(value)
		if !ValidCreatorPhrase(phrase, nil, nil, true) || nearDuplicateSelected(phrase, out) {
			continue
		}
		out = append(out, applyAcronymCasing(phrase))
		if len(out) >= 10 {
			break
		}
	}
	return out
}

func phraseListQuality(values []string) int {
	score := 0
	for _, value := range values {
		phrase := normalizeTopicPhrase(value)
		if !ValidCreatorPhrase(phrase, nil, nil, true) {
			continue
		}
		words := len(strings.Fields(phrase))
		score += 8
		if words >= 2 && words <= 5 {
			score += 4
		}
		if containsBoilerplateFragment(phrase) || isGenericAudiencePlaceholder(phrase) {
			score -= 20
		}
	}
	return score
}

func cleanStringList(values []string) []string {
	out := []string{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(cleanMetadataText(value)))
		if isUsefulTerm(value) {
			out = append(out, value)
		}
	}
	return unique(out)
}

func mustJSON(value any) string {
	b, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func ExtractYouTubeVideoID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("video_url is required")
	}
	if matched, _ := regexp.MatchString(`^[A-Za-z0-9_-]{11}$`, raw); matched {
		return raw, nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", errors.New("could not parse a YouTube video ID from that URL")
	}
	host := strings.ToLower(strings.TrimPrefix(u.Host, "www."))
	if host == "youtu.be" {
		id := strings.Trim(strings.Trim(u.Path, "/"), " ")
		if len(id) >= 11 {
			return id[:11], nil
		}
	}
	if strings.Contains(host, "youtube.com") {
		if id := u.Query().Get("v"); len(id) >= 11 {
			return id[:11], nil
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		for i, part := range parts {
			if (part == "shorts" || part == "embed" || part == "live") && i+1 < len(parts) && len(parts[i+1]) >= 11 {
				return parts[i+1][:11], nil
			}
		}
	}
	return "", errors.New("could not parse a YouTube video ID from that URL")
}

func ExtractKeywords(text string, limit int) []string {
	stop := map[string]bool{"the": true, "and": true, "for": true, "with": true, "from": true, "that": true, "this": true, "you": true, "your": true, "are": true, "was": true, "were": true, "how": true, "why": true, "what": true, "when": true, "where": true, "will": true, "can": true, "into": true, "about": true, "video": true, "shorts": true, "youtube": true, "official": true, "http": true, "https": true, "www": true, "com": true}
	counts := map[string]int{}
	var b strings.Builder
	flush := func() {
		word := strings.ToLower(strings.TrimSpace(b.String()))
		b.Reset()
		if len(word) < 3 || stop[word] {
			return
		}
		counts[word]++
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			flush()
		}
	}
	if b.Len() > 0 {
		flush()
	}
	type kv struct {
		k string
		v int
	}
	items := make([]kv, 0, len(counts))
	for k, v := range counts {
		items = append(items, kv{k, v})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].v == items[j].v {
			return items[i].k < items[j].k
		}
		return items[i].v > items[j].v
	})
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	out := make([]string, 0, limit)
	for _, item := range items[:limit] {
		out = append(out, item.k)
	}
	return out
}

func youtubeAPIURL(resource string, params map[string]string) string {
	u := url.URL{Scheme: "https", Host: "www.googleapis.com", Path: "/youtube/v3/" + resource}
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func apiURLWithoutKey(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if q.Has("key") {
		q.Set("key", "REDACTED")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func parseUintPtr(raw string) *uint64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return nil
	}
	return &v
}

func firstKeyword(keywords []string) string {
	if len(keywords) == 0 {
		return ""
	}
	return keywords[0]
}

func inferNiche(keywords []string, categoryID string) string {
	if len(keywords) == 0 {
		if categoryID != "" {
			return "inferred YouTube category " + categoryID
		}
		return "inferred general creator content"
	}
	return "inferred niche around " + strings.Join(topN(keywords, 3), ", ")
}

func inferAngle(title string, keywords []string) string {
	if len(keywords) == 0 {
		return "Inferred from the title structure and public metadata."
	}
	return "Create a clearer, faster version focused on " + strings.Join(topN(keywords, 3), ", ") + "."
}

func analyzeHook(title string) string {
	title = strings.TrimSpace(title)
	switch {
	case strings.Contains(title, "?"):
		return "Question-led hook; likely designed to open a curiosity gap."
	case strings.Contains(strings.ToLower(title), "how"):
		return "How-to hook; likely promises a practical outcome."
	case strings.Contains(title, "!"):
		return "High-emotion hook; likely optimized for immediacy."
	case len(title) > 70:
		return "Long-form title hook; front-load the strongest phrase for Shorts/Reels."
	default:
		return "Direct title hook; remake angles should make the payoff explicit in the first line."
	}
}

func analyzeTitleStructure(title string) string {
	words := len(strings.Fields(title))
	return fmt.Sprintf("Public title has %d words. This is an inferred structure read, not exact SEO ranking data.", words)
}

func analyzeDescriptionHashtags(description string) string {
	hashtags := regexp.MustCompile(`#\w+`).FindAllString(description, -1)
	if len(hashtags) == 0 {
		return "No public hashtags detected in the description metadata."
	}
	if len(hashtags) > 8 {
		hashtags = hashtags[:8]
	}
	return "Public description hashtags detected: " + strings.Join(hashtags, " ")
}

func performanceSignals(publishedAt string, views, likes, comments *uint64, now func() time.Time) map[string]any {
	signals := map[string]any{}
	var viewsPerDay float64
	if t, err := time.Parse(time.RFC3339, publishedAt); err == nil {
		ageDays := now().Sub(t).Hours() / 24
		if ageDays < 1 {
			ageDays = 1
		}
		signals["age_days"] = ageDays
		if views != nil {
			viewsPerDay = float64(*views) / ageDays
			signals["views_per_day"] = viewsPerDay
		}
	}
	if views != nil && *views > 0 {
		var interactions uint64
		if likes != nil {
			interactions += *likes
			signals["likes_per_1000_views"] = float64(*likes) / float64(*views) * 1000
		}
		if comments != nil {
			interactions += *comments
			signals["comments_per_1000_views"] = float64(*comments) / float64(*views) * 1000
		}
		signals["engagement_rate"] = float64(interactions) / float64(*views)
	}
	signals["velocity_label"] = velocityLabel(viewsPerDay)
	return signals
}

func velocityLabel(viewsPerDay float64) string {
	switch {
	case viewsPerDay >= 1000000:
		return "very_high"
	case viewsPerDay >= 100000:
		return "high"
	case viewsPerDay >= 10000:
		return "medium"
	default:
		return "low"
	}
}

func suggestedAngles(keywords []string, title string) []string {
	base := topN(keywords, 3)
	if len(base) == 0 {
		return []string{"Make a shorter version with the payoff in the first two seconds.", "Create a contrarian response using verified public facts.", "Turn the title premise into a step-by-step short."}
	}
	return []string{
		"Explain " + base[0] + " for beginners in 30 seconds.",
		"Compare " + strings.Join(base, " vs ") + " with a stronger first-line hook.",
		"Turn the public topic into a checklist, mistake list, or quick reaction format.",
	}
}

func topN(items []string, n int) []string {
	if n > len(items) {
		n = len(items)
	}
	if n < 0 {
		n = 0
	}
	out := make([]string, n)
	copy(out, items[:n])
	return out
}

func titlesText(videos []ChannelVideoSummary) string {
	var titles []string
	for _, v := range videos {
		titles = append(titles, v.Title)
	}
	return strings.Join(titles, " ")
}

func titlePatterns(videos []ChannelVideoSummary) []string {
	patterns := []string{}
	for _, video := range videos {
		title := strings.TrimSpace(video.Title)
		if title == "" {
			continue
		}
		if strings.Contains(title, "?") {
			patterns = append(patterns, "Question-led titles")
		}
		if regexp.MustCompile(`\d`).MatchString(title) {
			patterns = append(patterns, "Number-led titles")
		}
		if strings.Contains(strings.ToLower(title), "how") {
			patterns = append(patterns, "How-to titles")
		}
	}
	if len(patterns) == 0 {
		return []string{"Direct metadata titles; no dominant public pattern detected."}
	}
	return unique(topN(patterns, 5))
}

func uploadFrequency(videos []ChannelVideoSummary, now func() time.Time) string {
	if len(videos) < 2 {
		return "Not enough recent public videos to estimate upload frequency."
	}
	var oldest time.Time
	for _, video := range videos {
		t, err := time.Parse(time.RFC3339, video.PublishedAt)
		if err != nil {
			continue
		}
		if oldest.IsZero() || t.Before(oldest) {
			oldest = t
		}
	}
	if oldest.IsZero() {
		return "Not enough dated public videos to estimate upload frequency."
	}
	days := now().Sub(oldest).Hours() / 24
	if days < 1 {
		days = 1
	}
	return fmt.Sprintf("Approximately %.1f public uploads per week based on the latest %d videos.", float64(len(videos))/(days/7), len(videos))
}

func channelSignals(subscribersRaw string, videos []ChannelVideoSummary) (map[string]any, *float64) {
	var views []float64
	for _, video := range videos {
		if video.Views != nil {
			views = append(views, float64(*video.Views))
		}
	}
	out := map[string]any{"sample_size": len(views)}
	if len(views) == 0 {
		return out, nil
	}
	sort.Float64s(views)
	sum := 0.0
	for _, v := range views {
		sum += v
	}
	avg := sum / float64(len(views))
	out["min_views"] = views[0]
	out["median_views"] = views[len(views)/2]
	out["max_views"] = views[len(views)-1]
	out["average_views"] = avg
	subs := parseUintPtr(subscribersRaw)
	if subs == nil || *subs == 0 {
		return out, nil
	}
	ratio := avg / float64(*subs)
	return out, &ratio
}

func likelyStrategy(pillars []string, videos []ChannelVideoSummary) string {
	if len(pillars) == 0 {
		return "Insufficient public metadata to infer a clear strategy."
	}
	return "Likely strategy: repeat public topics around " + strings.Join(topN(pillars, 3), ", ") + " with titles optimized for quick recognition. This is inferred, not private channel analytics."
}

func opportunities(pillars []string) []string {
	if len(pillars) == 0 {
		return []string{"Connect more official sources or analyze more videos to identify gaps."}
	}
	return []string{"Create beginner explainers around " + pillars[0] + ".", "Test a contrarian or mistake-based hook for the strongest repeated topic.", "Localize the best pillar for your target country and language."}
}

func contentIdeas(pillars []string) []string {
	if len(pillars) == 0 {
		return []string{"Analyze a channel with public recent videos to generate evidence-backed ideas."}
	}
	ideas := []string{}
	for i := 0; i < 10; i++ {
		kw := pillars[i%len(pillars)]
		ideas = append(ideas, fmt.Sprintf("%s: %s in 30 seconds", titleIdeaPrefix(i), kw))
	}
	return ideas
}

func titleIdeaPrefix(i int) string {
	prefixes := []string{"Beginner guide", "Mistakes", "Before you try", "Fast checklist", "Myth vs reality", "Hidden signal", "Creator playbook", "Explained simply", "What changed", "Next move"}
	return prefixes[i%len(prefixes)]
}

func unique(items []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range items {
		if seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
