package research

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fakeNicheProvider struct {
	searchCalls int
	videos      []ChannelVideoSummary
	channels    map[string]NicheChannelStats
	searchErr   error
	channelErr  error
}

func (f *fakeNicheProvider) Status() ProviderStatus {
	return ProviderStatus{ID: "youtube_data_api", Status: StatusActive}
}

func (f *fakeNicheProvider) SearchVideos(ctx context.Context, query, region, language string, limit int) ([]ChannelVideoSummary, error) {
	_ = ctx
	_ = query
	_ = region
	_ = language
	_ = limit
	f.searchCalls++
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.videos, nil
}

func (f *fakeNicheProvider) ChannelStats(ctx context.Context, channelIDs []string) (map[string]NicheChannelStats, error) {
	_ = ctx
	_ = channelIDs
	if f.channelErr != nil {
		return nil, f.channelErr
	}
	return f.channels, nil
}

type fakeNicheTrends struct {
	values []string
}

func (f fakeNicheTrends) Discover(ctx context.Context, region, language string, limit int) ([]string, error) {
	_ = ctx
	_ = region
	_ = language
	_ = limit
	return f.values, nil
}

type fakeNicheStrategist struct {
	result NicheStrategyResult
	err    error
}

func (f fakeNicheStrategist) GenerateCandidates(ctx context.Context, input NicheStrategyInput) (NicheStrategyResult, error) {
	_ = ctx
	_ = input
	return f.result, f.err
}

type fakeRepairStrategist struct {
	result          NicheStrategyResult
	repaired        NicheStrategyResult
	repairCalls     int
	repairedIdeas   []VideoTopic
	ideaRepairCalls int
	missingCount    int
	wantMissing     int
	generateErr     error
	repairErr       error
	repairIssues    []string
}

func (f *fakeRepairStrategist) GenerateCandidates(ctx context.Context, input NicheStrategyInput) (NicheStrategyResult, error) {
	_ = ctx
	_ = input
	return f.result, f.generateErr
}

func (f *fakeRepairStrategist) RepairCandidates(ctx context.Context, input NicheStrategyInput, previous NicheStrategyResult, issues []string) (NicheStrategyResult, error) {
	_ = ctx
	_ = input
	_ = previous
	f.repairCalls++
	f.repairIssues = append([]string{}, issues...)
	return f.repaired, f.repairErr
}

func (f *fakeRepairStrategist) RepairPrimaryIdeas(ctx context.Context, input NicheStrategyInput, draft NicheDraft, missingCount int, acceptedTitles []string, rejectedTitles []string) ([]VideoTopic, error) {
	_ = ctx
	_ = input
	_ = draft
	_ = acceptedTitles
	_ = rejectedTitles
	f.ideaRepairCalls++
	f.missingCount = missingCount
	if f.wantMissing > 0 && missingCount != f.wantMissing {
		return nil, errors.New("unexpected missing count")
	}
	return f.repairedIdeas, f.repairErr
}

