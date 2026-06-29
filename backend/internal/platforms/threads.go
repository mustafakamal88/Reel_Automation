package platforms

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"trendcortex/api/internal/oauth"
)

const (
	threadsAuthURL  = "https://threads.net/oauth/authorize"
	threadsTokenURL = "https://graph.threads.net/oauth/access_token"
	threadsAPIBase  = "https://graph.threads.net/v1.0"
)

// ThreadsAdapter implements oauth.Adapter for Threads (Meta).
// Uses the Threads API (separate from Instagram Graph API).
// Requires Threads API access approval from Meta.
type ThreadsAdapter struct {
	appID       string
	appSecret   string
	redirectURI string
}

func NewThreadsAdapter(appID, appSecret, apiBase string) *ThreadsAdapter {
	return &ThreadsAdapter{
		appID:       appID,
		appSecret:   appSecret,
		redirectURI: apiBase + "/oauth/threads/callback",
	}
}

func (a *ThreadsAdapter) Platform() string { return "threads" }

func (a *ThreadsAdapter) RequiredScopes() []string {
	return []string{"threads_basic", "threads_content_publish"}
}

func (a *ThreadsAdapter) RateLimitConfig() oauth.RateLimitConfig {
	return oauth.RateLimitConfig{
		DailyUploadLimit: 250,
		MinSecsBetween:   0,
		MaxDurationSec:   300,
		MaxFileSizeMB:    1000,
	}
}

func (a *ThreadsAdapter) credentialsOK() bool {
	return a.appID != "" && a.appSecret != ""
}

func (a *ThreadsAdapter) CanPublish() bool { return a.credentialsOK() }

func (a *ThreadsAdapter) AuthStartURL(state, _ string) (string, error) {
	if !a.credentialsOK() {
		return "", oauth.ErrCredentialsMissing
	}
	params := url.Values{
		"client_id":     {a.appID},
		"redirect_uri":  {a.redirectURI},
		"response_type": {"code"},
		"scope":         {strings.Join(a.RequiredScopes(), ",")},
		"state":         {state},
	}
	return threadsAuthURL + "?" + params.Encode(), nil
}

func (a *ThreadsAdapter) HandleCallback(ctx context.Context, code, state, _ string) (*oauth.Tokens, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	// POST threadsTokenURL with code, client_id, client_secret, redirect_uri, grant_type=authorization_code
	return nil, fmt.Errorf("threads: HandleCallback not yet implemented")
}

func (a *ThreadsAdapter) RefreshToken(ctx context.Context, refreshToken string) (*oauth.Tokens, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	// GET threadsAPIBase/refresh_access_token?grant_type=th_refresh_token&access_token=...
	return nil, fmt.Errorf("threads: RefreshToken not yet implemented")
}

func (a *ThreadsAdapter) Disconnect(ctx context.Context, accessToken string) error {
	if !a.credentialsOK() {
		return oauth.ErrCredentialsMissing
	}
	return fmt.Errorf("threads: Disconnect not yet implemented")
}

func (a *ThreadsAdapter) PublishVideo(ctx context.Context, accessToken string, req oauth.PublishRequest) (*oauth.PublishResult, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	if accessToken == "" {
		return nil, oauth.ErrNotConnected
	}
	// Production (2-step container flow):
	// 1. POST threadsAPIBase/{user-id}/threads with media_type=VIDEO, video_url, text
	// 2. Poll until status=FINISHED
	// 3. POST threadsAPIBase/{user-id}/threads_publish with creation_id
	return nil, fmt.Errorf("threads: PublishVideo not yet implemented")
}

func (a *ThreadsAdapter) FetchAnalytics(ctx context.Context, accessToken, platformPostID string) (*oauth.AnalyticsResult, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	// GET threadsAPIBase/{media-id}/insights?metric=views,likes,replies,reposts,quotes
	return nil, fmt.Errorf("threads: FetchAnalytics not yet implemented")
}
