// Package scoring computes a deterministic topic score for a trend item.
// Every component is a pure function of fields already stored in the
// database (trend_items, trend_sources) — no network calls, no randomness.
package scoring

import (
	"fmt"
	"math"
	"strings"
)

// Input is the subset of trend_item + trend_source fields the scorer needs.
type Input struct {
	Topic            string
	Description      string
	PlatformHint     string
	Velocity         float64 // 0-100, growth/interest signal supplied by the source
	SourceConfidence float64 // 0-1, reliability of the trend_source
}

// Result is the full scoring breakdown, persisted to topic_scores.
type Result struct {
	TotalScore            float64
	VelocityScore         float64
	SourceConfidenceScore float64
	PlatformFitScore      float64
	SafetyScore           float64
	WatchTimeScore        float64
	CompetitionScore      float64
	Reason                string
	Flagged               bool // true when safety keywords were found
}

// weights sum to 1.0 — safety carries the heaviest weight on purpose.
const (
	weightVelocity   = 0.25
	weightConfidence = 0.15
	weightPlatform   = 0.15
	weightSafety     = 0.20
	weightWatchTime  = 0.15
	weightCompetition = 0.10
)

var platformFitTable = map[string]float64{
	"tiktok":    95,
	"instagram": 90,
	"reels":     90,
	"youtube":   85,
	"shorts":    85,
	"facebook":  70,
	"threads":   60,
	"x":         55,
}

// unsafeKeywords is a small deterministic content-safety denylist.
// Each match subtracts from the safety score; this is a foundation-phase
// heuristic, not a substitute for real moderation tooling.
var unsafeKeywords = []string{
	"violence", "gore", "hate", "nsfw", "explicit", "gambling", "weapon",
	"suicide", "self-harm", "drug abuse", "scam", "fraud", "extremist",
}

// engagingPatterns is a deterministic bonus list for watch-time potential —
// patterns historically correlated with short-form retention.
var engagingPatterns = []string{
	"how to", "top ", "vs ", "vs.", "mistakes", "secrets", "hack", "tips",
	"why ", "what if", "before and after", "in 60 seconds", "challenge",
}

// Score computes the deterministic topic score for a single trend item.
func Score(in Input) Result {
	velocityScore := clamp(in.Velocity, 0, 100)
	confidenceScore := clamp(in.SourceConfidence*100, 0, 100)
	platformScore := platformFitScore(in.PlatformHint)
	safetyScore, flagged, flaggedTerms := safetyScore(in.Topic, in.Description)
	watchTimeScore := watchTimeScore(in.Topic, in.Velocity)
	competitionScore := competitionScore(in.Topic, in.Velocity)

	total := velocityScore*weightVelocity +
		confidenceScore*weightConfidence +
		platformScore*weightPlatform +
		safetyScore*weightSafety +
		watchTimeScore*weightWatchTime +
		competitionScore*weightCompetition

	reason := buildReason(velocityScore, confidenceScore, platformScore, safetyScore, watchTimeScore, competitionScore, flagged, flaggedTerms)

	return Result{
		TotalScore:             round3(total),
		VelocityScore:          round3(velocityScore),
		SourceConfidenceScore:  round3(confidenceScore),
		PlatformFitScore:       round3(platformScore),
		SafetyScore:            round3(safetyScore),
		WatchTimeScore:         round3(watchTimeScore),
		CompetitionScore:       round3(competitionScore),
		Reason:                 reason,
		Flagged:                flagged,
	}
}

func platformFitScore(hint string) float64 {
	key := strings.ToLower(strings.TrimSpace(hint))
	if v, ok := platformFitTable[key]; ok {
		return v
	}
	if key == "" {
		return 50
	}
	return 40 // unrecognized platform hint
}

func safetyScore(topic, description string) (score float64, flagged bool, terms []string) {
	haystack := strings.ToLower(topic + " " + description)
	score = 100
	for _, kw := range unsafeKeywords {
		if strings.Contains(haystack, kw) {
			score -= 25
			flagged = true
			terms = append(terms, kw)
		}
	}
	return clamp(score, 0, 100), flagged, terms
}

func watchTimeScore(topic string, velocity float64) float64 {
	haystack := strings.ToLower(topic)
	score := 50.0
	bonusApplied := 0
	for _, p := range engagingPatterns {
		if bonusApplied >= 3 {
			break // cap the pattern bonus so one keyword-stuffed topic can't dominate
		}
		if strings.Contains(haystack, p) {
			score += 10
			bonusApplied++
		}
	}
	score += velocity * 0.2
	return clamp(score, 0, 100)
}

func competitionScore(topic string, velocity float64) float64 {
	// Higher = easier to win (less competitive). Very high velocity topics
	// tend to already be saturated; longer, more specific topic phrases
	// tend to be less contested.
	wordCount := len(strings.Fields(topic))
	specificityBonus := clamp(float64(wordCount-3)*5, 0, 20)
	score := 80 - velocity*0.5 + specificityBonus
	return clamp(score, 10, 90)
}

func buildReason(velocity, confidence, platform, safety, watchTime, competition float64, flagged bool, flaggedTerms []string) string {
	if flagged {
		return fmt.Sprintf("Safety flagged (matched: %s) — score suppressed regardless of other factors.", strings.Join(flaggedTerms, ", "))
	}

	type factor struct {
		name  string
		score float64
	}
	factors := []factor{
		{"velocity", velocity},
		{"source confidence", confidence},
		{"platform fit", platform},
		{"watch-time potential", watchTime},
		{"competition ease", competition},
	}
	best, worst := factors[0], factors[0]
	for _, f := range factors {
		if f.score > best.score {
			best = f
		}
		if f.score < worst.score {
			worst = f
		}
	}
	return fmt.Sprintf("Strongest factor: %s (%.0f). Weakest factor: %s (%.0f). Safety clean.", best.name, best.score, worst.name, worst.score)
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}