func TestNicheResearchValidatesCandidatesWithMeasuredEvidence(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	provider := &fakeNicheProvider{videos: sampleNicheVideos(now), channels: sampleNicheChannels()}
	report, err := ResearchNiches(context.Background(), NicheResearchRequest{Profile: sampleAIProfile()}, NicheResearchConfig{
		YouTube:         provider,
		Trends:          fakeNicheTrends{values: []string{"AI automation for small business", "workflow automation"}},
		GoogleAdsStatus: ProviderStatus{Status: StatusNotConfigured},
		Now:             func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("ResearchNiches error: %v", err)
	}
	if report.Status != StatusOK {
		t.Fatalf("status = %s, message = %s", report.Status, report.Message)
	}
	if len(report.Candidates) < 3 || len(report.Candidates) > 5 {
		t.Fatalf("candidate count = %d, want 3-5", len(report.Candidates))
	}
	seen := map[string]bool{}
	for i, candidate := range report.Candidates {
		if candidate.Level1 == "" || candidate.Level2 == "" || candidate.Level3 == "" {
			t.Fatalf("candidate missing three-level path: %#v", candidate)
		}
		if seen[strings.ToLower(candidate.Level3)] {
			t.Fatalf("duplicate niche candidate: %s", candidate.Level3)
		}
		seen[strings.ToLower(candidate.Level3)] = true
		if i < 3 && (candidate.Validation.SampledVideoCount == 0 || candidate.Validation.MedianSampledViews == 0) {
			t.Fatalf("candidate missing measured validation: %#v", candidate.Validation)
		}
		if candidate.Monetization.RPMEstimateAvailable {
			t.Fatalf("unexpected RPM estimate without calibration: %#v", candidate.Monetization)
		}
		if candidate.Monetization.UnavailableReason == "" {
			t.Fatalf("missing monetization unavailable reason")
		}
	}
	if provider.searchCalls == 0 || provider.searchCalls > 3 {
		t.Fatalf("search calls = %d, want capped budget", provider.searchCalls)
	}
}

func TestNicheResearchNeverAttemptsFourthLiveYouTubeSearch(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	provider := &fakeNicheProvider{videos: sampleNicheVideos(now), channels: sampleNicheChannels()}
	report, err := ResearchNiches(context.Background(), NicheResearchRequest{Profile: sampleAIProfile()}, NicheResearchConfig{
		YouTube:                      provider,
		Strategist:                   fakeNicheStrategist{result: strategyWithCandidates(5)},
		MaxYouTubeSearchesPerRequest: 3,
		DailyYouTubeSearchLimit:      80,
		Now:                          func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("ResearchNiches error: %v", err)
	}
	if report.Status != StatusOK {
		t.Fatalf("status = %s", report.Status)
	}
	if provider.searchCalls != 3 {
		t.Fatalf("search calls = %d, want exactly 3", provider.searchCalls)
	}
	for i, candidate := range report.Candidates {
		if i < 3 && candidate.MarketEvidence.Status != evidenceModeLiveValidated {
			t.Fatalf("candidate %d evidence status = %s, want live", i, candidate.MarketEvidence.Status)
		}
		if i >= 3 && candidate.MarketEvidence.Status == evidenceModeLiveValidated {
			t.Fatalf("candidate %d should not be live-validated after budget exhaustion", i)
		}
	}
}

func TestNicheResearchDuplicateCachedQueryDoesNotConsumeLiveSearch(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	provider := &fakeNicheProvider{videos: sampleNicheVideos(now), channels: sampleNicheChannels()}
	report, err := ResearchNiches(context.Background(), NicheResearchRequest{Profile: sampleAIProfile()}, NicheResearchConfig{
		YouTube:                      provider,
		Strategist:                   fakeNicheStrategist{result: strategyWithDuplicateQueries()},
		YouTubeCache:                 NewYouTubeEvidenceCache(24 * time.Hour),
		MaxYouTubeSearchesPerRequest: 3,
		DailyYouTubeSearchLimit:      80,
		Now:                          func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("ResearchNiches error: %v", err)
	}
	if provider.searchCalls != 1 {
		t.Fatalf("duplicate normalized query consumed %d searches, want 1", provider.searchCalls)
	}
	if len(report.Candidates) < 2 || report.Candidates[1].MarketEvidence.Status != "public_evidence_validated" {
		t.Fatalf("second duplicate candidate should use cached validation: %+v", report.Candidates)
	}
}

func TestNicheResearchSharedDailyLimitIsProcessLocalAndCapsRequests(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	provider := &fakeNicheProvider{videos: sampleNicheVideos(now), channels: sampleNicheChannels()}
	limiter := NewDailyYouTubeSearchLimiter(3, func() time.Time { return now })
	cfg := NicheResearchConfig{
		YouTube:                      provider,
		Strategist:                   fakeNicheStrategist{result: strategyWithCandidates(5)},
		YouTubeDailyLimiter:          limiter,
		MaxYouTubeSearchesPerRequest: 3,
		DailyYouTubeSearchLimit:      3,
		Now:                          func() time.Time { return now },
	}
	if _, err := ResearchNiches(context.Background(), NicheResearchRequest{Profile: sampleAIProfile()}, cfg); err != nil {
		t.Fatalf("first ResearchNiches error: %v", err)
	}
	if _, err := ResearchNiches(context.Background(), NicheResearchRequest{Profile: sampleAIProfile()}, cfg); err != nil {
		t.Fatalf("second ResearchNiches error: %v", err)
	}
	if provider.searchCalls != 3 {
		t.Fatalf("shared daily limit allowed %d searches, want 3", provider.searchCalls)
	}
	day, used, limit := limiter.Snapshot()
	if day != "2026-07-12" || used != 3 || limit != 3 {
		t.Fatalf("limiter snapshot = day %q used %d limit %d", day, used, limit)
	}
}

func TestNicheResearchOpenAIUnavailableReturnsStructuredApplicationState(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	report, err := ResearchNiches(context.Background(), NicheResearchRequest{Profile: sampleAIProfile()}, NicheResearchConfig{
		RequireOpenAI: true,
		Now:           func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("OpenAI unavailable should return application state, not generic error: %v", err)
	}
	if report.Status != "openai_unavailable" || report.AnalysisMode != "AI strategy unavailable" {
		t.Fatalf("status=%s analysis_mode=%s, want sanitized openai_unavailable state", report.Status, report.AnalysisMode)
	}
	if !strings.Contains(strings.ToLower(report.Message), "ai niche strategy") {
		t.Fatalf("message should state AI niche strategy is unavailable: %q", report.Message)
	}
	if len(report.Candidates) != 0 {
		t.Fatalf("OpenAI unavailable must not claim completed candidates: %d", len(report.Candidates))
	}
	if report.Profile.ProfessionalSkills == "" {
		t.Fatalf("profile inputs should remain in response for retry")
	}
}

func TestNicheResearchContinuesWhenYouTubeValidationFails(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{
			name: "quota",
			err:  &ProviderError{Code: ProviderErrorQuota, HTTPStatus: http.StatusForbidden, Reason: "quotaExceeded"},
		},
		{
			name: "invalid credentials",
			err:  &ProviderError{Code: ProviderErrorCredentials, HTTPStatus: http.StatusForbidden, Reason: "keyInvalid"},
		},
		{
			name: "timeout",
			err:  context.DeadlineExceeded,
		},
		{
			name: "upstream 5xx",
			err:  &ProviderError{Code: ProviderErrorTemporary, HTTPStatus: http.StatusBadGateway, Reason: "backendError"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, err := ResearchNiches(context.Background(), NicheResearchRequest{Profile: sampleAIProfile()}, NicheResearchConfig{
				YouTube: &fakeNicheProvider{searchErr: tc.err},
				Now:     func() time.Time { return time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC) },
			})
			if err != nil {
				t.Fatalf("validation failure should not fail AI-first research: %v", err)
			}
			if report.Status != StatusOK || len(report.Candidates) == 0 {
				t.Fatalf("expected partial AI result, got status=%s candidates=%d", report.Status, len(report.Candidates))
			}
		})
	}
}

func TestNicheResearchSucceedsWithoutYouTubeEvidence(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	report, err := ResearchNiches(context.Background(), NicheResearchRequest{Profile: sampleAIProfile()}, NicheResearchConfig{
		YouTube: &fakeNicheProvider{videos: []ChannelVideoSummary{}, channels: map[string]NicheChannelStats{}},
		Now:     func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("missing evidence should not be provider failure: %v", err)
	}
	if report.Status != StatusOK || len(report.Candidates) == 0 {
		t.Fatalf("expected AI-only result, got status=%s candidates=%d", report.Status, len(report.Candidates))
	}
	if report.Candidates[0].MarketEvidence.MedianViews != nil || report.Candidates[0].MarketEvidence.Engagement != nil {
		t.Fatalf("AI-only result should not fabricate evidence fields: %#v", report.Candidates[0].MarketEvidence)
	}
}

func TestProviderHTTPErrorClassificationUsesMockedHTTP(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		wantCode   string
		wantReason string
	}{
		{"quota", http.StatusForbidden, `{"error":{"errors":[{"reason":"quotaExceeded","message":"quota"}],"message":"quota"}}`, ProviderErrorQuota, "quotaExceeded"},
		{"daily limit", http.StatusForbidden, `{"error":{"errors":[{"reason":"dailyLimitExceeded"}]}}`, ProviderErrorQuota, "dailyLimitExceeded"},
		{"rate limit", http.StatusTooManyRequests, `{"error":{"errors":[{"reason":"rateLimitExceeded"}]}}`, ProviderErrorQuota, "rateLimitExceeded"},
		{"invalid key", http.StatusForbidden, `{"error":{"errors":[{"reason":"keyInvalid"}]}}`, ProviderErrorCredentials, "keyInvalid"},
		{"restriction", http.StatusForbidden, `{"error":{"errors":[{"reason":"ipRefererBlocked"}]}}`, ProviderErrorCredentials, "ipRefererBlocked"},
		{"temporary", http.StatusServiceUnavailable, `{"error":{"errors":[{"reason":"backendError"}]}}`, ProviderErrorTemporary, "backendError"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: tc.status,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(tc.body)),
					Request:    req,
				}, nil
			})}
			provider := NewYouTubeProvider("test-key", client)
			var decoded youtubeVideosResponse
			err := provider.getJSON(context.Background(), "https://example.invalid/youtube/v3/search", &decoded)
			var providerErr *ProviderError
			if !errors.As(err, &providerErr) {
				t.Fatalf("error = %T %v, want ProviderError", err, err)
			}
			if providerErr.Code != tc.wantCode || providerErr.Reason != tc.wantReason || providerErr.HTTPStatus != tc.status {
				t.Fatalf("provider err = %+v, want code %s reason %s status %d", providerErr, tc.wantCode, tc.wantReason, tc.status)
			}
		})
	}
}

func TestVideoTopicDeduplicationAndSustainabilityWarning(t *testing.T) {
	bp := nicheBlueprint{CorePhrase: "AI automation", TargetViewer: "UK small businesses", Level3: "AI workflow automation for UK small businesses"}
	topics, pillars, sustainability := buildVideoSustainability(bp, sampleAIProfile(), nil)
	if len(topics) != 50 {
		t.Fatalf("topics = %d, want 50", len(topics))
	}
	if len(pillars) < 4 {
		t.Fatalf("pillars = %d, want at least 4", len(pillars))
	}
	if sustainability.TopicRepetitionRisk == "high" {
		t.Fatalf("unexpected high repetition risk for full topic set")
	}

	short, _, shortSustainability := buildVideoSustainability(nicheBlueprint{CorePhrase: "x", TargetViewer: "y", Level3: "z"}, CreatorNicheProfile{WeeklyProductionCapacity: "2 videos per week"}, []ChannelVideoSummary{})
	short = short[:20]
	_, removed := dedupeVideoTopics(short, 50)
	if removed != 0 {
		t.Fatalf("removed = %d, want no duplicate removals in truncated set", removed)
	}
	shortSustainability.ViableTopicCount = 20
	if shortSustainability.ViableTopicCount >= 30 {
		t.Fatalf("test setup expected fewer than 30 topics")
	}
}

func TestDemandOutlierSupplyGapAndMissingMetrics(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	videos := sampleNicheVideos(now)
	zero := uint64(0)
	videos = append(videos, ChannelVideoSummary{VideoID: "zero", Title: "New channel AI automation question", ChannelID: "zero", ChannelTitle: "Zero Subs", PublishedAt: now.Add(-24 * time.Hour).Format(time.RFC3339), Views: &zero})
	channels := sampleNicheChannels()
	channels["hidden"] = NicheChannelStats{ChannelID: "hidden", Title: "Hidden Subs", HiddenSubs: true}
	channels["zero"] = NicheChannelStats{ChannelID: "zero", Title: "Zero Subs", Subscribers: &zero}
	outliers := detectOutliers(videos, channels, now)
	if len(outliers) == 0 {
		t.Fatalf("expected outlier evidence")
	}
	for _, outlier := range outliers {
		if outlier.PublicViews == 0 {
			t.Fatalf("zero-view video should not be an outlier")
		}
	}
	validation := buildNicheValidation(nicheBlueprint{CorePhrase: "AI automation", QueryPhrases: []string{"AI automation"}}, videos, []string{"AI automation"}, now)
	if validation.RecentPublicationVolume == 0 || validation.TotalSampledViews == 0 || !validation.RisingTopicOverlap {
		t.Fatalf("validation missing demand evidence: %#v", validation)
	}
	gaps := buildSupplyGaps(nicheBlueprint{CorePhrase: "AI automation", TargetViewer: "UK small businesses"}, validation, videos, outliers, sampleAIProfile())
	if len(gaps) == 0 {
		t.Fatalf("expected supported supply gaps")
	}
}

