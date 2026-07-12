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

func TestNicheLegacyCacheVersionRejected(t *testing.T) {
	srv := NewServer(&config.Config{}, nil, nil, nil)
	cacheKey := "legacy-profile-key"
	report := successfulTestNicheReport("legacy-report")
	report.SchemaVersion = ""
	report.Cache.SchemaVersion = ""
	srv.nicheReports[cacheKey] = nicheReportCacheItem{report: report, storedAt: time.Now().UTC(), cacheKey: cacheKey}
	if _, ok := srv.cachedNicheReport(cacheKey, 6*time.Hour); ok {
		t.Fatalf("legacy cache should not be reused directly")
	}
}

func TestNicheCurrentCachePassesFinalResponseValidation(t *testing.T) {
	srv := NewServer(&config.Config{}, nil, nil, nil)
	cacheKey := "current-profile-key"
	report := successfulTestNicheReport("current-report")
	report.Candidates[0].Dimensions.CreatorFit = research.ScoreExplanation{Score: 9, RatingBand: "high", Explanation: "Strong fit."}
	report.Candidates[0].Scores.PersonalFit = report.Candidates[0].Dimensions.CreatorFit
	report.Candidates[0].OverallScore = 44
	srv.nicheReports[cacheKey] = nicheReportCacheItem{report: report, storedAt: time.Now().UTC(), cacheKey: cacheKey}

	cached, ok := srv.cachedNicheReport(cacheKey, 6*time.Hour)
	if !ok {
		t.Fatalf("current cache should be reusable")
	}
	if cached.Candidates[0].Dimensions.CreatorFit.Score != 90 {
		t.Fatalf("cached score not normalized: %#v", cached.Candidates[0].Dimensions.CreatorFit)
	}
	if cached.Candidates[0].OverallScore == 44 {
		t.Fatalf("cached overall was not recalculated")
	}
}

func TestNicheCacheKeyVersioning(t *testing.T) {
	profile := research.CreatorNicheProfile{ProfessionalSkills: "teach code", LivedExperiences: "built apps", TeachingSubjects: "apps", TargetAudience: "students", TargetCountry: "US", TargetLanguage: "en"}
	if research.NicheResearchCacheKey(profile, "market_estimate") == research.LegacyNicheResearchCacheKey(profile, "market_estimate") {
		t.Fatalf("current cache key must differ from legacy key")
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
	dimensions := research.NicheScoreDimensions{
		CreatorFit:             research.ScoreExplanation{Score: 80, RatingBand: "very_high", Explanation: "Strong profile fit."},
		AudienceDemand:         research.ScoreExplanation{Score: 70, RatingBand: "high", Explanation: "Supported demand."},
		CompetitionOpportunity: research.ScoreExplanation{Score: 65, RatingBand: "high", Explanation: "Manageable competition."},
		Sustainability:         research.ScoreExplanation{Score: 85, RatingBand: "very_high", Explanation: "Enough topic depth."},
		Differentiation:        research.ScoreExplanation{Score: 72, RatingBand: "high", Explanation: "Distinct positioning."},
	}
	return research.NicheReport{
		ID:            id,
		SchemaVersion: research.NicheReportSchemaVersion,
		Status:        research.StatusOK,
		Message:       "ok",
		Profile:       research.CreatorNicheProfile{WeeklyProductionCapacity: "2 videos per week"},
		Cache:         research.NicheCacheInfo{SchemaVersion: research.NicheReportSchemaVersion},
		Candidates: []research.NicheCandidate{{
			ID:         "candidate-1",
			Name:       "Test niche",
			Dimensions: dimensions,
			Scores: research.NicheScores{
				PersonalFit:    dimensions.CreatorFit,
				Demand:         dimensions.AudienceDemand,
				OpportunityGap: dimensions.CompetitionOpportunity,
				Sustainability: dimensions.Sustainability,
				Overall:        research.ScoreExplanation{Score: 75, RatingBand: "high", Explanation: "old cached overall"},
				Confidence:     research.ScoreExplanation{Score: 75, RatingBand: "high", Explanation: "confidence"},
			},
		}},
	}
}
