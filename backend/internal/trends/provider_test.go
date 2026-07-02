package trends

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDiscover_NoProviderConfiguredReturnsHonestEmptyResult(t *testing.T) {
	discoverer := NewDiscoverer(Config{}, nil)

	got, err := discoverer.Discover(context.Background(), "US", "en-US", 20)
	if err != ErrProviderNotConfigured {
		t.Fatalf("err = %v, want ErrProviderNotConfigured", err)
	}
	if got.ProviderStatus != ProviderStatusNotConfigured {
		t.Fatalf("ProviderStatus = %q, want %q", got.ProviderStatus, ProviderStatusNotConfigured)
	}
	if len(got.Candidates) != 0 {
		t.Fatalf("Candidates length = %d, want 0", len(got.Candidates))
	}
	if !strings.Contains(got.Message, "TREND_DISCOVERY_PROVIDER=google_trends_rss") {
		t.Fatalf("Message = %q, want provider configuration guidance", got.Message)
	}
}

func TestParseGoogleTrendsRSS(t *testing.T) {
	now := time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)
	body := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss xmlns:ht="https://trends.google.com/trends/trendingsearches/daily">
  <channel>
    <item>
      <title>real fixture topic</title>
      <link>https://trends.google.com/trends/explore?q=real%20fixture%20topic</link>
      <pubDate>Thu, 02 Jul 2026 10:00:00 GMT</pubDate>
      <ht:approx_traffic>50K+</ht:approx_traffic>
      <ht:news_item>
        <ht:news_item_title>Evidence headline from provider</ht:news_item_title>
        <ht:news_item_url>https://example.com/source</ht:news_item_url>
      </ht:news_item>
    </item>
  </channel>
</rss>`)

	got, err := ParseGoogleTrendsRSS(body, "US", "en-US", now)
	if err != nil {
		t.Fatalf("ParseGoogleTrendsRSS: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	c := got[0]
	if c.Source != ProviderGoogleTrendsRSS {
		t.Fatalf("Source = %q, want %q", c.Source, ProviderGoogleTrendsRSS)
	}
	if c.Keyword != "real fixture topic" {
		t.Fatalf("Keyword = %q", c.Keyword)
	}
	if c.Score != 50 {
		t.Fatalf("Score = %v, want 50", c.Score)
	}
	if c.Velocity == nil || *c.Velocity != 50 {
		t.Fatalf("Velocity = %v, want 50", c.Velocity)
	}
	if c.SourceURL == "" {
		t.Fatalf("SourceURL is empty")
	}
	if c.Evidence != "50K+" {
		t.Fatalf("Evidence = %q, want traffic evidence", c.Evidence)
	}
}

func TestDiscover_GoogleTrendsRSSResponseShape(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("geo") != "GB" {
			t.Fatalf("geo = %q, want GB", r.URL.Query().Get("geo"))
		}
		if r.URL.Query().Get("hl") != "en-GB" {
			t.Fatalf("hl = %q, want en-GB", r.URL.Query().Get("hl"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/rss+xml"}},
			Body:       io.NopCloser(strings.NewReader(`<rss><channel><item><title>provider topic</title><link>https://example.com/provider-topic</link><pubDate>Thu, 02 Jul 2026 10:00:00 GMT</pubDate><approx_traffic>100K+</approx_traffic></item></channel></rss>`)),
			Request:    r,
		}, nil
	})}

	discoverer := NewDiscoverer(Config{
		Provider: ProviderGoogleTrendsRSS,
		BaseURL:  "https://provider.example/rss",
		Timeout:  time.Second,
	}, client)

	got, err := discoverer.Discover(context.Background(), "gb", "en-GB", 5)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got.ProviderStatus != ProviderStatusOK {
		t.Fatalf("ProviderStatus = %q, want %q", got.ProviderStatus, ProviderStatusOK)
	}
	if got.Region != "GB" {
		t.Fatalf("Region = %q, want GB", got.Region)
	}
	if len(got.Candidates) != 1 {
		t.Fatalf("Candidates length = %d, want 1", len(got.Candidates))
	}
	if got.Candidates[0].Keyword != "provider topic" {
		t.Fatalf("Keyword = %q", got.Candidates[0].Keyword)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestDiscoverySourceDoesNotContainForbiddenTrendFallbacks(t *testing.T) {
	forbidden := []string{
		"trending " + "now",
		"sample " + "trend",
		"demo " + "trend",
		"mock " + "trend",
		"fake " + "trend",
	}
	files := []string{
		"provider.go",
		"provider_test.go",
	}

	for _, file := range files {
		body, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		lower := strings.ToLower(string(body))
		for _, banned := range forbidden {
			if strings.Contains(lower, banned) {
				t.Fatalf("%s contains forbidden fallback string %q", file, banned)
			}
		}
	}
}
