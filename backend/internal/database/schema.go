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

-- Seed default platform rate limits
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

// Migrate runs the schema DDL against the connected database.
// Safe to run multiple times due to IF NOT EXISTS clauses.
func (db *DB) Migrate() error {
	_, err := db.Exec(Schema)
	return err
}
