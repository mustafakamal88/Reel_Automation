package trendintel

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	trendrss "trendcortex/api/internal/trends"
)

type Service struct {
	discoverer *trendrss.Discoverer
	youtube    *YouTubeClient
	cache      Cache
	resolver   LocationResolver
	now        func() time.Time
	country    string
}

type ServiceConfig struct {
	TrendProvider  string
	TrendBaseURL   string
	TrendTimeout   time.Duration
	YouTubeAPIKey  string
	DefaultCountry string
	HTTPClient     *http.Client
	Cache          Cache
	Resolver       LocationResolver
}

func NewService(cfg ServiceConfig) *Service {
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	if cfg.Resolver == nil {
		cfg.Resolver = UKPostcodeResolver{}
	}
	if cfg.Cache == nil {
		cfg.Cache = NewMemoryCache()
	}
	return &Service{
		discoverer: trendrss.NewDiscoverer(trendrss.Config{Provider: cfg.TrendProvider, BaseURL: cfg.TrendBaseURL, Timeout: cfg.TrendTimeout}, client),
		youtube:    NewYouTubeClient(cfg.YouTubeAPIKey, client),
		cache:      cfg.Cache,
		resolver:   cfg.Resolver,
		now:        func() time.Time { return time.Now().UTC() },
		country:    normalizeCountry(cfg.DefaultCountry, "GB"),
	}
}

func (s *Service) Search(ctx context.Context, raw Query) (Response, error) {
	q, err := normalizeQuery(raw, s.country)
	if err != nil {
		return Response{Status: "invalid_input", Message: err.Error(), Results: []Trend{}, FetchedAt: s.now()}, nil
	}
	var location *ResolvedLocation
	if q.Location != "" {
		resolved, _ := s.resolver.Resolve(ctx, q.Location)
		location = &resolved
		if resolved.Status == "resolved" && resolved.Country != "" {
			q.Country = resolved.Country
		}
	}
	cacheKey := CacheKey(q, "unified_search")
	ttl := ttlFor(q.Mode)
	if !q.Refresh {
		if cached, stored, err := s.cache.Get(ctx, cacheKey, ttl); err == nil && cached != nil {
			cached.Cache = CacheInfo{Hit: true, StoredAt: stored, TTL: ttl.String()}
			return *cached, nil
		}
	}

	now := s.now()
	res := Response{
		Status:           "ok",
		Mode:             q.Mode,
		Query:            q.Keyword,
		Country:          q.Country,
		CountryName:      supportedCountries[q.Country],
		Language:         q.Language,
		LanguageName:     supportedLanguages[q.Language],
		TimeWindow:       q.Window,
		Category:         q.Category,
		ResolvedLocation: location,
		LocalVideosOnly:  q.LocalVideosOnly,
		LocalRadiusKM:    q.LocalRadiusKM,
		Results:          []Trend{},
		FetchedAt:        now,
		Cache:            CacheInfo{Hit: false, TTL: ttl.String()},
	}

	keywords, rssProvenance, rssErr := s.seedKeywords(ctx, q, now)
	if rssErr != nil && q.Mode == ModeDiscover && errors.Is(rssErr, trendrss.ErrProviderNotConfigured) {
		res.Status = "temporarily_unavailable"
		res.Message = "Trend intelligence is temporarily unavailable."
		return res, nil
	}
	if len(keywords) == 0 && q.Mode == ModeKeyword {
		keywords = []seedKeyword{{Keyword: firstNonEmpty(q.ExactPhrase, q.Keyword), DiscoveredAt: now}}
	}

	for _, seed := range dedupeSeeds(keywords) {
		if seed.Keyword == "" {
			continue
		}
		videos, ytProv, err := s.youtube.SearchVideos(ctx, q, seed.Keyword, PublishedAfter(q.Window, now), location)
		provenance := append([]SourceProvenance{}, rssProvenance...)
		provenance = append(provenance, ytProv)
		if err != nil {
			if errors.Is(err, ErrYouTubeNotConfigured) {
				res.Message = "Video enrichment is not configured; showing current trend discovery only."
			} else {
				res.Message = "Some supporting content is temporarily unavailable."
			}
		}
		trend := buildTrend(q, seed, videos, provenance, now)
		if passesFilters(trend, q) {
			res.Results = append(res.Results, trend)
		}
		if q.Mode == ModeKeyword {
			for _, related := range relatedKeywords(seed.Keyword, videos) {
				relatedSeed := seedKeyword{Keyword: related, Related: []string{seed.Keyword}, DiscoveredAt: now}
				rt := buildTrend(q, relatedSeed, videosMatching(related, videos), provenance, now)
				if passesFilters(rt, q) {
					res.Results = append(res.Results, rt)
				}
			}
		}
	}
	res.Results = dedupeTrends(res.Results)
	sortTrends(res.Results, q.Sort)
	if len(res.Results) > q.Limit {
		res.Results = res.Results[:q.Limit]
	}
	if len(res.Results) == 0 && res.Message == "" {
		if q.LocalVideosOnly {
			res.Message = "No local videos with location metadata were found."
		} else {
			res.Message = "No strong opportunities matched these filters."
		}
	}
	_ = s.cache.Set(ctx, cacheKey, res, ttl)
	return res, nil
}

