package research

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestYouTubeProviderNotConfiguredIsHonest(t *testing.T) {
	provider := NewYouTubeProvider("", nil)

	status := provider.Status()
	if status.Status != StatusNotConfigured {
		t.Fatalf("Status = %q, want %q", status.Status, StatusNotConfigured)
	}
	if !strings.Contains(status.Message, "YOUTUBE_API_KEY") {
		t.Fatalf("Message = %q, want YOUTUBE_API_KEY guidance", status.Message)
	}

	result, err := provider.AnalyzeVideo(context.Background(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err != ErrNotConfigured {
		t.Fatalf("err = %v, want ErrNotConfigured", err)
	}
	if result.Status != StatusNotConfigured {
		t.Fatalf("result.Status = %q, want %q", result.Status, StatusNotConfigured)
	}
	if len(result.Limitations) == 0 {
		t.Fatalf("result.Limitations is empty")
	}
}

func TestExtractYouTubeVideoID(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ":       "dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ?si=abc":               "dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ":        "dQw4w9WgXcQ",
		"https://www.youtube.com/embed/dQw4w9WgXcQ?start=1": "dQw4w9WgXcQ",
		"dQw4w9WgXcQ": "dQw4w9WgXcQ",
	}
	for input, want := range cases {
		got, err := ExtractYouTubeVideoID(input)
		if err != nil {
			t.Fatalf("ExtractYouTubeVideoID(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("ExtractYouTubeVideoID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTikTokResearchStatusDoesNotReturnFakeActiveData(t *testing.T) {
	status := TikTokResearchStatus("", "", "")
	if status.Status != StatusNotConfigured {
		t.Fatalf("Status = %q, want %q", status.Status, StatusNotConfigured)
	}

	configured := TikTokResearchStatus("client", "secret", "")
	if configured.Status != StatusUnavailable {
		t.Fatalf("configured Status = %q, want %q", configured.Status, StatusUnavailable)
	}
	if strings.Contains(strings.ToLower(configured.Message), "active") {
		t.Fatalf("configured Message = %q should not imply active fake trend data", configured.Message)
	}
}

func TestKeywordExtractorRemovesNoiseAndFindsPhrases(t *testing.T) {
	got := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title:        "I Tested AI Video Automation for YouTube Shorts",
		Description:  "https://youtu.be/abcDEF12345 random ID xYz98765432. Learn AI video automation workflows for creators. #AIVideo #Shorts",
		Tags:         []string{"AI video automation", "creator workflow", "abcDEF12345"},
		ChannelTitle: "Creator Lab",
	})
	joined := strings.Join(append(append(got.PrimaryKeywords, got.SecondaryKeywords...), got.LongTailPhrases...), " ")
	if strings.Contains(joined, "abcdef12345") || strings.Contains(joined, "https") {
		t.Fatalf("keywords include noise: %+v", got)
	}
	if !strings.Contains(joined, "ai automation") {
		t.Fatalf("keywords missing meaningful phrase: %+v", got)
	}
	if len(got.Hashtags) == 0 || got.Hashtags[0] != "#aivideo" {
		t.Fatalf("hashtags = %+v, want #aivideo", got.Hashtags)
	}
}

func TestKeywordExtractorRejectsMetadataPollution(t *testing.T) {
	got := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title: "Install Codex Skills in Every Project",
		Description: `Learn a repeatable Codex setup workflow.

https://example.com/?utm_source=youtube&utm_medium=description&magicPath=abc
00:43 install step
Follow me on Instagram
Business inquiries: hello@example.com
Affiliate link: https://shop.example.com/ref?id=123
#CodexSkills #CodexSkills #AIWorkflow`,
		Tags: []string{"Codex skills", "AI workflow", "utm_source"},
	})
	joined := strings.ToLower(strings.Join(append(append(got.PrimaryKeywords, got.SecondaryKeywords...), got.LongTailPhrases...), " "))
	for _, noise := range []string{"utm_source", "utm_medium", "magicpath", "instagram", "affiliate", "hello", "example.com"} {
		if strings.Contains(joined, noise) {
			t.Fatalf("keywords include noise %q: %+v", noise, got)
		}
	}
	if len(got.Hashtags) != 2 {
		t.Fatalf("hashtags = %+v, want deduped hashtags", got.Hashtags)
	}
	if !strings.Contains(joined, "codex skills") && !strings.Contains(joined, "ai workflow") {
		t.Fatalf("missing source-specific topics: %+v", got)
	}
}

func TestKeywordExtractorRemovesFinancialDisclaimerAfterTradingContent(t *testing.T) {
	got := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title: "Every ICT Concept Explained in 20 Minutes",
		Description: `This lesson explains ICT trading concepts: liquidity, fair value gaps, order blocks, market structure, displacement, breaker blocks, premium and discount.

Disclaimer: This is not financial advice. Always do your own research and consult a licensed financial adviser before making any investment decisions. Past performance is not indicative of future results. Nothing here says you should buy or sell any asset. For educational purposes only.`,
		Tags: []string{"ICT trading", "liquidity", "fair value gap", "order block", "market structure"},
	})
	joined := strings.ToLower(strings.Join(append(append(got.PrimaryKeywords, got.SecondaryKeywords...), got.LongTailPhrases...), " | "))
	for _, want := range []string{"ict", "liquidity", "fair value gap", "order block", "market structure"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected trading concept %q to survive: %+v", want, got)
		}
	}
	for _, noise := range []string{"not", "own research", "future results", "funded days", "investment decisions", "licensed financial", "should buy", "buy sell", "educational purposes"} {
		if strings.Contains(joined, noise) {
			t.Fatalf("keywords include disclaimer noise %q: %+v", noise, got)
		}
	}
}

