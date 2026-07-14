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
	cleanTitle := cleanMetadataText(in.Title)
	cleanDescription := cleanMetadataText(in.Description)
	cleanTags := make([]string, 0, len(in.Tags))
	for _, tag := range in.Tags {
		cleaned := cleanMetadataText(tag)
		if cleaned != "" {
			cleanTags = append(cleanTags, cleaned)
		}
	}
	evidenceText := strings.Join(append([]string{cleanTitle, cleanDescription, categoryName(in.Category), topicLastSegment(strings.Join(in.TopicDetails, " "))}, cleanTags...), " ")
	evidenceTokens := tokenSet(tokenizeUseful(evidenceText, map[string]bool{}))
	titleTokens := tokenSet(tokenizeUseful(cleanTitle, map[string]bool{}))
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
	titleWeight := 6.5
	if len(in.RecentVideoTitles) > 0 {
		titleWeight = 0.8
	}
	addText(cleanTitle, titleWeight, true)
	addText(cleanMetadataText(in.ChannelTitle), 0.7, false)
	descParts := splitDescriptionForWeighting(in.Description)
	if descParts[0] != "" {
		addText(descParts[0], 1.2, true)
	}
	if descParts[1] != "" {
		addText(descParts[1], 0.25, false)
	}
	for _, tag := range cleanTags {
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
		term = normalizeTopicPhrase(term)
		if !isUsefulTerm(term) || !isRelevantPhrase(term, evidenceTokens, titleTokens, score) {
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
	primary = filterQualityPhrases(primary, evidenceTokens, titleTokens, 6)
	secondary = filterQualityPhrases(secondary, evidenceTokens, titleTokens, 10)
	longTail = filterQualityPhrases(longTail, evidenceTokens, titleTokens, 8)

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
	return sanitizeKeywordIntelligenceOutput(KeywordIntelligence{
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
	}, in)
}

func sanitizeKeywordIntelligenceOutput(kw KeywordIntelligence, in KeywordExtractionInput) KeywordIntelligence {
	cleanTitle := cleanMetadataText(in.Title)
	cleanDescription := cleanMetadataText(in.Description)
	cleanTags := make([]string, 0, len(in.Tags))
	for _, tag := range in.Tags {
		if cleaned := cleanMetadataText(tag); cleaned != "" {
			cleanTags = append(cleanTags, cleaned)
		}
	}
	evidenceText := strings.Join(append([]string{cleanTitle, cleanDescription, categoryName(in.Category), strings.Join(in.TopicDetails, " ")}, cleanTags...), " ")
	evidenceTokens := tokenSet(tokenizeUseful(evidenceText, map[string]bool{}))
	titleTokens := tokenSet(tokenizeUseful(cleanTitle, map[string]bool{}))
	reconstructed := reconstructTopicHierarchy(in, kw, evidenceTokens, titleTokens)
	if len(reconstructed.primary) > 0 {
		kw.PrimaryKeywords = reconstructed.primary
	} else {
		kw.PrimaryKeywords = filterQualityPhrases(kw.PrimaryKeywords, evidenceTokens, titleTokens, 6)
	}
	if len(reconstructed.supporting) > 0 {
		kw.SecondaryKeywords = reconstructed.supporting
	} else {
		kw.SecondaryKeywords = filterQualityPhrases(kw.SecondaryKeywords, evidenceTokens, titleTokens, 10)
	}
	if len(reconstructed.search) > 0 {
		kw.LongTailPhrases = reconstructed.search
	} else {
		kw.LongTailPhrases = filterQualityPhrases(kw.LongTailPhrases, evidenceTokens, titleTokens, 8)
	}
	kw.PrimaryKeywords = validatePhraseList(kw.PrimaryKeywords, evidenceTokens, titleTokens, 6, true)
	kw.SecondaryKeywords = validatePhraseList(kw.SecondaryKeywords, evidenceTokens, titleTokens, 10, true)
	kw.LongTailPhrases = validatePhraseList(kw.LongTailPhrases, evidenceTokens, titleTokens, 8, true)
	if len(kw.PrimaryKeywords) == 0 {
		if fallback := cleanTitleTopicFallback(in.Title, evidenceTokens, titleTokens); fallback != "" {
			kw.PrimaryKeywords = []string{fallback}
		}
	}
	if titleSubject := cleanTitleSingleSubject(in.Title); titleSubject != "" && !containsStringNormalized(kw.PrimaryKeywords, titleSubject) && !nearDuplicateSelected(titleSubject, kw.PrimaryKeywords) {
		kw.PrimaryKeywords = append([]string{titleSubject}, kw.PrimaryKeywords...)
		if len(kw.PrimaryKeywords) > 6 {
			kw.PrimaryKeywords = kw.PrimaryKeywords[:6]
		}
	}
	for _, tag := range in.Tags {
		tagSubject := cleanSingleTagSubject(tag)
		if tagSubject == "" || containsStringNormalized(kw.PrimaryKeywords, tagSubject) || containsStringNormalized(kw.SecondaryKeywords, tagSubject) || nearDuplicateSelected(tagSubject, append(kw.PrimaryKeywords, kw.SecondaryKeywords...)) {
			continue
		}
		kw.SecondaryKeywords = append(kw.SecondaryKeywords, tagSubject)
		if len(kw.SecondaryKeywords) >= 10 {
			break
		}
	}
	if len(kw.LongTailPhrases) == 0 {
		kw.LongTailPhrases = naturalSearchPhrasesFromTopics(append(kw.PrimaryKeywords, kw.SecondaryKeywords...), 5)
	}
	kw.PrimaryKeywords = applyAcronymCasingList(kw.PrimaryKeywords)
	kw.SecondaryKeywords = applyAcronymCasingList(kw.SecondaryKeywords)
	kw.LongTailPhrases = applyAcronymCasingList(kw.LongTailPhrases)
	kw.PrimaryTopics = kw.PrimaryKeywords
	kw.SupportingTerms = kw.SecondaryKeywords
	kw.SearchPhrases = kw.LongTailPhrases
	return kw
}

type topicHierarchy struct {
	primary    []string
	supporting []string
	search     []string
}

type topicCandidate struct {
	phrase  string
	score   float64
	sources map[string]bool
}

func reconstructTopicHierarchy(in KeywordExtractionInput, kw KeywordIntelligence, evidenceTokens, titleTokens map[string]bool) topicHierarchy {
	candidates := map[string]*topicCandidate{}
	add := func(raw, source string, weight float64) {
		phrase := normalizeTopicPhrase(raw)
		if phrase == "" || !ValidCreatorPhrase(phrase, evidenceTokens, titleTokens, true) || !isRelevantPhrase(phrase, evidenceTokens, titleTokens, weight) {
			return
		}
		if len(strings.Fields(phrase)) > 5 {
			return
		}
		key := topicDedupeKey(phrase)
		c := candidates[key]
		if c == nil {
			c = &topicCandidate{phrase: phrase, sources: map[string]bool{}}
			candidates[key] = c
		}
		c.score += weight + phraseSpecificityBonus(phrase)
		c.sources[source] = true
	}
	addText := func(text, source string, weight float64, maxN int) {
		cleaned := cleanMetadataText(text)
		tokens := tokenizeUseful(cleaned, map[string]bool{})
		for _, token := range tokens {
			add(token, source, weight)
		}
		for n := 2; n <= maxN; n++ {
			for _, phrase := range ngrams(tokens, n) {
				add(phrase, source, weight*float64(n)*0.95)
			}
		}
		for _, segment := range meaningfulTitleSegments(cleaned) {
			add(segment, source, weight*2.2)
		}
	}
	addText(in.Title, "title", 6.5, 5)
	descParts := splitDescriptionForWeighting(in.Description)
	addText(descParts[0], "description_intro", 3.2, 5)
	addText(descParts[1], "description", 1.1, 4)
	for _, tag := range in.Tags {
		addText(tag, "tag", 5.2, 5)
	}
	for _, topic := range in.TopicDetails {
		addText(topicLastSegment(topic), "topic_metadata", 0.9, 3)
	}
	addText(categoryName(in.Category), "category", 0.45, 3)
	for _, value := range append(append(append([]string{}, kw.PrimaryKeywords...), kw.SecondaryKeywords...), kw.LongTailPhrases...) {
		add(value, "existing", 2.2)
	}
	for _, phrase := range inferredSupportedDomainPhrases(in) {
		normalized := normalizeTopicPhrase(phrase)
		if normalized == "" || !ValidCreatorPhrase(normalized, evidenceTokens, titleTokens, true) {
			continue
		}
		key := topicDedupeKey(normalized)
		c := candidates[key]
		if c == nil {
			c = &topicCandidate{phrase: normalized, sources: map[string]bool{}}
			candidates[key] = c
		}
		c.score += 24 + phraseSpecificityBonus(normalized)
		c.sources["metadata_context"] = true
	}
	items := make([]topicCandidate, 0, len(candidates))
	for _, c := range candidates {
		if len(c.sources) > 1 {
			c.score += float64(len(c.sources)) * 2.4
		}
		if c.sources["title"] || c.sources["tag"] {
			c.score += 2.2
		}
		if isGenericTopicPhrase(c.phrase) {
			c.score -= 50
		}
		items = append(items, *c)
	}
	for i := range items {
		words := strings.Fields(items[i].phrase)
		if len(words) == 1 && isKnownAcronym(words[0]) {
			for _, other := range items {
				if len(strings.Fields(other.phrase)) > 1 && strings.Contains(other.phrase, words[0]) {
					items[i].score -= 20
					break
				}
			}
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			return len(items[i].phrase) > len(items[j].phrase)
		}
		return items[i].score > items[j].score
	})
	primary := []string{}
	supporting := []string{}
	search := []string{}
	for _, item := range items {
		phrase := item.phrase
		if !ValidCreatorPhrase(phrase, evidenceTokens, titleTokens, true) || nearDuplicateSelected(phrase, append(append(primary, supporting...), search...)) {
			continue
		}
		words := len(strings.Fields(phrase))
		switch {
		case len(primary) < 6 && (words >= 2 || item.score >= 8):
			primary = append(primary, phrase)
		case len(supporting) < 10:
			supporting = append(supporting, phrase)
		}
		if len(search) < 8 && words >= 2 && isNaturalSearchPhrase(phrase) {
			search = append(search, phrase)
		}
	}
	if len(primary) == 1 && len(supporting) > 0 && topicDedupeKey(primary[0]) == topicDedupeKey(supporting[0]) {
		supporting = supporting[1:]
	}
	return topicHierarchy{primary: primary, supporting: supporting, search: search}
}

