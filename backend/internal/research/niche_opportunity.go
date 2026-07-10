package research

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
)

const publicMonetizationLimitation = "Public YouTube data cannot reveal exact RPM or private traffic sources. Monetization estimates are based on public metadata, keyword/ad-market proxies, and category heuristics."

type NicheOpportunityRequest struct {
	SeedKeyword                    string `json:"seed_keyword"`
	Platform                       string `json:"platform"`
	Country                        string `json:"country"`
	Language                       string `json:"language"`
	Audience                       string `json:"audience"`
	ContentStyle                   string `json:"content_style"`
	MonetizationGoal               string `json:"monetization_goal"`
	CreatorSkillLevel              string `json:"creator_skill_level"`
	ProductionDifficultyPreference string `json:"production_difficulty_preference"`
}

type NicheOpportunityResponse struct {
	Status         string             `json:"status"`
	Message        string             `json:"message"`
	ProviderStatus []ProviderStatus   `json:"provider_status"`
	Opportunities  []NicheOpportunity `json:"opportunities"`
	Limitations    []string           `json:"limitations"`
}

type NicheOpportunity struct {
	NicheName                  string                `json:"niche_name"`
	Platform                   string                `json:"platform"`
	Country                    string                `json:"country"`
	Language                   string                `json:"language"`
	Audience                   string                `json:"audience"`
	ContentStyle               string                `json:"content_style"`
	DemandScore                float64               `json:"demand_score"`
	MonetizationScore          float64               `json:"monetization_score"`
	MonetizationConfidence     string                `json:"monetization_confidence"`
	CompetitionScore           float64               `json:"competition_score"`
	CreatorFitScore            float64               `json:"creator_fit_score"`
	SuccessProbabilityScore    float64               `json:"success_probability_score"`
	SuccessProbability         string                `json:"success_probability"`
	OpportunityScore           float64               `json:"opportunity_score"`
	Confidence                 float64               `json:"confidence"`
	EvidenceSources            []string              `json:"evidence_sources"`
	SupportingKeywords         []string              `json:"supporting_keywords"`
	RelatedChannels            []string              `json:"related_channels"`
	RelatedVideos              []ChannelVideoSummary `json:"related_videos"`
	EstimatedMonetizationLevel string                `json:"estimated_monetization_level"`
	MonetizationReason         string                `json:"monetization_reason"`
	CompetitionLevel           string                `json:"competition_level"`
	CompetitionReason          string                `json:"competition_reason"`
	DemandReason               string                `json:"demand_reason"`
	SuccessReason              string                `json:"success_reason"`
	Risks                      []string              `json:"risks"`
	First10VideoIdeas          []string              `json:"first_10_video_ideas"`
	SuggestedKeywords          []string              `json:"suggested_keywords"`
	SuggestedTitles            []string              `json:"suggested_titles"`
	SuggestedClipAngles        []string              `json:"suggested_clip_angles"`
	Limitations                []string              `json:"limitations"`
}

type OpportunityScoreInput struct {
	DemandScore       float64
	MonetizationScore float64
	CompetitionScore  float64
	CreatorFitScore   float64
}

func CalculateOpportunityScore(in OpportunityScoreInput) float64 {
	score := clampScore(in.DemandScore)*0.35 +
		clampScore(in.MonetizationScore)*0.30 +
		(100-clampScore(in.CompetitionScore))*0.25 +
		clampScore(in.CreatorFitScore)*0.10
	return round1(score)
}

func AnalyzeNicheOpportunities(ctx context.Context, yt *YouTubeProvider, req NicheOpportunityRequest, googleAdsStatus ProviderStatus) NicheOpportunityResponse {
	req = normalizeNicheOpportunityRequest(req)
	statuses := []ProviderStatus{yt.Status(), googleAdsStatus, YouTubeAnalyticsStatus()}
	limitations := []string{
		publicMonetizationLimitation,
		"Estimated from public/proxy signals, not exact YouTube RPM.",
	}
	if req.SeedKeyword == "" {
		return NicheOpportunityResponse{Status: "invalid_input", Message: "seed_keyword is required", ProviderStatus: statuses, Limitations: limitations}
	}
	if yt.apiKey == "" {
		return NicheOpportunityResponse{
			Status:         StatusNotConfigured,
			Message:        "YouTube Data API is required to evaluate real creator demand and competition.",
			ProviderStatus: statuses,
			Limitations:    append(limitations, "Connect YouTube Data API before Niche Finder can compare real public videos and channels."),
		}
	}

	var opportunities []NicheOpportunity
	for _, idea := range groupedNicheIdeas(req) {
		videos, err := yt.SearchVideos(ctx, idea.Query, req.Country, req.Language, 25)
		if err != nil {
			continue
		}
		if len(videos) == 0 {
			continue
		}
		opportunities = append(opportunities, buildOpportunity(req, idea, videos, googleAdsStatus))
	}
	sort.SliceStable(opportunities, func(i, j int) bool {
		return opportunities[i].OpportunityScore > opportunities[j].OpportunityScore
	})
	if len(opportunities) > 5 {
		opportunities = opportunities[:5]
	}
	if len(opportunities) == 0 {
		return NicheOpportunityResponse{
			Status:         "insufficient_data",
			Message:        "Not enough public YouTube metadata was returned to evaluate grouped niche opportunities for this seed.",
			ProviderStatus: statuses,
			Limitations:    limitations,
		}
	}
	return NicheOpportunityResponse{
		Status:         StatusOK,
		Message:        "Evaluated grouped creator opportunities using public YouTube metadata and proxy monetization signals.",
		ProviderStatus: statuses,
		Opportunities:  opportunities,
		Limitations:    limitations,
	}
}

