package research

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

const videoAnalysisSchemaVersion = "video_analysis_v2_quality_revenue_visuals"

type FormattedVideoMetadata struct {
	PublishedDate     string   `json:"published_date,omitempty"`
	PublishedRelative string   `json:"published_relative,omitempty"`
	Duration          string   `json:"duration,omitempty"`
	Views             string   `json:"views,omitempty"`
	Likes             string   `json:"likes,omitempty"`
	Comments          string   `json:"comments,omitempty"`
	Format            string   `json:"format,omitempty"`
	UnavailableCounts []string `json:"unavailable_counts,omitempty"`
}

type ScoreDimension struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Score       int    `json:"score"`
	Rating      string `json:"rating"`
	Explanation string `json:"explanation"`
	Evidence    string `json:"evidence,omitempty"`
}

type PerformanceMetric struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Value       string  `json:"value"`
	Score       int     `json:"score"`
	Explanation string  `json:"explanation"`
	RawValue    float64 `json:"raw_value,omitempty"`
}

type RevenueEstimate struct {
	Source                     string   `json:"source"`
	ModelType                  string   `json:"model_type"`
	Currency                   string   `json:"currency"`
	Low                        float64  `json:"low"`
	Midpoint                   float64  `json:"midpoint"`
	High                       float64  `json:"high"`
	FormattedRange             string   `json:"formatted_range"`
	RPMLow                     float64  `json:"rpm_low"`
	RPMHigh                    float64  `json:"rpm_high"`
	EstimatedRevenuePer1000    string   `json:"estimated_revenue_per_1000_views"`
	Confidence                 string   `json:"confidence"`
	ConfidenceScore            int      `json:"confidence_score"`
	CalculationBasis           string   `json:"calculation_basis"`
	Assumptions                []string `json:"assumptions"`
	Exclusions                 []string `json:"exclusions"`
	FutureRevenueScenario      string   `json:"future_revenue_scenario,omitempty"`
	MonetisationEligibility    string   `json:"monetisation_eligibility"`
	ActualAnalyticsUnavailable bool     `json:"actual_analytics_unavailable"`
}

type EvidenceBasis struct {
	Field  string `json:"field"`
	Source string `json:"source"`
	Basis  string `json:"basis"`
}

func formatVideoMetadata(item youtubeVideoItem, views, likes, comments *uint64, now func() time.Time, sourceURL string) FormattedVideoMetadata {
	unavailable := []string{}
	if likes == nil {
		unavailable = append(unavailable, "likes")
	}
	if comments == nil {
		unavailable = append(unavailable, "comments")
	}
	return FormattedVideoMetadata{
		PublishedDate:     formatPublishedDate(item.Snippet.PublishedAt),
		PublishedRelative: formatRelativeDate(item.Snippet.PublishedAt, now),
		Duration:          formatISO8601Duration(item.ContentDetails.Duration),
		Views:             formatUintCount(views),
		Likes:             formatUintCount(likes),
		Comments:          formatUintCount(comments),
		Format:            classifyVideoFormat(item.ContentDetails.Duration, sourceURL),
		UnavailableCounts: unavailable,
	}
}

