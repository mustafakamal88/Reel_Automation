package models

import "time"

// ─── Phase 4A: Trend discovery → scoring → daily batch → reel plans ──────────
//
// These models back the real automation pipeline: trend sources feed trend
// items, trend items get scored, the daily batch picks the top 6 scored
// items into reel plans, and each reel plan can request a video job, an
// export job (ZIP), and publish jobs. Nothing here fakes a completed state —
// video/export/publish stay in honest "not connected", "missing artifact",
// or "failed" states unless the real provider path succeeds.

// TrendSource is where trend_items are allowed to come from.
// Only "manual" can create trend_items today; any other
// source_type (youtube, google_trends, tiktok, x, instagram, ...) is a
// real future integration and currently reports provider_not_connected.
type TrendSource struct {
	ID          string    `json:"id" db:"id"`
	WorkspaceID string    `json:"workspace_id" db:"workspace_id"`
	Name        string    `json:"name" db:"name"`
	SourceType  string    `json:"source_type" db:"source_type"`
	Status      string    `json:"status" db:"status"`
	Confidence  float64   `json:"confidence" db:"confidence"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

const (
	TrendSourceTypeManual = "manual"
)

// IsDiscoverable reports whether this source type is allowed to create
// trend_items directly. Every other source type is a not-yet-connected
// external provider.
func (s TrendSource) IsDiscoverable() bool {
	return s.SourceType == TrendSourceTypeManual
}

// TrendItem is a single discovered topic candidate, always traceable back
// to the trend_source that produced it.
type TrendItem struct {
	ID            string    `json:"id" db:"id"`
	WorkspaceID   string    `json:"workspace_id" db:"workspace_id"`
	TrendSourceID string    `json:"trend_source_id" db:"trend_source_id"`
	Topic         string    `json:"topic" db:"topic"`
	Description   string    `json:"description" db:"description"`
	PlatformHint  string    `json:"platform_hint" db:"platform_hint"`
	Velocity      float64   `json:"velocity" db:"velocity"`
	Status        string    `json:"status" db:"status"`
	DiscoveredAt  time.Time `json:"discovered_at" db:"discovered_at"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

const (
	TrendItemStatusNew      = "new"
	TrendItemStatusScored   = "scored"
	TrendItemStatusBatched  = "batched"
	TrendItemStatusRejected = "rejected"
)

// TopicScore is the deterministic scoring result for a single trend item.
// One row per trend_item (UNIQUE constraint), recomputed by re-running
// POST /api/topics/score.
type TopicScore struct {
	ID                    string    `json:"id" db:"id"`
	WorkspaceID           string    `json:"workspace_id" db:"workspace_id"`
	TrendItemID           string    `json:"trend_item_id" db:"trend_item_id"`
	TotalScore            float64   `json:"total_score" db:"total_score"`
	VelocityScore         float64   `json:"velocity_score" db:"velocity_score"`
	SourceConfidenceScore float64   `json:"source_confidence_score" db:"source_confidence_score"`
	PlatformFitScore      float64   `json:"platform_fit_score" db:"platform_fit_score"`
	SafetyScore           float64   `json:"safety_score" db:"safety_score"`
	WatchTimeScore        float64   `json:"watch_time_score" db:"watch_time_score"`
	CompetitionScore      float64   `json:"competition_score" db:"competition_score"`
	Reason                string    `json:"reason" db:"reason"`
	BreakdownJSON         []byte    `json:"-" db:"breakdown"`
	CreatedAt             time.Time `json:"created_at" db:"created_at"`
}

// DailyBatch groups up to 6 reel_plans selected for a single calendar date.
type DailyBatch struct {
	ID          string    `json:"id" db:"id"`
	WorkspaceID string    `json:"workspace_id" db:"workspace_id"`
	BatchDate   string    `json:"batch_date" db:"batch_date"` // YYYY-MM-DD
	Status      string    `json:"status" db:"status"`
	ReelCount   int       `json:"reel_count" db:"reel_count"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

const (
	DailyBatchStatusPlanned = "planned" // no eligible scored topics yet
	DailyBatchStatusReady   = "ready"   // 1-6 reel_plans created
)

// ReelPlan is one of up to 6 daily reel slots: a scored trend item turned
// into a concrete content plan with draft copy. All draft fields are
// placeholders for human review — never auto-published.
//
// The video/thumbnail artifact fields stay NULL/empty until a real render
// pipeline writes a real video.mp4 / thumbnail.png to disk and records its
// path here. If provider credentials or FFmpeg are missing, these stay empty
// and ExportStatus reports the missing artifact honestly.
type ReelPlan struct {
	ID               string `json:"id" db:"id"`
	WorkspaceID      string `json:"workspace_id" db:"workspace_id"`
	DailyBatchID     string `json:"daily_batch_id" db:"daily_batch_id"`
	TrendItemID      string `json:"trend_item_id" db:"trend_item_id"`
	TopicScoreID     string `json:"topic_score_id" db:"topic_score_id"`
	Rank             int    `json:"rank" db:"rank"`
	Platform         string `json:"platform" db:"platform"`
	TitleIdea        string `json:"title_idea" db:"title_idea"`
	ScriptOutline    string `json:"script_outline" db:"script_outline"`
	DescriptionDraft string `json:"description_draft" db:"description_draft"`
	HashtagsDraft    string `json:"hashtags_draft" db:"hashtags_draft"`
	ThumbnailIdea    string `json:"thumbnail_idea" db:"thumbnail_idea"`
	Status           string `json:"status" db:"status"`

	VideoArtifactPath    *string  `json:"video_artifact_path" db:"video_artifact_path"`
	VideoFormat          string   `json:"video_format" db:"video_format"`
	VideoWidth           *int     `json:"video_width" db:"video_width"`
	VideoHeight          *int     `json:"video_height" db:"video_height"`
	VideoDurationSeconds *float64 `json:"video_duration_seconds" db:"video_duration_seconds"`
	VideoCodec           string   `json:"video_codec" db:"video_codec"`
	AudioCodec           string   `json:"audio_codec" db:"audio_codec"`

	ThumbnailArtifactPath *string `json:"thumbnail_artifact_path" db:"thumbnail_artifact_path"`
	ThumbnailFormat       string  `json:"thumbnail_format" db:"thumbnail_format"`
	ThumbnailWidth        *int    `json:"thumbnail_width" db:"thumbnail_width"`
	ThumbnailHeight       *int    `json:"thumbnail_height" db:"thumbnail_height"`

	ExportStatus string  `json:"export_status" db:"export_status"`
	ExportError  *string `json:"export_error" db:"export_error"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

const (
	ReelPlanStatusDraft          = "draft"
	ReelPlanStatusVideoRequested = "video_requested"
)

// Per-reel export readiness. Mirrors ExportJob's status vocabulary but
// scoped to a single reel's video.mp4 / thumbnail.png artifacts.
const (
	ReelExportStatusArtifactMissing  = "artifact_missing"           // neither video.mp4 nor thumbnail.png exist on disk yet
	ReelExportStatusVideoMissing     = "video_artifact_missing"     // thumbnail.png exists, video.mp4 does not
	ReelExportStatusThumbnailMissing = "thumbnail_artifact_missing" // video.mp4 exists, thumbnail.png does not
	ReelExportStatusReady            = "ready"                      // both real artifacts exist on disk
)

// VideoJob tracks the real media render pipeline for a reel plan. Status is
// always honest about what actually happened — never a fake "rendering" or
// "done" state.
type VideoJob struct {
	ID          string    `json:"id" db:"id"`
	WorkspaceID string    `json:"workspace_id" db:"workspace_id"`
	ReelPlanID  string    `json:"reel_plan_id" db:"reel_plan_id"`
	Status      string    `json:"status" db:"status"`
	Provider    string    `json:"provider" db:"provider"`
	Notes       string    `json:"notes" db:"notes"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

const (
	VideoJobStatusPendingProvider  = "pending_provider_connection"
	VideoJobStatusDraftReady       = "draft_ready"
	VideoJobStatusProviderMissing  = "provider_not_connected"
	VideoJobStatusRendererMissing  = "renderer_not_available"
	VideoJobStatusAudioMissing     = "audio_artifact_missing"
	VideoJobStatusThumbnailMissing = "thumbnail_artifact_missing"
	VideoJobStatusRendering        = "rendering"
	VideoJobStatusCompleted        = "completed"
	VideoJobStatusFailed           = "failed"
)

// ExportJob tracks a ZIP export request for a daily batch. The status
// always honestly reflects what was found/built — it never claims a
// completed ZIP exists unless every reel's video.mp4 and thumbnail.png
// were found as real files on disk and packaged.
type ExportJob struct {
	ID           string     `json:"id" db:"id"`
	WorkspaceID  string     `json:"workspace_id" db:"workspace_id"`
	DailyBatchID string     `json:"daily_batch_id" db:"daily_batch_id"`
	Status       string     `json:"status" db:"status"`
	ZipPath      *string    `json:"zip_path" db:"zip_path"`
	ErrorMessage *string    `json:"error_message" db:"error_message"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	CompletedAt  *time.Time `json:"completed_at" db:"completed_at"`
}

const (
	// ExportJobStatusNotImplemented is a legacy status from before Phase 4B;
	// kept only so old rows still decode. No longer produced by the handler.
	ExportJobStatusNotImplemented   = "zip_generation_not_implemented"
	ExportJobStatusVideoMissing     = "video_artifact_missing"
	ExportJobStatusThumbnailMissing = "thumbnail_artifact_missing"
	ExportJobStatusMediaMissing     = "media_artifacts_missing"
	ExportJobStatusFailed           = "failed"
	ExportJobStatusCompleted        = "completed"
)

const PublishJobStatusPlatformNotConnected PublishJobStatus = "platform_not_connected"
