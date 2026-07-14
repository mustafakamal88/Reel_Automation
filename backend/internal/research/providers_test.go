package research

import (
	"context"
	"encoding/json"
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
	if !strings.Contains(strings.ToLower(joined), "ai automation") {
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

func TestVideoTopicReconstructionRecoversSpecificICTTradingConcepts(t *testing.T) {
	got := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title: "Every ICT Concept Explained in 14 Minutes",
		Description: `This lesson covers ICT trading concepts including liquidity, fair value gaps, order blocks, displacement, break of structure, market structure, breakers, mitigation blocks, balanced price ranges, premium and discount.

Disclaimer: Not financial advice. Do your own research. Past performance does not guarantee future results.`,
		Tags:         []string{"ICT trading", "liquidity", "fair value gaps", "order blocks", "market structure", "FVG"},
		Category:     "27",
		TopicDetails: []string{"https://en.wikipedia.org/wiki/Foreign_exchange_market"},
	})
	all := strings.Join(append(append(got.PrimaryKeywords, got.SecondaryKeywords...), got.LongTailPhrases...), " | ")
	lower := strings.ToLower(all)
	for _, want := range []string{"ICT", "liquidity", "fair value gap", "order block", "market structure"} {
		if !strings.Contains(all, want) && !strings.Contains(lower, strings.ToLower(want)) {
			t.Fatalf("expected %q in reconstructed topics: %+v", want, got)
		}
	}
	if len(got.PrimaryKeywords) == 1 && strings.EqualFold(got.PrimaryKeywords[0], "ict concept") {
		t.Fatalf("generic ICT concept should not be the only primary topic: %+v", got)
	}
	if strings.Contains(all, "Ict") || strings.Contains(all, "fvg") {
		t.Fatalf("acronym casing not preserved: %+v", got)
	}
}

func TestKeywordExtractorSuppressesGenericOnlyTopic(t *testing.T) {
	got := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title:       "Concepts Explained for Beginners",
		Description: "This tutorial explains concepts in a beginner education video.",
		Tags:        []string{"concepts", "tutorial", "education", "beginners"},
		Category:    "27",
	})
	all := strings.ToLower(strings.Join(append(append(got.PrimaryKeywords, got.SecondaryKeywords...), got.LongTailPhrases...), " | "))
	for _, generic := range []string{"concept", "tutorial", "education", "beginner"} {
		if all == generic || strings.Contains(all, generic+" |") {
			t.Fatalf("generic phrase survived as topic %q: %+v", generic, got)
		}
	}
}

