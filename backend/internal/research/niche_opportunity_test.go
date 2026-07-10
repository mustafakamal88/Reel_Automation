package research

import (
	"strings"
	"testing"
)

func TestOpportunityScoreFormula(t *testing.T) {
	got := CalculateOpportunityScore(OpportunityScoreInput{
		DemandScore:       80,
		MonetizationScore: 70,
		CompetitionScore:  30,
		CreatorFitScore:   60,
	})
	want := 72.5
	if got != want {
		t.Fatalf("CalculateOpportunityScore = %.1f, want %.1f", got, want)
	}
}

func TestHighDemandHighMonetizationLowCompetitionScoresHigh(t *testing.T) {
	got := CalculateOpportunityScore(OpportunityScoreInput{
		DemandScore:       90,
		MonetizationScore: 88,
		CompetitionScore:  18,
		CreatorFitScore:   70,
	})
	if got < 80 {
		t.Fatalf("score = %.1f, want high opportunity", got)
	}
}

func TestHighDemandSaturatedCompetitionScoresLower(t *testing.T) {
	open := CalculateOpportunityScore(OpportunityScoreInput{DemandScore: 90, MonetizationScore: 75, CompetitionScore: 25, CreatorFitScore: 70})
	saturated := CalculateOpportunityScore(OpportunityScoreInput{DemandScore: 90, MonetizationScore: 75, CompetitionScore: 88, CreatorFitScore: 70})
	if saturated >= open {
		t.Fatalf("saturated score %.1f should be lower than open score %.1f", saturated, open)
	}
}

func TestGroupedNicheIdeasAreNotRawKeywords(t *testing.T) {
	req := normalizeNicheOpportunityRequest(NicheOpportunityRequest{SeedKeyword: "trading", Country: "GB", Audience: "UK Pakistani", ContentStyle: "short-form explainers"})
	for _, idea := range groupedNicheIdeas(req) {
		if strings.EqualFold(strings.TrimSpace(idea.Name), "trading") {
			t.Fatalf("raw keyword returned as niche: %+v", idea)
		}
		if len(strings.Fields(idea.Name)) < 3 {
			t.Fatalf("niche name is not grouped enough: %q", idea.Name)
		}
	}
}

func TestMissingGoogleAdsConfigUsesHeuristicMonetizationWithLimitation(t *testing.T) {
	status := GoogleAdsKeywordPlannerStatus("", "", "", "", "")
	if status.Status != StatusNotConfigured {
		t.Fatalf("status = %q, want not_configured", status.Status)
	}
	score, _, confidence, reason := scoreMonetization(nicheIdea{Name: "Trading education for beginners", Tags: []string{"trading", "education"}}, normalizeNicheOpportunityRequest(NicheOpportunityRequest{Country: "GB"}), status)
	if confidence != "low" {
		t.Fatalf("confidence = %q, want low", confidence)
	}
	if score <= 0 {
		t.Fatalf("score = %.1f, want heuristic score", score)
	}
	if !strings.Contains(strings.ToLower(reason), "not configured") || !strings.Contains(strings.ToLower(reason), "not exact youtube rpm") {
		t.Fatalf("reason missing heuristic limitation: %q", reason)
	}
}

func TestNicheOpportunityDoesNotClaimExactRPM(t *testing.T) {
	status := GoogleAdsKeywordPlannerStatus("", "", "", "", "")
	op := buildOpportunity(
		normalizeNicheOpportunityRequest(NicheOpportunityRequest{SeedKeyword: "software", Country: "US", Audience: "local creators", ContentStyle: "short-form tutorials"}),
		nicheIdea{Name: "Software tutorials for local creators", Query: "software tutorials", Tags: []string{"software", "tutorials"}},
		[]ChannelVideoSummary{
			{VideoID: "v1", Title: "Best software tutorial", ChannelTitle: "Creator Lab", PublishedAt: "2026-07-01T00:00:00Z", Views: uintPtr(120000)},
			{VideoID: "v2", Title: "Software tools explained", ChannelTitle: "Tool Lab", PublishedAt: "2026-06-20T00:00:00Z", Views: uintPtr(80000)},
		},
		status,
	)
	joined := strings.ToLower(op.MonetizationReason + " " + strings.Join(op.Limitations, " "))
	if strings.Contains(joined, "$") || strings.Contains(joined, " rpm:") || strings.Contains(joined, " cpm:") {
		t.Fatalf("opportunity claims exact RPM/CPM: %q", joined)
	}
	if !strings.Contains(joined, "not exact youtube rpm") {
		t.Fatalf("opportunity should explicitly label monetization as not exact RPM: %q", joined)
	}
}

func uintPtr(v uint64) *uint64 {
	return &v
}
