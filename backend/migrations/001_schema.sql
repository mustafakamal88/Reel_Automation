-- TrendCortex — initial schema migration
-- Apply once on a fresh database. Safe to re-run: all statements use IF NOT EXISTS.
-- PostgreSQL 13+ required (gen_random_uuid() is built-in from pg 13).
--
-- To apply manually:
--   psql $DATABASE_URL -f migrations/001_schema.sql
--
-- The Go backend applies this automatically via database.DB.Migrate() on startup.

-- ── Users & Workspaces ────────────────────────────────────────────────────────

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

-- ── Platform OAuth accounts ───────────────────────────────────────────────────
-- Raw tokens are NEVER stored here; only metadata + a reference to the vault row.

CREATE TABLE IF NOT EXISTS platform_accounts (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id       UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    platform           TEXT NOT NULL,
    handle             TEXT,
    token_status       TEXT NOT NULL DEFAULT 'not_connected',
    token_vault_ref_id UUID,
    scopes             TEXT[] NOT NULL DEFAULT '{}',
    can_publish        BOOLEAN NOT NULL DEFAULT FALSE,
    connected_at       TIMESTAMPTZ,
    expires_at         TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, platform)
);

-- Temporary OAuth state stored during the redirect flow (10-minute TTL).
CREATE TABLE IF NOT EXISTS oauth_connections (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    platform      TEXT NOT NULL,
    state         TEXT NOT NULL UNIQUE,
    code_verifier TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '10 minutes'
);

-- ── Token vault ───────────────────────────────────────────────────────────────
-- Stores AES-256-GCM ciphertext only. The KMS-managed data key is stored
-- separately and never alongside the ciphertext.

CREATE TABLE IF NOT EXISTS token_vault_refs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform_key      TEXT NOT NULL,
    workspace_id      UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    encrypted_payload BYTEA NOT NULL,
    key_version       INT NOT NULL DEFAULT 1,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Video batches & assets ────────────────────────────────────────────────────

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

-- ── Jobs ─────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS publish_jobs (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    video_asset_id   UUID NOT NULL REFERENCES video_assets(id) ON DELETE CASCADE,
    platform         TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'queued',
    retry_count      INT NOT NULL DEFAULT 0,
    error_message    TEXT,
    platform_post_id TEXT,
    scheduled_for    TIMESTAMPTZ,
    started_at       TIMESTAMPTZ,
    completed_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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

-- ── Analytics & observability ─────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS analytics_snapshots (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    platform       TEXT NOT NULL,
    video_asset_id UUID NOT NULL REFERENCES video_assets(id) ON DELETE CASCADE,
    views          BIGINT NOT NULL DEFAULT 0,
    likes          BIGINT NOT NULL DEFAULT 0,
    shares         BIGINT NOT NULL DEFAULT 0,
    comments       BIGINT NOT NULL DEFAULT 0,
    watch_time_sec BIGINT NOT NULL DEFAULT 0,
    snapshot_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform_rate_limits (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform           TEXT UNIQUE NOT NULL,
    daily_upload_limit INT NOT NULL DEFAULT 3,
    min_secs_between   INT NOT NULL DEFAULT 3600,
    max_duration_sec   INT NOT NULL DEFAULT 60,
    max_file_size_mb   INT NOT NULL DEFAULT 500,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS policy_change_log (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform     TEXT NOT NULL,
    change_type  TEXT NOT NULL,
    description  TEXT NOT NULL,
    effective_at TIMESTAMPTZ NOT NULL,
    source       TEXT NOT NULL DEFAULT 'manual',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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

-- ── Seed data ─────────────────────────────────────────────────────────────────

INSERT INTO platform_rate_limits (platform, daily_upload_limit, min_secs_between, max_duration_sec, max_file_size_mb)
VALUES
    ('youtube',   100, 0,    60,  256),
    ('tiktok',    10,  3600, 60,  287),
    ('instagram', 25,  0,    90,  1000),
    ('facebook',  25,  0,    240, 1000),
    ('threads',   250, 0,    300, 1000),
    ('x',         17,  0,    140, 512)
ON CONFLICT (platform) DO NOTHING;
