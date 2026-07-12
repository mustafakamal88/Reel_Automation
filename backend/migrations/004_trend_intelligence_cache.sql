CREATE TABLE IF NOT EXISTS trend_intelligence_cache (
    cache_key TEXT PRIMARY KEY,
    payload   JSONB NOT NULL,
    stored_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_trend_intelligence_cache_expires_at
ON trend_intelligence_cache(expires_at);
