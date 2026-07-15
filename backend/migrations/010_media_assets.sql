-- TrendCortex - Canonical durable media asset index.
-- Additive and repeatable. This migration never deletes durable objects.

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
    CONSTRAINT media_assets_type_check CHECK (asset_type IN ('source_video', 'generated_video', 'rendered_video', 'ai_scene_video', 'thumbnail', 'package', 'metadata')),
    CONSTRAINT media_assets_workflow_check CHECK (source_workflow IN ('clip_studio', 'clip_generator', 'project_output', 'ai_scene')),
    CONSTRAINT media_assets_status_check CHECK (status IN ('ready', 'processing', 'failed', 'unavailable', 'archived')),
    CONSTRAINT media_assets_storage_provider_check CHECK (storage_provider IN ('local', 's3')),
    CONSTRAINT media_assets_storage_key_check CHECK (
        length(storage_key) > 0
        AND storage_key !~ '(^/|(^|/)\.\.(/|$)|\\\\|//)'
    ),
    CONSTRAINT media_assets_checksum_check CHECK (storage_checksum_sha256 IS NULL OR storage_checksum_sha256 ~ '^[a-f0-9]{64}$'),
    CONSTRAINT media_assets_size_check CHECK (size_bytes IS NULL OR size_bytes >= 0),
    CONSTRAINT media_assets_duration_check CHECK (duration_seconds IS NULL OR duration_seconds >= 0),
    CONSTRAINT media_assets_width_check CHECK (width IS NULL OR width >= 0),
    CONSTRAINT media_assets_height_check CHECK (height IS NULL OR height >= 0),
    CONSTRAINT media_assets_ready_storage_check CHECK (
        status <> 'ready' OR (storage_provider IS NOT NULL AND storage_key IS NOT NULL)
    )
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

WITH source_assets AS (
    INSERT INTO media_assets (
        workspace_id, asset_type, source_workflow, display_name, original_filename, mime_type,
        size_bytes, storage_provider, storage_key, storage_etag, storage_checksum_sha256,
        status, failure_category, failure_message, created_at, updated_at
    )
    SELECT
        s.workspace_id,
        'source_video',
        'clip_studio',
        COALESCE(NULLIF(s.original_filename, ''), 'Clip Studio source'),
        NULLIF(s.original_filename, ''),
        NULLIF(s.content_type, ''),
        s.size_bytes,
        s.storage_provider,
        s.storage_key,
        NULLIF(s.storage_etag, ''),
        NULLIF(s.storage_checksum_sha256, ''),
        CASE WHEN s.status = 'ready' THEN 'ready' ELSE 'unavailable' END,
        CASE WHEN s.status = 'ready' THEN NULL ELSE 'source_unavailable' END,
        CASE WHEN s.status = 'ready' THEN NULL ELSE NULLIF(s.status_message, '') END,
        s.created_at,
        s.updated_at
    FROM clip_studio_sources s
    WHERE s.storage_provider IN ('local', 's3')
      AND s.storage_key IS NOT NULL
      AND s.storage_key <> ''
      AND s.storage_key !~ '(^/|(^|/)\.\.(/|$)|\\\\|//)'
    ON CONFLICT (workspace_id, storage_provider, storage_key) DO UPDATE SET
        display_name = COALESCE(NULLIF(media_assets.display_name, ''), EXCLUDED.display_name),
        original_filename = COALESCE(media_assets.original_filename, EXCLUDED.original_filename),
        mime_type = COALESCE(media_assets.mime_type, EXCLUDED.mime_type),
        size_bytes = COALESCE(media_assets.size_bytes, EXCLUDED.size_bytes),
        storage_etag = COALESCE(media_assets.storage_etag, EXCLUDED.storage_etag),
        storage_checksum_sha256 = COALESCE(media_assets.storage_checksum_sha256, EXCLUDED.storage_checksum_sha256),
        updated_at = NOW()
    RETURNING id, workspace_id, storage_provider, storage_key
)
UPDATE clip_studio_sources s
SET asset_id = a.id
FROM media_assets a
WHERE s.workspace_id = a.workspace_id
  AND s.storage_provider = a.storage_provider
  AND s.storage_key = a.storage_key
  AND s.asset_id IS DISTINCT FROM a.id;