func (s *Service) ResolveLocation(ctx context.Context, input string) ResolvedLocation {
	resolved, err := s.resolver.Resolve(ctx, input)
	if err != nil {
		return ResolvedLocation{Input: input, Status: "location_resolution_not_configured", Message: "Location resolution is not configured."}
	}
	return resolved
}

type seedKeyword struct {
	Keyword          string
	VolumeText       *string
	PublishedAt      *time.Time
	DiscoveredAt     time.Time
	TrendVelocity    *float64
	Related          []string
	GoogleTrendScore float64
}

func (s *Service) seedKeywords(ctx context.Context, q Query, now time.Time) ([]seedKeyword, []SourceProvenance, error) {
	if q.Mode == ModeKeyword {
		return []seedKeyword{{Keyword: firstNonEmpty(q.ExactPhrase, q.Keyword), DiscoveredAt: now}}, nil, nil
	}
	result, err := s.discoverer.Discover(ctx, q.Country, q.Language, q.Limit)
	prov := []SourceProvenance{{Provider: "google_trends_rss", RequestType: "current_trends", FetchedAt: now}}
	seeds := []seedKeyword{}
	for _, c := range result.Candidates {
		volume := ""
		if c.Evidence != "" && strings.ContainsAny(c.Evidence, "0123456789") {
			volume = c.Evidence
		}
		var volumePtr *string
		if volume != "" {
			volumePtr = &volume
		}
		published := c.DiscoveredAt
		seeds = append(seeds, seedKeyword{
			Keyword:          c.Keyword,
			VolumeText:       volumePtr,
			PublishedAt:      &published,
			DiscoveredAt:     c.DiscoveredAt,
			TrendVelocity:    c.Velocity,
			GoogleTrendScore: c.Score,
		})
	}
	return seeds, prov, err
}

func buildTrend(q Query, seed seedKeyword, videos []Video, provenance []SourceProvenance, now time.Time) Trend {
	count, totalViews, medianViews, avgViews, likes, comments, newest, strongestURL, thumb := videoStats(videos)
	related := append([]string{}, seed.Related...)
	related = append(related, relatedKeywords(seed.Keyword, videos)...)
	momentum, demand, competition, confidence, opportunity, reasons := score(seed, videos, q, now)
	age := ""
	if !seed.DiscoveredAt.IsZero() {
		age = humanAge(now.Sub(seed.DiscoveredAt))
	}
	var agePtr *string
	if age != "" {
		agePtr = &age
	}
	summary := "Current content demand signal based on recent public video activity."
	if seed.VolumeText != nil {
		summary = "Rising keyword with recent public video activity."
	}
	activity := ""
	if totalViews != nil {
		activity = fmt.Sprintf("%d recent videos sampled with %d total public views.", count, *totalViews)
	} else if count > 0 {
		activity = fmt.Sprintf("%d recent videos sampled.", count)
	}
	return Trend{
		ID:                         StableID(q.Country, q.Language, seed.Keyword),
		Keyword:                    seed.Keyword,
		NormalizedKeyword:          NormalizeKeyword(seed.Keyword),
		DisplayTitle:               titleCase(seed.Keyword),
		Summary:                    summary,
		CountryCode:                q.Country,
		LanguageCode:               q.Language,
		Region:                     regionFromLocation(q),
		Category:                   emptyAll(q.Category),
		RelatedKeywords:            topStrings(related, 8),
		SearchVolumeText:           seed.VolumeText,
		PublishedAt:                seed.PublishedAt,
		DiscoveredAt:               seed.DiscoveredAt,
		TrendAge:                   agePtr,
		TrendVelocity:              seed.TrendVelocity,
		VideoCountSampled:          ptrInt(count),
		TotalSampledViews:          totalViews,
		MedianSampledViews:         medianViews,
		AverageSampledViews:        avgViews,
		TotalSampledLikes:          likes,
		TotalSampledComments:       comments,
		NewestRelevantVideoAt:      newest,
		StrongestRelevantVideoURL:  strongestURL,
		StrongestThumbnailURL:      thumb,
		ConfidenceScore:            round(confidence),
		OpportunityScore:           round(opportunity),
		MomentumScore:              round(momentum),
		DemandScore:                round(demand),
		CompetitionScore:           round(competition),
		ScoringReasons:             reasons,
		SampledVideoActivity:       activity,
		SupportingContentAvailable: count > 0,
		FetchedAt:                  now,
		provenance:                 provenance,
	}
}

