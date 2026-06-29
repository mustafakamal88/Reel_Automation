package jobs

import (
	"context"
	"time"
)

// JobType identifies the kind of work a job performs.
type JobType string

const (
	JobTypePublishVideo  JobType = "publish_video"
	JobTypeCreateZip     JobType = "create_zip"
	JobTypeRefreshToken  JobType = "refresh_token"
	JobTypeFetchAnalytics JobType = "fetch_analytics"
)

// JobStatus reflects the current state of a queued job.
type JobStatus string

const (
	JobStatusQueued  JobStatus = "queued"
	JobStatusRunning JobStatus = "running"
	JobStatusDone    JobStatus = "done"
	JobStatusFailed  JobStatus = "failed"
	JobStatusSkipped JobStatus = "skipped"
)

// Job represents a single unit of async work.
type Job struct {
	ID          string
	Type        JobType
	WorkspaceID string
	Payload     []byte    // JSON-encoded job-specific payload
	Status      JobStatus
	RetryCount  int
	MaxRetries  int
	ScheduledAt time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	ErrorMsg    string
}

// Handler is the function signature for processing a job.
type Handler func(ctx context.Context, job *Job) error

// Queue defines the interface for a job queue backend.
// Production: implement with PostgreSQL SKIP LOCKED, Redis streams, or BullMQ.
type Queue interface {
	Enqueue(ctx context.Context, job *Job) error
	Dequeue(ctx context.Context, jobType JobType) (*Job, error)
	Complete(ctx context.Context, jobID string) error
	Fail(ctx context.Context, jobID, errMsg string) error
	Retry(ctx context.Context, jobID string, after time.Duration) error
}

// PublishVideoPayload is the JSON payload for a publish_video job.
type PublishVideoPayload struct {
	VideoAssetID string `json:"video_asset_id"`
	Platform     string `json:"platform"`
	AccessToken  string `json:"-"` // never persisted in queue; fetched from vault at execution time
}

// CreateZipPayload is the JSON payload for a create_zip job.
type CreateZipPayload struct {
	BatchID     string   `json:"batch_id"`
	VideoIDs    []string `json:"video_ids"`
	OutputPath  string   `json:"output_path"`
}

// ZipStructure documents the required layout of the daily batch ZIP.
// The actual file creation happens in the storage package.
//
// trendcortex-daily-batch-YYYY-MM-DD.zip
//   video-01/
//     video.mp4
//     thumbnail.jpg
//     title.txt
//     description.txt
//     hashtags.txt
//     captions.srt
//     platforms.json
//   video-02/ … video-06/
//   batch-summary.json
//   compliance-checklist.json
type ZipStructure struct{}
