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

const channelAnalysisSchemaVersion = "channel_analysis_v3_sample_separation"

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
	MonthlyRevenueRunRate   RevenueEstimate        `json:"monthly_revenue_run_rate,omitempty"`
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
	WeekType  string            `json:"week_type,omitempty"`
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
	SampledVideoCount            int      `json:"sampled_video_count"`
	RecentSampleCount            int      `json:"recent_sample_count,omitempty"`
	PerformanceSampleCount       int      `json:"performance_sample_count,omitempty"`
	SampleStart                  string   `json:"sample_start,omitempty"`
	SampleEnd                    string   `json:"sample_end,omitempty"`
	RecentSampleStart            string   `json:"recent_sample_start,omitempty"`
	RecentSampleEnd              string   `json:"recent_sample_end,omitempty"`
	PerformanceSampleStart       string   `json:"performance_sample_start,omitempty"`
	PerformanceSampleEnd         string   `json:"performance_sample_end,omitempty"`
	SampleDateSpanDays           float64  `json:"sample_date_span_days,omitempty"`
	UploadsPerMonth              float64  `json:"uploads_per_month,omitempty"`
	UploadsPerWeek               float64  `json:"uploads_per_week,omitempty"`
	MedianUploadIntervalDays     float64  `json:"median_upload_interval_days,omitempty"`
	CadenceAvailable             bool     `json:"cadence_available"`
	CadenceMethodology           string   `json:"cadence_methodology,omitempty"`
	RecommendedUploadsNext30Days int      `json:"recommended_uploads_next_30_days,omitempty"`
	CadenceConfidence            string   `json:"cadence_confidence,omitempty"`
	ProviderAvailability         string   `json:"provider_availability"`
	HiddenMetricNotes            []string `json:"hidden_metric_notes,omitempty"`
	ScoringMethodology           []string `json:"scoring_methodology,omitempty"`
	TopicMethodology             []string `json:"topic_methodology,omitempty"`
	RevenueMethodology           []string `json:"revenue_methodology,omitempty"`
	ClassificationRules          []string `json:"classification_rules,omitempty"`
	AnalysisTimestamp            string   `json:"analysis_timestamp"`
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
	recentVideos, _ := p.fetchChannelUploads(ctx, channel.ContentDetails.RelatedPlaylists.Uploads, 25)
	if len(recentVideos) == 0 {
		recentVideos, _ = p.fetchChannelVideos(ctx, channelID, "date", 25)
	}
	topVideos, _ := p.fetchChannelVideos(ctx, channelID, "viewCount", 25)
	recentVideos = enrichChannelVideos(recentVideos, p.now)
	topVideos = enrichChannelVideos(topVideos, p.now)
	recentUploadSample := recentUploadSample(recentVideos, 20)
	performanceSample := mergeChannelVideos(recentUploadSample, topVideos)
	performanceSample = enrichChannelVideos(performanceSample, p.now)
	handle := firstNonEmpty(channel.Snippet.CustomURL, resolved.Handle)
	identity := buildChannelIdentityContext(channel.Snippet.Title, handle, channel.Snippet.Description)
	keywordIntel := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title:             channel.Snippet.Title,
		Description:       cleanMetadataText(channel.Snippet.Description),
		ChannelTitle:      channel.Snippet.Title,
		TopicDetails:      channel.TopicDetails.TopicCategories,
		RecentVideoTitles: videoTitles(performanceSample),
	})
	keywordIntel = sanitizeChannelKeywordIntelligence(keywordIntel, performanceSample, identity)
	keywordIntel = p.enhanceChannelKeywords(ctx, keywordIntel, channel.Snippet.Title)
	keywordIntel = sanitizeChannelKeywordIntelligence(keywordIntel, performanceSample, identity)
	keywords := append(append([]string{}, keywordIntel.PrimaryKeywords...), keywordIntel.SecondaryKeywords...)
	nicheAnalysis := ClassifyNiche(KeywordExtractionInput{
		Title:             channel.Snippet.Title,
		Description:       channel.Snippet.Description,
		ChannelTitle:      channel.Snippet.Title,
		TopicDetails:      channel.TopicDetails.TopicCategories,
		RecentVideoTitles: videoTitles(performanceSample),
	}, keywordIntel)
	pillarObjects := buildChannelPillars(keywordIntel, performanceSample, identity)
	pillars := topPillarNames(pillarObjects, 6)
	keywords = sanitizeChannelKeywordList(keywords, performanceSample, identity, 12)
	assignVideoPillars(performanceSample, pillarObjects)
	assignVideoPillars(recentUploadSample, pillarObjects)
	assignVideoPillars(topVideos, pillarObjects)
	viewDistribution, ratio := channelSignals(channel.Statistics.SubscriberCount, performanceSample)
	performanceDistribution := PerformanceDistribution(performanceSample)
	patterns := titlePatterns(performanceSample)
	formats := formatPatternsFromVideos(performanceSample)
	channelOpps := ChannelOpportunities(nicheAnalysis, pillars, keywordIntel)
	ideas := SuggestedChannelIdeas(nicheAnalysis, keywordIntel, pillars)
	shortIdeas := SuggestedShortClipIdeas(nicheAnalysis, keywordIntel, pillars)
	dimensions, opportunityScore, analysisConfidence := buildChannelScoreModel(channel, recentUploadSample, performanceSample, pillarObjects, keywordIntel, nicheAnalysis, p.now)
	performanceMetrics := channelPerformanceMetrics(recentUploadSample, performanceSample, channel.Statistics.SubscriberCount, p.now)
	revenueEstimate := estimateChannelRevenue(performanceSample, nicheAnalysis)
	monthlyRunRate := estimateChannelMonthlyRunRate(recentUploadSample, nicheAnalysis, p.now)
	charts := buildChannelCharts(recentUploadSample, performanceSample, pillarObjects)
	videoGroups := buildChannelVideoGroups(performanceSample)
	packaging := buildChannelPackaging(performanceSample, pillarObjects)
	growthOpps := buildChannelGrowthOpportunities(nicheAnalysis, pillarObjects, videoGroups, packaging)
	growthOpps = sanitizeChannelGrowthOpportunities(growthOpps, pillarObjects, performanceSample, identity)
	contentPlan := buildChannelContentPlan(recentUploadSample, performanceSample, pillarObjects, growthOpps, nicheAnalysis, p.now)
	details := buildChannelAnalysisDetails(recentUploadSample, performanceSample, channel, p.now)
	canonicalURL := "https://www.youtube.com/channel/" + channelID
	cacheKey := "youtube_channel:" + channelAnalysisSchemaVersion + ":" + channelID
	score := float64(opportunityScore.Score)
	reason := opportunityScore.Explanation
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
		RecentVideos:        recentUploadSample,
		TopVideosSummary:    topNChannelVideos(topVideos, 10),
		ChannelSnapshot: map[string]any{
			"subscribers":           parseUintPtr(channel.Statistics.SubscriberCount),
			"total_views":           parseUintPtr(channel.Statistics.ViewCount),
			"video_count":           parseUintPtr(channel.Statistics.VideoCount),
			"country":               country,
			"channel_age":           channelAge(channel.Snippet.PublishedAt, p.now),
			"created_at":            channel.Snippet.PublishedAt,
			"last_public_upload_at": latestVideoDate(recentUploadSample),
			"recent_upload_cadence": uploadFrequency(recentUploadSample, p.now),
			"primary_format":        primaryChannelFormat(performanceSample),
			"dominant_topic":        firstPhrase(pillars),
		},
		ChannelNiche:            nicheAnalysis.PrimaryNiche,
		NicheAnalysis:           nicheAnalysis,
		ContentPillars:          pillars,
		KeywordIntelligence:     keywordIntel,
		KeywordClusters:         ChannelKeywordClusters(keywordIntel, performanceSample),
		FormatPatterns:          formats,
		TitlePatterns:           patterns,
		PerformanceDistribution: performanceDistribution,
		UploadFrequency:         uploadFrequency(recentUploadSample, p.now),
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
		MonthlyRevenueRunRate:   monthlyRunRate,
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
			"Revenue ranges are public-view advertising proxies from sampled videos and conservative RPM assumptions, not actual YouTube earnings.",
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
	result.Opportunities = sanitizeChannelOpportunityStrings(result.Opportunities, pillarObjects, performanceSample, identity)
	result.SuggestedContentIdeas = sanitizeChannelIdeaStrings(result.SuggestedContentIdeas, pillarObjects, performanceSample, identity, 10)
	result.SuggestedShortClipIdeas = sanitizeChannelIdeaStrings(result.SuggestedShortClipIdeas, pillarObjects, performanceSample, identity, 10)
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

