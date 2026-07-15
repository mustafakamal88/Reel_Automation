package database

// Schema contains the DDL to create all TrendCortex tables.
// Run this once on a fresh database or apply it through a migration tool.
// Each table uses UUID primary keys (gen_random_uuid()) available in Postgres 13+.
const Schema = `
-- Users & Workspaces
CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    workspace_id  UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS workspaces (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    owner_id   UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS brand_profiles (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    niche         TEXT NOT NULL DEFAULT '',
    brand_voice   TEXT NOT NULL DEFAULT '',
    content_style TEXT NOT NULL DEFAULT '',
    region        TEXT NOT NULL DEFAULT 'US',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Platform OAuth connections
-- Raw tokens are NEVER stored here; only metadata + vault reference ID.
CREATE TABLE IF NOT EXISTS platform_accounts (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    platform          TEXT NOT NULL,
    handle            TEXT,
    token_status      TEXT NOT NULL DEFAULT 'not_connected',
    token_vault_ref_id UUID,
    scopes            TEXT[] NOT NULL DEFAULT '{}',
    can_publish       BOOLEAN NOT NULL DEFAULT FALSE,
    connected_at      TIMESTAMPTZ,
    expires_at        TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, platform)
);

-- Temporary OAuth state during redirect flow
CREATE TABLE IF NOT EXISTS oauth_connections (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    platform      TEXT NOT NULL,
    state         TEXT NOT NULL UNIQUE,
    code_verifier TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '10 minutes'
);

-- Encrypted token storage (ciphertext only; key stored separately in KMS)
CREATE TABLE IF NOT EXISTS token_vault_refs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform_key      TEXT NOT NULL,
    workspace_id      UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    encrypted_payload BYTEA NOT NULL,
    key_version       INT NOT NULL DEFAULT 1,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Video batches (6 videos per day per workspace)
CREATE TABLE IF NOT EXISTS video_batches (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    date         DATE NOT NULL,
    status       TEXT NOT NULL DEFAULT 'draft',
    video_count  INT NOT NULL DEFAULT 0,
    zip_path     TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, date)
);

-- Individual video assets within a batch
CREATE TABLE IF NOT EXISTS video_assets (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id       UUID NOT NULL REFERENCES video_batches(id) ON DELETE CASCADE,
    workspace_id   UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    rank           INT NOT NULL,
    title          TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'draft',
    platforms      TEXT[] NOT NULL DEFAULT '{}',
    video_path     TEXT,
    thumbnail_path TEXT,
    captions_path  TEXT,
    description    TEXT NOT NULL DEFAULT '',
    hashtags       TEXT NOT NULL DEFAULT '',
    ai_disclosure  BOOLEAN NOT NULL DEFAULT TRUE,
    human_approved BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Per-video per-platform publish jobs
CREATE TABLE IF NOT EXISTS publish_jobs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    video_asset_id    UUID NOT NULL REFERENCES video_assets(id) ON DELETE CASCADE,
    platform          TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'queued',
    retry_count       INT NOT NULL DEFAULT 0,
    error_message     TEXT,
    platform_post_id  TEXT,
    scheduled_for     TIMESTAMPTZ,
    started_at        TIMESTAMPTZ,
    completed_at      TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ZIP download jobs
CREATE TABLE IF NOT EXISTS download_zip_jobs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    batch_id       UUID NOT NULL REFERENCES video_batches(id) ON DELETE CASCADE,
    status         TEXT NOT NULL DEFAULT 'queued',
    zip_path       TEXT,
    zip_size_bytes BIGINT,
    error_message  TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at   TIMESTAMPTZ
);

-- Analytics snapshots (fetched via platform APIs)
CREATE TABLE IF NOT EXISTS analytics_snapshots (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    platform        TEXT NOT NULL,
    video_asset_id  UUID NOT NULL REFERENCES video_assets(id) ON DELETE CASCADE,
    views           BIGINT NOT NULL DEFAULT 0,
    likes           BIGINT NOT NULL DEFAULT 0,
    shares          BIGINT NOT NULL DEFAULT 0,
    comments        BIGINT NOT NULL DEFAULT 0,
    watch_time_sec  BIGINT NOT NULL DEFAULT 0,
    snapshot_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Platform rate limits and constraints (updated when policies change)
CREATE TABLE IF NOT EXISTS platform_rate_limits (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform            TEXT UNIQUE NOT NULL,
    daily_upload_limit  INT NOT NULL DEFAULT 3,
    min_secs_between    INT NOT NULL DEFAULT 3600,
    max_duration_sec    INT NOT NULL DEFAULT 60,
    max_file_size_mb    INT NOT NULL DEFAULT 500,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Platform policy changes log
CREATE TABLE IF NOT EXISTS policy_change_log (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform     TEXT NOT NULL,
    change_type  TEXT NOT NULL,
    description  TEXT NOT NULL,
    effective_at TIMESTAMPTZ NOT NULL,
    source       TEXT NOT NULL DEFAULT 'manual',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Audit log for all sensitive actions
CREATE TABLE IF NOT EXISTS audit_log (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      UUID,
    action       TEXT NOT NULL,
    resource     TEXT NOT NULL,
    resource_id  UUID,
    ip_address   TEXT,
    user_agent   TEXT,
    metadata     JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Default platform rate limits
INSERT INTO platform_rate_limits (platform, daily_upload_limit, min_secs_between, max_duration_sec, max_file_size_mb)
VALUES
    ('youtube',   100, 0,    60,  256),
    ('tiktok',    10,  3600, 60,  287),
    ('instagram', 25,  0,    90,  1000),
    ('facebook',  25,  0,    240, 1000),
    ('threads',   25,  0,    60,  1000),
    ('x',         17,  0,    140, 512)
ON CONFLICT (platform) DO NOTHING;
`

