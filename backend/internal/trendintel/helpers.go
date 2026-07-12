package trendintel

import (
	"sort"
	"strings"
	"time"
)

func dedupeSeeds(seeds []seedKeyword) []seedKeyword {
	seen := map[string]bool{}
	out := []seedKeyword{}
	for _, seed := range seeds {
		key := NormalizeKeyword(seed.Keyword)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, seed)
	}
	return out
}

func dedupeTrends(in []Trend) []Trend {
	seen := map[string]Trend{}
	for _, t := range in {
		key := t.NormalizedKeyword
		existing, ok := seen[key]
		if !ok || t.OpportunityScore > existing.OpportunityScore {
			seen[key] = t
		}
	}
	out := make([]Trend, 0, len(seen))
	for _, t := range seen {
		out = append(out, t)
	}
	return out
}

func sortTrends(trends []Trend, sortBy string) {
	sort.SliceStable(trends, func(i, j int) bool {
		switch sortBy {
		case "momentum":
			return trends[i].MomentumScore > trends[j].MomentumScore
		case "demand":
			return trends[i].DemandScore > trends[j].DemandScore
		case "competition":
			return trends[i].CompetitionScore < trends[j].CompetitionScore
		case "newest":
			return trends[i].DiscoveredAt.After(trends[j].DiscoveredAt)
		default:
			return trends[i].OpportunityScore > trends[j].OpportunityScore
		}
	})
}

func relatedKeywords(seed string, videos []Video) []string {
	counts := map[string]int{}
	seedNorm := NormalizeKeyword(seed)
	for _, v := range videos {
		for _, term := range append(extractTerms(v.Title), v.Tags...) {
			norm := NormalizeKeyword(term)
			if norm == "" || norm == seedNorm || len(norm) < 3 {
				continue
			}
			counts[norm]++
		}
	}
	type pair struct {
		term  string
		count int
	}
	pairs := []pair{}
	for term, count := range counts {
		pairs = append(pairs, pair{term, count})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].count > pairs[j].count })
	out := []string{}
	for _, p := range pairs {
		out = append(out, p.term)
		if len(out) >= 6 {
			break
		}
	}
	return out
}

func videosMatching(term string, videos []Video) []Video {
	norm := NormalizeKeyword(term)
	out := []Video{}
	for _, v := range videos {
		if strings.Contains(NormalizeKeyword(v.Title+" "+v.Description+" "+strings.Join(v.Tags, " ")), norm) {
			out = append(out, v)
		}
	}
	return out
}

func extractTerms(title string) []string {
	words := strings.Fields(NormalizeKeyword(title))
	out := []string{}
	stop := map[string]bool{"the": true, "and": true, "for": true, "with": true, "from": true, "this": true, "that": true, "you": true, "video": true, "short": true, "new": true, "best": true}
	for i, word := range words {
		if len(word) >= 4 && !stop[word] {
			out = append(out, word)
		}
		if i+1 < len(words) {
			bigram := word + " " + words[i+1]
			if len(bigram) >= 7 && !stop[word] && !stop[words[i+1]] {
				out = append(out, bigram)
			}
		}
	}
	return out
}

func engagement(videos []Video) float64 {
	var views, likes, comments uint64
	for _, v := range videos {
		if v.Views != nil {
			views += *v.Views
		}
		if v.Likes != nil {
			likes += *v.Likes
		}
		if v.Comments != nil {
			comments += *v.Comments
		}
	}
	if views == 0 {
		return 0.25
	}
	return clampFloat(float64(likes+comments*2)/float64(views), 0, 1)
}

func medianViewStrength(videos []Video) float64 {
	_, _, median, _, _, _, _, _, _ := videoStats(videos)
	if median == nil {
		return 0.25
	}
	switch {
	case *median >= 1000000:
		return 1
	case *median >= 100000:
		return 0.75
	case *median >= 10000:
		return 0.5
	default:
		return 0.25
	}
}

func humanAge(d time.Duration) string {
	if d < time.Hour {
		return "under 1 hour"
	}
	if d < 48*time.Hour {
		return strconvInt(int(d.Hours())) + " hours"
	}
	return strconvInt(int(d.Hours()/24)) + " days"
}

func topStrings(in []string, n int) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range in {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
		if len(out) >= n {
			break
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func titleCase(raw string) string {
	words := strings.Fields(raw)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

func ptrInt(v int) *int { return &v }

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func round(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

func emptyAll(value string) string {
	if value == "all" {
		return ""
	}
	return value
}

func regionFromLocation(q Query) string {
	if q.Location != "" {
		return q.Location
	}
	return ""
}

func strconvInt(v int) string {
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
