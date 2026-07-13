package research

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

type KeywordExtractionInput struct {
	Title             string
	Description       string
	Tags              []string
	ChannelTitle      string
	Category          string
	TopicDetails      []string
	RecentVideoTitles []string
}

func ExtractKeywordIntelligence(in KeywordExtractionInput) KeywordIntelligence {
	type termScore struct {
		term  string
		score float64
	}
	scores := map[string]float64{}
	rejected := map[string]bool{}
	addText := func(text string, weight float64, phrases bool) {
		tokens := tokenizeUseful(cleanMetadataText(text), rejected)
		for _, token := range tokens {
			scores[token] += weight
		}
		if !phrases {
			return
		}
		for n := 2; n <= 4; n++ {
			for _, phrase := range ngrams(tokens, n) {
				scores[phrase] += weight * float64(n) * 0.9
			}
		}
	}
	titleWeight := 6.5
	if len(in.RecentVideoTitles) > 0 {
		titleWeight = 0.8
	}
	addText(in.Title, titleWeight, true)
	addText(in.ChannelTitle, 0.7, false)
	descParts := splitDescriptionForWeighting(in.Description)
	if descParts[0] != "" {
		addText(descParts[0], 1.2, true)
	}
	if descParts[1] != "" {
		addText(descParts[1], 0.25, false)
	}
	for _, tag := range in.Tags {
		addText(tag, 3.5, true)
		if strings.HasPrefix(strings.TrimSpace(tag), "#") {
			scores[strings.ToLower(strings.TrimSpace(tag))] += 4
		}
	}
	for i, title := range in.RecentVideoTitles {
		weight := 2.2
		if i < 5 {
			weight = 3
		}
		addText(title, weight, true)
	}
	addText(categoryName(in.Category), 0.5, false)
	for _, topic := range in.TopicDetails {
		addText(topicLastSegment(topic), 0.6, false)
	}

	hashSeen := map[string]bool{}
	hashtags := []string{}
	for _, h := range regexp.MustCompile(`#([A-Za-z][A-Za-z0-9_]{1,40})`).FindAllString(in.Description+" "+strings.Join(in.Tags, " "), -1) {
		h = strings.ToLower(normalizeCamelArtifact(h))
		if !hashSeen[h] {
			hashSeen[h] = true
			hashtags = append(hashtags, h)
		}
	}

	items := make([]termScore, 0, len(scores))
	for term, score := range scores {
		if !isUsefulTerm(term) {
			rejected[term] = true
			continue
		}
		items = append(items, termScore{term: term, score: score})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			return len(items[i].term) > len(items[j].term)
		}
		return items[i].score > items[j].score
	})

	primary := []string{}
	secondary := []string{}
	longTail := []string{}
	seen := map[string]bool{}
	for _, item := range items {
		if seen[item.term] || nearDuplicateSelected(item.term, append(append(primary, secondary...), longTail...)) {
			continue
		}
		seen[item.term] = true
		words := strings.Fields(item.term)
		switch {
		case len(words) >= 3 && isNaturalSearchPhrase(item.term) && len(longTail) < 8:
			longTail = append(longTail, item.term)
		case (len(words) >= 2 || item.score >= 3) && len(primary) < 6:
			primary = append(primary, item.term)
		case len(words) <= 3 && len(secondary) < 10:
			secondary = append(secondary, item.term)
		}
		if len(primary) >= 6 && len(secondary) >= 10 && len(longTail) >= 8 {
			break
		}
	}
	if len(longTail) < 5 {
		for _, item := range items {
			if len(strings.Fields(item.term)) >= 2 && isNaturalSearchPhrase(item.term) && !containsString(longTail, item.term) && len(longTail) < 8 {
				longTail = append(longTail, item.term)
			}
		}
	}
	if len(primary) == 0 {
		for _, phrase := range longTail {
			if len(primary) >= 6 {
				break
			}
			primary = append(primary, phrase)
		}
	}

	metadataScore := 35
	if strings.TrimSpace(in.Title) != "" {
		metadataScore += 20
	}
	if len(in.Tags) > 0 {
		metadataScore += 15
	}
	if len(tokensFromText(in.Description)) >= 30 {
		metadataScore += 15
	}
	if len(primary)+len(longTail) >= 10 {
		metadataScore += 15
	}
	if metadataScore > 100 {
		metadataScore = 100
	}
	return KeywordIntelligence{
		PrimaryKeywords:       primary,
		SecondaryKeywords:     secondary,
		LongTailPhrases:       longTail,
		PrimaryTopics:         primary,
		SupportingTerms:       secondary,
		SearchPhrases:         longTail,
		Hashtags:              hashtags,
		RejectedNoiseTerms:    mapKeys(rejected, 16),
		InferredSearchIntent:  inferSearchIntent(in.Title, primary, longTail),
		MetadataStrengthScore: metadataScore,
	}
}