// SchemaPhase4 adds the real trend discovery → scoring → daily batch →
// reel plan → video/export/publish job pipeline (Phase 4A).
//
// "social_connections" and "audit_events" from the Phase 4A spec are
// intentionally NOT duplicated here: platform_accounts (Phase 1.5) and
// audit_log already cover that exact purpose for this workspace model,
// and a second connection/audit table would just create two sources of
// truth that could drift apart. The handlers in this phase read
// platform_accounts directly when checking publish eligibility.
const SchemaPhase4 = `
CREATE TABLE IF NOT EXISTS trend_sources (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    source_type  TEXT NOT NULL DEFAULT 'manual',
    status       TEXT NOT NULL DEFAULT 'active',
    confidence   NUMERIC(4,3) NOT NULL DEFAULT 0.500,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS trend_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    trend_source_id UUID NOT NULL REFERENCES trend_sources(id) ON DELETE CASCADE,
    topic           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    platform_hint   TEXT NOT NULL DEFAULT '',
    velocity        NUMERIC(6,3) NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'new',
    discovered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS topic_scores (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id             UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    trend_item_id            UUID NOT NULL UNIQUE REFERENCES trend_items(id) ON DELETE CASCADE,
    total_score              NUMERIC(6,3) NOT NULL,
    velocity_score           NUMERIC(6,3) NOT NULL,
    source_confidence_score  NUMERIC(6,3) NOT NULL,
    platform_fit_score       NUMERIC(6,3) NOT NULL,
    safety_score             NUMERIC(6,3) NOT NULL,
    watch_time_score         NUMERIC(6,3) NOT NULL,
    competition_score        NUMERIC(6,3) NOT NULL,
    reason                   TEXT NOT NULL DEFAULT '',
    breakdown                JSONB NOT NULL DEFAULT '{}',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS daily_batches (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    batch_date   DATE NOT NULL,
    status       TEXT NOT NULL DEFAULT 'planned',
    reel_count   INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, batch_date)
);

CREATE TABLE IF NOT EXISTS reel_plans (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id      UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    daily_batch_id    UUID NOT NULL REFERENCES daily_batches(id) ON DELETE CASCADE,
    trend_item_id     UUID NOT NULL REFERENCES trend_items(id) ON DELETE CASCADE,
    topic_score_id    UUID NOT NULL REFERENCES topic_scores(id) ON DELETE CASCADE,
    rank              INT NOT NULL,
    platform          TEXT NOT NULL DEFAULT '',
    title_idea        TEXT NOT NULL DEFAULT '',
    script_outline    TEXT NOT NULL DEFAULT '',
    description_draft TEXT NOT NULL DEFAULT '',
    hashtags_draft    TEXT NOT NULL DEFAULT '',
    thumbnail_idea    TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'draft',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(daily_batch_id, rank)
);

CREATE TABLE IF NOT EXISTS video_jobs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    reel_plan_id UUID NOT NULL UNIQUE REFERENCES reel_plans(id) ON DELETE CASCADE,
    status       TEXT NOT NULL DEFAULT 'pending_provider_connection',
    provider     TEXT NOT NULL DEFAULT '',
    notes        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS export_jobs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    daily_batch_id UUID NOT NULL REFERENCES daily_batches(id) ON DELETE CASCADE,
    status         TEXT NOT NULL DEFAULT 'zip_generation_not_implemented',
    zip_path       TEXT,
    error_message  TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at   TIMESTAMPTZ
);

-- Extend the Phase 1.5 publish_jobs table to also accept reel_plan-sourced
-- jobs (Phase 4A) alongside the original video_asset-sourced jobs. The
-- table is empty in production (the old handlers are 501 stubs), so this
-- is a safe, non-destructive extension rather than a new table.
ALTER TABLE publish_jobs ALTER COLUMN video_asset_id DROP NOT NULL;
ALTER TABLE publish_jobs ADD COLUMN IF NOT EXISTS reel_plan_id UUID REFERENCES reel_plans(id) ON DELETE CASCADE;

CREATE TABLE IF NOT EXISTS trend_intelligence_cache (
    cache_key TEXT PRIMARY KEY,
    payload   JSONB NOT NULL,
    stored_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_trend_intelligence_cache_expires_at ON trend_intelligence_cache(expires_at);
`

// SchemaPhase4B adds per-reel video/thumbnail artifact tracking columns to
// reel_plans so ZIP export can honestly tell whether a real rendered video
// and thumbnail exist on disk for each reel, instead of always reporting
// "not implemented". These columns stay NULL/empty until a real render
// pipeline writes real files and records their paths — see
// migrations/003_phase4b_export.sql.
const SchemaPhase4B = `
ALTER TABLE reel_plans ADD COLUMN IF NOT EXISTS video_artifact_path TEXT;
ALTER TABLE reel_plans ADD COLUMN IF NOT EXISTS video_format TEXT NOT NULL DEFAULT '';
ALTER TABLE reel_plans ADD COLUMN IF NOT EXISTS video_width INT;
ALTER TABLE reel_plans ADD COLUMN IF NOT EXISTS video_height INT;
ALTER TABLE reel_plans ADD COLUMN IF NOT EXISTS video_duration_seconds NUMERIC(8,3);
ALTER TABLE reel_plans ADD COLUMN IF NOT EXISTS video_codec TEXT NOT NULL DEFAULT '';
ALTER TABLE reel_plans ADD COLUMN IF NOT EXISTS audio_codec TEXT NOT NULL DEFAULT '';
ALTER TABLE reel_plans ADD COLUMN IF NOT EXISTS thumbnail_artifact_path TEXT;
ALTER TABLE reel_plans ADD COLUMN IF NOT EXISTS thumbnail_format TEXT NOT NULL DEFAULT '';
ALTER TABLE reel_plans ADD COLUMN IF NOT EXISTS thumbnail_width INT;
ALTER TABLE reel_plans ADD COLUMN IF NOT EXISTS thumbnail_height INT;
ALTER TABLE reel_plans ADD COLUMN IF NOT EXISTS export_status TEXT NOT NULL DEFAULT 'artifact_missing';
ALTER TABLE reel_plans ADD COLUMN IF NOT EXISTS export_error TEXT;
`

