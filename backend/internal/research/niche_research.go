package research

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const publicRPMUnavailableMessage = "Currency estimate unavailable until sufficient monetization evidence is connected."

const (
	NicheStatusInsufficientEvidence           = "insufficient_evidence"
	NicheStatusNoMatchingContent              = "no_matching_content"
	NicheStatusQuotaTemporarilyUnavailable    = "quota_temporarily_unavailable"
	NicheStatusCredentialsInvalid             = "credentials_invalid"
	NicheStatusProviderTemporarilyUnavailable = "provider_temporarily_unavailable"
	NicheStatusValidationTimeout              = "validation_timeout"
	NicheStatusResearchFailed                 = "research_failed"
)

const (
	NicheMessageInsufficientEvidence   = "More public evidence is required before ranking this niche."
	NicheMessageNoMatchingContent      = "No reliable market evidence matched this profile."
	NicheMessageTemporarilyUnavailable = "Niche evidence is temporarily unavailable. Try again later."
	NicheMessageQuotaUnavailable       = "Research limits have been reached temporarily."
)

type CreatorNicheProfile struct {
	ProfessionalSkills       string   `json:"professional_skills"`
	Hobbies                  string   `json:"hobbies"`
	LivedExperiences         string   `json:"lived_experiences"`
	TeachingSubjects         string   `json:"teaching_subjects"`
	ThreeYearsAgoAdvice      string   `json:"three_years_ago_advice"`
	TargetAudience           string   `json:"target_audience"`
	TargetCountry            string   `json:"target_country"`
	TargetLanguage           string   `json:"target_language"`
	CreatorPresence          string   `json:"creator_presence"`
	ContentFormats           []string `json:"content_formats"`
	PrimaryMonetizationGoal  string   `json:"-"`
	OptionalBroadTopic       string   `json:"optional_broad_topic"`
	WeeklyProductionCapacity string   `json:"weekly_production_capacity"`
}

type NicheResearchRequest struct {
	Profile CreatorNicheProfile `json:"profile"`
	Refresh bool                `json:"refresh,omitempty"`
}

type NicheReport struct {
	ID                    string              `json:"id"`
	Status                string              `json:"status"`
	Message               string              `json:"message"`
	GeneratedAt           time.Time           `json:"generated_at"`
	EvidenceFreshness     string              `json:"evidence_freshness"`
	AnalysisMode          string              `json:"analysis_mode"`
	ProviderStatus        []ProviderStatus    `json:"provider_status,omitempty"`
	CreatorProfileSummary string              `json:"creator_profile_summary"`
	PrimaryRecommendation *NicheCandidate     `json:"primary_recommendation,omitempty"`
	AlternativeCandidates []NicheCandidate    `json:"alternative_candidates,omitempty"`
	Methodology           []string            `json:"methodology"`
	Profile               CreatorNicheProfile `json:"profile"`
	Candidates            []NicheCandidate    `json:"candidates"`
	Cache                 NicheCacheInfo      `json:"cache"`
	Limitations           []string            `json:"limitations"`
	CreatedAt             time.Time           `json:"created_at"`
	Internal              NicheInternal       `json:"-"`
}

type NicheCacheInfo struct {
	Hit               bool      `json:"hit"`
	CacheHit          bool      `json:"cache_hit"`
	Key               string    `json:"key,omitempty"`
	StoredAt          time.Time `json:"stored_at,omitempty"`
	TTL               string    `json:"ttl"`
	EvidenceFetchedAt time.Time `json:"evidence_fetched_at,omitempty"`
	EvidenceAge       string    `json:"evidence_age,omitempty"`
	Freshness         string    `json:"freshness,omitempty"`
}

type NicheCandidate struct {
	ID                       string                 `json:"id"`
	Name                     string                 `json:"name"`
	ConcisePositioning       string                 `json:"concise_positioning"`
	Category                 string                 `json:"category"`
	Subcategory              string                 `json:"subcategory"`
	TargetAudience           string                 `json:"target_audience"`
	AudienceProblems         []string               `json:"audience_problems"`
	CreatorAdvantages        []string               `json:"creator_advantages"`
	UniqueAngle              string                 `json:"unique_angle"`
	OverallScore             float64                `json:"overall_score"`
	Confidence               string                 `json:"confidence"`
	Dimensions               NicheScoreDimensions   `json:"dimensions"`
	ContentPillars           []ContentPillar        `json:"content_pillars"`
	TopicClusters            []TopicCluster         `json:"topic_clusters"`
	RecommendedTitles        []VideoTopic           `json:"recommended_titles"`
	OpportunityGaps          []string               `json:"opportunity_gaps"`
	EvidenceSummary          string                 `json:"evidence_summary"`
	MarketEvidence           MarketEvidence         `json:"market_evidence"`
	SearchQueriesUsed        []string               `json:"search_queries_used"`
	Runway                   ContentRunway          `json:"runway"`
	Level1                   string                 `json:"level_1"`
	Level2                   string                 `json:"level_2"`
	Level3                   string                 `json:"level_3"`
	NicheName                string                 `json:"niche_name"`
	CorePhrase               string                 `json:"core_phrase"`
	TargetViewer             string                 `json:"target_viewer"`
	ViewerProblem            string                 `json:"viewer_problem"`
	CreatorAdvantage         string                 `json:"creator_advantage"`
	RecommendedContentFormat string                 `json:"recommended_content_format"`
	MonetizationRoutes       []string               `json:"-"`
	Validation               NicheValidation        `json:"validation"`
	Outliers                 []OutlierEvidence      `json:"outliers"`
	SupplyGaps               []SupplyGap            `json:"supply_gaps"`
	Monetization             MonetizationEstimate   `json:"-"`
	VideoTopics              []VideoTopic           `json:"video_topics"`
	TopicPillars             []ContentPillar        `json:"topic_pillars"`
	First10Titles            []string               `json:"first_10_titles"`
	Sustainability           SustainabilityEvidence `json:"sustainability"`
	Scores                   NicheScores            `json:"scores"`
	Risks                    []string               `json:"risks"`
	RecommendedFirstAction   string                 `json:"recommended_first_action"`
	GeneratedReasoning       string                 `json:"-"`
}

type NicheScoreDimensions struct {
	CreatorFit             ScoreExplanation `json:"creator_fit"`
	AudienceDemand         ScoreExplanation `json:"audience_demand"`
	CompetitionOpportunity ScoreExplanation `json:"competition_opportunity"`
	Sustainability         ScoreExplanation `json:"sustainability"`
	Differentiation        ScoreExplanation `json:"differentiation"`
}

type TopicCluster struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Titles      []string `json:"titles"`
}

type MarketEvidence struct {
	Status         string     `json:"status"`
	SourceTypes    []string   `json:"source_types"`
	SampleSize     int        `json:"sample_size"`
	RecentActivity string     `json:"recent_activity"`
	MedianViews    *uint64    `json:"median_views"`
	Engagement     *float64   `json:"engagement"`
	CollectedAt    *time.Time `json:"collected_at"`
	Limitations    []string   `json:"limitations"`
}

type ContentRunway struct {
	ViableTopicCount       int    `json:"viable_topic_count"`
	EstimatedWeeks         int    `json:"estimated_weeks"`
	WeeklyCapacity         int    `json:"weekly_capacity"`
	EstimatedContentRunway string `json:"estimated_content_runway"`
}

type NicheValidation struct {
	SearchPhrases           []string `json:"search_phrases"`
	RecentPublicationVolume int      `json:"recent_publication_volume"`
	SampledVideoCount       int      `json:"sampled_video_count"`
	TotalSampledViews       uint64   `json:"total_sampled_views"`
	MedianSampledViews      uint64   `json:"median_sampled_views"`
	EngagementRate          float64  `json:"engagement_rate"`
	NewestActivity          string   `json:"newest_activity,omitempty"`
	RisingTopicOverlap      bool     `json:"rising_topic_overlap"`
	MarketEvidenceSummary   string   `json:"market_evidence_summary"`
	CompetitionLevel        string   `json:"competition_level"`
	ValidationBudgetUsed    int      `json:"validation_budget_used"`
	EvidenceConfidence      string   `json:"evidence_confidence"`
}

type OutlierEvidence struct {
	Title          string  `json:"title"`
	ThumbnailURL   string  `json:"thumbnail_url,omitempty"`
	CanonicalURL   string  `json:"canonical_url"`
	ChannelName    string  `json:"channel_name"`
	PublicationAge string  `json:"publication_age"`
	PublicViews    uint64  `json:"public_views"`
	Reason         string  `json:"outlier_reason"`
	Strength       float64 `json:"outlier_strength"`
}

type SupplyGap struct {
	Statement  string   `json:"statement"`
	Evidence   []string `json:"evidence"`
	Confidence string   `json:"confidence"`
}

type MonetizationEstimate struct {
	Mode                    string               `json:"mode"`
	CommercialPotential     string               `json:"commercial_potential"`
	Score                   float64              `json:"score"`
	RPMEstimateAvailable    bool                 `json:"rpm_estimate_available"`
	Currency                string               `json:"currency,omitempty"`
	RPMLow                  *float64             `json:"rpm_low,omitempty"`
	RPMMidpoint             *float64             `json:"rpm_midpoint,omitempty"`
	RPMHigh                 *float64             `json:"rpm_high,omitempty"`
	EstimatedEarnings       []EarningsProjection `json:"estimated_earnings,omitempty"`
	Confidence              string               `json:"confidence"`
	CalibrationType         string               `json:"calibration_type"`
	CalibrationAge          string               `json:"calibration_age,omitempty"`
	CalculationAssumptions  []string             `json:"calculation_assumptions"`
	UnavailableReason       string               `json:"unavailable_reason,omitempty"`
	Format                  string               `json:"format"`
	TargetMarket            string               `json:"target_market"`
	AdvertiserDemandSignals []string             `json:"advertiser_demand_signals"`
	EstimateDisclaimer      string               `json:"estimate_disclaimer"`
}

type EarningsProjection struct {
	Views    int     `json:"views"`
	Low      float64 `json:"low"`
	Midpoint float64 `json:"midpoint"`
	High     float64 `json:"high"`
	Formula  string  `json:"formula"`
}

type VideoTopic struct {
	Title          string `json:"title"`
	Pillar         string `json:"pillar"`
	Intent         string `json:"intent"`
	Difficulty     string `json:"difficulty"`
	Source         string `json:"source"`
	EvidenceStatus string `json:"evidence_status"`
}

type ContentPillar struct {
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Percentage    float64  `json:"percentage"`
	TopicCount    int      `json:"topic_count"`
	ExampleTitles []string `json:"example_titles,omitempty"`
}

type SustainabilityEvidence struct {
	ViableTopicCount       int     `json:"viable_topic_count"`
	ContentPillarCount     int     `json:"content_pillar_count"`
	TopicRepetitionRisk    string  `json:"topic_repetition_risk"`
	EstimatedContentRunway string  `json:"estimated_content_runway"`
	Score                  float64 `json:"score"`
	Warning                string  `json:"warning,omitempty"`
	DedupedRemoved         int     `json:"deduped_removed"`
}

type NicheScores struct {
	PersonalFit    ScoreExplanation `json:"personal_fit"`
	Demand         ScoreExplanation `json:"demand"`
	OpportunityGap ScoreExplanation `json:"opportunity_gap"`
	Monetization   ScoreExplanation `json:"monetization"`
	Sustainability ScoreExplanation `json:"sustainability"`
	Overall        ScoreExplanation `json:"overall"`
	Confidence     ScoreExplanation `json:"confidence"`
}

type ScoreExplanation struct {
	Score       float64  `json:"score"`
	Label       string   `json:"label"`
	Explanation string   `json:"explanation"`
	Factors     []string `json:"factors,omitempty"`
}

type NicheInternal struct {
	Provenance []string `json:"provenance"`
	CacheKey   string   `json:"cache_key"`
	ErrorCode  string   `json:"error_code,omitempty"`
	HTTPStatus int      `json:"http_status,omitempty"`
	Reason     string   `json:"reason,omitempty"`
}

type NicheResearchError struct {
	Code       string
	Message    string
	HTTPStatus int
	Reason     string
	Temporary  bool
	Err        error
}

func (e *NicheResearchError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code
}