func TestMonetizationUnavailableAndCalibratedRPMSeparation(t *testing.T) {
	profile := sampleAIProfile()
	bp := nicheBlueprint{Level1: "Technology", Level2: "AI automation", Level3: "AI workflow automation for UK small businesses", Routes: []string{"AdSense", "Affiliate marketing"}}
	unavailable := buildMonetizationEstimate(bp, profile, ProviderStatus{Status: StatusNotConfigured}, nil, NicheValidation{})
	if unavailable.RPMEstimateAvailable || unavailable.RPMLow != nil || unavailable.UnavailableReason == "" {
		t.Fatalf("expected unavailable RPM state: %#v", unavailable)
	}
	calibration := &NicheRPMCalibration{
		Country:      "GB",
		Format:       "long-form",
		Currency:     "GBP",
		Low:          2,
		Midpoint:     4,
		High:         6,
		Confidence:   "medium",
		Type:         "documented_internal_benchmark",
		CalibratedAt: time.Now().Add(-30 * 24 * time.Hour),
		Assumptions:  []string{"Documented benchmark dataset; public research estimate only."},
	}
	available := buildMonetizationEstimate(bp, profile, ProviderStatus{Status: StatusActive}, calibration, NicheValidation{})
	if !available.RPMEstimateAvailable || len(available.EstimatedEarnings) != 3 {
		t.Fatalf("expected calibrated RPM projections: %#v", available)
	}
	if available.EstimatedEarnings[0].Midpoint != 40 {
		t.Fatalf("10K midpoint = %.2f, want 40", available.EstimatedEarnings[0].Midpoint)
	}
	profile.TargetCountry = "US"
	separated := buildMonetizationEstimate(bp, profile, ProviderStatus{Status: StatusActive}, calibration, NicheValidation{})
	if separated.RPMEstimateAvailable {
		t.Fatalf("country separation failed: %#v", separated)
	}
	profile.TargetCountry = "GB"
	profile.ContentFormats = []string{"shorts"}
	separated = buildMonetizationEstimate(bp, profile, ProviderStatus{Status: StatusActive}, calibration, NicheValidation{})
	if separated.RPMEstimateAvailable {
		t.Fatalf("format separation failed: %#v", separated)
	}
}

func TestDeterministicScoringConfidenceAndPublicResponseSafety(t *testing.T) {
	profile := sampleAIProfile()
	validation := NicheValidation{SampledVideoCount: 25, RecentPublicationVolume: 8, MedianSampledViews: 50000, EngagementRate: 2.1, MarketEvidenceSummary: "Measured public evidence."}
	sustainability := SustainabilityEvidence{ViableTopicCount: 50, ContentPillarCount: 5, Score: 92}
	monetization := MonetizationEstimate{Score: 74, CommercialPotential: "High", Confidence: "low", RPMEstimateAvailable: false}
	bp := nicheBlueprint{CorePhrase: "AI automation", Level3: "AI workflow automation for UK small businesses"}
	a := scoreNiche(bp, profile, validation, []OutlierEvidence{{Strength: 55}}, []SupplyGap{{Statement: "gap"}}, monetization, sustainability)
	b := scoreNiche(bp, profile, validation, []OutlierEvidence{{Strength: 55}}, []SupplyGap{{Statement: "gap"}}, monetization, sustainability)
	if a.Overall.Score != b.Overall.Score {
		t.Fatalf("scoring not deterministic: %.1f != %.1f", a.Overall.Score, b.Overall.Score)
	}
	if !strings.Contains(a.Overall.Explanation, "0.25 creator fit") {
		t.Fatalf("missing transparent formula: %s", a.Overall.Explanation)
	}
	highMonetization := scoreNiche(bp, profile, validation, []OutlierEvidence{{Strength: 55}}, []SupplyGap{{Statement: "gap"}}, MonetizationEstimate{Score: 100, CommercialPotential: "High"}, sustainability)
	lowMonetization := scoreNiche(bp, profile, validation, []OutlierEvidence{{Strength: 55}}, []SupplyGap{{Statement: "gap"}}, MonetizationEstimate{Score: 0, CommercialPotential: "Low"}, sustainability)
	if highMonetization.Overall.Score != lowMonetization.Overall.Score {
		t.Fatalf("monetization changed score: %.1f != %.1f", highMonetization.Overall.Score, lowMonetization.Overall.Score)
	}
	report := NicheReport{Internal: NicheInternal{Provenance: []string{"youtube_data_api"}, CacheKey: "secretish"}}
	if report.Internal.Provenance[0] == "" {
		t.Fatalf("test setup failed")
	}
}

func TestAIScoreNormalizationOverridesModelTotalsAndBoundsDimensions(t *testing.T) {
	draft := NicheDraft{
		ID:                 "draft-1",
		Name:               "Cross-platform app tutorials",
		Subcategory:        "cross-platform apps",
		TargetAudience:     "vibe coders and students",
		DimensionScores:    NicheDraftDimensionScores{CreatorFit: 120, AudienceDemand: -10, CompetitionOpportunity: 60, Sustainability: 80, Differentiation: 40},
		DimensionReasoning: NicheDraftDimensionReasoning{CreatorFit: "Strong fit"},
		ContentPillars:     []ContentPillar{{Name: "Tutorials", Percentage: 50, TopicCount: 5}, {Name: "Comparisons", Percentage: 50, TopicCount: 5}},
		RecommendedTitles:  []VideoTopic{{Title: "Build a simple app workflow from scratch", Pillar: "Tutorials", Intent: "tutorial"}},
		SearchQuery:        "cross-platform app tutorials",
	}
	candidate := buildAICandidate(draft, sampleTitleQualityProfile(), nil, time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC))
	for label, score := range map[string]float64{
		"creator_fit":             candidate.Dimensions.CreatorFit.Score,
		"audience_demand":         candidate.Dimensions.AudienceDemand.Score,
		"competition_opportunity": candidate.Dimensions.CompetitionOpportunity.Score,
		"sustainability":          candidate.Dimensions.Sustainability.Score,
		"differentiation":         candidate.Dimensions.Differentiation.Score,
		"overall":                 candidate.OverallScore,
	} {
		if score < 0 || score > 100 {
			t.Fatalf("%s score %.1f outside 0-100", label, score)
		}
	}
	want := round1(0.25*candidate.Dimensions.CreatorFit.Score + 0.25*candidate.Dimensions.AudienceDemand.Score + 0.20*candidate.Dimensions.CompetitionOpportunity.Score + 0.20*candidate.Dimensions.Sustainability.Score + 0.10*candidate.Dimensions.Differentiation.Score)
	if candidate.OverallScore != want || candidate.Scores.Overall.Score != want {
		t.Fatalf("overall = %.1f / %.1f, want weighted %.1f", candidate.OverallScore, candidate.Scores.Overall.Score, want)
	}
}

func TestAIScoreNormalizationRepairsMixedTenPointScaleWithRatings(t *testing.T) {
	draft := baseNicheDraft("draft-mixed", "Cross-platform app tutorials", "cross-platform apps")
	draft.DimensionScores = NicheDraftDimensionScores{CreatorFit: 9, AudienceDemand: 80, CompetitionOpportunity: 100, Sustainability: 8, Differentiation: 7}
	draft.DimensionRatings = NicheDraftDimensionRatings{CreatorFit: "high", AudienceDemand: "high", CompetitionOpportunity: "very_high", Sustainability: "high", Differentiation: "moderate"}
	drafts, issues := validateNicheDrafts([]NicheDraft{draft}, sampleTitleQualityProfile())
	if len(drafts) != 1 {
		t.Fatalf("draft rejected, issues=%v", issues)
	}
	candidate := buildAICandidate(drafts[0], sampleTitleQualityProfile(), nil, time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC))
	if candidate.Dimensions.CreatorFit.Score != 90 || candidate.Dimensions.Sustainability.Score != 80 || candidate.Dimensions.Differentiation.Score != 70 {
		t.Fatalf("mixed scale not normalized safely: %#v", candidate.Dimensions)
	}
	if candidate.Dimensions.CreatorFit.RatingBand != "very_high" || candidate.Dimensions.AudienceDemand.RatingBand != "very_high" || candidate.Dimensions.Differentiation.RatingBand != "high" {
		t.Fatalf("rating bands not normalized safely: %#v", candidate.Dimensions)
	}
	if candidate.OverallScore != 86 {
		t.Fatalf("overall = %.1f, want rounded weighted score 86", candidate.OverallScore)
	}
}

