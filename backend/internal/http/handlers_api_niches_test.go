package http

import (
	"testing"
	"time"

	"trendcortex/api/internal/config"
	"trendcortex/api/internal/research"
)

func TestNicheCacheSuccessfulFallbackAgeLimitAndFailureIsolation(t *testing.T) {
	srv := NewServer(&config.Config{}, nil, nil, nil)
	cacheKey := "same-profile-key"
	now := time.Now().UTC()
	report := successfulTestNicheReport("ok-report")
	srv.nicheReports[cacheKey] = nicheReportCacheItem{report: report, storedAt: now.Add(-23 * time.Hour), cacheKey: cacheKey}

	cached, ok := srv.cachedSuccessfulNicheReport(cacheKey, 24*time.Hour)
	if !ok {
		t.Fatalf("expected successful stale cache inside max age")
	}
	cached = markNicheCache(cached, cacheKey, cached.Cache.StoredAt, 24*time.Hour, "stale")
	if !cached.Cache.CacheHit || cached.Cache.Freshness != "stale" || cached.Cache.EvidenceFetchedAt.IsZero() || cached.Cache.EvidenceAge == "" {
		t.Fatalf("cache metadata not marked stale: %+v", cached.Cache)
	}

	srv.nicheReports[cacheKey] = nicheReportCacheItem{report: report, storedAt: now.Add(-25 * time.Hour), cacheKey: cacheKey}
	if _, ok := srv.cachedSuccessfulNicheReport(cacheKey, 24*time.Hour); ok {
		t.Fatalf("expired stale cache should not be used")
	}

	failed := report
	failed.Status = research.NicheStatusQuotaTemporarilyUnavailable
	failed.Candidates = nil
	srv.nicheReports[cacheKey] = nicheReportCacheItem{report: failed, storedAt: now.Add(-time.Hour), cacheKey: cacheKey}
	if _, ok := srv.cachedSuccessfulNicheReport(cacheKey, 24*time.Hour); ok {
		t.Fatalf("cached failure should not be treated as successful research")
	}
}

func TestNicheCacheKeyIsolation(t *testing.T) {
	srv := NewServer(&config.Config{}, nil, nil, nil)
	srv.nicheReports["profile-a"] = nicheReportCacheItem{report: successfulTestNicheReport("a"), storedAt: time.Now().UTC(), cacheKey: "profile-a"}
	if _, ok := srv.cachedSuccessfulNicheReport("profile-b", 24*time.Hour); ok {
		t.Fatalf("cache lookup should not use another profile key")
	}
}

func successfulTestNicheReport(id string) research.NicheReport {
	return research.NicheReport{
		ID:      id,
		Status:  research.StatusOK,
		Message: "ok",
		Candidates: []research.NicheCandidate{{
			ID: "candidate-1",
		}},
	}
}