func (e *NicheResearchError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type NicheChannelStats struct {
	ChannelID   string
	Title       string
	Subscribers *uint64
	Views       *uint64
	VideoCount  *uint64
	HiddenSubs  bool
}

type NicheEvidenceProvider interface {
	Status() ProviderStatus
	SearchVideos(ctx context.Context, query, region, language string, limit int) ([]ChannelVideoSummary, error)
	ChannelStats(ctx context.Context, channelIDs []string) (map[string]NicheChannelStats, error)
}

type NicheTrendProvider interface {
	Discover(ctx context.Context, region, language string, limit int) ([]string, error)
}

type NicheRPMCalibration struct {
	Country      string
	Format       string
	Currency     string
	Low          float64
	Midpoint     float64
	High         float64
	Confidence   string
	Type         string
	CalibratedAt time.Time
	Assumptions  []string
}

type NicheResearchConfig struct {
	YouTube                      NicheEvidenceProvider
	Trends                       NicheTrendProvider
	GoogleAdsStatus              ProviderStatus
	Calibration                  *NicheRPMCalibration
	Strategist                   NicheStrategist
	OpenAIAPIKey                 string
	OpenAIModel                  string
	RequireOpenAI                bool
	YouTubeCache                 *YouTubeEvidenceCache
	YouTubeDailyLimiter          *DailyYouTubeSearchLimiter
	MaxYouTubeSearchesPerRequest int
	DailyYouTubeSearchLimit      int
	Now                          func() time.Time
}

type NicheStrategist interface {
	GenerateCandidates(ctx context.Context, input NicheStrategyInput) (NicheStrategyResult, error)
}

type NicheStrategyInput struct {
	Profile          CreatorNicheProfile      `json:"profile"`
	RisingSignals    []string                 `json:"rising_signals"`
	ExistingEvidence []CandidateEvidenceBrief `json:"existing_evidence"`
	GeneratedAt      time.Time                `json:"generated_at"`
}

type CandidateEvidenceBrief struct {
	Query       string `json:"query"`
	Status      string `json:"status"`
	SampleSize  int    `json:"sample_size"`
	CollectedAt string `json:"collected_at,omitempty"`
}

type NicheStrategyResult struct {
	CreatorProfileSummary string       `json:"creator_profile_summary"`
	PrimaryRecommendation string       `json:"primary_recommendation"`
	Candidates            []NicheDraft `json:"candidates"`
	Methodology           []string     `json:"methodology"`
	Limitations           []string     `json:"limitations"`
}

type NicheDraft struct {
	ID                     string                       `json:"id"`
	Name                   string                       `json:"name"`
	ConcisePositioning     string                       `json:"concise_positioning"`
	Category               string                       `json:"category"`
	Subcategory            string                       `json:"subcategory"`
	TargetAudience         string                       `json:"target_audience"`
	AudienceProblems       []string                     `json:"audience_problems"`
	CreatorAdvantages      []string                     `json:"creator_advantages"`
	UniqueAngle            string                       `json:"unique_angle"`
	DimensionScores        NicheDraftDimensionScores    `json:"dimension_scores"`
	DimensionReasoning     NicheDraftDimensionReasoning `json:"dimension_reasoning"`
	ContentPillars         []ContentPillar              `json:"content_pillars"`
	TopicClusters          []TopicCluster               `json:"topic_clusters"`
	RecommendedTitles      []VideoTopic                 `json:"recommended_titles"`
	OpportunityGaps        []string                     `json:"opportunity_gaps"`
	Risks                  []string                     `json:"risks"`
	RecommendedFirstAction string                       `json:"recommended_first_action"`
	SearchQuery            string                       `json:"search_query"`
	Runway                 string                       `json:"runway"`
	Reasoning              string                       `json:"reasoning"`
}

type NicheDraftDimensionScores struct {
	CreatorFit             float64 `json:"creator_fit"`
	AudienceDemand         float64 `json:"audience_demand"`
	CompetitionOpportunity float64 `json:"competition_opportunity"`
	Sustainability         float64 `json:"sustainability"`
	Differentiation        float64 `json:"differentiation"`
}

type NicheDraftDimensionReasoning struct {
	CreatorFit             string `json:"creator_fit"`
	AudienceDemand         string `json:"audience_demand"`
	CompetitionOpportunity string `json:"competition_opportunity"`
	Sustainability         string `json:"sustainability"`
	Differentiation        string `json:"differentiation"`
}

type OpenAINicheStrategist struct {
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

type heuristicNicheStrategist struct{}

func (heuristicNicheStrategist) GenerateCandidates(ctx context.Context, input NicheStrategyInput) (NicheStrategyResult, error) {
	_ = ctx
	blueprints := generateNicheBlueprints(input.Profile)
	drafts := []NicheDraft{}
	for i, bp := range blueprints {
		pillars := []ContentPillar{{Name: "Beginner foundations"}, {Name: "Tools and workflows"}, {Name: "Mistakes and fixes"}, {Name: "Case studies"}, {Name: "Comparisons"}}
		if strings.Contains(strings.ToLower(bp.Level3), "visa") || strings.Contains(strings.ToLower(bp.Level3), "student") {
			pillars = []ContentPillar{{Name: "Application steps"}, {Name: "Mistakes and refusals"}, {Name: "Money and housing"}, {Name: "Career route"}, {Name: "Case studies"}}
		}
		titles := []VideoTopic{}
		intents := []string{"tutorial", "comparison", "mistake", "case study", "opinion", "breakdown", "beginner guide", "experiment", "workflow", "checklist"}
		for len(titles) < 50 {
			intent := intents[len(titles)%len(intents)]
			pillar := pillars[len(titles)%len(pillars)].Name
			titles = append(titles, VideoTopic{Title: naturalTitle(intent, NicheDraft{Name: bp.Level3, Subcategory: bp.Level2, TargetAudience: bp.TargetViewer}), Pillar: pillar, Intent: intent, Difficulty: "medium", Source: "backend_heuristic_strategy", EvidenceStatus: evidenceModeAIStrategicAnalysis})
		}
		for p := range pillars {
			pillars[p].Percentage = 20
			pillars[p].TopicCount = 10
		}
		drafts = append(drafts, NicheDraft{
			ID:                     fmt.Sprintf("heuristic_%d", i+1),
			Name:                   bp.Level3,
			ConcisePositioning:     sentenceCase(bp.Level3) + " for " + bp.TargetViewer + ".",
			Category:               bp.Level1,
			Subcategory:            bp.Level2,
			TargetAudience:         bp.TargetViewer,
			AudienceProblems:       []string{bp.ViewerProblem},
			CreatorAdvantages:      []string{bp.Advantage},
			UniqueAngle:            bp.Advantage,
			DimensionScores:        NicheDraftDimensionScores{CreatorFit: 72, AudienceDemand: 68, CompetitionOpportunity: 64, Sustainability: 76, Differentiation: 66},
			DimensionReasoning:     NicheDraftDimensionReasoning{CreatorFit: "Matches supplied profile inputs.", AudienceDemand: "Demand requires public validation.", CompetitionOpportunity: "Opportunity depends on specific positioning.", Sustainability: "Has enough distinct pillars for a pilot runway.", Differentiation: "Uses creator-specific lived experience and format fit."},
			ContentPillars:         pillars,
			RecommendedTitles:      titles,
			OpportunityGaps:        []string{"Validate the strongest angle with a focused pilot before scaling."},
			Risks:                  []string{"Public evidence may be limited until validation runs."},
			RecommendedFirstAction: "Publish a pilot tutorial and compare retention, comments, and search phrasing before committing to a full series.",
			SearchQuery:            firstString(bp.QueryPhrases),
			Runway:                 "50-title starter runway",
			Reasoning:              bp.Reasoning,
		})
	}
	return NicheStrategyResult{
		CreatorProfileSummary: "Profile interpreted from supplied skills, audience, format, and topic inputs.",
		PrimaryRecommendation: firstStringFromDrafts(drafts),
		Candidates:            drafts,
		Methodology:           []string{"Used deterministic local strategy generation for tests when no OpenAI client was injected."},
		Limitations:           []string{"Local deterministic strategy is for development and tests; production injects OpenAI."},
	}, nil
}

func (s OpenAINicheStrategist) GenerateCandidates(ctx context.Context, input NicheStrategyInput) (NicheStrategyResult, error) {
	if strings.TrimSpace(s.APIKey) == "" {
		return NicheStrategyResult{}, ErrNotConfigured
	}
	model := strings.TrimSpace(s.Model)
	if model == "" {
		model = "gpt-4o-mini"
	}
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": nicheStrategySystemPrompt()},
			{"role": "user", "content": mustJSON(input)},
		},
		"temperature":     0.35,
		"response_format": nicheStrategyResponseFormat(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return NicheStrategyResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return NicheStrategyResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")
	client := s.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return NicheStrategyResult{}, err
	}
	defer res.Body.Close()
	resBody, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return NicheStrategyResult{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return NicheStrategyResult{}, fmt.Errorf("openai niche strategy returned HTTP %d: %s", res.StatusCode, trimForLog(resBody))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(resBody, &parsed); err != nil {
		return NicheStrategyResult{}, err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return NicheStrategyResult{}, errors.New("openai niche strategy response did not include content")
	}
	var strategy NicheStrategyResult
	if err := json.Unmarshal([]byte(parsed.Choices[0].Message.Content), &strategy); err != nil {
		return NicheStrategyResult{}, err
	}
	return strategy, nil
}

func nicheStrategySystemPrompt() string {
	return strings.Join([]string{
		"You are TrendCortex's niche intelligence strategist.",
		"Use only the supplied creator profile and trend/evidence context.",
		"Generate coherent creator niches, target audiences, content pillars, opportunity gaps, risks, and a 50-title runway.",
		"Do not invent live YouTube views, revenue, RPM, advertiser demand, legal certainty, private analytics, or current facts not supplied.",
		"Titles must sound natural, avoid repeatedly inserting the full niche/audience phrase, and cover tutorials, comparisons, mistakes, case studies, opinion, breakdowns, beginner guides, experiments, and workflows.",
		"Return strict JSON matching the schema.",
	}, " ")
}

func nicheStrategyResponseFormat() map[string]any {
	stringSchema := map[string]any{"type": "string"}
	numberSchema := map[string]any{"type": "number"}
	stringArray := map[string]any{"type": "array", "items": stringSchema}
	scoreProps := map[string]any{
		"creator_fit":             numberSchema,
		"audience_demand":         numberSchema,
		"competition_opportunity": numberSchema,
		"sustainability":          numberSchema,
		"differentiation":         numberSchema,
	}
	reasonProps := map[string]any{
		"creator_fit":             stringSchema,
		"audience_demand":         stringSchema,
		"competition_opportunity": stringSchema,
		"sustainability":          stringSchema,
		"differentiation":         stringSchema,
	}
	pillarSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"name":           stringSchema,
			"description":    stringSchema,
			"percentage":     numberSchema,
			"topic_count":    numberSchema,
			"example_titles": stringArray,
		},
		"required": []string{"name", "description", "percentage", "topic_count", "example_titles"},
	}
	topicSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"title":           stringSchema,
			"pillar":          stringSchema,
			"intent":          stringSchema,
			"difficulty":      stringSchema,
			"source":          stringSchema,
			"evidence_status": stringSchema,
		},
		"required": []string{"title", "pillar", "intent", "difficulty", "source", "evidence_status"},
	}
	clusterSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           map[string]any{"name": stringSchema, "description": stringSchema, "titles": stringArray},
		"required":             []string{"name", "description", "titles"},
	}
	candidateSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"id":                       stringSchema,
			"name":                     stringSchema,
			"concise_positioning":      stringSchema,
			"category":                 stringSchema,
			"subcategory":              stringSchema,
			"target_audience":          stringSchema,
			"audience_problems":        stringArray,
			"creator_advantages":       stringArray,
			"unique_angle":             stringSchema,
			"dimension_scores":         map[string]any{"type": "object", "additionalProperties": false, "properties": scoreProps, "required": []string{"creator_fit", "audience_demand", "competition_opportunity", "sustainability", "differentiation"}},
			"dimension_reasoning":      map[string]any{"type": "object", "additionalProperties": false, "properties": reasonProps, "required": []string{"creator_fit", "audience_demand", "competition_opportunity", "sustainability", "differentiation"}},
			"content_pillars":          map[string]any{"type": "array", "items": pillarSchema},
			"topic_clusters":           map[string]any{"type": "array", "items": clusterSchema},
			"recommended_titles":       map[string]any{"type": "array", "items": topicSchema},
			"opportunity_gaps":         stringArray,
			"risks":                    stringArray,
			"recommended_first_action": stringSchema,
			"search_query":             stringSchema,
			"runway":                   stringSchema,
			"reasoning":                stringSchema,
		},
		"required": []string{"id", "name", "concise_positioning", "category", "subcategory", "target_audience", "audience_problems", "creator_advantages", "unique_angle", "dimension_scores", "dimension_reasoning", "content_pillars", "topic_clusters", "recommended_titles", "opportunity_gaps", "risks", "recommended_first_action", "search_query", "runway", "reasoning"},
	}
	return map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "niche_research_response",
			"strict": true,
			"schema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"creator_profile_summary": stringSchema,
					"primary_recommendation":  stringSchema,
					"candidates":              map[string]any{"type": "array", "items": candidateSchema},
					"methodology":             stringArray,
					"limitations":             stringArray,
				},
				"required": []string{"creator_profile_summary", "primary_recommendation", "candidates", "methodology", "limitations"},
			},
		},
	}
}

