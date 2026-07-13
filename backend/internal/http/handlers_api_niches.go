package http

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"trendcortex/api/internal/research"
	trenddiscovery "trendcortex/api/internal/trends"
)

type nicheReportCacheItem struct {
	report   research.NicheReport
	storedAt time.Time
	cacheKey string
}

type nicheTrendAdapter struct {
	discoverer *trenddiscovery.Discoverer
}

func (a nicheTrendAdapter) Discover(ctx context.Context, region, language string, limit int) ([]string, error) {
	if a.discoverer == nil {
		return nil, trenddiscovery.ErrProviderNotConfigured
	}
	res, err := a.discoverer.Discover(ctx, region, language, limit)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, candidate := range res.Candidates {
		if strings.TrimSpace(candidate.Keyword) != "" {
			out = append(out, candidate.Keyword)
		}
	}
	return out, nil
}

func (s *Server) handleCreateNicheResearch(w http.ResponseWriter, r *http.Request) {
	var req research.NicheResearchRequest
	if err := decodeJSONBody(r, &req); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	cacheKey := research.NicheResearchCacheKey(req.Profile, "market_estimate")
	legacyCacheKey := research.LegacyNicheResearchCacheKey(req.Profile, "market_estimate")
	ttl := 6 * time.Hour
	maxStale := 24 * time.Hour
	if !req.Refresh {
		if report, ok := s.cachedNicheReport(cacheKey, ttl); ok {
			report = markNicheCache(report, cacheKey, report.Cache.StoredAt, ttl, "fresh_cache")
			log.Printf("niche_cache_decision=current_version_cache_hit cache_key=%s schema_version=%s", cacheKey, research.NicheReportSchemaVersion)
			jsonOK(w, report)
			return
		}
		if legacyCacheKey != cacheKey {
			if _, ok := s.cachedNicheReport(legacyCacheKey, ttl); ok {
				log.Printf("niche_cache_decision=legacy_cache_rejected legacy_cache_key=%s schema_version=%s", legacyCacheKey, research.NicheReportSchemaVersion)
			}
		}
	}
	log.Printf("niche_cache_decision=fresh_report cache_key=%s schema_version=%s", cacheKey, research.NicheReportSchemaVersion)

	timeout, err := time.ParseDuration(s.cfg.TrendDiscoveryTimeout)
	if err != nil {
		timeout = 10 * time.Second
	}
	cfg := research.NicheResearchConfig{
		YouTube:                      research.NewYouTubeProvider(s.cfg.YouTubeAPIKey, nil),
		Trends:                       nicheTrendAdapter{discoverer: trenddiscovery.NewDiscoverer(trenddiscovery.Config{Provider: s.cfg.TrendDiscoveryProvider, BaseURL: s.cfg.TrendDiscoveryBaseURL, Timeout: timeout}, nil)},
		GoogleAdsStatus:              s.googleAdsKeywordPlannerStatus(),
		OpenAIAPIKey:                 s.cfg.OpenAIAPIKey,
		OpenAIModel:                  s.cfg.OpenAITextModel,
		RequireOpenAI:                true,
		YouTubeCache:                 s.nicheYouTubeCache,
		YouTubeDailyLimiter:          s.nicheYouTubeDaily,
		MaxYouTubeSearchesPerRequest: 3,
		DailyYouTubeSearchLimit:      80,
	}
	report, researchErr := research.ResearchNiches(r.Context(), req, cfg)
	report, _, _ = research.FinalizeNicheReportForResponse(report)
	report.Cache = research.NicheCacheInfo{Hit: false, CacheHit: false, SchemaVersion: research.NicheReportSchemaVersion, Key: cacheKey, StoredAt: time.Now().UTC(), EvidenceFetchedAt: time.Now().UTC(), TTL: ttl.String(), Freshness: "fresh"}
	report.Internal.CacheKey = cacheKey
	if researchErr != nil {
		var typed *research.NicheResearchError
		if errors.As(researchErr, &typed) {
			log.Printf("niche_research_failed code=%s http_status=%d reason=%s cache_key=%s", typed.Code, typed.HTTPStatus, typed.Reason, cacheKey)
			if typed.Temporary {
				if cached, ok := s.cachedSuccessfulNicheReport(cacheKey, maxStale); ok {
					cached = markNicheCache(cached, cacheKey, cached.Cache.StoredAt, maxStale, "stale")
					cached.Message = "Showing the most recent verified evidence. Fresh validation is temporarily unavailable."
					cached.Limitations = append(cached.Limitations, "This report is cached and may not reflect the latest public evidence.")
					log.Printf("niche_cache_decision=current_version_cache_hit_stale cache_key=%s schema_version=%s", cacheKey, research.NicheReportSchemaVersion)
					jsonOK(w, cached)
					return
				}
			}
			report.Status = typed.Code
			report.Message = typed.Message
			jsonOK(w, report)
			return
		}
		log.Printf("niche_research_failed code=%s cache_key=%s", research.NicheStatusResearchFailed, cacheKey)
		report.Status = research.NicheStatusResearchFailed
		report.Message = research.NicheMessageTemporarilyUnavailable
		jsonOK(w, report)
		return
	}
	if report.Status == research.StatusOK {
		s.storeNicheReport(cacheKey, report)
	}
	jsonOK(w, report)
}

