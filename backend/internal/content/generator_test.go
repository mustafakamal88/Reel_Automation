package content

import (
	"strings"
	"testing"
	"time"

	"trendcortex/api/internal/models"
)

func TestValidateRequestRequiresRealTrendInput(t *testing.T) {
	_, err := ValidateRequest(models.ReelContentGenerationRequest{})
	if err == nil {
		t.Fatal("expected error for missing trend candidate")
	}
	if !strings.Contains(err.Error(), "real trend candidate payload is required") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestValidateRequestRejectsUnsupportedSource(t *testing.T) {
	_, err := ValidateRequest(models.ReelContentGenerationRequest{
		TrendCandidate: &models.TrendCandidate{
			ID:      "manual-1",
			Source:  "manual",
			Keyword: "Manual topic",
			Title:   "Manual topic",
		},
	})
	if err == nil {
		t.Fatal("expected unsupported source error")
	}
	if !strings.Contains(err.Error(), "not a connected real trend provider") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestValidateRequestDefaultsAndNormalizesShape(t *testing.T) {
	req, err := ValidateRequest(models.ReelContentGenerationRequest{
		TrendCandidate: &models.TrendCandidate{
			ID:           "real-1",
			Source:       "google_trends_rss",
			Keyword:      "Real topic",
			Title:        "Real topic",
			Region:       "US",
			Language:     "en-US",
			DiscoveredAt: time.Now(),
			Status:       models.TrendCandidateStatusDiscovered,
		},
		PlatformTargets: []string{"instagram", "bad", "x", "instagram"},
	})
	if err != nil {
		t.Fatalf("ValidateRequest returned error: %v", err)
	}
	if got := strings.Join(req.PlatformTargets, ","); got != "instagram,x" {
		t.Fatalf("platforms = %q", got)
	}
	if req.DurationTarget != "30s" {
		t.Fatalf("duration = %q", req.DurationTarget)
	}
}

func TestBuildPromptNoMockDemoSeededTrendStrings(t *testing.T) {
	prompt, err := buildPrompt(GenerateRequest{
		Candidate: models.TrendCandidate{
			ID:           "real-1",
			Source:       "google_trends_rss",
			Keyword:      "Real topic",
			Title:        "Real topic",
			DiscoveredAt: time.Now(),
		},
		PlatformTargets: []string{"instagram"},
		DurationTarget:  "30s",
		Language:        "en-US",
		Region:          "US",
	})
	if err != nil {
		t.Fatalf("buildPrompt returned error: %v", err)
	}
	for _, forbidden := range []string{"demo trend", "mock trend", "seeded trend"} {
		if strings.Contains(strings.ToLower(prompt), forbidden) {
			t.Fatalf("prompt contains forbidden fixture string %q", forbidden)
		}
	}
}
