package trendintel

import "time"

type SearchMode string

const (
	ModeDiscover SearchMode = "discover"
	ModeKeyword  SearchMode = "keyword"
)

type Query struct {
	Mode                SearchMode
	Keyword             string
	Include             []string
	Exclude             []string
	ExactPhrase         string
	Country             string
	Language            string
	Location            string
	LocalVideosOnly     bool
	LocalRadiusKM       int
	Window              string
	Category            string
	CustomNiche         string
	VideoDuration       string
	MinViews            *uint64
	MaxCompetition      *float64
	MinOpportunityScore *float64
	Sort                string
	Limit               int
	Refresh             bool
}

type Response struct {
	Status           string            `json:"status"`
	Message          string            `json:"message,omitempty"`
	Mode             SearchMode        `json:"mode"`
	Query            string            `json:"query,omitempty"`
	Country          string            `json:"country"`
	CountryName      string            `json:"country_name"`
	Language         string            `json:"language"`
	LanguageName     string            `json:"language_name"`
	TimeWindow       string            `json:"time_window"`
	Category         string            `json:"category"`
	ResolvedLocation *ResolvedLocation `json:"resolved_location,omitempty"`
	LocalVideosOnly  bool              `json:"local_videos_only"`
	LocalRadiusKM    int               `json:"local_radius_km,omitempty"`
	Results          []Trend           `json:"results"`
	FetchedAt        time.Time         `json:"fetched_at"`
	Cache            CacheInfo         `json:"cache"`
}

type Trend struct {
	ID                         string     `json:"id"`
	Keyword                    string     `json:"keyword"`
	NormalizedKeyword          string     `json:"normalized_keyword"`
	DisplayTitle               string     `json:"display_title"`
	Summary                    string     `json:"summary,omitempty"`
	CountryCode                string     `json:"country_code"`
	LanguageCode               string     `json:"language_code,omitempty"`
	Region                     string     `json:"region,omitempty"`
	Category                   string     `json:"category,omitempty"`
	RelatedKeywords            []string   `json:"related_keywords,omitempty"`
	SearchVolumeText           *string    `json:"search_volume_text,omitempty"`
	PublishedAt                *time.Time `json:"published_at,omitempty"`
	DiscoveredAt               time.Time  `json:"discovered_at"`
	TrendAge                   *string    `json:"trend_age,omitempty"`
	TrendVelocity              *float64   `json:"trend_velocity,omitempty"`
	VideoCountSampled          *int       `json:"video_count_sampled,omitempty"`
	TotalSampledViews          *uint64    `json:"total_sampled_views,omitempty"`
	MedianSampledViews         *uint64    `json:"median_sampled_views,omitempty"`
	AverageSampledViews        *uint64    `json:"average_sampled_views,omitempty"`
	TotalSampledLikes          *uint64    `json:"total_sampled_likes,omitempty"`
	TotalSampledComments       *uint64    `json:"total_sampled_comments,omitempty"`
	NewestRelevantVideoAt      *time.Time `json:"newest_relevant_video_at,omitempty"`
	StrongestRelevantVideoURL  string     `json:"strongest_relevant_video_url,omitempty"`
	StrongestThumbnailURL      string     `json:"strongest_thumbnail_url,omitempty"`
	ConfidenceScore            float64    `json:"confidence_score"`
	OpportunityScore           float64    `json:"opportunity_score"`
	MomentumScore              float64    `json:"momentum_score"`
	DemandScore                float64    `json:"demand_score"`
	CompetitionScore           float64    `json:"competition_score"`
	ScoringReasons             []string   `json:"scoring_reasons"`
	SampledVideoActivity       string     `json:"sampled_video_activity,omitempty"`
	SupportingContentAvailable bool       `json:"supporting_content_available"`
	FetchedAt                  time.Time  `json:"fetched_at"`

	provenance []SourceProvenance
}

type SourceProvenance struct {
	Provider    string    `json:"provider"`
	RequestType string    `json:"request_type"`
	URL         string    `json:"url,omitempty"`
	FetchedAt   time.Time `json:"fetched_at"`
}

type CacheInfo struct {
	Hit      bool       `json:"hit"`
	StoredAt *time.Time `json:"stored_at,omitempty"`
	TTL      string     `json:"ttl"`
}

type FilterMetadata struct {
	Countries       []Option `json:"countries"`
	Languages       []Option `json:"languages"`
	TimeWindows     []Option `json:"time_windows"`
	Categories      []Option `json:"categories"`
	VideoDurations  []Option `json:"video_durations"`
	LocalRadiiKM    []int    `json:"local_radii_km"`
	SortOrders      []Option `json:"sort_orders"`
	DefaultCountry  string   `json:"default_country"`
	DefaultLanguage string   `json:"default_language"`
}

type Option struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type ResolvedLocation struct {
	Input     string   `json:"input"`
	Country   string   `json:"country"`
	Region    string   `json:"region,omitempty"`
	City      string   `json:"city,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	Status    string   `json:"status"`
	Message   string   `json:"message,omitempty"`
}