func ClassifyNiche(in KeywordExtractionInput, kw KeywordIntelligence) NicheAnalysis {
	haystack := strings.ToLower(strings.Join(append(append([]string{in.Title, in.Description, in.ChannelTitle, categoryName(in.Category)}, in.Tags...), append(in.TopicDetails, append(kw.PrimaryKeywords, kw.LongTailPhrases...)...)...), " "))
	rules := []struct {
		broad string
		niche string
		terms []string
	}{
		{"Software education", "AI-assisted development", []string{"codex", "chatgpt", "openai", "coding", "developer", "agent", "llm", "prompt", "automation", "install"}},
		{"Technology", "Consumer technology", []string{"software", "iphone", "android", "review", "tech", "app", "camera", "laptop", "tesla"}},
		{"Business and finance", "Finance education", []string{"money", "finance", "business", "startup", "investing", "stock", "crypto", "sales", "marketing"}},
		{"Education", "Practical tutorial", []string{"how to", "tutorial", "learn", "course", "guide", "explained", "beginner", "lesson"}},
		{"Gaming", "Gaming guide or entertainment", []string{"game", "gaming", "minecraft", "roblox", "fortnite", "ps5", "xbox", "nintendo"}},
		{"Entertainment", "Film and animation", []string{"movie", "film", "animation", "anime", "trailer", "scene", "disney", "pixar"}},
		{"Sports", "Sports analysis or training", []string{"football", "soccer", "nba", "nfl", "cricket", "match", "goal", "training"}},
		{"Health", "Fitness and wellness", []string{"fitness", "workout", "health", "diet", "weight loss", "gym", "muscle"}},
		{"Lifestyle", "Beauty and fashion", []string{"makeup", "beauty", "fashion", "outfit", "skincare", "style"}},
		{"Food", "Cooking and recipes", []string{"recipe", "food", "cooking", "restaurant", "meal", "kitchen", "grill"}},
		{"Travel", "Travel and places", []string{"travel", "hotel", "flight", "city", "country", "tour", "trip"}},
		{"Entertainment", "Comedy and skits", []string{"funny", "comedy", "meme", "prank", "joke", "skit"}},
		{"News", "News and politics", []string{"news", "election", "politics", "government", "breaking", "president"}},
		{"Lifestyle", "Personal lifestyle", []string{"lifestyle", "family", "daily", "vlog", "culture", "home"}},
		{"Entertainment", "General entertainment", []string{"entertainment", "celebrity", "viral", "reaction", "challenge", "story"}},
	}
	bestBroad := "General"
	bestNiche := "General public video"
	bestScore := 0
	evidence := []string{}
	for _, rule := range rules {
		score := 0
		found := []string{}
		for _, term := range rule.terms {
			if strings.Contains(haystack, term) {
				score++
				found = append(found, term)
			}
		}
		if score > bestScore {
			bestScore = score
			bestBroad = rule.broad
			bestNiche = rule.niche
			evidence = found
		}
	}
	if bestScore == 0 && len(kw.PrimaryKeywords) > 0 {
		evidence = topN(kw.PrimaryKeywords, 4)
	}
	if broad, niche, ok := categoryNicheOverride(in.Category, haystack); ok {
		bestBroad = broad
		bestNiche = niche
		if len(evidence) == 0 {
			evidence = []string{categoryName(in.Category)}
		}
	}
	format := detectContentFormat(in.Title + " " + in.Description)
	specificTopic := grammaticallyMeaningfulTopic(kw, in.Title, bestNiche)
	sub := specificTopic
	confidence := 0.42 + float64(bestScore)*0.09
	if confidence > 0.9 {
		confidence = 0.9
	}
	return NicheAnalysis{
		BroadCategory:        bestBroad,
		PrimaryNiche:         bestNiche,
		Niche:                bestNiche,
		SubNiche:             sub,
		SpecificTopic:        specificTopic,
		AudienceType:         inferAudienceType(bestNiche, haystack),
		ContentFormat:        format,
		Confidence:           confidence,
		EvidenceTerms:        unique(topN(append(evidence, kw.PrimaryKeywords...), 8)),
		TargetAudience:       inferAudienceType(bestNiche, haystack),
		InferredContentAngle: inferContentAngle(format, sub, bestNiche),
	}
}

