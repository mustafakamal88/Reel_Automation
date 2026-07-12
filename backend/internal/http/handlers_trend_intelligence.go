package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"trendcortex/api/internal/trendintel"
)

func (s *Server) trendIntelligenceService() *trendintel.Service {
	timeout, err := time.ParseDuration(s.cfg.TrendDiscoveryTimeout)
	if err != nil {
		timeout = 10 * time.Second
	}
	var cache trendintel.Cache
	if s.db != nil {
		cache = trendintel.NewDBCache(s.db.DB)
	}
	return trendintel.NewService(trendintel.ServiceConfig{
		TrendProvider:  s.cfg.TrendDiscoveryProvider,
		TrendBaseURL:   s.cfg.TrendDiscoveryBaseURL,
		TrendTimeout:   timeout,
		YouTubeAPIKey:  s.cfg.YouTubeAPIKey,
		DefaultCountry: s.cfg.DefaultTrendCountry,
		Cache:          cache,
	})
}

func (s *Server) handleTrendIntelligenceSearch(w http.ResponseWriter, r *http.Request) {
	q := trendQueryFromRequest(r)
	if strings.TrimSpace(r.URL.Query().Get("q")) != "" || strings.TrimSpace(r.URL.Query().Get("keyword")) != "" {
		q.Mode = trendintel.ModeKeyword
	} else {
		q.Mode = trendintel.ModeDiscover
	}
	res, err := s.trendIntelligenceService().Search(r.Context(), q)
	if err != nil {
		jsonErrorCode(w, "trend_intelligence_unavailable", "Trend intelligence is temporarily unavailable.", http.StatusServiceUnavailable)
		return
	}
	jsonOK(w, res)
}

func (s *Server) handleTrendIntelligenceFilters(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, trendintel.Metadata(s.cfg.DefaultTrendCountry))
}

func (s *Server) handleTrendIntelligenceDetail(w http.ResponseWriter, r *http.Request) {
	_ = r.PathValue("id")
	jsonErrorCode(w, "not_found", "Trend details are unavailable for this result. Run the search again to refresh the unified result set.", http.StatusNotFound)
}

func (s *Server) handleResolveLocation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Location string `json:"location"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		jsonErrorCode(w, "invalid_input", "location is required", http.StatusBadRequest)
		return
	}
	resolved := s.trendIntelligenceService().ResolveLocation(r.Context(), body.Location)
	jsonOK(w, resolved)
}

func trendQueryFromRequest(r *http.Request) trendintel.Query {
	values := r.URL.Query()
	q := trendintel.Query{
		Keyword:         firstQuery(valuesGet(values, "q"), valuesGet(values, "keyword")),
		ExactPhrase:     valuesGet(values, "exact_phrase"),
		Country:         valuesGet(values, "country"),
		Language:        valuesGet(values, "language"),
		Location:        firstQuery(valuesGet(values, "postcode"), valuesGet(values, "location")),
		LocalVideosOnly: parseBool(valuesGet(values, "local_videos_only")),
		Window:          valuesGet(values, "window"),
		Category:        valuesGet(values, "category"),
		CustomNiche:     valuesGet(values, "niche"),
		VideoDuration:   valuesGet(values, "video_duration"),
		Sort:            valuesGet(values, "sort"),
		Refresh:         parseBool(valuesGet(values, "refresh")),
	}
	q.Include = append(q.Include, values["include"]...)
	q.Include = append(q.Include, splitCSV(valuesGet(values, "include_words"))...)
	q.Exclude = append(q.Exclude, values["exclude"]...)
	q.Exclude = append(q.Exclude, splitCSV(valuesGet(values, "exclude_words"))...)
	if v, err := strconv.Atoi(valuesGet(values, "radius_km")); err == nil {
		q.LocalRadiusKM = v
	}
	if v, err := strconv.Atoi(valuesGet(values, "limit")); err == nil {
		q.Limit = v
	}
	if v, err := strconv.ParseUint(valuesGet(values, "min_views"), 10, 64); err == nil {
		q.MinViews = &v
	}
	if v, err := strconv.ParseFloat(valuesGet(values, "max_competition"), 64); err == nil {
		q.MaxCompetition = &v
	}
	if v, err := strconv.ParseFloat(valuesGet(values, "min_opportunity"), 64); err == nil {
		q.MinOpportunityScore = &v
	}
	return q
}

func valuesGet(values map[string][]string, key string) string {
	if got := values[key]; len(got) > 0 {
		return got[0]
	}
	return ""
}

func firstQuery(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseBool(raw string) bool {
	raw = strings.ToLower(strings.TrimSpace(raw))
	return raw == "true" || raw == "1" || raw == "yes"
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := []string{}
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			out = append(out, strings.TrimSpace(part))
		}
	}
	return out
}
