package trendintel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBuildYouTubeQueryIncludeExcludeExact(t *testing.T) {
	q := Query{Keyword: "ai video", Include: []string{"tutorial", "shorts"}, Exclude: []string{"news"}, ExactPhrase: "ai tools", Category: "technology"}
	got := BuildYouTubeQuery(q, q.Keyword)
	for _, want := range []string{`"ai tools"`, "tutorial", "shorts", "tech", "-news"} {
		if !strings.Contains(got, want) {
			t.Fatalf("query %q missing %q", got, want)
		}
	}
}

func TestPublishedAfterWindow(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	if got := PublishedAfter("24h", now); !got.Equal(now.Add(-24 * time.Hour)) {
		t.Fatalf("PublishedAfter = %s", got)
	}
}

func TestNormalizeKeywordDedupesVariants(t *testing.T) {
	a := NormalizeKeyword("Best A.I. tools")
	b := NormalizeKeyword("best ai tool")
	if a != b {
		t.Fatalf("normalized variants differ: %q != %q", a, b)
	}
}

func TestUKPostcodeResolver(t *testing.T) {
	got, err := UKPostcodeResolver{}.Resolve(context.Background(), "sw1a 1aa")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "resolved" || got.Country != "GB" || got.Latitude == nil || got.Longitude == nil {
		t.Fatalf("unexpected resolution: %+v", got)
	}
	invalid, _ := UKPostcodeResolver{}.Resolve(context.Background(), "Kabul")
	if invalid.Status != "location_resolution_not_configured" {
		t.Fatalf("invalid status = %q", invalid.Status)
	}
}

func TestYouTubeSearchForwardsCountryLanguageAndWindow(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/search"):
			if r.URL.Query().Get("regionCode") != "GB" {
				t.Fatalf("regionCode = %q", r.URL.Query().Get("regionCode"))
			}
			if r.URL.Query().Get("relevanceLanguage") != "en" {
				t.Fatalf("relevanceLanguage = %q", r.URL.Query().Get("relevanceLanguage"))
			}
			if r.URL.Query().Get("publishedAfter") == "" {
				t.Fatalf("publishedAfter missing")
			}
			return jsonResponse(r, `{"items":[{"id":{"videoId":"v1"}}]}`), nil
		case strings.HasSuffix(r.URL.Path, "/videos"):
			if r.URL.Query().Get("id") != "v1" {
				t.Fatalf("batch id = %q", r.URL.Query().Get("id"))
			}
			return jsonResponse(r, `{"items":[{"id":"v1","snippet":{"publishedAt":"2026-07-11T10:00:00Z","title":"AI tools tutorial","description":"AI tools","channelId":"c1","channelTitle":"Creator","tags":["ai tools"],"thumbnails":{"high":{"url":"https://img.example/v1.jpg"}}},"statistics":{"viewCount":"1000","likeCount":"100","commentCount":"10"},"contentDetails":{"duration":"PT1M"}}]}`), nil
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		return nil, nil
	})
	client := NewYouTubeClient("test-key", &http.Client{Transport: transport})
	q, err := normalizeQuery(Query{Keyword: "ai tools", Country: "GB", Language: "en", Window: "24h"}, "GB")
	if err != nil {
		t.Fatal(err)
	}
	videos, _, err := client.SearchVideos(context.Background(), q, q.Keyword, time.Now().Add(-24*time.Hour), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(videos) != 1 || videos[0].Views == nil || *videos[0].Views != 1000 {
		t.Fatalf("videos = %+v", videos)
	}
}

func TestScoringDeterministicAndPublicResponseOmitsProvenance(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	views := uint64(10000)
	likes := uint64(500)
	videos := []Video{{ID: "v1", Title: "AI tools", PublishedAt: now.Add(-2 * time.Hour), Views: &views, Likes: &likes}}
	seed := seedKeyword{Keyword: "AI tools", DiscoveredAt: now.Add(-time.Hour), GoogleTrendScore: 80}
	q, _ := normalizeQuery(Query{Country: "GB", Language: "en", Window: "24h", Category: "technology"}, "GB")
	a := buildTrend(q, seed, videos, []SourceProvenance{{Provider: "internal"}}, now)
	b := buildTrend(q, seed, videos, []SourceProvenance{{Provider: "internal"}}, now)
	if a.OpportunityScore != b.OpportunityScore || a.MomentumScore != b.MomentumScore {
		t.Fatalf("scoring is not deterministic: %+v %+v", a, b)
	}
	body := mustMarshal(a)
	for _, forbidden := range []string{"provider", "source_provider", "google_trends", "youtube_data_api"} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Fatalf("public trend JSON leaks %q: %s", forbidden, body)
		}
	}
}

func TestDiscoverFallsBackToTrendSeedsWhenYouTubeMissing(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.URL.Host, "trends.example") {
			t.Fatalf("unexpected host %s", r.URL.Host)
		}
		if r.URL.Query().Get("geo") != "GB" {
			t.Fatalf("geo = %q", r.URL.Query().Get("geo"))
		}
		if r.URL.Query().Get("hl") != "en" {
			t.Fatalf("hl = %q", r.URL.Query().Get("hl"))
		}
		return xmlResponse(r, `<rss><channel><item><title>AI tools</title><link>https://trends.example/item</link><pubDate>Sat, 11 Jul 2026 10:00:00 GMT</pubDate><approx_traffic>20K+ searches</approx_traffic></item></channel></rss>`), nil
	})
	svc := NewService(ServiceConfig{
		TrendProvider:  "google_trends_rss",
		TrendBaseURL:   "https://trends.example/trending/rss",
		TrendTimeout:   time.Second,
		DefaultCountry: "GB",
		HTTPClient:     &http.Client{Transport: transport},
	})
	svc.now = func() time.Time { return now }
	got, err := svc.Search(context.Background(), Query{Mode: ModeDiscover, Country: "GB", Language: "en", Window: "24h", Limit: 5, Refresh: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "ok" || len(got.Results) != 1 {
		t.Fatalf("response = %+v", got)
	}
	if got.Results[0].Keyword != "AI tools" || got.Results[0].SupportingContentAvailable {
		t.Fatalf("unexpected result = %+v", got.Results[0])
	}
	body := mustMarshal(got)
	for _, forbidden := range []string{"provider", "source_provider", "google_trends", "youtube_data_api"} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Fatalf("public response leaks %q: %s", forbidden, body)
		}
	}
}

func TestMemoryCache(t *testing.T) {
	cache := NewMemoryCache()
	res := Response{Status: "ok", Results: []Trend{{Keyword: "AI"}}}
	if err := cache.Set(context.Background(), "k", res, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, stored, err := cache.Get(context.Background(), "k", time.Minute)
	if err != nil || got == nil || stored == nil || got.Results[0].Keyword != "AI" {
		t.Fatalf("cache get = %+v %v %v", got, stored, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(r *http.Request, body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/json"}}, Request: r}
}

func xmlResponse(r *http.Request, body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": []string{"application/rss+xml"}}, Request: r}
}

func mustMarshal(v any) string {
	body, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(body)
}