func TestKeywordExtractorPreventsDuplicateTopicClusterLabels(t *testing.T) {
	got := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title:       "Fair Value Gap Trading Explained",
		Description: "Fair value gaps and FVG trading examples for ICT traders. Fair value gap setups appear with market structure.",
		Tags:        []string{"fair value gap", "fair value gaps", "FVG", "ICT trading", "market structure"},
	})
	seen := map[string]bool{}
	for _, phrase := range append(got.PrimaryKeywords, got.SecondaryKeywords...) {
		key := topicDedupeKey(phrase)
		if seen[key] {
			t.Fatalf("duplicate normalized graph label %q in %+v", key, got)
		}
		seen[key] = true
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

func TestScienceAndProgrammingEducationSpecificTopics(t *testing.T) {
	cases := []struct {
		name string
		in   KeywordExtractionInput
		want []string
	}{
		{
			name: "science",
			in:   KeywordExtractionInput{Title: "Photosynthesis Explained for Beginners", Description: "Chlorophyll, sunlight, carbon dioxide, glucose, oxygen, and plant cells are explained.", Tags: []string{"photosynthesis", "chlorophyll", "plant cells"}, Category: "27"},
			want: []string{"photosynthesis", "chlorophyll", "plant cell"},
		},
		{
			name: "programming",
			in:   KeywordExtractionInput{Title: "Python Decorators Explained", Description: "Learn wrapper functions, closures, decorators, and reusable Python patterns.", Tags: []string{"python decorators", "closures", "wrapper functions", "programming"}, Category: "28"},
			want: []string{"python decorator", "closure", "wrapper function"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractKeywordIntelligence(tc.in)
			joined := strings.ToLower(strings.Join(append(append(got.PrimaryKeywords, got.SecondaryKeywords...), got.LongTailPhrases...), " | "))
			for _, want := range tc.want {
				if !strings.Contains(joined, want) {
					t.Fatalf("missing %q in %+v", want, got)
				}
			}
		})
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

func TestPromotionalDescriptionKeepsUsefulTechnicalContent(t *testing.T) {
	got := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title: "Build an API Rate Limiter in Go",
		Description: `Subscribe and use code SAVE20.
Business inquiries: hello@example.com.
This tutorial covers token buckets, middleware, HTTP handlers, API rate limiting, and Redis counters.`,
		Tags: []string{"API rate limiter", "Go middleware", "token bucket", "Redis counters"},
	})
	joined := strings.Join(append(append(got.PrimaryKeywords, got.SecondaryKeywords...), got.LongTailPhrases...), " | ")
	lower := strings.ToLower(joined)
	for _, want := range []string{"API rate limiter", "token bucket", "middleware", "redis counter"} {
		if !strings.Contains(joined, want) && !strings.Contains(lower, strings.ToLower(want)) {
			t.Fatalf("missing technical topic %q: %+v", want, got)
		}
	}
	for _, noise := range []string{"subscribe", "save20", "business inquiries"} {
		if strings.Contains(lower, noise) {
			t.Fatalf("promotional noise survived %q: %+v", noise, got)
		}
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

func TestSparseMetadataFallbackDoesNotDuplicateCoreTopic(t *testing.T) {
	got := ExtractKeywordIntelligence(KeywordExtractionInput{Title: "ICT Basics"})
	all := append(got.PrimaryKeywords, got.SecondaryKeywords...)
	seen := map[string]bool{}
	for _, phrase := range all {
		key := topicDedupeKey(phrase)
		if seen[key] {
			t.Fatalf("sparse metadata duplicated topic %q: %+v", key, got)
		}
		seen[key] = true
	}
	if len(all) > 2 {
		t.Fatalf("sparse metadata should return few honest terms: %+v", got)
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

func TestCreativeOutputsAreCompleteAndClean(t *testing.T) {
	kw := ExtractKeywordIntelligence(KeywordExtractionInput{
		Title:       "Every ICT Concept Explained in 14 Minutes",
		Description: "ICT trading concepts include liquidity, fair value gaps, order blocks, displacement, and market structure.",
		Tags:        []string{"ICT trading", "liquidity", "fair value gaps", "order blocks", "market structure"},
	})
	niche := ClassifyNiche(KeywordExtractionInput{Title: "Every ICT Concept Explained in 14 Minutes", Description: "ICT trading concepts include liquidity, fair value gaps and order blocks."}, kw)
	opps := BuildCreatorOpportunities(niche, kw, AnalyzeHookIntelligence("Every ICT Concept Explained in 14 Minutes"), "Every ICT Concept Explained in 14 Minutes")
	all := append(append(append(opps.TitleIdeas, opps.ScriptPrompts...), opps.ShortFormClipIdeas...), opps.SuggestedRemakeAngles...)
	if len(opps.TitleIdeas) == 0 {
		t.Fatalf("expected title ideas: %+v", opps)
	}
	for _, value := range all {
		if isTruncatedGeneratedTitle(value) {
			t.Fatalf("truncated creative output: %q", value)
		}
		if strings.Contains(value, ";.") || strings.Contains(value, "..") || strings.Contains(strings.ToLower(value), "cleaned metadata") || strings.Contains(strings.ToLower(value), "evidence basis") {
			t.Fatalf("unclean creative output: %q", value)
		}
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

func TestChannelAnalyzerLaunchIntelligenceContract(t *testing.T) {
	provider := NewYouTubeProvider("yt-key", &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/youtube/v3/channels":
			return jsonResponse(`{"items":[{"id":"UCcreator","snippet":{"title":"Creator Systems Lab","customUrl":"@creatorsystems","description":"Practical tutorials for AI video workflows, creator automation, and YouTube systems.\nDisclaimer: affiliate links may be present.","country":"GB","publishedAt":"2020-01-01T00:00:00Z","thumbnails":{"high":{"url":"https://img.example/avatar.jpg"}}},"statistics":{"viewCount":"12000000","subscriberCount":"85000","videoCount":"220"},"topicDetails":{"topicCategories":["https://en.wikipedia.org/wiki/Artificial_intelligence"]},"brandingSettings":{"image":{"bannerExternalUrl":"https://img.example/banner.jpg"}}}]}`), nil
		case "/youtube/v3/search":
			return jsonResponse(`{"items":[{"id":{"videoId":"v1"},"snippet":{"title":"AI Video Workflow for Solo Creators"}},{"id":{"videoId":"v2"},"snippet":{"title":"Creator Automation Setup Guide"}},{"id":{"videoId":"v3"},"snippet":{"title":"YouTube Systems That Save 10 Hours"}},{"id":{"videoId":"v4"},"snippet":{"title":"AI Video Workflow Mistakes"}}]}`), nil
		case "/youtube/v3/videos":
			return jsonResponse(`{"items":[
				{"id":"v1","snippet":{"publishedAt":"2026-07-01T00:00:00Z","title":"AI Video Workflow for Solo Creators","description":"A practical AI video workflow tutorial.","thumbnails":{"high":{"url":"https://img.example/v1.jpg"}}},"statistics":{"viewCount":"180000","likeCount":"7200","commentCount":"360"},"contentDetails":{"duration":"PT9M"}},
				{"id":"v2","snippet":{"publishedAt":"2026-06-20T00:00:00Z","title":"Creator Automation Setup Guide","description":"Creator automation systems explained.","thumbnails":{"high":{"url":"https://img.example/v2.jpg"}}},"statistics":{"viewCount":"120000","commentCount":"210"},"contentDetails":{"duration":"PT11M"}},
				{"id":"v3","snippet":{"publishedAt":"2026-06-10T00:00:00Z","title":"YouTube Systems That Save 10 Hours","description":"YouTube systems for planning and production.","thumbnails":{"high":{"url":"https://img.example/v3.jpg"}}},"statistics":{"viewCount":"90000","likeCount":"3000"},"contentDetails":{"duration":"PT55S"}},
				{"id":"v4","snippet":{"publishedAt":"2026-05-01T00:00:00Z","title":"AI Video Workflow Mistakes","description":"Avoid common workflow mistakes.","thumbnails":{"high":{"url":"https://img.example/v4.jpg"}}},"statistics":{"viewCount":"60000","likeCount":"1800","commentCount":"90"},"contentDetails":{"duration":"PT7M"}}
			]}`), nil
		default:
			t.Fatalf("unexpected URL: %s", req.URL.String())
			return nil, nil
		}
	})})
	provider.now = func() time.Time { return mustTime("2026-07-10T00:00:00Z") }

	result, err := provider.AnalyzeChannel(context.Background(), "https://www.youtube.com/@creatorsystems")
	if err != nil {
		t.Fatalf("AnalyzeChannel: %v", err)
	}
	if result.SchemaVersion != channelAnalysisSchemaVersion || result.Cache.SchemaVersion != channelAnalysisSchemaVersion {
		t.Fatalf("schema/cache version missing: result=%q cache=%q", result.SchemaVersion, result.Cache.SchemaVersion)
	}
	if result.CanonicalChannelURL != "https://www.youtube.com/channel/UCcreator" || result.ChannelHandle != "@creatorsystems" {
		t.Fatalf("identity not normalized: url=%q handle=%q", result.CanonicalChannelURL, result.ChannelHandle)
	}
	if result.Description == "" || strings.Contains(strings.ToLower(result.Description), "affiliate links") {
		t.Fatalf("description was not sanitized: %q", result.Description)
	}
	if len(result.ScoreDimensions) < 6 || result.OpportunityScore.Score <= 0 || result.AnalysisConfidence.Score <= 0 {
		t.Fatalf("missing score model: score=%+v confidence=%+v dims=%+v", result.OpportunityScore, result.AnalysisConfidence, result.ScoreDimensions)
	}
	if len(result.PerformanceCharts.UploadPerformance) == 0 || len(result.PerformanceCharts.UploadCadence) == 0 || len(result.PerformanceCharts.TopicPerformance) == 0 {
		t.Fatalf("missing real chart data: %+v", result.PerformanceCharts)
	}
	if len(result.ChannelPillars) == 0 || strings.Contains(strings.ToLower(result.ChannelPillars[0].Name), "general viewers") {
		t.Fatalf("bad pillars: %+v", result.ChannelPillars)
	}
	if result.RevenueEstimate.Source != "public_estimate" || !result.RevenueEstimate.ActualAnalyticsUnavailable || strings.Contains(strings.ToLower(result.RevenueEstimate.CalculationBasis), "actual earnings") {
		t.Fatalf("bad revenue estimate: %+v", result.RevenueEstimate)
	}
	if len(result.TopVideoGroups) == 0 || len(result.GrowthOpportunities) == 0 || len(result.ContentPlan) != 4 {
		t.Fatalf("missing strategy sections: groups=%+v opps=%+v plan=%+v", result.TopVideoGroups, result.GrowthOpportunities, result.ContentPlan)
	}
	blob, _ := json.Marshal(result.CtaContext)
	lower := strings.ToLower(string(blob))
	for _, rejected := range []string{"general viewers in this niche", "hypothetical", "api key", "prompt"} {
		if strings.Contains(lower, rejected) {
			t.Fatalf("CTA context leaked rejected text %q: %s", rejected, lower)
		}
	}
}

func TestChannelAnalyzerRejectsPlainSearchTerms(t *testing.T) {
	provider := NewYouTubeProvider("yt-key", &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("provider should not be called for unsupported plain search terms: %s", req.URL.String())
		return nil, nil
	})})
	result, err := provider.AnalyzeChannel(context.Background(), "MKBHD")
	if err != nil {
		t.Fatalf("AnalyzeChannel should return structured invalid result, got error: %v", err)
	}
	if result.Status != "invalid_input" || !strings.Contains(strings.ToLower(result.Message), "plain search terms") {
		t.Fatalf("expected invalid plain search term: %+v", result)
	}
}