func AnalyzeHookIntelligence(title string) HookIntelligence {
	lower := strings.ToLower(title)
	hookType := "story"
	switch {
	case strings.Contains(lower, "how to") || strings.HasPrefix(lower, "how "):
		hookType = "how_to"
	case regexp.MustCompile(`\b\d+\b`).MatchString(title):
		hookType = "list"
	case strings.Contains(lower, "reaction") || strings.Contains(lower, "react"):
		hookType = "reaction"
	case strings.Contains(title, "?") || strings.Contains(lower, "secret") || strings.Contains(lower, "hidden"):
		hookType = "curiosity"
	case strings.Contains(lower, "challenge"):
		hookType = "challenge"
	case strings.Contains(lower, " vs ") || strings.Contains(lower, " versus ") || strings.Contains(lower, "better"):
		hookType = "comparison"
	case strings.Contains(lower, "news") || strings.Contains(lower, "breaking") || strings.Contains(lower, "2026"):
		hookType = "news"
	case strings.Contains(lower, "compilation") || strings.Contains(lower, "best moments"):
		hookType = "compilation"
	case strings.Contains(lower, "review"):
		hookType = "review"
	}
	words := strings.Fields(title)
	triggers := []string{}
	for _, trigger := range []string{"best", "worst", "secret", "hidden", "new", "first", "last", "mistake", "easy", "fast", "shocking"} {
		if strings.Contains(lower, trigger) {
			triggers = append(triggers, trigger)
		}
	}
	clarity := 60
	if len(words) >= 4 && len(words) <= 12 {
		clarity += 20
	}
	if strings.Contains(title, "|") || strings.Contains(title, ":") {
		clarity += 8
	}
	specificity := 45
	if regexp.MustCompile(`\b[A-Z][A-Za-z0-9]+\b`).MatchString(title) || regexp.MustCompile(`\b\d+\b`).MatchString(title) {
		specificity += 18
	}
	if len(words) >= 5 {
		specificity += 12
	}
	curiosity := 45 + len(triggers)*8
	if strings.Contains(title, "?") {
		curiosity += 15
	}
	if curiosity > 100 {
		curiosity = 100
	}
	audienceSignal := 42
	if containsAny([]string{lower}, []string{"beginners", "creators", "developers", "parents", "students", "gamers", "investors", "owners"}) {
		audienceSignal += 28
	}
	valuePromise := 48
	if containsAny([]string{lower}, []string{"how", "guide", "learn", "fix", "build", "make", "review", "explained", "setup"}) {
		valuePromise += 24
	}
	clarity = clampInt(clarity)
	specificity = clampInt(specificity)
	audienceSignal = clampInt(audienceSignal)
	valuePromise = clampInt(valuePromise)
	remake := (clarity + curiosity + specificity + valuePromise) / 4
	if hookType == "how_to" || hookType == "list" || hookType == "comparison" {
		remake += 8
	}
	if remake > 100 {
		remake = 100
	}
	return HookIntelligence{
		HookType:             hookType,
		TitleLength:          len([]rune(title)),
		TitlePattern:         titlePattern(title),
		EmotionalTriggers:    triggers,
		ClarityScore:         clarity,
		SpecificityScore:     specificity,
		CuriosityScore:       curiosity,
		AudienceSignalScore:  audienceSignal,
		ValuePromiseScore:    valuePromise,
		RemakePotentialScore: remake,
		Explanation:          explainHookScore(title, hookType, clarity, specificity, curiosity, audienceSignal, valuePromise),
	}
}