func ResearchNiches(ctx context.Context, req NicheResearchRequest, cfg NicheResearchConfig) (NicheReport, error) {
	now := time.Now().UTC
	if cfg.Now != nil {
		now = cfg.Now
	}
	profile := normalizeCreatorNicheProfile(req.Profile)
	generatedAt := now()
	report := NicheReport{
		ID:                nicheReportID(profile, generatedAt),
		Status:            StatusOK,
		Message:           "Generated AI-assisted niche strategy with bounded public evidence validation where available.",
		GeneratedAt:       generatedAt,
		EvidenceFreshness: "ai_only",
		AnalysisMode:      "ai_strategic_analysis",
		Profile:           profile,
		CreatedAt:         generatedAt,
		Cache:             NicheCacheInfo{TTL: "6h", Freshness: "fresh"},
		Limitations: []string{
			"OpenAI strategic analysis does not invent live YouTube numbers.",
			"YouTube validation is optional, capped, cached, and labelled separately from AI reasoning.",
			"Public video metadata cannot reveal private retention, traffic sources, or guaranteed outcomes.",
		},
		Methodology: []string{
			"Normalize creator profile inputs.",
			"Collect inexpensive trend signals where configured.",
			"Ask OpenAI for structured niche strategy and content runway candidates.",
			"Validate model output, filter repetitive titles, normalize pillar percentages, and calculate weighted scores in backend code.",
			"Optionally validate only the highest-ranked candidates with a capped YouTube search budget.",
		},
	}
	if meaningfulProfileFields(profile) < 3 {
		report.Status = "invalid_input"
		report.Message = "Add at least three creator-profile inputs before researching niches."
		return report, nil
	}

	rising := []string{}
	trendStatus := ProviderStatus{ID: "google_trends_rss", Name: "Google Trends RSS", Platform: "google_trends", Status: StatusNotConfigured, Message: "Trend provider is not configured."}
	if cfg.Trends != nil {
		trends, err := cfg.Trends.Discover(ctx, profile.TargetCountry, profile.TargetLanguage, 12)
		if err == nil {
			rising = trends
			trendStatus.Status = StatusActive
			trendStatus.Message = "Trend signals were available for strategic grounding."
		} else {
			trendStatus.Status = StatusUnavailable
			trendStatus.Message = "Trend signals were unavailable; AI analysis continued without them."
		}
	}
	youtubeStatus := ProviderStatus{ID: "youtube_data_api", Name: "YouTube Data API", Platform: "youtube", Status: StatusNotConfigured, Message: "YouTube validation was skipped because credentials are not configured."}
	if cfg.YouTube != nil {
		youtubeStatus = cfg.YouTube.Status()
	}
	report.ProviderStatus = []ProviderStatus{trendStatus, youtubeStatus}

	strategist := cfg.Strategist
	if strategist == nil && strings.TrimSpace(cfg.OpenAIAPIKey) != "" {
		strategist = OpenAINicheStrategist{APIKey: cfg.OpenAIAPIKey, Model: cfg.OpenAIModel}
	}
	if strategist == nil && cfg.RequireOpenAI {
		report.Status = "openai_unavailable"
		report.Message = "AI niche strategy is unavailable because OpenAI is not configured."
		report.AnalysisMode = "openai_unavailable"
		return report, nil
	}
	if strategist == nil {
		strategist = heuristicNicheStrategist{}
	}

	strategy, err := strategist.GenerateCandidates(ctx, NicheStrategyInput{Profile: profile, RisingSignals: rising, GeneratedAt: generatedAt})
	if err != nil {
		report.Status = "openai_unavailable"
		report.Message = "AI niche strategy is temporarily unavailable."
		report.AnalysisMode = "openai_unavailable"
		return report, nil
	}
	report.CreatorProfileSummary = strings.TrimSpace(strategy.CreatorProfileSummary)
	report.Methodology = appendStringGroups(report.Methodology, strategy.Methodology)
	report.Limitations = appendStringGroups(report.Limitations, strategy.Limitations)

	drafts := validateNicheDrafts(strategy.Candidates, profile)
	if len(drafts) == 0 {
		report.Status = "invalid_model_output"
		report.Message = "AI niche strategy returned no usable candidates."
		report.AnalysisMode = "invalid_model_output"
		return report, nil
	}
	candidates := make([]NicheCandidate, 0, len(drafts))
	for _, draft := range drafts {
		candidate := buildAICandidate(draft, profile, rising, generatedAt)
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].OverallScore > candidates[j].OverallScore
	})
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}

	budget := newYouTubeSearchBudget(cfg.MaxYouTubeSearchesPerRequest, cfg.DailyYouTubeSearchLimit, cfg.YouTubeDailyLimiter, now)
	if cfg.YouTube != nil && cfg.YouTube.Status().Status == StatusActive && budget.maxPerRequest > 0 {
		for i := range candidates {
			if i >= 3 || !budget.allow() {
				candidates[i].MarketEvidence.Status = evidenceModeAIStrategicAnalysis
				candidates[i].EvidenceSummary = appendSentence(candidates[i].EvidenceSummary, "YouTube validation was skipped by the request budget.")
				continue
			}
			query := bestCandidateSearchQuery(candidates[i])
			evidence, err := collectBudgetedNicheEvidence(ctx, cfg.YouTube, cfg.YouTubeCache, budget, query, profile, generatedAt)
			if err != nil {
				researchErr := classifyNicheResearchError(err)
				if researchErr.Code == NicheStatusQuotaTemporarilyUnavailable {
					report.AnalysisMode = evidenceModeLimitedEvidence
					report.Message = "Generated AI niche strategy. YouTube quota was reached, so remaining validation used AI and non-YouTube evidence only."
					report.Limitations = append(report.Limitations, "YouTube quota was reached during validation; AI analysis is not presented as verified YouTube evidence.")
					break
				}
				candidates[i].MarketEvidence.Limitations = append(candidates[i].MarketEvidence.Limitations, "YouTube validation was temporarily unavailable.")
				continue
			}
			applyEvidenceToCandidate(&candidates[i], evidence, rising, generatedAt)
		}
		report.EvidenceFreshness = evidenceFreshness(candidates)
		report.AnalysisMode = analysisMode(candidates)
	} else {
		report.AnalysisMode = evidenceModeAIStrategicAnalysis
		report.EvidenceFreshness = "no_youtube_validation"
		report.Limitations = append(report.Limitations, "YouTube validation was skipped because credentials are missing, unavailable, or disabled.")
	}

	report.Candidates = candidates
	report.PrimaryRecommendation = &report.Candidates[0]
	if len(report.Candidates) > 1 {
		report.AlternativeCandidates = append([]NicheCandidate{}, report.Candidates[1:]...)
	}
	report.Internal.Provenance = []string{"openai_structured_strategy", "backend_score_normalization", "optional_budgeted_youtube_validation", "current_trend_overlap"}
	return report, nil
}

type nicheBlueprint struct {
	Level1        string
	Level2        string
	Level3        string
	CorePhrase    string
	TargetViewer  string
	ViewerProblem string
	Advantage     string
	Format        string
	Routes        []string
	QueryPhrases  []string
	Reasoning     string
}

type nicheEvidence struct {
	videos       []ChannelVideoSummary
	channelStats map[string]NicheChannelStats
	budgetUsed   int
	query        string
	mode         string
	collectedAt  time.Time
}

const (
	evidenceModeLiveValidated       = "live_validated"
	evidenceModeCacheValidated      = "cache_validated"
	evidenceModeTrendSupported      = "trend_supported"
	evidenceModeAIStrategicAnalysis = "ai_strategic_analysis"
	evidenceModeLimitedEvidence     = "limited_evidence"
)

type YouTubeEvidenceCache struct {
	mu    sync.Mutex
	items map[string]youtubeEvidenceCacheItem
	TTL   time.Duration
}

type youtubeEvidenceCacheItem struct {
	evidence nicheEvidence
	storedAt time.Time
}

func NewYouTubeEvidenceCache(ttl time.Duration) *YouTubeEvidenceCache {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &YouTubeEvidenceCache{items: map[string]youtubeEvidenceCacheItem{}, TTL: ttl}
}

func (c *YouTubeEvidenceCache) get(key string, now time.Time) (nicheEvidence, time.Time, bool, bool) {
	if c == nil {
		return nicheEvidence{}, time.Time{}, false, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.items[key]
	if !ok {
		return nicheEvidence{}, time.Time{}, false, false
	}
	ttl := c.TTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	ev := item.evidence
	ev.mode = evidenceModeCacheValidated
	ev.collectedAt = item.storedAt
	return ev, item.storedAt, true, now.Sub(item.storedAt) <= ttl
}

func (c *YouTubeEvidenceCache) set(key string, ev nicheEvidence, now time.Time) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = youtubeEvidenceCacheItem{evidence: ev, storedAt: now}
}

type DailyYouTubeSearchLimiter struct {
	mu    sync.Mutex
	day   string
	used  int
	limit int
	now   func() time.Time
}

func NewDailyYouTubeSearchLimiter(limit int, now func() time.Time) *DailyYouTubeSearchLimiter {
	if limit <= 0 {
		limit = 80
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &DailyYouTubeSearchLimiter{limit: limit, now: now}
}

func (l *DailyYouTubeSearchLimiter) TryConsume() bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	day := l.now().UTC().Format("2006-01-02")
	if l.day != day {
		l.day = day
		l.used = 0
	}
	if l.used >= l.limit {
		return false
	}
	l.used++
	return true
}

