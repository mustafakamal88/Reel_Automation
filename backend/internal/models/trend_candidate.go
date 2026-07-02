package models

import "time"

const (
	TrendCandidateStatusDiscovered = "discovered"
)

// TrendCandidate is a real provider-sourced discovery result. It is separate
// from trend_items because discovery can show raw provider candidates before
// the app decides whether to persist or score them.
type TrendCandidate struct {
	ID           string    `json:"id"`
	Source       string    `json:"source"`
	Region       string    `json:"region"`
	Language     string    `json:"language"`
	Keyword      string    `json:"keyword"`
	Title        string    `json:"title"`
	Score        float64   `json:"score"`
	Velocity     *float64  `json:"velocity,omitempty"`
	DiscoveredAt time.Time `json:"discovered_at"`
	SourceURL    string    `json:"source_url,omitempty"`
	Evidence     string    `json:"evidence,omitempty"`
	Status       string    `json:"status"`
}
