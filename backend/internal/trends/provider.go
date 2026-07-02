package trends

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"trendcortex/api/internal/models"
)

const ProviderGoogleTrendsRSS = "google_trends_rss"

var ErrProviderNotConfigured = errors.New("trend discovery provider is not configured")

type ProviderStatus string

const (
	ProviderStatusNotConfigured ProviderStatus = "provider_not_configured"
	ProviderStatusOK            ProviderStatus = "ok"
	ProviderStatusNoData        ProviderStatus = "no_data"
	ProviderStatusError         ProviderStatus = "provider_error"
)

type Config struct {
	Provider string
	BaseURL  string
	Timeout  time.Duration
}

type DiscoverResult struct {
	Provider       string                  `json:"provider"`
	ProviderURL    string                  `json:"provider_url,omitempty"`
	ProviderStatus ProviderStatus          `json:"provider_status"`
	Message        string                  `json:"message,omitempty"`
	Region         string                  `json:"region"`
	Language       string                  `json:"language"`
	Candidates     []models.TrendCandidate `json:"candidates"`
	DiscoveredAt   time.Time               `json:"discovered_at"`
}

type Discoverer struct {
	cfg    Config
	client *http.Client
}

func NewDiscoverer(cfg Config, client *http.Client) *Discoverer {
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &Discoverer{cfg: cfg, client: client}
}

func (d *Discoverer) Discover(ctx context.Context, region, language string, limit int) (DiscoverResult, error) {
	region = normalizeRegion(region)
	language = normalizeLanguage(language)
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	now := time.Now().UTC()
	provider := strings.TrimSpace(strings.ToLower(d.cfg.Provider))
	result := DiscoverResult{
		Provider:       provider,
		ProviderStatus: ProviderStatusNotConfigured,
		Message:        "Set TREND_DISCOVERY_PROVIDER=google_trends_rss on the backend to enable real trend discovery.",
		Region:         region,
		Language:       language,
		Candidates:     []models.TrendCandidate{},
		DiscoveredAt:   now,
	}
	if provider == "" {
		return result, ErrProviderNotConfigured
	}
	if provider != ProviderGoogleTrendsRSS {
		result.ProviderStatus = ProviderStatusNotConfigured
		result.Message = fmt.Sprintf("TREND_DISCOVERY_PROVIDER=%q is not supported by this build.", provider)
		return result, ErrProviderNotConfigured
	}

	endpoint, err := googleTrendsRSSURL(d.cfg.BaseURL, region, language)
	if err != nil {
		result.ProviderStatus = ProviderStatusError
		result.Message = err.Error()
		return result, err
	}
	result.ProviderURL = endpoint

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		result.ProviderStatus = ProviderStatusError
		result.Message = err.Error()
		return result, err
	}
	req.Header.Set("User-Agent", "TrendCortex/phase-4g real trend discovery")
	res, err := d.client.Do(req)
	if err != nil {
		result.ProviderStatus = ProviderStatusError
		result.Message = "trend provider request failed: " + err.Error()
		return result, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		result.ProviderStatus = ProviderStatusError
		result.Message = fmt.Sprintf("trend provider returned HTTP %d", res.StatusCode)
		return result, fmt.Errorf("trend provider returned HTTP %d", res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		result.ProviderStatus = ProviderStatusError
		result.Message = "trend provider response read failed: " + err.Error()
		return result, err
	}
	candidates, err := ParseGoogleTrendsRSS(body, region, language, now)
	if err != nil {
		result.ProviderStatus = ProviderStatusError
		result.Message = "trend provider response parse failed: " + err.Error()
		return result, err
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	result.Candidates = candidates
	if len(candidates) == 0 {
		result.ProviderStatus = ProviderStatusNoData
		result.Message = "The configured provider returned no trend candidates."
		return result, nil
	}
	result.ProviderStatus = ProviderStatusOK
	result.Message = ""
	return result, nil
}

type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title           string `xml:"title"`
	Link            string `xml:"link"`
	PubDate         string `xml:"pubDate"`
	ApproxTraffic   string `xml:"approx_traffic"`
	HTApproxTraffic string `xml:"ht>approx_traffic"`
	NewsItemTitle   string `xml:"news_item>news_item_title"`
	HTNewsItemTitle string `xml:"ht>news_item>news_item_title"`
	NewsItemURL     string `xml:"news_item>news_item_url"`
	HTNewsItemURL   string `xml:"ht>news_item>news_item_url"`
}