func BuildCreatorOpportunities(niche NicheAnalysis, kw KeywordIntelligence, hook HookIntelligence, title string) CreatorOpportunities {
	core := firstNonEmpty(niche.SpecificTopic, firstPhrase(kw.PrimaryTopics), firstPhrase(kw.PrimaryKeywords), "the topic")
	format := strings.ReplaceAll(hook.HookType, "_", " ")
	audience := firstNonEmpty(niche.TargetAudience, niche.AudienceType, "the likely audience")
	return CreatorOpportunities{
		SuggestedRemakeAngles: []string{
			"Checklist angle - change the format into a step-by-step " + format + " for " + audience + "; public evidence basis: title/topic terms around " + core + "; confidence: medium.",
			"Comparison angle - keep the source premise but compare " + core + " with a close alternative; evidence basis: cleaned search phrases; confidence: medium.",
			"Beginner-to-advanced angle - preserve the topic but make the difficulty level explicit for " + audience + "; evidence basis: inferred audience and title structure; confidence: medium.",
			"Time-boxed workflow angle - turn " + core + " into a concise setup or decision workflow; evidence basis: metadata hook type and topic clarity; confidence: medium.",
			"Contrarian angle - challenge a common assumption about " + core + " without claiming private performance data; evidence basis: public title and description terms; confidence: low-medium.",
		},
		TitleIdeas: []string{
			"How to Approach " + titleCase(core) + " Without Wasting Time",
			titleCase(core) + ": A Practical Guide for " + titleCase(audience),
			"Before You Try " + titleCase(core) + ", Check These Steps",
			"The " + titleCase(core) + " Workflow I Would Use Today",
			"Is " + titleCase(core) + " Still Worth It? A Metadata-Based Breakdown",
		},
		ShortFormClipIdeas: []string{
			"Adaptation idea: one clear takeaway from the public title promise about " + core + ".",
			"Cut a 20-second myth-versus-reality version around " + core + ".",
			"Adaptation idea: three-step checklist viewers can screenshot.",
			"Adaptation idea: compare the source premise with one realistic alternative.",
			"Adaptation idea: turn one cleaned search phrase into a single-question short.",
		},
		ScriptPrompts: []string{
			"Write a 30-second " + format + " script for " + audience + " about " + core + ", grounded only in public metadata.",
			"Write a 60-second tutorial script that preserves the source premise from the title \"" + title + "\" and teaches one practical takeaway.",
			"Write a short adaptation using these inferred public topics: " + strings.Join(topN(kw.PrimaryTopics, 4), ", ") + ". Do not mention private analytics or transcript-only moments.",
			"Write a comparison script around " + core + " that states the analysis is based on public metadata.",
			"Write a concise hook and outline for a truthful remake of: " + title,
		},
		ContentGaps:         []string{"Beginner explainer", "Comparison angle", "Mistake-led breakdown"},
		UnderusedTopics:     topN(append(kw.SecondaryKeywords, kw.LongTailPhrases...), 5),
		LocalizationOptions: []string{"Local examples for target country", "Bilingual hook", "Regional creator examples without claiming private analytics"},
	}
}

func ChannelKeywordClusters(kw KeywordIntelligence, videos []ChannelVideoSummary) []KeywordCluster {
	clusters := []KeywordCluster{}
	for _, term := range topN(kw.PrimaryKeywords, 6) {
		evidence := []string{}
		for _, video := range videos {
			if strings.Contains(strings.ToLower(video.Title), strings.ToLower(term)) {
				evidence = append(evidence, video.Title)
			}
			if len(evidence) >= 3 {
				break
			}
		}
		clusters = append(clusters, KeywordCluster{Name: term, Terms: relatedTerms(term, kw), Evidence: evidence})
	}
	return clusters
}

func ChannelStrategy(niche NicheAnalysis, pillars []string, patterns []string, distribution map[string]any) string {
	if len(pillars) == 0 {
		return "Insufficient public metadata to infer a clear strategy."
	}
	return "Publish repeatable " + niche.ContentFormat + " content in " + niche.PrimaryNiche + " around " + strings.Join(topN(pillars, 3), ", ") + ". Public title patterns suggest " + strings.Join(topN(patterns, 2), " and ") + "; view distribution highlights which visible topics overperform. This is inferred from public metadata only."
}

func ChannelOpportunities(niche NicheAnalysis, pillars []string, kw KeywordIntelligence) CreatorOpportunities {
	core := firstNonEmpty(firstPhrase(pillars), firstPhrase(kw.PrimaryKeywords), niche.SubNiche)
	return CreatorOpportunities{
		ContentGaps: []string{
			"Beginner entry point for " + core,
			"Comparison content against adjacent " + niche.PrimaryNiche + " topics",
			"Update/news format using the channel's strongest repeated phrases",
		},
		UnderusedTopics: topN(append(kw.SecondaryKeywords, kw.LongTailPhrases...), 6),
		LocalizationOptions: []string{
			"Adapt top pillar for US, UK, India, or Pakistan examples based on target audience.",
			"Translate the hook while keeping public metadata terms intact.",
			"Use region-specific examples without implying private audience geography.",
		},
		SuggestedRemakeAngles: []string{
			"Turn " + core + " into a short checklist.",
			"Make a beginner version of the highest-signal pillar.",
			"Create a comparison against a close alternative topic.",
			"Use a curiosity title but deliver a concrete takeaway in the first five seconds.",
			"Repurpose a top title into a Shorts-first question.",
		},
	}
}