func recentUploadSample(videos []ChannelVideoSummary, limit int) []ChannelVideoSummary {
	dated := []ChannelVideoSummary{}
	seen := map[string]bool{}
	for _, video := range videos {
		if video.VideoID == "" || seen[video.VideoID] || video.PublishedAt == "" {
			continue
		}
		if _, err := time.Parse(time.RFC3339, video.PublishedAt); err != nil {
			continue
		}
		seen[video.VideoID] = true
		dated = append(dated, video)
	}
	sort.Slice(dated, func(i, j int) bool { return dated[i].PublishedAt > dated[j].PublishedAt })
	if limit > 0 && len(dated) > limit {
		dated = dated[:limit]
	}
	sort.Slice(dated, func(i, j int) bool { return dated[i].PublishedAt < dated[j].PublishedAt })
	return dated
}

func assignVideoPillars(videos []ChannelVideoSummary, pillars []ChannelContentPillar) {
	for i := range videos {
		videos[i].Pillar = matchPillar(videos[i], pillars)
	}
}

func matchPillar(video ChannelVideoSummary, pillars []ChannelContentPillar) string {
	titleTokens := tokenSet(tokenizeUseful(video.Title, map[string]bool{}))
	for _, pillar := range pillars {
		if channelPhraseSupportedByTitle(pillar.Name, titleTokens) {
			return pillar.Name
		}
	}
	return ""
}

type channelIdentityContext struct {
	Names       map[string]bool
	Tokens      map[string]bool
	Description map[string]bool
}

func buildChannelIdentityContext(title, handle, description string) channelIdentityContext {
	ctx := channelIdentityContext{Names: map[string]bool{}, Tokens: map[string]bool{}, Description: map[string]bool{}}
	addName := func(raw string) {
		cleaned := normalizeTopicPhrase(strings.TrimPrefix(strings.ToLower(raw), "@"))
		if cleaned == "" {
			return
		}
		ctx.Names[cleaned] = true
		for _, token := range tokenizeUseful(cleaned, map[string]bool{}) {
			ctx.Tokens[token] = true
		}
	}
	addName(title)
	addName(handle)
	addName(strings.ReplaceAll(handle, "-", " "))
	for _, token := range tokenizeUseful(description, map[string]bool{}) {
		ctx.Description[token] = true
	}
	for _, phrase := range extractCreatorNameVariants(description) {
		addName(phrase)
	}
	return ctx
}

