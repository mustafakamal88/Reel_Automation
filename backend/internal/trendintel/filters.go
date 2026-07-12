package trendintel

import (
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

var supportedCountries = map[string]string{
	"GB": "United Kingdom",
	"US": "United States",
	"PK": "Pakistan",
	"IN": "India",
	"AE": "United Arab Emirates",
	"SA": "Saudi Arabia",
	"CA": "Canada",
	"AU": "Australia",
	"DE": "Germany",
	"FR": "France",
	"ES": "Spain",
	"PT": "Portugal",
	"BR": "Brazil",
}

var supportedLanguages = map[string]string{
	"en": "English",
	"ur": "Urdu",
	"ps": "Pashto",
	"ar": "Arabic",
	"hi": "Hindi",
	"es": "Spanish",
	"fr": "French",
	"de": "German",
	"pt": "Portuguese",
}

var categories = []Option{
	{"All", "all"},
	{"News", "news"},
	{"Technology", "technology"},
	{"Business", "business"},
	{"Finance", "finance"},
	{"Entertainment", "entertainment"},
	{"Gaming", "gaming"},
	{"Sports", "sports"},
	{"Education", "education"},
	{"Health", "health"},
	{"Fitness", "fitness"},
	{"Food", "food"},
	{"Beauty", "beauty"},
	{"Fashion", "fashion"},
	{"Travel", "travel"},
	{"Science", "science"},
	{"Politics", "politics"},
	{"Religion", "religion"},
	{"Comedy", "comedy"},
	{"Lifestyle", "lifestyle"},
}

var timeWindows = map[string]time.Duration{
	"4h":  4 * time.Hour,
	"24h": 24 * time.Hour,
	"48h": 48 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
}

func Metadata(defaultCountry string) FilterMetadata {
	countries := make([]Option, 0, len(supportedCountries))
	for code, name := range supportedCountries {
		countries = append(countries, Option{Label: name, Value: code})
	}
	sort.Slice(countries, func(i, j int) bool { return countries[i].Label < countries[j].Label })
	languages := make([]Option, 0, len(supportedLanguages))
	for code, name := range supportedLanguages {
		languages = append(languages, Option{Label: name, Value: code})
	}
	sort.Slice(languages, func(i, j int) bool { return languages[i].Label < languages[j].Label })
	return FilterMetadata{
		Countries:       countries,
		Languages:       languages,
		TimeWindows:     []Option{{"Last 4 hours", "4h"}, {"Last 24 hours", "24h"}, {"Last 48 hours", "48h"}, {"Last 7 days", "7d"}, {"Last 30 days", "30d"}},
		Categories:      categories,
		VideoDurations:  []Option{{"Any duration", "any"}, {"Short", "short"}, {"Medium", "medium"}, {"Long", "long"}},
		LocalRadiiKM:    []int{10, 25, 50, 100},
		SortOrders:      []Option{{"Best opportunity", "opportunity"}, {"Momentum", "momentum"}, {"Demand", "demand"}, {"Lower competition", "competition"}, {"Newest", "newest"}},
		DefaultCountry:  normalizeCountry(defaultCountry, "GB"),
		DefaultLanguage: "en",
	}
}

func normalizeQuery(q Query, fallbackCountry string) (Query, error) {
	q.Keyword = strings.TrimSpace(q.Keyword)
	q.ExactPhrase = strings.TrimSpace(q.ExactPhrase)
	q.Location = strings.TrimSpace(q.Location)
	q.CustomNiche = cleanText(q.CustomNiche, 80)
	if q.Mode == "" {
		if q.Keyword == "" {
			q.Mode = ModeDiscover
		} else {
			q.Mode = ModeKeyword
		}
	}
	if q.Mode != ModeDiscover && q.Mode != ModeKeyword {
		return q, errors.New("mode must be discover or keyword")
	}
	if q.Mode == ModeKeyword && q.Keyword == "" && q.ExactPhrase == "" {
		return q, errors.New("enter a keyword or exact phrase")
	}
	if len(q.Keyword) > 120 || len(q.ExactPhrase) > 120 || len(q.Location) > 120 {
		return q, errors.New("search inputs must be 120 characters or fewer")
	}
	q.Country = normalizeCountry(q.Country, fallbackCountry)
	if q.Country == "" {
		return q, errors.New("unsupported country")
	}
	q.Language = normalizeLanguageCode(q.Language)
	if q.Language == "" {
		return q, errors.New("unsupported language")
	}
	if _, ok := timeWindows[q.Window]; !ok {
		q.Window = "24h"
	}
	q.Category = normalizeCategory(q.Category)
	q.VideoDuration = strings.ToLower(strings.TrimSpace(q.VideoDuration))
	if q.VideoDuration == "" {
		q.VideoDuration = "any"
	}
	if q.VideoDuration != "any" && q.VideoDuration != "short" && q.VideoDuration != "medium" && q.VideoDuration != "long" {
		return q, errors.New("unsupported video duration")
	}
	if q.LocalRadiusKM == 0 {
		q.LocalRadiusKM = 25
	}
	if !validRadius(q.LocalRadiusKM) {
		return q, errors.New("local radius must be 10, 25, 50, or 100 km")
	}
	q.Sort = strings.ToLower(strings.TrimSpace(q.Sort))
	if q.Sort == "" {
		q.Sort = "opportunity"
	}
	q.Limit = clampInt(q.Limit, 1, 50, 20)
	q.Include = cleanTerms(q.Include, 12, 40)
	q.Exclude = cleanTerms(q.Exclude, 12, 40)
	return q, nil
}

func BuildYouTubeQuery(q Query, keyword string) string {
	parts := []string{}
	if q.ExactPhrase != "" {
		parts = append(parts, `"`+q.ExactPhrase+`"`)
	} else if keyword != "" {
		parts = append(parts, keyword)
	}
	parts = append(parts, q.Include...)
	if q.Category != "" && q.Category != "all" {
		parts = append(parts, categoryExpansion(q.Category))
	}
	if q.CustomNiche != "" {
		parts = append(parts, q.CustomNiche)
	}
	for _, term := range q.Exclude {
		parts = append(parts, "-"+term)
	}
	return strings.Join(parts, " ")
}

func PublishedAfter(window string, now time.Time) time.Time {
	return now.Add(-timeWindows[window])
}

func normalizeCountry(raw, fallback string) string {
	code := strings.ToUpper(strings.TrimSpace(raw))
	if code == "" {
		code = strings.ToUpper(strings.TrimSpace(fallback))
	}
	if _, ok := supportedCountries[code]; ok {
		return code
	}
	return ""
}

func normalizeLanguageCode(raw string) string {
	code := strings.ToLower(strings.TrimSpace(raw))
	if code == "" {
		return "en"
	}
	if i := strings.Index(code, "-"); i > 0 {
		code = code[:i]
	}
	if _, ok := supportedLanguages[code]; ok {
		return code
	}
	return ""
}

func NormalizeKeyword(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.ReplaceAll(raw, "a.i.", "ai")
	raw = strings.ReplaceAll(raw, "a.i", "ai")
	var b strings.Builder
	lastSpace := false
	for _, r := range raw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	words := strings.Fields(b.String())
	if len(words) > 1 {
		for i, word := range words {
			if len(word) > 3 && strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss") {
				words[i] = strings.TrimSuffix(word, "s")
			}
		}
	}
	return strings.Join(words, " ")
}