func ParseGoogleTrendsRSS(body []byte, region, language string, fallbackTime time.Time) ([]models.TrendCandidate, error) {
	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, err
	}

	candidates := make([]models.TrendCandidate, 0, len(feed.Channel.Items))
	for idx, item := range feed.Channel.Items {
		keyword := strings.TrimSpace(item.Title)
		if keyword == "" {
			continue
		}
		discoveredAt := fallbackTime
		if parsed, err := http.ParseTime(strings.TrimSpace(item.PubDate)); err == nil {
			discoveredAt = parsed.UTC()
		}
		traffic := firstNonEmpty(item.ApproxTraffic, item.HTApproxTraffic)
		score := scoreFromTraffic(traffic, idx)
		velocity := score
		sourceURL := firstNonEmpty(item.Link, item.NewsItemURL, item.HTNewsItemURL)
		evidence := firstNonEmpty(traffic, item.NewsItemTitle, item.HTNewsItemTitle)

		candidates = append(candidates, models.TrendCandidate{
			ID:           candidateID(ProviderGoogleTrendsRSS, region, keyword, discoveredAt),
			Source:       ProviderGoogleTrendsRSS,
			Region:       region,
			Language:     language,
			Keyword:      keyword,
			Title:        keyword,
			Score:        score,
			Velocity:     &velocity,
			DiscoveredAt: discoveredAt,
			SourceURL:    sourceURL,
			Evidence:     evidence,
			Status:       models.TrendCandidateStatusDiscovered,
		})
	}
	return candidates, nil
}

func googleTrendsRSSURL(baseURL, region, language string) (string, error) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://trends.google.com/trending/rss"
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("geo", region)
	q.Set("hl", language)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func normalizeRegion(region string) string {
	region = strings.ToUpper(strings.TrimSpace(region))
	if region == "" {
		return "US"
	}
	if len(region) > 8 {
		return region[:8]
	}
	return region
}

func normalizeLanguage(language string) string {
	language = strings.TrimSpace(language)
	if language == "" {
		return "en-US"
	}
	return language
}

func scoreFromTraffic(traffic string, index int) float64 {
	n := trafficNumber(traffic)
	if n <= 0 {
		score := 100 - float64(index*3)
		if score < 1 {
			return 1
		}
		return score
	}
	score := float64(n) / 1000
	if score > 100 {
		return 100
	}
	if score < 1 {
		return 1
	}
	return score
}

func trafficNumber(s string) int {
	cleaned := strings.NewReplacer(",", "", "+", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(s)))
	cleaned = strings.TrimSuffix(cleaned, "searches")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return 0
	}
	if strings.HasSuffix(cleaned, "k") {
		n, _ := strconv.ParseFloat(strings.TrimSuffix(cleaned, "k"), 64)
		return int(n * 1000)
	}
	if strings.HasSuffix(cleaned, "m") {
		n, _ := strconv.ParseFloat(strings.TrimSuffix(cleaned, "m"), 64)
		return int(n * 1000000)
	}
	n, _ := strconv.Atoi(cleaned)
	return n
}

func candidateID(source, region, keyword string, discoveredAt time.Time) string {
	sum := sha1.Sum([]byte(strings.Join([]string{source, region, strings.ToLower(keyword), discoveredAt.Format("2006-01-02")}, "|")))
	return hex.EncodeToString(sum[:])
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}