type nicheIdea struct {
	Name  string
	Query string
	Tags  []string
}

func groupedNicheIdeas(req NicheOpportunityRequest) []nicheIdea {
	seed := strings.TrimSpace(req.SeedKeyword)
	audience := strings.TrimSpace(req.Audience)
	if audience == "" || strings.EqualFold(audience, "global") {
		audience = countryAudience(req.Country)
	}
	style := strings.TrimSpace(req.ContentStyle)
	if style == "" {
		style = "short-form explainers"
	}
	base := normalizeNicheName(seed, audience, style)
	return uniqueNicheIdeas([]nicheIdea{
		{Name: base, Query: seed + " " + audience + " " + style, Tags: []string{seed, audience, style}},
		{Name: titleCase(seed) + " explainers for " + audience, Query: seed + " explained " + audience, Tags: []string{seed, "explainers", audience}},
		{Name: "Beginner " + seed + " education for " + audience, Query: "beginner " + seed + " tutorial " + audience, Tags: []string{seed, "beginner", "education"}},
		{Name: titleCase(seed) + " tools, reviews, and buying guides", Query: seed + " tools review best buying guide", Tags: []string{seed, "tools", "reviews", "buying"}},
		{Name: titleCase(seed) + " news and trend breakdowns", Query: seed + " news explained trend breakdown", Tags: []string{seed, "news", "breakdown"}},
	})
}

func normalizeNicheName(seed, audience, style string) string {
	cleanSeed := strings.TrimSpace(seed)
	if cleanSeed == "" {
		cleanSeed = "creator topic"
	}
	switch {
	case strings.Contains(strings.ToLower(cleanSeed), "ai"):
		return "AI tools for " + strings.ToLower(audience) + " creators"
	case strings.Contains(strings.ToLower(cleanSeed), "visa"):
		return titleCase(cleanSeed) + " explainers for " + audience
	case strings.Contains(strings.ToLower(cleanSeed), "football"):
		return "Football transfer explainers for " + audience
	case strings.Contains(strings.ToLower(cleanSeed), "movie") || strings.Contains(strings.ToLower(cleanSeed), "film"):
		return "Movie scene breakdowns for " + audience
	default:
		return titleCase(cleanSeed) + " " + compactStyle(style) + " for " + audience
	}
}

