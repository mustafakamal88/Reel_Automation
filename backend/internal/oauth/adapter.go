package oauth

import (
	"context"
	"errors"
	"time"
)

// ErrCredentialsMissing is returned by any adapter when the required
// OAuth client credentials are not configured in the environment.
var ErrCredentialsMissing = errors.New("oauth: platform credentials not configured — add client ID/secret to .env")

// ErrNotConnected is returned when a publishing action is attempted
// on a platform that has not been connected via OAuth.
var ErrNotConnected = errors.New("oauth: platform account not connected")

// ErrTokenExpired signals that the stored token has expired and
// a refresh or re-authorisation is required.
var ErrTokenExpired = errors.New("oauth: access token expired")

// Tokens holds the OAuth token pair returned after a successful
// code exchange. These are ONLY handled server-side and must never
// be sent to the browser.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Scopes       []string
	PlatformUID  string // platform user/channel ID for deduplication
	Handle       string // @handle or channel name
}

// PublishRequest describes a video to be uploaded to a platform.
type PublishRequest struct {
	VideoPath     string
	ThumbnailPath string
	Title         string
	Description   string
	Hashtags      []string
	CaptionsPath  string
	AIDisclosure  bool // must be set true for AI-generated content where required
}

// PublishResult contains the platform's response after a successful upload.
type PublishResult struct {
	PlatformPostID string
	PostURL        string
	PublishedAt    time.Time
}

// AnalyticsResult contains a snapshot of video performance metrics.
type AnalyticsResult struct {
	Views         int64
	Likes         int64
	Shares        int64
	Comments      int64
	WatchTimeSec  int64
	SnapshotAt    time.Time
}

// Adapter defines the interface every platform must implement.
// If credentials are missing, methods must return ErrCredentialsMissing —
// not fake-success responses.
type Adapter interface {
	// Platform returns the unique platform key (e.g. "youtube").
	Platform() string

	// AuthStartURL returns the URL the user should be redirected to
	// in order to begin the OAuth flow. The state parameter must be
	// a cryptographically random value stored server-side for CSRF protection.
	AuthStartURL(state, codeChallenge string) (string, error)

	// HandleCallback exchanges the OAuth code for tokens.
	// Returns ErrCredentialsMissing if client credentials are not configured.
	HandleCallback(ctx context.Context, code, state, codeVerifier string) (*Tokens, error)

	// RefreshToken exchanges a refresh token for a new access token.
	// The new tokens must be re-encrypted and stored — the old entry deleted.
	RefreshToken(ctx context.Context, refreshToken string) (*Tokens, error)

	// Disconnect revokes the access token on the platform side.
	// The caller is responsible for deleting the stored credentials after this call.
	Disconnect(ctx context.Context, accessToken string) error

	// CanPublish returns true only if credentials are configured AND
	// the account has the required publishing scopes.
	CanPublish() bool

	// PublishVideo uploads and publishes a video to the platform.
	// Returns ErrCredentialsMissing if credentials are absent.
	// Returns ErrNotConnected if the account has not been linked.
	PublishVideo(ctx context.Context, accessToken string, req PublishRequest) (*PublishResult, error)

	// FetchAnalytics retrieves a performance snapshot for a given post.
	FetchAnalytics(ctx context.Context, accessToken, platformPostID string) (*AnalyticsResult, error)

	// RequiredScopes returns the OAuth scopes this adapter needs.
	RequiredScopes() []string

	// RateLimitConfig returns platform-specific rate limit constants.
	RateLimitConfig() RateLimitConfig
}

// RateLimitConfig holds per-platform publishing constraints.
type RateLimitConfig struct {
	DailyUploadLimit int
	MinSecsBetween   int
	MaxDurationSec   int
	MaxFileSizeMB    int
}

// Registry maps platform keys to their Adapter implementations.
type Registry map[string]Adapter

// Get returns the adapter for the given platform key or an error.
func (r Registry) Get(platform string) (Adapter, error) {
	a, ok := r[platform]
	if !ok {
		return nil, errors.New("oauth: unknown platform: " + platform)
	}
	return a, nil
}