// SchemaContentProjects adds the persistent Content Project foundation used by
// the Content workspace. It is additive and keeps legacy Script Studio content
// importable without altering older research or reel tables.
const SchemaContentProjects = `
CREATE TABLE IF NOT EXISTS content_projects (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id            UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    title                   TEXT NOT NULL,
    topic                   TEXT NOT NULL DEFAULT '',
    source_type             TEXT NOT NULL DEFAULT 'manual',
    source_reference        TEXT NOT NULL DEFAULT '',
    source_label            TEXT NOT NULL DEFAULT '',
    status                  TEXT NOT NULL DEFAULT 'idea',
    current_stage           TEXT NOT NULL DEFAULT 'idea',
    target_platforms        TEXT[] NOT NULL DEFAULT '{}',
    content_format          TEXT NOT NULL DEFAULT 'short_video',
    target_duration_seconds INT NOT NULL DEFAULT 30,
    language                TEXT NOT NULL DEFAULT 'en-US',
    creative_brief          JSONB NOT NULL DEFAULT '{}',
    hook                    TEXT NOT NULL DEFAULT '',
    main_script             TEXT NOT NULL DEFAULT '',
    caption                 TEXT NOT NULL DEFAULT '',
    hashtags                TEXT[] NOT NULL DEFAULT '{}',
    platform_text           JSONB NOT NULL DEFAULT '{}',
    legacy_import_key       TEXT NOT NULL DEFAULT '',
    legacy_saved_at         TIMESTAMPTZ,
    archived_at             TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_content_projects_workspace_updated ON content_projects(workspace_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_content_projects_workspace_status ON content_projects(workspace_id, status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_content_projects_legacy_import_key
    ON content_projects(workspace_id, legacy_import_key)
    WHERE legacy_import_key <> '';
`

const SchemaContentProjectScenes = `
CREATE TABLE IF NOT EXISTS content_project_scenes (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id               UUID NOT NULL REFERENCES content_projects(id) ON DELETE CASCADE,
    position                 INT NOT NULL,
    title                    TEXT NOT NULL DEFAULT '',
    spoken_text              TEXT NOT NULL DEFAULT '',
    on_screen_text           TEXT NOT NULL DEFAULT '',
    visual_direction         TEXT NOT NULL DEFAULT '',
    broll_direction          TEXT NOT NULL DEFAULT '',
    camera_direction         TEXT NOT NULL DEFAULT '',
    transition_direction     TEXT NOT NULL DEFAULT '',
    planned_duration_seconds INT NOT NULL DEFAULT 5,
    production_notes         TEXT NOT NULL DEFAULT '',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT content_project_scenes_position_positive CHECK (position > 0),
    CONSTRAINT content_project_scenes_duration_bounds CHECK (planned_duration_seconds BETWEEN 1 AND 600)
);

CREATE INDEX IF NOT EXISTS idx_content_project_scenes_project_position
    ON content_project_scenes(project_id, position, created_at);

CREATE INDEX IF NOT EXISTS idx_content_project_scenes_project_updated
    ON content_project_scenes(project_id, updated_at DESC);
`

const SchemaContentProjectOutputs = `
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
    storage_provider       TEXT,
    storage_key            TEXT,
    storage_etag           TEXT,
    storage_checksum_sha256 TEXT,
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
`

const SchemaDurableMediaStorage = `
ALTER TABLE content_project_outputs
    ADD COLUMN IF NOT EXISTS storage_provider TEXT,
    ADD COLUMN IF NOT EXISTS storage_key TEXT,
    ADD COLUMN IF NOT EXISTS storage_etag TEXT,
    ADD COLUMN IF NOT EXISTS storage_checksum_sha256 TEXT;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'content_project_outputs_storage_provider_check') THEN
        ALTER TABLE content_project_outputs ADD CONSTRAINT content_project_outputs_storage_provider_check CHECK (storage_provider IS NULL OR storage_provider IN ('local', 's3'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'content_project_outputs_storage_key_check') THEN
        ALTER TABLE content_project_outputs ADD CONSTRAINT content_project_outputs_storage_key_check CHECK (storage_key IS NULL OR (length(storage_key) > 0 AND storage_key !~ '(^/|(^|/)\.\.(/|$)|\\\\|//)'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'content_project_outputs_checksum_check') THEN
        ALTER TABLE content_project_outputs ADD CONSTRAINT content_project_outputs_checksum_check CHECK (storage_checksum_sha256 IS NULL OR storage_checksum_sha256 ~ '^[a-f0-9]{64}$');
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_content_project_outputs_storage_object
    ON content_project_outputs(storage_provider, storage_key)
    WHERE storage_provider IS NOT NULL AND storage_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS clip_studio_sources (
    id                       TEXT PRIMARY KEY,
    workspace_id             UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    source_kind              TEXT NOT NULL,
    original_filename        TEXT,
    original_url             TEXT,
    storage_provider         TEXT,
    storage_key              TEXT,
    storage_etag             TEXT,
    storage_checksum_sha256  TEXT,
    content_type             TEXT,
    size_bytes               BIGINT,
    source_model             TEXT,
    rights_metadata          JSONB NOT NULL DEFAULT '{}',
    status                   TEXT NOT NULL,
    status_message           TEXT,
    direct_video             BOOLEAN NOT NULL DEFAULT FALSE,
    supported_type           BOOLEAN NOT NULL DEFAULT FALSE,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clip_studio_sources_kind_check CHECK (source_kind IN ('upload', 'url')),
    CONSTRAINT clip_studio_sources_status_check CHECK (status IN ('ready', 'metadata_only', 'rights_required', 'failed', 'unavailable')),
    CONSTRAINT clip_studio_sources_storage_provider_check CHECK (storage_provider IS NULL OR storage_provider IN ('local', 's3')),
    CONSTRAINT clip_studio_sources_storage_key_check CHECK (storage_key IS NULL OR (length(storage_key) > 0 AND storage_key !~ '(^/|(^|/)\.\.(/|$)|\\\\|//)')),
    CONSTRAINT clip_studio_sources_checksum_check CHECK (storage_checksum_sha256 IS NULL OR storage_checksum_sha256 ~ '^[a-f0-9]{64}$'),
    CONSTRAINT clip_studio_sources_size_check CHECK (size_bytes IS NULL OR size_bytes >= 0),
    CONSTRAINT clip_studio_sources_ready_storage_check CHECK (status <> 'ready' OR (storage_provider IS NOT NULL AND storage_key IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS idx_clip_studio_sources_workspace_updated
    ON clip_studio_sources(workspace_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_clip_studio_sources_storage_object
    ON clip_studio_sources(storage_provider, storage_key)
    WHERE storage_provider IS NOT NULL AND storage_key IS NOT NULL;
`