func inferredSupportedDomainPhrases(in KeywordExtractionInput) []string {
	raw := strings.ToLower(strings.Join(append(append([]string{in.Title, cleanMetadataText(in.Description), in.ChannelTitle, categoryName(in.Category)}, in.Tags...), in.TopicDetails...), " "))
	out := []string{}
	if regexp.MustCompile(`\bict\b`).MatchString(raw) && containsAny([]string{raw}, []string{"trading", "trader", "funded", "forex", "financial instrument", "market"}) {
		if strings.Contains(raw, "concept") {
			out = append(out, "ict trading concepts")
		} else {
			out = append(out, "ict trading")
		}
	}
	if containsAny([]string{raw}, []string{"missed entry", "missed trade", "missed setup"}) && containsAny([]string{raw}, []string{"trade", "trading", "entry"}) {
		out = append(out, "missed trade entries", "trade entry management", "reviewing a missed setup")
	}
	if containsAny([]string{raw}, []string{"same trade", "trade idea"}) {
		out = append(out, "same trade idea")
	}
	return out
}

func ClassifyNiche(in KeywordExtractionInput, kw KeywordIntelligence) NicheAnalysis {
	haystack := strings.ToLower(strings.Join(append(append([]string{in.Title, cleanMetadataText(in.Description), in.ChannelTitle, categoryName(in.Category)}, in.Tags...), append(in.TopicDetails, append(kw.PrimaryKeywords, kw.LongTailPhrases...)...)...), " "))
	rules := []struct {
		broad string
		niche string
		terms []string
	}{
		{"Software education", "AI-assisted development", []string{"codex", "chatgpt", "openai", "coding", "developer", "agent", "llm", "prompt", "automation", "install"}},
		{"Technology", "Consumer technology", []string{"software", "iphone", "android", "review", "tech", "app", "camera", "laptop", "tesla"}},
		{"Business and finance", "Trading education", []string{"ict", "trading", "trader", "forex", "trade entry", "missed entry", "price action", "setup"}},
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
	format := detectContentFormat(in.Title + " " + cleanMetadataText(in.Description))
	specificTopic := grammaticallyMeaningfulTopic(kw, in.Title, bestNiche)
	specificTopic = applyAcronymCasing(specificTopic)
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
		EvidenceTerms:        applyAcronymCasingList(unique(topN(append(evidence, kw.PrimaryKeywords...), 8))),
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
	core := bestCreatorCore(niche, kw, title)
	cleanTitle := cleanTitleForCreativePrompt(title)
	if cleanTitle == "" {
		cleanTitle = strings.TrimSpace(title)
	}
	format := strings.ReplaceAll(hook.HookType, "_", " ")
	audience := audienceOrFallback(niche.TargetAudience, niche.AudienceType)
	if core == "" {
		return CreatorOpportunities{}
	}
	core = applyAcronymCasing(core)
	audience = applyAcronymCasing(audience)
	themes := groundedCreativeThemes(kw, core)
	themeA := firstNonEmpty(firstTheme(themes, 0), core)
	themeB := firstTheme(themes, 1)
	themeC := firstNonEmpty(firstTheme(themes, 2), themeA, core)
	themeADistinct := !nearDuplicateSelected(themeA, []string{core})
	angles := []string{
		"Checklist angle: Break " + titleCase(core) + " into practical steps for " + audience + ".",
		"Progression angle: Teach " + titleCase(core) + " from beginner context to the decision point.",
		"Workflow angle: Turn " + titleCase(core) + " into a concise decision flow using " + titleCase(themeA) + ".",
		"Mistake-led angle: Show the common error around " + titleCase(core) + " and how to review it.",
		"Case-study angle: Rebuild the original title premise as one concrete walkthrough.",
	}
	titles := []string{
		titleCase(core) + " Explained With a Clear Example",
		"How to Review " + titleCase(core) + " Without Guessing",
		"A Practical Checklist for " + titleCase(core),
		"What to Check Before Acting on " + titleCase(core),
	}
	if themeADistinct {
		titles = append(titles, "How "+titleCase(themeA)+" Fits Into "+titleCase(core))
	}
	shorts := []string{
		"Short-form idea: define " + titleCase(themeA) + " in one clear example.",
		"Short-form idea: three signs that " + titleCase(themeC) + " matters in the example.",
		"Short-form idea: a beginner mistake around " + titleCase(core) + " corrected quickly.",
		"Short-form idea: turn " + titleCase(themeA) + " into a single-question hook.",
	}
	scripts := []string{
		"Write a 30-second " + format + " script for " + audience + " about " + core + " using " + themeA + " as the main example.",
		"Write a 60-second tutorial script that preserves the public title premise \"" + cleanTitle + "\" and teaches " + themeC + ".",
		"Write a short adaptation using these approved public topics: " + strings.Join(topN(themes, 4), ", ") + ".",
		"Write a concise hook and outline for a truthful remake about " + core + ".",
	}
	if strings.TrimSpace(themeB) != "" {
		angles = append([]string{"Comparison angle: Show when " + titleCase(themeA) + " and " + titleCase(themeB) + " point to different creator lessons."}, angles...)
		titles = append([]string{titleCase(themeA) + " vs " + titleCase(themeB) + ": When Each Matters"}, titles...)
		shorts = append([]string{"Short-form idea: compare " + titleCase(themeA) + " and " + titleCase(themeB) + " in 30 seconds."}, shorts...)
		scripts = append(scripts, "Write a comparison script around "+themeA+" and "+themeB+" for "+audience+".")
	}
	opps := CreatorOpportunities{
		SuggestedRemakeAngles: filterCreativeOutputs(angles),
		TitleIdeas:            filterCreativeOutputs(titles),
		ShortFormClipIdeas:    filterCreativeOutputs(shorts),
		ScriptPrompts:         filterCreativeOutputs(scripts),
		ContentGaps:           applyAcronymCasingList([]string{"Beginner explainer for " + core, "Mistake-led breakdown of " + themeC}),
		UnderusedTopics:       filterQualityPhrases(append(kw.SecondaryKeywords, kw.LongTailPhrases...), tokenSet(tokenizeUseful(strings.Join(append([]string{cleanTitle}, kw.PrimaryKeywords...), " "), map[string]bool{})), map[string]bool{}, 5),
		LocalizationOptions:   []string{"Local examples for target country", "Bilingual hook", "Regional creator examples without claiming private analytics"},
	}
	if len(kw.PrimaryTopics) == 0 {
		opps.ScriptPrompts = filterCreativeOutputs([]string{
			"Write a concise short-form script about " + core + ", grounded only in public metadata.",
			"Write a hook and outline for the public title: " + cleanTitle,
		})
	}
	return opps
}

func groundedCreativeThemes(kw KeywordIntelligence, core string) []string {
	values := append(append(append([]string{}, kw.SecondaryKeywords...), kw.PrimaryKeywords...), kw.LongTailPhrases...)
	out := []string{}
	for _, value := range values {
		phrase := applyAcronymCasing(normalizeTopicPhrase(value))
		if phrase == "" || isGenericTopicPhrase(phrase) || creativeOutputRejected(phrase) || nearDuplicateSelected(phrase, append(out, core)) {
			continue
		}
		out = append(out, phrase)
		if len(out) >= 8 {
			break
		}
	}
	if len(out) == 0 && strings.TrimSpace(core) != "" {
		out = append(out, core)
	}
	return out
}

func firstTheme(values []string, index int) string {
	if index >= 0 && index < len(values) {
		return values[index]
	}
	return ""
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
	skipSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if trimmed == "" {
			skipSection = false
			continue
		}
		if isBoilerplateSectionHeading(lower) {
			skipSection = true
			continue
		}
		if skipSection {
			if looksLikeContentLine(lower) {
				skipSection = false
			} else {
				continue
			}
		}
		trimmed = regexp.MustCompile(`(?i)^\s*(disclaimer|risk warning|affiliate disclosure|copyright disclaimer)\s*[:\-–]\s*`).ReplaceAllString(trimmed, "")
		lower = strings.ToLower(strings.TrimSpace(trimmed))
		if isBoilerplateLine(lower) {
			continue
		}
		trimmed = regexp.MustCompile(`https?://\S+|www\.\S+`).ReplaceAllString(trimmed, " ")
		trimmed = regexp.MustCompile(`\b[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}\b`).ReplaceAllString(trimmed, " ")
		trimmed = regexp.MustCompile(`\b\d{1,2}:\d{2}(?::\d{2})?\b`).ReplaceAllString(trimmed, " ")
		trimmed = regexp.MustCompile(`(?i)\b(utm_source|utm_medium|utm_campaign|utm_term|utm_content|ref|fbclid|gclid|mc_cid|mc_eid)=[^\s&]+`).ReplaceAllString(trimmed, " ")
		trimmed = stripInlineBoilerplate(trimmed)
		trimmed = normalizeCamelArtifact(trimmed)
		for _, sentence := range splitMetadataSentences(trimmed) {
			sentence = strings.TrimSpace(sentence)
			if sentence == "" || isBoilerplateLine(strings.ToLower(sentence)) {
				continue
			}
			kept = append(kept, sentence)
		}
	}
	return strings.Join(kept, " ")
}