func TestFinalResponseValidationRepairsLegacyMixedScaleAndOverall(t *testing.T) {
	draft := baseNicheDraft("draft-legacy", "Legacy cached app tutorials", "cross-platform apps")
	drafts, _ := validateNicheDrafts([]NicheDraft{draft}, sampleTitleQualityProfile())
	candidate := buildAICandidate(drafts[0], sampleTitleQualityProfile(), nil, time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC))
	candidate.Dimensions.CreatorFit = ScoreExplanation{Score: 9, RatingBand: "high", Explanation: "Strong fit due to teaching background."}
	candidate.Dimensions.AudienceDemand = ScoreExplanation{Score: 80, RatingBand: "high", Explanation: "Strong audience demand."}
	candidate.Dimensions.CompetitionOpportunity = ScoreExplanation{Score: 100, RatingBand: "very_high", Explanation: "Strong opportunity."}
	candidate.Dimensions.Sustainability = ScoreExplanation{Score: 8, RatingBand: "high", Explanation: "Sustainable niche."}
	candidate.Dimensions.Differentiation = ScoreExplanation{Score: 7, RatingBand: "moderate", Explanation: "Distinct creator angle."}
	candidate.OverallScore = 44
	candidate.Scores.Overall = ScoreExplanation{Score: 44, RatingBand: "moderate", Explanation: "model overall"}
	report := NicheReport{Status: StatusOK, Profile: sampleTitleQualityProfile(), Candidates: []NicheCandidate{candidate}}

	finalized, issues, ok := FinalizeNicheReportForResponse(report)
	if !ok {
		t.Fatalf("finalizer rejected repairable legacy report: %v", issues)
	}
	primary := finalized.PrimaryRecommendation
	if primary == nil {
		t.Fatalf("missing primary")
	}
	got := []float64{primary.Dimensions.CreatorFit.Score, primary.Dimensions.AudienceDemand.Score, primary.Dimensions.CompetitionOpportunity.Score, primary.Dimensions.Sustainability.Score, primary.Dimensions.Differentiation.Score}
	want := []float64{90, 80, 100, 80, 70}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("scores = %v, want %v", got, want)
		}
	}
	if primary.OverallScore != calculateNicheOverallScore(primary.Dimensions) || primary.OverallScore == 44 {
		t.Fatalf("overall not recalculated: %.1f", primary.OverallScore)
	}
}

func TestFinalResponseValidationKeepsGenuineVeryLowEight(t *testing.T) {
	score, ok := finalizeScoreExplanation("creator_fit", "Genuine low", ScoreExplanation{Score: 8, RatingBand: "very_low", Explanation: "Very low fit for this creator."}, &[]string{})
	if !ok {
		t.Fatalf("genuine very-low score rejected")
	}
	if score.Score != 8 || score.RatingBand != "very_low" {
		t.Fatalf("score = %#v, want 8 very_low", score)
	}
}

func TestAIScoreValidationNormalizesHighRatingWithSingleDigit(t *testing.T) {
	draft := baseNicheDraft("draft-bad", "Legacy score candidate", "cross-platform apps")
	draft.DimensionScores.CreatorFit = 9
	draft.DimensionRatings.CreatorFit = "high"
	draft.DimensionReasoning.CreatorFit = "Strong alignment with the creator profile."
	drafts, issues := validateNicheDrafts([]NicheDraft{draft}, sampleTitleQualityProfile())
	if len(drafts) != 1 {
		t.Fatalf("single-digit high score should normalize when rating/reasoning support conversion, issues=%v", issues)
	}
	if drafts[0].DimensionScores.CreatorFit != 90 || drafts[0].DimensionRatings.CreatorFit != "very_high" {
		t.Fatalf("score/rating not normalized: scores=%#v ratings=%#v issues=%v", drafts[0].DimensionScores, drafts[0].DimensionRatings, issues)
	}
}

func TestAIScoreValidationRejectsSingleDigitPositiveReasoningWithoutRating(t *testing.T) {
	draft := baseNicheDraft("draft-no-rating", "Missing rating candidate", "cross-platform apps")
	draft.DimensionScores.CreatorFit = 9
	draft.DimensionRatings.CreatorFit = ""
	draft.DimensionReasoning.CreatorFit = "Strong alignment with the creator profile."
	drafts, issues := validateNicheDrafts([]NicheDraft{draft}, sampleTitleQualityProfile())
	if len(drafts) != 0 {
		t.Fatalf("single-digit positive reasoning without rating should be rejected")
	}
	if !strings.Contains(strings.Join(issues, " "), "positive reasoning") {
		t.Fatalf("issues should explain positive reasoning conflict: %v", issues)
	}
}

func TestAIScoreValidationKeepsGenuineVeryLowSingleDigit(t *testing.T) {
	draft := baseNicheDraft("draft-low", "Genuine low score candidate", "cross-platform apps")
	draft.DimensionScores.CreatorFit = 8
	draft.DimensionRatings.CreatorFit = "very_low"
	draft.DimensionReasoning.CreatorFit = "Very low fit for the supplied creator background."
	drafts, issues := validateNicheDrafts([]NicheDraft{draft}, sampleTitleQualityProfile())
	if len(drafts) != 1 {
		t.Fatalf("genuine very-low score rejected, issues=%v", issues)
	}
	if drafts[0].DimensionScores.CreatorFit != 8 {
		t.Fatalf("very-low score changed to %.1f", drafts[0].DimensionScores.CreatorFit)
	}
}

func TestAIScoreValidationAdjustsRatingBandMismatch(t *testing.T) {
	draft := baseNicheDraft("draft-mismatch", "Mismatched score candidate", "cross-platform apps")
	draft.DimensionScores.AudienceDemand = 82
	draft.DimensionRatings.AudienceDemand = "moderate"
	drafts, issues := validateNicheDrafts([]NicheDraft{draft}, sampleTitleQualityProfile())
	if len(drafts) != 1 {
		t.Fatalf("rating-band mismatch should be adjusted, issues=%v", issues)
	}
	if drafts[0].DimensionRatings.AudienceDemand != "very_high" {
		t.Fatalf("rating band = %q, want very_high", drafts[0].DimensionRatings.AudienceDemand)
	}
}

func TestWeightedScoreUsesNormalIntegerRoundingAndIgnoresModelOverall(t *testing.T) {
	dimensions := NicheScoreDimensions{
		CreatorFit:             ScoreExplanation{Score: 9},
		AudienceDemand:         ScoreExplanation{Score: 80},
		CompetitionOpportunity: ScoreExplanation{Score: 100},
		Sustainability:         ScoreExplanation{Score: 8},
		Differentiation:        ScoreExplanation{Score: 7},
	}
	if got := calculateNicheOverallScore(dimensions); got != 45 {
		t.Fatalf("weighted 44.55 rounded to %.1f, want 45", got)
	}
}

func TestTitleQualityAndPillarNormalization(t *testing.T) {
	pillars := []ContentPillar{
		{Name: "Tutorials", Percentage: 40, TopicCount: 3},
		{Name: "tutorials", Percentage: 20, TopicCount: 2},
		{Name: "", Percentage: 40, TopicCount: 4},
		{Name: "Case studies", Percentage: 40, TopicCount: 4},
	}
	raw := []VideoTopic{
		{Title: "Build a login screen from scratch", Pillar: "Tutorials", Intent: "tutorial"},
		{Title: "Build a login screen from scratch", Pillar: "Tutorials", Intent: "tutorial"},
		{Title: "Cross-platform apps for vibe coders and students: full complete workflow guide", Pillar: "Tutorials"},
		{Title: "Visa rule confirmed for financial apps", Pillar: "Case studies"},
		{Title: "I tested three app builders on the same idea", Pillar: "Case studies", Intent: "experiment"},
	}
	titles := validateVideoTitles(raw, pillars, "cross-platform apps", "vibe coders and students")
	if len(titles) != 2 {
		t.Fatalf("titles = %#v, want two clean titles", titles)
	}
	normalized := normalizeContentPillars(pillars, titles)
	if len(normalized) != 2 {
		t.Fatalf("pillars = %#v, want duplicate/empty names normalized to two pillars", normalized)
	}
	total := 0.0
	for _, pillar := range normalized {
		if pillar.Name == "" || pillar.TopicCount == 0 {
			t.Fatalf("invalid pillar after normalization: %#v", pillar)
		}
		total += pillar.Percentage
	}
	if math.Abs(total-100) > 0.2 {
		t.Fatalf("pillar percentages total %.1f, want approximately 100", total)
	}
}

