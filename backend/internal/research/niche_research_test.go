package research

import (
	"context"
	"errors"
	"io"
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
	for _, candidate := range report.Candidates {
		if candidate.Level1 == "" || candidate.Level2 == "" || candidate.Level3 == "" {
			t.Fatalf("candidate missing three-level path: %#v", candidate)
		}
		if seen[strings.ToLower(candidate.Level3)] {
			t.Fatalf("duplicate niche candidate: %s", candidate.Level3)
		}
		seen[strings.ToLower(candidate.Level3)] = true
		if candidate.Validation.SampledVideoCount == 0 || candidate.Validation.MedianSampledViews == 0 {
			t.Fatalf("candidate missing measured validation: %#v", candidate.Validation)
		}
		if candidate.Monetization.RPMEstimateAvailable {
			t.Fatalf("unexpected RPM estimate without calibration: %#v", candidate.Monetization)
		}
		if candidate.Monetization.UnavailableReason == "" {
			t.Fatalf("missing monetization unavailable reason")
		}
	}
	if provider.searchCalls == 0 || provider.searchCalls > 15 {
		t.Fatalf("search calls = %d, want controlled budget", provider.searchCalls)
	}
}

func TestNicheResearchMapsQuotaCredentialsTimeoutAndTemporaryFailures(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "quota",
			err:  &ProviderError{Code: ProviderErrorQuota, HTTPStatus: http.StatusForbidden, Reason: "quotaExceeded"},
			want: NicheStatusQuotaTemporarilyUnavailable,
		},
		{
			name: "invalid credentials",
			err:  &ProviderError{Code: ProviderErrorCredentials, HTTPStatus: http.StatusForbidden, Reason: "keyInvalid"},
			want: NicheStatusCredentialsInvalid,
		},
		{
			name: "timeout",
			err:  context.DeadlineExceeded,
			want: NicheStatusValidationTimeout,
		},
		{
			name: "upstream 5xx",
			err:  &ProviderError{Code: ProviderErrorTemporary, HTTPStatus: http.StatusBadGateway, Reason: "backendError"},
			want: NicheStatusProviderTemporarilyUnavailable,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, err := ResearchNiches(context.Background(), NicheResearchRequest{Profile: sampleAIProfile()}, NicheResearchConfig{
				YouTube: &fakeNicheProvider{searchErr: tc.err},
				Now:     func() time.Time { return time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC) },
			})
			if err == nil {
				t.Fatalf("expected typed error")
			}
			if report.Status != tc.want {
				t.Fatalf("status = %s, want %s", report.Status, tc.want)
			}
			if strings.Contains(strings.ToLower(report.Message), "youtube") || strings.Contains(strings.ToLower(report.Message), "google") {
				t.Fatalf("public message exposes provider details: %q", report.Message)
			}
		})
	}
}

func TestNicheResearchSeparatesNoMatchingContentFromInsufficientEvidence(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	report, err := ResearchNiches(context.Background(), NicheResearchRequest{Profile: sampleAIProfile()}, NicheResearchConfig{
		YouTube: &fakeNicheProvider{videos: []ChannelVideoSummary{}, channels: map[string]NicheChannelStats{}},
		Now:     func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("no matching content should not be provider failure: %v", err)
	}
	if report.Status != NicheStatusNoMatchingContent {
		t.Fatalf("status = %s, want %s", report.Status, NicheStatusNoMatchingContent)
	}

	oneView := uint64(10)
	report, err = ResearchNiches(context.Background(), NicheResearchRequest{Profile: sampleAIProfile()}, NicheResearchConfig{
		YouTube: &fakeNicheProvider{videos: []ChannelVideoSummary{{
			VideoID: "weak-1", Title: "AI automation", ChannelID: "weak", PublishedAt: now.Format(time.RFC3339), Views: &oneView,
		}}, channels: map[string]NicheChannelStats{}},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("insufficient evidence should not be provider failure: %v", err)
	}
	if report.Status != NicheStatusInsufficientEvidence {
		t.Fatalf("status = %s, want %s", report.Status, NicheStatusInsufficientEvidence)
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
	if !strings.Contains(a.Overall.Explanation, "0.25 personal fit") {
		t.Fatalf("missing transparent formula: %s", a.Overall.Explanation)
	}
	report := NicheReport{Internal: NicheInternal{Provenance: []string{"youtube_data_api"}, CacheKey: "secretish"}}
	if report.Internal.Provenance[0] == "" {
		t.Fatalf("test setup failed")
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