func TestChannelScoreEvidenceCapForTinySamples(t *testing.T) {
	now := func() time.Time { return mustTime("2026-07-10T00:00:00Z") }
	video := ChannelVideoSummary{VideoID: "v1", Title: "AI Workflow", PublishedAt: "2026-07-09T00:00:00Z", Duration: "PT9M", Views: uint64Ptr(1000000)}
	videos := enrichChannelVideos([]ChannelVideoSummary{video}, now)
	channel := youtubeChannelItem{}
	channel.Statistics.ViewCount = "1000000"
	dims, score, confidence := buildChannelScoreModel(channel, videos, videos, []ChannelContentPillar{{Name: "AI workflow", UploadCount: 1}}, KeywordIntelligence{PrimaryKeywords: []string{"ai workflow"}, MetadataStrengthScore: 80}, NicheAnalysis{PrimaryNiche: "AI tools", Confidence: 0.9}, now)
	if len(dims) == 0 || score.Score > 55 || confidence.Score >= 70 {
		t.Fatalf("small sample should cap score/confidence: score=%+v confidence=%+v dims=%+v", score, confidence, dims)
	}
}

func TestChannelPillarsExcludeCreatorIdentityAndRequireTitleEvidence(t *testing.T) {
	now := func() time.Time { return mustTime("2026-07-10T00:00:00Z") }
	videos := enrichChannelVideos([]ChannelVideoSummary{
		channelTestVideo("v1", "iPhone 20 Review: What Actually Changed", "Marques Brownlee reviews a phone.", "2026-07-01T00:00:00Z", 9000000),
		channelTestVideo("v2", "Android Phone Camera Test", "Consumer electronics from a YouTuber geek tech head internet personality.", "2026-06-15T00:00:00Z", 7000000),
		channelTestVideo("v3", "Folding Phone Review After One Month", "Marques Brownlee hosts this video.", "2026-06-01T00:00:00Z", 6000000),
	}, now)
	identity := buildChannelIdentityContext("MKBHD", "@mkbhd", "Marques Brownlee is a YouTuber, geek, tech head and internet personality.")
	kw := KeywordIntelligence{PrimaryKeywords: []string{"marque brownlee", "consumer electronic", "tuber geek consumer electronic", "phone review", "smartphone review"}}
	pillars := buildChannelPillars(kw, videos, identity)
	joined := strings.ToLower(strings.Join(topPillarNames(pillars, 10), " "))
	for _, forbidden := range []string{"marque brownlee", "tuber geek", "internet personality", "consumer electronic"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("identity/biography phrase leaked into pillars %q: %+v", forbidden, pillars)
		}
	}
	if len(pillars) == 0 {
		t.Fatalf("expected evidence-backed phone pillar")
	}
	for _, pillar := range pillars {
		if pillar.ShareOfUploads <= 0 || pillar.UploadCount == 0 || pillar.MedianViews == nil {
			t.Fatalf("pillar lacks real support: %+v", pillar)
		}
	}
	if normalizeChannelPillarName("consumer electronic") != "consumer electronics" {
		t.Fatalf("consumer electronics normalization failed")
	}
	if validChannelTopicPhrase("electronic tech head internet", tokenSet(tokenizeUseful("phone review camera test", nil)), identity) {
		t.Fatalf("noun-pile/biography phrase was accepted")
	}
	if validChannelTopicPhrase("biggest ever", tokenSet(tokenizeUseful("biggest ever phone review", nil)), identity) {
		t.Fatalf("generic superlative phrase was accepted")
	}
}