func SuggestedChannelIdeas(niche NicheAnalysis, kw KeywordIntelligence, pillars []string) []string {
	seeds := highSignalIdeaSeeds(unique(append(append([]string{}, pillars...), kw.PrimaryKeywords...)))
	if len(seeds) == 0 {
		seeds = []string{niche.SubNiche}
	}
	templates := []string{
		"The 2026 Beginner Guide to %s",
		"5 %s Mistakes Viewers Keep Making",
		"%s Explained With One Real-World Example",
		"Before You Copy This %s Trend, Watch This",
		"%s vs the Alternative: What Actually Matters",
		"The Hidden Signal Behind %s",
		"How I Would Start With %s Today",
		"What Changed in %s This Week",
		"The Fast Checklist for %s",
		"Why %s Keeps Getting Attention",
	}
	ideas := []string{}
	for i, tmpl := range templates {
		seed := titleCase(seeds[i%len(seeds)])
		ideas = append(ideas, sprintf(tmpl, seed))
	}
	return ideas
}

func removeChannelIdentityKeywords(kw KeywordIntelligence, channelTitle string) KeywordIntelligence {
	titleTokens := tokenizeUseful(channelTitle, map[string]bool{})
	if len(titleTokens) == 0 {
		return kw
	}
	kw.PrimaryKeywords = filterIdentityTerms(kw.PrimaryKeywords, titleTokens)
	kw.SecondaryKeywords = filterIdentityTerms(kw.SecondaryKeywords, titleTokens)
	kw.LongTailPhrases = filterIdentityTerms(kw.LongTailPhrases, titleTokens)
	return kw
}

func filterIdentityTerms(values, titleTokens []string) []string {
	out := []string{}
	for _, value := range values {
		lower := strings.ToLower(value)
		matches := 0
		for _, token := range titleTokens {
			if strings.Contains(lower, token) {
				matches++
			}
		}
		if matches == len(titleTokens) || (len(titleTokens) == 1 && matches == 1) {
			continue
		}
		out = append(out, value)
	}
	return out
}

func highSignalIdeaSeeds(values []string) []string {
	out := []string{}
	for _, value := range values {
		words := strings.Fields(strings.ToLower(value))
		if len(words) == 1 && (stopWords[words[0]] || words[0] == "pro" || words[0] == "best") {
			continue
		}
		if strings.Contains(strings.ToLower(value), " they ") {
			continue
		}
		out = append(out, value)
	}
	return out
}

func SuggestedShortClipIdeas(niche NicheAnalysis, kw KeywordIntelligence, pillars []string) []string {
	core := firstNonEmpty(firstPhrase(pillars), firstPhrase(kw.PrimaryKeywords), niche.SubNiche)
	return []string{
		"15-second definition of " + core,
		"One mistake people make with " + core,
		"Before/after example from the strongest public title pattern",
		"Rapid comparison: " + core + " vs the closest alternative",
		"Three visible signals that make this topic work",
		"One myth about " + core + " corrected quickly",
		"Screenshot checklist for " + niche.AudienceType,
		"Question-led hook using the top long-tail phrase",
		"News/update angle based on the latest public title",
		"Remix a top channel pillar into a 30-second tutorial",
	}
}

func PerformanceDistribution(videos []ChannelVideoSummary) map[string]any {
	out, _ := channelSignals("", videos)
	views := []float64{}
	for _, video := range videos {
		if video.Views != nil {
			views = append(views, float64(*video.Views))
		}
	}
	if len(views) == 0 {
		return out
	}
	sort.Float64s(views)
	max := views[len(views)-1]
	sum := 0.0
	topSum := 0.0
	for i, v := range views {
		sum += v
		if i >= len(views)-3 {
			topSum += v
		}
	}
	if sum > 0 {
		out["view_concentration"] = topSum / sum
	}
	outliers := []string{}
	for _, video := range videos {
		if video.Views != nil && float64(*video.Views) >= max*0.75 {
			outliers = append(outliers, video.Title)
		}
	}
	out["outlier_videos"] = topN(outliers, 5)
	return out
}

func cleanMetadataText(text string) string {
	text = strings.ReplaceAll(text, "&amp;", " and ")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	kept := []string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if trimmed == "" {
			continue
		}
		if isBoilerplateLine(lower) {
			continue
		}
		trimmed = regexp.MustCompile(`https?://\S+|www\.\S+`).ReplaceAllString(trimmed, " ")
		trimmed = regexp.MustCompile(`\b[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}\b`).ReplaceAllString(trimmed, " ")
		trimmed = regexp.MustCompile(`\b\d{1,2}:\d{2}(?::\d{2})?\b`).ReplaceAllString(trimmed, " ")
		trimmed = regexp.MustCompile(`(?i)\b(utm_source|utm_medium|utm_campaign|utm_term|utm_content|ref|fbclid|gclid|mc_cid|mc_eid)=[^\s&]+`).ReplaceAllString(trimmed, " ")
		trimmed = normalizeCamelArtifact(trimmed)
		kept = append(kept, trimmed)
	}
	return strings.Join(kept, " ")
}