const SchemaClipStudioExports = `
CREATE TABLE IF NOT EXISTS clip_studio_exports (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id             UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    filename                 TEXT NOT NULL,
    export_kind              TEXT NOT NULL,
    generation_id            TEXT,
    storage_provider         TEXT NOT NULL,
    storage_key              TEXT NOT NULL,
    storage_etag             TEXT,
    storage_checksum_sha256  TEXT,
    mime_type                TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes               BIGINT,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT clip_studio_exports_kind_check CHECK (export_kind IN ('manual_zip', 'generated_zip', 'ai_scene_zip', 'ai_scene_video', 'ai_scene_thumbnail')),
    CONSTRAINT clip_studio_exports_storage_provider_check CHECK (storage_provider IN ('local', 's3')),
    CONSTRAINT clip_studio_exports_storage_key_check CHECK (length(storage_key) > 0 AND storage_key !~ '(^/|(^|/)\.\.(/|$)|\\\\|//)'),
    CONSTRAINT clip_studio_exports_checksum_check CHECK (storage_checksum_sha256 IS NULL OR storage_checksum_sha256 ~ '^[a-f0-9]{64}$'),
    CONSTRAINT clip_studio_exports_size_check CHECK (size_bytes IS NULL OR size_bytes >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_clip_studio_exports_workspace_filename
    ON clip_studio_exports(workspace_id, filename);

CREATE INDEX IF NOT EXISTS idx_clip_studio_exports_generation
    ON clip_studio_exports(workspace_id, generation_id, export_kind)
    WHERE generation_id IS NOT NULL;
`

