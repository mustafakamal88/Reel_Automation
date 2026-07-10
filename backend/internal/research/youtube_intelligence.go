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
		tokens := tokenizeUseful(text, rejected)
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
	addText(in.Title, 5, true)
	addText(in.ChannelTitle, 1.2, true)
	addText(in.Description, 0.6, true)
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
	addText(categoryName(in.Category), 1.5, true)
	for _, topic := range in.TopicDetails {
		addText(topicLastSegment(topic), 1.8, true)
	}

	hashSeen := map[string]bool{}
	hashtags := []string{}
	for _, h := range regexp.MustCompile(`#([A-Za-z][A-Za-z0-9_]{1,40})`).FindAllString(in.Description+" "+strings.Join(in.Tags, " "), -1) {
		h = strings.ToLower(h)
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
		if seen[item.term] || overlapsSelected(item.term, primary) {
			continue
		}
		seen[item.term] = true
		words := strings.Fields(item.term)
		switch {
		case len(words) >= 3 && len(longTail) < 8:
			longTail = append(longTail, item.term)
		case len(primary) < 8:
			primary = append(primary, item.term)
		case len(secondary) < 12:
			secondary = append(secondary, item.term)
		}
		if len(primary) >= 8 && len(secondary) >= 12 && len(longTail) >= 8 {
			break
		}
	}
	if len(longTail) < 5 {
		for _, item := range items {
			if len(strings.Fields(item.term)) >= 2 && !containsString(longTail, item.term) && len(longTail) < 8 {
				longTail = append(longTail, item.term)
			}
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
		Hashtags:              hashtags,
		RejectedNoiseTerms:    mapKeys(rejected, 16),
		InferredSearchIntent:  inferSearchIntent(in.Title, primary, longTail),
		MetadataStrengthScore: metadataScore,
	}
}

func ClassifyNiche(in KeywordExtractionInput, kw KeywordIntelligence) NicheAnalysis {
	haystack := strings.ToLower(strings.Join(append(append([]string{in.Title, in.Description, in.ChannelTitle, categoryName(in.Category)}, in.Tags...), append(in.TopicDetails, append(kw.PrimaryKeywords, kw.LongTailPhrases...)...)...), " "))
	rules := []struct {
		niche string
		terms []string
	}{
		{"AI tools", []string{"ai", "chatgpt", "openai", "midjourney", "automation", "agent", "llm", "prompt"}},
		{"technology/software", []string{"software", "iphone", "android", "review", "tech", "app", "coding", "developer", "camera", "laptop"}},
		{"finance/business", []string{"money", "finance", "business", "startup", "investing", "stock", "crypto", "sales", "marketing"}},
		{"education/tutorial", []string{"how to", "tutorial", "learn", "course", "guide", "explained", "beginner", "lesson"}},
		{"gaming", []string{"game", "gaming", "minecraft", "roblox", "fortnite", "ps5", "xbox", "nintendo"}},
		{"movies/animation", []string{"movie", "film", "animation", "anime", "trailer", "scene", "disney", "pixar"}},
		{"sports", []string{"football", "soccer", "nba", "nfl", "cricket", "match", "goal", "training"}},
		{"health/fitness", []string{"fitness", "workout", "health", "diet", "weight loss", "gym", "muscle"}},
		{"beauty/fashion", []string{"makeup", "beauty", "fashion", "outfit", "skincare", "style"}},
		{"food", []string{"recipe", "food", "cooking", "restaurant", "meal", "kitchen"}},
		{"travel", []string{"travel", "hotel", "flight", "city", "country", "tour", "trip"}},
		{"comedy", []string{"funny", "comedy", "meme", "prank", "joke", "skit"}},
		{"news/politics", []string{"news", "election", "politics", "government", "breaking", "president"}},
		{"culture/lifestyle", []string{"lifestyle", "family", "daily", "vlog", "culture", "home"}},
		{"entertainment", []string{"entertainment", "celebrity", "viral", "reaction", "challenge", "story"}},
	}
	bestNiche := "other"
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
			bestNiche = rule.niche
			evidence = found
		}
	}
	if bestNiche == "other" && len(kw.PrimaryKeywords) > 0 {
		evidence = topN(kw.PrimaryKeywords, 4)
	}
	format := detectContentFormat(in.Title + " " + in.Description)
	sub := firstNonEmpty(firstPhrase(kw.LongTailPhrases), firstPhrase(kw.PrimaryKeywords), "general")
	confidence := 0.42 + float64(bestScore)*0.09
	if confidence > 0.9 {
		confidence = 0.9
	}
	return NicheAnalysis{
		PrimaryNiche:         bestNiche,
		SubNiche:             sub,
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
	curiosity := 45 + len(triggers)*8
	if strings.Contains(title, "?") {
		curiosity += 15
	}
	if curiosity > 100 {
		curiosity = 100
	}
	remake := (clarity + curiosity) / 2
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
		CuriosityScore:       curiosity,
		RemakePotentialScore: remake,
	}
}