func (s *Server) handleGetNicheResearch(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		jsonErrorCode(w, "not_found", "Niche report not found.", http.StatusNotFound)
		return
	}
	s.nicheMu.Lock()
	defer s.nicheMu.Unlock()
	for _, item := range s.nicheReports {
		if item.report.ID == id {
			report := item.report
			report, _, _ = research.FinalizeNicheReportForResponse(report)
			report = markNicheCache(report, item.cacheKey, item.storedAt, 6*time.Hour, "fresh_cache")
			jsonOK(w, report)
			return
		}
	}
	jsonErrorCode(w, "not_found", "Niche report not found. Run a new Niche Finder research request.", http.StatusNotFound)
}

func (s *Server) handleAnalyseNicheGaps(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	s.nicheMu.Lock()
	defer s.nicheMu.Unlock()
	for key, item := range s.nicheReports {
		if item.report.ID != id {
			continue
		}
		report := item.report
		for i := range report.Candidates {
			report.Candidates[i].SupplyGaps = append(report.Candidates[i].SupplyGaps, research.SupplyGap{
				Statement:  "Deeper comment analysis is quota-protected and not run automatically.",
				Evidence:   []string{"Use this action after selecting the strongest candidate; no raw comments or provider payloads are exposed."},
				Confidence: "low",
			})
		}
		item.report = report
		item.storedAt = time.Now().UTC()
		s.nicheReports[key] = item
		report, _, _ = research.FinalizeNicheReportForResponse(report)
		jsonOK(w, report)
		return
	}
	jsonErrorCode(w, "not_found", "Niche report not found. Run a new Niche Finder research request.", http.StatusNotFound)
}

func (s *Server) handleNicheFilters(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]any{
		"countries": []map[string]string{
			{"label": "United Kingdom", "value": "GB"},
			{"label": "United States", "value": "US"},
			{"label": "Pakistan", "value": "PK"},
			{"label": "India", "value": "IN"},
		},
		"languages": []map[string]string{
			{"label": "English", "value": "en"},
			{"label": "Urdu", "value": "ur"},
			{"label": "Hindi", "value": "hi"},
			{"label": "Arabic", "value": "ar"},
		},
		"formats": []map[string]string{
			{"label": "Long-form", "value": "long-form"},
			{"label": "Shorts", "value": "shorts"},
			{"label": "Both", "value": "both"},
		},
	})
}