const SchemaMediaAssets = `
CREATE TABLE IF NOT EXISTS media_assets (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id             UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id               UUID REFERENCES content_projects(id) ON DELETE SET NULL,
    scene_id                 UUID REFERENCES content_project_scenes(id) ON DELETE SET NULL,
    asset_type               TEXT NOT NULL,
    source_workflow          TEXT NOT NULL DEFAULT 'clip_studio',
    display_name             TEXT NOT NULL,
    original_filename        TEXT,
    mime_type                TEXT,
    size_bytes               BIGINT,
    duration_seconds         NUMERIC(10,3),
    width                    INT,
    height                   INT,
    storage_provider         TEXT NOT NULL,
    storage_key              TEXT NOT NULL,
    storage_etag             TEXT,
    storage_checksum_sha256  TEXT,
    status                   TEXT NOT NULL DEFAULT 'ready',
    failure_category         TEXT,
    failure_message          TEXT,
    archived_at              TIMESTAMPTZ,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_verified_at         TIMESTAMPTZ,
    CONSTRAINT media_assets_type_check CHECK (asset_type IN ('source_video', 'generated_video', 'rendered_video', 'ai_scene_video', 'thumbnail', 'package', 'metadata', 'audio', 'voiceover')),
    CONSTRAINT media_assets_workflow_check CHECK (source_workflow IN ('clip_studio', 'clip_generator', 'project_output', 'ai_scene', 'voice_studio', 'movie_studio')),
    CONSTRAINT media_assets_status_check CHECK (status IN ('ready', 'processing', 'failed', 'unavailable', 'archived')),
    CONSTRAINT media_assets_storage_provider_check CHECK (storage_provider IN ('local', 's3')),
    CONSTRAINT media_assets_storage_key_check CHECK (length(storage_key) > 0 AND storage_key !~ '(^/|(^|/)\.\.(/|$)|\\\\|//)'),
    CONSTRAINT media_assets_checksum_check CHECK (storage_checksum_sha256 IS NULL OR storage_checksum_sha256 ~ '^[a-f0-9]{64}$'),
    CONSTRAINT media_assets_size_check CHECK (size_bytes IS NULL OR size_bytes >= 0),
    CONSTRAINT media_assets_duration_check CHECK (duration_seconds IS NULL OR duration_seconds >= 0),
    CONSTRAINT media_assets_width_check CHECK (width IS NULL OR width >= 0),
    CONSTRAINT media_assets_height_check CHECK (height IS NULL OR height >= 0),
    CONSTRAINT media_assets_ready_storage_check CHECK (status <> 'ready' OR (storage_provider IS NOT NULL AND storage_key IS NOT NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_media_assets_workspace_storage_object
    ON media_assets(workspace_id, storage_provider, storage_key);

CREATE INDEX IF NOT EXISTS idx_media_assets_workspace_created
    ON media_assets(workspace_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_media_assets_workspace_type
    ON media_assets(workspace_id, asset_type);

CREATE INDEX IF NOT EXISTS idx_media_assets_workspace_status
    ON media_assets(workspace_id, status);

CREATE INDEX IF NOT EXISTS idx_media_assets_project
    ON media_assets(project_id)
    WHERE project_id IS NOT NULL;

ALTER TABLE media_assets DROP CONSTRAINT IF EXISTS media_assets_type_check;
ALTER TABLE media_assets
    ADD CONSTRAINT media_assets_type_check CHECK (asset_type IN ('source_video', 'generated_video', 'rendered_video', 'ai_scene_video', 'thumbnail', 'package', 'metadata', 'audio', 'voiceover'));

ALTER TABLE media_assets DROP CONSTRAINT IF EXISTS media_assets_workflow_check;
ALTER TABLE media_assets
    ADD CONSTRAINT media_assets_workflow_check CHECK (source_workflow IN ('clip_studio', 'clip_generator', 'project_output', 'ai_scene', 'voice_studio', 'movie_studio'));

CREATE TABLE IF NOT EXISTS voice_generations (
    id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id               UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id                 UUID REFERENCES content_projects(id) ON DELETE SET NULL,
    scene_id                   UUID REFERENCES content_project_scenes(id) ON DELETE SET NULL,
    asset_id                   UUID REFERENCES media_assets(id) ON DELETE SET NULL,
    parent_generation_id       UUID REFERENCES voice_generations(id) ON DELETE SET NULL,
    mode                       TEXT NOT NULL,
    source_type                TEXT NOT NULL,
    source_script_id           TEXT,
    voice_id                   TEXT NOT NULL,
    voice_display_name         TEXT NOT NULL,
    speed                      NUMERIC(4,2) NOT NULL DEFAULT 1.0,
    delivery_preset            TEXT NOT NULL DEFAULT 'natural',
    custom_instructions        TEXT,
    output_format              TEXT NOT NULL DEFAULT 'mp3',
    provider_name              TEXT NOT NULL,
    provider_model             TEXT NOT NULL,
    provider_reference         TEXT,
    source_text_hash           TEXT NOT NULL,
    input_characters           INT NOT NULL DEFAULT 0,
    estimated_duration_seconds NUMERIC(10,3),
    generated_duration_seconds NUMERIC(10,3),
    file_size_bytes            BIGINT,
    scene_count                INT NOT NULL DEFAULT 0,
    usage_kind                 TEXT NOT NULL,
    status                     TEXT NOT NULL,
    failure_category           TEXT,
    failure_message            TEXT,
    idempotency_key            TEXT,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at               TIMESTAMPTZ,
    CONSTRAINT voice_generations_mode_check CHECK (mode IN ('full', 'scene')),
    CONSTRAINT voice_generations_source_type_check CHECK (source_type IN ('blank', 'project_script', 'script_lab', 'scene', 'all_scenes', 'upload')),
    CONSTRAINT voice_generations_usage_kind_check CHECK (usage_kind IN ('preview', 'production', 'upload')),
    CONSTRAINT voice_generations_status_check CHECK (status IN ('queued', 'preparing', 'generating', 'processing', 'saving', 'completed', 'partially_completed', 'failed')),
    CONSTRAINT voice_generations_format_check CHECK (output_format IN ('mp3', 'opus', 'aac', 'flac', 'wav', 'pcm')),
    CONSTRAINT voice_generations_hash_check CHECK (source_text_hash ~ '^[a-f0-9]{64}$'),
    CONSTRAINT voice_generations_chars_check CHECK (input_characters >= 0),
    CONSTRAINT voice_generations_file_size_check CHECK (file_size_bytes IS NULL OR file_size_bytes >= 0)
);

CREATE INDEX IF NOT EXISTS idx_voice_generations_workspace_created
    ON voice_generations(workspace_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_voice_generations_project
    ON voice_generations(project_id, created_at DESC)
    WHERE project_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_voice_generations_idempotency
    ON voice_generations(workspace_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND idempotency_key <> '';

ALTER TABLE content_projects
    ADD COLUMN IF NOT EXISTS active_voiceover_asset_id UUID REFERENCES media_assets(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_content_projects_active_voiceover
    ON content_projects(active_voiceover_asset_id)
    WHERE active_voiceover_asset_id IS NOT NULL;

CREATE OR REPLACE FUNCTION media_assets_validate_owner()
RETURNS trigger AS $$
DECLARE
    project_workspace UUID;
    scene_project UUID;
BEGIN
    IF NEW.project_id IS NOT NULL THEN
        SELECT workspace_id INTO project_workspace FROM content_projects WHERE id = NEW.project_id;
        IF project_workspace IS NULL OR project_workspace <> NEW.workspace_id THEN
            RAISE EXCEPTION 'media asset project must belong to workspace';
        END IF;
    END IF;
    IF NEW.scene_id IS NOT NULL THEN
        SELECT project_id INTO scene_project FROM content_project_scenes WHERE id = NEW.scene_id;
        IF scene_project IS NULL OR NEW.project_id IS NULL OR scene_project <> NEW.project_id THEN
            RAISE EXCEPTION 'media asset scene must belong to project';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_media_assets_validate_owner ON media_assets;
CREATE TRIGGER trg_media_assets_validate_owner
BEFORE INSERT OR UPDATE OF workspace_id, project_id, scene_id ON media_assets
FOR EACH ROW EXECUTE FUNCTION media_assets_validate_owner();

ALTER TABLE content_project_outputs
    ADD COLUMN IF NOT EXISTS asset_id UUID REFERENCES media_assets(id) ON DELETE SET NULL;

ALTER TABLE clip_studio_sources
    ADD COLUMN IF NOT EXISTS asset_id UUID REFERENCES media_assets(id) ON DELETE SET NULL;

ALTER TABLE clip_studio_exports
    ADD COLUMN IF NOT EXISTS asset_id UUID REFERENCES media_assets(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_content_project_outputs_asset_id
    ON content_project_outputs(asset_id)
    WHERE asset_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_clip_studio_sources_asset_id
    ON clip_studio_sources(asset_id)
    WHERE asset_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_clip_studio_exports_asset_id
    ON clip_studio_exports(asset_id)
    WHERE asset_id IS NOT NULL;

INSERT INTO media_assets (
    workspace_id, asset_type, source_workflow, display_name, original_filename, mime_type,
    size_bytes, storage_provider, storage_key, storage_etag, storage_checksum_sha256,
    status, failure_category, failure_message, created_at, updated_at
)
SELECT
    s.workspace_id, 'source_video', 'clip_studio',
    COALESCE(NULLIF(s.original_filename, ''), 'Clip Studio source'),
    NULLIF(s.original_filename, ''), NULLIF(s.content_type, ''), s.size_bytes,
    s.storage_provider, s.storage_key, NULLIF(s.storage_etag, ''), NULLIF(s.storage_checksum_sha256, ''),
    CASE WHEN s.status = 'ready' THEN 'ready' ELSE 'unavailable' END,
    CASE WHEN s.status = 'ready' THEN NULL ELSE 'source_unavailable' END,
    CASE WHEN s.status = 'ready' THEN NULL ELSE NULLIF(s.status_message, '') END,
    s.created_at, s.updated_at
FROM clip_studio_sources s
WHERE s.storage_provider IN ('local', 's3') AND s.storage_key IS NOT NULL AND s.storage_key <> ''
  AND s.storage_key !~ '(^/|(^|/)\.\.(/|$)|\\\\|//)'
ON CONFLICT (workspace_id, storage_provider, storage_key) DO UPDATE SET
    display_name = COALESCE(NULLIF(media_assets.display_name, ''), EXCLUDED.display_name),
    original_filename = COALESCE(media_assets.original_filename, EXCLUDED.original_filename),
    mime_type = COALESCE(media_assets.mime_type, EXCLUDED.mime_type),
    size_bytes = COALESCE(media_assets.size_bytes, EXCLUDED.size_bytes),
    storage_etag = COALESCE(media_assets.storage_etag, EXCLUDED.storage_etag),
    storage_checksum_sha256 = COALESCE(media_assets.storage_checksum_sha256, EXCLUDED.storage_checksum_sha256),
    updated_at = NOW();

UPDATE clip_studio_sources s
SET asset_id = a.id
FROM media_assets a
WHERE s.workspace_id = a.workspace_id
  AND s.storage_provider = a.storage_provider
  AND s.storage_key = a.storage_key
  AND s.asset_id IS DISTINCT FROM a.id;

INSERT INTO media_assets (
    workspace_id, asset_type, source_workflow, display_name, original_filename, mime_type,
    size_bytes, storage_provider, storage_key, storage_etag, storage_checksum_sha256,
    status, created_at, updated_at
)
SELECT
    e.workspace_id,
    CASE WHEN e.export_kind = 'ai_scene_video' THEN 'ai_scene_video' WHEN e.export_kind = 'ai_scene_thumbnail' THEN 'thumbnail' ELSE 'package' END,
    CASE WHEN e.export_kind LIKE 'ai_scene_%' THEN 'ai_scene' ELSE 'clip_studio' END,
    e.filename, e.filename, NULLIF(e.mime_type, ''), e.size_bytes, e.storage_provider, e.storage_key,
    NULLIF(e.storage_etag, ''), NULLIF(e.storage_checksum_sha256, ''), 'ready', e.created_at, e.updated_at
FROM clip_studio_exports e
WHERE e.storage_provider IN ('local', 's3') AND e.storage_key IS NOT NULL AND e.storage_key <> ''
  AND e.storage_key !~ '(^/|(^|/)\.\.(/|$)|\\\\|//)'
ON CONFLICT (workspace_id, storage_provider, storage_key) DO UPDATE SET
    asset_type = CASE WHEN media_assets.asset_type = 'metadata' THEN EXCLUDED.asset_type ELSE media_assets.asset_type END,
    source_workflow = COALESCE(NULLIF(media_assets.source_workflow, ''), EXCLUDED.source_workflow),
    display_name = COALESCE(NULLIF(media_assets.display_name, ''), EXCLUDED.display_name),
    original_filename = COALESCE(media_assets.original_filename, EXCLUDED.original_filename),
    mime_type = COALESCE(media_assets.mime_type, EXCLUDED.mime_type),
    size_bytes = COALESCE(media_assets.size_bytes, EXCLUDED.size_bytes),
    storage_etag = COALESCE(media_assets.storage_etag, EXCLUDED.storage_etag),
    storage_checksum_sha256 = COALESCE(media_assets.storage_checksum_sha256, EXCLUDED.storage_checksum_sha256),
    updated_at = NOW();

UPDATE clip_studio_exports e
SET asset_id = a.id
FROM media_assets a
WHERE e.workspace_id = a.workspace_id
  AND e.storage_provider = a.storage_provider
  AND e.storage_key = a.storage_key
  AND e.asset_id IS DISTINCT FROM a.id;

INSERT INTO media_assets (
    workspace_id, project_id, scene_id, asset_type, source_workflow, display_name, original_filename, mime_type,
    size_bytes, duration_seconds, width, height, storage_provider, storage_key, storage_etag, storage_checksum_sha256,
    status, failure_category, failure_message, archived_at, created_at, updated_at
)
SELECT
    p.workspace_id, o.project_id, o.scene_id,
    CASE WHEN o.output_type IN ('uploaded_clip', 'imported_clip') THEN 'source_video'
         WHEN o.output_type = 'rendered_video' THEN 'rendered_video'
         WHEN o.mime_type = 'application/zip' THEN 'package'
         ELSE 'generated_video' END,
    'clip_generator', o.display_name, NULLIF(o.original_filename, ''), NULLIF(o.mime_type, ''),
    o.file_size_bytes, o.duration_seconds, o.width, o.height, o.storage_provider, o.storage_key,
    NULLIF(o.storage_etag, ''), NULLIF(o.storage_checksum_sha256, ''),
    CASE WHEN o.archived_at IS NOT NULL THEN 'archived' WHEN o.status = 'completed' THEN 'ready' WHEN o.status IN ('queued', 'processing') THEN 'processing' WHEN o.status = 'failed' THEN 'failed' ELSE 'unavailable' END,
    NULLIF(o.failure_category, ''), NULLIF(o.failure_message, ''), o.archived_at, o.created_at, o.updated_at
FROM content_project_outputs o
JOIN content_projects p ON p.id = o.project_id
WHERE o.storage_provider IN ('local', 's3') AND o.storage_key IS NOT NULL AND o.storage_key <> ''
  AND o.storage_key !~ '(^/|(^|/)\.\.(/|$)|\\\\|//)'
ON CONFLICT (workspace_id, storage_provider, storage_key) DO UPDATE SET
    project_id = COALESCE(media_assets.project_id, EXCLUDED.project_id),
    scene_id = COALESCE(media_assets.scene_id, EXCLUDED.scene_id),
    asset_type = CASE WHEN media_assets.asset_type = 'package' THEN media_assets.asset_type ELSE EXCLUDED.asset_type END,
    source_workflow = EXCLUDED.source_workflow,
    display_name = COALESCE(NULLIF(EXCLUDED.display_name, ''), media_assets.display_name),
    original_filename = COALESCE(media_assets.original_filename, EXCLUDED.original_filename),
    mime_type = COALESCE(media_assets.mime_type, EXCLUDED.mime_type),
    size_bytes = COALESCE(media_assets.size_bytes, EXCLUDED.size_bytes),
    duration_seconds = COALESCE(media_assets.duration_seconds, EXCLUDED.duration_seconds),
    width = COALESCE(media_assets.width, EXCLUDED.width),
    height = COALESCE(media_assets.height, EXCLUDED.height),
    storage_etag = COALESCE(media_assets.storage_etag, EXCLUDED.storage_etag),
    storage_checksum_sha256 = COALESCE(media_assets.storage_checksum_sha256, EXCLUDED.storage_checksum_sha256),
    updated_at = NOW();

UPDATE content_project_outputs o
SET asset_id = a.id
FROM content_projects p, media_assets a
WHERE p.id = o.project_id
  AND p.workspace_id = a.workspace_id
  AND o.storage_provider = a.storage_provider
  AND o.storage_key = a.storage_key
  AND o.asset_id IS DISTINCT FROM a.id;
`

