package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"trendcortex/api/internal/config"
	"trendcortex/api/internal/models"
)

func TestHandleGenerateResearchScriptValidatesRequiredFields(t *testing.T) {
	srv := NewServer(&config.Config{OpenAIAPIKey: "present"}, nil, nil, nil)
	rec := postResearchScript(t, srv, models.ResearchScriptGenerationRequest{
		SourceType: "youtube_video_analysis",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "analysis needs a title or topic") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestHandleGenerateResearchScriptMissingOpenAIKey(t *testing.T) {
	srv := NewServer(&config.Config{}, nil, nil, nil)
	rec := postResearchScript(t, srv, validResearchScriptRequest())

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"not_configured"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestHandleGenerateResearchScriptWithMockedProvider(t *testing.T) {
	fake := &fakeContentGenerator{pkg: models.ReelContentPackage{
		Title:              "Generated title",
		Hook:               "Generated hook",
		Script:             "Generated script",
		Caption:            "Generated caption",
		Hashtags:           []string{"#youtube"},
		ThumbnailBrief:     "Thumbnail",
		InstagramCaption:   "IG",
		TikTokCaption:      "TT",
		YouTubeDescription: "YT desc",
		FacebookCaption:    "FB",
		XCaption:           "X",
	}}
	srv := NewServer(&config.Config{OpenAIAPIKey: "present"}, nil, nil, nil)
	srv.content = fake

	rec := postResearchScript(t, srv, validResearchScriptRequest())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var body models.ResearchScriptGenerationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Package.Title != "Generated title" {
		t.Fatalf("title = %q", body.Package.Title)
	}
	if fake.req.Candidate.Source != "youtube_video_analysis" {
		t.Fatalf("source = %q", fake.req.Candidate.Source)
	}
	if fake.req.Candidate.SourceURL != "https://www.youtube.com/watch?v=abc123" {
		t.Fatalf("source url = %q", fake.req.Candidate.SourceURL)
	}
}

func TestHandleGenerateResearchScriptPreservesMetadataAndLimitations(t *testing.T) {
	srv := NewServer(&config.Config{OpenAIAPIKey: "present"}, nil, nil, nil)
	srv.content = &fakeContentGenerator{pkg: models.ReelContentPackage{
		Title:              "Generated title",
		Hook:               "Generated hook",
		Script:             "Generated script",
		Caption:            "Generated caption",
		Hashtags:           []string{"#youtube"},
		ThumbnailBrief:     "Thumbnail",
		YouTubeDescription: "YT desc",
	}}

	rec := postResearchScript(t, srv, validResearchScriptRequest())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var body models.ResearchScriptGenerationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	pkg := body.Package
	if pkg.SourceType != "youtube_video_analysis" {
		t.Fatalf("source_type = %q", pkg.SourceType)
	}
	if pkg.SourceURL != "https://www.youtube.com/watch?v=abc123" {
		t.Fatalf("source_url = %q", pkg.SourceURL)
	}
	if got := strings.Join(pkg.InferredKeywords, ","); !strings.Contains(got, "retention") {
		t.Fatalf("keywords = %q", got)
	}
	if pkg.InferredNiche != "Creator education" || pkg.InferredAngle != "Retention teardown" {
		t.Fatalf("niche/angle = %q / %q", pkg.InferredNiche, pkg.InferredAngle)
	}
	if !strings.Contains(pkg.GroundingEvidence, "YouTube keywords are inferred from public metadata, not exact search ranking terms.") {
		t.Fatalf("grounding missing YouTube limitation: %s", pkg.GroundingEvidence)
	}
	if pkg.CreatedAt == "" {
		t.Fatal("created_at was empty")
	}
}

func TestResearchScriptRequestSanitizesContaminatedCreativeContext(t *testing.T) {
	req := validResearchScriptRequest()
	req.Topic = "not"
	req.Keywords = []string{"ICT trading", "not", "own research consult licensed", "fair value gap"}
	req.Summary = "Write a short adaptation using these inferred public topics: financial, not. Always do your own research and consult a licensed financial adviser."
	req.SuggestedAngle = "Comparison angle around fair value gap. Not financial advice."
	req.Evidence = map[string]any{
		"script_prompts": []any{"Write about liquidity", "Write about own research consult licensed"},
	}

	genReq, candidate, keywords, _, angle, err := researchScriptGenerateRequest(req)
	if err != nil {
		t.Fatalf("researchScriptGenerateRequest: %v", err)
	}
	joined := strings.ToLower(strings.Join(append([]string{candidate.Title, candidate.Keyword, candidate.Evidence, angle, genReq.Candidate.Evidence}, keywords...), " | "))
	for _, noise := range []string{"own research", "licensed financial", "not financial advice", "financial, not", "own research consult"} {
		if strings.Contains(joined, noise) {
			t.Fatalf("script generation context includes rejected phrase %q: %s", noise, joined)
		}
	}
	if !strings.Contains(joined, "ict trading") || !strings.Contains(joined, "fair value gap") || !strings.Contains(joined, "liquidity") {
		t.Fatalf("script generation context lost legitimate topics: %s", joined)
	}
}

func validResearchScriptRequest() models.ResearchScriptGenerationRequest {
	return models.ResearchScriptGenerationRequest{
		SourceType:      "youtube_video_analysis",
		SourceID:        "abc123",
		SourceURL:       "https://www.youtube.com/watch?v=abc123",
		Topic:           "retention hooks",
		Title:           "How creators keep viewers watching",
		Summary:         "Public YouTube metadata suggests this video performs on strong title structure and clear hook framing.",
		Keywords:        []string{"retention", "creator hooks"},
		InferredNiche:   "Creator education",
		InferredAngle:   "Retention teardown",
		SuggestedAngle:  "Open with the mistake most creators make in the first three seconds.",
		TargetPlatforms: []string{"instagram", "youtube"},
		DurationSeconds: 30,
		Limitations:     []string{"Public metadata only."},
		Language:        "en-US",
		Region:          "US",
		Metadata: map[string]any{
			"score": 81,
		},
	}
}

func postResearchScript(t *testing.T, srv *Server, body models.ResearchScriptGenerationRequest) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/research/script", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	srv.handleGenerateResearchScript(rec, req)
	return rec
}
