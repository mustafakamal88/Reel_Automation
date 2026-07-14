-- TrendCortex — Content Project scene planning
-- Safe, additive migration for persistent ordered production scenes.

CREATE TABLE IF NOT EXISTS content_project_scenes (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id               UUID NOT NULL REFERENCES content_projects(id) ON DELETE CASCADE,
    position                 INT NOT NULL,
    title                    TEXT NOT NULL DEFAULT '',
    spoken_text              TEXT NOT NULL DEFAULT '',
    on_screen_text           TEXT NOT NULL DEFAULT '',
    visual_direction         TEXT NOT NULL DEFAULT '',
    broll_direction          TEXT NOT NULL DEFAULT '',
    camera_direction         TEXT NOT NULL DEFAULT '',
    transition_direction     TEXT NOT NULL DEFAULT '',
    planned_duration_seconds INT NOT NULL DEFAULT 5,
    production_notes         TEXT NOT NULL DEFAULT '',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT content_project_scenes_position_positive CHECK (position > 0),
    CONSTRAINT content_project_scenes_duration_bounds CHECK (planned_duration_seconds BETWEEN 1 AND 600)
);

CREATE INDEX IF NOT EXISTS idx_content_project_scenes_project_position
    ON content_project_scenes(project_id, position, created_at);

CREATE INDEX IF NOT EXISTS idx_content_project_scenes_project_updated
    ON content_project_scenes(project_id, updated_at DESC);