func TestChannelDownstreamSanitizesMalformedIdentityRecommendations(t *testing.T) {
	identity := buildChannelIdentityContext("MKBHD", "@mkbhd", "Marques Brownlee is a YouTuber and internet personality.")
	videos := []ChannelVideoSummary{channelTestVideo("v1", "iPhone Camera Review", "", "2026-07-01T00:00:00Z", 1000000)}
	pillars := []ChannelContentPillar{{Name: "phone reviews", UploadCount: 2, ShareOfUploads: 1}}
	opps := sanitizeChannelGrowthOpportunities([]ChannelOpportunity{
		{Title: "Bad", Why: "Expand the proven pillar: marque brownlee", SampleTitle: "How to marque brownlee", NextAction: "The next step after Marque Brownlee", RecommendedFormat: "Long-form"},
		{Title: "Duplicate", Why: "Expand the proven pillar: marque brownlee", SampleTitle: "Marque Brownlee for beginners", NextAction: "The next step after Marque Brownlee", RecommendedFormat: "Long-form"},
	}, pillars, videos, identity)
	blob, _ := json.Marshal(opps)
	lower := strings.ToLower(string(blob))
	for _, forbidden := range []string{"how to marque brownlee", "marque brownlee for beginners", "the next step after marque brownlee"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("malformed recommendation leaked: %q in %s", forbidden, lower)
		}
	}
}