func splitDescriptionForWeighting(description string) [2]string {
	cleaned := cleanMetadataText(description)
	tokens := strings.Fields(cleaned)
	if len(tokens) <= 80 {
		return [2]string{cleaned, ""}
	}
	return [2]string{strings.Join(tokens[:80], " "), strings.Join(tokens[80:], " ")}
}

func isBoilerplateLine(lower string) bool {
	if strings.TrimSpace(lower) == "" {
		return true
	}
	boilerplate := []string{
		"subscribe", "follow me", "follow us", "affiliate", "sponsored", "sponsor", "use code", "discount code",
		"shop now", "my gear", "business inquiries", "contact:",
		"chapters", "timestamps", "all rights reserved", "thanks for watching", "like and comment", "turn on notifications",
		"clip licensing", "licensed under", "creative commons", "provided referral", "support our mission", "ted member",
		"browse pots", "home cooks supports content", "music used", "intro cool end", "end cool end",
	}
	for _, phrase := range boilerplate {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func normalizeCamelArtifact(value string) string {
	value = regexp.MustCompile(`([a-z])([A-Z])`).ReplaceAllString(value, `$1 $2`)
	value = strings.ReplaceAll(value, "_", " ")
	return value
}

func tokenizeUseful(text string, rejected map[string]bool) []string {
	tokens := []string{}
	for _, token := range tokensFromText(text) {
		if !isUsefulTerm(token) {
			rejected[token] = true
			continue
		}
		tokens = append(tokens, token)
	}
	return tokens
}

func tokensFromText(text string) []string {
	var tokens []string
	var b strings.Builder
	flush := func() {
		word := strings.ToLower(strings.Trim(b.String(), "-_ "))
		b.Reset()
		if word != "" {
			tokens = append(tokens, word)
		}
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			flush()
		}
	}
	if b.Len() > 0 {
		flush()
	}
	return tokens
}

func isUsefulTerm(term string) bool {
	term = strings.ToLower(strings.TrimSpace(term))
	if isNoisePhrase(term) {
		return false
	}
	if term == "ai" {
		return true
	}
	if len(term) < 3 || stopWords[term] {
		return false
	}
	if regexp.MustCompile(`^[a-z]$|^\d+$|^[a-f0-9]{8,}$|^[a-z0-9_-]{11,}$|^utm_|^[?&=]+$`).MatchString(term) {
		return false
	}
	if strings.Count(term, "-")+strings.Count(term, "_") >= 3 {
		return false
	}
	if lowInformationWords[term] {
		return false
	}
	words := strings.Fields(term)
	if len(words) > 1 {
		useful := 0
		for _, w := range words {
			if isUsefulTerm(w) {
				useful++
			}
		}
		return useful >= 2
	}
	return true
}

var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "from": true, "that": true, "this": true, "they": true, "them": true, "their": true, "say": true, "says": true, "said": true, "you": true, "your": true, "its": true,
	"are": true, "was": true, "were": true, "how": true, "why": true, "what": true, "when": true, "where": true, "will": true,
	"can": true, "into": true, "about": true, "video": true, "videos": true, "shorts": true, "youtube": true, "official": true,
	"http": true, "https": true, "www": true, "com": true, "watch": true, "subscribe": true, "channel": true, "new": true, "full": true,
	"lnk":     true,
	"episode": true, "part": true, "best": true, "scenes": true, "2026": true, "2025": true, "2024": true, "like": true, "comment": true,
	"share": true, "follow": true, "instagram": true, "tiktok": true, "facebook": true, "twitter": true, "xcom": true,
	"utm_source": true, "utm_medium": true, "utm_campaign": true, "magicpath": true, "grill": true, "skill": true,
}

var lowInformationWords = map[string]bool{
	"link": true, "links": true, "click": true, "here": true, "more": true, "today": true, "every": true,
	"project": true, "projects": true, "thing": true, "things": true, "stuff": true, "watching": true,
	"official": true, "random": true, "description": true, "below": true, "above": true, "page": true,
}

func ngrams(tokens []string, n int) []string {
	out := []string{}
	if len(tokens) < n {
		return out
	}
	for i := 0; i <= len(tokens)-n; i++ {
		chunk := tokens[i : i+n]
		if isUsefulTerm(strings.Join(chunk, " ")) {
			out = append(out, strings.Join(chunk, " "))
		}
	}
	return out
}

