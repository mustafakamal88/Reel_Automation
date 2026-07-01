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

// Migrate runs the schema DDL against the connected database.
// Safe to run multiple times due to IF NOT EXISTS clauses.
func (db *DB) Migrate() error {
	if _, err := db.Exec(Schema); err != nil {
		return err
	}
	if _, err := db.Exec(SchemaPhase4); err != nil {
		return err
	}
	_, err := db.Exec(SchemaPhase4B)
	return err
}
