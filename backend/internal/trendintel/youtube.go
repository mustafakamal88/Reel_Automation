package trendintel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ErrYouTubeNotConfigured = errors.New("youtube api key is not configured")

type YouTubeClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

type Video struct {
	ID           string
	Title        string
	Description  string
	Tags         []string
	ChannelID    string
	ChannelTitle string
	PublishedAt  time.Time
	CategoryID   string
	Duration     string
	Views        *uint64
	Likes        *uint64
	Comments     *uint64
	ThumbnailURL string
}

func NewYouTubeClient(apiKey string, client *http.Client) *YouTubeClient {
	if client == nil {
		client = &http.Client{Timeout: 12 * time.Second}
	}
	return &YouTubeClient{apiKey: strings.TrimSpace(apiKey), baseURL: "https://www.googleapis.com/youtube/v3", client: client}
}

func (c *YouTubeClient) SearchVideos(ctx context.Context, q Query, keyword string, publishedAfter time.Time, location *ResolvedLocation) ([]Video, SourceProvenance, error) {
	prov := SourceProvenance{Provider: "youtube_data_api", RequestType: "video_search", FetchedAt: time.Now().UTC()}
	if c.apiKey == "" {
		return nil, prov, ErrYouTubeNotConfigured
	}
	params := url.Values{}
	params.Set("part", "snippet")
	params.Set("type", "video")
	params.Set("q", BuildYouTubeQuery(q, keyword))
	params.Set("regionCode", q.Country)
	params.Set("relevanceLanguage", q.Language)
	params.Set("publishedAfter", publishedAfter.Format(time.RFC3339))
	params.Set("maxResults", strconv.Itoa(clampInt(q.Limit, 1, 50, 20)))
	params.Set("order", "relevance")
	params.Set("key", c.apiKey)
	if q.VideoDuration != "" && q.VideoDuration != "any" {
		params.Set("videoDuration", q.VideoDuration)
	}
	if q.LocalVideosOnly && location != nil && location.Latitude != nil && location.Longitude != nil {
		params.Set("location", fmt.Sprintf("%.5f,%.5f", *location.Latitude, *location.Longitude))
		params.Set("locationRadius", fmt.Sprintf("%dkm", q.LocalRadiusKM))
	}
	endpoint := c.endpoint("search", params)
	prov.URL = apiURLWithoutKey(endpoint)
	var search youtubeSearchResponse
	if err := c.getJSON(ctx, endpoint, &search); err != nil {
		return nil, prov, err
	}
	ids := []string{}
	for _, item := range search.Items {
		if item.ID.VideoID != "" {
			ids = append(ids, item.ID.VideoID)
		}
	}
	if len(ids) == 0 {
		return []Video{}, prov, nil
	}
	videos, err := c.VideoDetails(ctx, ids)
	return videos, prov, err
}

func (c *YouTubeClient) VideoDetails(ctx context.Context, ids []string) ([]Video, error) {
	if c.apiKey == "" {
		return nil, ErrYouTubeNotConfigured
	}
	if len(ids) == 0 {
		return []Video{}, nil
	}
	if len(ids) > 50 {
		ids = ids[:50]
	}
	params := url.Values{}
	params.Set("part", "snippet,statistics,contentDetails")
	params.Set("id", strings.Join(ids, ","))
	params.Set("key", c.apiKey)
	var res youtubeVideosResponse
	if err := c.getJSON(ctx, c.endpoint("videos", params), &res); err != nil {
		return nil, err
	}
	out := make([]Video, 0, len(res.Items))
	for _, item := range res.Items {
		published, _ := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
		out = append(out, Video{
			ID:           item.ID,
			Title:        item.Snippet.Title,
			Description:  item.Snippet.Description,
			Tags:         item.Snippet.Tags,
			ChannelID:    item.Snippet.ChannelID,
			ChannelTitle: item.Snippet.ChannelTitle,
			PublishedAt:  published.UTC(),
			CategoryID:   item.Snippet.CategoryID,
			Duration:     item.ContentDetails.Duration,
			Views:        parseUintPtr(item.Statistics.ViewCount),
			Likes:        parseUintPtr(item.Statistics.LikeCount),
			Comments:     parseUintPtr(item.Statistics.CommentCount),
			ThumbnailURL: bestThumbnail(item.Snippet.Thumbnails),
		})
	}
	return out, nil
}

