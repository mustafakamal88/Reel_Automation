package content

import (
	"encoding/json"
	"reflect"
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

func TestParseReelContentPackageAcceptsHashtagArray(t *testing.T) {
	pkg, err := parseReelContentPackage(`{
		"title": "Title",
		"hook": "Hook",
		"script": "Script",
		"caption": "Caption",
		"hashtags": [" #One ", "#two,#three", "two"],
		"thumbnail_brief": "Thumbnail",
		"instagram_caption": "Instagram",
		"tiktok_caption": "TikTok",
		"youtube_title": "YouTube",
		"youtube_description": "Description",
		"facebook_caption": "Facebook",
		"x_caption": "X",
		"safety_grounding_notes": ["Grounded"]
	}`)
	if err != nil {
		t.Fatalf("parseReelContentPackage returned error: %v", err)
	}
	want := []string{"#One", "#two", "#three"}
	if !reflect.DeepEqual(pkg.Hashtags, want) {
		t.Fatalf("hashtags = %#v, want %#v", pkg.Hashtags, want)
	}
}

func TestParseReelContentPackageAcceptsHashtagString(t *testing.T) {
	pkg, err := parseReelContentPackage(`{
		"title": "Title",
		"hook": "Hook",
		"script": "Script",
		"caption": "Caption",
		"hashtags": "#one #two #three",
		"thumbnail_brief": "Thumbnail",
		"instagram_caption": "Instagram",
		"tiktok_caption": "TikTok",
		"youtube_title": "YouTube",
		"youtube_description": "Description",
		"facebook_caption": "Facebook",
		"x_caption": "X",
		"safety_grounding_notes": ["Grounded"]
	}`)
	if err != nil {
		t.Fatalf("parseReelContentPackage returned error: %v", err)
	}
	want := []string{"#one", "#two", "#three"}
	if !reflect.DeepEqual(pkg.Hashtags, want) {
		t.Fatalf("hashtags = %#v, want %#v", pkg.Hashtags, want)
	}
}

func TestParseReelContentPackageRejectsInvalidJSON(t *testing.T) {
	_, err := parseReelContentPackage(`{"title":`)
	if err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestReelContentResponseFormatRequiresHashtagArray(t *testing.T) {
	body := map[string]any{
		"response_format": reelContentResponseFormat(),
	}
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	schema := string(payload)
	for _, want := range []string{`"type":"json_schema"`, `"hashtags":{"items":{"type":"string"},"type":"array"}`} {
		if !strings.Contains(schema, want) {
			t.Fatalf("schema %s does not contain %s", schema, want)
		}
	}
}
