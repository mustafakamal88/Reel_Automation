-- TrendCortex - Durable media storage
-- Safe additive migration. Legacy storage_reference paths remain unchanged.

ALTER TABLE content_project_outputs
    ADD COLUMN IF NOT EXISTS storage_provider TEXT,
    ADD COLUMN IF NOT EXISTS storage_key TEXT,
    ADD COLUMN IF NOT EXISTS storage_etag TEXT,
    ADD COLUMN IF NOT EXISTS storage_checksum_sha256 TEXT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'content_project_outputs_storage_provider_check'
    ) THEN
        ALTER TABLE content_project_outputs
            ADD CONSTRAINT content_project_outputs_storage_provider_check
            CHECK (storage_provider IS NULL OR storage_provider IN ('local', 's3'));
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'content_project_outputs_storage_key_check'
    ) THEN
        ALTER TABLE content_project_outputs
            ADD CONSTRAINT content_project_outputs_storage_key_check
            CHECK (
                storage_key IS NULL OR (
                    length(storage_key) > 0
                    AND storage_key !~ '(^/|(^|/)\.\.(/|$)|\\\\|//)'
                )
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'content_project_outputs_checksum_check'
    ) THEN
        ALTER TABLE content_project_outputs
            ADD CONSTRAINT content_project_outputs_checksum_check
            CHECK (storage_checksum_sha256 IS NULL OR storage_checksum_sha256 ~ '^[a-f0-9]{64}$');
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
    CONSTRAINT clip_studio_sources_storage_key_check CHECK (
        storage_key IS NULL OR (
            length(storage_key) > 0
            AND storage_key !~ '(^/|(^|/)\.\.(/|$)|\\\\|//)'
        )
    ),
    CONSTRAINT clip_studio_sources_checksum_check CHECK (storage_checksum_sha256 IS NULL OR storage_checksum_sha256 ~ '^[a-f0-9]{64}$'),
    CONSTRAINT clip_studio_sources_size_check CHECK (size_bytes IS NULL OR size_bytes >= 0),
    CONSTRAINT clip_studio_sources_ready_storage_check CHECK (
        status <> 'ready' OR (storage_provider IS NOT NULL AND storage_key IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_clip_studio_sources_workspace_updated
    ON clip_studio_sources(workspace_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_clip_studio_sources_storage_object
    ON clip_studio_sources(storage_provider, storage_key)
    WHERE storage_provider IS NOT NULL AND storage_key IS NOT NULL;