func buildScoreDimensions(item youtubeVideoItem, kw KeywordIntelligence, hook HookIntelligence, niche NicheAnalysis, views, likes, comments *uint64, now func() time.Time, sourceURL string) ([]ScoreDimension, ScoreDimension) {
	format := classifyVideoFormat(item.ContentDetails.Duration, sourceURL)
	metadata := clampInt(kw.MetadataStrengthScore)
	titleHook := clampInt((hook.ClarityScore + hook.SpecificityScore + hook.CuriosityScore + hook.AudienceSignalScore + hook.ValuePromiseScore) / 5)
	topicClarity := clampInt(int(niche.Confidence*100)*65/100 + minInt(len(kw.PrimaryTopics), 5)*7)
	audienceFit := clampInt(hook.AudienceSignalScore/2 + int(niche.Confidence*50))
	searchIntent := 45
	if len(kw.SearchPhrases) >= 3 {
		searchIntent += 25
	}
	if strings.Contains(strings.ToLower(kw.InferredSearchIntent), "tutorial") || strings.Contains(strings.ToLower(kw.InferredSearchIntent), "review") {
		searchIntent += 10
	}
	remake := hook.RemakePotentialScore
	monetisation := monetisationPotentialScore(niche, format, views)
	confidence := analysisConfidenceScore(item, kw, niche, views, likes, comments)
	dims := []ScoreDimension{
		scoreDimension("metadata_quality", "Metadata Quality", metadata, "Title, public description, tags, topics, duration, and visible engagement determine how much evidence is available.", ""),
		scoreDimension("title_hook_strength", "Title and Hook Strength", titleHook, hook.Explanation, item.Snippet.Title),
		scoreDimension("topic_clarity", "Topic Clarity", topicClarity, "Measures whether cleaned public metadata points to a coherent topic rather than fragments.", niche.SpecificTopic),
		scoreDimension("audience_fit", "Audience Fit", audienceFit, "Estimates how clearly the title and public metadata identify who the video is for.", niche.TargetAudience),
		scoreDimension("search_intent_strength", "Search Intent Strength", clampInt(searchIntent), kw.InferredSearchIntent, strings.Join(topN(kw.SearchPhrases, 3), ", ")),
		scoreDimension("remake_potential", "Remake Potential", remake, "Higher when the public title premise can be adapted into concrete alternate formats without inventing private performance data.", strings.Join(topN(kw.PrimaryTopics, 3), ", ")),
		scoreDimension("monetisation_potential", "Monetisation Potential", monetisation, "Public estimate based on niche advertiser fit, format, duration, and view count. It is not actual YouTube revenue.", format),
	}
	conf := scoreDimension("analysis_confidence", "Analysis Confidence", confidence, "Confidence reflects metadata completeness, keyword quality, classification agreement, visible engagement counts, channel context, and the absence of transcript evidence.", "")
	return dims, conf
}

func buildPerformanceProfile(publishedAt string, views, likes, comments *uint64, now func() time.Time) []PerformanceMetric {
	signals := performanceSignals(publishedAt, views, likes, comments, now)
	out := []PerformanceMetric{}
	if raw, ok := asFloat(signals["views_per_day"]); ok {
		out = append(out, PerformanceMetric{ID: "views_per_day", Label: "Views per day", Value: formatFloat(raw, 0), RawValue: raw, Score: scoreViewsPerDay(raw), Explanation: "TrendCortex public-signal scale based on current public views divided by video age."})
	}
	if raw, ok := asFloat(signals["likes_per_1000_views"]); ok {
		out = append(out, PerformanceMetric{ID: "likes_per_1000_views", Label: "Likes per 1,000 views", Value: formatFloat(raw, 1), RawValue: raw, Score: scoreRate(raw, 5, 60), Explanation: "TrendCortex public-signal scale; hidden likes are treated as unavailable, not zero."})
	}
	if raw, ok := asFloat(signals["comments_per_1000_views"]); ok {
		out = append(out, PerformanceMetric{ID: "comments_per_1000_views", Label: "Comments per 1,000 views", Value: formatFloat(raw, 1), RawValue: raw, Score: scoreRate(raw, 0.5, 10), Explanation: "TrendCortex public-signal scale; hidden comments are treated as unavailable, not zero."})
	}
	if raw, ok := asFloat(signals["engagement_rate"]); ok {
		out = append(out, PerformanceMetric{ID: "public_engagement_rate", Label: "Public engagement rate", Value: fmt.Sprintf("%.2f%%", raw*100), RawValue: raw, Score: scoreRate(raw*100, 0.5, 8), Explanation: "Visible likes and comments divided by public views. Missing visible counts lower confidence."})
	}
	return out
}