func isBoilerplateSectionHeading(lower string) bool {
	lower = strings.TrimSpace(strings.Trim(lower, ":-–— "))
	headings := []string{
		"disclaimer", "risk warning", "risk disclaimer", "affiliate disclosure", "sponsored", "sponsors", "links",
		"my links", "socials", "social links", "chapters", "timestamps", "gear", "equipment", "software",
		"copyright disclaimer", "legal", "important disclosure",
	}
	for _, heading := range headings {
		if lower == heading || strings.HasPrefix(lower, heading+":") {
			return true
		}
	}
	return false
}

func looksLikeContentLine(lower string) bool {
	if isBoilerplateLine(lower) {
		return false
	}
	if regexp.MustCompile(`^\d{1,2}:?\d{2}`).MatchString(lower) {
		return false
	}
	return containsAny([]string{lower}, []string{"learn", "how", "trade", "setup", "entry", "strategy", "example", "tutorial", "review"})
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
	if disclaimerPattern().MatchString(lower) {
		return true
	}
	if regexp.MustCompile(`(?i)^\s*(\d{1,2}:)?\d{1,2}:\d{2}\s+`).MatchString(lower) {
		return true
	}
	boilerplate := []string{
		"subscribe", "follow me", "follow us", "affiliate", "sponsored", "sponsor", "use code", "discount code",
		"shop now", "my gear", "business inquiries", "contact:",
		"chapters", "timestamps", "all rights reserved", "thanks for watching", "like and comment", "turn on notifications",
		"clip licensing", "licensed under", "creative commons", "provided referral", "support our mission", "ted member",
		"browse pots", "home cooks supports content", "music used", "intro cool end", "end cool end",
		"not financial advice", "financial advice", "do your own research", "own research", "past performance",
		"future results", "investment decisions", "licensed financial", "financial adviser", "financial advisor",
		"educational purposes only", "for educational purposes", "should buy", "should sell", "buy or sell",
		"risk warning", "trading involves risk", "copyright disclaimer", "fair use", "all opinions are my own",
		"business enquiry", "business inquiries", "for business", "join my discord", "telegram", "whatsapp",
		"disclaimer", "entertainment trade risk", "trade risk", "educational entertainment",
		"hypothetical performance", "simulated performance", "actual trading", "benefit of hindsight",
		"representation being made", "no representation", "results may vary", "limitation unlike actual performance",
		"trading results", "hypothetical or simulated", "not indicative of future",
	}
	for _, phrase := range boilerplate {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

func splitMetadataSentences(value string) []string {
	value = regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(value), " ")
	if value == "" {
		return nil
	}
	parts := regexp.MustCompile(`[.!?•]+`).Split(value, -1)
	if len(parts) == 0 {
		return []string{value}
	}
	return parts
}

func stripInlineBoilerplate(value string) string {
	cleaned := value
	patterns := []string{
		`(?i)\b(not\s+financial\s+advice|financial\s+advice|do\s+your\s+own\s+research|past\s+performance\s+[^.?!]*future\s+results|hypothetical\s+(or\s+)?simulated\s+performance|simulated\s+performance|actual\s+trading|benefit\s+of\s+hindsight|representation\s+being\s+made|no\s+representation|results\s+may\s+vary|limitation\s+unlike\s+actual\s+performance|consult\s+(a\s+)?licensed\s+financial\s+(adviser|advisor)|for\s+educational\s+purposes\s+only|investment\s+decisions?|should\s+(buy|sell)|buy\s+or\s+sell|trading\s+involves\s+risk)\b[^.?!]*`,
		`(?i)\b(affiliate\s+links?|sponsored\s+by|paid\s+promotion|use\s+code|discount\s+code|business\s+inquir(y|ies)|contact\s+me|follow\s+(me|us)|subscribe|like\s+and\s+comment|turn\s+on\s+notifications)\b[^.?!]*`,
		`(?i)\b(copyright\s+disclaimer|fair\s+use|all\s+rights\s+reserved|no\s+copyright\s+infringement)\b[^.?!]*`,
	}
	for _, pattern := range patterns {
		cleaned = regexp.MustCompile(pattern).ReplaceAllString(cleaned, " ")
	}
	cleaned = regexp.MustCompile(`(?m)(^|\s)#[A-Za-z0-9_]{1,40}(\s|$)`).ReplaceAllString(cleaned, " ")
	return regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(cleaned), " ")
}

func disclaimerPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?i)\b(not\s+financial\s+advice|do\s+your\s+own\s+research|past\s+performance|future\s+results|hypothetical\s+(or\s+)?simulated\s+performance|simulated\s+performance|actual\s+trading|benefit\s+of\s+hindsight|hindsight|representation\s+being\s+made|no\s+representation|results\s+may\s+vary|limitation\s+unlike\s+actual\s+performance|investment\s+decisions?|licensed\s+financial\s+(adviser|advisor)|educational\s+purposes\s+only|should\s+(buy|sell)|buy\s+or\s+sell|trading\s+involves\s+risk|risk\s+warning|affiliate\s+disclosure|paid\s+promotion|copyright\s+disclaimer|fair\s+use)\b`)
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
	term = normalizeTopicPhrase(term)
	if !ValidCreatorPhrase(term, nil, nil, false) {
		return false
	}
	if term == "ai" {
		return true
	}
	if len(term) < 3 || stopWords[term] {
		return false
	}
	if alphaDigitTokenPattern.MatchString(term) && len(term) >= 8 {
		return false
	}
	if uselessTokenPattern.MatchString(term) {
		return false
	}
	if opaqueTokenPattern.MatchString(term) && digitOrSeparatorPattern.MatchString(term) {
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
		if stopWords[words[0]] || stopWords[words[len(words)-1]] {
			return false
		}
		for _, w := range words {
			if isUsefulTerm(w) {
				useful++
			}
		}
		return useful >= 2 && float64(useful)/float64(len(words)) >= 0.5
	}
	return true
}

var (
	alphaDigitTokenPattern  = regexp.MustCompile(`[a-z]+\d|\d+[a-z]+`)
	uselessTokenPattern     = regexp.MustCompile(`^[a-z]$|^\d+$|^utm_|^[?&=]+$`)
	opaqueTokenPattern      = regexp.MustCompile(`^[a-f0-9]{8,}$|^[a-z0-9_-]{11,}$`)
	digitOrSeparatorPattern = regexp.MustCompile(`\d|_|-`)
)

var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "from": true, "that": true, "this": true, "they": true, "them": true, "their": true, "say": true, "says": true, "said": true, "you": true, "your": true, "its": true,
	"are": true, "was": true, "were": true, "how": true, "why": true, "what": true, "when": true, "where": true, "will": true,
	"can": true, "into": true, "about": true, "video": true, "videos": true, "shorts": true, "youtube": true, "official": true,
	"http": true, "https": true, "www": true, "com": true, "watch": true, "subscribe": true, "channel": true, "new": true, "full": true,
	"lnk":     true,
	"episode": true, "part": true, "best": true, "scenes": true, "2026": true, "2025": true, "2024": true, "like": true, "comment": true,
	"share": true, "follow": true, "instagram": true, "tiktok": true, "facebook": true, "twitter": true, "xcom": true,
	"utm_source": true, "utm_medium": true, "utm_campaign": true, "magicpath": true, "grill": true, "short": true,
	"not": true, "any": true, "own": true, "only": true, "should": true, "before": true, "after": true,
	"explain": true, "explains": true, "explained": true, "learn": true, "learning": true,
	"lecture": true, "educational": true, "entertainment": true, "minute": true, "minutes": true, "updated": true,
	"january": true, "february": true, "march": true, "april": true, "may": true, "june": true,
	"july": true, "august": true, "september": true, "october": true, "november": true, "december": true,
	"youtu": true, "tube": true, "tested": true, "cover": true, "covers": true, "include": true, "includes": true, "including": true,
}

var lowInformationWords = map[string]bool{
	"link": true, "links": true, "click": true, "here": true, "more": true, "today": true, "every": true,
	"project": true, "projects": true, "thing": true, "things": true, "stuff": true, "watching": true,
	"official": true, "random": true, "description": true, "below": true, "above": true, "page": true,
	"financial": true, "finance": true, "results": true, "future": true, "past": true, "purposes": true,
	"purpose": true, "research": true, "consult": true, "licensed": true, "adviser": true, "advisor": true,
	"decisions": true, "decision": true, "guaranteed": true, "guarantee": true, "funded": true, "days": true,
	"lecture": true, "educational": true, "entertainment": true, "minute": true, "minutes": true, "updated": true,
	"tested": true, "cover": true, "covers": true,
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
	normalized := strings.ReplaceAll(strings.ToLower(term), "-", " ")
	for _, item := range selected {
		other := strings.ReplaceAll(strings.ToLower(item), "-", " ")
		if normalized == other {
			return true
		}
		if topicDedupeKey(normalized) != "" && topicDedupeKey(normalized) == topicDedupeKey(other) {
			return true
		}
		if containmentDuplicate(normalized, other) {
			return true
		}
		if jaccardSimilarity(strings.Fields(normalized), strings.Fields(other)) >= 0.82 {
			return true
		}
	}
	return false
}

func containmentDuplicate(a, b string) bool {
	if !(strings.Contains(a, b) || strings.Contains(b, a)) {
		return false
	}
	shorter, longer := a, b
	if len(shorter) > len(longer) {
		shorter, longer = longer, shorter
	}
	shortWords := strings.Fields(shorter)
	longWords := strings.Fields(longer)
	if len(shortWords) == 0 || len(longWords) == 0 {
		return false
	}
	if len(shortWords) == 1 {
		return true
	}
	if len(longWords)-len(shortWords) <= 1 {
		return true
	}
	return false
}

func isNaturalSearchPhrase(value string) bool {
	value = normalizeTopicPhrase(value)
	if !ValidCreatorPhrase(value, nil, nil, true) {
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
	if disclaimerPattern().MatchString(value) {
		return false
	}
	return !regexp.MustCompile(`(?i)\butm|http|affiliate|sponsor|instagram|tiktok|facebook|twitter|financial advice|own research|educational purposes\b`).MatchString(value)
}

func meaningfulTitleSegments(value string) []string {
	parts := regexp.MustCompile(`(?i)\s+(\||-|–|—|:|\(|\)|/|\bvs\b|\bversus\b)\s+`).Split(value, -1)
	out := []string{}
	for _, part := range parts {
		part = normalizeTopicPhrase(part)
		if part != "" && len(strings.Fields(part)) >= 2 && len(strings.Fields(part)) <= 5 && !isGenericTopicPhrase(part) {
			out = append(out, part)
		}
	}
	return out
}

func isGenericTopicPhrase(value string) bool {
	phrase := normalizeTopicPhrase(value)
	if phrase == "" {
		return true
	}
	generic := map[string]bool{
		"concept": true, "concepts": true, "video": true, "tutorial": true, "education": true, "beginner": true, "beginners": true,
		"explained": true, "explain": true, "guide": true, "lesson": true, "course": true, "basics": true, "basic": true,
	}
	if generic[phrase] {
		return true
	}
	words := strings.Fields(phrase)
	meaningful := 0
	for _, word := range words {
		if !generic[word] && !stopWords[word] && !lowInformationWords[word] {
			meaningful++
		}
	}
	if meaningful == 0 {
		return true
	}
	if meaningful == 1 && len(words) >= 2 {
		for _, word := range words {
			if generic[word] {
				return true
			}
		}
	}
	return false
}

func phraseSpecificityBonus(phrase string) float64 {
	words := strings.Fields(normalizeTopicPhrase(phrase))
	score := float64(len(words)) * 0.7
	for _, word := range words {
		if len(word) >= 7 {
			score += 0.8
		}
		if isKnownAcronym(word) {
			score += 1.2
		}
	}
	return score
}

func topicDedupeKey(value string) string {
	words := strings.Fields(normalizeTopicPhrase(value))
	out := make([]string, 0, len(words))
	hasFairValueGap := strings.Contains(strings.Join(words, " "), "fair value gap")
	for _, word := range words {
		switch word {
		case "concept", "concepts", "tutorial", "explained", "explain", "guide", "lesson", "beginner", "beginners", "basic", "basics", "example", "examples", "setup", "setups":
			continue
		case "fvg":
			if hasFairValueGap {
				continue
			}
			out = append(out, word)
		default:
			out = append(out, word)
		}
	}
	if len(out) == 0 {
		return strings.Join(words, " ")
	}
	return strings.Join(out, " ")
}

func isNoisePhrase(value string) bool {
	lower := normalizeTopicPhrase(value)
	if lower == "" {
		return true
	}
	if containsBoilerplateFragment(lower) {
		return true
	}
	if disclaimerPattern().MatchString(lower) {
		return true
	}
	noise := []string{
		"utm", "browser made possible", "made possible", "possible grant", "friends scrimba", "our friends scrimba", "scrimba", "scrimba contents", "dub track", "change dub", "hindi dubbed", "melt labs",
		"home cooks supports", "supports content", "browse pots", "provided referral", "referral meaning", "clip licensing",
		"licensed under", "creative commons", "kevin mac leod", "kevin mac", "monkeys spinning", "spinning monkeys", "send clips", "funny pictures visit",
		"clips funny pictures", "pictures visit", "mac leod", "leod artist", "matthew campen", "campen msph", "msph intro", "intro cool", "cool end", "support our mission", "ted member",
		"always own research", "own research consult", "future results", "past performance", "funded days purposes", "making investment decisions",
		"buy sell", "should buy", "should sell", "guaranteed past", "educational purposes", "financial adviser", "financial advisor",
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
	if containsAny([]string{haystack}, []string{"ict", "forex", "trade entry", "missed entry", "missed trade", "price action"}) {
		if strings.Contains(haystack, "beginner") || strings.Contains(haystack, "learn") {
			return "beginner traders"
		}
		if strings.Contains(haystack, "ict") {
			return "ICT traders"
		}
		return "traders"
	}
	if strings.Contains(haystack, "beginner") || strings.Contains(haystack, "learn") {
		return "beginners learning the topic"
	}
	switch niche {
	case "Software and technology", "AI-assisted development", "Consumer technology":
		return "tech-curious creators and buyers"
	case "Trading education":
		return "traders"
	case "Finance education":
		return "operators, founders, and finance-minded viewers"
	case "Practical tutorial":
		return "learners seeking practical steps"
	case "Gaming guide or entertainment":
		return "gaming fans and players"
	case "Film and animation", "General entertainment", "Comedy and skits":
		return "entertainment fans"
	default:
		return "viewers interested in " + strings.ToLower(niche)
	}
}

func inferContentAngle(format, sub, niche string) string {
	return "A " + format + " angle for " + applyAcronymCasing(sub) + " within " + applyAcronymCasing(niche) + ", inferred from public metadata."
}

func grammaticallyMeaningfulTopic(kw KeywordIntelligence, title, fallback string) string {
	titleTokens := tokenizeUseful(title, map[string]bool{})
	bestTitlePhrase := ""
	bestTitleScore := -1
	for n := minInt(4, len(titleTokens)); n >= 2; n-- {
		for _, phrase := range ngrams(titleTokens, n) {
			if isNaturalSearchPhrase(phrase) && !isGenericTopicPhrase(phrase) {
				score := titleTopicPhraseScore(phrase)
				if score > bestTitleScore {
					bestTitlePhrase = phrase
					bestTitleScore = score
				}
			}
		}
	}
	if bestTitlePhrase != "" {
		return bestTitlePhrase
	}
	candidates := append(append([]string{}, kw.PrimaryTopics...), kw.PrimaryKeywords...)
	candidates = append(candidates, kw.SearchPhrases...)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if isNaturalSearchPhrase(candidate) && !isGenericTopicPhrase(candidate) {
			return applyAcronymCasing(candidate)
		}
	}
	return fallback
}

func titleTopicPhraseScore(phrase string) int {
	words := strings.Fields(normalizeTopicPhrase(phrase))
	if len(words) == 0 {
		return -100
	}
	score := len(words) * 10
	if len(words) >= 2 && len(words) <= 4 {
		score += 20
	}
	if len(words) > 5 {
		score -= 20
	}
	last := words[len(words)-1]
	if last == "true" || last == "fair" || last == "value" || last == "and" {
		score -= 30
	}
	if len(words[0]) <= 3 && len(words) >= 4 {
		score -= 8
	}
	if isGenericTopicPhrase(phrase) {
		score -= 60
	}
	return score
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
		if containsAny([]string{haystack}, []string{"ict", "forex", "trading", "trade entry", "missed entry", "missed trade", "price action"}) {
			return "Business and finance", "Trading education", true
		}
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

func normalizeTopicPhrase(value string) string {
	value = strings.ToLower(strings.TrimSpace(normalizeCamelArtifact(value)))
	value = strings.ReplaceAll(value, "&", " and ")
	value = regexp.MustCompile(`[^a-z0-9#+\-/\s]`).ReplaceAllString(value, " ")
	value = strings.ReplaceAll(value, "_", " ")
	value = regexp.MustCompile(`\s+`).ReplaceAllString(value, " ")
	words := strings.Fields(value)
	if len(words) == 0 {
		return ""
	}
	out := make([]string, 0, len(words))
	pluralExceptions := map[string]bool{"concepts": true, "entries": true, "photosynthesis": true, "redis": true, "kubernetes": true, "physics": true, "mathematics": true, "analytics": true}
	for _, word := range words {
		word = strings.Trim(word, "-/ ")
		if word == "" {
			continue
		}
		if len(word) > 4 && strings.HasSuffix(word, "s") && !pluralExceptions[word] && !strings.HasSuffix(word, "ss") && !strings.HasSuffix(word, "sis") && !strings.HasSuffix(word, "us") {
			word = strings.TrimSuffix(word, "s")
		}
		if len(out) == 0 || out[len(out)-1] != word {
			out = append(out, word)
		}
	}
	return strings.Join(out, " ")
}