func TestKeywordExtractorPreservesNonFinancialEducationalThemes(t *testing.T) {
	got := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title:       "Photosynthesis Explained for Beginners",
		Description: "A biology lesson about chlorophyll, sunlight, carbon dioxide, glucose, oxygen, and plant cells. Includes a simple classroom experiment.",
		Tags:        []string{"biology", "photosynthesis", "plant cells", "science lesson"},
		Category:    "27",
	})
	joined := strings.ToLower(strings.Join(append(append(got.PrimaryKeywords, got.SecondaryKeywords...), got.LongTailPhrases...), " | "))
	for _, want := range []string{"photosynthesis", "biology", "plant cell"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected educational theme %q to survive: %+v", want, got)
		}
	}
	if strings.Contains(joined, "financial") {
		t.Fatalf("unexpected finance leakage in non-financial video: %+v", got)
	}
}

func TestKeywordExtractorUsesTitleAndTagsWhenDescriptionIsPromotional(t *testing.T) {
	got := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title: "Beginner Python Decorators Explained",
		Description: `Subscribe for more videos!
Follow me on Instagram and TikTok.
Business inquiries: hello@example.com
Affiliate links: https://example.com/gear
Use code SAVE20 and join my Discord.`,
		Tags: []string{"python decorators", "python tutorial", "programming"},
	})
	joined := strings.ToLower(strings.Join(append(append(got.PrimaryKeywords, got.SecondaryKeywords...), got.LongTailPhrases...), " | "))
	for _, noise := range []string{"subscribe", "instagram", "business inquiries", "affiliate", "save20", "discord"} {
		if strings.Contains(joined, noise) {
			t.Fatalf("keywords include promotional noise %q: %+v", noise, got)
		}
	}
	if !strings.Contains(joined, "python decorator") {
		t.Fatalf("title/tags did not produce useful topic intelligence: %+v", got)
	}
}