func TestRunwayValidationKeepsTruthfulIncompleteIdeasAndMatchingPillars(t *testing.T) {
	draft := baseNicheDraft("draft-50", "Cross-platform app tutorials", "cross-platform apps")
	drafts, issues := validateNicheDrafts([]NicheDraft{draft}, sampleTitleQualityProfile())
	if len(drafts) != 1 {
		t.Fatalf("draft rejected: %v", issues)
	}
	candidate := buildAICandidate(drafts[0], sampleTitleQualityProfile(), nil, time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC))
	if len(candidate.RecommendedTitles) != 10 || candidate.Runway.ViableTopicCount != 10 {
		t.Fatalf("ideas=%d runway=%d, want 10", len(candidate.RecommendedTitles), candidate.Runway.ViableTopicCount)
	}
	if candidate.Runway.WeeklyCapacity != 2 || candidate.Runway.EstimatedWeeks != 5 {
		t.Fatalf("runway = %#v, want 5 weeks at 2/week", candidate.Runway)
	}
	total := 0
	for _, pillar := range candidate.ContentPillars {
		total += pillar.TopicCount
	}
	if total != len(candidate.RecommendedTitles) {
		t.Fatalf("pillar topic total = %d, ideas = %d", total, len(candidate.RecommendedTitles))
	}
	if candidate.Runway.Heading != "Initial content runway" || len(candidate.First10Titles) != 10 || len(candidate.RecommendedTitles) != 10 {
		t.Fatalf("runway/preview mismatch: heading=%q first10=%d full=%d", candidate.Runway.Heading, len(candidate.First10Titles), len(candidate.RecommendedTitles))
	}
}

func TestFinalResponseValidationKeepsIncompleteRunwayTruthful(t *testing.T) {
	draft := baseNicheDraft("draft-10", "Ten idea app tutorials", "cross-platform apps")
	drafts, issues := validateNicheDrafts([]NicheDraft{draft}, sampleTitleQualityProfile())
	if len(drafts) != 1 {
		t.Fatalf("draft rejected: %v", issues)
	}
	candidate := buildAICandidate(drafts[0], sampleTitleQualityProfile(), nil, time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC))
	candidate.RecommendedTitles = candidate.RecommendedTitles[:10]
	candidate.VideoTopics = candidate.RecommendedTitles
	candidate.Runway = ContentRunway{ViableTopicCount: 50, EstimatedWeeks: 25, WeeklyCapacity: 2, EstimatedContentRunway: "25 weeks at stated capacity", Heading: "50-video runway"}
	report := NicheReport{Status: StatusOK, Profile: sampleTitleQualityProfile(), Candidates: []NicheCandidate{candidate}}

	finalized, _, ok := FinalizeNicheReportForResponse(report)
	if !ok {
		t.Fatalf("finalizer rejected incomplete but valid runway")
	}
	primary := finalized.PrimaryRecommendation
	if primary == nil {
		t.Fatalf("missing primary")
	}
	if len(primary.RecommendedTitles) != 10 || primary.Runway.ViableTopicCount != 10 || primary.Runway.EstimatedWeeks != 5 {
		t.Fatalf("runway = ideas %d count %d weeks %d, want 10/10/5", len(primary.RecommendedTitles), primary.Runway.ViableTopicCount, primary.Runway.EstimatedWeeks)
	}
	if primary.Runway.Heading != "Initial content runway" || primary.Runway.Limitation == "" {
		t.Fatalf("runway should be explicitly incomplete: %#v", primary.Runway)
	}
}

func TestDuplicateAndNearDuplicateTitlesRejected(t *testing.T) {
	pillars := []ContentPillar{{Name: "Tutorials"}}
	titles := validateVideoTitles([]VideoTopic{
		{Title: "Build a login screen from scratch", Pillar: "Tutorials"},
		{Title: "Build login screen from scratch!", Pillar: "Tutorials"},
		{Title: "Compare Flutter and React Native for students", Pillar: "Tutorials"},
	}, pillars, "cross-platform apps", "students")
	if len(titles) != 2 {
		t.Fatalf("titles = %#v, want exact/near duplicate removed", titles)
	}
}

func TestFashionReportRejectsSoftwareContaminatedIdeas(t *testing.T) {
	draft := fashionDraft()
	draft.RecommendedTitles = append(draft.RecommendedTitles,
		VideoTopic{Title: "Fastest path versus safest path for a login flow", Pillar: "Budget styling"},
		VideoTopic{Title: "Five mistakes beginners make building a data screen", Pillar: "Body confidence"},
		VideoTopic{Title: "I tried three approaches to the same settings page", Pillar: "Beginner outfits"},
		VideoTopic{Title: "Case study: turning a rough idea into a student app", Pillar: "Personal style"},
	)
	drafts, _ := validateNicheDrafts([]NicheDraft{draft}, fashionProfile())
	if len(drafts) != 1 {
		t.Fatalf("fashion draft rejected entirely")
	}
	for _, topic := range drafts[0].RecommendedTitles {
		lower := strings.ToLower(topic.Title)
		for _, forbidden := range []string{"login flow", "data screen", "settings page", "student app"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("contaminated idea survived: %q", topic.Title)
			}
		}
	}
}

func TestSoftwareReportKeepsRelevantSoftwareIdeas(t *testing.T) {
	pillars := []ContentPillar{{Name: "Tutorials"}, {Name: "Case studies"}}
	titles := validateVideoTitles([]VideoTopic{
		{Title: "Build a login screen from scratch", Pillar: "Tutorials"},
		{Title: "I rebuilt a student app in one weekend", Pillar: "Case studies"},
		{Title: "Budget fashion advice for different body types", Pillar: "Tutorials"},
	}, pillars, "cross-platform app tutorials", "students")
	if len(titles) != 2 {
		t.Fatalf("software titles = %#v, want two relevant software ideas", titles)
	}
}

func TestFinalIdeasReferenceValidPrimaryPillarsAndHideInternalLabels(t *testing.T) {
	draft := baseNicheDraft("draft-pillars", "Cross-platform app tutorials", "cross-platform apps")
	draft.RecommendedTitles = append(draft.RecommendedTitles, VideoTopic{Title: "Build a fashion capsule wardrobe", Pillar: "Foreign pillar", EvidenceStatus: evidenceModeAIStrategicAnalysis})
	drafts, _ := validateNicheDrafts([]NicheDraft{draft}, sampleTitleQualityProfile())
	candidate := buildAICandidate(drafts[0], sampleTitleQualityProfile(), nil, time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC))
	report := NicheReport{Status: StatusOK, Profile: sampleTitleQualityProfile(), Candidates: []NicheCandidate{candidate}}
	finalized, _, ok := FinalizeNicheReportForResponse(report)
	if !ok {
		t.Fatalf("finalizer rejected valid report")
	}
	validPillars := validPillarMap(finalized.PrimaryRecommendation.ContentPillars)
	body, _ := json.Marshal(finalized)
	if strings.Contains(strings.ToLower(string(body)), "ai_strategic_analysis") {
		t.Fatalf("internal evidence label leaked: %s", string(body))
	}
	for _, topic := range finalized.PrimaryRecommendation.RecommendedTitles {
		if !validPillars[strings.ToLower(topic.Pillar)] {
			t.Fatalf("topic references invalid pillar: %#v", topic)
		}
	}
}