const SchemaMovieStudio = `
ALTER TABLE media_assets DROP CONSTRAINT IF EXISTS media_assets_workflow_check;
ALTER TABLE media_assets
    ADD CONSTRAINT media_assets_workflow_check CHECK (source_workflow IN ('clip_studio', 'clip_generator', 'project_output', 'ai_scene', 'voice_studio', 'movie_studio'));

ALTER TABLE content_projects
    ADD COLUMN IF NOT EXISTS active_movie_edit_id UUID,
    ADD COLUMN IF NOT EXISTS active_rendered_video_asset_id UUID REFERENCES media_assets(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS movie_edits (
    id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id               UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id                    UUID REFERENCES users(id) ON DELETE SET NULL,
    content_project_id         UUID REFERENCES content_projects(id) ON DELETE SET NULL,
    name                       TEXT NOT NULL,
    status                     TEXT NOT NULL DEFAULT 'draft',
    version                    INT NOT NULL DEFAULT 1,
    timeline_schema_version    INT NOT NULL DEFAULT 1,
    aspect_ratio               TEXT NOT NULL DEFAULT '9:16',
    output_width               INT NOT NULL DEFAULT 1080,
    output_height              INT NOT NULL DEFAULT 1920,
    frame_rate                 INT NOT NULL DEFAULT 30,
    quality_preset             TEXT NOT NULL DEFAULT 'standard',
    voiceover_asset_id         UUID REFERENCES media_assets(id) ON DELETE SET NULL,
    music_asset_id             UUID REFERENCES media_assets(id) ON DELETE SET NULL,
    caption_settings           JSONB NOT NULL DEFAULT '{}',
    transition_settings        JSONB NOT NULL DEFAULT '{}',
    branding_settings          JSONB NOT NULL DEFAULT '{}',
    duration_seconds           NUMERIC(10,3),
    latest_successful_render_id UUID,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT movie_edits_status_check CHECK (status IN ('draft', 'rendering', 'rendered', 'archived')),
    CONSTRAINT movie_edits_aspect_check CHECK (aspect_ratio IN ('9:16')),
    CONSTRAINT movie_edits_quality_check CHECK (quality_preset IN ('draft', 'standard', 'high')),
    CONSTRAINT movie_edits_dimensions_check CHECK (output_width > 0 AND output_height > 0 AND frame_rate > 0),
    CONSTRAINT movie_edits_version_check CHECK (version > 0)
);

CREATE TABLE IF NOT EXISTS movie_scenes (
    id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    movie_edit_id              UUID NOT NULL REFERENCES movie_edits(id) ON DELETE CASCADE,
    source_project_scene_id    UUID REFERENCES content_project_scenes(id) ON DELETE SET NULL,
    position                   INT NOT NULL,
    title                      TEXT NOT NULL DEFAULT '',
    script_text                TEXT NOT NULL DEFAULT '',
    voiceover_asset_id         UUID REFERENCES media_assets(id) ON DELETE SET NULL,
    visual_asset_id            UUID REFERENCES media_assets(id) ON DELETE SET NULL,
    secondary_asset_id         UUID REFERENCES media_assets(id) ON DELETE SET NULL,
    start_seconds              NUMERIC(10,3) NOT NULL DEFAULT 0,
    duration_seconds           NUMERIC(10,3) NOT NULL DEFAULT 4,
    trim_in_seconds            NUMERIC(10,3),
    trim_out_seconds           NUMERIC(10,3),
    fit_mode                   TEXT NOT NULL DEFAULT 'fill_crop',
    focal_x                    NUMERIC(4,3) NOT NULL DEFAULT 0.5,
    focal_y                    NUMERIC(4,3) NOT NULL DEFAULT 0.5,
    zoom                       NUMERIC(5,3) NOT NULL DEFAULT 1.0,
    motion_preset              TEXT NOT NULL DEFAULT 'none',
    transition_type            TEXT NOT NULL DEFAULT 'cut',
    transition_duration_seconds NUMERIC(10,3) NOT NULL DEFAULT 0.25,
    muted                      BOOLEAN NOT NULL DEFAULT TRUE,
    volume                     NUMERIC(4,3) NOT NULL DEFAULT 1.0,
    caption_text               TEXT NOT NULL DEFAULT '',
    match_reason               TEXT NOT NULL DEFAULT '',
    match_confidence           NUMERIC(4,3),
    settings                   JSONB NOT NULL DEFAULT '{}',
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT movie_scenes_position_check CHECK (position > 0),
    CONSTRAINT movie_scenes_duration_check CHECK (duration_seconds > 0 AND duration_seconds <= 120),
    CONSTRAINT movie_scenes_trim_check CHECK (trim_in_seconds IS NULL OR trim_in_seconds >= 0),
    CONSTRAINT movie_scenes_trim_order_check CHECK (trim_out_seconds IS NULL OR trim_in_seconds IS NULL OR trim_out_seconds > trim_in_seconds),
    CONSTRAINT movie_scenes_fit_check CHECK (fit_mode IN ('fill_crop', 'fit_background', 'original')),
    CONSTRAINT movie_scenes_motion_check CHECK (motion_preset IN ('none', 'slow_zoom_in', 'slow_zoom_out', 'pan_left', 'pan_right', 'pan_up', 'pan_down')),
    CONSTRAINT movie_scenes_transition_check CHECK (transition_type IN ('cut', 'crossfade', 'fade_black', 'slide')),
    CONSTRAINT movie_scenes_transition_duration_check CHECK (transition_duration_seconds >= 0 AND transition_duration_seconds <= 2),
    CONSTRAINT movie_scenes_volume_check CHECK (volume >= 0 AND volume <= 2),
    CONSTRAINT movie_scenes_focal_check CHECK (focal_x >= 0 AND focal_x <= 1 AND focal_y >= 0 AND focal_y <= 1),
    CONSTRAINT movie_scenes_zoom_check CHECK (zoom >= 0.5 AND zoom <= 3)
);

CREATE TABLE IF NOT EXISTS movie_render_jobs (
    id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    movie_edit_id              UUID NOT NULL REFERENCES movie_edits(id) ON DELETE CASCADE,
    workspace_id               UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id                    UUID REFERENCES users(id) ON DELETE SET NULL,
    status                     TEXT NOT NULL DEFAULT 'queued',
    current_stage              TEXT NOT NULL DEFAULT 'queued',
    completed_scene_count      INT NOT NULL DEFAULT 0,
    total_scene_count          INT NOT NULL DEFAULT 0,
    retry_of_render_job_id     UUID REFERENCES movie_render_jobs(id) ON DELETE SET NULL,
    render_plan_version        INT NOT NULL DEFAULT 1,
    output_asset_id            UUID REFERENCES media_assets(id) ON DELETE SET NULL,
    output_duration_seconds    NUMERIC(10,3),
    output_size_bytes          BIGINT,
    output_width               INT,
    output_height              INT,
    output_video_codec         TEXT,
    output_audio_codec         TEXT,
    quality_preset             TEXT NOT NULL DEFAULT 'standard',
    idempotency_key            TEXT,
    error_code                 TEXT,
    failure_message            TEXT,
    diagnostics                JSONB NOT NULL DEFAULT '{}',
    started_at                 TIMESTAMPTZ,
    completed_at               TIMESTAMPTZ,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT movie_render_jobs_status_check CHECK (status IN ('queued', 'preparing_assets', 'probing_media', 'rendering_scenes', 'mixing_audio', 'encoding_final', 'uploading', 'completed', 'failed', 'cancelled')),
    CONSTRAINT movie_render_jobs_scene_count_check CHECK (completed_scene_count >= 0 AND total_scene_count >= 0),
    CONSTRAINT movie_render_jobs_size_check CHECK (output_size_bytes IS NULL OR output_size_bytes >= 0)
);

CREATE TABLE IF NOT EXISTS movie_render_usage (
    id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id               UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id                    UUID REFERENCES users(id) ON DELETE SET NULL,
    movie_edit_id              UUID REFERENCES movie_edits(id) ON DELETE SET NULL,
    render_job_id              UUID REFERENCES movie_render_jobs(id) ON DELETE SET NULL,
    status                     TEXT NOT NULL,
    output_duration_seconds    NUMERIC(10,3),
    output_width               INT,
    output_height              INT,
    quality_preset             TEXT NOT NULL DEFAULT 'standard',
    scene_count                INT NOT NULL DEFAULT 0,
    captions_enabled           BOOLEAN NOT NULL DEFAULT FALSE,
    music_enabled              BOOLEAN NOT NULL DEFAULT FALSE,
    processing_duration_seconds NUMERIC(10,3),
    storage_bytes_created      BIGINT,
    retry_of_render_job_id     UUID,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'content_projects_active_movie_edit_fk') THEN
        ALTER TABLE content_projects ADD CONSTRAINT content_projects_active_movie_edit_fk
            FOREIGN KEY (active_movie_edit_id) REFERENCES movie_edits(id) ON DELETE SET NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'movie_edits_latest_render_fk') THEN
        ALTER TABLE movie_edits ADD CONSTRAINT movie_edits_latest_render_fk
            FOREIGN KEY (latest_successful_render_id) REFERENCES movie_render_jobs(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_movie_edits_workspace_updated ON movie_edits(workspace_id, updated_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_movie_edits_project ON movie_edits(content_project_id, updated_at DESC) WHERE content_project_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_movie_scenes_edit_position ON movie_scenes(movie_edit_id, position, id);
CREATE INDEX IF NOT EXISTS idx_movie_render_jobs_workspace_created ON movie_render_jobs(workspace_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_movie_render_jobs_edit ON movie_render_jobs(movie_edit_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_movie_render_jobs_idempotency ON movie_render_jobs(workspace_id, movie_edit_id, idempotency_key) WHERE idempotency_key IS NOT NULL AND idempotency_key <> '';
CREATE INDEX IF NOT EXISTS idx_content_projects_active_movie_edit ON content_projects(active_movie_edit_id) WHERE active_movie_edit_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_content_projects_active_rendered_video ON content_projects(active_rendered_video_asset_id) WHERE active_rendered_video_asset_id IS NOT NULL;
`

// Migrate runs the schema DDL against the connected database.
// Safe to run multiple times due to IF NOT EXISTS clauses.
func (db *DB) Migrate() error {
	if _, err := db.Exec(Schema); err != nil {
		return err
	}
	if _, err := db.Exec(SchemaPhase4); err != nil {
		return err
	}
	if _, err := db.Exec(SchemaPhase4B); err != nil {
		return err
	}
	if _, err := db.Exec(SchemaContentProjects); err != nil {
		return err
	}
	if _, err := db.Exec(SchemaContentProjectScenes); err != nil {
		return err
	}
	if _, err := db.Exec(SchemaContentProjectOutputs); err != nil {
		return err
	}
	if _, err := db.Exec(SchemaDurableMediaStorage); err != nil {
		return err
	}
	if _, err := db.Exec(SchemaClipStudioExports); err != nil {
		return err
	}
	if _, err := db.Exec(SchemaMediaAssets); err != nil {
		return err
	}
	_, err := db.Exec(SchemaMovieStudio)
	return err
}
