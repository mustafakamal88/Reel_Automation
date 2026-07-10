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
	assertNoExactRankingClaim(t, result.Limitations)
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
