-- TrendCortex - Content Project outputs
-- Safe, additive migration for persistent project-linked clip output metadata.

CREATE TABLE IF NOT EXISTS content_project_outputs (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id             UUID NOT NULL REFERENCES content_projects(id) ON DELETE CASCADE,
    scene_id               UUID REFERENCES content_project_scenes(id) ON DELETE SET NULL,
    output_scope           TEXT NOT NULL,
    output_type            TEXT NOT NULL,
    source_workflow        TEXT NOT NULL DEFAULT 'clip_generator',
    render_job_id          TEXT,
    status                 TEXT NOT NULL DEFAULT 'queued',
    original_filename      TEXT,
    display_name           TEXT NOT NULL,
    mime_type              TEXT,
    file_size_bytes        BIGINT,
    duration_seconds       NUMERIC(10,3),
    width                  INT,
    height                 INT,
    storage_reference      TEXT,
    failure_category       TEXT,
    failure_message        TEXT,
    retryable              BOOLEAN NOT NULL DEFAULT FALSE,
    retry_count            INT NOT NULL DEFAULT 0,
    request_payload        JSONB,
    archived_at            TIMESTAMPTZ,
    completed_at           TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT content_project_outputs_scope_check CHECK (output_scope IN ('project', 'scene')),
    CONSTRAINT content_project_outputs_type_check CHECK (output_type IN ('generated_clip', 'uploaded_clip', 'imported_clip', 'rendered_video')),
    CONSTRAINT content_project_outputs_status_check CHECK (status IN ('queued', 'processing', 'completed', 'failed', 'unavailable', 'archived')),
    CONSTRAINT content_project_outputs_scene_scope_check CHECK (
        (output_scope = 'scene' AND scene_id IS NOT NULL) OR
        (output_scope = 'project' AND scene_id IS NULL)
    ),
    CONSTRAINT content_project_outputs_file_size_check CHECK (file_size_bytes IS NULL OR file_size_bytes >= 0),
    CONSTRAINT content_project_outputs_duration_check CHECK (duration_seconds IS NULL OR duration_seconds >= 0),
    CONSTRAINT content_project_outputs_width_check CHECK (width IS NULL OR width >= 0),
    CONSTRAINT content_project_outputs_height_check CHECK (height IS NULL OR height >= 0),
    CONSTRAINT content_project_outputs_retry_count_check CHECK (retry_count >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_content_project_outputs_project_render_job
    ON content_project_outputs(project_id, render_job_id)
    WHERE render_job_id IS NOT NULL AND render_job_id <> '';

CREATE INDEX IF NOT EXISTS idx_content_project_outputs_project_updated
    ON content_project_outputs(project_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_content_project_outputs_project_status
    ON content_project_outputs(project_id, status);

CREATE INDEX IF NOT EXISTS idx_content_project_outputs_scene
    ON content_project_outputs(scene_id)
    WHERE scene_id IS NOT NULL;