func TestChannelCadenceAndPlanVolumeUseSampledWindow(t *testing.T) {
	now := func() time.Time { return mustTime("2026-07-10T00:00:00Z") }
	videos := []ChannelVideoSummary{
		channelTestVideo("v1", "Phone Review", "", "2026-07-01T00:00:00Z", 100),
		channelTestVideo("v2", "Camera Review", "", "2026-06-01T00:00:00Z", 100),
		channelTestVideo("v3", "Laptop Review", "", "2026-05-01T00:00:00Z", 100),
	}
	summary := sampledCadenceSummary(videos, now)
	if summary.UploadsPerMonth < 0.9 || summary.UploadsPerMonth > 1.1 || summary.MedianIntervalDays < 30 || summary.MedianIntervalDays > 31 || summary.Confidence != "Low" {
		t.Fatalf("bad cadence summary: %+v", summary)
	}
	shortWindow := []ChannelVideoSummary{
		channelTestVideo("v1", "Phone Review", "", "2026-07-01T00:00:00Z", 100),
		channelTestVideo("v2", "Camera Review", "", "2026-07-03T00:00:00Z", 100),
	}
	shortSummary := sampledCadenceSummary(shortWindow, now)
	if shortSummary.UploadsPerMonth < 15 || shortSummary.UploadsPerMonth > 16 {
		t.Fatalf("short sampled window should use median interval, got %+v", shortSummary)
	}
	plan := buildChannelContentPlan(videos, videos, []ChannelContentPillar{{Name: "phone reviews", UploadCount: 2, ShareOfUploads: 0.67}}, nil, NicheAnalysis{}, now)
	uploads := 0
	for _, week := range plan {
		for _, idea := range week.Ideas {
			if idea.Format != "Preparation" {
				uploads++
			}
		}
	}
	if uploads != 2 {
		t.Fatalf("plan upload volume = %d, want 2 from monthly cadence: %+v", uploads, plan)
	}
	for _, week := range plan {
		if week.WeekType != "Publishing week" {
			for _, idea := range week.Ideas {
				if idea.Format != "Preparation" {
					t.Fatalf("preparation week counted as upload: %+v", week)
				}
			}
		}
	}
}

func TestChannelPackagingDistinguishesModelNumbersFromNumberLedTitles(t *testing.T) {
	modelTitles := []ChannelVideoSummary{
		channelTestVideo("v1", "iPhone 17 Review", "", "2026-07-01T00:00:00Z", 100),
		channelTestVideo("v2", "M4 MacBook Air Review", "", "2026-07-02T00:00:00Z", 100),
		channelTestVideo("v3", "Nothing Phone 4b Camera Test", "", "2026-07-03T00:00:00Z", 100),
	}
	pkg := buildChannelPackaging(modelTitles, nil)
	if pkg.NumberTitleShare != 0 || pkg.StrongestPattern == "Number-led titles" {
		t.Fatalf("model numbers counted as number-led titles: %+v", pkg)
	}
	listTitles := []ChannelVideoSummary{
		channelTestVideo("v1", "5 Features You Need", "", "2026-07-01T00:00:00Z", 100),
		channelTestVideo("v2", "10 Mistakes to Avoid", "", "2026-07-02T00:00:00Z", 100),
		channelTestVideo("v3", "3 Reasons This Works", "", "2026-07-03T00:00:00Z", 100),
	}
	pkg = buildChannelPackaging(listTitles, nil)
	if pkg.NumberTitleShare != 1 || pkg.StrongestPattern != "Number-led titles" {
		t.Fatalf("true numbered-list titles not detected: %+v", pkg)
	}
}

func TestChannelChartsRevenueAndDetailsAreCautious(t *testing.T) {
	now := func() time.Time { return mustTime("2026-07-10T00:00:00Z") }
	videos := enrichChannelVideos([]ChannelVideoSummary{
		channelTestVideo("v1", "Phone Review", "", "2026-07-01T00:00:00Z", 100000),
		channelTestVideo("v2", "Phone Camera Test", "", "2026-06-01T00:00:00Z", 200000),
	}, now)
	pillars := []ChannelContentPillar{{Name: "phone reviews", UploadCount: 2, ShareOfUploads: 1, MedianViews: float64Ptr(150000)}}
	charts := buildChannelCharts(videos, videos, pillars)
	if charts.FormatPerformance[0].Label != "Long-form" || charts.FormatPerformance[0].Description == "" {
		t.Fatalf("bad format chart label: %+v", charts.FormatPerformance)
	}
	channel := youtubeChannelItem{}
	channel.Statistics.ViewCount = "5000000000"
	revenue := estimateChannelRevenue(videos, NicheAnalysis{PrimaryNiche: "technology"})
	if revenue.ModelType != "sampled_video_views_x_estimated_rpm_range" || !strings.Contains(strings.ToLower(revenue.CalculationBasis), "sampled-video advertising proxy") {
		t.Fatalf("revenue methodology is not cautious: %+v", revenue)
	}
	details := buildChannelAnalysisDetails(videos, videos, channel, now)
	if details.UploadsPerMonth <= 0 || details.CadenceConfidence == "" {
		t.Fatalf("missing cadence details: %+v", details)
	}
}

