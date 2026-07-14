-- TrendCortex — Content Project foundation
-- Safe, additive migration for persistent creator production projects.

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