func overlapsSelected(term string, selected []string) bool {
	for _, item := range selected {
		if strings.Contains(item, term) || strings.Contains(term, item) {
			return true
		}
	}
	return false
}

func nearDuplicateSelected(term string, selected []string) bool {
	if overlapsSelected(term, selected) {
		return true
	}
	normalized := strings.ReplaceAll(strings.ToLower(term), "-", " ")
	for _, item := range selected {
		other := strings.ReplaceAll(strings.ToLower(item), "-", " ")
		if normalized == other {
			return true
		}
		if jaccardSimilarity(strings.Fields(normalized), strings.Fields(other)) >= 0.75 {
			return true
		}
	}
	return false
}

func isNaturalSearchPhrase(value string) bool {
	if isNoisePhrase(value) {
		return false
	}
	words := strings.Fields(value)
	if len(words) < 2 || len(words) > 6 {
		return false
	}
	useful := 0
	for _, word := range words {
		if isUsefulTerm(word) {
			useful++
		}
	}
	if useful < 2 {
		return false
	}
	return !regexp.MustCompile(`(?i)\butm|http|affiliate|sponsor|instagram|tiktok|facebook|twitter\b`).MatchString(value)
}

func isNoisePhrase(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return true
	}
	noise := []string{
		"utm", "browser made possible", "made possible", "possible grant", "friends scrimba", "our friends scrimba", "scrimba", "scrimba contents", "dub track", "change dub", "hindi dubbed", "melt labs",
		"home cooks supports", "supports content", "browse pots", "provided referral", "referral meaning", "clip licensing",
		"licensed under", "creative commons", "kevin mac leod", "monkeys spinning", "send clips", "funny pictures visit",
		"pictures visit", "mac leod", "leod artist", "matthew campen", "campen msph", "msph intro", "intro cool", "cool end", "support our mission", "ted member",
	}
	for _, item := range noise {
		if strings.Contains(lower, item) {
			return true
		}
	}
	return false
}

func inferSearchIntent(title string, primary, longTail []string) string {
	lower := strings.ToLower(title)
	switch {
	case strings.Contains(lower, "how") || containsAny(longTail, []string{"how to", "tutorial", "guide"}):
		return "Informational/tutorial intent inferred from public title and metadata."
	case strings.Contains(lower, "review") || strings.Contains(lower, "vs"):
		return "Comparison/review intent inferred from public title and metadata."
	case strings.Contains(lower, "news") || strings.Contains(lower, "2026"):
		return "Freshness/news intent inferred from public title and metadata."
	default:
		return "Discovery/entertainment intent inferred from public metadata, not exact search ranking data."
	}
}

func detectContentFormat(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "how to") || strings.Contains(lower, "tutorial"):
		return "tutorial"
	case regexp.MustCompile(`\b\d+\b`).MatchString(text):
		return "listicle"
	case strings.Contains(lower, "review"):
		return "review"
	case strings.Contains(lower, "react"):
		return "reaction"
	case strings.Contains(lower, "compilation") || strings.Contains(lower, "moments"):
		return "compilation"
	case strings.Contains(lower, "news") || strings.Contains(lower, "breaking"):
		return "news"
	default:
		return "explainer"
	}
}

func formatPatternsFromVideos(videos []ChannelVideoSummary) []string {
	counts := map[string]int{}
	for _, video := range videos {
		counts[detectContentFormat(video.Title)]++
	}
	type kv struct {
		k string
		v int
	}
	items := []kv{}
	for k, v := range counts {
		items = append(items, kv{k, v})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].v > items[j].v })
	out := []string{}
	for _, item := range items {
		out = append(out, item.k)
	}
	return out
}

func titlePattern(title string) string {
	switch {
	case strings.Contains(title, "?"):
		return "Question-led curiosity title"
	case regexp.MustCompile(`^\d+|\b\d+\b`).MatchString(title):
		return "Number/list title"
	case strings.Contains(title, ":") || strings.Contains(title, "|"):
		return "Two-part title with qualifier"
	default:
		return "Direct promise title"
	}
}

func inferAudienceType(niche, haystack string) string {
	if strings.Contains(haystack, "beginner") || strings.Contains(haystack, "learn") {
		return "beginners"
	}
	switch niche {
	case "technology/software", "AI tools":
		return "tech-curious creators and buyers"
	case "finance/business":
		return "operators, founders, and finance-minded viewers"
	case "education/tutorial":
		return "learners seeking practical steps"
	case "gaming":
		return "gaming fans and players"
	case "movies/animation", "entertainment":
		return "entertainment fans"
	default:
		return "general viewers in this niche"
	}
}