func buildOpportunity(req NicheOpportunityRequest, idea nicheIdea, videos []ChannelVideoSummary, googleAdsStatus ProviderStatus) NicheOpportunity {
	demand, demandReason := scoreDemand(videos, req)
	monetization, level, confidence, monetizationReason := scoreMonetization(idea, req, googleAdsStatus)
	competition, competitionLevel, competitionReason := scoreCompetition(videos)
	creatorFit := scoreCreatorFit(req, idea)
	opportunity := CalculateOpportunityScore(OpportunityScoreInput{DemandScore: demand, MonetizationScore: monetization, CompetitionScore: competition, CreatorFitScore: creatorFit})
	successScore, successLabel, successReason := scoreSuccessProbability(demand, monetization, competition, creatorFit, req)
	keywords := unique(append(append([]string{}, idea.Tags...), extractTermsFromTitles(videos)...))
	videoIdeas := first10VideoIdeas(idea.Name, keywords, req)
	return NicheOpportunity{
		NicheName:                  idea.Name,
		Platform:                   req.Platform,
		Country:                    req.Country,
		Language:                   req.Language,
		Audience:                   req.Audience,
		ContentStyle:               req.ContentStyle,
		DemandScore:                demand,
		MonetizationScore:          monetization,
		MonetizationConfidence:     confidence,
		CompetitionScore:           competition,
		CreatorFitScore:            creatorFit,
		SuccessProbabilityScore:    successScore,
		SuccessProbability:         successLabel,
		OpportunityScore:           opportunity,
		Confidence:                 confidenceScore(videos, googleAdsStatus),
		EvidenceSources:            evidenceSources(googleAdsStatus),
		SupportingKeywords:         topN(keywords, 12),
		RelatedChannels:            topN(channelNames(videos), 8),
		RelatedVideos:              topNChannelVideos(videos, 8),
		EstimatedMonetizationLevel: level,
		MonetizationReason:         monetizationReason,
		CompetitionLevel:           competitionLevel,
		CompetitionReason:          competitionReason,
		DemandReason:               demandReason,
		SuccessReason:              successReason,
		Risks:                      opportunityRisks(req, demand, monetization, competition),
		First10VideoIdeas:          videoIdeas,
		SuggestedKeywords:          topN(keywords, 10),
		SuggestedTitles:            suggestedTitles(idea.Name, keywords),
		SuggestedClipAngles:        suggestedClipAngles(idea.Name, req),
		Limitations: []string{
			publicMonetizationLimitation,
			"Estimated from public/proxy signals, not exact YouTube RPM.",
		},
	}
}

func scoreDemand(videos []ChannelVideoSummary, req NicheOpportunityRequest) (float64, string) {
	recent := 0
	totalViews := uint64(0)
	engaged := 0
	now := time.Now().UTC()
	for _, video := range videos {
		if video.Views != nil {
			totalViews += *video.Views
			if *video.Views >= 50000 {
				engaged++
			}
		}
		if t, err := time.Parse(time.RFC3339, video.PublishedAt); err == nil && now.Sub(t) <= 90*24*time.Hour {
			recent++
		}
	}
	avgViews := float64(totalViews) / math.Max(1, float64(len(videos)))
	volumeScore := minFloat(35, float64(len(videos))*1.4)
	recencyScore := minFloat(25, float64(recent)*2.2)
	viewScore := minFloat(30, math.Log10(avgViews+10)*8)
	engagementScore := minFloat(10, float64(engaged)*1.5)
	score := round1(volumeScore + recencyScore + viewScore + engagementScore)
	return score, "Demand is estimated from YouTube search result volume, recent uploads, visible view counts, engagement availability, and region/language relevance."
}

func scoreMonetization(idea nicheIdea, req NicheOpportunityRequest, googleAdsStatus ProviderStatus) (float64, string, string, string) {
	haystack := strings.ToLower(strings.Join(append(idea.Tags, req.SeedKeyword, req.MonetizationGoal), " "))
	score := 38.0
	reasons := []string{"Estimated from public/proxy signals, not exact YouTube RPM."}
	highIntent := []string{"software", "saas", "finance", "insurance", "business", "trading", "legal", "health", "education", "real estate", "jobs", "tools", "reviews", "buying", "best", "course"}
	for _, term := range highIntent {
		if strings.Contains(haystack, term) {
			score += 7
			reasons = append(reasons, "contains commercial-intent term: "+term)
		}
	}
	if strings.Contains(haystack, "finance") || strings.Contains(haystack, "trading") || strings.Contains(haystack, "insurance") || strings.Contains(haystack, "software") || strings.Contains(haystack, "saas") {
		score += 12
		reasons = append(reasons, "category tends to attract higher advertiser intent")
	}
	if strings.Contains(haystack, "meme") || strings.Contains(haystack, "entertainment") || strings.Contains(haystack, "movie") || strings.Contains(haystack, "football") {
		score -= 10
		reasons = append(reasons, "entertainment-led niches often rely more on scale than high advertiser intent")
	}
	if strings.EqualFold(req.Country, "US") || strings.EqualFold(req.Country, "GB") {
		score += 8
		reasons = append(reasons, "target country usually has stronger advertiser buying power")
	}
	confidence := "low"
	if googleAdsStatus.Status == StatusActive {
		confidence = "medium"
		reasons = append(reasons, "Google Ads Keyword Planner can improve this estimate when keyword metrics are fetched")
	} else {
		reasons = append(reasons, "Google Ads Keyword Planner is not configured, so no CPC, bid, or search-volume data is used")
	}
	score = round1(clampScore(score))
	return score, monetizationLevel(score), confidence, strings.Join(reasons, " ")
}