func BuildCreatorOpportunities(niche NicheAnalysis, kw KeywordIntelligence, hook HookIntelligence, title string) CreatorOpportunities {
	core := firstNonEmpty(firstPhrase(kw.PrimaryKeywords), niche.SubNiche, "the topic")
	format := strings.ReplaceAll(hook.HookType, "_", " ")
	return CreatorOpportunities{
		SuggestedRemakeAngles: []string{
			"Remake the premise as a faster " + format + " focused on " + core + ".",
			"Turn the strongest claim into a before/after breakdown for " + niche.AudienceType + ".",
			"Build a checklist version around " + core + " with one clear takeaway per beat.",
			"Create a contrarian version: what most creators miss about " + core + ".",
			"Localize the idea for a specific audience while keeping the same public metadata angle.",
		},
		TitleIdeas: []string{
			"What Nobody Explains About " + titleCase(core),
			"The Fastest Way to Understand " + titleCase(core),
			"I Rebuilt This Idea for " + titleCase(niche.AudienceType),
			"5 Signals Hidden in " + titleCase(core),
			"Before You Copy This " + titleCase(niche.PrimaryNiche) + " Trend",
		},
		ShortFormClipIdeas: []string{
			"Open with the biggest visible result, then explain the public signal behind it.",
			"Cut a 20-second myth-versus-reality version around " + core + ".",
			"Make a three-step checklist viewers can screenshot.",
			"Create a reaction clip focused on the title promise and whether it pays off.",
			"Turn one long-tail phrase into a single-question short.",
		},
		ScriptPrompts: []string{
			"Write a 30-second " + hook.HookType + " script for " + niche.AudienceType + " about " + core + ".",
			"Write a short script that opens with the public performance signal and then teaches one takeaway.",
			"Write a remake script using these inferred public keywords: " + strings.Join(topN(kw.PrimaryKeywords, 4), ", ") + ".",
			"Write a script that clearly states this is based on public metadata, not private analytics.",
			"Write a punchier version of: " + title,
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
	return "Likely strategy: publish repeatable " + niche.ContentFormat + " content in " + niche.PrimaryNiche + " around " + strings.Join(topN(pillars, 3), ", ") + ". Public title patterns suggest " + strings.Join(topN(patterns, 2), " and ") + "; view distribution highlights which visible topics overperform. This is inferred from public metadata only."
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
	seeds := unique(append(append([]string{}, pillars...), kw.PrimaryKeywords...))
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
	if term == "ai" {
		return true
	}
	if len(term) < 3 || stopWords[term] {
		return false
	}
	if regexp.MustCompile(`^[a-z]$|^\d+$|^[a-f0-9]{8,}$|^[a-z0-9_-]{11,}$`).MatchString(term) {
		return false
	}
	if strings.Count(term, "-")+strings.Count(term, "_") >= 3 {
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
	"the": true, "and": true, "for": true, "with": true, "from": true, "that": true, "this": true, "you": true, "your": true,
	"are": true, "was": true, "were": true, "how": true, "why": true, "what": true, "when": true, "where": true, "will": true,
	"can": true, "into": true, "about": true, "video": true, "videos": true, "shorts": true, "youtube": true, "official": true,
	"http": true, "https": true, "www": true, "com": true, "watch": true, "subscribe": true, "channel": true, "new": true, "full": true,
	"episode": true, "part": true, "best": true, "scenes": true, "2026": true, "2025": true, "2024": true, "like": true, "comment": true,
	"share": true, "follow": true, "instagram": true, "tiktok": true, "facebook": true, "twitter": true, "xcom": true,
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