func inferContentAngle(format, sub, niche string) string {
	return "A " + format + " angle for " + sub + " within " + niche + ", inferred from public metadata."
}

func grammaticallyMeaningfulTopic(kw KeywordIntelligence, title, fallback string) string {
	titleTokens := tokenizeUseful(title, map[string]bool{})
	for n := minInt(4, len(titleTokens)); n >= 2; n-- {
		for _, phrase := range ngrams(titleTokens, n) {
			if isNaturalSearchPhrase(phrase) {
				return phrase
			}
		}
	}
	candidates := append(append([]string{}, kw.PrimaryTopics...), kw.PrimaryKeywords...)
	candidates = append(candidates, kw.SearchPhrases...)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if isNaturalSearchPhrase(candidate) {
			return candidate
		}
	}
	return fallback
}

func categoryNicheOverride(categoryID, haystack string) (string, string, bool) {
	switch strings.TrimSpace(categoryID) {
	case "10":
		return "Music", "Music video", true
	case "15":
		return "Pets and animals", "Animal entertainment", true
	case "20":
		return "Gaming", "Gaming guide or entertainment", true
	case "17":
		return "Sports", "Sports analysis or training", true
	case "19":
		return "Travel", "Travel and places", true
	case "23":
		return "Entertainment", "Comedy and skits", true
	case "25":
		return "News", "News and politics", true
	case "26":
		if strings.Contains(haystack, "makeup") || strings.Contains(haystack, "beauty") || strings.Contains(haystack, "skincare") {
			return "Lifestyle", "Beauty and fashion", true
		}
		if strings.Contains(haystack, "cook") || strings.Contains(haystack, "recipe") || strings.Contains(haystack, "kitchen") {
			return "Food", "Cooking and recipes", true
		}
	case "27":
		if strings.Contains(haystack, "finance") || strings.Contains(haystack, "investing") || strings.Contains(haystack, "money") || strings.Contains(haystack, "budget") || strings.Contains(haystack, "stock") {
			return "Business and finance", "Finance education", true
		}
		return "Education", "Practical tutorial", true
	case "28":
		return "Technology", "Software and technology", true
	}
	return "", "", false
}

func explainHookScore(title, hookType string, clarity, specificity, curiosity, audienceSignal, valuePromise int) string {
	return fmt.Sprintf("Title/metadata hook analysis: %q reads as a %s hook. Clarity %d, specificity %d, curiosity %d, audience signal %d, and value promise %d are based only on the public title text.", title, strings.ReplaceAll(hookType, "_", " "), clarity, specificity, curiosity, audienceSignal, valuePromise)
}

func relatedTerms(term string, kw KeywordIntelligence) []string {
	out := []string{term}
	for _, candidate := range append(kw.SecondaryKeywords, kw.LongTailPhrases...) {
		if len(out) >= 5 {
			break
		}
		if strings.Contains(candidate, term) || strings.Contains(term, candidate) {
			out = append(out, candidate)
		}
	}
	return unique(out)
}

func categoryName(id string) string {
	names := map[string]string{
		"1": "Film and Animation", "2": "Autos and Vehicles", "10": "Music", "15": "Pets and Animals",
		"17": "Sports", "19": "Travel and Events", "20": "Gaming", "22": "People and Blogs",
		"23": "Comedy", "24": "Entertainment", "25": "News and Politics", "26": "Howto and Style",
		"27": "Education", "28": "Science and Technology",
	}
	if name, ok := names[strings.TrimSpace(id)]; ok {
		return name
	}
	return id
}

func topicLastSegment(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if idx := strings.LastIndex(raw, "/"); idx >= 0 && idx+1 < len(raw) {
		raw = raw[idx+1:]
	}
	return strings.ReplaceAll(raw, "_", " ")
}

func channelAge(publishedAt string, now func() time.Time) string {
	t, err := time.Parse(time.RFC3339, publishedAt)
	if err != nil {
		return ""
	}
	years := now().Sub(t).Hours() / 24 / 365
	return sprintf("%.1f years", years)
}

func firstPhrase(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[0]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func titleCase(value string) string {
	words := strings.Fields(value)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

func containsAny(items, needles []string) bool {
	haystack := strings.ToLower(strings.Join(items, " "))
	for _, needle := range needles {
		if strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func mapKeys(values map[string]bool, limit int) []string {
	out := []string{}
	for k := range values {
		out = append(out, k)
	}
	sort.Strings(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func sprintf(format string, args ...any) string {
	return strings.TrimSpace(fmt.Sprintf(format, args...))
}
