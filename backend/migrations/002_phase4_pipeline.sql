-- TrendCortex — Phase 4A: trend discovery → scoring → daily batch pipeline
-- Apply once on a database that already has 001_schema.sql applied.
-- Safe to re-run: all statements use IF NOT EXISTS / are idempotent.
--
-- To apply manually:
--   psql $DATABASE_URL -f migrations/002_phase4_pipeline.sql
--
-- The Go backend applies this automatically via database.DB.Migrate() on
-- startup (see internal/database/schema.go: SchemaPhase4).
--
-- Note: "social_connections" and "audit_events" from the Phase 4A spec are
-- intentionally not duplicated here — platform_accounts (001_schema.sql)
-- and audit_log already serve that exact purpose for this workspace model.

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

ALTER TABLE publish_jobs ALTER COLUMN video_asset_id DROP NOT NULL;
ALTER TABLE publish_jobs ADD COLUMN IF NOT EXISTS reel_plan_id UUID REFERENCES reel_plans(id) ON DELETE CASCADE;
