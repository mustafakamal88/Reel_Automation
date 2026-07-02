package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"trendcortex/api/internal/config"
	"trendcortex/api/internal/content"
	"trendcortex/api/internal/models"
	trenddiscovery "trendcortex/api/internal/trends"
)

type fakeContentGenerator struct {
	pkg models.ReelContentPackage
	err error
	req content.GenerateRequest
}

func (f *fakeContentGenerator) Generate(_ context.Context, req content.GenerateRequest) (models.ReelContentPackage, error) {
	f.req = req
	if f.err != nil {
		return models.ReelContentPackage{}, f.err
	}
	return f.pkg, nil
}

func TestHandleGenerateReelContentMissingOpenAIKey(t *testing.T) {
	srv := NewServer(&config.Config{}, nil, nil, nil)
	rec := postGenerateReelContent(t, srv, validGenerateRequest())

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "OPENAI_API_KEY is not configured") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestHandleGenerateReelContentRequiresRealTrendPayload(t *testing.T) {
	srv := NewServer(&config.Config{OpenAIAPIKey: "present"}, nil, nil, nil)
	rec := postGenerateReelContent(t, srv, models.ReelContentGenerationRequest{})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "real trend candidate payload is required") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestHandleGenerateReelContentResolvesCandidateIDFromRealProvider(t *testing.T) {
	pubDate := "Thu, 02 Jul 2026 10:00:00 GMT"
	rss := `<?xml version="1.0" encoding="UTF-8"?><rss><channel><item><title>Real RSS trend</title><link>https://trends.google.com/trending/rss</link><pubDate>` + pubDate + `</pubDate><approx_traffic>10K+ searches</approx_traffic></item></channel></rss>`
	candidates, err := trenddiscovery.ParseGoogleTrendsRSS([]byte(rss), "US", "en-US", time.Now().UTC())
	if err != nil {
		t.Fatalf("parse fixture RSS: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidate count = %d", len(candidates))
	}

	fake := &fakeContentGenerator{pkg: models.ReelContentPackage{Title: "Generated from id"}}
	srv := NewServer(&config.Config{
		OpenAIAPIKey: "present",
	}, nil, nil, nil)
	srv.content = fake
	srv.discover = func(_ context.Context, region, language string, limit int) (trenddiscovery.DiscoverResult, error) {
		if region != "US" || language != "en-US" || limit != 100 {
			t.Fatalf("discover args = %q %q %d", region, language, limit)
		}
		return trenddiscovery.DiscoverResult{
			Provider:       trenddiscovery.ProviderGoogleTrendsRSS,
			ProviderStatus: trenddiscovery.ProviderStatusOK,
			Region:         region,
			Language:       language,
			Candidates:     candidates,
			DiscoveredAt:   time.Now().UTC(),
		}, nil
	}

	rec := postGenerateReelContent(t, srv, models.ReelContentGenerationRequest{
		TrendCandidateID: candidates[0].ID,
		PlatformTargets:  []string{"instagram"},
		DurationTarget:   "30s",
		Language:         "en-US",
		Region:           "US",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if fake.req.Candidate.ID != candidates[0].ID {
		t.Fatalf("resolved candidate id = %q", fake.req.Candidate.ID)
	}
	if fake.req.Candidate.Source != trenddiscovery.ProviderGoogleTrendsRSS {
		t.Fatalf("resolved source = %q", fake.req.Candidate.Source)
	}
}

func TestHandleGenerateReelContentFakeProviderReturnsExpectedPackage(t *testing.T) {
	fake := &fakeContentGenerator{pkg: models.ReelContentPackage{
		Title:            "Generated title",
		Hook:             "Generated hook",
		Script:           "Generated script",
		Caption:          "Generated caption",
		Hashtags:         []string{"#realtrend"},
		ThumbnailBrief:   "Generated thumbnail brief",
		InstagramCaption: "Instagram caption",
		TikTokCaption:    "TikTok caption",
		YouTubeTitle:     "YouTube title",
		FacebookCaption:  "Facebook caption",
		XCaption:         "X caption",
		ProviderMetadata: models.ReelProviderMetadata{
			Provider: "fake",
			Model:    "fake-model",
		},
	}}
	srv := NewServer(&config.Config{OpenAIAPIKey: "present"}, nil, nil, nil)
	srv.content = fake

	rec := postGenerateReelContent(t, srv, validGenerateRequest())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var body models.ReelContentGenerationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Package.Title != "Generated title" {
		t.Fatalf("title = %q", body.Package.Title)
	}
	if fake.req.Candidate.ID != "candidate-real-1" {
		t.Fatalf("candidate id = %q", fake.req.Candidate.ID)
	}
}

func TestHandleGenerateReelContentEndpointShapeStable(t *testing.T) {
	fake := &fakeContentGenerator{pkg: models.ReelContentPackage{
		Title:              "Title",
		Hook:               "Hook",
		Script:             "Script",
		Caption:            "Caption",
		Hashtags:           []string{"#tag"},
		ThumbnailBrief:     "Brief",
		InstagramCaption:   "IG",
		TikTokCaption:      "TT",
		YouTubeTitle:       "YT title",
		YouTubeDescription: "YT desc",
		FacebookCaption:    "FB",
		XCaption:           "X",
		SafetyGrounding:    []string{"Grounded in supplied candidate only."},
		ProviderMetadata: models.ReelProviderMetadata{
			Provider:          "fake",
			Model:             "fake-model",
			SourceCandidateID: "candidate-real-1",
			Source:            "google_trends_rss",
			PlatformTargets:   []string{"instagram", "tiktok"},
			DurationTarget:    "45s",
			GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
		},
	}}
	srv := NewServer(&config.Config{OpenAIAPIKey: "present"}, nil, nil, nil)
	srv.content = fake

	rec := postGenerateReelContent(t, srv, validGenerateRequest())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	pkg, ok := body["package"].(map[string]any)
	if !ok {
		t.Fatalf("response missing package object: %v", body)
	}
	for _, key := range []string{
		"title", "hook", "script", "caption", "hashtags", "thumbnail_brief",
		"instagram_caption", "tiktok_caption", "youtube_title", "youtube_description",
		"facebook_caption", "x_caption", "safety_grounding_notes", "provider_metadata",
	} {
		if _, ok := pkg[key]; !ok {
			t.Fatalf("package missing key %q", key)
		}
	}
}

func TestHandleGenerateReelContentProviderFailureIsHonest(t *testing.T) {
	srv := NewServer(&config.Config{OpenAIAPIKey: "present"}, nil, nil, nil)
	srv.content = &fakeContentGenerator{err: errors.New("OpenAI text generation returned HTTP 500: upstream failed")}

	rec := postGenerateReelContent(t, srv, validGenerateRequest())
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "upstream failed") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func validGenerateRequest() models.ReelContentGenerationRequest {
	return models.ReelContentGenerationRequest{
		TrendCandidate: &models.TrendCandidate{
			ID:           "candidate-real-1",
			Source:       "google_trends_rss",
			Region:       "US",
			Language:     "en-US",
			Keyword:      "Real RSS trend",
			Title:        "Real RSS trend",
			Score:        90,
			DiscoveredAt: time.Now().UTC(),
			SourceURL:    "https://trends.google.com/trending/rss?geo=US&hl=en-US",
			Evidence:     "10K+ searches",
			Status:       models.TrendCandidateStatusDiscovered,
		},
		PlatformTargets: []string{"instagram", "tiktok"},
		DurationTarget:  "45s",
		Language:        "en-US",
		Region:          "US",
	}
}

func postGenerateReelContent(t *testing.T, srv *Server, body models.ReelContentGenerationRequest) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/reels/generate-script", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	srv.handleGenerateReelContent(rec, req)
	return rec
}