func (l *DailyYouTubeSearchLimiter) Snapshot() (day string, used int, limit int) {
	if l == nil {
		return "", 0, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.day, l.used, l.limit
}

type youtubeSearchBudget struct {
	maxPerRequest int
	used          int
	dailyLimiter  *DailyYouTubeSearchLimiter
}

func newYouTubeSearchBudget(maxPerRequest, dailyLimit int, limiter *DailyYouTubeSearchLimiter, now func() time.Time) *youtubeSearchBudget {
	if maxPerRequest <= 0 {
		maxPerRequest = 3
	}
	if dailyLimit <= 0 {
		dailyLimit = 80
	}
	if limiter == nil {
		limiter = NewDailyYouTubeSearchLimiter(dailyLimit, now)
	}
	return &youtubeSearchBudget{maxPerRequest: maxPerRequest, dailyLimiter: limiter}
}

func (b *youtubeSearchBudget) allow() bool {
	if b == nil {
		return false
	}
	return b.used < b.maxPerRequest
}

func (b *youtubeSearchBudget) consume() bool {
	if !b.allow() {
		return false
	}
	if b.dailyLimiter != nil && !b.dailyLimiter.TryConsume() {
		return false
	}
	b.used++
	return true
}

func generateNicheBlueprints(profile CreatorNicheProfile) []nicheBlueprint {
	topic := firstNonEmpty(profile.OptionalBroadTopic, profile.TeachingSubjects, profile.ProfessionalSkills, profile.LivedExperiences, profile.Hobbies)
	audience := firstNonEmpty(profile.TargetAudience, countryAudience(profile.TargetCountry))
	format := primaryFormat(profile.ContentFormats)
	routes := normalizeMonetizationRoutes(profile.PrimaryMonetizationGoal)
	baseProblem := firstNonEmpty(profile.ThreeYearsAgoAdvice, "needs practical, trustworthy guidance")

	var out []nicheBlueprint
	add := func(level1, level2, level3, phrase, problem, advantage string) {
		out = append(out, nicheBlueprint{
			Level1:        titleCase(level1),
			Level2:        titleCase(level2),
			Level3:        sentenceCase(level3),
			CorePhrase:    strings.TrimSpace(phrase),
			TargetViewer:  audience,
			ViewerProblem: problem,
			Advantage:     advantage,
			Format:        format,
			Routes:        routes,
			QueryPhrases: unique([]string{
				phrase,
				phrase + " " + audience,
				level2 + " tutorial " + audience,
				level3,
			}),
			Reasoning: "Candidate derived from the creator profile; demand, competition, monetization, and outlier metrics are measured separately.",
		})
	}

	lowTopic := strings.ToLower(topic)
	switch {
	case strings.Contains(lowTopic, "ai") || strings.Contains(lowTopic, "automation") || strings.Contains(strings.ToLower(profile.ProfessionalSkills), "software"):
		add("Technology", "AI automation", "AI workflow automation for "+audience, "AI workflow automation "+audience, "wants practical automations without hiring a developer", "software and automation experience gives credible workflows")
		add("Business", "Small business systems", "no-code AI systems for "+audience, "no code AI systems "+audience, "needs lower-cost systems for marketing, admin, and operations", "can translate technical automation into business outcomes")
		add("Education", "AI skills training", "beginner AI tutorials for "+audience, "beginner AI tutorials "+audience, "needs a clear path from curiosity to useful skill", "can teach what changed and what still matters")
	case strings.Contains(lowTopic, "visa") || strings.Contains(strings.ToLower(profile.LivedExperiences), "visa") || strings.Contains(strings.ToLower(profile.TargetAudience), "student"):
		add("Education", "International student guidance", "UK student visa guidance for "+audience, "UK student visa guidance "+audience, "needs current, practical steps without confusing jargon", "lived experience creates trust and empathy")
		add("Careers", "Graduate route planning", "UK graduate visa and career planning for "+audience, "UK graduate visa career planning "+audience, "needs a realistic route after study", "can explain what they wish they knew earlier")
		add("Personal finance", "Student life logistics", "UK student money and housing guidance for "+audience, "UK international student money housing "+audience, "needs practical decisions before arriving", "can connect visa, work, money, and local context")
	default:
		add("Education", topic, topic+" for "+audience, topic+" "+audience, baseProblem, "profile shows direct interest or experience")
		add("How-to", topic+" tutorials", "beginner "+topic+" tutorials for "+audience, "beginner "+topic+" tutorial "+audience, "needs step-by-step help", "can teach from personal experience")
		add("Reviews", topic+" tools", topic+" tools and comparisons for "+audience, topic+" tools review "+audience, "needs help choosing what to use", "can evaluate options through a creator lens")
	}
	if len(out) < 5 {
		add("Community", topic+" questions", "practical Q&A for "+audience+" about "+topic, topic+" questions "+audience, "has repeated questions not answered in one place", "can build a repeatable Q&A format")
	}
	return uniqueBlueprints(out)
}

func collectNicheEvidence(ctx context.Context, provider NicheEvidenceProvider, bp nicheBlueprint, profile CreatorNicheProfile) (nicheEvidence, error) {
	seen := map[string]bool{}
	videos := []ChannelVideoSummary{}
	budget := 0
	for _, phrase := range topN(bp.QueryPhrases, 3) {
		found, err := provider.SearchVideos(ctx, phrase, profile.TargetCountry, profile.TargetLanguage, 12)
		budget++
		if err != nil {
			return nicheEvidence{budgetUsed: budget}, err
		}
		for _, video := range found {
			key := firstNonEmpty(video.VideoID, strings.ToLower(video.Title+"|"+video.ChannelTitle))
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			videos = append(videos, video)
			if len(videos) >= 30 {
				break
			}
		}
		if len(videos) >= 30 {
			break
		}
	}
	if len(videos) == 0 {
		fallback := strings.TrimSpace(firstNonEmpty(profile.OptionalBroadTopic, bp.Level2) + " " + bp.TargetViewer)
		found, err := provider.SearchVideos(ctx, fallback, profile.TargetCountry, profile.TargetLanguage, 12)
		budget++
		if err == nil {
			for _, video := range found {
				key := firstNonEmpty(video.VideoID, strings.ToLower(video.Title+"|"+video.ChannelTitle))
				if key == "" || seen[key] {
					continue
				}
				seen[key] = true
				videos = append(videos, video)
			}
		} else {
			return nicheEvidence{budgetUsed: budget}, err
		}
	}
	if len(videos) == 0 && strings.TrimSpace(profile.OptionalBroadTopic) != "" {
		found, err := provider.SearchVideos(ctx, profile.OptionalBroadTopic, profile.TargetCountry, profile.TargetLanguage, 12)
		budget++
		if err == nil {
			for _, video := range found {
				key := firstNonEmpty(video.VideoID, strings.ToLower(video.Title+"|"+video.ChannelTitle))
				if key == "" || seen[key] {
					continue
				}
				seen[key] = true
				videos = append(videos, video)
			}
		} else {
			return nicheEvidence{budgetUsed: budget}, err
		}
	}
	channelIDs := []string{}
	for _, video := range videos {
		if video.ChannelID != "" {
			channelIDs = append(channelIDs, video.ChannelID)
		}
	}
	stats, err := provider.ChannelStats(ctx, unique(channelIDs))
	if err != nil {
		return nicheEvidence{videos: videos, budgetUsed: budget}, err
	}
	if stats == nil {
		stats = map[string]NicheChannelStats{}
	}
	return nicheEvidence{videos: videos, channelStats: stats, budgetUsed: budget}, nil
}

func collectBudgetedNicheEvidence(ctx context.Context, provider NicheEvidenceProvider, cache *YouTubeEvidenceCache, budget *youtubeSearchBudget, query string, profile CreatorNicheProfile, now time.Time) (nicheEvidence, error) {
	query = normalizeEvidenceQuery(query)
	if query == "" {
		return nicheEvidence{mode: evidenceModeAIStrategicAnalysis, collectedAt: now}, nil
	}
	key := youtubeEvidenceCacheKey(query, profile.TargetCountry, profile.TargetLanguage, "120d")
	if cache != nil {
		if ev, _, ok, fresh := cache.get(key, now); ok {
			if fresh {
				return ev, nil
			}
			if provider == nil || provider.Status().Status != StatusActive || !budget.allow() {
				return ev, nil
			}
		}
	}
	if provider == nil || provider.Status().Status != StatusActive || !budget.consume() {
		return nicheEvidence{query: query, mode: evidenceModeLimitedEvidence, collectedAt: now}, nil
	}
	videos, err := provider.SearchVideos(ctx, query, profile.TargetCountry, profile.TargetLanguage, 12)
	if err != nil {
		if cache != nil {
			if ev, _, ok, _ := cache.get(key, now); ok {
				return ev, nil
			}
		}
		return nicheEvidence{query: query, budgetUsed: budget.used, mode: evidenceModeLimitedEvidence, collectedAt: now}, err
	}
	channelIDs := []string{}
	for _, video := range videos {
		if video.ChannelID != "" {
			channelIDs = append(channelIDs, video.ChannelID)
		}
	}
	stats := map[string]NicheChannelStats{}
	if len(channelIDs) > 0 {
		found, err := provider.ChannelStats(ctx, unique(channelIDs))
		if err != nil {
			return nicheEvidence{query: query, videos: videos, budgetUsed: budget.used, mode: evidenceModeLimitedEvidence, collectedAt: now}, err
		}
		stats = found
	}
	ev := nicheEvidence{videos: videos, channelStats: stats, budgetUsed: budget.used, query: query, mode: evidenceModeLiveValidated, collectedAt: now}
	cache.set(key, ev, now)
	return ev, nil
}

func normalizeEvidenceQuery(query string) string {
	query = strings.ToLower(strings.TrimSpace(query))
	query = strings.Join(strings.Fields(query), " ")
	if len([]rune(query)) > 110 {
		query = string([]rune(query)[:110])
	}
	return query
}

func youtubeEvidenceCacheKey(query, country, language, window string) string {
	parts := []string{normalizeEvidenceQuery(query), strings.ToUpper(strings.TrimSpace(country)), strings.ToLower(strings.Split(strings.TrimSpace(language), "-")[0]), strings.TrimSpace(window)}
	sum := sha1.Sum([]byte(strings.Join(parts, "|")))
	return "yt_evidence_" + hex.EncodeToString(sum[:])
}

func trimForLog(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 500 {
		return text[:500] + "..."
	}
	return text
}

func classifyNicheResearchError(err error) *NicheResearchError {
	var researchErr *NicheResearchError
	if errors.As(err, &researchErr) {
		return researchErr
	}
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		code := NicheStatusResearchFailed
		msg := NicheMessageTemporarilyUnavailable
		temporary := false
		switch {
		case providerErr.Code == ProviderErrorQuota || providerErr.Reason == "quotaExceeded" || providerErr.Reason == "dailyLimitExceeded" || providerErr.Reason == "rateLimitExceeded":
			code = NicheStatusQuotaTemporarilyUnavailable
			msg = NicheMessageQuotaUnavailable
			temporary = true
		case providerErr.Code == ProviderErrorCredentials:
			code = NicheStatusCredentialsInvalid
			msg = "Local research configuration is invalid."
		case providerErr.Code == ProviderErrorTimeout:
			code = NicheStatusValidationTimeout
			msg = NicheMessageTemporarilyUnavailable
			temporary = true
		case providerErr.Code == ProviderErrorTemporary || providerErr.HTTPStatus >= 500:
			code = NicheStatusProviderTemporarilyUnavailable
			msg = NicheMessageTemporarilyUnavailable
			temporary = true
		}
		return &NicheResearchError{Code: code, Message: msg, HTTPStatus: providerErr.HTTPStatus, Reason: providerErr.Reason, Temporary: temporary, Err: err}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &NicheResearchError{Code: NicheStatusValidationTimeout, Message: NicheMessageTemporarilyUnavailable, Temporary: true, Err: err}
	}
	return &NicheResearchError{Code: NicheStatusResearchFailed, Message: NicheMessageTemporarilyUnavailable, Err: err}
}

func buildNicheCandidate(bp nicheBlueprint, profile CreatorNicheProfile, evidence nicheEvidence, rising []string, googleAdsStatus ProviderStatus, calibration *NicheRPMCalibration, now time.Time) NicheCandidate {
	validation := buildNicheValidation(bp, evidence.videos, rising, now)
	outliers := detectOutliers(evidence.videos, evidence.channelStats, now)
	topics, pillars, sustainability := buildVideoSustainability(bp, profile, evidence.videos)
	gaps := buildSupplyGaps(bp, validation, evidence.videos, outliers, profile)
	monetization := buildMonetizationEstimate(bp, profile, googleAdsStatus, calibration, validation)
	scores := scoreNiche(bp, profile, validation, outliers, gaps, monetization, sustainability)
	risks := nicheRisks(validation, sustainability, monetization)
	return NicheCandidate{
		ID:                       stableNicheID(bp.Level3, profile.TargetCountry, profile.TargetLanguage, primaryFormat(profile.ContentFormats)),
		Level1:                   bp.Level1,
		Level2:                   bp.Level2,
		Level3:                   bp.Level3,
		NicheName:                bp.Level3,
		CorePhrase:               bp.CorePhrase,
		TargetViewer:             bp.TargetViewer,
		ViewerProblem:            bp.ViewerProblem,
		CreatorAdvantage:         bp.Advantage,
		RecommendedContentFormat: bp.Format,
		MonetizationRoutes:       bp.Routes,
		Validation:               validation,
		Outliers:                 topOutliers(outliers, 4),
		SupplyGaps:               gaps,
		Monetization:             monetization,
		VideoTopics:              topics,
		TopicPillars:             pillars,
		First10Titles:            firstTopicTitles(topics, 10),
		Sustainability:           sustainability,
		Scores:                   scores,
		Risks:                    risks,
		RecommendedFirstAction:   recommendedFirstAction(bp, validation, sustainability),
		GeneratedReasoning:       bp.Reasoning,
	}
}