func applyAcronymCasingList(values []string) []string {
	out := []string{}
	for _, value := range values {
		cased := applyAcronymCasing(value)
		if cased != "" && !nearDuplicateSelected(cased, out) {
			out = append(out, cased)
		}
	}
	return out
}

func applyAcronymCasing(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	words := strings.Fields(value)
	for i, word := range words {
		trimmed := strings.Trim(word, ".,;:!?()[]{}")
		if isKnownAcronym(trimmed) {
			upper := strings.ToUpper(trimmed)
			words[i] = strings.Replace(word, trimmed, upper, 1)
		}
	}
	out := strings.Join(words, " ")
	out = regexp.MustCompile(`(?i)\bfvg\b`).ReplaceAllString(out, "FVG")
	return strings.TrimSpace(out)
}

func isKnownAcronym(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ict", "rpm", "seo", "api", "ai", "url", "ctr", "fvg":
		return true
	default:
		return false
	}
}

func tokenSet(tokens []string) map[string]bool {
	out := map[string]bool{}
	for _, token := range tokens {
		out[normalizeTopicPhrase(token)] = true
	}
	return out
}

func isRelevantPhrase(term string, evidenceTokens, titleTokens map[string]bool, score float64) bool {
	term = normalizeTopicPhrase(term)
	if term == "" || isNoisePhrase(term) {
		return false
	}
	words := strings.Fields(term)
	if len(words) == 1 {
		return titleTokens[term] || score >= 3.5
	}
	matches := 0
	titleMatches := 0
	meaningful := 0
	for _, word := range words {
		if isUsefulTerm(word) {
			meaningful++
		}
		if evidenceTokens[word] {
			matches++
		}
		if titleTokens[word] {
			titleMatches++
		}
	}
	if meaningful < 2 {
		return false
	}
	if titleMatches > 0 && matches >= meaningful-1 {
		return true
	}
	return matches >= meaningful && score >= 1.2
}