func (c *YouTubeClient) endpoint(resource string, params url.Values) string {
	u, _ := url.Parse(strings.TrimRight(c.baseURL, "/") + "/" + resource)
	u.RawQuery = params.Encode()
	return u.String()
}

func (c *YouTubeClient) getJSON(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "TrendCortex/trend-intelligence")
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return err
	}
	if res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("search limit reached")
	}
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusBadRequest {
		return fmt.Errorf("video intelligence credentials are invalid or request is malformed")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("video intelligence request failed")
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("malformed video intelligence response: %w", err)
	}
	return nil
}

type youtubeSearchResponse struct {
	Items []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
	} `json:"items"`
}

type youtubeVideosResponse struct {
	Items []youtubeVideoItem `json:"items"`
}

type youtubeVideoItem struct {
	ID      string `json:"id"`
	Snippet struct {
		PublishedAt  string   `json:"publishedAt"`
		ChannelID    string   `json:"channelId"`
		Title        string   `json:"title"`
		Description  string   `json:"description"`
		ChannelTitle string   `json:"channelTitle"`
		Tags         []string `json:"tags"`
		CategoryID   string   `json:"categoryId"`
		Thumbnails   struct {
			Default  thumb `json:"default"`
			Medium   thumb `json:"medium"`
			High     thumb `json:"high"`
			Standard thumb `json:"standard"`
			Maxres   thumb `json:"maxres"`
		} `json:"thumbnails"`
	} `json:"snippet"`
	Statistics struct {
		ViewCount    string `json:"viewCount"`
		LikeCount    string `json:"likeCount"`
		CommentCount string `json:"commentCount"`
	} `json:"statistics"`
	ContentDetails struct {
		Duration string `json:"duration"`
	} `json:"contentDetails"`
}

type thumb struct {
	URL string `json:"url"`
}

func bestThumbnail(t struct {
	Default  thumb `json:"default"`
	Medium   thumb `json:"medium"`
	High     thumb `json:"high"`
	Standard thumb `json:"standard"`
	Maxres   thumb `json:"maxres"`
}) string {
	for _, candidate := range []string{t.Maxres.URL, t.Standard.URL, t.High.URL, t.Medium.URL, t.Default.URL} {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func videoStats(videos []Video) (count int, totalViews, medianViews, avgViews, likes, comments *uint64, newest *time.Time, strongestURL, thumb string) {
	count = len(videos)
	if count == 0 {
		return count, nil, nil, nil, nil, nil, nil, "", ""
	}
	viewVals := []uint64{}
	var tv, tl, tc uint64
	var hasViews, hasLikes, hasComments bool
	var strongestViews uint64
	for _, v := range videos {
		if !v.PublishedAt.IsZero() && (newest == nil || v.PublishedAt.After(*newest)) {
			t := v.PublishedAt
			newest = &t
		}
		if v.Views != nil {
			hasViews = true
			tv += *v.Views
			viewVals = append(viewVals, *v.Views)
			if *v.Views >= strongestViews {
				strongestViews = *v.Views
				strongestURL = "https://www.youtube.com/watch?v=" + v.ID
				thumb = v.ThumbnailURL
			}
		}
		if v.Likes != nil {
			hasLikes = true
			tl += *v.Likes
		}
		if v.Comments != nil {
			hasComments = true
			tc += *v.Comments
		}
	}
	if hasViews {
		totalViews = ptrUint64(tv)
		avg := tv / uint64(len(viewVals))
		avgViews = &avg
		sort.Slice(viewVals, func(i, j int) bool { return viewVals[i] < viewVals[j] })
		median := viewVals[len(viewVals)/2]
		medianViews = &median
	}
	if hasLikes {
		likes = ptrUint64(tl)
	}
	if hasComments {
		comments = ptrUint64(tc)
	}
	return count, totalViews, medianViews, avgViews, likes, comments, newest, strongestURL, thumb
}

func parseUintPtr(raw string) *uint64 {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return nil
	}
	return &v
}

func ptrUint64(v uint64) *uint64 { return &v }

func apiURLWithoutKey(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	q := u.Query()
	if q.Has("key") {
		q.Set("key", "redacted")
	}
	u.RawQuery = q.Encode()
	return u.String()
}