func validateNicheDrafts(drafts []NicheDraft, profile CreatorNicheProfile) []NicheDraft {
	out := []NicheDraft{}
	seen := map[string]bool{}
	for _, draft := range drafts {
		draft.Name = strings.TrimSpace(draft.Name)
		if draft.Name == "" {
			continue
		}
		key := strings.ToLower(draft.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		if draft.TargetAudience == "" {
			draft.TargetAudience = firstNonEmpty(profile.TargetAudience, countryAudience(profile.TargetCountry))
		}
		draft.RecommendedTitles = validateVideoTitles(draft.RecommendedTitles, draft.ContentPillars, draft.Name, draft.TargetAudience)
		draft.ContentPillars = normalizeContentPillars(draft.ContentPillars, draft.RecommendedTitles)
		if len(draft.RecommendedTitles) < 10 {
			draft.RecommendedTitles = append(draft.RecommendedTitles, fallbackTitles(draft, 10-len(draft.RecommendedTitles))...)
			draft.ContentPillars = normalizeContentPillars(draft.ContentPillars, draft.RecommendedTitles)
		}
		if len(draft.RecommendedTitles) > 50 {
			draft.RecommendedTitles = draft.RecommendedTitles[:50]
		}
		out = append(out, draft)
	}
	return out
}

func buildAICandidate(draft NicheDraft, profile CreatorNicheProfile, rising []string, now time.Time) NicheCandidate {
	dimensions := normalizeDimensions(draft)
	overall := calculateNicheOverallScore(dimensions)
	confidenceScore := round1((dimensions.CreatorFit.Score + dimensions.AudienceDemand.Score + dimensions.Sustainability.Score) / 3)
	topics := draft.RecommendedTitles
	pillars := normalizeContentPillars(draft.ContentPillars, topics)
	runway := buildRunway(len(topics), profile.WeeklyProductionCapacity)
	status := evidenceModeAIStrategicAnalysis
	sourceTypes := []string{"openai_structured_strategy"}
	if phraseOverlaps(draft.Name, rising) || phraseOverlaps(draft.SearchQuery, rising) {
		status = evidenceModeTrendSupported
		sourceTypes = append(sourceTypes, "trend_signal")
	}
	marketEvidence := MarketEvidence{
		Status:         status,
		SourceTypes:    sourceTypes,
		SampleSize:     0,
		RecentActivity: "Unavailable",
		MedianViews:    nil,
		Engagement:     nil,
		CollectedAt:    nil,
		Limitations:    []string{"No live YouTube statistics are attached to this candidate yet."},
	}
	validation := NicheValidation{
		SearchPhrases:         topN(unique([]string{draft.SearchQuery, draft.Name, draft.Subcategory}), 3),
		MarketEvidenceSummary: "Strategic AI analysis using creator profile and available trend context. No live YouTube statistics are claimed for this candidate.",
		CompetitionLevel:      scoreLabel(dimensions.CompetitionOpportunity.Score),
		EvidenceConfidence:    confidenceLabel(confidenceScore),
	}
	sustainability := SustainabilityEvidence{
		ViableTopicCount:       len(topics),
		ContentPillarCount:     len(pillars),
		TopicRepetitionRisk:    repetitionRisk(len(topics), len(pillars)),
		EstimatedContentRunway: runway.EstimatedContentRunway,
		Score:                  dimensions.Sustainability.Score,
	}
	scores := NicheScores{
		PersonalFit:    dimensions.CreatorFit,
		Demand:         dimensions.AudienceDemand,
		OpportunityGap: dimensions.CompetitionOpportunity,
		Sustainability: dimensions.Sustainability,
		Overall:        ScoreExplanation{Score: overall, Label: scoreLabel(overall), Explanation: "Formula: 0.25 creator fit + 0.25 audience demand + 0.20 competition opportunity + 0.20 sustainability + 0.10 differentiation."},
		Confidence:     ScoreExplanation{Score: confidenceScore, Label: confidenceLabel(confidenceScore), Explanation: "Confidence reflects profile specificity, evidence availability, and title/pillar validation."},
	}
	return NicheCandidate{
		ID:                       stableNicheID(draft.ID, draft.Name, profile.TargetCountry, profile.TargetLanguage),
		Name:                     draft.Name,
		ConcisePositioning:       strings.TrimSpace(draft.ConcisePositioning),
		Category:                 firstNonEmpty(draft.Category, "Creator strategy"),
		Subcategory:              firstNonEmpty(draft.Subcategory, draft.Name),
		TargetAudience:           draft.TargetAudience,
		AudienceProblems:         cleanStringList(draft.AudienceProblems),
		CreatorAdvantages:        cleanStringList(draft.CreatorAdvantages),
		UniqueAngle:              strings.TrimSpace(draft.UniqueAngle),
		OverallScore:             overall,
		Confidence:               scores.Confidence.Label,
		Dimensions:               dimensions,
		ContentPillars:           pillars,
		TopicClusters:            draft.TopicClusters,
		RecommendedTitles:        topics,
		OpportunityGaps:          cleanStringList(draft.OpportunityGaps),
		EvidenceSummary:          validation.MarketEvidenceSummary,
		MarketEvidence:           marketEvidence,
		SearchQueriesUsed:        validation.SearchPhrases,
		Runway:                   runway,
		Level1:                   firstNonEmpty(draft.Category, "Creator strategy"),
		Level2:                   firstNonEmpty(draft.Subcategory, draft.Name),
		Level3:                   draft.Name,
		NicheName:                draft.Name,
		CorePhrase:               firstNonEmpty(draft.SearchQuery, draft.Name),
		TargetViewer:             draft.TargetAudience,
		ViewerProblem:            strings.Join(topN(cleanStringList(draft.AudienceProblems), 2), " "),
		CreatorAdvantage:         strings.Join(topN(cleanStringList(draft.CreatorAdvantages), 2), " "),
		RecommendedContentFormat: primaryFormat(profile.ContentFormats),
		Validation:               validation,
		Outliers:                 []OutlierEvidence{},
		SupplyGaps:               supplyGapsFromStrings(draft.OpportunityGaps),
		Monetization:             MonetizationEstimate{RPMEstimateAvailable: false, UnavailableReason: publicRPMUnavailableMessage, CommercialPotential: "Not evaluated", Confidence: "not_applicable"},
		VideoTopics:              topics,
		TopicPillars:             pillars,
		First10Titles:            firstTopicTitles(topics, 10),
		Sustainability:           sustainability,
		Scores:                   scores,
		Risks:                    cleanStringList(draft.Risks),
		RecommendedFirstAction:   strings.TrimSpace(draft.RecommendedFirstAction),
		GeneratedReasoning:       strings.TrimSpace(draft.Reasoning),
	}
}

func applyEvidenceToCandidate(candidate *NicheCandidate, evidence nicheEvidence, rising []string, now time.Time) {
	validation := buildNicheValidation(nicheBlueprint{CorePhrase: candidate.CorePhrase, QueryPhrases: []string{evidence.query}}, evidence.videos, rising, now)
	outliers := detectOutliers(evidence.videos, evidence.channelStats, now)
	candidate.Validation = validation
	candidate.Outliers = topOutliers(outliers, 4)
	candidate.SearchQueriesUsed = unique(append(candidate.SearchQueriesUsed, evidence.query))
	candidate.MarketEvidence = marketEvidenceFromValidation(validation, evidence)
	candidate.EvidenceSummary = validation.MarketEvidenceSummary
	candidate.Scores.Demand = ScoreExplanation{Score: scoreDemandValidation(validation), Label: scoreLabel(scoreDemandValidation(validation)), Explanation: validation.MarketEvidenceSummary}
	candidate.Scores.OpportunityGap = ScoreExplanation{Score: scoreOpportunityGap(validation, outliers, candidate.SupplyGaps), Label: scoreLabel(scoreOpportunityGap(validation, outliers, candidate.SupplyGaps)), Explanation: "Uses capped public evidence, outliers, and gap statements when validation is available."}
	candidate.Dimensions.AudienceDemand = candidate.Scores.Demand
	candidate.Dimensions.CompetitionOpportunity = candidate.Scores.OpportunityGap
	candidate.OverallScore = calculateNicheOverallScore(candidate.Dimensions)
	candidate.Scores.Overall.Score = candidate.OverallScore
	candidate.Scores.Overall.Label = scoreLabel(candidate.OverallScore)
}

func marketEvidenceFromValidation(v NicheValidation, ev nicheEvidence) MarketEvidence {
	var median *uint64
	if v.MedianSampledViews > 0 {
		value := v.MedianSampledViews
		median = &value
	}
	var engagement *float64
	if v.EngagementRate > 0 {
		value := v.EngagementRate
		engagement = &value
	}
	collected := ev.collectedAt
	return MarketEvidence{
		Status:         firstNonEmpty(ev.mode, evidenceModeLiveValidated),
		SourceTypes:    []string{"youtube_public_search", "youtube_public_video_statistics"},
		SampleSize:     v.SampledVideoCount,
		RecentActivity: firstNonEmpty(v.NewestActivity, "Unavailable"),
		MedianViews:    median,
		Engagement:     engagement,
		CollectedAt:    &collected,
		Limitations:    []string{"A capped YouTube sample validates public activity only; it does not represent full market size or private channel analytics."},
	}
}

func videosWithPublicMetrics(videos []ChannelVideoSummary) int {
	count := 0
	for _, video := range videos {
		if video.Views != nil && *video.Views > 0 {
			count++
		}
	}
	return count
}

func buildNicheValidation(bp nicheBlueprint, videos []ChannelVideoSummary, rising []string, now time.Time) NicheValidation {
	recent := 0
	var newest time.Time
	totalViews := uint64(0)
	viewValues := []uint64{}
	likes := uint64(0)
	comments := uint64(0)
	for _, video := range videos {
		if video.Views != nil {
			totalViews += *video.Views
			viewValues = append(viewValues, *video.Views)
		}
		if video.Likes != nil {
			likes += *video.Likes
		}
		if video.Comments != nil {
			comments += *video.Comments
		}
		if t, err := time.Parse(time.RFC3339, video.PublishedAt); err == nil {
			if newest.IsZero() || t.After(newest) {
				newest = t
			}
			if now.Sub(t) <= 120*24*time.Hour {
				recent++
			}
		}
	}
	median := medianUint64(viewValues)
	engagement := 0.0
	if totalViews > 0 {
		engagement = round2(float64(likes+comments) / float64(totalViews) * 100)
	}
	overlap := phraseOverlaps(bp.CorePhrase, rising)
	comp := competitionLabel(len(videos), median)
	conf := "low"
	if len(videos) >= 20 && median > 0 {
		conf = "medium"
	}
	if len(videos) >= 25 && recent >= 8 && median >= 10000 {
		conf = "high"
	}
	newestActivity := ""
	if !newest.IsZero() {
		newestActivity = humanAge(now.Sub(newest)) + " ago"
	}
	return NicheValidation{
		SearchPhrases:           topN(bp.QueryPhrases, 4),
		RecentPublicationVolume: recent,
		SampledVideoCount:       len(videos),
		TotalSampledViews:       totalViews,
		MedianSampledViews:      median,
		EngagementRate:          engagement,
		NewestActivity:          newestActivity,
		RisingTopicOverlap:      overlap,
		CompetitionLevel:        comp,
		ValidationBudgetUsed:    minInt(len(bp.QueryPhrases), 3),
		EvidenceConfidence:      conf,
		MarketEvidenceSummary:   marketEvidenceSummary(len(videos), recent, median, overlap),
	}
}

func detectOutliers(videos []ChannelVideoSummary, channels map[string]NicheChannelStats, now time.Time) []OutlierEvidence {
	baseline := medianVideoViews(videos)
	out := []OutlierEvidence{}
	for _, video := range videos {
		if video.Views == nil || *video.Views == 0 {
			continue
		}
		views := *video.Views
		ch := channels[video.ChannelID]
		subs := uint64(0)
		if ch.Subscribers != nil {
			subs = *ch.Subscribers
		}
		relativeToSubs := 0.0
		if subs > 0 {
			relativeToSubs = float64(views) / float64(subs)
		}
		relativeToSample := 0.0
		if baseline > 0 {
			relativeToSample = float64(views) / float64(baseline)
		}
		engagement := 0.0
		if video.Likes != nil {
			engagement += float64(*video.Likes)
		}
		if video.Comments != nil {
			engagement += float64(*video.Comments)
		}
		engagementRate := 0.0
		if views > 0 {
			engagementRate = engagement / float64(views)
		}
		recentBoost := 0.0
		age := ""
		if t, err := time.Parse(time.RFC3339, video.PublishedAt); err == nil {
			d := now.Sub(t)
			age = humanAge(d) + " ago"
			if d <= 30*24*time.Hour {
				recentBoost = 1.2
			} else if d <= 120*24*time.Hour {
				recentBoost = 0.6
			}
		}
		strength := 0.0
		if relativeToSubs >= 1.2 && subs <= 250000 {
			strength += minFloat(45, relativeToSubs*12)
		}
		if relativeToSample >= 2 {
			strength += minFloat(35, relativeToSample*8)
		}
		if engagementRate >= 0.015 {
			strength += 10
		}
		strength += recentBoost * 8
		strength = round1(clampScore(strength))
		if strength < 35 {
			continue
		}
		reasons := []string{}
		if subs == 0 {
			reasons = append(reasons, "subscriber count unavailable or zero")
		} else if relativeToSubs >= 1.2 {
			reasons = append(reasons, "views materially exceed channel subscriber base")
		}
		if relativeToSample >= 2 {
			reasons = append(reasons, "views are well above the sampled niche baseline")
		}
		if recentBoost > 0 {
			reasons = append(reasons, "recent activity shows velocity")
		}
		if engagementRate >= 0.015 {
			reasons = append(reasons, "engagement is strong relative to views")
		}
		out = append(out, OutlierEvidence{
			Title:          video.Title,
			ThumbnailURL:   video.ThumbnailURL,
			CanonicalURL:   "https://www.youtube.com/watch?v=" + video.VideoID,
			ChannelName:    firstNonEmpty(video.ChannelTitle, ch.Title),
			PublicationAge: age,
			PublicViews:    views,
			Reason:         strings.Join(reasons, "; "),
			Strength:       strength,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Strength > out[j].Strength })
	return out
}

func buildSupplyGaps(bp nicheBlueprint, validation NicheValidation, videos []ChannelVideoSummary, outliers []OutlierEvidence, profile CreatorNicheProfile) []SupplyGap {
	gaps := []SupplyGap{}
	if validation.MedianSampledViews >= 10000 && validation.RecentPublicationVolume <= 6 {
		gaps = append(gaps, SupplyGap{Statement: "Strong demand but few recent beginner tutorials", Evidence: []string{"Median sampled views are meaningful", "Recent publication volume is low"}, Confidence: "medium"})
	}
	if validation.RisingTopicOverlap && validation.RecentPublicationVolume < 10 {
		gaps = append(gaps, SupplyGap{Statement: "Current interest with limited recent coverage", Evidence: []string{"Candidate overlaps rising public topics", "Recent video supply is limited"}, Confidence: "medium"})
	}
	if strings.EqualFold(profile.TargetCountry, "GB") && !titlesContain(videos, []string{"uk", "britain", "london", "gb"}) {
		gaps = append(gaps, SupplyGap{Statement: "High interest with weak UK-specific coverage", Evidence: []string{"Target market is GB", "Sampled titles rarely mention UK-specific context"}, Confidence: "medium"})
	}
	if len(outliers) > 0 {
		gaps = append(gaps, SupplyGap{Statement: "Small or mid-sized channels can still break through with specific packaging", Evidence: []string{outliers[0].Title}, Confidence: "medium"})
	}
	if questionSignalCount(videos) >= 3 {
		gaps = append(gaps, SupplyGap{Statement: "Recurring viewer questions appear in public titles and descriptions", Evidence: []string{"Multiple sampled titles use question-led framing"}, Confidence: "low"})
	}
	if len(gaps) == 0 {
		gaps = append(gaps, SupplyGap{Statement: "Gap evidence is limited; validate with a pilot video before committing heavily", Evidence: []string{"Sample does not show a strong supply gap yet"}, Confidence: "low"})
	}
	return topSupplyGaps(gaps, 4)
}

func buildMonetizationEstimate(bp nicheBlueprint, profile CreatorNicheProfile, googleAdsStatus ProviderStatus, calibration *NicheRPMCalibration, validation NicheValidation) MonetizationEstimate {
	signals := []string{}
	score := 45.0
	haystack := strings.ToLower(strings.Join([]string{bp.Level1, bp.Level2, bp.Level3, profile.PrimaryMonetizationGoal, profile.TargetAudience}, " "))
	commercialTerms := []string{"business", "software", "automation", "ai", "services", "leads", "affiliate", "tools", "finance", "visa", "education", "career", "legal", "insurance"}
	for _, term := range commercialTerms {
		if strings.Contains(haystack, term) {
			score += 5
			signals = append(signals, "commercial-intent term: "+term)
		}
	}
	if strings.EqualFold(profile.TargetCountry, "GB") || strings.EqualFold(profile.TargetCountry, "US") {
		score += 8
		signals = append(signals, "target market has stronger advertiser demand")
	}
	if len(bp.Routes) >= 2 {
		score += 8
		signals = append(signals, "multiple monetization routes fit the audience")
	}
	if googleAdsStatus.Status == StatusActive {
		score += 7
		signals = append(signals, "advertiser-demand metrics can be used when enabled")
	} else {
		signals = append(signals, "advertiser-demand metrics are not configured")
	}
	score = round1(clampScore(score))
	format := primaryFormat(profile.ContentFormats)
	estimate := MonetizationEstimate{
		Mode:                    "market_estimate",
		CommercialPotential:     commercialPotential(score),
		Score:                   score,
		RPMEstimateAvailable:    false,
		Confidence:              monetizationConfidence(score, googleAdsStatus),
		CalibrationType:         "none",
		CalculationAssumptions:  []string{"Public niche research uses market signals only; exact video revenue requires authorized owned-channel analytics."},
		UnavailableReason:       publicRPMUnavailableMessage,
		Format:                  format,
		TargetMarket:            profile.TargetCountry,
		AdvertiserDemandSignals: topN(signals, 8),
		EstimateDisclaimer:      "This is a market estimate, not actual RPM, guaranteed earnings, or an exact platform payout.",
	}
	if calibration != nil && calibrationMatches(*calibration, profile.TargetCountry, format) {
		estimate.RPMEstimateAvailable = true
		estimate.Currency = calibration.Currency
		estimate.RPMLow = &calibration.Low
		estimate.RPMMidpoint = &calibration.Midpoint
		estimate.RPMHigh = &calibration.High
		estimate.Confidence = calibration.Confidence
		estimate.CalibrationType = calibration.Type
		estimate.CalibrationAge = humanAge(time.Since(calibration.CalibratedAt))
		estimate.CalculationAssumptions = append([]string{}, calibration.Assumptions...)
		estimate.EstimatedEarnings = earningsProjections(*calibration)
		estimate.UnavailableReason = ""
	}
	if len(profile.ContentFormats) > 1 && containsExactString(profile.ContentFormats, "both") {
		estimate.RPMEstimateAvailable = false
		estimate.Currency = ""
		estimate.RPMLow = nil
		estimate.RPMMidpoint = nil
		estimate.RPMHigh = nil
		estimate.EstimatedEarnings = nil
		estimate.UnavailableReason = "Combined long-form and Shorts currency estimates are unavailable unless each format has separate calibration."
	}
	_ = validation
	return estimate
}

func buildVideoSustainability(bp nicheBlueprint, profile CreatorNicheProfile, videos []ChannelVideoSummary) ([]VideoTopic, []ContentPillar, SustainabilityEvidence) {
	pillars := []string{"Beginner foundations", "Tools and workflows", "Mistakes and fixes", "Case studies", "Market updates"}
	if strings.Contains(strings.ToLower(bp.Level3), "visa") || strings.Contains(strings.ToLower(bp.Level3), "student") {
		pillars = []string{"Application steps", "Mistakes and refusals", "Money and housing", "Career route", "Case studies"}
	}
	if strings.Contains(strings.ToLower(bp.Level3), "business") || strings.Contains(strings.ToLower(bp.Level3), "automation") {
		pillars = []string{"Beginner workflows", "Tool comparisons", "Client case studies", "Pricing and ROI", "Mistakes and fixes"}
	}
	raw := []VideoTopic{}
	intents := []string{"tutorial", "comparison", "case study", "mistake", "checklist", "explainer", "Q&A", "update", "workflow", "strategy"}
	for _, pillar := range pillars {
		for _, intent := range intents {
			raw = append(raw, VideoTopic{
				Title:          topicTitle(intent, bp.CorePhrase, bp.TargetViewer, pillar),
				Pillar:         pillar,
				Intent:         intent,
				Difficulty:     topicDifficulty(intent, profile.WeeklyProductionCapacity),
				Source:         "generated from creator profile and niche structure",
				EvidenceStatus: "unvalidated idea",
			})
		}
	}
	for _, video := range topNChannelVideos(videos, 10) {
		kw := ExtractKeywordIntelligence(KeywordExtractionInput{Title: video.Title})
		core := firstString(kw.PrimaryKeywords)
		if core == "" {
			continue
		}
		raw = append(raw, VideoTopic{Title: "What " + titleCase(core) + " Means for " + bp.TargetViewer, Pillar: "Market updates", Intent: "response", Difficulty: "medium", Source: "derived from sampled public video patterns", EvidenceStatus: "related opportunity"})
	}
	topics, removed := dedupeVideoTopics(raw, 50)
	pillarCounts := map[string]int{}
	for _, topic := range topics {
		pillarCounts[topic.Pillar]++
	}
	pillarSummary := []ContentPillar{}
	for _, pillar := range pillars {
		if pillarCounts[pillar] > 0 {
			pillarSummary = append(pillarSummary, ContentPillar{Name: pillar, TopicCount: pillarCounts[pillar]})
		}
	}
	risk := "low"
	if len(topics) < 30 {
		risk = "high"
	} else if removed > 12 {
		risk = "medium"
	}
	score := round1(clampScore(float64(len(topics))*1.4 + float64(len(pillarSummary))*6))
	warning := ""
	if len(topics) < 30 {
		warning = "Fewer than 30 credible topics remain after deduplication."
	}
	return topics, pillarSummary, SustainabilityEvidence{
		ViableTopicCount:       len(topics),
		ContentPillarCount:     len(pillarSummary),
		TopicRepetitionRisk:    risk,
		EstimatedContentRunway: estimateRunway(len(topics), profile.WeeklyProductionCapacity),
		Score:                  score,
		Warning:                warning,
		DedupedRemoved:         removed,
	}
}

func scoreNiche(bp nicheBlueprint, profile CreatorNicheProfile, validation NicheValidation, outliers []OutlierEvidence, gaps []SupplyGap, monetization MonetizationEstimate, sustainability SustainabilityEvidence) NicheScores {
	personalFit := scorePersonalFit(bp, profile)
	demand := scoreDemandValidation(validation)
	gapScore := scoreOpportunityGap(validation, outliers, gaps)
	sustainabilityScore := sustainability.Score
	differentiationScore := round1(clampScore((personalFit + gapScore) / 2))
	overall := round1(clampScore(
		0.25*personalFit +
			0.25*demand +
			0.20*gapScore +
			0.20*sustainabilityScore +
			0.10*differentiationScore,
	))
	conf := scoreConfidence(validation, monetization, sustainability)
	return NicheScores{
		PersonalFit:    ScoreExplanation{Score: personalFit, Label: scoreLabel(personalFit), Explanation: "Considers expertise, lived experience, audience understanding, and preferred creator format."},
		Demand:         ScoreExplanation{Score: demand, Label: scoreLabel(demand), Explanation: validation.MarketEvidenceSummary},
		OpportunityGap: ScoreExplanation{Score: gapScore, Label: scoreLabel(gapScore), Explanation: "Rewards real demand, manageable supply, outlier evidence, and supported gap statements."},
		Monetization:   ScoreExplanation{Score: 0, Label: "Not scored", Explanation: "Monetisation is not part of the Niche Finder score."},
		Sustainability: ScoreExplanation{Score: sustainabilityScore, Label: scoreLabel(sustainabilityScore), Explanation: "Based on distinct topic count, pillar depth, creator interest, and production practicality."},
		Overall:        ScoreExplanation{Score: overall, Label: scoreLabel(overall), Explanation: "Formula: 0.25 creator fit + 0.25 audience demand + 0.20 competition opportunity + 0.20 sustainability + 0.10 differentiation."},
		Confidence:     ScoreExplanation{Score: conf, Label: confidenceLabel(conf), Explanation: "Confidence is separate from score and reflects evidence coverage, monetization calibration, and topic depth."},
	}
}

func normalizeDimensions(draft NicheDraft) NicheScoreDimensions {
	return NicheScoreDimensions{
		CreatorFit:             scoreFromDraft(draft.DimensionScores.CreatorFit, draft.DimensionReasoning.CreatorFit),
		AudienceDemand:         scoreFromDraft(draft.DimensionScores.AudienceDemand, draft.DimensionReasoning.AudienceDemand),
		CompetitionOpportunity: scoreFromDraft(draft.DimensionScores.CompetitionOpportunity, draft.DimensionReasoning.CompetitionOpportunity),
		Sustainability:         scoreFromDraft(draft.DimensionScores.Sustainability, draft.DimensionReasoning.Sustainability),
		Differentiation:        scoreFromDraft(draft.DimensionScores.Differentiation, draft.DimensionReasoning.Differentiation),
	}
}

func scoreFromDraft(score float64, explanation string) ScoreExplanation {
	score = round1(clampScore(score))
	if score == 0 {
		score = 50
	}
	return ScoreExplanation{Score: score, Label: scoreLabel(score), Explanation: firstNonEmpty(strings.TrimSpace(explanation), "Model-provided dimension reasoning was unavailable.")}
}

func calculateNicheOverallScore(d NicheScoreDimensions) float64 {
	return round1(clampScore(
		0.25*d.CreatorFit.Score +
			0.25*d.AudienceDemand.Score +
			0.20*d.CompetitionOpportunity.Score +
			0.20*d.Sustainability.Score +
			0.10*d.Differentiation.Score,
	))
}

func validateVideoTitles(titles []VideoTopic, pillars []ContentPillar, niche, audience string) []VideoTopic {
	validPillars := map[string]bool{}
	for _, pillar := range pillars {
		if strings.TrimSpace(pillar.Name) != "" {
			validPillars[strings.ToLower(strings.TrimSpace(pillar.Name))] = true
		}
	}
	out := []VideoTopic{}
	seen := map[string]bool{}
	for _, topic := range titles {
		title := strings.Join(strings.Fields(strings.TrimSpace(topic.Title)), " ")
		key := strings.ToLower(title)
		if title == "" || seen[key] || len([]rune(title)) > 95 || excessiveTitleRepetition(title, niche, audience) || malformedTitle(title) || unsupportedCurrentClaim(title) {
			continue
		}
		seen[key] = true
		topic.Title = title
		if strings.TrimSpace(topic.Pillar) == "" || (len(validPillars) > 0 && !validPillars[strings.ToLower(strings.TrimSpace(topic.Pillar))]) {
			if len(pillars) > 0 {
				topic.Pillar = pillars[len(out)%len(pillars)].Name
			} else {
				topic.Pillar = "Core videos"
			}
		}
		topic.Intent = firstNonEmpty(topic.Intent, inferTitleIntent(title))
		topic.Difficulty = firstNonEmpty(topic.Difficulty, "medium")
		topic.Source = firstNonEmpty(topic.Source, "openai_structured_strategy")
		topic.EvidenceStatus = firstNonEmpty(topic.EvidenceStatus, "ai_strategic_analysis")
		out = append(out, topic)
	}
	return out
}

func excessiveTitleRepetition(title, niche, audience string) bool {
	lower := strings.ToLower(title)
	for _, phrase := range []string{strings.ToLower(niche), strings.ToLower(audience), "vibe coder, students", "vibe coder students"} {
		phrase = strings.TrimSpace(phrase)
		if phrase == "" {
			continue
		}
		if strings.Count(lower, phrase) > 1 {
			return true
		}
		if len(strings.Fields(phrase)) >= 4 && strings.Contains(lower, phrase) {
			return true
		}
	}
	words := strings.Fields(lower)
	counts := map[string]int{}
	for _, word := range words {
		word = strings.Trim(word, ".,:;!?()[]")
		if len(word) < 5 {
			continue
		}
		counts[word]++
		if counts[word] >= 3 {
			return true
		}
	}
	return false
}

func malformedTitle(title string) bool {
	if strings.Contains(title, "??") || strings.Contains(title, "!!") || strings.Contains(title, "::") || strings.Contains(title, "  ") {
		return true
	}
	trimmed := strings.TrimSpace(title)
	return strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, ":") || strings.HasSuffix(trimmed, ":")
}

func unsupportedCurrentClaim(title string) bool {
	lower := strings.ToLower(title)
	currentMarkers := []string{"new law", "visa rule", "tax rule", "guaranteed", "will happen", "confirmed", "breaking"}
	claimDomains := []string{"visa", "legal", "law", "tax", "financial", "investment", "mortgage", "immigration"}
	for _, marker := range currentMarkers {
		if !strings.Contains(lower, marker) {
			continue
		}
		for _, domain := range claimDomains {
			if strings.Contains(lower, domain) {
				return true
			}
		}
	}
	return false
}

func normalizeContentPillars(pillars []ContentPillar, topics []VideoTopic) []ContentPillar {
	if len(pillars) == 0 {
		pillars = []ContentPillar{{Name: "Foundations"}, {Name: "Workflows"}, {Name: "Case studies"}, {Name: "Mistakes"}, {Name: "Comparisons"}}
	}
	counts := map[string]int{}
	examples := map[string][]string{}
	for _, topic := range topics {
		pillar := strings.TrimSpace(topic.Pillar)
		if pillar == "" {
			continue
		}
		counts[pillar]++
		if len(examples[pillar]) < 3 {
			examples[pillar] = append(examples[pillar], topic.Title)
		}
	}
	out := []ContentPillar{}
	indexByName := map[string]int{}
	for _, pillar := range pillars {
		name := strings.TrimSpace(pillar.Name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		count := counts[name]
		if count == 0 && pillar.TopicCount > 0 {
			count = pillar.TopicCount
		}
		if count == 0 {
			count = 1
			if len(topics)/len(pillars) > count {
				count = len(topics) / len(pillars)
			}
		}
		pct := pillar.Percentage
		if pct <= 0 && len(topics) > 0 {
			pct = round1(float64(count) / float64(len(topics)) * 100)
		}
		normalized := ContentPillar{Name: name, Description: pillar.Description, Percentage: pct, TopicCount: count, ExampleTitles: appendStringGroups(pillar.ExampleTitles, examples[name])}
		if existing, ok := indexByName[key]; ok {
			out[existing].TopicCount += normalized.TopicCount
			out[existing].Percentage += normalized.Percentage
			out[existing].ExampleTitles = appendStringGroups(out[existing].ExampleTitles, normalized.ExampleTitles)
			if out[existing].Description == "" {
				out[existing].Description = normalized.Description
			}
			continue
		}
		indexByName[key] = len(out)
		out = append(out, normalized)
	}
	totalPct := 0.0
	for _, pillar := range out {
		totalPct += pillar.Percentage
	}
	if len(out) > 0 && totalPct > 0 {
		adjustment := 100 / totalPct
		for i := range out {
			out[i].Percentage = round1(out[i].Percentage * adjustment)
		}
	}
	return out
}

func fallbackTitles(draft NicheDraft, count int) []VideoTopic {
	pillars := draft.ContentPillars
	if len(pillars) == 0 {
		pillars = []ContentPillar{{Name: "Foundations"}, {Name: "Workflows"}, {Name: "Case studies"}, {Name: "Mistakes"}, {Name: "Comparisons"}}
	}
	intents := []string{"tutorial", "comparison", "mistake", "case study", "opinion", "breakdown", "beginner guide", "experiment", "workflow", "checklist"}
	out := []VideoTopic{}
	for i := 0; i < count && i < len(intents); i++ {
		title := naturalTitle(intents[i], draft)
		out = append(out, VideoTopic{Title: title, Pillar: pillars[i%len(pillars)].Name, Intent: intents[i], Difficulty: "medium", Source: "backend_title_validation", EvidenceStatus: "ai_strategic_analysis"})
	}
	return out
}

func naturalTitle(intent string, draft NicheDraft) string {
	topic := firstNonEmpty(draft.Subcategory, draft.Name, "this workflow")
	switch intent {
	case "tutorial":
		return "Build a simple " + topic + " workflow from scratch"
	case "comparison":
		return "The fastest path versus the safest path"
	case "mistake":
		return "Five mistakes beginners make before they see results"
	case "case study":
		return "I tested the workflow on a realistic example"
	case "opinion":
		return "What most advice gets wrong about this niche"
	case "breakdown":
		return "A practical breakdown of the first week"
	case "beginner guide":
		return "Start here if you are completely new"
	case "experiment":
		return "I tried three approaches and tracked what changed"
	case "workflow":
		return "A repeatable weekly workflow you can copy"
	default:
		return "A practical checklist before you start"
	}
}

func inferTitleIntent(title string) string {
	lower := strings.ToLower(title)
	switch {
	case strings.Contains(lower, " vs ") || strings.Contains(lower, "versus"):
		return "comparison"
	case strings.Contains(lower, "mistake"):
		return "mistake"
	case strings.Contains(lower, "case study") || strings.Contains(lower, "tested"):
		return "case study"
	case strings.Contains(lower, "how to") || strings.Contains(lower, "build"):
		return "tutorial"
	case strings.Contains(lower, "beginner"):
		return "beginner guide"
	default:
		return "breakdown"
	}
}

func buildRunway(topicCount int, capacity string) ContentRunway {
	weekly := weeklyCapacity(capacity)
	if weekly <= 0 {
		weekly = 2
	}
	weeks := int(math.Ceil(float64(topicCount) / float64(weekly)))
	return ContentRunway{ViableTopicCount: topicCount, WeeklyCapacity: weekly, EstimatedWeeks: weeks, EstimatedContentRunway: estimateRunway(topicCount, capacity)}
}

func weeklyCapacity(value string) int {
	for _, field := range strings.Fields(value) {
		n, err := strconv.Atoi(strings.Trim(field, " ,./"))
		if err == nil && n > 0 && n < 30 {
			return n
		}
	}
	return 2
}

func repetitionRisk(topicCount, pillarCount int) string {
	if topicCount < 30 || pillarCount < 3 {
		return "high"
	}
	if topicCount < 45 {
		return "medium"
	}
	return "low"
}

func supplyGapsFromStrings(values []string) []SupplyGap {
	out := []SupplyGap{}
	for _, value := range cleanStringList(values) {
		out = append(out, SupplyGap{Statement: value, Confidence: "medium"})
	}
	return out
}

func bestCandidateSearchQuery(c NicheCandidate) string {
	return firstNonEmpty(firstString(c.SearchQueriesUsed), c.CorePhrase, c.Name, c.NicheName)
}

func evidenceFreshness(candidates []NicheCandidate) string {
	for _, c := range candidates {
		if c.MarketEvidence.Status == evidenceModeLiveValidated && c.MarketEvidence.CollectedAt != nil {
			return "live"
		}
	}
	for _, c := range candidates {
		if c.MarketEvidence.Status == evidenceModeCacheValidated && c.MarketEvidence.CollectedAt != nil {
			return "cached"
		}
	}
	return "strategic_only"
}

func analysisMode(candidates []NicheCandidate) string {
	for _, c := range candidates {
		if c.MarketEvidence.Status == evidenceModeLiveValidated {
			return evidenceModeLiveValidated
		}
	}
	for _, c := range candidates {
		if c.MarketEvidence.Status == evidenceModeCacheValidated {
			return evidenceModeCacheValidated
		}
	}
	for _, c := range candidates {
		if c.MarketEvidence.Status == evidenceModeTrendSupported {
			return evidenceModeTrendSupported
		}
	}
	return evidenceModeAIStrategicAnalysis
}

func appendSentence(base, next string) string {
	base = strings.TrimSpace(base)
	next = strings.TrimSpace(next)
	if base == "" {
		return next
	}
	if next == "" {
		return base
	}
	return base + " " + next
}

func appendStringGroups(base []string, values ...[]string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range base {
		item = strings.TrimSpace(item)
		if item != "" && !seen[strings.ToLower(item)] {
			seen[strings.ToLower(item)] = true
			out = append(out, item)
		}
	}
	for _, group := range values {
		for _, item := range group {
			item = strings.TrimSpace(item)
			if item != "" && !seen[strings.ToLower(item)] {
				seen[strings.ToLower(item)] = true
				out = append(out, item)
			}
		}
	}
	return out
}

func firstStringFromDrafts(drafts []NicheDraft) string {
	for _, draft := range drafts {
		if strings.TrimSpace(draft.Name) != "" {
			return strings.TrimSpace(draft.Name)
		}
	}
	return ""
}

func scorePersonalFit(bp nicheBlueprint, profile CreatorNicheProfile) float64 {
	score := 35.0
	haystack := strings.ToLower(strings.Join([]string{profile.ProfessionalSkills, profile.Hobbies, profile.LivedExperiences, profile.TeachingSubjects, profile.ThreeYearsAgoAdvice}, " "))
	for _, term := range strings.Fields(strings.ToLower(bp.CorePhrase)) {
		if len(term) > 2 && strings.Contains(haystack, term) {
			score += 8
		}
	}
	if profile.TargetAudience != "" {
		score += 12
	}
	if profile.CreatorPresence != "" {
		score += 8
	}
	if len(profile.ContentFormats) > 0 {
		score += 8
	}
	if profile.WeeklyProductionCapacity != "" {
		score += 6
	}
	return round1(clampScore(score))
}

func scoreDemandValidation(v NicheValidation) float64 {
	score := float64(v.SampledVideoCount)*1.1 + float64(v.RecentPublicationVolume)*2.2 + math.Log10(float64(v.MedianSampledViews)+10)*10
	if v.RisingTopicOverlap {
		score += 10
	}
	if v.EngagementRate >= 1.5 {
		score += 8
	}
	return round1(clampScore(score))
}

func scoreOpportunityGap(v NicheValidation, outliers []OutlierEvidence, gaps []SupplyGap) float64 {
	score := 35.0
	if v.MedianSampledViews >= 10000 {
		score += 15
	}
	if v.RecentPublicationVolume <= 8 {
		score += 12
	}
	if v.CompetitionLevel == "manageable" || v.CompetitionLevel == "medium" {
		score += 10
	}
	score += minFloat(18, float64(len(outliers))*6)
	score += minFloat(15, float64(len(gaps))*4)
	return round1(clampScore(score))
}

func scoreConfidence(v NicheValidation, m MonetizationEstimate, s SustainabilityEvidence) float64 {
	score := 35.0
	if v.SampledVideoCount >= 20 {
		score += 20
	}
	if v.MedianSampledViews > 0 {
		score += 10
	}
	if s.ViableTopicCount >= 30 {
		score += 15
	}
	if m.RPMEstimateAvailable {
		score += 15
	} else if m.Confidence == "medium" {
		score += 5
	}
	return round1(clampScore(score))
}

func normalizeCreatorNicheProfile(p CreatorNicheProfile) CreatorNicheProfile {
	p.ProfessionalSkills = strings.TrimSpace(p.ProfessionalSkills)
	p.Hobbies = strings.TrimSpace(p.Hobbies)
	p.LivedExperiences = strings.TrimSpace(p.LivedExperiences)
	p.TeachingSubjects = strings.TrimSpace(p.TeachingSubjects)
	p.ThreeYearsAgoAdvice = strings.TrimSpace(p.ThreeYearsAgoAdvice)
	p.TargetAudience = strings.TrimSpace(p.TargetAudience)
	p.TargetCountry = firstNonEmpty(strings.ToUpper(strings.TrimSpace(p.TargetCountry)), "GB")
	p.TargetLanguage = firstNonEmpty(strings.TrimSpace(p.TargetLanguage), "en")
	p.CreatorPresence = firstNonEmpty(strings.TrimSpace(p.CreatorPresence), "faceless")
	p.PrimaryMonetizationGoal = firstNonEmpty(strings.TrimSpace(p.PrimaryMonetizationGoal), "AdSense")
	p.OptionalBroadTopic = strings.TrimSpace(p.OptionalBroadTopic)
	p.WeeklyProductionCapacity = firstNonEmpty(strings.TrimSpace(p.WeeklyProductionCapacity), "2 videos per week")
	p.ContentFormats = cleanProfileStrings(p.ContentFormats)
	if len(p.ContentFormats) == 0 {
		p.ContentFormats = []string{"long-form"}
	}
	return p
}

func meaningfulProfileFields(p CreatorNicheProfile) int {
	values := []string{p.ProfessionalSkills, p.Hobbies, p.LivedExperiences, p.TeachingSubjects, p.ThreeYearsAgoAdvice, p.TargetAudience, p.OptionalBroadTopic}
	count := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

func stableNicheID(parts ...string) string {
	h := sha1.Sum([]byte(strings.ToLower(strings.Join(parts, "|"))))
	return "niche_" + hex.EncodeToString(h[:])[:14]
}

func nicheReportID(profile CreatorNicheProfile, now time.Time) string {
	return stableNicheID(profile.TargetCountry, profile.TargetLanguage, profile.TargetAudience, profile.OptionalBroadTopic, now.Format("20060102150405"))
}

func NicheResearchCacheKey(profile CreatorNicheProfile, mode string) string {
	profile = normalizeCreatorNicheProfile(profile)
	parts := []string{
		profile.ProfessionalSkills,
		profile.Hobbies,
		profile.LivedExperiences,
		profile.TeachingSubjects,
		profile.ThreeYearsAgoAdvice,
		profile.TargetAudience,
		profile.TargetCountry,
		profile.TargetLanguage,
		profile.CreatorPresence,
		strings.Join(profile.ContentFormats, ","),
		profile.PrimaryMonetizationGoal,
		profile.OptionalBroadTopic,
		profile.WeeklyProductionCapacity,
		mode,
	}
	return stableNicheID(parts...)
}

func sentenceCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func primaryFormat(formats []string) string {
	joined := strings.ToLower(strings.Join(formats, ","))
	switch {
	case strings.Contains(joined, "both"):
		return "both"
	case strings.Contains(joined, "short"):
		return "shorts"
	default:
		return "long-form"
	}
}

func normalizeMonetizationRoutes(goal string) []string {
	goal = strings.ToLower(goal)
	routes := []string{}
	add := func(label string) {
		for _, existing := range routes {
			if existing == label {
				return
			}
		}
		routes = append(routes, label)
	}
	if strings.Contains(goal, "adsense") || strings.Contains(goal, "ad") {
		add("AdSense")
	}
	if strings.Contains(goal, "affiliate") {
		add("Affiliate marketing")
	}
	if strings.Contains(goal, "sponsor") {
		add("Sponsorships")
	}
	if strings.Contains(goal, "product") || strings.Contains(goal, "digital") {
		add("Digital products")
	}
	if strings.Contains(goal, "service") {
		add("Services")
	}
	if strings.Contains(goal, "lead") {
		add("Leads")
	}
	if len(routes) == 0 {
		add("AdSense")
	}
	return routes
}

func uniqueBlueprints(in []nicheBlueprint) []nicheBlueprint {
	seen := map[string]bool{}
	out := []nicheBlueprint{}
	for _, bp := range in {
		key := normalizeTopicKey(bp.Level3)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, bp)
	}
	if len(out) > 5 {
		return out[:5]
	}
	return out
}

func medianUint64(values []uint64) uint64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]uint64{}, values...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	return cp[len(cp)/2]
}

