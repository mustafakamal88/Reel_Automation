package platforms

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"trendcortex/api/internal/oauth"
)

const (
	youtubeAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	youtubeTokenURL = "https://oauth2.googleapis.com/token"
	youtubeRevokeURL = "https://oauth2.googleapis.com/revoke"
)

// YouTubeAdapter implements oauth.Adapter for YouTube Shorts.
// Uses Google's OAuth 2.0 with PKCE.
type YouTubeAdapter struct {
	clientID     string
	clientSecret string
	redirectURI  string
}

// NewYouTubeAdapter creates the adapter. Returns a ready adapter even if credentials
// are missing — methods will return ErrCredentialsMissing at call time, not at
// construction time, so the registry can always be fully populated.
func NewYouTubeAdapter(clientID, clientSecret, apiBase string) *YouTubeAdapter {
	return &YouTubeAdapter{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  apiBase + "/oauth/youtube/callback",
	}
}

func (a *YouTubeAdapter) Platform() string { return "youtube" }

func (a *YouTubeAdapter) RequiredScopes() []string {
	return []string{
		"https://www.googleapis.com/auth/youtube.upload",
		"https://www.googleapis.com/auth/youtube.readonly",
	}
}

func (a *YouTubeAdapter) RateLimitConfig() oauth.RateLimitConfig {
	return oauth.RateLimitConfig{
		DailyUploadLimit: 100,
		MinSecsBetween:   0,
		MaxDurationSec:   60,
		MaxFileSizeMB:    256,
	}
}

func (a *YouTubeAdapter) credentialsOK() bool {
	return a.clientID != "" && a.clientSecret != ""
}

func (a *YouTubeAdapter) CanPublish() bool { return a.credentialsOK() }

func (a *YouTubeAdapter) AuthStartURL(state, codeChallenge string) (string, error) {
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
		"access_type":           {"offline"},
		"prompt":                {"consent"},
	}
	return youtubeAuthURL + "?" + params.Encode(), nil
}

func (a *YouTubeAdapter) HandleCallback(ctx context.Context, code, state, codeVerifier string) (*oauth.Tokens, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	// Production: POST to youtubeTokenURL with code + codeVerifier.
	// Parse JSON response → Tokens. Decrypt nothing here; caller encrypts and stores.
	return nil, fmt.Errorf("youtube: HandleCallback not yet implemented — wire HTTP client to %s", youtubeTokenURL)
}

func (a *YouTubeAdapter) RefreshToken(ctx context.Context, refreshToken string) (*oauth.Tokens, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	return nil, fmt.Errorf("youtube: RefreshToken not yet implemented — POST grant_type=refresh_token to %s", youtubeTokenURL)
}

func (a *YouTubeAdapter) Disconnect(ctx context.Context, accessToken string) error {
	if !a.credentialsOK() {
		return oauth.ErrCredentialsMissing
	}
	// Production: POST token=accessToken to youtubeRevokeURL.
	return fmt.Errorf("youtube: Disconnect not yet implemented — POST to %s", youtubeRevokeURL)
}

func (a *YouTubeAdapter) PublishVideo(ctx context.Context, accessToken string, req oauth.PublishRequest) (*oauth.PublishResult, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	if accessToken == "" {
		return nil, oauth.ErrNotConnected
	}
	// Production: multipart upload to https://www.googleapis.com/upload/youtube/v3/videos
	// with Snippet + Status parts. Set selfDeclaredMadeForKids appropriately.
	// AI disclosure: add #AIGenerated in description when req.AIDisclosure is true.
	return nil, fmt.Errorf("youtube: PublishVideo not yet implemented")
}

func (a *YouTubeAdapter) FetchAnalytics(ctx context.Context, accessToken, platformPostID string) (*oauth.AnalyticsResult, error) {
	if !a.credentialsOK() {
		return nil, oauth.ErrCredentialsMissing
	}
	// Production: GET https://www.googleapis.com/youtube/v3/videos?part=statistics&id={platformPostID}
	return nil, fmt.Errorf("youtube: FetchAnalytics not yet implemented")
}