func scoreCompetition(videos []ChannelVideoSummary) (float64, string, string) {
	if len(videos) == 0 {
		return 100, "saturated", "No comparable videos were available, so competition is treated as high uncertainty."
	}
	channelCounts := map[string]int{}
	topViewDominance := 0
	highViewVideos := 0
	for _, video := range videos {
		key := firstNonEmpty(video.ChannelID, video.ChannelTitle)
		if key != "" {
			channelCounts[key]++
		}
		if video.Views != nil {
			if *video.Views >= 1000000 {
				topViewDominance++
			}
			if *video.Views >= 100000 {
				highViewVideos++
			}
		}
	}
	maxByChannel := 0
	for _, count := range channelCounts {
		if count > maxByChannel {
			maxByChannel = count
		}
	}
	score := float64(len(videos))*1.2 + float64(highViewVideos)*2.4 + float64(topViewDominance)*4 + float64(maxByChannel)*3
	score = round1(clampScore(score))
	level := "low"
	switch {
	case score >= 75:
		level = "saturated"
	case score >= 55:
		level = "high"
	case score >= 35:
		level = "medium"
	}
	return score, level, "Competition is estimated from similar videos found, visible view concentration, repeat channel presence, and how dominated top results look for a new creator."
}

func scoreCreatorFit(req NicheOpportunityRequest, idea nicheIdea) float64 {
	score := 60.0
	skill := strings.ToLower(req.CreatorSkillLevel)
	difficulty := strings.ToLower(req.ProductionDifficultyPreference)
	if strings.Contains(skill, "advanced") || strings.Contains(skill, "expert") {
		score += 12
	}
	if strings.Contains(skill, "beginner") && strings.Contains(strings.ToLower(strings.Join(idea.Tags, " ")), "beginner") {
		score += 10
	}
	if strings.Contains(difficulty, "low") || strings.Contains(difficulty, "simple") {
		score += 8
	}
	if strings.Contains(strings.ToLower(req.ContentStyle), "short") {
		score += 8
	}
	return round1(clampScore(score))
}

func scoreSuccessProbability(demand, monetization, competition, fit float64, req NicheOpportunityRequest) (float64, string, string) {
	score := round1(clampScore(demand*0.32 + monetization*0.20 + (100-competition)*0.28 + fit*0.20))
	label := "low"
	switch {
	case score >= 78:
		label = "strong"
	case score >= 64:
		label = "promising"
	case score >= 45:
		label = "possible"
	}
	reason := "Success probability considers demand, competition saturation, production difficulty, audience clarity, and short-form suitability."
	if demand >= 70 && competition >= 70 {
		reason = "High demand but saturated; success depends on a sharper angle, consistency, and stronger packaging."
	} else if demand < 55 && competition < 40 {
		reason = "Lower demand but low competition; this can work with a specific audience and repeatable format."
	} else if monetization >= 70 && fit < 60 {
		reason = "High monetization potential, but it likely requires expertise or higher production quality."
	} else if strings.Contains(strings.ToLower(req.ContentStyle), "short") && competition < 65 {
		reason = "Good short-form opportunity because demand exists and competition is not fully saturated."
	}
	return score, label, reason
}

func normalizeNicheOpportunityRequest(req NicheOpportunityRequest) NicheOpportunityRequest {
	req.SeedKeyword = strings.TrimSpace(req.SeedKeyword)
	req.Platform = firstNonEmpty(strings.TrimSpace(req.Platform), "youtube")
	req.Country = firstNonEmpty(strings.TrimSpace(req.Country), "US")
	req.Language = firstNonEmpty(strings.TrimSpace(req.Language), "en-US")
	req.Audience = firstNonEmpty(strings.TrimSpace(req.Audience), "Global")
	req.ContentStyle = firstNonEmpty(strings.TrimSpace(req.ContentStyle), "short-form explainers")
	req.MonetizationGoal = firstNonEmpty(strings.TrimSpace(req.MonetizationGoal), "ads, affiliates, and products")
	req.CreatorSkillLevel = firstNonEmpty(strings.TrimSpace(req.CreatorSkillLevel), "intermediate")
	req.ProductionDifficultyPreference = firstNonEmpty(strings.TrimSpace(req.ProductionDifficultyPreference), "medium")
	return req
}

func monetizationLevel(score float64) string {
	switch {
	case score >= 82:
		return "very_high"
	case score >= 65:
		return "high"
	case score >= 42:
		return "medium"
	default:
		return "low"
	}
}

func confidenceScore(videos []ChannelVideoSummary, googleAdsStatus ProviderStatus) float64 {
	base := 0.45 + minFloat(0.25, float64(len(videos))/100)
	if googleAdsStatus.Status == StatusActive {
		base += 0.15
	}
	return round1(minFloat(base, 0.78))
}