func TestChannelRecentCadenceExcludesHistoricalTopVideos(t *testing.T) {
	now := func() time.Time { return mustTime("2026-07-14T00:00:00Z") }
	recent := enrichChannelVideos([]ChannelVideoSummary{
		channelTestVideo("r1", "Weekly Phone Review 1", "", "2026-06-15T00:00:00Z", 100000),
		channelTestVideo("r2", "Weekly Phone Review 2", "", "2026-06-22T00:00:00Z", 120000),
		channelTestVideo("r3", "Weekly Phone Review 3", "", "2026-06-29T00:00:00Z", 130000),
		channelTestVideo("r4", "Weekly Phone Review 4", "", "2026-07-06T00:00:00Z", 140000),
		channelTestVideo("r5", "Weekly Phone Review 5", "", "2026-07-13T00:00:00Z", 150000),
	}, now)
	performance := mergeChannelVideos(recent, enrichChannelVideos([]ChannelVideoSummary{
		channelTestVideo("oldtop", "Old Viral Phone Review", "", "2018-03-09T00:00:00Z", 90000000),
	}, now))
	recentSummary := sampledCadenceSummary(recent, now)
	if recentSummary.UploadsPerMonth < 4.0 || recentSummary.UploadsPerMonth > 4.6 {
		t.Fatalf("recent cadence = %+v, want weekly cadence around 4.3/month", recentSummary)
	}
	metrics := channelPerformanceMetrics(recent, performance, "", now)
	got := metrics[0]
	for _, metric := range metrics {
		if metric.ID == "uploads_per_month" {
			got = metric
			break
		}
	}
	if got.RawValue < 4.0 || got.RawValue > 4.6 {
		t.Fatalf("uploads/month used historical top video: %+v", got)
	}
	charts := buildChannelCharts(recent, performance, nil)
	for _, point := range charts.UploadCadence {
		if strings.Contains(point.Label, "2018") {
			t.Fatalf("cadence chart included historical top video: %+v", charts.UploadCadence)
		}
	}
}

func TestAnalyzeChannelUsesUploadsPlaylistForRecentSample(t *testing.T) {
	provider := NewYouTubeProvider("yt-key", &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/youtube/v3/channels":
			return jsonResponse(`{"items":[{"id":"UCuploads","snippet":{"title":"Uploads Source","customUrl":"@uploads","description":"Weekly public uploads.","publishedAt":"2020-01-01T00:00:00Z"},"statistics":{"viewCount":"1000000","subscriberCount":"10000","videoCount":"50"},"contentDetails":{"relatedPlaylists":{"uploads":"UUuploads"}}}]}`), nil
		case "/youtube/v3/playlistItems":
			return jsonResponse(`{"items":[
				{"snippet":{"publishedAt":"2026-06-01T00:00:00Z","title":"Recent Upload 1","resourceId":{"videoId":"r1"}}},
				{"snippet":{"publishedAt":"2026-06-08T00:00:00Z","title":"Recent Upload 2","resourceId":{"videoId":"r2"}}},
				{"snippet":{"publishedAt":"2026-06-15T00:00:00Z","title":"Recent Upload 3","resourceId":{"videoId":"r3"}}},
				{"snippet":{"publishedAt":"2026-06-22T00:00:00Z","title":"Recent Upload 4","resourceId":{"videoId":"r4"}}}
			]}`), nil
		case "/youtube/v3/search":
			if req.URL.Query().Get("order") == "date" {
				t.Fatalf("date search fallback should not be used when uploads playlist returns videos")
			}
			return jsonResponse(`{"items":[{"id":{"videoId":"oldtop"},"snippet":{"title":"Old Top Upload"}}]}`), nil
		case "/youtube/v3/videos":
			return jsonResponse(`{"items":[
				{"id":"r1","snippet":{"publishedAt":"2026-06-01T00:00:00Z","title":"Recent Upload 1"},"statistics":{"viewCount":"100000"},"contentDetails":{"duration":"PT8M"}},
				{"id":"r2","snippet":{"publishedAt":"2026-06-08T00:00:00Z","title":"Recent Upload 2"},"statistics":{"viewCount":"110000"},"contentDetails":{"duration":"PT8M"}},
				{"id":"r3","snippet":{"publishedAt":"2026-06-15T00:00:00Z","title":"Recent Upload 3"},"statistics":{"viewCount":"120000"},"contentDetails":{"duration":"PT8M"}},
				{"id":"r4","snippet":{"publishedAt":"2026-06-22T00:00:00Z","title":"Recent Upload 4"},"statistics":{"viewCount":"130000"},"contentDetails":{"duration":"PT8M"}},
				{"id":"oldtop","snippet":{"publishedAt":"2018-03-09T00:00:00Z","title":"Old Top Upload"},"statistics":{"viewCount":"10000000"},"contentDetails":{"duration":"PT8M"}}
			]}`), nil
		default:
			t.Fatalf("unexpected URL: %s", req.URL.String())
			return nil, nil
		}
	})})
	provider.now = func() time.Time { return mustTime("2026-07-01T00:00:00Z") }
	result, err := provider.AnalyzeChannel(context.Background(), "@uploads")
	if err != nil {
		t.Fatalf("AnalyzeChannel: %v", err)
	}
	if result.AnalysisDetails.RecentSampleCount != 4 || result.AnalysisDetails.PerformanceSampleCount != 5 {
		t.Fatalf("bad sample separation: %+v", result.AnalysisDetails)
	}
	if result.AnalysisDetails.UploadsPerMonth < 4 || result.AnalysisDetails.UploadsPerMonth > 4.6 {
		t.Fatalf("bad uploads-playlist cadence: %+v", result.AnalysisDetails)
	}
	for _, point := range result.PerformanceCharts.UploadCadence {
		if strings.Contains(point.Label, "2018") {
			t.Fatalf("cadence chart included top-video date: %+v", result.PerformanceCharts.UploadCadence)
		}
	}
}