func filterQualityPhrases(values []string, evidenceTokens, titleTokens map[string]bool, limit int) []string {
	out := []string{}
	for _, value := range values {
		phrase := normalizeTopicPhrase(value)
		if !ValidCreatorPhrase(phrase, evidenceTokens, titleTokens, true) || !isRelevantPhrase(phrase, evidenceTokens, titleTokens, 3.5) || nearDuplicateSelected(phrase, out) {
			continue
		}
		out = append(out, phrase)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func firstHighQualityPhrase(values []string) string {
	for _, value := range values {
		phrase := normalizeTopicPhrase(value)
		if isUsefulTerm(phrase) && !isGenericCreativeSeed(phrase) {
			return phrase
		}
	}
	return ""
}

func bestCreatorCore(niche NicheAnalysis, kw KeywordIntelligence, title string) string {
	candidates := append([]string{}, kw.LongTailPhrases...)
	candidates = append(candidates, kw.PrimaryTopics...)
	candidates = append(candidates, kw.PrimaryKeywords...)
	candidates = append(candidates, kw.SecondaryKeywords...)
	candidates = append(candidates, niche.SpecificTopic)
	titleTokens := tokenSet(tokenizeUseful(cleanMetadataText(title), map[string]bool{}))
	best := ""
	bestScore := -1
	for _, candidate := range candidates {
		phrase := normalizeTopicPhrase(candidate)
		if !isUsefulTerm(phrase) || isGenericCreativeSeed(phrase) || creativeOutputRejected(phrase) {
			continue
		}
		score := creatorCoreScore(phrase, titleTokens)
		if score > bestScore {
			best = phrase
			bestScore = score
		}
	}
	return best
}

func creatorCoreScore(phrase string, titleTokens map[string]bool) int {
	words := strings.Fields(normalizeTopicPhrase(phrase))
	if len(words) == 0 {
		return -100
	}
	score := len(words) * 8
	if len(words) >= 2 && len(words) <= 4 {
		score += 18
	}
	if len(words) > 5 {
		score -= 20
	}
	titleMatches := 0
	for _, word := range words {
		if lowInformationWords[word] || stopWords[word] {
			score -= 12
		}
		if titleTokens[word] {
			titleMatches++
		}
	}
	if titleMatches == len(words) {
		score += 35
		if len(words) == 1 {
			score += 20
		}
	} else if titleMatches > 0 {
		score += titleMatches * 8
	}
	if regexp.MustCompile(`(?i)\b(explain|explained|explains|learn|updated|minute|every)\b`).MatchString(phrase) {
		score -= 10
	}
	return score
}

func isGenericCreativeSeed(value string) bool {
	value = normalizeTopicPhrase(value)
	if value == "" || isNoisePhrase(value) {
		return true
	}
	words := strings.Fields(value)
	if len(words) == 1 {
		return lowInformationWords[value] || stopWords[value]
	}
	return false
}

func filterCreativeOutputs(values []string) []string {
	out := []string{}
	for _, value := range values {
		cleaned := cleanupGeneratedText(value)
		if cleaned == "" || creativeOutputRejected(cleaned) || isTruncatedGeneratedTitle(cleaned) || nearDuplicateSelected(cleaned, out) {
			continue
		}
		out = append(out, cleaned)
	}
	return out
}

func creativeOutputRejected(value string) bool {
	lower := strings.ToLower(value)
	if containsBoilerplateFragment(lower) {
		return true
	}
	if disclaimerPattern().MatchString(lower) {
		return true
	}
	if regexp.MustCompile(`(?i)\b(not|own research|future results|past performance|buy sell|should buy|should sell|funded days|educational purposes only)\b`).MatchString(lower) {
		return true
	}
	if regexp.MustCompile(`(?i)\b(explained in|using these inferred public topics:\s*(not|financial|finance)\b)`).MatchString(lower) {
		return true
	}
	if regexp.MustCompile(`(?i)\b(cleaned metadata|evidence basis|claiming private performance data|schema|provider|youtube_data_api)\b`).MatchString(lower) {
		return true
	}
	return false
}

func cleanupGeneratedText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = regexp.MustCompile(`\s+`).ReplaceAllString(value, " ")
	value = strings.ReplaceAll(value, ";.", ".")
	value = strings.ReplaceAll(value, "..", ".")
	value = strings.ReplaceAll(value, " ,", ",")
	value = strings.ReplaceAll(value, " .", ".")
	value = applyAcronymCasing(value)
	value = strings.TrimSpace(value)
	for regexp.MustCompile(`[;:,]\s*$`).MatchString(value) {
		value = strings.TrimSpace(value[:len(value)-1])
	}
	return value
}

func isTruncatedGeneratedTitle(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	last := strings.ToLower(strings.Trim(value[strings.LastIndex(value, " ")+1:], ".!?;:, "))
	switch last {
	case "the", "a", "an", "to", "from", "of", "for", "with", "and", "or", "but", "into", "by":
		return true
	default:
		return false
	}
}

func containsBoilerplateFragment(value string) bool {
	lower := normalizeTopicPhrase(value)
	fragments := []string{
		"not financial advice",
		"financial advice",
		"own research",
		"past performance",
		"future result",
		"hypothetical performance",
		"hypothetical simulated performance",
		"simulated performance",
		"actual performance",
		"actual trading",
		"benefit hindsight",
		"benefit of hindsight",
		"hindsight representation",
		"representation being",
		"representation made",
		"no representation",
		"results may vary",
		"limitation unlike actual performance",
		"trading result",
		"investment decision",
		"licensed financial",
		"educational purpose",
		"educational entertainment",
		"education entertainment",
		"entertainment trade risk",
		"trade risk",
		"should buy",
		"should sell",
		"buy sell",
		"affiliate disclosure",
		"paid promotion",
		"copyright disclaimer",
	}
	for _, fragment := range fragments {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

func ValidCreatorPhrase(value string, evidenceTokens, titleTokens map[string]bool, requirePhrase bool) bool {
	if hasRepeatedRawWords(value) {
		return false
	}
	phrase := normalizeTopicPhrase(value)
	if phrase == "" || containsBoilerplateFragment(phrase) || isNoisePhrase(phrase) || isGenericTopicPhrase(phrase) || isGenericAudiencePlaceholder(phrase) {
		return false
	}
	words := strings.Fields(phrase)
	if requirePhrase && (len(words) < 2 || len(words) > 6) {
		return false
	}
	if hasRepeatedWords(words) || hasMalformedAdjacency(phrase) {
		return false
	}
	if len(words) > 1 && (stopWords[words[0]] || stopWords[words[len(words)-1]]) {
		return false
	}
	meaningful := 0
	for _, word := range words {
		if !stopWords[word] && !lowInformationWords[word] {
			meaningful++
		}
	}
	if len(words) > 1 && float64(meaningful)/float64(len(words)) < 0.6 {
		return false
	}
	if evidenceTokens != nil && titleTokens != nil && requirePhrase {
		supported := 0
		for _, word := range words {
			if evidenceTokens[word] || titleTokens[word] || isKnownAcronym(word) {
				supported++
			}
		}
		if supported < minInt(2, len(words)) {
			return false
		}
	}
	return true
}

func validatePhraseList(values []string, evidenceTokens, titleTokens map[string]bool, limit int, requirePhrase bool) []string {
	out := []string{}
	for _, value := range values {
		phrase := normalizeTopicPhrase(value)
		if !ValidCreatorPhrase(phrase, evidenceTokens, titleTokens, requirePhrase) || nearDuplicateSelected(phrase, out) {
			continue
		}
		out = append(out, phrase)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func hasRepeatedWords(words []string) bool {
	for i := 1; i < len(words); i++ {
		if words[i] == words[i-1] {
			return true
		}
	}
	if len(words) >= 4 && strings.Join(words[:2], " ") == strings.Join(words[2:4], " ") {
		return true
	}
	return false
}

func hasRepeatedRawWords(value string) bool {
	words := strings.Fields(strings.ToLower(value))
	for i := 1; i < len(words); i++ {
		if strings.Trim(words[i], ".,;:!?") == strings.Trim(words[i-1], ".,;:!?") {
			return true
		}
	}
	return false
}

func hasMalformedAdjacency(phrase string) bool {
	bad := []string{
		"missed entry navigate", "entry navigate", "navigate same", "designed benefit", "benefit hindsight",
		"hindsight representation", "representation being", "rule hypothetical", "hypothetical simulated",
		"limitation unlike", "unlike actual", "actual performance", "made benefit",
		"entry using", "using ict execution", "execution idea", "trade entry using",
	}
	for _, item := range bad {
		if strings.Contains(phrase, item) {
			return true
		}
	}
	return false
}

func isGenericAudiencePlaceholder(value string) bool {
	lower := normalizeTopicPhrase(value)
	switch lower {
	case "general viewer", "general viewers", "general viewers niche", "people interested topic", "people interested", "content creator", "content creators", "broad audience", "relevant viewer", "relevant viewers", "creator audience":
		return true
	default:
		return strings.Contains(lower, "general viewer") || strings.Contains(lower, "people interested in") || strings.Contains(lower, "broad audience")
	}
}

func cleanTitleTopicFallback(title string, evidenceTokens, titleTokens map[string]bool) string {
	cleaned := cleanMetadataText(title)
	tokens := tokenizeUseful(cleaned, map[string]bool{})
	best := ""
	bestScore := -100
	for n := minInt(5, len(tokens)); n >= 2; n-- {
		for _, phrase := range ngrams(tokens, n) {
			if !ValidCreatorPhrase(phrase, evidenceTokens, titleTokens, true) {
				continue
			}
			score := titleTopicPhraseScore(phrase)
			if score > bestScore {
				best = phrase
				bestScore = score
			}
		}
	}
	return best
}

func cleanTitleSingleSubject(title string) string {
	tokens := tokenizeUseful(cleanMetadataText(title), map[string]bool{})
	if len(tokens) == 0 {
		return ""
	}
	if len(tokens) > 3 {
		return ""
	}
	for _, token := range tokens {
		phrase := normalizeTopicPhrase(token)
		if phrase != "" && !stopWords[phrase] && !lowInformationWords[phrase] && !isGenericTopicPhrase(phrase) && !containsBoilerplateFragment(phrase) {
			return phrase
		}
	}
	return ""
}

func cleanSingleTagSubject(tag string) string {
	tokens := tokenizeUseful(cleanMetadataText(tag), map[string]bool{})
	if len(tokens) != 1 {
		return ""
	}
	phrase := normalizeTopicPhrase(tokens[0])
	if phrase != "" && !stopWords[phrase] && !lowInformationWords[phrase] && !isGenericTopicPhrase(phrase) && !containsBoilerplateFragment(phrase) {
		return phrase
	}
	return ""
}

func containsStringNormalized(values []string, value string) bool {
	key := normalizeTopicPhrase(value)
	for _, candidate := range values {
		if normalizeTopicPhrase(candidate) == key {
			return true
		}
	}
	return false
}

func cleanTitleForCreativePrompt(title string) string {
	title = strings.TrimSpace(title)
	if title == "" || containsBoilerplateFragment(title) {
		return cleanMetadataText(title)
	}
	title = regexp.MustCompile(`https?://\S+|www\.\S+`).ReplaceAllString(title, " ")
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
	return strings.TrimSpace(title)
}

func naturalSearchPhrasesFromTopics(values []string, limit int) []string {
	out := []string{}
	for _, value := range values {
		phrase := normalizeTopicPhrase(value)
		if !ValidCreatorPhrase(phrase, nil, nil, true) || nearDuplicateSelected(phrase, out) {
			continue
		}
		out = append(out, phrase)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func audienceOrFallback(values ...string) string {
	for _, value := range values {
		normalized := normalizeTopicPhrase(value)
		if normalized != "" && !isGenericAudiencePlaceholder(normalized) && !containsBoilerplateFragment(normalized) {
			return applyAcronymCasing(normalized)
		}
	}
	return "the likely viewer"
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
		clean := strings.Trim(word, ".,;:!?()[]{}")
		if isKnownAcronym(clean) {
			words[i] = strings.Replace(word, clean, strings.ToUpper(clean), 1)
		} else if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return applyAcronymCasing(strings.Join(words, " "))
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
