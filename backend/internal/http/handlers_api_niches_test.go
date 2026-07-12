package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"trendcortex/api/internal/config"
	"trendcortex/api/internal/research"
)

func TestHandleCreateNicheResearchOpenAIUnavailableApplicationState(t *testing.T) {
	srv := NewServer(&config.Config{}, nil, nil, nil)
	body := `{"profile":{"professional_skills":"teach how to code","hobbies":"coding","lived_experiences":"how to build apps","teaching_subjects":"cross-platform apps","target_audience":"vibe coders and students","target_country":"US","target_language":"en","creator_presence":"faceless","content_formats":["long-form"],"optional_broad_topic":"programming","weekly_production_capacity":"2 videos per week"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/niches/research", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	srv.handleCreateNicheResearch(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 application state; body=%s", rec.Code, rec.Body.String())
	}
	var report research.NicheReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v; body=%s", err, rec.Body.String())
	}
	if report.Status != "openai_unavailable" || report.AnalysisMode != "openai_unavailable" {
		t.Fatalf("status=%s analysis_mode=%s, want openai_unavailable", report.Status, report.AnalysisMode)
	}
	if !strings.Contains(strings.ToLower(report.Message), "ai niche strategy") {
		t.Fatalf("message should explain unavailable AI strategy: %q", report.Message)
	}
	if len(report.Candidates) != 0 {
		t.Fatalf("OpenAI unavailable response must not include candidates: %d", len(report.Candidates))
	}
	if report.Profile.ProfessionalSkills != "teach how to code" {
		t.Fatalf("profile inputs not preserved for retry: %#v", report.Profile)
	}
}

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
