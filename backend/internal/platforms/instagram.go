package platforms

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"trendcortex/api/internal/oauth"
)

const (
	instagramAuthURL  = "https://api.instagram.com/oauth/authorize"
	instagramTokenURL = "https://api.instagram.com/oauth/access_token"
	// Graph API is used for long-lived tokens and publishing
	metaGraphBase = "https://graph.facebook.com/v19.0"
)

// InstagramAdapter implements oauth.Adapter for Instagram Reels.
// Uses Instagram Basic Display API + Content Publishing API (requires Meta review).
type InstagramAdapter struct {
	appID       string
	appSecret   string
	redirectURI string
}

func NewInstagramAdapter(appID, appSecret, apiBase string) *InstagramAdapter {
	return &InstagramAdapter{
		appID:       appID,
		appSecret:   appSecret,
		redirectURI: apiBase + "/oauth/instagram/callback",
	}
}

func (a *InstagramAdapter) Platform() string { return "instagram" }

func (a *InstagramAdapter) RequiredScopes() []string {
	return []string{"instagram_basic", "instagram_content_publish", "pages_show_list"}
}

func (a *InstagramAdapter) RateLimitConfig() oauth.RateLimitConfig {
	return oauth.RateLimitConfig{
		DailyUploadLimit: 25,
		MinSecsBetween:   0,
		MaxDurationSec:   90,
		MaxFileSizeMB:    1000,
	}
}

func (a *InstagramAdapter) credentialsOK() bool {
	return a.appID != "" && a.appSecret != ""
}

func (a *InstagramAdapter) CanPublish() bool { return a.credentialsOK() }

func (a *InstagramAdapter) AuthStartURL(state, _ string) (string, error) {
	if !a.credentialsOK() {
		return "", oauth.ErrCredentialsMissing
	}
	// Instagram uses standard OAuth 2.0 without PKCE at the authorization step.
	params := url.Values{
		"client_id":     {a.appID},
		"redirect_uri":  {a.redirectURI},
		"response_type": {"code"},
		"scope":         {strings.Join(a.RequiredScopes(), ",")},
		"state":         {state},
	}
	return instagramAuthURL + "?" + params.Encode(), nil
}

func (a *InstagramAdapter) HandleCallback(ctx context.Context, code, state, _ string) (*oauth.Tokens, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	// Production:
	// 1. POST instagramTokenURL to get short-lived token
	// 2. Exchange for long-lived token via GET metaGraphBase/access_token?...
	return nil, fmt.Errorf("instagram: HandleCallback not yet implemented")
}

func (a *InstagramAdapter) RefreshToken(ctx context.Context, refreshToken string) (*oauth.Tokens, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	// Long-lived IG tokens can be refreshed via GET metaGraphBase/refresh_access_token
	return nil, fmt.Errorf("instagram: RefreshToken not yet implemented")
}

func (a *InstagramAdapter) Disconnect(ctx context.Context, accessToken string) error {
	if !a.credentialsOK() {
		return oauth.ErrCredentialsMissing
	}
	// DELETE metaGraphBase/{ig-user-id}/permissions
	return fmt.Errorf("instagram: Disconnect not yet implemented")
}

func (a *InstagramAdapter) PublishVideo(ctx context.Context, accessToken string, req oauth.PublishRequest) (*oauth.PublishResult, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	if accessToken == "" {
		return nil, oauth.ErrNotConnected
	}
	// Production (2-step container flow):
	// 1. POST metaGraphBase/{ig-user-id}/media with media_type=REELS, video_url, caption, share_to_feed=true
	// 2. Poll until status_code=FINISHED
	// 3. POST metaGraphBase/{ig-user-id}/media_publish with creation_id
	return nil, fmt.Errorf("instagram: PublishVideo not yet implemented")
}

func (a *InstagramAdapter) FetchAnalytics(ctx context.Context, accessToken, platformPostID string) (*oauth.AnalyticsResult, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	// GET metaGraphBase/{media-id}/insights?metric=plays,likes,shares,comments,reach
	return nil, fmt.Errorf("instagram: FetchAnalytics not yet implemented")
}
