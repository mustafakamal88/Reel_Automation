package models

import "time"

// ─── User & Workspace ─────────────────────────────────────────

type User struct {
	ID           string    `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	WorkspaceID  string    `json:"workspace_id" db:"workspace_id"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type Workspace struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	OwnerID   string    `json:"owner_id" db:"owner_id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type BrandProfile struct {
	ID           string    `json:"id" db:"id"`
	WorkspaceID  string    `json:"workspace_id" db:"workspace_id"`
	Name         string    `json:"name" db:"name"`
	Niche        string    `json:"niche" db:"niche"`
	BrandVoice   string    `json:"brand_voice" db:"brand_voice"`
	ContentStyle string    `json:"content_style" db:"content_style"`
	Region       string    `json:"region" db:"region"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// ─── Platform OAuth & Token Storage ───────────────────────────

// PlatformKey identifies a supported publishing/data platform.
type PlatformKey string

const (
	PlatformYouTube   PlatformKey = "youtube"
	PlatformTikTok    PlatformKey = "tiktok"
	PlatformInstagram PlatformKey = "instagram"
	PlatformFacebook  PlatformKey = "facebook"
	PlatformThreads   PlatformKey = "threads"
	PlatformX         PlatformKey = "x"
)

// TokenStatus reflects the current OAuth connection health.
type TokenStatus string

const (
	TokenStatusNotConnected TokenStatus = "not_connected"
	TokenStatusConnected    TokenStatus = "connected"
	TokenStatusExpired      TokenStatus = "expired"
	TokenStatusRevoked      TokenStatus = "revoked"
	TokenStatusNeedsReview  TokenStatus = "needs_review"
)

// PlatformAccount stores the connection status for a platform account.
// Raw tokens are NEVER stored here — only reference IDs pointing to
// the encrypted vault entry.
type PlatformAccount struct {
	ID              string      `json:"id" db:"id"`
	WorkspaceID     string      `json:"workspace_id" db:"workspace_id"`
	Platform        PlatformKey `json:"platform" db:"platform"`
	Handle          *string     `json:"handle" db:"handle"`
	TokenStatus     TokenStatus `json:"token_status" db:"token_status"`
	TokenVaultRefID *string     `json:"-" db:"token_vault_ref_id"`
	Scopes          []string    `json:"scopes" db:"scopes"`
	CanPublish      bool        `json:"can_publish" db:"can_publish"`
	ConnectedAt     *time.Time  `json:"connected_at" db:"connected_at"`
	ExpiresAt       *time.Time  `json:"expires_at" db:"expires_at"`
	CreatedAt       time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at" db:"updated_at"`
}

// OAuthConnection tracks the OAuth state during the redirect flow.
type OAuthConnection struct {
	ID          string      `json:"id" db:"id"`
	WorkspaceID string      `json:"workspace_id" db:"workspace_id"`
	Platform    PlatformKey `json:"platform" db:"platform"`
	State       string      `json:"-" db:"state"`
	CodeVerifier *string    `json:"-" db:"code_verifier"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	ExpiresAt   time.Time   `json:"expires_at" db:"expires_at"`
}

// TokenVaultRef is a pointer to an encrypted token stored in the vault.
// The encrypted bytes live in a separate secure store — never alongside
// cleartext data in application tables.
type TokenVaultRef struct {
	ID               string      `json:"id" db:"id"`
	PlatformKey      PlatformKey `json:"platform_key" db:"platform_key"`
	WorkspaceID      string      `json:"workspace_id" db:"workspace_id"`
	EncryptedPayload []byte      `json:"-" db:"encrypted_payload"`
	KeyVersion       int         `json:"key_version" db:"key_version"`
	CreatedAt        time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at" db:"updated_at"`
}

// ─── Video Batch & Assets ─────────────────────────────────────

type BatchStatus string

const (
	BatchStatusDraft       BatchStatus = "draft"
	BatchStatusRendering   BatchStatus = "rendering"
	BatchStatusReady       BatchStatus = "ready"
	BatchStatusZipped      BatchStatus = "zipped"
	BatchStatusQueued      BatchStatus = "queued"
	BatchStatusPublishing  BatchStatus = "publishing"
	BatchStatusPublished   BatchStatus = "published"
	BatchStatusFailed      BatchStatus = "failed"
	BatchStatusNeedsReview BatchStatus = "needs_review"
)

type VideoBatch struct {
	ID          string      `json:"id" db:"id"`
	WorkspaceID string      `json:"workspace_id" db:"workspace_id"`
	Date        string      `json:"date" db:"date"` // YYYY-MM-DD
	Status      BatchStatus `json:"status" db:"status"`
	VideoCount  int         `json:"video_count" db:"video_count"`
	ZipPath     *string     `json:"zip_path" db:"zip_path"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`
}

type VideoAsset struct {
	ID          string      `json:"id" db:"id"`
	BatchID     string      `json:"batch_id" db:"batch_id"`
	WorkspaceID string      `json:"workspace_id" db:"workspace_id"`
	Rank        int         `json:"rank" db:"rank"`
	Title       string      `json:"title" db:"title"`
	Status      BatchStatus `json:"status" db:"status"`
	Platforms   []string    `json:"platforms" db:"platforms"`

	// File paths (relative to storage root, populated after render)
	VideoPath     *string `json:"video_path" db:"video_path"`
	ThumbnailPath *string `json:"thumbnail_path" db:"thumbnail_path"`
	CaptionsPath  *string `json:"captions_path" db:"captions_path"`

	// Metadata
	Description    string `json:"description" db:"description"`
	Hashtags       string `json:"hashtags" db:"hashtags"`
	AIDisclosure   bool   `json:"ai_disclosure" db:"ai_disclosure"`
	HumanApproved  bool   `json:"human_approved" db:"human_approved"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ─── Publishing Jobs ───────────────────────────────────────────

type PublishJobStatus string

const (
	PublishJobQueued     PublishJobStatus = "queued"
	PublishJobRunning    PublishJobStatus = "running"
	PublishJobDone       PublishJobStatus = "done"
	PublishJobFailed     PublishJobStatus = "failed"
	PublishJobSkipped    PublishJobStatus = "skipped"
)

type PublishJob struct {
	ID            string           `json:"id" db:"id"`
	WorkspaceID   string           `json:"workspace_id" db:"workspace_id"`
	VideoAssetID  *string          `json:"video_asset_id,omitempty" db:"video_asset_id"`
	ReelPlanID    *string          `json:"reel_plan_id,omitempty" db:"reel_plan_id"`
	Platform      PlatformKey      `json:"platform" db:"platform"`
	Status        PublishJobStatus `json:"status" db:"status"`
	RetryCount    int              `json:"retry_count" db:"retry_count"`
	ErrorMessage  *string          `json:"error_message" db:"error_message"`
	PlatformPostID *string         `json:"platform_post_id" db:"platform_post_id"`
	ScheduledFor  *time.Time       `json:"scheduled_for" db:"scheduled_for"`
	StartedAt     *time.Time       `json:"started_at" db:"started_at"`
	CompletedAt   *time.Time       `json:"completed_at" db:"completed_at"`
	CreatedAt     time.Time        `json:"created_at" db:"created_at"`
}

type DownloadZipJob struct {
	ID          string           `json:"id" db:"id"`
	WorkspaceID string           `json:"workspace_id" db:"workspace_id"`
	BatchID     string           `json:"batch_id" db:"batch_id"`
	Status      PublishJobStatus `json:"status" db:"status"`
	ZipPath     *string          `json:"zip_path" db:"zip_path"`
	ZipSizeBytes *int64          `json:"zip_size_bytes" db:"zip_size_bytes"`
	ErrorMessage *string         `json:"error_message" db:"error_message"`
	CreatedAt   time.Time        `json:"created_at" db:"created_at"`
	CompletedAt *time.Time       `json:"completed_at" db:"completed_at"`
}

// ─── Analytics & Limits ───────────────────────────────────────

type AnalyticsSnapshot struct {
	ID           string      `json:"id" db:"id"`
	WorkspaceID  string      `json:"workspace_id" db:"workspace_id"`
	Platform     PlatformKey `json:"platform" db:"platform"`
	VideoAssetID string      `json:"video_asset_id" db:"video_asset_id"`
	Views        int64       `json:"views" db:"views"`
	Likes        int64       `json:"likes" db:"likes"`
	Shares       int64       `json:"shares" db:"shares"`
	Comments     int64       `json:"comments" db:"comments"`
	WatchTimeSec int64       `json:"watch_time_sec" db:"watch_time_sec"`
	SnapshotAt   time.Time   `json:"snapshot_at" db:"snapshot_at"`
}

type PlatformRateLimit struct {
	ID                string      `json:"id" db:"id"`
	Platform          PlatformKey `json:"platform" db:"platform"`
	DailyUploadLimit  int         `json:"daily_upload_limit" db:"daily_upload_limit"`
	MinSecsBetween    int         `json:"min_secs_between" db:"min_secs_between"`
	MaxDurationSec    int         `json:"max_duration_sec" db:"max_duration_sec"`
	MaxFileSizeMB     int         `json:"max_file_size_mb" db:"max_file_size_mb"`
	UpdatedAt         time.Time   `json:"updated_at" db:"updated_at"`
}

// ─── Audit & Policy ───────────────────────────────────────────

type AuditLog struct {
	ID          string    `json:"id" db:"id"`
	WorkspaceID string    `json:"workspace_id" db:"workspace_id"`
	UserID      *string   `json:"user_id" db:"user_id"`
	Action      string    `json:"action" db:"action"`
	Resource    string    `json:"resource" db:"resource"`
	ResourceID  *string   `json:"resource_id" db:"resource_id"`
	IPAddress   *string   `json:"ip_address" db:"ip_address"`
	UserAgent   *string   `json:"user_agent" db:"user_agent"`
	Metadata    []byte    `json:"metadata" db:"metadata"` // JSON blob
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type PolicyChangeLog struct {
	ID          string    `json:"id" db:"id"`
	Platform    PlatformKey `json:"platform" db:"platform"`
	ChangeType  string    `json:"change_type" db:"change_type"`
	Description string    `json:"description" db:"description"`
	EffectiveAt time.Time `json:"effective_at" db:"effective_at"`
	Source      string    `json:"source" db:"source"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}