func TestFocusedIdeaRepairRequestsOnlyMissingCount(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	strategist := &fakeRepairStrategist{result: strategyWithCandidates(1), repairedIdeas: repairIdeas(40), wantMissing: 40}
	report, err := ResearchNiches(context.Background(), NicheResearchRequest{Profile: sampleAIProfile()}, NicheResearchConfig{
		Strategist: strategist,
		Now:        func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("ResearchNiches error: %v", err)
	}
	if strategist.ideaRepairCalls != 1 || strategist.missingCount != 40 {
		t.Fatalf("repair calls=%d missing=%d, want one repair for 40", strategist.ideaRepairCalls, strategist.missingCount)
	}
	if report.PrimaryRecommendation == nil || len(report.PrimaryRecommendation.RecommendedTitles) != 50 {
		t.Fatalf("repair did not complete runway")
	}
}

func TestOldEvidenceCannotBeHighConfidenceOrLiveValidated(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	old := now.AddDate(0, -5, 0).Format(time.RFC3339)
	views := uint64(5760041)
	videos := []ChannelVideoSummary{{VideoID: "old1", Title: "Fashion advice for beginners", ChannelID: "ch1", PublishedAt: old, Views: &views}}
	validation := buildNicheValidation(nicheBlueprint{CorePhrase: "beginner fashion guidance", QueryPhrases: []string{"beginner fashion guidance"}}, videos, nil, now)
	ev := nicheEvidence{videos: videos, query: "beginner fashion guidance", mode: evidenceModeLiveValidated, collectedAt: now}
	me := marketEvidenceFromValidation(validation, ev)
	candidate := buildAICandidate(fashionDraft(), fashionProfile(), nil, now)
	candidate.Validation = validation
	candidate.MarketEvidence = me
	candidate.Sustainability.ViableTopicCount = 50
	candidate.RecommendedTitles = repairFashionIdeas(50)
	candidate = finalizeEvidenceAndConfidence(candidate)
	if candidate.Confidence == "high" || candidate.MarketEvidence.Status == evidenceModeLiveValidated {
		t.Fatalf("old evidence confidence/status = %q/%q, want not high/live", candidate.Confidence, candidate.MarketEvidence.Status)
	}
}

func TestRecentEvidenceCanReturnLiveValidated(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	videos := sampleNicheVideos(now)
	validation := buildNicheValidation(nicheBlueprint{CorePhrase: "cross-platform apps", QueryPhrases: []string{"cross-platform apps"}}, videos, nil, now)
	me := marketEvidenceFromValidation(validation, nicheEvidence{videos: videos, query: "cross-platform apps", mode: evidenceModeLiveValidated, collectedAt: now})
	if me.Status != evidenceModeLiveValidated {
		t.Fatalf("status = %q, want live_validated", me.Status)
	}
}

func TestEvidenceQueryAvoidsBroadGeneratedTitleAsDominantQuery(t *testing.T) {
	candidate := buildAICandidate(fashionDraft(), fashionProfile(), nil, time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC))
	candidate.SearchQueriesUsed = []string{"Fashion Empowerment Hub", "Fashion"}
	queries := evidenceQueriesForCandidate(candidate)
	if len(queries) == 0 {
		t.Fatalf("missing queries")
	}
	first := strings.ToLower(queries[0])
	if first == "fashion" || first == "fashion empowerment hub" {
		t.Fatalf("dominant query too broad/generated: %v", queries)
	}
}

func TestAlternativeCandidatesRequireSemanticDistinctness(t *testing.T) {
	primary := buildAICandidate(baseNicheDraft("p", "Cross-platform app workflows", "cross-platform apps"), sampleAIProfile(), nil, time.Now())
	rename := buildAICandidate(baseNicheDraft("r", "Cross-platform app workflow guides", "cross-platform apps"), sampleAIProfile(), nil, time.Now())
	distinct := buildAICandidate(baseNicheDraft("d", "No-code MVP build tutorials", "no-code mvp builds"), sampleAIProfile(), nil, time.Now())
	alts := distinctAlternativeCandidates(primary, []NicheCandidate{rename, distinct}, 2)
	if len(alts) != 1 || alts[0].Name != distinct.Name {
		t.Fatalf("alternatives = %#v, want only distinct candidate", alts)
	}
}

func TestNicheSchemaVersionSeparatesCurrentCacheKey(t *testing.T) {
	profile := sampleAIProfile()
	current := NicheResearchCacheKey(profile, "market_estimate")
	legacy := LegacyNicheResearchCacheKey(profile, "market_estimate")
	if current == legacy || !strings.Contains(NicheReportSchemaVersion, "v3") {
		t.Fatalf("cache versioning failed: current=%s legacy=%s version=%s", current, legacy, NicheReportSchemaVersion)
	}
}