func (s *Server) cachedNicheReport(cacheKey string, ttl time.Duration) (research.NicheReport, bool) {
	s.nicheMu.Lock()
	defer s.nicheMu.Unlock()
	item, ok := s.nicheReports[cacheKey]
	if !ok || item.report.Status != research.StatusOK || len(item.report.Candidates) == 0 || time.Since(item.storedAt) > ttl {
		return research.NicheReport{}, false
	}
	report := item.report
	if report.SchemaVersion != research.NicheReportSchemaVersion && report.Cache.SchemaVersion != research.NicheReportSchemaVersion {
		log.Printf("niche_cache_decision=legacy_cache_rejected cache_key=%s schema_version=%s", cacheKey, firstNonEmpty(report.SchemaVersion, report.Cache.SchemaVersion, "legacy"))
		return research.NicheReport{}, false
	}
	if research.NicheReportNeedsAlternativeRefresh(report) {
		log.Printf("niche_cache_decision=regeneration_required cache_key=%s reason=alternative_generation_revision", cacheKey)
		return research.NicheReport{}, false
	}
	var okFinal bool
	report, _, okFinal = research.FinalizeNicheReportForResponse(report)
	if !okFinal || report.Status != research.StatusOK || len(report.Candidates) == 0 {
		log.Printf("niche_cache_decision=regeneration_required cache_key=%s reason=final_validation_failed", cacheKey)
		return research.NicheReport{}, false
	}
	report.Cache.StoredAt = item.storedAt
	report.Cache.TTL = ttl.String()
	return report, true
}

func (s *Server) cachedSuccessfulNicheReport(cacheKey string, maxAge time.Duration) (research.NicheReport, bool) {
	s.nicheMu.Lock()
	defer s.nicheMu.Unlock()
	item, ok := s.nicheReports[cacheKey]
	if !ok || item.report.Status != research.StatusOK || len(item.report.Candidates) == 0 || time.Since(item.storedAt) > maxAge {
		return research.NicheReport{}, false
	}
	report := item.report
	if report.SchemaVersion != research.NicheReportSchemaVersion && report.Cache.SchemaVersion != research.NicheReportSchemaVersion {
		log.Printf("niche_cache_decision=legacy_cache_rejected cache_key=%s schema_version=%s", cacheKey, firstNonEmpty(report.SchemaVersion, report.Cache.SchemaVersion, "legacy"))
		return research.NicheReport{}, false
	}
	if research.NicheReportNeedsAlternativeRefresh(report) {
		log.Printf("niche_cache_decision=regeneration_required cache_key=%s reason=alternative_generation_revision", cacheKey)
		return research.NicheReport{}, false
	}
	var okFinal bool
	report, _, okFinal = research.FinalizeNicheReportForResponse(report)
	if !okFinal || report.Status != research.StatusOK || len(report.Candidates) == 0 {
		log.Printf("niche_cache_decision=regeneration_required cache_key=%s reason=final_validation_failed", cacheKey)
		return research.NicheReport{}, false
	}
	report.Cache.StoredAt = item.storedAt
	report.Cache.TTL = maxAge.String()
	return report, true
}

func (s *Server) storeNicheReport(cacheKey string, report research.NicheReport) {
	s.nicheMu.Lock()
	defer s.nicheMu.Unlock()
	report, _, _ = research.FinalizeNicheReportForResponse(report)
	report.Cache.SchemaVersion = research.NicheReportSchemaVersion
	s.nicheReports[cacheKey] = nicheReportCacheItem{report: report, storedAt: time.Now().UTC(), cacheKey: cacheKey}
}

func markNicheCache(report research.NicheReport, cacheKey string, storedAt time.Time, ttl time.Duration, freshness string) research.NicheReport {
	now := time.Now().UTC()
	report.Cache.Hit = true
	report.Cache.CacheHit = true
	report.Cache.SchemaVersion = research.NicheReportSchemaVersion
	report.Cache.Key = cacheKey
	report.Cache.StoredAt = storedAt
	report.Cache.EvidenceFetchedAt = storedAt
	report.Cache.EvidenceAge = humanDuration(now.Sub(storedAt))
	report.Cache.TTL = ttl.String()
	report.Cache.Freshness = freshness
	return report
}

func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return "less than 1 minute"
	}
	if d < time.Hour {
		return strconv.Itoa(int(d.Minutes())) + " minutes"
	}
	if d < 48*time.Hour {
		return strconv.Itoa(int(d.Hours())) + " hours"
	}
	return strconv.Itoa(int(d.Hours()/24)) + " days"
}
