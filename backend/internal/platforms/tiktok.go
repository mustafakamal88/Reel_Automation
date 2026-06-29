package platforms

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"trendcortex/api/internal/oauth"
)

const (
	tiktokAuthURL   = "https://www.tiktok.com/v2/auth/authorize/"
	tiktokTokenURL  = "https://open.tiktokapis.com/v2/oauth/token/"
	tiktokRevokeURL = "https://open.tiktokapis.com/v2/oauth/revoke/"
)

// TikTokAdapter implements oauth.Adapter for TikTok Content Posting API.
// Uses OAuth 2.0 with PKCE. Requires TikTok for Developers app approval.
type TikTokAdapter struct {
	clientKey    string
	clientSecret string
	redirectURI  string
}

func NewTikTokAdapter(clientKey, clientSecret, apiBase string) *TikTokAdapter {
	return &TikTokAdapter{
		clientKey:    clientKey,
		clientSecret: clientSecret,
		redirectURI:  apiBase + "/oauth/tiktok/callback",
	}
}

func (a *TikTokAdapter) Platform() string { return "tiktok" }

func (a *TikTokAdapter) RequiredScopes() []string {
	return []string{"video.upload", "user.info.basic"}
}

func (a *TikTokAdapter) RateLimitConfig() oauth.RateLimitConfig {
	return oauth.RateLimitConfig{
		DailyUploadLimit: 10,
		MinSecsBetween:   3600,
		MaxDurationSec:   60,
		MaxFileSizeMB:    287,
	}
}

func (a *TikTokAdapter) credentialsOK() bool {
	return a.clientKey != "" && a.clientSecret != ""
}

func (a *TikTokAdapter) CanPublish() bool { return a.credentialsOK() }

func (a *TikTokAdapter) AuthStartURL(state, codeChallenge string) (string, error) {
	if !a.credentialsOK() {
		return "", oauth.ErrCredentialsMissing
	}
	params := url.Values{
		"client_key":            {a.clientKey},
		"redirect_uri":          {a.redirectURI},
		"response_type":         {"code"},
		"scope":                 {strings.Join(a.RequiredScopes(), ",")},
		"state":                 {state},
		"code_challenge":        {codeChallenge},
		"code_challenge_method": {"S256"},
	}
	return tiktokAuthURL + "?" + params.Encode(), nil
}

func (a *TikTokAdapter) HandleCallback(ctx context.Context, code, state, codeVerifier string) (*oauth.Tokens, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	return nil, fmt.Errorf("tiktok: HandleCallback not yet implemented — POST to %s", tiktokTokenURL)
}

func (a *TikTokAdapter) RefreshToken(ctx context.Context, refreshToken string) (*oauth.Tokens, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	return nil, fmt.Errorf("tiktok: RefreshToken not yet implemented — POST grant_type=refresh_token to %s", tiktokTokenURL)
}

func (a *TikTokAdapter) Disconnect(ctx context.Context, accessToken string) error {
	if !a.credentialsOK() {
		return oauth.ErrCredentialsMissing
	}
	return fmt.Errorf("tiktok: Disconnect not yet implemented — POST to %s", tiktokRevokeURL)
}

func (a *TikTokAdapter) PublishVideo(ctx context.Context, accessToken string, req oauth.PublishRequest) (*oauth.PublishResult, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	if accessToken == "" {
		return nil, oauth.ErrNotConnected
	}
	// Production:
	// 1. POST https://open.tiktokapis.com/v2/post/publish/video/init/ to init upload
	// 2. Upload video bytes
	// 3. POST https://open.tiktokapis.com/v2/post/publish/status/fetch/ to confirm
	// Note: TikTok requires that AI-generated content labels be applied.
	return nil, fmt.Errorf("tiktok: PublishVideo not yet implemented")
}

func (a *TikTokAdapter) FetchAnalytics(ctx context.Context, accessToken, platformPostID string) (*oauth.AnalyticsResult, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	return nil, fmt.Errorf("tiktok: FetchAnalytics not yet implemented")
}