func TestResearchReturnsPrimaryPlusTwoAlternativesWithoutExtraYouTubeSearches(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	provider := &fakeNicheProvider{videos: sampleNicheVideos(now), channels: sampleNicheChannels()}
	report, err := ResearchNiches(context.Background(), NicheResearchRequest{Profile: sampleAIProfile()}, NicheResearchConfig{
		YouTube:                      provider,
		Strategist:                   fakeNicheStrategist{result: strategyWithCandidates(3)},
		MaxYouTubeSearchesPerRequest: 3,
		DailyYouTubeSearchLimit:      80,
		Now:                          func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("ResearchNiches error: %v", err)
	}
	if report.PrimaryRecommendation == nil || len(report.AlternativeCandidates) < 2 {
		t.Fatalf("primary/alternatives missing: primary=%v alternatives=%d", report.PrimaryRecommendation != nil, len(report.AlternativeCandidates))
	}
	if provider.searchCalls > 3 {
		t.Fatalf("search calls = %d, want capped at 3", provider.searchCalls)
	}
}

func TestRepairAttemptDoesNotConsumeYouTubeBudgetBeforeValidCandidates(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	bad := baseNicheDraft("bad", "Bad scale", "cross-platform apps")
	bad.DimensionScores.CreatorFit = 9
	bad.DimensionRatings.CreatorFit = ""
	bad.DimensionReasoning.CreatorFit = "Strong alignment with the creator profile."
	repaired := strategyWithCandidates(3)
	strategist := &fakeRepairStrategist{
		result:   NicheStrategyResult{CreatorProfileSummary: "bad", Candidates: []NicheDraft{bad}},
		repaired: repaired,
	}
	provider := &fakeNicheProvider{videos: sampleNicheVideos(now), channels: sampleNicheChannels()}
	report, err := ResearchNiches(context.Background(), NicheResearchRequest{Profile: sampleAIProfile()}, NicheResearchConfig{
		YouTube:                      provider,
		Strategist:                   strategist,
		MaxYouTubeSearchesPerRequest: 3,
		DailyYouTubeSearchLimit:      80,
		Now:                          func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("ResearchNiches error: %v", err)
	}
	if strategist.repairCalls != 1 {
		t.Fatalf("repair calls = %d, want 1", strategist.repairCalls)
	}
	if report.Status != StatusOK || provider.searchCalls > 3 {
		t.Fatalf("status=%s searchCalls=%d", report.Status, provider.searchCalls)
	}
}

func TestEmptyAlternativeStateIsHonestLimitation(t *testing.T) {
	report, err := ResearchNiches(context.Background(), NicheResearchRequest{Profile: sampleAIProfile()}, NicheResearchConfig{
		Strategist: fakeNicheStrategist{result: strategyWithCandidates(1)},
		Now:        func() time.Time { return time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("ResearchNiches error: %v", err)
	}
	if len(report.AlternativeCandidates) != 0 {
		t.Fatalf("alternatives = %d, want none", len(report.AlternativeCandidates))
	}
	if !strings.Contains(strings.ToLower(strings.Join(report.Limitations, " ")), "alternative") {
		t.Fatalf("missing honest alternative limitation: %v", report.Limitations)
	}
}

func TestOldSparseEvidenceLimitsConfidenceAndOpportunity(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	old := now.Add(-240 * 24 * time.Hour).Format(time.RFC3339)
	views := uint64(669110)
	videos := make([]ChannelVideoSummary, 12)
	for i := range videos {
		videos[i] = ChannelVideoSummary{VideoID: intString(i + 1), Title: "Old app tutorial " + intString(i+1), ChannelID: "ch1", PublishedAt: old, Views: &views}
	}
	validation := buildNicheValidation(nicheBlueprint{CorePhrase: "cross-platform apps", QueryPhrases: []string{"cross-platform apps"}}, videos, nil, now)
	if validation.RecentPublicationVolume != 0 {
		t.Fatalf("recent volume = %d, want 0", validation.RecentPublicationVolume)
	}
	if !strings.Contains(validation.MarketEvidenceSummary, "older sampled videos") {
		t.Fatalf("summary should acknowledge old evidence: %q", validation.MarketEvidenceSummary)
	}
	gapScore := scoreOpportunityGap(validation, nil, []SupplyGap{{Statement: "gap", Confidence: "medium"}})
	conf := scoreConfidence(validation, MonetizationEstimate{}, SustainabilityEvidence{ViableTopicCount: 50})
	if gapScore >= 100 || conf > 54 {
		t.Fatalf("gapScore=%.1f confidence=%.1f, want capped by old sparse evidence", gapScore, conf)
	}
}

func TestProductionProfileFixtureLocalNicheResult(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	provider := &fakeNicheProvider{videos: sampleNicheVideos(now), channels: sampleNicheChannels()}
	strategist := &fakeRepairStrategist{result: strategyWithCandidates(3), repairedIdeas: repairIdeas(40), wantMissing: 40}
	report, err := ResearchNiches(context.Background(), NicheResearchRequest{Profile: sampleTitleQualityProfile()}, NicheResearchConfig{
		YouTube:                      provider,
		Strategist:                   strategist,
		MaxYouTubeSearchesPerRequest: 3,
		DailyYouTubeSearchLimit:      80,
		Now:                          func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("ResearchNiches error: %v", err)
	}
	primary := report.PrimaryRecommendation
	if primary == nil {
		t.Fatalf("missing primary recommendation")
	}
	if len(primary.RecommendedTitles) != 50 || primary.Runway.EstimatedWeeks != 25 {
		t.Fatalf("ideas=%d weeks=%d, want 50 ideas and 25 weeks", len(primary.RecommendedTitles), primary.Runway.EstimatedWeeks)
	}
	if strategist.ideaRepairCalls != 1 || strategist.missingCount != 40 {
		t.Fatalf("idea repair calls=%d missing=%d, want one repair for 40", strategist.ideaRepairCalls, strategist.missingCount)
	}
	if len(report.AlternativeCandidates) < 2 {
		t.Fatalf("alternatives=%d, want at least two", len(report.AlternativeCandidates))
	}
	for _, dim := range []ScoreExplanation{primary.Dimensions.CreatorFit, primary.Dimensions.AudienceDemand, primary.Dimensions.CompetitionOpportunity, primary.Dimensions.Sustainability, primary.Dimensions.Differentiation} {
		if dim.Score < 0 || dim.Score > 100 || dim.Score != math.Round(dim.Score) {
			t.Fatalf("invalid dimension score: %#v", dim)
		}
		if dim.Score <= 10 && (dim.RatingBand == "high" || dim.RatingBand == "very_high") {
			t.Fatalf("single-digit score has positive rating: %#v", dim)
		}
	}
	want := calculateNicheOverallScore(primary.Dimensions)
	if primary.OverallScore != want {
		t.Fatalf("overall=%.1f, want %.1f", primary.OverallScore, want)
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	lower := strings.ToLower(string(body))
	for _, forbidden := range []string{"rpm", "revenue", "earnings", "monetization", "monetisation"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("niche response contains forbidden monetization term %q: %s", forbidden, lower)
		}
	}
	if provider.searchCalls > 3 {
		t.Fatalf("search calls=%d, want at most 3", provider.searchCalls)
	}
}

func TestRunwayUsesStatedCapacityAndHandlesMalformedCapacity(t *testing.T) {
	runway := buildRunway(50, "2 videos per week")
	if runway.WeeklyCapacity != 2 || runway.EstimatedWeeks != 25 {
		t.Fatalf("runway = %#v, want 2/week and 25 weeks", runway)
	}
	malformed := buildRunway(50, "zero videos per week")
	if malformed.WeeklyCapacity != 2 || malformed.EstimatedWeeks != 25 {
		t.Fatalf("malformed runway = %#v, want safe default", malformed)
	}
}

func TestNicheResearchCacheKeyIncludesCountryFormatLanguageAndMode(t *testing.T) {
	a := sampleAIProfile()
	b := a
	if NicheResearchCacheKey(a, "market_estimate") == NicheResearchCacheKey(b, "market_estimate") {
		// Same inputs should be stable.
	} else {
		t.Fatalf("same profile did not produce stable cache key")
	}
	b.TargetCountry = "US"
	if NicheResearchCacheKey(a, "market_estimate") == NicheResearchCacheKey(b, "market_estimate") {
		t.Fatalf("cache key did not separate countries")
	}
	b = a
	b.ContentFormats = []string{"shorts"}
	if NicheResearchCacheKey(a, "market_estimate") == NicheResearchCacheKey(b, "market_estimate") {
		t.Fatalf("cache key did not separate formats")
	}
	if NicheResearchCacheKey(a, "market_estimate") == NicheResearchCacheKey(a, "connected_channel") {
		t.Fatalf("cache key did not separate monetization mode")
	}
}

func strategyWithCandidates(count int) NicheStrategyResult {
	candidates := make([]NicheDraft, 0, count)
	for i := 0; i < count; i++ {
		names := []string{"Cross-platform app workflows", "Flutter student project guides", "No-code MVP build tutorials", "App launch mistake breakdowns", "Beginner product dashboards"}
		name := names[i%len(names)]
		query := []string{"cross-platform apps", "flutter student projects", "no-code mvp builds", "app launch mistakes", "product dashboard tutorials"}[i%5]
		candidates = append(candidates, baseNicheDraft("draft-"+intString(i+1), name, query))
	}
	return NicheStrategyResult{
		CreatorProfileSummary: "Software educator for coding students.",
		PrimaryRecommendation: candidates[0].Name,
		Candidates:            candidates,
		Methodology:           []string{"OpenAI structured strategy test fixture."},
	}
}

func strategyWithDuplicateQueries() NicheStrategyResult {
	a := baseNicheDraft("draft-1", "App workflow tutorials", "Cross Platform Apps")
	b := baseNicheDraft("draft-2", "Practical app build guides", "cross   platform   apps")
	return NicheStrategyResult{
		CreatorProfileSummary: "Software educator for coding students.",
		PrimaryRecommendation: a.Name,
		Candidates:            []NicheDraft{a, b},
	}
}

func baseNicheDraft(id, name, query string) NicheDraft {
	pillars := []ContentPillar{
		{Name: "Tutorials", Percentage: 25, TopicCount: 3},
		{Name: "Comparisons", Percentage: 25, TopicCount: 3},
		{Name: "Mistakes", Percentage: 20, TopicCount: 2},
		{Name: "Case studies", Percentage: 20, TopicCount: 2},
		{Name: "Workflows", Percentage: 10, TopicCount: 2},
	}
	titles := []VideoTopic{
		{Title: "Build a simple app workflow from scratch", Pillar: "Tutorials", Intent: "tutorial"},
		{Title: "Flutter versus React Native for a first project", Pillar: "Comparisons", Intent: "comparison"},
		{Title: "Five mistakes beginners make before launch", Pillar: "Mistakes", Intent: "mistake"},
		{Title: "I rebuilt a student app in one weekend", Pillar: "Case studies", Intent: "case study"},
		{Title: "I tried three builders and tracked the tradeoffs", Pillar: "Workflows", Intent: "experiment"},
		{Title: "A weekly workflow for shipping small apps", Pillar: "Workflows", Intent: "workflow"},
		{Title: "Start here if you are new to app development", Pillar: "Tutorials", Intent: "beginner guide"},
		{Title: "What most advice gets wrong about app tools", Pillar: "Comparisons", Intent: "opinion"},
		{Title: "A practical breakdown of auth and storage", Pillar: "Tutorials", Intent: "breakdown"},
		{Title: "The tool stack I would pick for a class project", Pillar: "Comparisons", Intent: "tool breakdown"},
	}
	return NicheDraft{
		ID:                     id,
		Name:                   name,
		ConcisePositioning:     name + " for vibe coders and students.",
		Category:               "Technology",
		Subcategory:            "Cross-platform apps",
		TargetAudience:         "vibe coders and students",
		AudienceProblems:       []string{"Need practical app-building guidance."},
		CreatorAdvantages:      []string{"Can teach coding through real app builds."},
		UniqueAngle:            "Practical app builds with clear tradeoffs.",
		DimensionScores:        NicheDraftDimensionScores{CreatorFit: 80, AudienceDemand: 70, CompetitionOpportunity: 65, Sustainability: 85, Differentiation: 72},
		DimensionRatings:       NicheDraftDimensionRatings{CreatorFit: "very_high", AudienceDemand: "high", CompetitionOpportunity: "high", Sustainability: "very_high", Differentiation: "high"},
		DimensionReasoning:     NicheDraftDimensionReasoning{CreatorFit: "Matches coding background.", AudienceDemand: "Audience has practical learning demand.", CompetitionOpportunity: "Specific positioning avoids broad tutorials.", Sustainability: "Enough workflows and case studies.", Differentiation: "Creator can show practical builds."},
		ContentPillars:         pillars,
		RecommendedTitles:      titles,
		OpportunityGaps:        []string{"Few practical app-building series connect tools to shipped examples."},
		Risks:                  []string{"Tool-specific content can date quickly."},
		RecommendedFirstAction: "Publish one beginner workflow and one tool comparison.",
		SearchQuery:            query,
		Runway:                 "50-title starter runway",
		Reasoning:              "Structured strategy fixture.",
	}
}

func repairIdeas(count int) []VideoTopic {
	pillars := []string{"Tutorials", "Comparisons", "Mistakes", "Case studies", "Workflows"}
	angles := []string{
		"onboarding flow", "offline sync", "user profile", "payment settings", "notification center",
		"search filters", "admin table", "mobile navigation", "file upload", "team permissions",
		"error states", "empty states", "analytics chart", "project setup", "deployment checklist",
		"form validation", "accessibility pass", "performance audit", "database model", "API contract",
		"authentication edge case", "responsive layout", "state management", "testing plan", "release notes",
		"student portfolio", "course tracker", "booking calendar", "habit dashboard", "notes editor",
		"invoice screen", "chat interface", "settings modal", "pricing page", "content queue",
		"review workflow", "import wizard", "export tool", "feedback panel", "starter template",
	}
	out := make([]VideoTopic, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, VideoTopic{
			Title:      "Build a " + angles[i%len(angles)] + " for student app builders",
			Pillar:     pillars[i%len(pillars)],
			Intent:     "tutorial",
			Difficulty: "medium",
		})
	}
	return out
}

func fashionProfile() CreatorNicheProfile {
	return CreatorNicheProfile{
		ProfessionalSkills:       "personal styling and beginner fashion education",
		Hobbies:                  "fashion, inclusive styling, budget outfits",
		TeachingSubjects:         "beginner fashion guidance and body type styling",
		TargetAudience:           "beginners building confidence with affordable outfits",
		TargetCountry:            "GB",
		TargetLanguage:           "en",
		CreatorPresence:          "personal brand",
		ContentFormats:           []string{"long-form"},
		OptionalBroadTopic:       "Fashion",
		WeeklyProductionCapacity: "2 videos per week",
	}
}

func fashionDraft() NicheDraft {
	pillars := []ContentPillar{{Name: "Budget styling"}, {Name: "Body confidence"}, {Name: "Beginner outfits"}, {Name: "Personal style"}}
	return NicheDraft{
		ID:                 "fashion-primary",
		Name:               "Inclusive beginner fashion guidance",
		Category:           "Fashion",
		Subcategory:        "Beginner personal styling",
		TargetAudience:     "beginners building confidence with affordable outfits",
		AudienceProblems:   []string{"Viewers need affordable styling advice for different body types."},
		CreatorAdvantages:  []string{"Can explain styling choices with inclusive examples."},
		UniqueAngle:        "Budget-conscious, inclusive personal styling for beginners.",
		DimensionScores:    NicheDraftDimensionScores{CreatorFit: 82, AudienceDemand: 72, CompetitionOpportunity: 68, Sustainability: 80, Differentiation: 75},
		DimensionRatings:   NicheDraftDimensionRatings{CreatorFit: "very_high", AudienceDemand: "high", CompetitionOpportunity: "high", Sustainability: "very_high", Differentiation: "high"},
		DimensionReasoning: NicheDraftDimensionReasoning{CreatorFit: "Fashion profile fit.", AudienceDemand: "Specific beginner demand.", CompetitionOpportunity: "Narrow styling angle.", Sustainability: "Several repeatable formats.", Differentiation: "Inclusive and budget-specific."},
		ContentPillars:     pillars,
		RecommendedTitles:  repairFashionIdeas(10),
		OpportunityGaps:    []string{"Beginner fashion advice often lacks inclusive body-type examples."},
		Risks:              []string{"Fashion examples can become seasonal."},
		SearchQuery:        "beginner fashion guidance",
	}
}

func repairFashionIdeas(count int) []VideoTopic {
	pillars := []string{"Budget styling", "Body confidence", "Beginner outfits", "Personal style"}
	angles := []string{"capsule wardrobe", "body type outfit", "budget blazer look", "first date outfit", "workwear basics", "colour matching", "shoe pairing", "makeup and outfit balance", "seasonal layering", "confidence audit"}
	out := []VideoTopic{}
	for i := 0; i < count; i++ {
		out = append(out, VideoTopic{Title: "Beginner fashion guide to " + angles[i%len(angles)] + " " + intString(i+1), Pillar: pillars[i%len(pillars)], Intent: "tutorial", Difficulty: "low"})
	}
	return out
}

func sampleTitleQualityProfile() CreatorNicheProfile {
	return CreatorNicheProfile{
		ProfessionalSkills:       "teach how to code",
		Hobbies:                  "coding",
		LivedExperiences:         "how to build apps",
		TeachingSubjects:         "cross-platform apps",
		TargetAudience:           "vibe coders and students",
		TargetCountry:            "US",
		TargetLanguage:           "en",
		CreatorPresence:          "faceless",
		ContentFormats:           []string{"long-form"},
		OptionalBroadTopic:       "programming",
		WeeklyProductionCapacity: "2 videos per week",
	}
}

func sampleAIProfile() CreatorNicheProfile {
	return CreatorNicheProfile{
		ProfessionalSkills:       "software development and AI automation",
		LivedExperiences:         "building automation tools for teams",
		TeachingSubjects:         "AI workflows and software systems",
		ThreeYearsAgoAdvice:      "how to automate repetitive business tasks",
		TargetAudience:           "UK small businesses",
		TargetCountry:            "GB",
		TargetLanguage:           "en",
		CreatorPresence:          "faceless channel",
		ContentFormats:           []string{"long-form"},
		PrimaryMonetizationGoal:  "affiliate marketing and AdSense",
		OptionalBroadTopic:       "AI automation",
		WeeklyProductionCapacity: "2 videos per week",
	}
}

func sampleNicheVideos(now time.Time) []ChannelVideoSummary {
	views := []uint64{45000, 120000, 18000, 90000, 400000, 26000, 8000, 61000, 125000, 30000, 77000, 15000}
	out := []ChannelVideoSummary{}
	for i, view := range views {
		likes := view / 40
		comments := view / 300
		channelID := "mid"
		if i%3 == 0 {
			channelID = "small"
		}
		if i%5 == 0 {
			channelID = "hidden"
		}
		out = append(out, ChannelVideoSummary{
			VideoID:      "vid" + intString(i),
			Title:        []string{"How to automate invoices with AI", "AI automation workflow for UK business", "Best AI tools for small business?", "Beginner AI automation tutorial"}[i%4],
			Description:  "Public metadata description about pricing and setup questions.",
			ChannelID:    channelID,
			ChannelTitle: "Channel " + channelID,
			PublishedAt:  now.Add(time.Duration(-i*12) * time.Hour).Format(time.RFC3339),
			Views:        &view,
			Likes:        &likes,
			Comments:     &comments,
		})
	}
	return out
}

func sampleNicheChannels() map[string]NicheChannelStats {
	smallSubs := uint64(12000)
	midSubs := uint64(90000)
	return map[string]NicheChannelStats{
		"small":  {ChannelID: "small", Title: "Small Channel", Subscribers: &smallSubs},
		"mid":    {ChannelID: "mid", Title: "Mid Channel", Subscribers: &midSubs},
		"hidden": {ChannelID: "hidden", Title: "Hidden Channel", HiddenSubs: true},
	}
}
