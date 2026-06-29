package platforms

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"trendcortex/api/internal/oauth"
)

const (
	facebookAuthURL = "https://www.facebook.com/v19.0/dialog/oauth"
)

// FacebookAdapter implements oauth.Adapter for Facebook Reels.
// Uses the Facebook Graph API for Reels publishing to a Page.
// Requires: Facebook Page, pages_manage_posts + pages_read_engagement scopes.
type FacebookAdapter struct {
	appID       string
	appSecret   string
	redirectURI string
}

func NewFacebookAdapter(appID, appSecret, apiBase string) *FacebookAdapter {
	return &FacebookAdapter{
		appID:       appID,
		appSecret:   appSecret,
		redirectURI: apiBase + "/oauth/facebook/callback",
	}
}

func (a *FacebookAdapter) Platform() string { return "facebook" }

func (a *FacebookAdapter) RequiredScopes() []string {
	return []string{"pages_manage_posts", "pages_read_engagement", "pages_show_list"}
}

func (a *FacebookAdapter) RateLimitConfig() oauth.RateLimitConfig {
	return oauth.RateLimitConfig{
		DailyUploadLimit: 25,
		MinSecsBetween:   0,
		MaxDurationSec:   240,
		MaxFileSizeMB:    1000,
	}
}

func (a *FacebookAdapter) credentialsOK() bool {
	return a.appID != "" && a.appSecret != ""
}

func (a *FacebookAdapter) CanPublish() bool { return a.credentialsOK() }

func (a *FacebookAdapter) AuthStartURL(state, _ string) (string, error) {
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
	return facebookAuthURL + "?" + params.Encode(), nil
}

func (a *FacebookAdapter) HandleCallback(ctx context.Context, code, state, _ string) (*oauth.Tokens, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	// Production:
	// POST metaGraphBase/oauth/access_token with code, client_id, client_secret, redirect_uri
	// Exchange user token for Page access token via GET metaGraphBase/{page-id}?fields=access_token
	return nil, fmt.Errorf("facebook: HandleCallback not yet implemented")
}

func (a *FacebookAdapter) RefreshToken(ctx context.Context, refreshToken string) (*oauth.Tokens, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	// Facebook long-lived tokens are valid for 60 days; extend via
	// GET metaGraphBase/oauth/access_token?grant_type=fb_exchange_token
	return nil, fmt.Errorf("facebook: RefreshToken not yet implemented")
}

func (a *FacebookAdapter) Disconnect(ctx context.Context, accessToken string) error {
	if !a.credentialsOK() {
		return oauth.ErrCredentialsMissing
	}
	// DELETE metaGraphBase/{user-id}/permissions?access_token=...
	return fmt.Errorf("facebook: Disconnect not yet implemented")
}

func (a *FacebookAdapter) PublishVideo(ctx context.Context, accessToken string, req oauth.PublishRequest) (*oauth.PublishResult, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	if accessToken == "" {
		return nil, oauth.ErrNotConnected
	}
	// Production (Reels on Pages):
	// POST metaGraphBase/{page-id}/video_reels with upload_phase=start
	// Upload video bytes
	// POST metaGraphBase/{page-id}/video_reels with upload_phase=finish and video_state=PUBLISHED
	return nil, fmt.Errorf("facebook: PublishVideo not yet implemented")
}

func (a *FacebookAdapter) FetchAnalytics(ctx context.Context, accessToken, platformPostID string) (*oauth.AnalyticsResult, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	// GET metaGraphBase/{video-id}/video_insights
	return nil, fmt.Errorf("facebook: FetchAnalytics not yet implemented")
}
