package content

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"trendcortex/api/internal/models"
)

const (
	ProviderOpenAI = "openai"
	DefaultModel   = "gpt-4o-mini"
)

var (
	ErrProviderNotConfigured = errors.New("OPENAI_API_KEY is not configured")
	ErrInvalidTrendCandidate = errors.New("real trend candidate payload is required")
)

type Generator interface {
	Generate(ctx context.Context, req GenerateRequest) (models.ReelContentPackage, error)
}

type GenerateRequest struct {
	Candidate       models.TrendCandidate
	PlatformTargets []string
	DurationTarget  string
	ToneStyle       string
	Language        string
	Region          string
}

type OpenAIGenerator struct {
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

func (g OpenAIGenerator) Generate(ctx context.Context, req GenerateRequest) (models.ReelContentPackage, error) {
	if strings.TrimSpace(g.APIKey) == "" {
		return models.ReelContentPackage{}, ErrProviderNotConfigured
	}
	model := strings.TrimSpace(g.Model)
	if model == "" {
		model = DefaultModel
	}
	prompt, err := buildPrompt(req)
	if err != nil {
		return models.ReelContentPackage{}, err
	}

	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You generate concise social video scripts from real trend candidate data supplied by the backend. Use only the supplied candidate fields as grounding. Do not add source claims, facts, numbers, names, dates, or evidence that are not present in the supplied JSON. Return valid JSON only.",
			},
			{"role": "user", "content": prompt},
		},
		"temperature":     0.6,
		"response_format": reelContentResponseFormat(),
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return models.ReelContentPackage{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return models.ReelContentPackage{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+g.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	client := g.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	res, err := client.Do(httpReq)
	if err != nil {
		return models.ReelContentPackage{}, fmt.Errorf("OpenAI text generation request failed: %w", err)
	}
	defer res.Body.Close()
	resBody, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return models.ReelContentPackage{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return models.ReelContentPackage{}, fmt.Errorf("OpenAI text generation returned HTTP %d: %s", res.StatusCode, trimForLog(resBody))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(resBody, &parsed); err != nil {
		return models.ReelContentPackage{}, err
	}
	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return models.ReelContentPackage{}, errors.New("OpenAI text generation response did not include message content")
	}

	pkg, err := parseReelContentPackage(parsed.Choices[0].Message.Content)
	if err != nil {
		return models.ReelContentPackage{}, fmt.Errorf("OpenAI text generation JSON parse failed: %w", err)
	}
	pkg.ProviderMetadata = metadata(req, model)
	return pkg, nil
}

func ValidateRequest(req models.ReelContentGenerationRequest) (GenerateRequest, error) {
	if req.TrendCandidate == nil {
		return GenerateRequest{}, ErrInvalidTrendCandidate
	}
	candidate := *req.TrendCandidate
	if strings.TrimSpace(candidate.ID) == "" && strings.TrimSpace(req.TrendCandidateID) != "" {
		candidate.ID = strings.TrimSpace(req.TrendCandidateID)
	}
	if strings.TrimSpace(candidate.ID) == "" || strings.TrimSpace(firstNonEmpty(candidate.Title, candidate.Keyword)) == "" {
		return GenerateRequest{}, ErrInvalidTrendCandidate
	}
	if !isRealSupportedSource(candidate.Source) {
		return GenerateRequest{}, fmt.Errorf("%w: source %q is not a connected real trend provider", ErrInvalidTrendCandidate, candidate.Source)
	}

	platforms := normalizePlatforms(req.PlatformTargets)
	if len(platforms) == 0 {
		platforms = []string{"instagram", "tiktok", "youtube", "facebook", "x"}
	}
	duration := strings.TrimSpace(req.DurationTarget)
	if duration == "" {
		duration = "30s"
	}
	language := strings.TrimSpace(firstNonEmpty(req.Language, candidate.Language))
	if language == "" {
		language = "en-US"
	}
	region := strings.TrimSpace(firstNonEmpty(req.Region, candidate.Region))
	if region == "" {
		region = "US"
	}

	return GenerateRequest{
		Candidate:       candidate,
		PlatformTargets: platforms,
		DurationTarget:  duration,
		ToneStyle:       strings.TrimSpace(req.ToneStyle),
		Language:        language,
		Region:          region,
	}, nil
}

func buildPrompt(req GenerateRequest) (string, error) {
	candidatePayload, err := json.MarshalIndent(req.Candidate, "", "  ")
	if err != nil {
		return "", err
	}
	instructions := map[string]any{
		"task":             "Generate a grounded short-form reel script and caption package from the candidate JSON.",
		"candidate":        json.RawMessage(candidatePayload),
		"platform_targets": req.PlatformTargets,
		"duration_target":  req.DurationTarget,
		"tone_style":       req.ToneStyle,
		"language":         req.Language,
		"region":           req.Region,
		"output_shape": []string{
			"title", "hook", "script", "caption", "hashtags", "thumbnail_brief",
			"instagram_caption", "tiktok_caption", "youtube_title", "youtube_description",
			"facebook_caption", "x_caption", "safety_grounding_notes",
		},
		"field_contract": map[string]string{
			"hashtags": "JSON array of hashtag strings only, never a single string. Example: [\"#one\", \"#two\", \"#three\"]",
		},
		"grounding_rules": []string{
			"Treat the candidate title/keyword as the topic.",
			"Use candidate evidence and source_url only as the cited grounding context.",
			"Do not claim facts beyond the candidate fields.",
			"If evidence is thin, say so in safety_grounding_notes.",
			"Do not include provider_metadata; the backend fills it.",
		},
	}
	out, err := json.MarshalIndent(instructions, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

type reelContentPackageOpenAI struct {
	Title              string                      `json:"title"`
	Hook               string                      `json:"hook"`
	Script             string                      `json:"script"`
	Caption            string                      `json:"caption"`
	Hashtags           flexibleHashtags            `json:"hashtags"`
	ThumbnailBrief     string                      `json:"thumbnail_brief"`
	InstagramCaption   string                      `json:"instagram_caption"`
	TikTokCaption      string                      `json:"tiktok_caption"`
	YouTubeTitle       string                      `json:"youtube_title"`
	YouTubeDescription string                      `json:"youtube_description"`
	FacebookCaption    string                      `json:"facebook_caption"`
	XCaption           string                      `json:"x_caption"`
	SafetyGrounding    []string                    `json:"safety_grounding_notes"`
	ProviderMetadata   models.ReelProviderMetadata `json:"provider_metadata"`
}

type flexibleHashtags []string

func (h *flexibleHashtags) UnmarshalJSON(data []byte) error {
	var tags []string
	if err := json.Unmarshal(data, &tags); err == nil {
		*h = normalizeHashtags(tags)
		return nil
	}

	var tagString string
	if err := json.Unmarshal(data, &tagString); err != nil {
		return err
	}
	*h = normalizeHashtagString(tagString)
	return nil
}

func parseReelContentPackage(content string) (models.ReelContentPackage, error) {
	var openAI reelContentPackageOpenAI
	if err := json.Unmarshal([]byte(content), &openAI); err != nil {
		return models.ReelContentPackage{}, err
	}
	return models.ReelContentPackage{
		Title:              openAI.Title,
		Hook:               openAI.Hook,
		Script:             openAI.Script,
		Caption:            openAI.Caption,
		Hashtags:           []string(openAI.Hashtags),
		ThumbnailBrief:     openAI.ThumbnailBrief,
		InstagramCaption:   openAI.InstagramCaption,
		TikTokCaption:      openAI.TikTokCaption,
		YouTubeTitle:       openAI.YouTubeTitle,
		YouTubeDescription: openAI.YouTubeDescription,
		FacebookCaption:    openAI.FacebookCaption,
		XCaption:           openAI.XCaption,
		SafetyGrounding:    openAI.SafetyGrounding,
		ProviderMetadata:   openAI.ProviderMetadata,
	}, nil
}

func normalizeHashtags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		out = appendHashtags(out, seen, tag)
	}
	return out
}

func normalizeHashtagString(tags string) []string {
	return appendHashtags(nil, map[string]bool{}, tags)
}

func appendHashtags(out []string, seen map[string]bool, tags string) []string {
	fields := strings.FieldsFunc(tags, func(r rune) bool {
		return unicode.IsSpace(r) || r == ','
	})
	for _, field := range fields {
		tag := strings.Trim(field, "\"'`[](){}.,;:")
		if tag == "" {
			continue
		}
		if !strings.HasPrefix(tag, "#") {
			tag = "#" + tag
		}
		key := strings.ToLower(tag)
		if seen[key] {
			continue
		}
		out = append(out, tag)
		seen[key] = true
	}
	return out
}

func reelContentResponseFormat() map[string]any {
	stringSchema := map[string]any{"type": "string"}
	stringArraySchema := map[string]any{
		"type":  "array",
		"items": stringSchema,
	}
	properties := map[string]any{
		"title":                  stringSchema,
		"hook":                   stringSchema,
		"script":                 stringSchema,
		"caption":                stringSchema,
		"hashtags":               stringArraySchema,
		"thumbnail_brief":        stringSchema,
		"instagram_caption":      stringSchema,
		"tiktok_caption":         stringSchema,
		"youtube_title":          stringSchema,
		"youtube_description":    stringSchema,
		"facebook_caption":       stringSchema,
		"x_caption":              stringSchema,
		"safety_grounding_notes": stringArraySchema,
	}
	return map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "reel_content_package",
			"strict": true,
			"schema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties":           properties,
				"required": []string{
					"title", "hook", "script", "caption", "hashtags", "thumbnail_brief",
					"instagram_caption", "tiktok_caption", "youtube_title", "youtube_description",
					"facebook_caption", "x_caption", "safety_grounding_notes",
				},
			},
		},
	}
}

func metadata(req GenerateRequest, model string) models.ReelProviderMetadata {
	return models.ReelProviderMetadata{
		Provider:          ProviderOpenAI,
		Model:             model,
		SourceCandidateID: req.Candidate.ID,
		Source:            req.Candidate.Source,
		SourceURL:         req.Candidate.SourceURL,
		PlatformTargets:   req.PlatformTargets,
		DurationTarget:    req.DurationTarget,
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
	}
}

func isRealSupportedSource(source string) bool {
	switch strings.TrimSpace(strings.ToLower(source)) {
	case "google_trends_rss":
		return true
	default:
		return false
	}
}

func normalizePlatforms(platforms []string) []string {
	allowed := map[string]bool{
		"instagram": true,
		"tiktok":    true,
		"youtube":   true,
		"facebook":  true,
		"x":         true,
	}
	seen := map[string]bool{}
	out := []string{}
	for _, p := range platforms {
		p = strings.TrimSpace(strings.ToLower(p))
		if allowed[p] && !seen[p] {
			out = append(out, p)
			seen[p] = true
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func trimForLog(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) > 500 {
		return s[:500] + "..."
	}
	return s
}