func score(seed seedKeyword, videos []Video, q Query, now time.Time) (momentum, demand, competition, confidence, opportunity float64, reasons []string) {
	recentCount := 0
	var views uint64
	for _, v := range videos {
		if !v.PublishedAt.IsZero() && now.Sub(v.PublishedAt) <= 48*time.Hour {
			recentCount++
		}
		if v.Views != nil {
			views += *v.Views
		}
	}
	trendSignal := seed.GoogleTrendScore / 100
	if trendSignal == 0 && seed.VolumeText != nil {
		trendSignal = 0.55
	}
	freshness := 0.3
	if seed.PublishedAt != nil {
		freshness = clampFloat(1-(now.Sub(*seed.PublishedAt).Hours()/168), 0, 1)
	}
	videoRecency := clampFloat(float64(recentCount)/10, 0, 1)
	activity := clampFloat(float64(len(videos))/20, 0, 1)
	viewDemand := clampFloat(math.Log10(float64(views)+1)/7, 0, 1)
	momentum = 100 * (0.35*freshness + 0.25*videoRecency + 0.25*trendSignal + 0.15*activity)
	demand = 100 * (0.35*trendSignal + 0.35*viewDemand + 0.15*activity + 0.15*engagement(videos))
	competition = 100 * (0.45*activity + 0.35*medianViewStrength(videos) + 0.20*videoRecency)
	confidence = 100 * clampFloat(0.25+0.35*trendSignal+0.40*clampFloat(float64(len(videos))/10, 0, 1), 0, 1)
	opportunity = (0.36 * demand) + (0.32 * momentum) + (0.22 * (100 - competition)) + (0.10 * confidence)
	switch {
	case momentum >= 70:
		reasons = append(reasons, "Rising rapidly")
	case momentum >= 50:
		reasons = append(reasons, "Strong recent interest")
	}
	if demand >= 65 {
		reasons = append(reasons, "High viewer demand")
	}
	if competition <= 45 && len(videos) > 0 {
		reasons = append(reasons, "Lower content competition")
	}
	if competition >= 75 {
		reasons = append(reasons, "Highly competitive")
	}
	if confidence < 45 {
		reasons = append(reasons, "Limited supporting data")
	}
	if len(videos) == 0 {
		reasons = append(reasons, "Too new to score confidently")
	}
	return
}

func ttlFor(mode SearchMode) time.Duration {
	if mode == ModeDiscover {
		return 20 * time.Minute
	}
	return 45 * time.Minute
}

func passesFilters(t Trend, q Query) bool {
	if q.MinViews != nil && (t.TotalSampledViews == nil || *t.TotalSampledViews < *q.MinViews) {
		return false
	}
	if q.MaxCompetition != nil && t.CompetitionScore > *q.MaxCompetition {
		return false
	}
	if q.MinOpportunityScore != nil && t.OpportunityScore < *q.MinOpportunityScore {
		return false
	}
	return true
}