func TestKeywordExtractorSparseMetadataDoesNotPadNoise(t *testing.T) {
	got := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title: "ICT Basics",
	})
	joined := strings.ToLower(strings.Join(append(append(got.PrimaryKeywords, got.SecondaryKeywords...), got.LongTailPhrases...), " | "))
	if strings.Contains(joined, "not") || strings.Contains(joined, "financial") {
		t.Fatalf("sparse metadata produced contaminated topics: %+v", got)
	}
	if len(got.PrimaryKeywords) > 2 || len(got.LongTailPhrases) > 2 {
		t.Fatalf("sparse metadata should not be padded: %+v", got)
	}
}

func TestKeywordExtractorMergesNearDuplicatePhrases(t *testing.T) {
	got := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title:       "Fair Value Gap Trading Explained",
		Description: "Fair value gaps, fair value gap setups, and FVG trading examples for ICT traders.",
		Tags:        []string{"fair value gaps", "fair value gap", "FVG trading"},
	})
	all := append(append(got.PrimaryKeywords, got.SecondaryKeywords...), got.LongTailPhrases...)
	count := 0
	for _, phrase := range all {
		if strings.Contains(strings.ToLower(phrase), "fair value gap") {
			count++
		}
	}
	if count > 2 {
		t.Fatalf("near duplicate fair value gap phrases were not merged enough: %+v", got)
	}
}

func TestCreativeOpportunitiesUseOnlySanitizedTopicIntelligence(t *testing.T) {
	kw := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title:       "Every ICT Concept Explained in 20 Minutes",
		Description: "ICT trading concepts include liquidity, fair value gaps, order blocks, and market structure. Not financial advice. Always do your own research and consult a licensed financial adviser.",
		Tags:        []string{"ICT trading", "liquidity", "fair value gap", "order block"},
	})
	niche := ClassifyNiche(KeywordExtractionInput{Title: "Every ICT Concept Explained in 20 Minutes"}, kw)
	if strings.Contains(niche.SpecificTopic, "explained in") {
		t.Fatalf("specific topic uses incomplete title fragment: %+v", niche)
	}
	opps := BuildCreatorOpportunities(niche, kw, AnalyzeHookIntelligence("Every ICT Concept Explained in 20 Minutes"), "Every ICT Concept Explained in 20 Minutes")
	creative := strings.ToLower(strings.Join(append(append(append(opps.TitleIdeas, opps.ScriptPrompts...), opps.ShortFormClipIdeas...), opps.SuggestedRemakeAngles...), " | "))
	for _, noise := range []string{"not financial", "own research", "licensed financial", "future results", "funded days", "using these inferred public topics: financial", "using these inferred public topics: not"} {
		if strings.Contains(creative, noise) {
			t.Fatalf("creative outputs include rejected phrase %q: %+v", noise, opps)
		}
	}
	if !strings.Contains(creative, "ict") && !strings.Contains(creative, "liquidity") && !strings.Contains(creative, "fair value gap") {
		t.Fatalf("creative outputs lost legitimate trading topics: %+v", opps)
	}
}

func TestKeywordExtractorUsesRecentVideoTitlesForChannelTopics(t *testing.T) {
	got := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title:             "MKBHD",
		Description:       "Quality tech reviews about phones, cameras, electric cars, and software.",
		TopicDetails:      []string{"https://en.wikipedia.org/wiki/Technology"},
		RecentVideoTitles: []string{"iPhone 20 Review: What Actually Changed", "The Best Android Phone Camera Test", "Tesla Robotaxi Tech Explained"},
	})
	joined := strings.ToLower(strings.Join(got.PrimaryKeywords, " "))
	if !strings.Contains(joined, "iphone") && !strings.Contains(joined, "android") && !strings.Contains(joined, "tesla") && !strings.Contains(joined, "phone") {
		t.Fatalf("recent titles did not drive primary topics: %+v", got)
	}
}