func evidenceSources(googleAdsStatus ProviderStatus) []string {
	sources := []string{"YouTube Data API public search/video metadata", "category and commercial-intent heuristics"}
	if googleAdsStatus.Status == StatusActive {
		sources = append(sources, "Google Ads Keyword Planner proxy metrics")
	} else {
		sources = append(sources, "Google Ads Keyword Planner not configured")
	}
	return sources
}

func first10VideoIdeas(niche string, keywords []string, req NicheOpportunityRequest) []string {
	core := firstNonEmpty(firstString(keywords), req.SeedKeyword, niche)
	ideas := []string{
		"Start here: what " + core + " means for " + req.Audience,
		"3 mistakes beginners make with " + core,
		"Before you try " + core + ", watch this",
		"Best free tools for " + core,
		"What creators get wrong about " + niche,
		"Case study: a simple " + core + " workflow",
		"Do this for 7 days in " + niche,
		"High-demand questions about " + core + " answered",
		"Cheap vs expensive options in " + niche,
		"The 30-second checklist for " + core,
	}
	return ideas
}

func suggestedTitles(niche string, keywords []string) []string {
	core := firstNonEmpty(firstString(keywords), niche)
	return []string{
		"I Tested " + titleCase(core) + " So You Don't Have To",
		"The Beginner Guide to " + titleCase(niche),
		"Nobody Explains " + titleCase(core) + " Like This",
		"Best " + titleCase(core) + " Tools Ranked",
		"Stop Making This " + titleCase(core) + " Mistake",
	}
}

func suggestedClipAngles(niche string, req NicheOpportunityRequest) []string {
	return []string{
		"Open with a concrete viewer problem, then show the simplest fix.",
		"Compare a beginner path against a creator/pro path.",
		"Use a current example from public YouTube results as the setup.",
		"Localize the hook for " + req.Audience + " in " + req.Country + ".",
		"End with a checklist viewers can save.",
	}
}

func opportunityRisks(req NicheOpportunityRequest, demand, monetization, competition float64) []string {
	risks := []string{}
	if competition >= 65 {
		risks = append(risks, "Top results look competitive; a new creator needs a clearer audience promise.")
	}
	if monetization >= 70 {
		risks = append(risks, "Higher monetization niches may require expertise, compliance care, and stronger trust signals.")
	}
	if demand < 50 {
		risks = append(risks, "Demand signal is modest from public YouTube metadata.")
	}
	if strings.Contains(strings.ToLower(req.SeedKeyword), "health") || strings.Contains(strings.ToLower(req.SeedKeyword), "finance") || strings.Contains(strings.ToLower(req.SeedKeyword), "trading") {
		risks = append(risks, "Avoid personalized financial, legal, or medical claims unless properly qualified.")
	}
	if len(risks) == 0 {
		risks = append(risks, "Public metadata can miss private retention, traffic-source, and monetization realities.")
	}
	return risks
}

func extractTermsFromTitles(videos []ChannelVideoSummary) []string {
	terms := []string{}
	for _, video := range videos {
		kw := ExtractKeywordIntelligence(KeywordExtractionInput{Title: video.Title})
		terms = append(terms, kw.PrimaryKeywords...)
		terms = append(terms, kw.SecondaryKeywords...)
	}
	return unique(terms)
}

func channelNames(videos []ChannelVideoSummary) []string {
	names := []string{}
	for _, video := range videos {
		if strings.TrimSpace(video.ChannelTitle) != "" {
			names = append(names, video.ChannelTitle)
		}
	}
	return unique(names)
}

func uniqueNicheIdeas(ideas []nicheIdea) []nicheIdea {
	seen := map[string]bool{}
	out := []nicheIdea{}
	for _, idea := range ideas {
		name := strings.TrimSpace(idea.Name)
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		idea.Name = name
		out = append(out, idea)
		seen[strings.ToLower(name)] = true
	}
	return out
}

func compactStyle(style string) string {
	style = strings.ToLower(strings.TrimSpace(style))
	switch {
	case strings.Contains(style, "tutorial"):
		return "tutorials"
	case strings.Contains(style, "review"):
		return "reviews"
	case strings.Contains(style, "breakdown"):
		return "breakdowns"
	case strings.Contains(style, "short"):
		return "short-form explainers"
	default:
		return "explainers"
	}
}

func countryAudience(country string) string {
	switch strings.ToUpper(strings.TrimSpace(country)) {
	case "GB", "UK":
		return "UK viewers"
	case "PK":
		return "Pakistani viewers"
	case "IN":
		return "Indian viewers"
	default:
		return "global viewers"
	}
}

func firstString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func clampScore(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
