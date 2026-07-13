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
	ChannelID               string                 `json:"channel_id,omitempty"`
	ChannelTitle            string                 `json:"channel_title,omitempty"`
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
		Description:                item.Snippet.Description,
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
		Status:     StatusNotConfigured,
		Message:    "Add YOUTUBE_API_KEY in Settings/Railway variables to analyze YouTube channels.",
		ChannelURL: channelURL,
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
	channelID, err := p.resolveChannelID(ctx, channelURL)
	if err != nil {
		result.Status = "invalid_input"
		result.Message = err.Error()
		return result, nil
	}
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
	videos := mergeChannelVideos(recentVideos, topVideos)
	keywordIntel := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title:             channel.Snippet.Title,
		Description:       channel.Snippet.Description,
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
	viewDistribution, ratio := channelSignals(channel.Statistics.SubscriberCount, videos)
	performanceDistribution := PerformanceDistribution(videos)
	patterns := titlePatterns(videos)
	formats := formatPatternsFromVideos(videos)
	channelOpps := ChannelOpportunities(nicheAnalysis, pillars, keywordIntel)
	ideas := SuggestedChannelIdeas(nicheAnalysis, keywordIntel, pillars)
	shortIdeas := SuggestedShortClipIdeas(nicheAnalysis, keywordIntel, pillars)
	score, reason := ScoreEvidence(ScoringInput{
		SourceConfidence: 0.9,
		Views:            parseUintPtr(channel.Statistics.ViewCount),
		KeywordCount:     len(keywords),
		HasEvidenceURL:   true,
		RegionMatch:      0.5,
		NicheMatch:       0.6,
	})
	result = ChannelAnalysisResult{
		Status:             StatusOK,
		Message:            "Analyzed public YouTube Data API channel metadata. Strategy notes are inferred from public channel and recent video metadata.",
		ChannelURL:         channelURL,
		ChannelID:          channelID,
		ChannelTitle:       channel.Snippet.Title,
		Description:        channel.Snippet.Description,
		Subscribers:        parseUintPtr(channel.Statistics.SubscriberCount),
		Views:              parseUintPtr(channel.Statistics.ViewCount),
		VideoCount:         parseUintPtr(channel.Statistics.VideoCount),
		Country:            channel.Snippet.Country,
		PublicTopicDetails: channel.TopicDetails.TopicCategories,
		RecentVideos:       recentVideos,
		TopVideosSummary:   topNChannelVideos(topVideos, 10),
		ChannelSnapshot: map[string]any{
			"subscribers": parseUintPtr(channel.Statistics.SubscriberCount),
			"total_views": parseUintPtr(channel.Statistics.ViewCount),
			"video_count": parseUintPtr(channel.Statistics.VideoCount),
			"country":     channel.Snippet.Country,
			"channel_age": channelAge(channel.Snippet.PublishedAt, p.now),
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
		Limitations: []string{
			"Public metadata only: official YouTube Data API fields are used; no scraping, downloads, private analytics, retention, revenue, or traffic sources.",
			"Keyword clusters and strategy are inferred from public titles, descriptions, topics, tags where available, and visible counts; no exact search ranking keywords are claimed.",
			"Recent/top video selection is based on official API responses and available public counts.",
		},
		Metadata: ProviderResultMetadata{
			SourceProvider: "youtube_data_api",
			SourceURL:      "https://www.youtube.com/channel/" + channelID,
			EvidenceURL:    apiURLWithoutKey(channelAPIURL),
			Country:        channel.Snippet.Country,
			Platform:       "youtube",
			Topic:          nicheAnalysis.PrimaryNiche,
			Keyword:        firstKeyword(keywords),
			Score:          score,
			Views:          parseUintPtr(channel.Statistics.ViewCount),
			Confidence:     0.74,
			Limitations:    []string{"Strategy is inferred from public metadata only."},
			FetchedAt:      p.now(),
			ScoreReason:    reason,
		},
	}
	return result, nil
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

func (p *YouTubeProvider) resolveChannelID(ctx context.Context, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("channel_url is required")
	}
	if strings.HasPrefix(raw, "UC") && len(raw) >= 20 && !strings.Contains(raw, "/") {
		return raw, nil
	}
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) >= 2 && parts[0] == "channel" && strings.HasPrefix(parts[1], "UC") {
			return parts[1], nil
		}
		for _, part := range parts {
			if strings.HasPrefix(part, "@") {
				return p.resolveChannelByHandle(ctx, part)
			}
		}
		if len(parts) >= 2 && (parts[0] == "c" || parts[0] == "user") {
			return p.resolveChannelByQuery(ctx, parts[1])
		}
	}
	if strings.HasPrefix(raw, "@") {
		return p.resolveChannelByHandle(ctx, raw)
	}
	return p.resolveChannelByQuery(ctx, raw)
}

func (p *YouTubeProvider) resolveChannelByHandle(ctx context.Context, handle string) (string, error) {
	handle = strings.TrimSpace(handle)
	endpoint := youtubeAPIURL("channels", map[string]string{"part": "id", "forHandle": handle, "key": p.apiKey})
	var res youtubeChannelsResponse
	if err := p.getJSON(ctx, endpoint, &res); err == nil && len(res.Items) > 0 {
		return res.Items[0].ID, nil
	}
	return p.resolveChannelByQuery(ctx, strings.TrimPrefix(handle, "@"))
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
	payload := map[string]any{
		"model": strings.TrimSpace(os.Getenv("OPENAI_TEXT_MODEL")),
		"messages": []map[string]string{
			{"role": "system", "content": "Improve YouTube creator intelligence using only the provided public metadata summary. Do not invent non-public analytics, exact search ranking terms, retention, revenue, demographics, or traffic sources. Return compact JSON only."},
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
	if len(enhanced.PrimaryKeywords) > 0 {
		kw.PrimaryKeywords = cleanStringList(enhanced.PrimaryKeywords)
		kw.PrimaryTopics = kw.PrimaryKeywords
	}
	if len(enhanced.SecondaryKeywords) > 0 {
		kw.SecondaryKeywords = cleanStringList(enhanced.SecondaryKeywords)
		kw.SupportingTerms = kw.SecondaryKeywords
	}
	if len(enhanced.LongTailPhrases) > 0 {
		kw.LongTailPhrases = cleanStringList(enhanced.LongTailPhrases)
		kw.SearchPhrases = kw.LongTailPhrases
	}
	if strings.TrimSpace(enhanced.InferredSearchIntent) != "" {
		kw.InferredSearchIntent = strings.TrimSpace(enhanced.InferredSearchIntent)
	}
	return kw
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
