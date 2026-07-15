-- TrendCortex - Durable Clip Studio export/download mapping.

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
    CONSTRAINT clip_studio_exports_storage_key_check CHECK (
        length(storage_key) > 0
        AND storage_key !~ '(^/|(^|/)\.\.(/|$)|\\\\|//)'
    ),
    CONSTRAINT clip_studio_exports_checksum_check CHECK (storage_checksum_sha256 IS NULL OR storage_checksum_sha256 ~ '^[a-f0-9]{64}$'),
    CONSTRAINT clip_studio_exports_size_check CHECK (size_bytes IS NULL OR size_bytes >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_clip_studio_exports_workspace_filename
    ON clip_studio_exports(workspace_id, filename);

CREATE INDEX IF NOT EXISTS idx_clip_studio_exports_generation
    ON clip_studio_exports(workspace_id, generation_id, export_kind)
    WHERE generation_id IS NOT NULL;