func medianVideoViews(videos []ChannelVideoSummary) uint64 {
	values := []uint64{}
	for _, video := range videos {
		if video.Views != nil {
			values = append(values, *video.Views)
		}
	}
	return medianUint64(values)
}

func phraseOverlaps(phrase string, values []string) bool {
	phrase = strings.ToLower(phrase)
	for _, value := range values {
		value = strings.ToLower(value)
		for _, term := range strings.Fields(phrase) {
			if len(term) > 3 && strings.Contains(value, term) {
				return true
			}
		}
	}
	return false
}

func competitionLabel(videoCount int, medianViews uint64) string {
	switch {
	case videoCount >= 28 && medianViews >= 100000:
		return "high"
	case videoCount >= 18 && medianViews >= 25000:
		return "medium"
	default:
		return "manageable"
	}
}

func marketEvidenceSummary(count, recent int, median uint64, overlap bool) string {
	parts := []string{
		"Sampled public videos: " + intString(count),
		"recent uploads: " + intString(recent),
		"median sampled views: " + uintString(median),
	}
	if overlap {
		parts = append(parts, "overlaps current rising topics")
	}
	return strings.Join(parts, "; ") + "."
}

func topOutliers(items []OutlierEvidence, n int) []OutlierEvidence {
	if len(items) < n {
		n = len(items)
	}
	return append([]OutlierEvidence{}, items[:n]...)
}

