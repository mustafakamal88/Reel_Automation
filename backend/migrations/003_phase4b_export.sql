-- TrendCortex — Phase 4B: real ZIP export — per-reel video/thumbnail artifact tracking
-- Apply once on a database that already has 002_phase4_pipeline.sql applied.
-- Safe to re-run: all statements use ADD COLUMN IF NOT EXISTS.
--
-- To apply manually:
--   psql $DATABASE_URL -f migrations/003_phase4b_export.sql
--
-- The Go backend applies this automatically via database.DB.Migrate() on
-- startup (see internal/database/schema.go: SchemaPhase4B).
--
-- These columns stay NULL/empty until a real render pipeline writes a real
-- video.mp4 / thumbnail.png to disk and records its path here — export
-- honestly reports artifacts as missing until that happens.

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
