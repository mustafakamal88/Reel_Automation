-- TrendCortex - Production Voice Studio persistence.
-- Additive and repeatable. No durable media objects are deleted.

ALTER TABLE media_assets DROP CONSTRAINT IF EXISTS media_assets_type_check;
ALTER TABLE media_assets
    ADD CONSTRAINT media_assets_type_check CHECK (asset_type IN ('source_video', 'generated_video', 'rendered_video', 'ai_scene_video', 'thumbnail', 'package', 'metadata', 'audio', 'voiceover'));

ALTER TABLE media_assets DROP CONSTRAINT IF EXISTS media_assets_workflow_check;
ALTER TABLE media_assets
    ADD CONSTRAINT media_assets_workflow_check CHECK (source_workflow IN ('clip_studio', 'clip_generator', 'project_output', 'ai_scene', 'voice_studio'));

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