func titlesContain(videos []ChannelVideoSummary, terms []string) bool {
	for _, video := range videos {
		haystack := strings.ToLower(video.Title + " " + video.Description)
		for _, term := range terms {
			if strings.Contains(haystack, term) {
				return true
			}
		}
	}
	return false
}

func questionSignalCount(videos []ChannelVideoSummary) int {
	count := 0
	for _, video := range videos {
		title := strings.ToLower(video.Title)
		if strings.Contains(title, "?") || strings.HasPrefix(title, "how ") || strings.HasPrefix(title, "why ") || strings.Contains(title, "what ") {
			count++
		}
	}
	return count
}

func topSupplyGaps(gaps []SupplyGap, n int) []SupplyGap {
	if len(gaps) < n {
		n = len(gaps)
	}
	return append([]SupplyGap{}, gaps[:n]...)
}

func commercialPotential(score float64) string {
	switch {
	case score >= 72:
		return "High"
	case score >= 50:
		return "Medium"
	default:
		return "Low"
	}
}

func monetizationConfidence(score float64, status ProviderStatus) string {
	if status.Status == StatusActive && score >= 60 {
		return "medium"
	}
	return "low"
}

func calibrationMatches(c NicheRPMCalibration, country, format string) bool {
	return strings.EqualFold(c.Country, country) && strings.EqualFold(c.Format, format) && c.Low > 0 && c.Midpoint >= c.Low && c.High >= c.Midpoint
}

