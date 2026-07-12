package trendintel

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type Cache interface {
	Get(ctx context.Context, key string, maxAge time.Duration) (*Response, *time.Time, error)
	Set(ctx context.Context, key string, response Response, ttl time.Duration) error
}

type DBCache struct {
	db *sql.DB
}

func NewDBCache(db *sql.DB) *DBCache {
	if db == nil {
		return nil
	}
	return &DBCache{db: db}
}

func (c *DBCache) Get(ctx context.Context, key string, maxAge time.Duration) (*Response, *time.Time, error) {
	var payload []byte
	var stored time.Time
	err := c.db.QueryRowContext(ctx, `SELECT payload, stored_at FROM trend_intelligence_cache WHERE cache_key = $1 AND expires_at > NOW()`, key).Scan(&payload, &stored)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if time.Since(stored) > maxAge {
		return nil, nil, nil
	}
	var res Response
	if err := json.Unmarshal(payload, &res); err != nil {
		return nil, nil, err
	}
	return &res, &stored, nil
}

func (c *DBCache) Set(ctx context.Context, key string, response Response, ttl time.Duration) error {
	payload, err := json.Marshal(response)
	if err != nil {
		return err
	}
	_, err = c.db.ExecContext(ctx, `
		INSERT INTO trend_intelligence_cache (cache_key, payload, stored_at, expires_at)
		VALUES ($1, $2, NOW(), NOW() + make_interval(secs => $3))
		ON CONFLICT (cache_key) DO UPDATE
		SET payload = EXCLUDED.payload, stored_at = EXCLUDED.stored_at, expires_at = EXCLUDED.expires_at`,
		key, payload, ttl.Seconds())
	return err
}

type MemoryCache struct {
	items map[string]memoryCacheItem
}

type memoryCacheItem struct {
	response Response
	stored   time.Time
	expires  time.Time
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{items: map[string]memoryCacheItem{}}
}

func (c *MemoryCache) Get(ctx context.Context, key string, maxAge time.Duration) (*Response, *time.Time, error) {
	_ = ctx
	item, ok := c.items[key]
	if !ok || time.Now().After(item.expires) || time.Since(item.stored) > maxAge {
		return nil, nil, nil
	}
	res := item.response
	stored := item.stored
	return &res, &stored, nil
}

func (c *MemoryCache) Set(ctx context.Context, key string, response Response, ttl time.Duration) error {
	_ = ctx
	c.items[key] = memoryCacheItem{response: response, stored: time.Now().UTC(), expires: time.Now().UTC().Add(ttl)}
	return nil
}