func TestChannelCadenceRequiresTwoRecentDates(t *testing.T) {
	now := func() time.Time { return mustTime("2026-07-14T00:00:00Z") }
	summary := sampledCadenceSummary([]ChannelVideoSummary{
		channelTestVideo("r1", "One Recent Upload", "", "2026-07-13T00:00:00Z", 100000),
	}, now)
	if summary.Available || summary.UploadsPerMonth != 0 || !strings.Contains(strings.ToLower(summary.Methodology), "insufficient recent upload data") {
		t.Fatalf("single upload should be unavailable: %+v", summary)
	}
	metrics := channelPerformanceMetrics([]ChannelVideoSummary{channelTestVideo("r1", "One Recent Upload", "", "2026-07-13T00:00:00Z", 100000)}, nil, "", now)
	if metricValueForTest(metrics, "uploads_per_month") != "Insufficient recent upload data" {
		t.Fatalf("metric should be honest unavailable: %+v", metrics)
	}
}

func TestChannelMonthlyRunRateExcludesHistoricalTopViews(t *testing.T) {
	now := func() time.Time { return mustTime("2026-07-14T00:00:00Z") }
	recent := enrichChannelVideos([]ChannelVideoSummary{
		channelTestVideo("r1", "Weekly Phone Review 1", "", "2026-07-01T00:00:00Z", 100000),
		channelTestVideo("r2", "Weekly Phone Review 2", "", "2026-07-08T00:00:00Z", 100000),
	}, now)
	performance := mergeChannelVideos(recent, []ChannelVideoSummary{channelTestVideo("oldtop", "Old Viral Phone Review", "", "2018-03-09T00:00:00Z", 100000000)})
	sampled := estimateChannelRevenue(performance, NicheAnalysis{PrimaryNiche: "technology"})
	runRate := estimateChannelMonthlyRunRate(recent, NicheAnalysis{PrimaryNiche: "technology"}, now)
	if sampled.Low <= runRate.Low {
		t.Fatalf("sampled lifetime proxy should include old top views separately: sampled=%+v runRate=%+v", sampled, runRate)
	}
	if strings.Contains(runRate.CalculationBasis, "100,200,000") || !strings.Contains(strings.ToLower(runRate.CalculationBasis), "historical top-video views are excluded") {
		t.Fatalf("run-rate basis did not exclude top views: %+v", runRate)
	}
}

func TestChannelIncidentalSubjectsDoNotBecomeDurablePillars(t *testing.T) {
	now := func() time.Time { return mustTime("2026-07-14T00:00:00Z") }
	videos := enrichChannelVideos([]ChannelVideoSummary{
		channelTestVideo("v1", "I Gave Away A Tesla In A Challenge", "", "2026-07-01T00:00:00Z", 10000000),
		channelTestVideo("v2", "Last To Leave The Tesla Wins", "", "2026-06-01T00:00:00Z", 9000000),
		channelTestVideo("v3", "Survive 100 Hours In A Circle Challenge", "", "2026-05-01T00:00:00Z", 8000000),
		channelTestVideo("v4", "I Gave Away $1,000,000", "", "2026-04-01T00:00:00Z", 7000000),
	}, now)
	identity := buildChannelIdentityContext("Challenge Creator", "@challenge", "")
	pillars := buildChannelPillars(KeywordIntelligence{}, videos, identity)
	names := strings.ToLower(strings.Join(topPillarNames(pillars, 10), " "))
	if strings.Contains(names, "electric vehicle") {
		t.Fatalf("incidental object became pillar: %+v", pillars)
	}
	if !strings.Contains(names, "challenge") {
		t.Fatalf("expected durable format pillar to remain: %+v", pillars)
	}
}

func metricValueForTest(metrics []PerformanceMetric, id string) string {
	for _, metric := range metrics {
		if metric.ID == id {
			return metric.Value
		}
	}
	return ""
}

func uint64Ptr(v uint64) *uint64 {
	return &v
}