func TestAnalyzeVideoReturnsStructuredIntelligence(t *testing.T) {
	provider := NewYouTubeProvider("yt-key", &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(`{"items":[{"id":"dQw4w9WgXcQ","snippet":{"publishedAt":"2026-07-01T00:00:00Z","channelId":"UC123","title":"How AI Video Automation Works for Creators","description":"A tutorial for creator workflows. #AIVideo","channelTitle":"Creator Lab","tags":["AI video automation","creator tools"],"categoryId":"28"},"statistics":{"viewCount":"100000","likeCount":"5000","commentCount":"200"},"contentDetails":{"duration":"PT8M"},"topicDetails":{"topicCategories":["https://en.wikipedia.org/wiki/Artificial_intelligence"]}}]}`), nil
	})})
	provider.now = func() time.Time { return mustTime("2026-07-10T00:00:00Z") }

	result, err := provider.AnalyzeVideo(context.Background(), "https://www.youtube.com/watch?v=dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("AnalyzeVideo: %v", err)
	}
	if result.Status != StatusOK {
		t.Fatalf("status = %q", result.Status)
	}
	if len(result.KeywordIntelligence.PrimaryKeywords) == 0 || result.NicheAnalysis.PrimaryNiche == "" || result.HookIntelligence.HookType == "" {
		t.Fatalf("missing structured intelligence: %+v", result)
	}
	if _, ok := result.PerformanceSignals["views_per_day"]; !ok {
		t.Fatalf("missing views_per_day: %+v", result.PerformanceSignals)
	}
	if result.SchemaVersion != videoAnalysisSchemaVersion {
		t.Fatalf("schema version = %q", result.SchemaVersion)
	}
	if result.FormattedMetadata.Duration != "8m 0s" || result.FormattedMetadata.PublishedDate != "1 Jul 2026" {
		t.Fatalf("formatted metadata = %+v", result.FormattedMetadata)
	}
	if len(result.ScoreDimensions) == 0 || result.AnalysisConfidence.Score <= 0 {
		t.Fatalf("missing calibrated scores: dimensions=%+v confidence=%+v", result.ScoreDimensions, result.AnalysisConfidence)
	}
	if len(result.PerformanceProfile) == 0 {
		t.Fatalf("missing performance profile")
	}
	if result.RevenueEstimate.Source != "public_estimate" || result.RevenueEstimate.High < result.RevenueEstimate.Low || result.RevenueEstimate.Midpoint < result.RevenueEstimate.Low || result.RevenueEstimate.Midpoint > result.RevenueEstimate.High {
		t.Fatalf("bad revenue estimate: %+v", result.RevenueEstimate)
	}
	if strings.Contains(strings.ToLower(result.RevenueEstimate.CalculationBasis), "you earned") {
		t.Fatalf("revenue estimate implies actual earnings: %+v", result.RevenueEstimate)
	}
	assertNoExactRankingClaim(t, result.Limitations)
}

func TestVideoFormattingHiddenCountsAndShorts(t *testing.T) {
	provider := NewYouTubeProvider("yt-key", &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(`{"items":[{"id":"dQw4w9WgXcQ","snippet":{"publishedAt":"2026-07-09T00:00:00Z","channelId":"UC123","title":"30 Second Pasta Trick","description":"","channelTitle":"Kitchen Lab","tags":["pasta recipe"],"categoryId":"26"},"statistics":{"viewCount":"0","commentCount":"0"},"contentDetails":{"duration":"PT45S"},"topicDetails":{"topicCategories":["https://en.wikipedia.org/wiki/Cooking"]}}]}`), nil
	})})
	provider.now = func() time.Time { return mustTime("2026-07-10T00:00:00Z") }

	result, err := provider.AnalyzeVideo(context.Background(), "https://www.youtube.com/shorts/dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("AnalyzeVideo: %v", err)
	}
	if result.FormattedMetadata.Format != "short_form" {
		t.Fatalf("format = %q", result.FormattedMetadata.Format)
	}
	if result.FormattedMetadata.Views != "0" || result.FormattedMetadata.Comments != "0" || result.FormattedMetadata.Likes != "Unavailable" {
		t.Fatalf("hidden/zero counts not distinguished: %+v", result.FormattedMetadata)
	}
	if result.RevenueEstimate.Source != "public_estimate" || result.RevenueEstimate.ActualAnalyticsUnavailable != true {
		t.Fatalf("bad public revenue source: %+v", result.RevenueEstimate)
	}
}

