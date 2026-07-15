-- TrendCortex - authenticated sessions and workspace membership.
-- Additive and repeatable. Existing users, workspaces, and content are preserved.

CREATE INDEX IF NOT EXISTS idx_users_workspace ON users(workspace_id);

CREATE TABLE IF NOT EXISTS workspace_memberships (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role         TEXT NOT NULL DEFAULT 'owner',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, user_id),
    CONSTRAINT workspace_memberships_role_check CHECK (role IN ('owner', 'admin', 'editor', 'viewer'))
);

CREATE INDEX IF NOT EXISTS idx_workspace_memberships_user
    ON workspace_memberships(user_id, workspace_id);

INSERT INTO workspace_memberships (workspace_id, user_id, role)
SELECT w.id, u.id, CASE WHEN w.owner_id = u.id THEN 'owner' ELSE 'editor' END
FROM users u
JOIN workspaces w ON w.id = u.workspace_id OR w.owner_id = u.id
ON CONFLICT (workspace_id, user_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS user_sessions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_hash   TEXT UNIQUE NOT NULL,
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id   UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    csrf_hash      TEXT NOT NULL,
    user_agent     TEXT,
    ip_address     TEXT,
    expires_at     TIMESTAMPTZ NOT NULL,
    revoked_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_lookup
    ON user_sessions(session_hash)
    WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_sessions_user
    ON user_sessions(user_id, expires_at DESC);
