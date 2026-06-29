package platforms

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"trendcortex/api/internal/oauth"
)

const (
	xAuthURL   = "https://twitter.com/i/oauth2/authorize"
	xTokenURL  = "https://api.twitter.com/2/oauth2/token"
	xRevokeURL = "https://api.twitter.com/2/oauth2/revoke"
	xAPIBase   = "https://api.twitter.com/2"
)

// XAdapter implements oauth.Adapter for X (formerly Twitter).
// Uses OAuth 2.0 with PKCE (Authorization Code Flow with PKCE).
// Requires X Developer portal app with appropriate access level.
type XAdapter struct {
	clientID     string
	clientSecret string
	redirectURI  string
}

func NewXAdapter(clientID, clientSecret, apiBase string) *XAdapter {
	return &XAdapter{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  apiBase + "/oauth/x/callback",
	}
}

func (a *XAdapter) Platform() string { return "x" }

func (a *XAdapter) RequiredScopes() []string {
	return []string{"tweet.read", "tweet.write", "users.read", "media.write", "offline.access"}
}

func (a *XAdapter) RateLimitConfig() oauth.RateLimitConfig {
	return oauth.RateLimitConfig{
		DailyUploadLimit: 17,    // Free tier: 17 posts/day via API
		MinSecsBetween:   0,
		MaxDurationSec:   140,
		MaxFileSizeMB:    512,
	}
}

func (a *XAdapter) credentialsOK() bool {
	return a.clientID != "" && a.clientSecret != ""
}

func (a *XAdapter) CanPublish() bool { return a.credentialsOK() }

func (a *XAdapter) AuthStartURL(state, codeChallenge string) (string, error) {
	if !a.credentialsOK() {
		return "", oauth.ErrCredentialsMissing
	}
	params := url.Values{
		"client_id":             {a.clientID},
		"redirect_uri":          {a.redirectURI},
		"response_type":         {"code"},
		"scope":                 {strings.Join(a.RequiredScopes(), " ")},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}
	return xAuthURL + "?" + params.Encode(), nil
}

func (a *XAdapter) HandleCallback(ctx context.Context, code, state, codeVerifier string) (*oauth.Tokens, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	// POST xTokenURL with grant_type=authorization_code, code, codeVerifier, redirect_uri
	// Basic auth: base64(clientID:clientSecret)
	return nil, fmt.Errorf("x: HandleCallback not yet implemented — POST to %s", xTokenURL)
}

func (a *XAdapter) RefreshToken(ctx context.Context, refreshToken string) (*oauth.Tokens, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	// POST xTokenURL with grant_type=refresh_token, refresh_token=..., client_id=...
	return nil, fmt.Errorf("x: RefreshToken not yet implemented — POST to %s", xTokenURL)
}

func (a *XAdapter) Disconnect(ctx context.Context, accessToken string) error {
	if !a.credentialsOK() {
		return oauth.ErrCredentialsMissing
	}
	// POST xRevokeURL with token=accessToken, client_id=...
	return fmt.Errorf("x: Disconnect not yet implemented — POST to %s", xRevokeURL)
}

func (a *XAdapter) PublishVideo(ctx context.Context, accessToken string, req oauth.PublishRequest) (*oauth.PublishResult, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	if accessToken == "" {
		return nil, oauth.ErrNotConnected
	}
	// Production (chunked media upload):
	// 1. POST https://upload.twitter.com/1.1/media/upload.json?command=INIT
	// 2. Upload chunks via APPEND commands
	// 3. FINALIZE and poll until processing_info.state=succeeded
	// 4. POST xAPIBase/tweets with media.media_ids=[mediaID] and text
	return nil, fmt.Errorf("x: PublishVideo not yet implemented")
}

func (a *XAdapter) FetchAnalytics(ctx context.Context, accessToken, platformPostID string) (*oauth.AnalyticsResult, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	// GET xAPIBase/tweets/{id}?tweet.fields=public_metrics
	return nil, fmt.Errorf("x: FetchAnalytics not yet implemented")
}
