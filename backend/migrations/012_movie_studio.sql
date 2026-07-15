-- TrendCortex - Production Movie Studio persistence.
-- Additive and repeatable. Completed renders and durable media are preserved.

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
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'content_projects_active_movie_edit_fk'
    ) THEN
        ALTER TABLE content_projects
            ADD CONSTRAINT content_projects_active_movie_edit_fk
            FOREIGN KEY (active_movie_edit_id) REFERENCES movie_edits(id) ON DELETE SET NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'movie_edits_latest_render_fk'
    ) THEN
        ALTER TABLE movie_edits
            ADD CONSTRAINT movie_edits_latest_render_fk
            FOREIGN KEY (latest_successful_render_id) REFERENCES movie_render_jobs(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_movie_edits_workspace_updated
    ON movie_edits(workspace_id, updated_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_movie_edits_project
    ON movie_edits(content_project_id, updated_at DESC)
    WHERE content_project_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_movie_scenes_edit_position
    ON movie_scenes(movie_edit_id, position, id);

CREATE INDEX IF NOT EXISTS idx_movie_render_jobs_workspace_created
    ON movie_render_jobs(workspace_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_movie_render_jobs_edit
    ON movie_render_jobs(movie_edit_id, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_movie_render_jobs_idempotency
    ON movie_render_jobs(workspace_id, movie_edit_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND idempotency_key <> '';

CREATE INDEX IF NOT EXISTS idx_content_projects_active_movie_edit
    ON content_projects(active_movie_edit_id)
    WHERE active_movie_edit_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_content_projects_active_rendered_video
    ON content_projects(active_rendered_video_asset_id)
    WHERE active_rendered_video_asset_id IS NOT NULL;