func estimateRevenue(item youtubeVideoItem, niche NicheAnalysis, views *uint64, now func() time.Time, sourceURL string) RevenueEstimate {
	format := classifyVideoFormat(item.ContentDetails.Duration, sourceURL)
	rpmLow, rpmHigh := rpmRange(niche, format)
	confidenceScore := 42
	if views != nil && *views >= 1000 {
		confidenceScore += 8
	}
	if format != "unknown" {
		confidenceScore += 8
	} else {
		rpmLow *= 0.65
		rpmHigh *= 1.35
	}
	if item.Snippet.CategoryID == "10" {
		confidenceScore -= 10
	}
	if strings.Contains(strings.ToLower(item.Snippet.Title+" "+item.Snippet.Description), "kids") {
		confidenceScore -= 8
		rpmLow *= 0.7
		rpmHigh *= 0.85
	}
	rpmLow = roundMoney(rpmLow)
	rpmHigh = roundMoney(rpmHigh)
	var low, high float64
	publicViews := uint64(0)
	if views != nil {
		publicViews = *views
		low = float64(*views) / 1000 * rpmLow
		high = float64(*views) / 1000 * rpmHigh
	}
	mid := (low + high) / 2
	confidenceScore = clampInt(confidenceScore)
	return RevenueEstimate{
		Source:                     "public_estimate",
		ModelType:                  "public_views_x_estimated_rpm_range",
		Currency:                   "USD",
		Low:                        roundMoney(low),
		Midpoint:                   roundMoney(mid),
		High:                       roundMoney(high),
		FormattedRange:             formatMoneyRange(low, high),
		RPMLow:                     rpmLow,
		RPMHigh:                    rpmHigh,
		EstimatedRevenuePer1000:    fmt.Sprintf("$%.2f-$%.2f estimated RPM", rpmLow, rpmHigh),
		Confidence:                 ratingForScore(confidenceScore),
		ConfidenceScore:            confidenceScore,
		CalculationBasis:           fmt.Sprintf("Based on %s public views and an estimated %s RPM range. Formula: public views / 1,000 x estimated RPM range.", formatUint(publicViews), strings.ReplaceAll(format, "_", " ")),
		Assumptions:                revenueAssumptions(item, niche, format),
		Exclusions:                 []string{"Actual YouTube Analytics revenue", "Exact RPM or playback-based CPM", "Audience geography", "Premium revenue adjustments", "Invalid-traffic adjustments", "Sponsorships", "Affiliate revenue", "Memberships", "Merchandise", "Course or product sales"},
		FutureRevenueScenario:      fmt.Sprintf("Each additional 10,000 public views would add roughly %s at the current estimated RPM range.", formatMoneyRange(10000/1000*rpmLow, 10000/1000*rpmHigh)),
		MonetisationEligibility:    "Unknown from public metadata",
		ActualAnalyticsUnavailable: true,
	}
}

func evidenceBasis() []EvidenceBasis {
	return []EvidenceBasis{
		{Field: "video metadata", Source: "YouTube Data API public videos.list fields", Basis: "title, description, tags, category, topic details, duration, published date, and visible counts"},
		{Field: "keywords and niche", Source: "TrendCortex deterministic analysis", Basis: "cleaned public title, description, tags, category, and topic metadata"},
		{Field: "revenue estimate", Source: "TrendCortex public estimate", Basis: "public views multiplied by a conservative estimated RPM range; no private YouTube Analytics"},
	}
}

func scoreDimension(id, label string, score int, explanation, evidence string) ScoreDimension {
	score = clampInt(score)
	return ScoreDimension{ID: id, Label: label, Score: score, Rating: ratingForScore(score), Explanation: strings.TrimSpace(explanation), Evidence: strings.TrimSpace(evidence)}
}

func ratingForScore(score int) string {
	switch {
	case score >= 85:
		return "Strong"
	case score >= 70:
		return "Good"
	case score >= 50:
		return "Moderate"
	case score >= 30:
		return "Limited"
	default:
		return "Weak"
	}
}

func classifyVideoFormat(duration, sourceURL string) string {
	seconds, ok := parseISO8601DurationSeconds(duration)
	lowerURL := strings.ToLower(sourceURL)
	switch {
	case strings.Contains(lowerURL, "/shorts/") && ok && seconds <= 180:
		return "short_form"
	case strings.Contains(lowerURL, "/live/"):
		return "livestream"
	case ok && seconds == 0:
		return "livestream"
	case ok && seconds > 180:
		return "long_form"
	case ok && seconds <= 180:
		return "short_form"
	default:
		return "unknown"
	}
}

func parseISO8601DurationSeconds(raw string) (int, bool) {
	if strings.TrimSpace(raw) == "" {
		return 0, false
	}
	re := regexp.MustCompile(`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`)
	m := re.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return 0, false
	}
	total := 0
	multipliers := []int{86400, 3600, 60, 1}
	for i := 1; i < len(m); i++ {
		if m[i] == "" {
			continue
		}
		var v int
		fmt.Sscanf(m[i], "%d", &v)
		total += v * multipliers[i-1]
	}
	return total, true
}