func float64Ptr(v float64) *float64 {
	return &v
}

func channelTestVideo(id, title, description, published string, views uint64) ChannelVideoSummary {
	return ChannelVideoSummary{VideoID: id, Title: title, Description: description, PublishedAt: published, Duration: "PT8M", Views: uint64Ptr(views)}
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

func TestVideoAnalyzerRemovesTradingDisclaimersFromTopics(t *testing.T) {
	in := KeywordExtractionInput{
		Title:       "Missed Entry? Navigate the Same Trade Idea",
		Description: "We review a missed trade entry and how to manage the same trade idea.\n\nDisclaimer: Hypothetical or simulated performance results have limitations unlike actual trading. No representation is being made that any account will achieve profits. Benefit of hindsight. Not financial advice.",
		Tags:        []string{"ICT trading", "trade entry", "missed setup"},
		Category:    "27",
	}
	kw := ExtractKeywordIntelligence(in)
	joined := strings.ToLower(strings.Join(append(append([]string{}, kw.PrimaryKeywords...), append(kw.SecondaryKeywords, kw.LongTailPhrases...)...), " "))
	for _, polluted := range []string{"hypothetical", "simulated", "hindsight", "representation", "limitation unlike", "actual performance", "missed entry navigate"} {
		if strings.Contains(joined, polluted) {
			t.Fatalf("polluted phrase leaked into topics: %q in %+v", polluted, kw)
		}
	}
	if !strings.Contains(joined, "trade") && !strings.Contains(joined, "ict") {
		t.Fatalf("clean trading topic was not retained: %+v", kw)
	}
}

func TestPhraseQualityRejectsFragmentsGenericAudienceAndRepeats(t *testing.T) {
	evidence := tokenSet(tokenizeUseful("missed trade entries trade entry management ict traders price action", map[string]bool{}))
	title := tokenSet(tokenizeUseful("missed entry same trade idea", map[string]bool{}))
	rejected := []string{
		"missed entry navigate same",
		"benefit hindsight representation being",
		"rule hypothetical simulated performance",
		"general viewers in this niche",
		"trade trade entry",
	}
	for _, phrase := range rejected {
		if ValidCreatorPhrase(phrase, evidence, title, true) {
			t.Fatalf("phrase should be rejected: %q", phrase)
		}
	}
	if !ValidCreatorPhrase("missed trade entries", evidence, title, true) {
		t.Fatalf("clean phrase should be accepted")
	}
}

func TestAnalyzeVideoPollutedMetadataUsesCleanFallbacks(t *testing.T) {
	provider := NewYouTubeProvider("yt-key", &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(`{"items":[{"id":"E7B4vBUEdHw","snippet":{"publishedAt":"2026-07-01T00:00:00Z","channelId":"UCtrade","title":"Missed Entry? Navigate Same Trade Idea","description":"Reviewing a missed trade entry using ICT execution ideas.\nDisclaimer: Hypothetical simulated performance has limitations unlike actual performance. No representation being made. Benefit of hindsight. Not financial advice.","channelTitle":"Trading Lab","tags":["ICT trading","missed trade entry","trade execution"],"categoryId":"27"},"statistics":{"viewCount":"50000","likeCount":"2500","commentCount":"120"},"contentDetails":{"duration":"PT12M"},"topicDetails":{"topicCategories":["https://en.wikipedia.org/wiki/Foreign_exchange_market"]}}]}`), nil
	})})
	provider.now = func() time.Time { return mustTime("2026-07-10T00:00:00Z") }

	result, err := provider.AnalyzeVideo(context.Background(), "https://youtu.be/E7B4vBUEdHw")
	if err != nil {
		t.Fatalf("AnalyzeVideo: %v", err)
	}
	blob, _ := json.Marshal(result)
	lower := strings.ToLower(string(blob))
	for _, polluted := range []string{"hypothetical simulated", "representation being", "benefit hindsight", "limitation unlike actual performance", "general viewers in this niche", "missed entry navigate"} {
		if strings.Contains(lower, polluted) {
			t.Fatalf("polluted fragment leaked into result: %q\n%s", polluted, lower)
		}
	}
	if result.SchemaVersion != "video_analysis_v3_phrase_quality" {
		t.Fatalf("schema version was not bumped: %s", result.SchemaVersion)
	}
	if result.NicheAnalysis.TargetAudience == "" || strings.Contains(strings.ToLower(result.NicheAnalysis.TargetAudience), "general viewers") {
		t.Fatalf("bad target audience: %+v", result.NicheAnalysis)
	}
}

func TestOpenAIContaminationRejected(t *testing.T) {
	got := cleanOpenAIKeywordList([]string{"benefit hindsight representation being", "missed trade entries", "general viewers in this niche"})
	if len(got) != 1 || strings.ToLower(got[0]) != "missed trade entries" {
		t.Fatalf("OpenAI cleanup did not reject contaminated output: %+v", got)
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