func extractCreatorNameVariants(description string) []string {
	cleaned := cleanMetadataText(description)
	out := []string{}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:hosted by|created by|creator|founder|by)\s+([A-Z][A-Za-z]+(?:\s+[A-Z][A-Za-z]+){0,2})\b`),
		regexp.MustCompile(`(?i)\b([A-Z][A-Za-z]+(?:\s+[A-Z][A-Za-z]+){1,2})\s+(?:is|,)\s+(?:a|an)?\s*(?:youtuber|creator|internet personality|reviewer|host)\b`),
	}
	for _, pattern := range patterns {
		for _, match := range pattern.FindAllStringSubmatch(cleaned, -1) {
			if len(match) > 1 {
				out = append(out, match[1])
			}
		}
	}
	return out
}

func sanitizeChannelKeywordIntelligence(kw KeywordIntelligence, videos []ChannelVideoSummary, identity channelIdentityContext) KeywordIntelligence {
	kw.PrimaryKeywords = sanitizeChannelKeywordList(kw.PrimaryKeywords, videos, identity, 6)
	kw.SecondaryKeywords = sanitizeChannelKeywordList(kw.SecondaryKeywords, videos, identity, 10)
	kw.LongTailPhrases = sanitizeChannelKeywordList(kw.LongTailPhrases, videos, identity, 8)
	kw.PrimaryTopics = kw.PrimaryKeywords
	kw.SupportingTerms = kw.SecondaryKeywords
	kw.SearchPhrases = kw.LongTailPhrases
	return kw
}

func sanitizeChannelKeywordList(values []string, videos []ChannelVideoSummary, identity channelIdentityContext, limit int) []string {
	_, titleTokens := channelEvidenceTokens(videos)
	out := []string{}
	for _, value := range values {
		phrase := normalizeChannelPillarName(value)
		if !validChannelTopicPhrase(phrase, titleTokens, identity) || nearDuplicateSelected(phrase, out) {
			continue
		}
		out = append(out, applyAcronymCasing(phrase))
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func buildChannelPillars(kw KeywordIntelligence, videos []ChannelVideoSummary, identity channelIdentityContext) []ChannelContentPillar {
	_, titleTokens := channelEvidenceTokens(videos)
	seed := channelPillarSeeds(kw, videos, identity)
	candidates := []string{}
	for _, raw := range seed {
		name := normalizeChannelPillarName(raw)
		if !validChannelTopicPhrase(name, titleTokens, identity) || nearDuplicateSelected(name, candidates) {
			continue
		}
		candidates = append(candidates, name)
	}
	if len(candidates) == 0 {
		return []ChannelContentPillar{}
	}
	total := len(videos)
	out := []ChannelContentPillar{}
	for _, name := range candidates {
		matches := []ChannelVideoSummary{}
		for _, video := range videos {
			if channelPhraseMatchesVideo(name, video) {
				matches = append(matches, video)
			}
		}
		if len(matches) < 2 {
			continue
		}
		if !durableChannelPillar(name, matches, total) {
			continue
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
		if share <= 0 || median == nil || strongest == nil {
			continue
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
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].UploadCount == out[j].UploadCount && out[i].MedianViews != nil && out[j].MedianViews != nil {
			return *out[i].MedianViews > *out[j].MedianViews
		}
		return out[i].UploadCount > out[j].UploadCount
	})
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

func channelPillarSeeds(kw KeywordIntelligence, videos []ChannelVideoSummary, identity channelIdentityContext) []string {
	seeds := append(append([]string{}, kw.PrimaryKeywords...), kw.SecondaryKeywords...)
	seeds = append(seeds, kw.LongTailPhrases...)
	seeds = append(channelThematicPillarSeeds(videos), seeds...)
	freq := map[string]int{}
	for _, video := range videos {
		tokens := tokenizeUseful(video.Title, identity.Tokens)
		for n := 2; n <= 4; n++ {
			for _, phrase := range ngrams(tokens, n) {
				name := normalizeChannelPillarName(phrase)
				if name != "" {
					freq[name]++
				}
			}
		}
	}
	type item struct {
		phrase string
		count  int
	}
	items := []item{}
	for phrase, count := range freq {
		if count > 0 {
			items = append(items, item{phrase: phrase, count: count})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count == items[j].count {
			return len(items[i].phrase) < len(items[j].phrase)
		}
		return items[i].count > items[j].count
	})
	for _, item := range items {
		seeds = append(seeds, item.phrase)
	}
	return seeds
}

func channelThematicPillarSeeds(videos []ChannelVideoSummary) []string {
	counts := map[string]int{}
	add := func(name string, ok bool) {
		if ok {
			counts[name]++
		}
	}
	for _, video := range videos {
		lower := normalizeTopicPhrase(video.Title)
		hasPhone := containsAnyNormalized(lower, []string{"phone", "iphone", "android", "smartphone"})
		hasReview := containsAnyNormalized(lower, []string{"review", "test", "hands on", "camera"})
		add("smartphone reviews", hasPhone && hasReview)
		add("camera comparisons", strings.Contains(lower, "camera") && containsAnyNormalized(lower, []string{"test", "comparison", "versus", "vs", "review"}))
		add("Apple product reviews", containsAnyNormalized(lower, []string{"iphone", "ios", "macbook", "apple", "airpod"}) && containsAnyNormalized(lower, []string{"review", "hands on", "feature", "test"}))
		add("electric vehicles", containsAnyNormalized(lower, []string{"electric car", "electric vehicle", "tesla", "robotaxi", "ev"}) && containsAnyNormalized(lower, []string{"review", "test", "explained", "tech", "driving", "drive", "autonomous", "vehicle", "car"}))
		add("AI and computer science", containsAnyNormalized(lower, []string{"ai", "algorithm", "quantum", "turing", "computer", "coding", "code"}))
		add("AI creator workflows", containsAnyNormalized(lower, []string{"ai", "automation", "workflow", "system"}) && containsAnyNormalized(lower, []string{"creator", "youtube", "video"}))
		add("programming concepts", containsAnyNormalized(lower, []string{"code", "coding", "programming", "compiler", "database"}))
		add("challenge videos", containsAnyNormalized(lower, []string{"challenge", "last to", "last leave", "circle", "wins", "survive", "survival"}))
		add("large scale giveaways", containsAnyNormalized(lower, []string{"money", "giveaway", "prize", "win", "wins"}) && containsAnyNormalized(lower, []string{"challenge", "last", "circle", "people", "days"}))
	}
	out := []string{}
	for name, count := range counts {
		if count >= 2 {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func containsAnyNormalized(text string, needles []string) bool {
	text = normalizeTopicPhrase(text)
	for _, needle := range needles {
		if strings.Contains(text, normalizeTopicPhrase(needle)) {
			return true
		}
	}
	return false
}

func channelEvidenceTokens(videos []ChannelVideoSummary) (map[string]bool, map[string]bool) {
	all := map[string]bool{}
	title := map[string]bool{}
	for _, video := range videos {
		for _, token := range tokenizeUseful(video.Title, map[string]bool{}) {
			all[token] = true
			title[token] = true
		}
		for _, token := range tokenizeUseful(video.Description, map[string]bool{}) {
			all[token] = true
		}
	}
	return all, title
}

func normalizeChannelPillarName(value string) string {
	phrase := normalizeTopicPhrase(value)
	replacements := map[string]string{
		"consumer electronic":     "consumer electronics",
		"smartphone review":       "smartphone reviews",
		"phone review":            "phone reviews",
		"electric vehicle":        "electric vehicles",
		"electric vehicle review": "electric vehicle reviews",
		"car review":              "car reviews",
		"camera comparison":       "camera comparisons",
		"camera test":             "camera tests",
		"product review":          "product reviews",
		"challenge video":         "challenge videos",
		"large scale giveaway":    "large scale giveaways",
	}
	if replacement, ok := replacements[phrase]; ok {
		return replacement
	}
	return phrase
}

func validChannelTopicPhrase(phrase string, titleTokens map[string]bool, identity channelIdentityContext) bool {
	phrase = normalizeChannelPillarName(phrase)
	recognized := recognizedChannelPillar(phrase)
	if recognized && !channelBiographyFragment(phrase) && !channelNounPile(phrase) {
		return true
	}
	if !ValidCreatorPhrase(phrase, nil, nil, true) || (!recognized && channelIdentityPhrase(phrase, identity)) || channelBiographyFragment(phrase) || channelNounPile(phrase) {
		return false
	}
	words := strings.Fields(phrase)
	if len(words) < 2 || len(words) > 5 {
		return false
	}
	supported := 0
	for _, word := range words {
		if titleTokens[word] || isKnownAcronym(word) {
			supported++
		}
	}
	if supported == 0 && recognized {
		return true
	}
	return supported >= minInt(2, len(words))
}

func recognizedChannelPillar(phrase string) bool {
	switch normalizeTopicPhrase(phrase) {
	case "smartphone review", "smartphone reviews", "camera comparison", "camera comparisons", "apple product review", "apple product reviews", "electric vehicle", "electric vehicles", "ai and computer science", "ai creator workflow", "ai creator workflows", "programming concept", "programming concepts", "challenge video", "challenge videos", "large scale giveaway", "large scale giveaways":
		return true
	default:
		return false
	}
}

func channelIdentityPhrase(phrase string, identity channelIdentityContext) bool {
	phrase = normalizeTopicPhrase(phrase)
	if identity.Names[phrase] {
		return true
	}
	words := strings.Fields(phrase)
	if len(words) == 0 {
		return true
	}
	identityHits := 0
	for _, word := range words {
		if identity.Tokens[word] {
			identityHits++
		}
	}
	return identityHits > 0
}

func channelBiographyFragment(phrase string) bool {
	lower := normalizeTopicPhrase(phrase)
	for _, term := range []string{"youtuber", "you tuber", "internet personality", "tech head", "geek", "host", "reviewer", "influencer"} {
		if strings.Contains(lower, term) {
			return true
		}
	}
	words := strings.Fields(lower)
	if len(words) <= 2 && (strings.Contains(lower, "creator") || strings.Contains(lower, "channel")) {
		return true
	}
	for _, term := range []string{"cooked", "driving", "reach", "leave", "wins", "win", "work"} {
		if len(words) <= 3 && containsAnyNormalized(lower, []string{term}) {
			return true
		}
	}
	return false
}

func channelNounPile(phrase string) bool {
	words := strings.Fields(normalizeTopicPhrase(phrase))
	if len(words) < 4 {
		return false
	}
	generic := 0
	for _, word := range words {
		if lowInformationWords[word] || word == "tech" || word == "technology" || word == "consumer" || word == "electronic" || word == "electronics" || word == "content" || word == "video" {
			generic++
		}
	}
	return generic >= len(words)-1
}

func channelPhraseSupportedByTitle(phrase string, titleTokens map[string]bool) bool {
	words := strings.Fields(normalizeTopicPhrase(phrase))
	if len(words) == 0 {
		return false
	}
	hits := 0
	for _, word := range words {
		if titleTokens[word] || singularPluralTokenHit(word, titleTokens) {
			hits++
		}
	}
	if len(words) <= 2 {
		return hits == len(words)
	}
	return hits >= len(words)-1
}

func channelPhraseMatchesVideo(phrase string, video ChannelVideoSummary) bool {
	normalized := normalizeTopicPhrase(phrase)
	title := normalizeTopicPhrase(video.Title)
	switch normalized {
	case "smartphone review", "smartphone reviews":
		return containsAnyNormalized(title, []string{"phone", "iphone", "android", "smartphone"}) && containsAnyNormalized(title, []string{"review", "test", "hands on", "camera"})
	case "camera comparison", "camera comparisons":
		return strings.Contains(title, "camera") && containsAnyNormalized(title, []string{"test", "comparison", "versus", "vs", "review"})
	case "apple product review", "apple product reviews":
		return containsAnyNormalized(title, []string{"iphone", "ios", "macbook", "apple", "airpod"}) && containsAnyNormalized(title, []string{"review", "hands on", "feature", "test"})
	case "electric vehicle", "electric vehicles":
		return containsAnyNormalized(title, []string{"electric car", "electric vehicle", "tesla", "robotaxi", "ev"}) && containsAnyNormalized(title, []string{"review", "test", "explained", "tech", "driving", "drive", "autonomous", "vehicle", "car"})
	case "ai and computer science":
		return containsAnyNormalized(title, []string{"ai", "algorithm", "quantum", "turing", "computer", "coding", "code"})
	case "ai creator workflow", "ai creator workflows":
		return containsAnyNormalized(title, []string{"ai", "automation", "workflow", "system"}) && containsAnyNormalized(title, []string{"creator", "youtube", "video"})
	case "programming concept", "programming concepts":
		return containsAnyNormalized(title, []string{"code", "coding", "programming", "compiler", "database"})
	case "challenge video", "challenge videos":
		return containsAnyNormalized(title, []string{"challenge", "last to", "last leave", "circle", "wins", "survive", "survival"})
	case "large scale giveaway", "large scale giveaways":
		return containsAnyNormalized(title, []string{"money", "giveaway", "prize", "win", "wins"}) && containsAnyNormalized(title, []string{"challenge", "last", "circle", "people", "days"})
	default:
		return channelPhraseSupportedByTitle(phrase, tokenSet(tokenizeUseful(video.Title, map[string]bool{})))
	}
}

func durableChannelPillar(name string, matches []ChannelVideoSummary, total int) bool {
	if len(matches) < 2 || total <= 0 {
		return false
	}
	dates := map[string]bool{}
	for _, video := range matches {
		if t, err := time.Parse(time.RFC3339, video.PublishedAt); err == nil {
			dates[t.Format("2006-01-02")] = true
		}
	}
	if len(dates) < 2 {
		return false
	}
	share := float64(len(matches)) / float64(total)
	normalized := normalizeChannelPillarName(name)
	switch normalized {
	case "challenge videos", "large scale giveaways":
		return len(matches) >= 2 && share >= 0.06
	case "electric vehicles":
		if len(matches) < 3 || share < 0.10 {
			return false
		}
		editorial := 0
		for _, video := range matches {
			title := normalizeTopicPhrase(video.Title)
			if containsAnyNormalized(title, []string{"review", "test", "explained", "tech", "driving", "drive", "autonomous", "vehicle", "car"}) &&
				!containsAnyNormalized(title, []string{"giveaway", "wins", "prize", "challenge"}) {
				editorial++
			}
		}
		return editorial >= 2
	default:
		return len(matches) >= 2 && share >= 0.06
	}
}

func singularPluralTokenHit(word string, tokens map[string]bool) bool {
	if tokens[word] {
		return true
	}
	if strings.HasSuffix(word, "s") && tokens[strings.TrimSuffix(word, "s")] {
		return true
	}
	return tokens[word+"s"]
}

func channelSingleUploadCanQualify(name string, video ChannelVideoSummary) bool {
	titleTokens := tokenSet(tokenizeUseful(video.Title, map[string]bool{}))
	return channelPhraseSupportedByTitle(name, titleTokens) && video.Views != nil && *video.Views > 0
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

func buildChannelScoreModel(channel youtubeChannelItem, recentUploads, performanceSample []ChannelVideoSummary, pillars []ChannelContentPillar, kw KeywordIntelligence, niche NicheAnalysis, now func() time.Time) ([]ScoreDimension, ScoreDimension, ScoreDimension) {
	metrics := channelRawMetrics(recentUploads, performanceSample, channel.Statistics.SubscriberCount, now)
	sampleSize := len(videoViewFloats(performanceSample))
	topicClarity := clampInt(25 + len(pillars)*10 + int(niche.Confidence*14))
	if len(kw.PrimaryKeywords) >= 4 {
		topicClarity += 8
	}
	if len(pillars) == 0 {
		topicClarity = minInt(topicClarity, 42)
	}
	packaging := scorePackaging(performanceSample)
	consistency := scoreConsistency(metrics.uploadsPerMonth, metrics.daysSinceLastUpload)
	repeatability := scoreRepeatability(performanceSample, pillars)
	momentum := scoreMomentum(recentUploads)
	formatEfficiency := scoreFormatEfficiency(performanceSample)
	monetisation := monetisationPotentialScore(niche, primaryChannelFormat(performanceSample), parseUintPtr(channel.Statistics.ViewCount))
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
	if visibleEngagementCount(performanceSample) >= 6 {
		evidence += 10
	}
	evidence = clampInt(evidence)
	dims := []ScoreDimension{
		scoreDimension("topic_clarity", "Topic clarity", topicClarity, "Measures whether titles, channel description, topics, and repeated phrases point to a coherent creator lane.", firstPhrase(kw.PrimaryKeywords)),
		scoreDimension("packaging_strength", "Packaging strength", packaging, "Title structure score from specificity, numbers, questions, repeated winning forms, and avoidable repetition. Thumbnail claims are limited to URL availability.", ""),
		scoreDimension("publishing_consistency", "Publishing consistency", consistency, "Uses only the recent upload sample: median interval and days since last public upload.", metrics.cadenceEvidence),
		scoreDimension("content_repeatability", "Content repeatability", repeatability, "Rewards repeatable pillars with multiple uploads and resists one-off outlier dependence.", fmt.Sprintf("%d pillars", len(pillars))),
		scoreDimension("format_efficiency", "Format efficiency", formatEfficiency, "Compares median views per detected format only when duration classification is available.", primaryChannelFormat(performanceSample)),
		scoreDimension("recent_momentum", "Recent momentum", momentum, "Uses only recent-upload views per day and share of recent uploads above their median. It does not infer subscriber growth history.", ""),
		scoreDimension("monetisation_potential", "Monetisation potential", monetisation, "Estimated from public views, detected format, and broad niche RPM assumptions. It is not actual revenue.", niche.PrimaryNiche),
		scoreDimension("evidence_quality", "Evidence quality", evidence, "Caps the overall score when public evidence is incomplete or visible engagement counts are hidden.", fmt.Sprintf("%d sampled videos", sampleSize)),
	}
	weighted := float64(topicClarity)*0.14 + float64(packaging)*0.12 + float64(consistency)*0.14 + float64(repeatability)*0.14 + float64(formatEfficiency)*0.10 + float64(momentum)*0.14 + float64(monetisation)*0.10 + float64(evidence)*0.12
	if len(pillars) == 0 && weighted > 58 {
		weighted = 58
	}
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
	cadenceAvailable    bool
	cadenceEvidence     string
}

func channelRawMetrics(recentUploads, performanceSample []ChannelVideoSummary, subscribersRaw string, now func() time.Time) channelMetrics {
	views := videoViewFloats(performanceSample)
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
	likesRate, commentsRate := engagementRates(performanceSample)
	metrics.likesPer1000 = likesRate
	metrics.commentsPer1000 = commentsRate
	metrics.medianDuration = medianDurationSeconds(performanceSample)
	cadence := sampledCadenceSummary(recentUploads, now)
	metrics.uploadsPerMonth, metrics.daysSinceLastUpload = cadence.UploadsPerMonth, cadence.DaysSinceLast
	metrics.cadenceAvailable = cadence.Available
	metrics.cadenceEvidence = cadence.Methodology
	if subs := parseUintPtr(subscribersRaw); subs != nil && *subs > 0 && metrics.medianViews > 0 {
		ratio := metrics.medianViews / float64(*subs)
		metrics.subscriberViewRatio = &ratio
	}
	return metrics
}

func channelPerformanceMetrics(recentUploads, performanceSample []ChannelVideoSummary, subscribersRaw string, now func() time.Time) []PerformanceMetric {
	m := channelRawMetrics(recentUploads, performanceSample, subscribersRaw, now)
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
		out = append(out, PerformanceMetric{ID: "uploads_per_month", Label: "Uploads per month", Value: formatFloat(m.uploadsPerMonth, 1), RawValue: m.uploadsPerMonth, Score: scoreConsistency(m.uploadsPerMonth, m.daysSinceLastUpload), Explanation: "Median interval from the recent upload sample only; historical top videos are excluded."})
	} else if !m.cadenceAvailable {
		out = append(out, PerformanceMetric{ID: "uploads_per_month", Label: "Uploads per month", Value: "Insufficient recent upload data", RawValue: 0, Score: 25, Explanation: "At least two dated recent uploads are required for cadence."})
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

func estimateChannelRevenue(videos []ChannelVideoSummary, niche NicheAnalysis) RevenueEstimate {
	format := primaryChannelFormat(videos)
	rpmLow, rpmHigh := rpmRange(niche, format)
	if format == "short_form" {
		rpmLow *= 0.12
		rpmHigh *= 0.18
	}
	sampledViews := uint64(0)
	for _, video := range videos {
		if video.Views != nil {
			sampledViews += *video.Views
		}
	}
	sampledLow := float64(sampledViews) / 1000 * rpmLow
	sampledHigh := float64(sampledViews) / 1000 * rpmHigh
	low := sampledLow
	high := sampledHigh
	confidence := 38
	if sampledViews > 0 {
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
		ModelType:                  "sampled_video_views_x_estimated_rpm_range",
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
		CalculationBasis:           fmt.Sprintf("Uses %s public views across the performance sample multiplied by a broad estimated RPM range. This is a sampled-video advertising proxy, not recent monthly revenue or actual YouTube revenue.", formatUint(sampledViews)),
		Assumptions:                []string{"Only videos in the performance sample are included.", "RPM varies by audience geography, ad fill, topic, format, seasonality, and monetisation eligibility.", "Historical top-video views may be included here for performance context, but they are excluded from current cadence and monthly run-rate."},
		Exclusions:                 []string{"Actual YouTube Analytics revenue", "Sponsorships", "Affiliate revenue", "Memberships", "Merchandise", "Courses", "Private, deleted, hidden, or unlisted videos", "Invalid traffic and Premium adjustments"},
		MonetisationEligibility:    "Unknown from public metadata",
		ActualAnalyticsUnavailable: true,
	}
}

func estimateChannelMonthlyRunRate(recentUploads []ChannelVideoSummary, niche NicheAnalysis, now func() time.Time) RevenueEstimate {
	format := primaryChannelFormat(recentUploads)
	rpmLow, rpmHigh := rpmRange(niche, format)
	if format == "short_form" {
		rpmLow *= 0.12
		rpmHigh *= 0.18
	}
	recentViews := uint64(0)
	for _, video := range recentUploads {
		if video.Views != nil {
			recentViews += *video.Views
		}
	}
	cadence := sampledCadenceSummary(recentUploads, now)
	months := maxFloat(1, cadence.SpanDays/30.4375)
	low := float64(recentViews) / 1000 * rpmLow / months
	high := float64(recentViews) / 1000 * rpmHigh / months
	confidence := 30
	if cadence.Available {
		confidence += 16
	}
	if len(recentUploads) >= 8 {
		confidence += 10
	}
	if recentViews > 0 {
		confidence += 10
	}
	confidence = clampInt(confidence)
	basis := "Insufficient recent upload data for a current monthly run-rate."
	if recentViews > 0 {
		basis = fmt.Sprintf("Uses %s public views from the recent upload sample over a %.1f-month observation window, multiplied by a broad estimated RPM range. Historical top-video views are excluded. This is not actual YouTube revenue.", formatUint(recentViews), months)
	}
	return RevenueEstimate{
		Source:                     "public_estimate",
		ModelType:                  "recent_upload_views_monthly_run_rate_x_estimated_rpm_range",
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
		CalculationBasis:           basis,
		Assumptions:                []string{"Only recent-upload sample views are included.", "Observation window comes from recent upload dates only.", "RPM is estimated from broad topic and format assumptions."},
		Exclusions:                 []string{"Actual YouTube Analytics revenue", "Historical top-video views", "Sponsorships", "Affiliate revenue", "Memberships", "Merchandise", "Private, deleted, hidden, or unlisted videos"},
		MonetisationEligibility:    "Unknown from public metadata",
		ActualAnalyticsUnavailable: true,
	}
}

func buildChannelCharts(recentUploads, performanceSample []ChannelVideoSummary, pillars []ChannelContentPillar) ChannelCharts {
	sorted := append([]ChannelVideoSummary{}, recentUploads...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].PublishedAt < sorted[j].PublishedAt })
	perf := []ChannelChartPoint{}
	for _, video := range sorted {
		if video.Views == nil || video.PublishedAt == "" {
			continue
		}
		perf = append(perf, ChannelChartPoint{Label: formatPublishedDate(video.PublishedAt), Date: video.PublishedAt, Title: video.Title, Views: video.Views, Value: float64(*video.Views), Duration: formatISO8601Duration(video.Duration), Format: video.Format, Pillar: video.Pillar, VideoID: video.VideoID})
	}
	dist := []ChannelChartPoint{}
	views := videoViewFloats(performanceSample)
	sort.Float64s(views)
	for i, v := range views {
		dist = append(dist, ChannelChartPoint{Label: fmt.Sprintf("Rank %d by views", i+1), Value: v, Description: "Individual sampled video views sorted ascending"})
	}
	cadenceCounts := map[string]int{}
	for _, video := range recentUploads {
		if t, err := time.Parse(time.RFC3339, video.PublishedAt); err == nil {
			key := t.Format("Jan 2006")
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
	for _, video := range performanceSample {
		if video.Views != nil && video.Format != "" && video.Format != "unknown" {
			formatMedian[video.Format] = append(formatMedian[video.Format], float64(*video.Views))
		}
	}
	formatPoints := []ChannelChartPoint{}
	for _, key := range mapKeysFloatSlice(formatMedian) {
		formatPoints = append(formatPoints, ChannelChartPoint{Label: formatVideoFormatLabel(key), Value: medianFloat(formatMedian[key]), Description: fmt.Sprintf("%d sampled video(s)", len(formatMedian[key]))})
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
		if isTrueNumberLedTitle(title) {
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

func isTrueNumberLedTitle(title string) bool {
	lower := strings.ToLower(strings.TrimSpace(title))
	if regexp.MustCompile(`^\s*\d+\s+`).MatchString(lower) {
		return true
	}
	if regexp.MustCompile(`\b\d+\s+(features|mistakes|reasons|ways|tips|steps|things|questions|lessons|rules|changes|problems|ideas)\b`).MatchString(lower) {
		return true
	}
	if regexp.MustCompile(`\b(top|best)\s+\d+\b`).MatchString(lower) {
		return true
	}
	return false
}

func buildChannelGrowthOpportunities(niche NicheAnalysis, pillars []ChannelContentPillar, groups []ChannelVideoGroup, packaging ChannelPackaging) []ChannelOpportunity {
	out := []ChannelOpportunity{}
	core := firstPhraseFromPillars(pillars)
	if core == "" {
		core = "a validated high-performing subject"
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
		SampleTitle:       naturalChannelTitle(core, "followup"),
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
			SampleTitle:       naturalChannelTitle(core, "packaging"),
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
		SampleTitle:       naturalChannelTitle(core, "beginner"),
		NextAction:        "Turn the strongest pillar into a glossary, checklist, or first-principles explainer.",
	})
	return out
}

func buildChannelContentPlan(recentUploads, performanceSample []ChannelVideoSummary, pillars []ChannelContentPillar, opps []ChannelOpportunity, niche NicheAnalysis, now func() time.Time) []ChannelPlanWeek {
	cadence := sampledCadenceSummary(recentUploads, now)
	recommendedUploads := recommendedMonthlyUploads(cadence, performanceSample)
	core := firstPhraseFromPillars(pillars)
	if core == "" {
		core = "validated topic"
	}
	types := []string{"Publishing week", "Research/preparation week", "Packaging test week", "Review/repurpose week"}
	themes := []string{"Proven-topic follow-up", "Research adjacent angles", "Packaging test", "Review and repurpose"}
	target := "Insufficient recent upload data; use the next 30 days for preparation until cadence is clear."
	if cadence.Available {
		target = fmt.Sprintf("Recommended publishing target: %d upload%s in the next 30 days.", recommendedUploads, pluralSuffix(recommendedUploads))
	}
	weeks := []ChannelPlanWeek{}
	usedTitles := map[string]bool{}
	uploadIndex := 0
	for i := 1; i <= 4; i++ {
		theme := themes[i-1]
		weekType := types[i-1]
		ideas := []ChannelPlanIdea{}
		if uploadIndex < recommendedUploads {
			theme = "Proven-topic follow-up"
			weekType = "Publishing week"
			opp := ChannelOpportunity{RecommendedFormat: "Long-form", Title: theme}
			if len(opps) > 0 {
				opp = opps[uploadIndex%len(opps)]
			}
			pillar := core
			if len(pillars) > 0 {
				pillar = pillars[uploadIndex%len(pillars)].Name
			}
			title := firstNonEmpty(opp.SampleTitle, naturalChannelTitle(pillar, "followup"))
			for usedTitles[strings.ToLower(title)] {
				title = naturalChannelTitle(pillar, "packaging")
			}
			ideas = append(ideas, ChannelPlanIdea{
				WorkingTitle:      title,
				ContentPillar:     pillar,
				Format:            firstNonEmpty(opp.RecommendedFormat, "Long-form"),
				Objective:         opp.Title,
				Evidence:          strings.Join(topN(opp.Evidence, 2), " · "),
				HookDirection:     "Open with the viewer problem, then show the concrete payoff.",
				RecommendedTiming: fmt.Sprintf("Week %d", i),
			})
			usedTitles[strings.ToLower(title)] = true
			uploadIndex++
		} else {
			ideas = append(ideas, ChannelPlanIdea{
				WorkingTitle:      []string{"Package the next upload", "Research adjacent angles", "Run one title and thumbnail test", "Review performance and decide the next test"}[i-1],
				ContentPillar:     core,
				Format:            "Preparation",
				Objective:         weekType,
				Evidence:          "Cadence does not support inventing an extra upload this week.",
				HookDirection:     "Use this week to improve the next publishable concept instead of forcing volume.",
				RecommendedTiming: fmt.Sprintf("Week %d", i),
			})
		}
		weeks = append(weeks, ChannelPlanWeek{Week: i, Theme: theme, Cadence: target, WeekType: weekType, Ideas: ideas, Rationale: "Cadence is based on recent public uploads only; preparation weeks are not counted as upload recommendations."})
	}
	return weeks
}

func recommendedMonthlyUploads(cadence sampledCadence, videos []ChannelVideoSummary) int {
	if !cadence.Available {
		return 0
	}
	uploadsPerMonth := cadence.UploadsPerMonth
	if primaryChannelFormat(videos) == "short_form" && uploadsPerMonth >= 12 {
		return minInt(12, int(uploadsPerMonth+0.5))
	}
	switch {
	case uploadsPerMonth < 0.75:
		return 1
	case uploadsPerMonth < 1.75:
		return 2
	case uploadsPerMonth < 3:
		return 2
	case uploadsPerMonth < 6:
		return 4
	default:
		return 4
	}
}

func pluralSuffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
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

func sanitizeChannelGrowthOpportunities(opps []ChannelOpportunity, pillars []ChannelContentPillar, videos []ChannelVideoSummary, identity channelIdentityContext) []ChannelOpportunity {
	validPillars := topPillarNames(pillars, 5)
	fallback := firstVideoSubject(videos, identity)
	out := []ChannelOpportunity{}
	seen := map[string]bool{}
	for _, opp := range opps {
		opp.Why = sanitizeChannelText(opp.Why, validPillars, fallback, identity)
		opp.SampleTitle = sanitizeChannelGeneratedTitle(opp.SampleTitle, validPillars, fallback, identity)
		opp.NextAction = sanitizeChannelText(opp.NextAction, validPillars, fallback, identity)
		key := strings.ToLower(opp.Title + "|" + opp.SampleTitle)
		if opp.SampleTitle == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, opp)
	}
	if len(out) == 0 && fallback != "" {
		out = append(out, ChannelOpportunity{
			Title:             "Use a specific high-performing subject",
			Why:               "The sampled metadata does not support clean repeatable pillars, so the next idea should come from a specific video subject.",
			Evidence:          topVideoTitles(videos, 2),
			RecommendedFormat: "Long-form",
			SuggestedAudience: "Current viewers",
			Confidence:        "Limited",
			SampleTitle:       naturalChannelTitle(fallback, "followup"),
			NextAction:        "Choose one high-performing sampled upload and make a direct, evidence-based follow-up.",
		})
	}
	return topNChannelOpportunities(out, 4)
}

func sanitizeChannelOpportunityStrings(values []string, pillars []ChannelContentPillar, videos []ChannelVideoSummary, identity channelIdentityContext) []string {
	valid := topPillarNames(pillars, 5)
	fallback := firstVideoSubject(videos, identity)
	out := []string{}
	for _, value := range values {
		cleaned := sanitizeChannelText(value, valid, fallback, identity)
		if cleaned != "" && !nearDuplicateSelected(cleaned, out) {
			out = append(out, cleaned)
		}
	}
	return out
}

func sanitizeChannelIdeaStrings(values []string, pillars []ChannelContentPillar, videos []ChannelVideoSummary, identity channelIdentityContext, limit int) []string {
	valid := topPillarNames(pillars, 5)
	fallback := firstVideoSubject(videos, identity)
	out := []string{}
	for _, value := range values {
		cleaned := sanitizeChannelGeneratedTitle(value, valid, fallback, identity)
		if cleaned != "" && !nearDuplicateSelected(cleaned, out) {
			out = append(out, cleaned)
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func sanitizeChannelText(value string, validPillars []string, fallback string, identity channelIdentityContext) string {
	value = strings.TrimSpace(cleanupGeneratedText(value))
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if channelTextContainsIdentityTopic(lower, identity) || malformedChannelRecommendation(lower) {
		if fallback == "" && len(validPillars) > 0 {
			fallback = validPillars[0]
		}
		if fallback == "" {
			return ""
		}
		return "Use a specific sampled subject such as " + applyAcronymCasing(fallback) + " instead of channel identity or biography language."
	}
	return value
}

func sanitizeChannelGeneratedTitle(value string, validPillars []string, fallback string, identity channelIdentityContext) string {
	value = strings.TrimSpace(cleanupGeneratedText(value))
	if fallback == "" && len(validPillars) > 0 {
		fallback = validPillars[0]
	}
	if value == "" || malformedTitle(value) || malformedChannelRecommendation(strings.ToLower(value)) || channelTextContainsIdentityTopic(strings.ToLower(value), identity) {
		if fallback == "" {
			return ""
		}
		return naturalChannelTitle(fallback, "followup")
	}
	return value
}

func channelTextContainsIdentityTopic(lower string, identity channelIdentityContext) bool {
	normalized := normalizeTopicPhrase(lower)
	for name := range identity.Names {
		if name != "" && strings.Contains(normalized, name) {
			return true
		}
	}
	return false
}

func malformedChannelRecommendation(lower string) bool {
	for _, pattern := range []string{"how to marque", "for beginners", "next step after"} {
		if strings.Contains(lower, pattern) && regexp.MustCompile(`(?i)\b(marque|brownlee|youtuber|internet personality|tech head)\b`).MatchString(lower) {
			return true
		}
	}
	return regexp.MustCompile(`(?i)\bhow to\s+[a-z]+(?:\s+[a-z]+)?\s*$`).MatchString(lower)
}

func naturalChannelTitle(topic, mode string) string {
	topic = applyAcronymCasing(normalizeChannelPillarName(topic))
	if topic == "" {
		topic = "this topic"
	}
	switch mode {
	case "beginner":
		return "A practical beginner guide to " + topic
	case "packaging":
		return "What viewers need to know before choosing " + topic
	default:
		return "What changed in " + topic + " and why it matters"
	}
}

func firstVideoSubject(videos []ChannelVideoSummary, identity channelIdentityContext) string {
	for _, video := range videos {
		tokens := tokenizeUseful(video.Title, identity.Tokens)
		for n := 3; n >= 2; n-- {
			for _, phrase := range ngrams(tokens, n) {
				name := normalizeChannelPillarName(phrase)
				if !channelBiographyFragment(name) && !channelIdentityPhrase(name, identity) {
					return name
				}
			}
		}
	}
	return ""
}

func topVideoTitles(videos []ChannelVideoSummary, n int) []string {
	out := []string{}
	for _, video := range videos {
		if video.Title != "" {
			out = append(out, video.Title)
		}
		if len(out) >= n {
			break
		}
	}
	return out
}

func topNChannelOpportunities(values []ChannelOpportunity, n int) []ChannelOpportunity {
	if n > len(values) {
		n = len(values)
	}
	out := make([]ChannelOpportunity, n)
	copy(out, values[:n])
	return out
}

func buildChannelAnalysisDetails(recentUploads, performanceSample []ChannelVideoSummary, channel youtubeChannelItem, now func() time.Time) ChannelAnalysisDetails {
	hidden := []string{}
	if channel.Statistics.SubscriberCount == "" {
		hidden = append(hidden, "Subscriber count is hidden or unavailable.")
	}
	if visibleLikesCount(performanceSample) < len(performanceSample) {
		hidden = append(hidden, "Some like counts are hidden or unavailable and are not treated as zero.")
	}
	if visibleCommentsCount(performanceSample) < len(performanceSample) {
		hidden = append(hidden, "Some comment counts are hidden or unavailable and are not treated as zero.")
	}
	recentStart, recentEnd := sampleRange(recentUploads)
	performanceStart, performanceEnd := sampleRange(performanceSample)
	cadence := sampledCadenceSummary(recentUploads, now)
	recommended := recommendedMonthlyUploads(cadence, performanceSample)
	return ChannelAnalysisDetails{
		SampledVideoCount:            len(performanceSample),
		RecentSampleCount:            len(recentUploads),
		PerformanceSampleCount:       len(performanceSample),
		SampleStart:                  performanceStart,
		SampleEnd:                    performanceEnd,
		RecentSampleStart:            recentStart,
		RecentSampleEnd:              recentEnd,
		PerformanceSampleStart:       performanceStart,
		PerformanceSampleEnd:         performanceEnd,
		SampleDateSpanDays:           round1(cadence.SpanDays),
		UploadsPerMonth:              round1(cadence.UploadsPerMonth),
		UploadsPerWeek:               round1(cadence.UploadsPerWeek),
		MedianUploadIntervalDays:     round1(cadence.MedianIntervalDays),
		CadenceAvailable:             cadence.Available,
		CadenceMethodology:           cadence.Methodology,
		RecommendedUploadsNext30Days: recommended,
		CadenceConfidence:            cadence.Confidence,
		ProviderAvailability:         "Public YouTube metadata available",
		HiddenMetricNotes:            hidden,
		ScoringMethodology:           []string{"Channel opportunity is bounded 0-100.", "Weights: topic clarity, packaging, consistency, repeatability, format efficiency, momentum, monetisation potential, and evidence quality.", "Evidence quality caps low-sample or hidden-stat analyses."},
		TopicMethodology:             []string{"Pillars are derived from repeated public title evidence, with channel identity, handle, creator-name variants, biography fragments, category-only terms, and description-only phrases removed.", "Rendered pillars require multiple supporting uploads, separate publishing dates, positive upload share, median views from supporting videos, and phrase-quality validation.", "A subject/object that appears incidentally does not qualify unless it recurs as an editorial theme or format."},
		RevenueMethodology:           []string{"Estimated ad revenue across sampled videos uses the performance sample and broad estimated RPM ranges.", "Current estimated monthly run-rate uses only the recent upload sample and excludes historical top-video views.", "Both figures are public metadata proxies, not actual YouTube revenue."},
		ClassificationRules:          []string{"Short-form: duration up to 180 seconds.", "Long-form: duration above 180 seconds.", "Livestream only when duration/source signals support it; otherwise unknown."},
		AnalysisTimestamp:            now().Format(time.RFC3339),
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
	summary := sampledCadenceSummary(videos, now)
	return summary.UploadsPerMonth, summary.DaysSinceLast
}

type sampledCadence struct {
	Count              int
	Oldest             time.Time
	Newest             time.Time
	SpanDays           float64
	MedianIntervalDays float64
	UploadsPerMonth    float64
	UploadsPerWeek     float64
	DaysSinceLast      float64
	Available          bool
	Confidence         string
	Methodology        string
}

func sampledCadenceSummary(videos []ChannelVideoSummary, now func() time.Time) sampledCadence {
	dates := []time.Time{}
	for _, video := range videos {
		t, err := time.Parse(time.RFC3339, video.PublishedAt)
		if err != nil {
			continue
		}
		dates = append(dates, t)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	count := len(dates)
	if count == 0 {
		return sampledCadence{Confidence: "Unavailable", Methodology: "Insufficient recent upload data: no dated recent public uploads were available."}
	}
	oldest := dates[0]
	newest := dates[count-1]
	spanDays := newest.Sub(oldest).Hours() / 24
	daysSinceLast := now().Sub(newest).Hours() / 24
	if daysSinceLast < 0 {
		daysSinceLast = 0
	}
	if count < 2 {
		return sampledCadence{Count: count, Oldest: oldest, Newest: newest, SpanDays: spanDays, DaysSinceLast: daysSinceLast, Confidence: "Unavailable", Methodology: "Insufficient recent upload data: at least two dated recent uploads are required for cadence."}
	}
	intervals := []float64{}
	for i := 1; i < len(dates); i++ {
		days := dates[i].Sub(dates[i-1]).Hours() / 24
		if days > 0 {
			intervals = append(intervals, days)
		}
	}
	if len(intervals) == 0 {
		return sampledCadence{Count: count, Oldest: oldest, Newest: newest, SpanDays: spanDays, DaysSinceLast: daysSinceLast, Confidence: "Unavailable", Methodology: "Insufficient recent upload data: dated uploads did not produce positive intervals."}
	}
	prelimMedian := medianFloat(intervals)
	filtered := []float64{}
	gapCap := maxFloat(90, prelimMedian*4)
	for _, days := range intervals {
		if days <= gapCap {
			filtered = append(filtered, days)
		}
	}
	if len(filtered) < 1 {
		filtered = intervals
	}
	medianInterval := medianFloat(filtered)
	if medianInterval <= 0 {
		return sampledCadence{Count: count, Oldest: oldest, Newest: newest, SpanDays: spanDays, DaysSinceLast: daysSinceLast, Confidence: "Unavailable", Methodology: "Insufficient recent upload data: cadence interval could not be calculated."}
	}
	uploadsPerMonth := 30.4375 / medianInterval
	if uploadsPerMonth > 60 {
		uploadsPerMonth = 60
	}
	confidence := "Low"
	if count >= 12 && len(filtered) >= 8 {
		confidence = "Good"
	} else if count >= 4 && len(filtered) >= 3 {
		confidence = "Moderate"
	}
	method := fmt.Sprintf("Median interval %.1f days from %d adjacent recent-upload interval(s); %d extreme gap(s) excluded.", medianInterval, len(filtered), len(intervals)-len(filtered))
	return sampledCadence{Count: count, Oldest: oldest, Newest: newest, SpanDays: spanDays, MedianIntervalDays: medianInterval, UploadsPerMonth: uploadsPerMonth, UploadsPerWeek: uploadsPerMonth / 4.345, DaysSinceLast: daysSinceLast, Available: true, Confidence: confidence, Methodology: method}
}

func cadenceObservationMonths(videos []ChannelVideoSummary) float64 {
	summary := sampledCadenceSummary(videos, func() time.Time { return time.Now().UTC() })
	if summary.Count == 0 {
		return 1
	}
	days := summary.SpanDays
	if days < 30 {
		days = 30
	}
	return days / 30
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
		if isTrueNumberLedTitle(title) {
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
				return formatVideoFormatLabel(video.Format)
			}
		}
	}
	return "Long-form"
}

func formatVideoFormatLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "long_form":
		return "Long-form"
	case "short_form":
		return "Shorts"
	case "livestream":
		return "Livestream"
	default:
		return "Unknown"
	}
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

type youtubePlaylistItemsResponse struct {
	Items []struct {
		Snippet struct {
			PublishedAt  string `json:"publishedAt"`
			ChannelID    string `json:"channelId"`
			ChannelTitle string `json:"channelTitle"`
			Title        string `json:"title"`
			Description  string `json:"description"`
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
			ResourceID struct {
				VideoID string `json:"videoId"`
			} `json:"resourceId"`
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

func (p *YouTubeProvider) fetchChannelUploads(ctx context.Context, uploadsPlaylistID string, limit int) ([]ChannelVideoSummary, error) {
	uploadsPlaylistID = strings.TrimSpace(uploadsPlaylistID)
	if uploadsPlaylistID == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 25
	}
	playlistURL := youtubeAPIURL("playlistItems", map[string]string{"part": "snippet", "playlistId": uploadsPlaylistID, "maxResults": strconv.Itoa(limit), "key": p.apiKey})
	var playlist youtubePlaylistItemsResponse
	if err := p.getJSON(ctx, playlistURL, &playlist); err != nil {
		return nil, err
	}
	ids := []string{}
	summaries := map[string]ChannelVideoSummary{}
	for _, item := range playlist.Items {
		id := item.Snippet.ResourceID.VideoID
		if id == "" {
			continue
		}
		ids = append(ids, id)
		summaries[id] = ChannelVideoSummary{
			VideoID:      id,
			Title:        item.Snippet.Title,
			Description:  item.Snippet.Description,
			ChannelID:    item.Snippet.ChannelID,
			ChannelTitle: item.Snippet.ChannelTitle,
			PublishedAt:  item.Snippet.PublishedAt,
			ThumbnailURL: firstNonEmpty(item.Snippet.Thumbnails.High.URL, item.Snippet.Thumbnails.Medium.URL, item.Snippet.Thumbnails.Default.URL),
		}
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
		summary, ok := summaries[video.ID]
		if !ok {
			continue
		}
		out = append(out, ChannelVideoSummary{
			VideoID:      video.ID,
			Title:        firstNonEmpty(video.Snippet.Title, summary.Title),
			Description:  firstNonEmpty(video.Snippet.Description, summary.Description),
			ChannelID:    firstNonEmpty(video.Snippet.ChannelID, summary.ChannelID),
			ChannelTitle: firstNonEmpty(video.Snippet.ChannelTitle, summary.ChannelTitle),
			PublishedAt:  firstNonEmpty(video.Snippet.PublishedAt, summary.PublishedAt),
			Duration:     video.ContentDetails.Duration,
			ThumbnailURL: firstNonEmpty(bestThumbnail(video), summary.ThumbnailURL),
			Views:        parseUintPtr(video.Statistics.ViewCount),
			Likes:        parseUintPtr(video.Statistics.LikeCount),
			Comments:     parseUintPtr(video.Statistics.CommentCount),
		})
	}
	return out, nil
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
		if isTrueNumberLedTitle(title) {
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
	summary := sampledCadenceSummary(videos, now)
	if !summary.Available {
		return "Insufficient recent upload data."
	}
	return fmt.Sprintf("Approximately %.1f public uploads per month (%.1f per week), based on a %.1f-day median interval across %d recent uploads from %s to %s. Cadence confidence: %s.", summary.UploadsPerMonth, summary.UploadsPerWeek, summary.MedianIntervalDays, summary.Count, summary.Oldest.Format("2 Jan 2006"), summary.Newest.Format("2 Jan 2006"), summary.Confidence)
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