WITH export_assets AS (
    INSERT INTO media_assets (
        workspace_id, asset_type, source_workflow, display_name, original_filename, mime_type,
        size_bytes, storage_provider, storage_key, storage_etag, storage_checksum_sha256,
        status, created_at, updated_at
    )
    SELECT
        e.workspace_id,
        CASE
            WHEN e.export_kind = 'ai_scene_video' THEN 'ai_scene_video'
            WHEN e.export_kind = 'ai_scene_thumbnail' THEN 'thumbnail'
            ELSE 'package'
        END,
        CASE WHEN e.export_kind LIKE 'ai_scene_%' THEN 'ai_scene' ELSE 'clip_studio' END,
        e.filename,
        e.filename,
        NULLIF(e.mime_type, ''),
        e.size_bytes,
        e.storage_provider,
        e.storage_key,
        NULLIF(e.storage_etag, ''),
        NULLIF(e.storage_checksum_sha256, ''),
        'ready',
        e.created_at,
        e.updated_at
    FROM clip_studio_exports e
    WHERE e.storage_provider IN ('local', 's3')
      AND e.storage_key IS NOT NULL
      AND e.storage_key <> ''
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
        updated_at = NOW()
    RETURNING id, workspace_id, storage_provider, storage_key
)
UPDATE clip_studio_exports e
SET asset_id = a.id
FROM media_assets a
WHERE e.workspace_id = a.workspace_id
  AND e.storage_provider = a.storage_provider
  AND e.storage_key = a.storage_key
  AND e.asset_id IS DISTINCT FROM a.id;

WITH output_assets AS (
    INSERT INTO media_assets (
        workspace_id, project_id, scene_id, asset_type, source_workflow, display_name, original_filename, mime_type,
        size_bytes, duration_seconds, width, height, storage_provider, storage_key, storage_etag, storage_checksum_sha256,
        status, failure_category, failure_message, archived_at, created_at, updated_at
    )
    SELECT
        p.workspace_id,
        o.project_id,
        o.scene_id,
        CASE
            WHEN o.output_type IN ('uploaded_clip', 'imported_clip') THEN 'source_video'
            WHEN o.output_type = 'rendered_video' THEN 'rendered_video'
            WHEN o.mime_type = 'application/zip' THEN 'package'
            ELSE 'generated_video'
        END,
        'clip_generator',
        o.display_name,
        NULLIF(o.original_filename, ''),
        NULLIF(o.mime_type, ''),
        o.file_size_bytes,
        o.duration_seconds,
        o.width,
        o.height,
        o.storage_provider,
        o.storage_key,
        NULLIF(o.storage_etag, ''),
        NULLIF(o.storage_checksum_sha256, ''),
        CASE WHEN o.archived_at IS NOT NULL THEN 'archived' WHEN o.status = 'completed' THEN 'ready' WHEN o.status IN ('queued', 'processing') THEN 'processing' WHEN o.status = 'failed' THEN 'failed' ELSE 'unavailable' END,
        NULLIF(o.failure_category, ''),
        NULLIF(o.failure_message, ''),
        o.archived_at,
        o.created_at,
        o.updated_at
    FROM content_project_outputs o
    JOIN content_projects p ON p.id = o.project_id
    WHERE o.storage_provider IN ('local', 's3')
      AND o.storage_key IS NOT NULL
      AND o.storage_key <> ''
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
        updated_at = NOW()
    RETURNING id, workspace_id, storage_provider, storage_key
)
UPDATE content_project_outputs o
SET asset_id = a.id
FROM content_projects p, media_assets a
WHERE p.id = o.project_id
  AND p.workspace_id = a.workspace_id
  AND o.storage_provider = a.storage_provider
  AND o.storage_key = a.storage_key
  AND o.asset_id IS DISTINCT FROM a.id;