func TestAnalyzeChannelReturnsSpecificPillarsAndIdeas(t *testing.T) {
	provider := NewYouTubeProvider("yt-key", &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/youtube/v3/channels":
			return jsonResponse(`{"items":[{"id":"UCmkbhd","snippet":{"title":"MKBHD","description":"Quality tech reviews about phones, cameras, electric cars, and software.","country":"US","publishedAt":"2008-03-25T00:00:00Z"},"statistics":{"viewCount":"5000000000","subscriberCount":"20000000","videoCount":"1800"},"topicDetails":{"topicCategories":["https://en.wikipedia.org/wiki/Technology"]}}]}`), nil
		case "/youtube/v3/search":
			return jsonResponse(`{"items":[{"id":{"videoId":"v1"},"snippet":{"title":"iPhone 20 Review: What Actually Changed"}},{"id":{"videoId":"v2"},"snippet":{"title":"The Best Android Phone Camera Test"}},{"id":{"videoId":"v3"},"snippet":{"title":"Tesla Robotaxi Tech Explained"}}]}`), nil
		case "/youtube/v3/videos":
			return jsonResponse(`{"items":[{"id":"v1","snippet":{"publishedAt":"2026-07-01T00:00:00Z","title":"iPhone 20 Review: What Actually Changed"},"statistics":{"viewCount":"9000000","likeCount":"300000","commentCount":"12000"}},{"id":"v2","snippet":{"publishedAt":"2026-06-01T00:00:00Z","title":"The Best Android Phone Camera Test"},"statistics":{"viewCount":"7000000","likeCount":"220000","commentCount":"9000"}},{"id":"v3","snippet":{"publishedAt":"2026-05-01T00:00:00Z","title":"Tesla Robotaxi Tech Explained"},"statistics":{"viewCount":"5000000","likeCount":"180000","commentCount":"8000"}}]}`), nil
		default:
			t.Fatalf("unexpected URL: %s", req.URL.String())
			return nil, nil
		}
	})})
	provider.now = func() time.Time { return mustTime("2026-07-10T00:00:00Z") }

	result, err := provider.AnalyzeChannel(context.Background(), "@mkbhd")
	if err != nil {
		t.Fatalf("AnalyzeChannel: %v", err)
	}
	if result.Status != StatusOK {
		t.Fatalf("status = %q", result.Status)
	}
	if len(result.ContentPillars) == 0 || len(result.SuggestedContentIdeas) != 10 || !strings.Contains(strings.ToLower(strings.Join(result.SuggestedContentIdeas, " ")), "phone") {
		t.Fatalf("channel intelligence not specific enough: pillars=%+v ideas=%+v", result.ContentPillars, result.SuggestedContentIdeas)
	}
	assertNoExactRankingClaim(t, result.Limitations)
}

func TestOpenAIEnhancementFallbackWorks(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "bad-key")
	provider := NewYouTubeProvider("yt-key", &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})})
	kw := KeywordIntelligence{PrimaryKeywords: []string{"ai video automation"}, MetadataStrengthScore: 80}
	got := provider.enhanceKeywordsWithOpenAI(context.Background(), kw, map[string]any{"analysis_type": "test"})
	if got.PrimaryKeywords[0] != "ai video automation" {
		t.Fatalf("fallback changed keywords: %+v", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}}
}

func mustTime(raw string) time.Time {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		panic(err)
	}
	return t
}

func assertNoExactRankingClaim(t *testing.T, limitations []string) {
	t.Helper()
	joined := strings.ToLower(strings.Join(limitations, " "))
	if strings.Contains(joined, "exact search ranking keywords") && !strings.Contains(joined, "no exact search ranking") && !strings.Contains(joined, "not an exact") {
		t.Fatalf("limitations may imply exact ranking keywords: %q", joined)
	}
}