func earningsProjections(c NicheRPMCalibration) []EarningsProjection {
	views := []int{10000, 100000, 1000000}
	out := []EarningsProjection{}
	for _, v := range views {
		out = append(out, EarningsProjection{
			Views:    v,
			Low:      round2(float64(v) / 1000 * c.Low),
			Midpoint: round2(float64(v) / 1000 * c.Midpoint),
			High:     round2(float64(v) / 1000 * c.High),
			Formula:  "views / 1000 x estimated RPM",
		})
	}
	return out
}

func topicTitle(intent, core, audience, pillar string) string {
	switch intent {
	case "tutorial":
		return "How to use " + core + " for " + audience + " in " + pillar
	case "comparison":
		return "Best options for " + core + " in " + pillar + ": beginner comparison"
	case "case study":
		return "I tested " + core + " for " + pillar + " in a real " + strings.ToLower(audience) + " scenario"
	case "mistake":
		return "Avoid these " + pillar + " mistakes with " + core
	case "checklist":
		return "The " + pillar + " checklist for " + core
	case "explainer":
		return "What " + core + " means for " + pillar + " in simple terms"
	case "Q&A":
		return "Answering the biggest " + pillar + " questions about " + core
	case "update":
		return "What changed recently in " + core + " for " + pillar
	case "workflow":
		return "A repeatable " + pillar + " workflow for " + core
	default:
		return "A practical " + pillar + " strategy for " + core
	}
}

func topicDifficulty(intent, capacity string) string {
	if strings.Contains(strings.ToLower(capacity), "1") || strings.Contains(strings.ToLower(capacity), "low") {
		if intent == "case study" || intent == "comparison" {
			return "medium"
		}
		return "low"
	}
	if intent == "case study" {
		return "high"
	}
	return "medium"
}

func dedupeVideoTopics(raw []VideoTopic, limit int) ([]VideoTopic, int) {
	seen := map[string]bool{}
	out := []VideoTopic{}
	removed := 0
	for _, topic := range raw {
		key := normalizeTopicKey(topic.Title)
		if key == "" || seen[key] {
			removed++
			continue
		}
		seen[key] = true
		out = append(out, topic)
		if len(out) >= limit {
			break
		}
	}
	return out, removed
}

func normalizeTopicKey(value string) string {
	value = strings.ToLower(value)
	replacer := strings.NewReplacer("how to ", "", "best ", "", "beginner ", "", "the ", "", "a ", "", "?", "", ":", "", "-", " ")
	value = replacer.Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func estimateRunway(topicCount int, capacity string) string {
	perWeek := 2.0
	fields := strings.Fields(capacity)
	for _, field := range fields {
		if len(field) == 1 && field[0] >= '1' && field[0] <= '9' {
			perWeek = float64(field[0] - '0')
			break
		}
	}
	weeks := int(math.Ceil(float64(topicCount) / perWeek))
	if weeks <= 0 {
		weeks = 1
	}
	return intString(weeks) + " weeks at stated capacity"
}

func firstTopicTitles(topics []VideoTopic, limit int) []string {
	out := []string{}
	for _, topic := range topics {
		out = append(out, topic.Title)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func nicheRisks(v NicheValidation, s SustainabilityEvidence, m MonetizationEstimate) []string {
	risks := []string{}
	if v.CompetitionLevel == "high" {
		risks = append(risks, "Top results look competitive; packaging and credibility must be sharper than broad tutorials.")
	}
	if s.Warning != "" {
		risks = append(risks, s.Warning)
	}
	if !m.RPMEstimateAvailable {
		risks = append(risks, "No valid RPM calibration is available, so currency projections are hidden.")
	}
	if len(risks) == 0 {
		risks = append(risks, "Public evidence can miss retention, private analytics, and audience loyalty.")
	}
	return risks
}

func recommendedFirstAction(bp nicheBlueprint, v NicheValidation, s SustainabilityEvidence) string {
	if s.Warning != "" {
		return "Write 20 more distinct titles before committing to this niche."
	}
	if v.RecentPublicationVolume < 6 {
		return "Publish one beginner-focused validation video and track click-through and comments."
	}
	return "Build a four-video starter series around the strongest content pillar."
}

func scoreLabel(score float64) string {
	switch {
	case score >= 75:
		return "strong"
	case score >= 55:
		return "promising"
	case score >= 40:
		return "mixed"
	default:
		return "weak"
	}
}

func confidenceLabel(score float64) string {
	switch {
	case score >= 75:
		return "high"
	case score >= 55:
		return "medium"
	default:
		return "low"
	}
}

func intString(v int) string {
	return strconvFormatInt(int64(v))
}

func uintString(v uint64) string {
	return strconvFormatInt(int64(v))
}

func strconvFormatInt(v int64) string {
	if v == 0 {
		return "0"
	}
	negative := v < 0
	if negative {
		v = -v
	}
	digits := []byte{}
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func humanAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	hours := int(d.Hours())
	switch {
	case hours < 1:
		return "less than an hour"
	case hours < 48:
		return intString(hours) + " hours"
	case hours < 24*60:
		return intString(hours/24) + " days"
	case hours < 24*365:
		return intString(hours/(24*30)) + " months"
	default:
		return intString(hours/(24*365)) + " years"
	}
}

func containsExactString(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), needle) {
			return true
		}
	}
	return false
}

func cleanProfileStrings(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (p *YouTubeProvider) ChannelStats(ctx context.Context, channelIDs []string) (map[string]NicheChannelStats, error) {
	if p.apiKey == "" {
		return nil, ErrNotConfigured
	}
	channelIDs = unique(channelIDs)
	out := map[string]NicheChannelStats{}
	for len(channelIDs) > 0 {
		batch := channelIDs
		if len(batch) > 50 {
			batch = channelIDs[:50]
		}
		channelIDs = channelIDs[len(batch):]
		apiURL := youtubeAPIURL("channels", map[string]string{
			"part": "snippet,statistics",
			"id":   strings.Join(batch, ","),
			"key":  p.apiKey,
		})
		var res youtubeChannelsResponse
		if err := p.getJSON(ctx, apiURL, &res); err != nil {
			return out, err
		}
		for _, item := range res.Items {
			stats := NicheChannelStats{
				ChannelID:   item.ID,
				Title:       item.Snippet.Title,
				Subscribers: parseUintPtr(item.Statistics.SubscriberCount),
				Views:       parseUintPtr(item.Statistics.ViewCount),
				VideoCount:  parseUintPtr(item.Statistics.VideoCount),
			}
			stats.HiddenSubs = stats.Subscribers == nil
			out[item.ID] = stats
		}
	}
	return out, nil
}

var errNicheReportNotFound = errors.New("niche report not found")