func formatISO8601Duration(raw string) string {
	seconds, ok := parseISO8601DurationSeconds(raw)
	if !ok {
		return "Unavailable"
	}
	if seconds == 0 {
		return "Live or unavailable"
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func formatPublishedDate(raw string) string {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return "Unavailable"
	}
	return t.Format("2 Jan 2006")
}

func formatRelativeDate(raw string, now func() time.Time) string {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return ""
	}
	days := int(now().Sub(t).Hours() / 24)
	switch {
	case days <= 0:
		return "Published today"
	case days == 1:
		return "Published 1 day ago"
	default:
		return fmt.Sprintf("Published %d days ago", days)
	}
}

func formatUintCount(v *uint64) string {
	if v == nil {
		return "Unavailable"
	}
	return formatUint(*v)
}

func formatUint(v uint64) string {
	s := fmt.Sprintf("%d", v)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, ",")
}

func rpmRange(niche NicheAnalysis, format string) (float64, float64) {
	combined := strings.ToLower(niche.BroadCategory + " " + niche.PrimaryNiche + " " + niche.Niche + " " + niche.SpecificTopic)
	low, high := 0.8, 4.5
	switch {
	case strings.Contains(combined, "finance") || strings.Contains(combined, "business"):
		low, high = 3.5, 14
	case strings.Contains(combined, "software") || strings.Contains(combined, "technology") || strings.Contains(combined, "ai"):
		low, high = 2.2, 9
	case strings.Contains(combined, "education"):
		low, high = 1.5, 6
	case strings.Contains(combined, "beauty") || strings.Contains(combined, "food"):
		low, high = 1.2, 5
	case strings.Contains(combined, "music") || strings.Contains(combined, "comedy") || strings.Contains(combined, "gaming"):
		low, high = 0.4, 3
	}
	if format == "short_form" {
		low *= 0.08
		high *= 0.22
	}
	if format == "livestream" {
		low *= 0.7
		high *= 1.2
	}
	return low, high
}

func revenueAssumptions(item youtubeVideoItem, niche NicheAnalysis, format string) []string {
	return []string{
		"Uses public view count only; monetisation status is unknown.",
		"Assumes the video is eligible for monetisation, which public metadata does not prove.",
		"Estimated RPM range is adjusted for inferred topic, format, duration, and uncertainty.",
		"Likely topic: " + firstNonEmpty(niche.SpecificTopic, niche.Niche, niche.PrimaryNiche, categoryName(item.Snippet.CategoryID)),
	}
}

func monetisationPotentialScore(niche NicheAnalysis, format string, views *uint64) int {
	low, high := rpmRange(niche, format)
	score := int((low+high)/2*8) + 35
	if views != nil && *views >= 100000 {
		score += 8
	}
	if format == "short_form" {
		score -= 15
	}
	return clampInt(score)
}

func analysisConfidenceScore(item youtubeVideoItem, kw KeywordIntelligence, niche NicheAnalysis, views, likes, comments *uint64) int {
	score := 25
	if strings.TrimSpace(item.Snippet.Title) != "" {
		score += 12
	}
	if len(tokensFromText(cleanMetadataText(item.Snippet.Description))) >= 20 {
		score += 10
	}
	if len(item.Snippet.Tags) > 0 {
		score += 8
	}
	if len(kw.PrimaryTopics) >= 2 && len(kw.SearchPhrases) >= 2 {
		score += 12
	}
	if niche.Confidence >= 0.7 {
		score += 10
	}
	if item.ContentDetails.Duration != "" {
		score += 6
	}
	if views != nil {
		score += 5
	}
	if likes != nil {
		score += 4
	}
	if comments != nil {
		score += 4
	}
	score -= 8 // no transcript evidence in this provider path
	return clampInt(score)
}

func formatMoneyRange(low, high float64) string {
	return fmt.Sprintf("$%.0f-$%.0f", math.Round(low), math.Round(high))
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

func scoreViewsPerDay(v float64) int {
	switch {
	case v >= 100000:
		return 100
	case v >= 10000:
		return 80
	case v >= 1000:
		return 62
	case v >= 100:
		return 42
	default:
		return 24
	}
}

func scoreRate(v, low, high float64) int {
	if high <= low {
		return 0
	}
	return clampInt(int((v - low) / (high - low) * 100))
}

func asFloat(v any) (float64, bool) {
	switch typed := v.(type) {
	case float64:
		return typed, !math.IsNaN(typed) && !math.IsInf(typed, 0)
	case int:
		return float64(typed), true
	default:
		return 0, false
	}
}

func formatFloat(v float64, precision int) string {
	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, v)
}

func clampInt(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