func StableID(country, language, keyword string) string {
	return "tr_" + safeKey(country+"_"+language+"_"+NormalizeKeyword(keyword))
}

func CacheKey(q Query, requestType string) string {
	values := url.Values{}
	values.Set("type", requestType)
	values.Set("mode", string(q.Mode))
	values.Set("q", NormalizeKeyword(q.Keyword+" "+q.ExactPhrase))
	values.Set("country", q.Country)
	values.Set("language", q.Language)
	values.Set("window", q.Window)
	values.Set("category", q.Category)
	values.Set("niche", q.CustomNiche)
	values.Set("location", NormalizeKeyword(q.Location))
	values.Set("local", boolString(q.LocalVideosOnly))
	values.Set("radius", intString(q.LocalRadiusKM))
	values.Set("duration", q.VideoDuration)
	values.Set("include", strings.Join(q.Include, ","))
	values.Set("exclude", strings.Join(q.Exclude, ","))
	return values.Encode()
}

func cleanTerms(terms []string, maxTerms, maxLen int) []string {
	out := []string{}
	for _, raw := range terms {
		for _, split := range strings.Split(raw, ",") {
			term := cleanText(split, maxLen)
			if term != "" {
				out = append(out, term)
			}
			if len(out) >= maxTerms {
				return out
			}
		}
	}
	return out
}

var safeTextRe = regexp.MustCompile(`[^a-zA-Z0-9 '\-]+`)

func cleanText(raw string, maxLen int) string {
	raw = safeTextRe.ReplaceAllString(strings.TrimSpace(raw), " ")
	raw = strings.Join(strings.Fields(raw), " ")
	if len(raw) > maxLen {
		raw = raw[:maxLen]
	}
	return raw
}

func normalizeCategory(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "all"
	}
	for _, c := range categories {
		if raw == c.Value {
			return raw
		}
	}
	return "all"
}

func categoryExpansion(category string) string {
	switch category {
	case "technology":
		return "tech"
	case "business":
		return "business"
	case "finance":
		return "finance money"
	case "entertainment":
		return "entertainment"
	default:
		return category
	}
}

func validRadius(km int) bool {
	return km == 10 || km == 25 || km == 50 || km == 100
}

func clampInt(v, min, max, fallback int) int {
	if v == 0 {
		return fallback
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func intString(v int) string {
	if v == 0 {
		return "0"
	}
	digits := []byte{}
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}

func safeKey(raw string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(raw) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}
